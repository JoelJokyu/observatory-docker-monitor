package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moby/moby/client"
	"observatory.local/platform/internal/collector"
	"observatory.local/platform/internal/events"
	"observatory.local/platform/internal/httpapi"
	containerlogs "observatory.local/platform/internal/logs"
	"observatory.local/platform/internal/store"
)

var version = "dev"
var revision = "unknown"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("Observatory %s (%s)\n", version, revision)
		return
	}
	if err := run(); err != nil {
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required; see README.md")
	}
	interval := 10 * time.Second
	if v := os.Getenv("SAMPLE_INTERVAL"); v != "" {
		var err error
		interval, err = time.ParseDuration(v)
		if err != nil || interval < 5*time.Second || interval > time.Minute {
			return errors.New("SAMPLE_INTERVAL must be between 5s and 1m")
		}
	}
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	db, err := store.Open(startup, url)
	cancel()
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Pool.Close()
	db.SampleInterval = interval
	engine, err := client.New(client.FromEnv)
	if err != nil {
		return err
	}
	defer engine.Close()
	collect := &collector.Collector{Engine: engine, Store: db, Interval: interval, Log: log}
	eventCollect := &events.Collector{Engine: engine, Store: db, Log: log}
	eventDone := make(chan struct{})
	go func() { defer close(eventDone); eventCollect.Run(ctx) }()
	done := make(chan struct{})
	go func() { defer close(done); collect.Run(ctx) }()
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	api := &httpapi.Server{Alerts: db, Sessions: db, Events: db, EventStatus: eventCollect.Status, Logs: &containerlogs.Service{Engine: engine}, Store: db, Status: collect.Status, Ping: db.Pool.Ping, Interval: interval, Log: log}
	server := &http.Server{Addr: addr, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	serverErr := make(chan error, 1)
	go func() { log.Info("dashboard listening", "address", addr); serverErr <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
	case err = <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			stop()
		}
	}
	stop()
	shutdown, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	shutdownErr := server.Shutdown(shutdown)
	<-done
	<-eventDone
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return shutdownErr
}
