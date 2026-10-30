package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	containerlogs "observatory.local/platform/internal/logs"
)

type fakeLogReader struct {
	calls    int
	limit    int
	from, to time.Time
	err      error
	started  chan struct{}
	release  chan struct{}
}

func (f *fakeLogReader) Snapshot(ctx context.Context, id string, from, to time.Time, limit int) (containerlogs.Snapshot, error) {
	if f.started != nil {
		f.started <- struct{}{}
		select {
		case <-f.release:
		case <-ctx.Done():
			return containerlogs.Snapshot{}, ctx.Err()
		}
	} else {
		f.calls++
		f.limit = limit
		f.from = from
		f.to = to
	}
	return containerlogs.Snapshot{ContainerID: id, From: from, To: to, Limit: limit, Source: "docker", Entries: []containerlogs.Entry{}}, f.err
}
func TestLogQueryValidationAndWindow(t *testing.T) {
	source := &fakeLogReader{}
	server := testServer(&fakeRepository{})
	server.Logs = source
	handler := server.Handler()
	base := "/api/containers/" + strings.Repeat("a", 64) + "/logs"
	for _, suffix := range []string{"?limit=0", "?limit=1001", "?limit=no", "?from=no", "?from=2020-01-01T00:00:00Z", "?to=2020-01-01T00:00:00Z"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", base+suffix, nil))
		if w.Code != 400 {
			t.Fatalf("invalid query accepted %s: %d", suffix, w.Code)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/containers/not-a-docker-id/logs", nil))
	if w.Code != 400 || source.calls != 0 {
		t.Fatal("invalid queries reached Docker")
	}
	to := time.Now().UTC()
	from := to.Add(-time.Hour)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", base+"?from="+from.Format(time.RFC3339Nano)+"&to="+to.Format(time.RFC3339Nano)+"&limit=250", nil))
	if w.Code != 200 || source.calls != 1 || source.limit != 250 || !source.from.Equal(from) || !source.to.Equal(to) {
		t.Fatal("snapshot window changed", w.Code)
	}
	if w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), `"entries":[]`) {
		t.Fatal("response caching or empty-array contract failed")
	}
}
func TestLogFailureResponses(t *testing.T) {
	for _, test := range []struct {
		code   string
		err    error
		status int
	}{
		{"container_not_found", &containerlogs.Failure{Code: "container_not_found"}, 404},
		{"logs_unsupported", &containerlogs.Failure{Code: "logs_unsupported"}, 422},
		{"invalid_docker_response", &containerlogs.Failure{Code: "invalid_docker_response"}, 502},
		{"docker_unavailable", errors.New("opaque Docker failure"), 503},
		{"logs_timeout", context.DeadlineExceeded, 504},
	} {
		t.Run(test.code, func(t *testing.T) {
			server := testServer(&fakeRepository{})
			server.Logs = &fakeLogReader{err: test.err}
			w := httptest.NewRecorder()
			server.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/containers/"+strings.Repeat("a", 64)+"/logs", nil))
			if w.Code != test.status || !strings.Contains(w.Body.String(), test.code) {
				t.Fatalf("wrong failure response: %d %s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "opaque Docker failure") {
				t.Fatal("raw upstream error leaked")
			}
		})
	}
}
func TestConcurrentLogRequestsAreBounded(t *testing.T) {
	source := &fakeLogReader{started: make(chan struct{}, 4), release: make(chan struct{})}
	defer close(source.release)
	server := testServer(&fakeRepository{})
	server.Logs = source
	handler := server.Handler()
	path := "/api/containers/" + strings.Repeat("a", 64) + "/logs"
	done := make(chan struct{}, 4)
	for i := 0; i < 4; i++ {
		go func() {
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
			done <- struct{}{}
		}()
	}
	for i := 0; i < 4; i++ {
		select {
		case <-source.started:
		case <-time.After(2 * time.Second):
			t.Fatal("initial requests never started")
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	if w.Code != 503 || w.Header().Get("Retry-After") != "2" || !strings.Contains(w.Body.String(), "logs_busy") {
		t.Fatal("fifth snapshot was not bounded")
	}
	// Cleanup releases all readers; no test goroutine remains blocked.
	for i := 0; i < 4; i++ {
		source.release <- struct{}{}
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}
