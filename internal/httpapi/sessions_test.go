package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"observatory.local/platform/internal/collector"
	"observatory.local/platform/internal/store"
	"strings"
	"testing"
	"time"
)

type sessionRepo struct {
	calls         int
	fail          error
	name, project string
	standalone    bool
	items         []store.Session
}

func (f *sessionRepo) StartSession(_ context.Context, name, project string, standalone bool, _ int) (store.Session, error) {
	f.calls++
	f.name = name
	f.project = project
	f.standalone = standalone
	return store.Session{ID: strings.Repeat("a", 32), Name: name}, f.fail
}
func (f *sessionRepo) FinishSession(context.Context, string) (store.Session, error) {
	f.calls++
	return store.Session{}, f.fail
}
func (f *sessionRepo) Sessions(_ context.Context, project string, standalone bool, _ int) ([]store.Session, error) {
	f.calls++
	f.project = project
	f.standalone = standalone
	if f.items == nil {
		return []store.Session{}, f.fail
	}
	return f.items, f.fail
}
func (f *sessionRepo) SessionSummary(context.Context, string) (store.SessionSummary, error) {
	f.calls++
	return store.SessionSummary{}, f.fail
}
func TestSessionWriteValidationAndOrigins(t *testing.T) {
	repo := &sessionRepo{}
	s := testServer(&fakeRepository{})
	s.Sessions = repo
	now := time.Now()
	s.Status = func() collector.Status { return collector.Status{Healthy: true, LastInventory: &now} }
	handler := s.Handler()
	for _, body := range []string{`{}`, `null`, `{"name":"x"}`, `{"name":"x","project":"chat","standalone":true}`, `{"name":" ","project":"chat"}`, `{"name":"bad\nname","project":"chat"}`, `{"name":"` + strings.Repeat("x", 81) + `","project":"chat"}`, `{"name":"x","project":"chat","unknown":true}`, `{"name":"x","project":"chat"} {}`, strings.Repeat(" ", 2050) + `{}`} {
		r := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal("invalid payload", body, w.Code, w.Body.String())
		}
	}
	for _, tc := range []struct {
		origin, media, fetch string
		status               int
	}{{"http://evil.test", "application/json", "", 403}, {"null", "application/json", "", 403}, {"", "text/plain", "", 415}, {"", "application/json", "cross-site", 403}, {"http://example.com/extra", "application/json", "", 403}} {
		r := httptest.NewRequest("POST", "http://example.com/api/sessions", strings.NewReader(`{"name":"x","project":"chat"}`))
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Content-Type", tc.media)
		r.Header.Set("Sec-Fetch-Site", tc.fetch)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatal(tc, w.Code)
		}
	}
	if repo.calls != 0 {
		t.Fatal("invalid request reached storage")
	}
	r := httptest.NewRequest("POST", "http://example.com/api/sessions", strings.NewReader(`{"name":" baseline ","project":"chat"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 201 || repo.name != "baseline" || repo.project != "chat" {
		t.Fatal("valid create rejected", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("session write cached")
	}
	old := now.Add(-time.Hour)
	s.Status = func() collector.Status { return collector.Status{LastInventory: &old} }
	r = httptest.NewRequest("POST", "/api/sessions", strings.NewReader(`{"name":"x","standalone":true}`))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 409 || repo.calls != 1 {
		t.Fatal("started from stale inventory")
	}
}
func TestSessionReadLimitsAndErrors(t *testing.T) {
	repo := &sessionRepo{items: make([]store.Session, 101)}
	s := testServer(&fakeRepository{})
	s.Sessions = repo
	h := s.Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/sessions?standalone=true", nil))
	var list struct {
		Sessions []store.Session `json:"sessions"`
		More     bool            `json:"has_more"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list.Sessions) != 100 || !list.More || !repo.standalone {
		t.Fatal("list bounds", w.Body.String())
	}
	for _, path := range []string{"/api/sessions/bad", "/api/sessions?standalone=true&project=x", "/api/sessions?standalone=invalid"} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 400 {
			t.Fatal("invalid read", path, w.Code)
		}
	}
	for _, tc := range []struct {
		failure error
		code    int
	}{{store.ErrSessionNotFound, 404}, {errors.New("offline"), 503}} {
		repo.fail = tc.failure
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/sessions/"+strings.Repeat("a", 32), nil))
		if w.Code != tc.code {
			t.Fatal("error classification", w.Code)
		}
	}
	// A stale Docker inventory must not prevent finishing a stored session.
	repo.fail = nil
	r := httptest.NewRequest("POST", "/api/sessions/"+strings.Repeat("a", 32)+"/finish", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("finish depends on Docker", w.Code)
	}
}
