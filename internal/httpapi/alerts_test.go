package httpapi

import (
	"context"
	"net/http/httptest"
	"observatory.local/platform/internal/alerts"
	"strings"
	"testing"
)

type fakeAlerts struct {
	calls   int
	rule    alerts.Rule
	project string
	solo    bool
}

func (f *fakeAlerts) AlertRule(_ context.Context, p string) (alerts.Rule, error) {
	f.calls++
	return alerts.DefaultRule(p), nil
}
func (f *fakeAlerts) SaveAlertRule(_ context.Context, r alerts.Rule) (alerts.Rule, error) {
	f.calls++
	f.rule = r
	return r, nil
}
func (f *fakeAlerts) Alerts(_ context.Context, p string, solo, include bool) ([]alerts.Alert, int64, error) {
	f.calls++
	f.project = p
	f.solo = solo
	return make([]alerts.Alert, 101), 4, nil
}
func (f *fakeAlerts) DismissAlert(context.Context, string) error { f.calls++; return nil }
func TestAlertRoutesValidateScopeAndWrites(t *testing.T) {
	repo := &fakeAlerts{}
	s := testServer(&fakeRepository{})
	s.Alerts = repo
	h := s.Handler()
	for _, path := range []string{"/api/alert-rules", "/api/alerts?standalone=true&project=x", "/api/alerts?include_dismissed=maybe"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 400 {
			t.Fatal(path, w.Code)
		}
	}
	for _, body := range []string{`{}`, `{"enabled":true,"cpu_threshold":0,"memory_threshold":80}`, `{"enabled":true,"cpu_threshold":80,"memory_threshold":101}`, `{"enabled":true,"cpu_threshold":80,"memory_threshold":80,"extra":true}`} {
		r := httptest.NewRequest("PUT", "/api/alert-rules?project=chat", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal(body, w.Code, w.Body.String())
		}
	}
	if repo.calls != 0 {
		t.Fatal("invalid alert requests reached database")
	}
	body := `{"enabled":true,"cpu_enabled":true,"memory_enabled":true,"cpu_threshold":80,"memory_threshold":80,"nonzero_exit":true,"oom":true,"unhealthy":true}`
	r := httptest.NewRequest("PUT", "http://example.com/api/alert-rules?project=chat", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://evil.test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-site write accepted")
	}
	r = httptest.NewRequest("PUT", "/api/alert-rules?project=chat", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || !repo.rule.Enabled || repo.rule.Project != "chat" {
		t.Fatal("valid project rule rejected", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/alerts?standalone=true", nil))
	if w.Code != 200 || !repo.solo || !strings.Contains(w.Body.String(), `"has_more":true`) || !strings.Contains(w.Body.String(), `"needs_review":4`) {
		t.Fatal("inbox scopes/bounds", w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("alerts cached")
	}
	r = httptest.NewRequest("POST", "/api/alerts/not-an-id/dismiss", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("invalid dismiss ID accepted")
	}
}
