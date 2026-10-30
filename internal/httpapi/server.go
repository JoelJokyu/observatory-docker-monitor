package httpapi

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"time"

	"observatory.local/platform/internal/collector"
	"observatory.local/platform/internal/events"
	"observatory.local/platform/internal/model"
	"observatory.local/platform/internal/store"
)

//go:embed web/*
var assets embed.FS

type Repository interface {
	Containers(context.Context) ([]model.Container, error)
	History(context.Context, string, time.Time, time.Time, int) ([]store.Bucket, error)
}
type Server struct {
	Alerts      AlertRepository
	Sessions    SessionRepository
	Events      EventRepository
	EventStatus func() events.Status
	Logs        LogReader
	Store       Repository
	Status      func() collector.Status
	Ping        func(context.Context) error
	Interval    time.Duration
	Log         *slog.Logger
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	logSlots := make(chan struct{}, 4)
	mux.HandleFunc("GET /api/containers/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		select {
		case logSlots <- struct{}{}:
			defer func() { <-logSlots }()
			s.logSnapshot(w, r)
		default:
			w.Header().Set("Retry-After", "2")
			respond(w, 503, map[string]string{"code": "logs_busy", "error": "Log snapshot capacity is busy. Retry shortly."})
		}
	})
	mux.HandleFunc("GET /api/sessions", s.sessions)
	mux.HandleFunc("POST /api/sessions", s.startSession)
	mux.HandleFunc("POST /api/sessions/{id}/finish", s.finishSession)
	mux.HandleFunc("GET /api/sessions/{id}", s.sessionSummary)
	mux.HandleFunc("GET /api/alerts", s.alertList)
	mux.HandleFunc("GET /api/alert-rules", s.alertRule)
	mux.HandleFunc("PUT /api/alert-rules", s.alertRule)
	mux.HandleFunc("POST /api/alerts/{id}/dismiss", s.dismissAlert)
	mux.HandleFunc("GET /api/events", s.eventTimeline)
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/containers", s.containers)
	mux.HandleFunc("GET /api/containers/{id}/history", s.history)
	web, _ := fs.Sub(assets, "web")
	mux.Handle("GET /", http.FileServer(http.FS(web)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
			w.Header().Set("Cache-Control", "no-store")
		}
		mux.ServeHTTP(w, r)
	})
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	databaseOK := s.Ping(ctx) == nil
	status := s.Status()
	fresh := status.LastInventory != nil && time.Since(*status.LastInventory) <= max(3*s.Interval, 15*time.Second)
	code := http.StatusOK
	if !databaseOK || !status.Healthy || !fresh {
		code = http.StatusServiceUnavailable
	}
	respond(w, code, map[string]any{"database_ok": databaseOK, "collector": status, "inventory_fresh": fresh, "sample_interval_seconds": s.Interval.Seconds()})
}
func (s *Server) containers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items, err := s.Store.Containers(ctx)
	if err != nil {
		s.Log.Error("list query failed", "error", err)
		respond(w, 503, map[string]string{"error": "Database temporarily unavailable"})
		return
	}
	var projects []string
	if s.Events != nil {
		projects, err = s.Events.Projects(ctx)
		if err != nil {
			s.Log.Error("project query failed", "error", err)
			respond(w, 503, map[string]string{"error": "Projects temporarily unavailable"})
			return
		}
	}
	respond(w, 200, map[string]any{"projects": projects, "containers": items, "collector": s.Status(), "server_time": time.Now().UTC(), "sample_interval_seconds": s.Interval.Seconds()})
}
func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	from := now.Add(-time.Hour)
	to := now
	var err error
	if v := r.URL.Query().Get("from"); v != "" {
		from, err = time.Parse(time.RFC3339, v)
		if err != nil {
			respond(w, 400, map[string]string{"error": "from must be RFC3339"})
			return
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		to, err = time.Parse(time.RFC3339, v)
		if err != nil {
			respond(w, 400, map[string]string{"error": "to must be RFC3339"})
			return
		}
	}
	if !from.Before(to) || to.Sub(from) > 7*24*time.Hour || from.Before(now.Add(-7*24*time.Hour-time.Minute)) || to.After(now.Add(time.Minute)) {
		respond(w, 400, map[string]string{"error": "Choose an ordered time range within the last seven days"})
		return
	}
	bucket := max(int(math.Ceil(to.Sub(from).Seconds()/600)), int(math.Ceil(s.Interval.Seconds())))
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	data, err := s.Store.History(ctx, r.PathValue("id"), from, to, bucket)
	if err != nil {
		s.Log.Error("history query failed", "error", err)
		respond(w, 503, map[string]string{"error": "History temporarily unavailable"})
		return
	}
	respond(w, 200, map[string]any{"from": from, "to": to, "bucket_seconds": bucket, "aggregation": "mean", "samples": data})
}
