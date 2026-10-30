package httpapi

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"observatory.local/platform/internal/store"
)

type SessionRepository interface {
	StartSession(context.Context, string, string, bool, int) (store.Session, error)
	FinishSession(context.Context, string) (store.Session, error)
	Sessions(context.Context, string, bool, int) ([]store.Session, error)
	SessionSummary(context.Context, string) (store.SessionSummary, error)
}

func validSessionID(id string) bool {
	_, err := hex.DecodeString(id)
	return len(id) == 32 && err == nil
}
func sessionWriteAllowed(w http.ResponseWriter, r *http.Request) bool {
	// New local write endpoints accept JSON only and reject cross-site requests.
	// This is origin checking, not authentication.
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		respond(w, 403, map[string]string{"error": "Cross-site session changes are not allowed"})
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if err != nil || u.Host != r.Host || u.Scheme != scheme || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			respond(w, 403, map[string]string{"error": "Session changes must come from this dashboard"})
			return false
		}
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		respond(w, 415, map[string]string{"error": "Send application/json"})
		return false
	}
	return true
}
func readSessionJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		respond(w, 400, map[string]string{"error": "Invalid session JSON (maximum 2 KiB)"})
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		respond(w, 400, map[string]string{"error": "Send one JSON object"})
		return false
	}
	return true
}
func (s *Server) sessionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrSessionActive):
		respond(w, 409, map[string]string{"code": "session_active", "error": "A session is already active for this project. Open it to continue or finish it."})
	case errors.Is(err, store.ErrSessionScope):
		respond(w, 409, map[string]string{"error": "Start a container in this project and wait for fresh inventory before starting a session."})
	case errors.Is(err, store.ErrSessionNotFound):
		respond(w, 404, map[string]string{"error": "Session not found or expired"})
	default:
		s.Log.Error("session operation failed", "error", err)
		respond(w, 503, map[string]string{"error": "Sessions temporarily unavailable. Refresh to check whether your change was saved before retrying."})
	}
}
func (s *Server) sessions(w http.ResponseWriter, r *http.Request) {
	if s.Sessions == nil {
		respond(w, 503, map[string]string{"error": "Sessions are not configured"})
		return
	}
	q := r.URL.Query()
	project := q.Get("project")
	standalone := q.Get("standalone") == "true"
	if len(project) > 512 || (q.Get("standalone") != "" && q.Get("standalone") != "true" && q.Get("standalone") != "false") || (project != "" && standalone) {
		respond(w, 400, map[string]string{"error": "Choose either a project or standalone scope"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items, err := s.Sessions.Sessions(ctx, project, standalone, 101)
	if err != nil {
		s.sessionError(w, err)
		return
	}
	more := len(items) > 100
	if more {
		items = items[:100]
	}
	respond(w, 200, map[string]any{"sessions": items, "has_more": more, "server_time": time.Now().UTC()})
}
func (s *Server) startSession(w http.ResponseWriter, r *http.Request) {
	if !sessionWriteAllowed(w, r) {
		return
	}
	var input struct {
		Name       string `json:"name"`
		Project    string `json:"project"`
		Standalone bool   `json:"standalone"`
	}
	if !readSessionJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	invalidName := !utf8.ValidString(input.Name) || utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 80 || strings.IndexFunc(input.Name, unicode.IsControl) >= 0
	if invalidName || len(input.Project) > 512 || strings.IndexByte(input.Project, 0) >= 0 || ((input.Project == "") == !input.Standalone) {
		respond(w, 400, map[string]string{"error": "Use a name of 1–80 characters and choose one project or standalone containers"})
		return
	}
	if s.Sessions == nil {
		respond(w, 503, map[string]string{"error": "Sessions are not configured"})
		return
	}
	status := s.Status()
	if status.LastInventory == nil || time.Since(*status.LastInventory) > max(3*s.Interval, 15*time.Second) {
		respond(w, 409, map[string]string{"error": "Wait for fresh Docker inventory before starting a session"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	v, err := s.Sessions.StartSession(ctx, input.Name, input.Project, input.Standalone, int(s.Interval.Seconds()))
	if err != nil {
		s.sessionError(w, err)
		return
	}
	respond(w, 201, v)
}
func (s *Server) finishSession(w http.ResponseWriter, r *http.Request) {
	if !sessionWriteAllowed(w, r) {
		return
	}
	id := r.PathValue("id")
	if !validSessionID(id) {
		respond(w, 400, map[string]string{"error": "Invalid session ID"})
		return
	}
	var input struct{}
	if !readSessionJSON(w, r, &input) {
		return
	}
	if s.Sessions == nil {
		respond(w, 503, map[string]string{"error": "Sessions are not configured"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	v, err := s.Sessions.FinishSession(ctx, id)
	if err != nil {
		s.sessionError(w, err)
		return
	}
	respond(w, 200, v)
}
func (s *Server) sessionSummary(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validSessionID(id) {
		respond(w, 400, map[string]string{"error": "Invalid session ID"})
		return
	}
	if s.Sessions == nil {
		respond(w, 503, map[string]string{"error": "Sessions are not configured"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	v, err := s.Sessions.SessionSummary(ctx, id)
	if err != nil {
		s.sessionError(w, err)
		return
	}
	respond(w, 200, map[string]any{"summary": v, "collector": s.Status(), "coverage_notice": "Summaries use available recorded observations, not continuous coverage. CPU means are sample averages; peaks are per container. Docker event collection can have gaps. Logs are not archived."})
}
