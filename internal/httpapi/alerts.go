package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"
	"observatory.local/platform/internal/alerts"
	"observatory.local/platform/internal/store"
	"strings"
	"time"
)

type AlertRepository interface {
	AlertRule(context.Context, string) (alerts.Rule, error)
	SaveAlertRule(context.Context, alerts.Rule) (alerts.Rule, error)
	Alerts(context.Context, string, bool, bool) ([]alerts.Alert, int64, error)
	DismissAlert(context.Context, string) error
}

func alertScope(w http.ResponseWriter, r *http.Request) (string, bool, bool) {
	q := r.URL.Query()
	project := q.Get("project")
	solo := q.Get("standalone")
	if len(project) > 512 || strings.IndexByte(project, 0) >= 0 || (solo != "" && solo != "true" && solo != "false") || (project != "" && solo == "true") {
		respond(w, 400, map[string]string{"error": "Choose a project or standalone scope"})
		return "", false, false
	}
	return project, solo == "true", true
}
func (s *Server) alertFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrAlertNotFound) {
		respond(w, 404, map[string]string{"error": "Alert not found or expired"})
		return
	}
	s.Log.Error("alert operation failed", "error", err)
	respond(w, 503, map[string]string{"error": "Alerts temporarily unavailable. Refresh before retrying changes."})
}
func (s *Server) alertList(w http.ResponseWriter, r *http.Request) {
	project, solo, ok := alertScope(w, r)
	if !ok {
		return
	}
	if s.Alerts == nil {
		respond(w, 503, map[string]string{"error": "Alerts are not configured"})
		return
	}
	include := r.URL.Query().Get("include_dismissed")
	if include != "" && include != "true" && include != "false" {
		respond(w, 400, map[string]string{"error": "include_dismissed must be true or false"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items, count, err := s.Alerts.Alerts(ctx, project, solo, include == "true")
	if err != nil {
		s.alertFailure(w, err)
		return
	}
	more := len(items) > 100
	if more {
		items = items[:100]
	}
	respond(w, 200, map[string]any{"alerts": items, "needs_review": count, "has_more": more, "server_time": time.Now().UTC(), "sample_interval_seconds": s.Interval.Seconds(), "collector": s.Status()})
}
func (s *Server) alertRule(w http.ResponseWriter, r *http.Request) {
	project, solo, ok := alertScope(w, r)
	if !ok {
		return
	}
	if project == "" && !solo {
		respond(w, 400, map[string]string{"error": "Select one project or standalone containers to configure alerts"})
		return
	}
	if s.Alerts == nil {
		respond(w, 503, map[string]string{"error": "Alerts are not configured"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if r.Method == "GET" {
		rule, err := s.Alerts.AlertRule(ctx, project)
		if err != nil {
			s.alertFailure(w, err)
			return
		}
		respond(w, 200, rule)
		return
	}
	if !sessionWriteAllowed(w, r) {
		return
	}
	var input struct {
		Enabled         bool    `json:"enabled"`
		CPUEnabled      bool    `json:"cpu_enabled"`
		MemoryEnabled   bool    `json:"memory_enabled"`
		CPUThreshold    float64 `json:"cpu_threshold"`
		MemoryThreshold float64 `json:"memory_threshold"`
		NonzeroExit     bool    `json:"nonzero_exit"`
		OOM             bool    `json:"oom"`
		Unhealthy       bool    `json:"unhealthy"`
	}
	if !readSessionJSON(w, r, &input) {
		return
	}
	if math.IsNaN(input.CPUThreshold) || math.IsInf(input.CPUThreshold, 0) || input.CPUThreshold <= 0 || input.CPUThreshold > 100000 || math.IsNaN(input.MemoryThreshold) || math.IsInf(input.MemoryThreshold, 0) || input.MemoryThreshold <= 0 || input.MemoryThreshold > 100 {
		respond(w, 400, map[string]string{"error": "CPU must be above 0 and at most 100000%; memory must be above 0 and at most 100%"})
		return
	}
	rule, err := s.Alerts.SaveAlertRule(ctx, alerts.Rule{Project: project, Enabled: input.Enabled, CPUEnabled: input.CPUEnabled, MemoryEnabled: input.MemoryEnabled, CPUThreshold: input.CPUThreshold, MemoryThreshold: input.MemoryThreshold, NonzeroExit: input.NonzeroExit, OOM: input.OOM, Unhealthy: input.Unhealthy})
	if err != nil {
		s.alertFailure(w, err)
		return
	}
	respond(w, 200, rule)
}
func (s *Server) dismissAlert(w http.ResponseWriter, r *http.Request) {
	if !sessionWriteAllowed(w, r) {
		return
	}
	id := r.PathValue("id")
	if !validSessionID(id) {
		respond(w, 400, map[string]string{"error": "Invalid alert ID"})
		return
	}
	var input struct{}
	if !readSessionJSON(w, r, &input) {
		return
	}
	if s.Alerts == nil {
		respond(w, 503, map[string]string{"error": "Alerts are not configured"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := s.Alerts.DismissAlert(ctx, id); err != nil {
		s.alertFailure(w, err)
		return
	}
	respond(w, 200, map[string]bool{"dismissed": true})
}
