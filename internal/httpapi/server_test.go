package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"observatory.local/platform/internal/collector"
	"observatory.local/platform/internal/model"
	"observatory.local/platform/internal/store"
)

type fakeRepository struct {
	calls  int
	bucket int
}

func (f *fakeRepository) Containers(context.Context) ([]model.Container, error) {
	return []model.Container{}, nil
}
func (f *fakeRepository) History(_ context.Context, _ string, _, _ time.Time, bucket int) ([]store.Bucket, error) {
	f.calls++
	f.bucket = bucket
	return []store.Bucket{}, nil
}
func testServer(repo Repository) *Server {
	return &Server{Store: repo, Interval: 10 * time.Second, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Status: func() collector.Status { return collector.Status{} }, Ping: func(context.Context) error { return nil }}
}
func TestHistoryValidationAndResolution(t *testing.T) {
	repo := &fakeRepository{}
	handler := testServer(repo).Handler()
	for _, query := range []string{"from=invalid", "from=2020-01-01T00:00:00Z", "from=2026-01-02T00:00:00Z&to=2026-01-01T00:00:00Z"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/containers/id/history?"+query, nil))
		if w.Code != 400 {
			t.Fatalf("accepted invalid range %s: %d", query, w.Code)
		}
	}
	if repo.calls != 0 {
		t.Fatal("invalid queries reached storage")
	}
	now := time.Now().UTC()
	from := now.Add(-7 * 24 * time.Hour)
	path := "/api/containers/id/history?from=" + from.Format(time.RFC3339) + "&to=" + now.Format(time.RFC3339)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	if w.Code != 200 || repo.bucket < 1008 {
		t.Fatalf("unexpected history response %d bucket %d", w.Code, repo.bucket)
	}
}
func TestHealthDoesNotClaimHealthyBeforeCollection(t *testing.T) {
	s := testServer(&fakeRepository{})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/health", nil))
	if w.Code != 503 {
		t.Fatal("no collection must not be healthy")
	}
	now := time.Now()
	s.Status = func() collector.Status { return collector.Status{Healthy: true, LastInventory: &now} }
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/health", nil))
	if w.Code != 200 {
		t.Fatal("fresh collector with healthy database should pass")
	}
	old := now.Add(-time.Hour)
	s.Status = func() collector.Status { return collector.Status{Healthy: true, LastInventory: &old} }
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/health", nil))
	if w.Code != 503 {
		t.Fatal("stale collector must be degraded")
	}
}
func TestEmptyContainersReturnsArray(t *testing.T) {
	w := httptest.NewRecorder()
	testServer(&fakeRepository{}).Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/containers", nil))
	var body struct {
		Containers []model.Container `json:"containers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Containers == nil {
		t.Fatalf("expected empty array: %s", w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("telemetry must not be cached")
	}
}

func TestEmbeddedReactAssets(t *testing.T) {
	handler := testServer(&fakeRepository{}).Handler()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `id="root"`) {
		t.Fatal("React entry point missing")
	}
	scripts := regexp.MustCompile(`src="(/assets/[^"]+\.js)"`).FindStringSubmatch(w.Body.String())
	if len(scripts) != 2 {
		t.Fatal("bundled React script not referenced")
	}
	for _, route := range []string{scripts[1], "/theme.js"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", route, nil))
		if response.Code != 200 || response.Body.Len() == 0 {
			t.Fatalf("asset not served: %s", route)
		}
	}
}
