package httpapi

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"time"

	containerlogs "observatory.local/platform/internal/logs"
)

type LogReader interface {
	Snapshot(context.Context, string, time.Time, time.Time, int) (containerlogs.Snapshot, error)
}

func (s *Server) logSnapshot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := hex.DecodeString(id); err != nil || len(id) != 64 {
		respond(w, 400, map[string]string{"code": "invalid_query", "error": "A full Docker container ID is required"})
		return
	}
	now := time.Now().UTC()
	from, to := now.Add(-time.Hour), now
	for name, destination := range map[string]*time.Time{"from": &from, "to": &to} {
		if raw := r.URL.Query().Get(name); raw != "" {
			parsed, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				respond(w, 400, map[string]string{"code": "invalid_query", "error": name + " must be an RFC3339 timestamp"})
				return
			}
			*destination = parsed
		}
	}
	if !from.Before(to) || to.Sub(from) > 7*24*time.Hour || from.Before(now.Add(-7*24*time.Hour-time.Minute)) || to.After(now.Add(time.Minute)) {
		respond(w, 400, map[string]string{"code": "invalid_query", "error": "Choose an ordered time range within the last seven days"})
		return
	}
	limit := containerlogs.DefaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > containerlogs.MaxLimit {
			respond(w, 400, map[string]string{"code": "invalid_query", "error": "limit must be between 1 and 1000"})
			return
		}
		limit = n
	}
	if s.Logs == nil {
		respond(w, 503, map[string]string{"code": "logs_not_configured", "error": "Docker log access is not configured"})
		return
	}
	snapshot, err := s.Logs.Snapshot(r.Context(), id, from, to, limit)
	if err == nil {
		respond(w, 200, snapshot)
		return
	}
	code, message, status := "docker_unavailable", "Docker log access is temporarily unavailable. Retry the snapshot.", http.StatusServiceUnavailable
	var failure *containerlogs.Failure
	switch {
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, context.DeadlineExceeded):
		code, message, status = "logs_timeout", "The log request timed out. Try a narrower time window.", http.StatusGatewayTimeout
	case errors.As(err, &failure):
		code = failure.Code
		switch code {
		case "container_not_found":
			message, status = "Docker no longer has this container. Resource history may still be available, but logs are not archived by Observatory.", http.StatusNotFound
		case "logs_unsupported":
			message, status = "This container's Docker logging configuration does not provide readable logs.", http.StatusUnprocessableEntity
		case "invalid_docker_response":
			message, status = "Docker returned an incomplete or invalid log stream. Retry the snapshot.", http.StatusBadGateway
		}
	}
	// Never write raw log contents or opaque Docker error strings to platform logs.
	s.Log.Warn("log snapshot failed", "container_id", id, "code", code)
	respond(w, status, map[string]string{"code": code, "error": message})
}
