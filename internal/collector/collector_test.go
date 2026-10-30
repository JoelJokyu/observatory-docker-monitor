package collector

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"observatory.local/platform/internal/model"
)

type fakeEngine struct {
	failList  bool
	failStats bool
}

func (e *fakeEngine) ContainerList(context.Context, client.ContainerListOptions) (client.ContainerListResult, error) {
	if e.failList {
		return client.ContainerListResult{}, errors.New("offline")
	}
	return client.ContainerListResult{Items: []container.Summary{{ID: "container-1", Names: []string{"/chat"}, State: "running", Labels: map[string]string{"com.docker.compose.project": "chat-project", "com.docker.compose.service": "api"}}}}, nil
}
func (e *fakeEngine) ContainerStats(context.Context, string, client.ContainerStatsOptions) (client.ContainerStatsResult, error) {
	if e.failStats {
		return client.ContainerStatsResult{}, errors.New("stopped during sampling")
	}
	payload := `{"read":"` + time.Now().UTC().Format(time.RFC3339Nano) + `","cpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":1000,"online_cpus":4},"memory_stats":{"usage":1000,"limit":2000}}`
	return client.ContainerStatsResult{Body: io.NopCloser(strings.NewReader(payload))}, nil
}

type fakeStore struct {
	inventory     []model.Container
	inventories   int
	samples       []model.Sample
	failSave      bool
	failInventory bool
}

func (s *fakeStore) Inventory(_ context.Context, containers []model.Container) error {
	s.inventory = containers
	s.inventories++
	if s.failInventory {
		return errors.New("database unavailable")
	}
	return nil
}
func (s *fakeStore) Save(_ context.Context, v model.Sample) error {
	if s.failSave {
		return errors.New("database unavailable")
	}
	s.samples = append(s.samples, v)
	return nil
}
func (s *fakeStore) Prune(context.Context, time.Time) error { return nil }
func TestCollectionFailureAndRecovery(t *testing.T) {
	engine := &fakeEngine{}
	db := &fakeStore{}
	c := &Collector{Engine: engine, Store: db, Interval: 10 * time.Second, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), previous: make(map[string]Stats)}
	c.cycle(context.Background())
	if !c.Status().Healthy || len(db.samples) != 1 {
		t.Fatal("initial collection failed")
	}
	if len(db.inventory) != 1 || db.inventory[0].Project != "chat-project" || db.inventory[0].Service != "api" {
		t.Fatal("Compose labels lost before persistence")
	}
	engine.failList = true
	c.cycle(context.Background())
	if c.Status().Healthy || db.inventories != 1 || len(db.samples) != 1 {
		t.Fatal("failed list must preserve inventory and avoid fabricated samples")
	}
	engine.failList = false
	c.cycle(context.Background())
	if !c.Status().Healthy || db.samples[1].CPU != nil {
		t.Fatal("recovery must resume with unknown CPU baseline")
	}
	engine.failStats = true
	c.cycle(context.Background())
	if c.Status().Healthy || len(db.samples) != 2 {
		t.Fatal("stats failure must not fabricate a sample")
	}
	engine.failStats = false
	db.failSave = true
	c.cycle(context.Background())
	if c.Status().Healthy {
		t.Fatal("storage failure must degrade collector status")
	}
}

func TestInventoryWriteFailureResetsRateBaseline(t *testing.T) {
	engine := &fakeEngine{}
	db := &fakeStore{}
	c := &Collector{Engine: engine, Store: db, Interval: 10 * time.Second, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), previous: make(map[string]Stats)}
	c.cycle(context.Background())
	db.failInventory = true
	c.cycle(context.Background())
	if c.Status().Healthy || len(c.previous) != 0 {
		t.Fatal("inventory write failure must invalidate the rate baseline")
	}
	db.failInventory = false
	c.cycle(context.Background())
	if !c.Status().Healthy || len(db.samples) != 2 || db.samples[1].CPU != nil {
		t.Fatal("recovery must begin with unknown rates")
	}
}
