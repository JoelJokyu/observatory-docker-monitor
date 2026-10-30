package httpapi

import (
	"context"
	"encoding/hex"
	"net/http"
	"observatory.local/platform/internal/events"
	"observatory.local/platform/internal/model"
	"strconv"
	"time"
)

type EventRepository interface {
	Projects(context.Context) ([]string, error)
	Events(context.Context, time.Time, time.Time, string, string, bool, int) ([]model.Event, error)
}

func (s *Server) eventTimeline(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	from, to := now.Add(-time.Hour), now
	q := r.URL.Query()
	for key, target := range map[string]*time.Time{"from": &from, "to": &to} {
		if value := q.Get(key); value != "" {
			parsed, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				respond(w, 400, map[string]string{"error": key + " must be RFC3339"})
				return
			}
			*target = parsed
		}
	}
	if !from.Before(to) || to.Sub(from) > 7*24*time.Hour || from.Before(now.Add(-7*24*time.Hour-time.Minute)) || to.After(now.Add(time.Minute)) {
		respond(w, 400, map[string]string{"error": "Choose an ordered time range within the last seven days"})
		return
	}
	limit := 200
	if value := q.Get("limit"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 500 {
			respond(w, 400, map[string]string{"error": "limit must be between 1 and 500"})
			return
		}
		limit = n
	}
	id, project := q.Get("container_id"), q.Get("project")
	if id != "" {
		_, err := hex.DecodeString(id)
		if len(id) != 64 || err != nil {
			respond(w, 400, map[string]string{"error": "container_id must be a full Docker ID"})
			return
		}
	}
	if len(project) > 512 {
		respond(w, 400, map[string]string{"error": "Project name is too long"})
		return
	}
	standalone := false
	if value := q.Get("standalone"); value != "" {
		if value != "true" && value != "false" {
			respond(w, 400, map[string]string{"error": "standalone must be true or false"})
			return
		}
		standalone = value == "true"
	}
	if standalone && project != "" {
		respond(w, 400, map[string]string{"error": "Choose either a project or standalone containers"})
		return
	}
	if s.Events == nil {
		respond(w, 503, map[string]string{"error": "Event collection is not configured"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items, err := s.Events.Events(ctx, from, to, project, id, standalone, limit+1)
	if err != nil {
		s.Log.Error("event query failed", "error", err)
		respond(w, 503, map[string]string{"error": "Events temporarily unavailable"})
		return
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	status := events.Status{Message: "Event collector unavailable"}
	if s.EventStatus != nil {
		status = s.EventStatus()
	}
	respond(w, 200, map[string]any{"events": items, "from": from, "to": to, "has_more": more, "limit": limit, "collector": status, "coverage_notice": "Recorded events only. Docker replay is limited to its recent event buffer; startup or disconnection can leave gaps. Events are retained for seven days."})
}
