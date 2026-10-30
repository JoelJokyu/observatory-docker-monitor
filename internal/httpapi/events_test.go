package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"observatory.local/platform/internal/model"
	"strings"
	"testing"
	"time"
)

type eventRepo struct {
	calls       int
	project, id string
	standalone  bool
	limit       int
	from, to    time.Time
}

func (f *eventRepo) Projects(context.Context) ([]string, error) { return []string{"chat"}, nil }
func (f *eventRepo) Events(_ context.Context, from, to time.Time, project, id string, standalone bool, limit int) ([]model.Event, error) {
	f.calls++
	f.from = from
	f.to = to
	f.project = project
	f.id = id
	f.standalone = standalone
	f.limit = limit
	return []model.Event{{ID: "1"}, {ID: "2"}, {ID: "3"}}, nil
}
func TestEventValidationAndBounds(t *testing.T) {
	repo := &eventRepo{}
	s := testServer(&fakeRepository{})
	s.Events = repo
	handler := s.Handler()
	for _, q := range []string{"from=wrong", "limit=0", "limit=501", "container_id=short", "standalone=maybe", "standalone=true&project=chat", "from=2020-01-01T00:00:00Z", "project=" + strings.Repeat("a", 513)} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/events?"+q, nil))
		if w.Code != 400 {
			t.Fatal(q, w.Code, w.Body.String())
		}
	}
	if repo.calls != 0 {
		t.Fatal("invalid queries reached storage")
	}
	id := strings.Repeat("a", 64)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/events?project=chat&container_id="+id+"&limit=2", nil))
	var body struct {
		Events   []model.Event `json:"events"`
		HasMore  bool          `json:"has_more"`
		Coverage string        `json:"coverage_notice"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != 200 || len(body.Events) != 2 || !body.HasMore || body.Coverage == "" {
		t.Fatal("missing bounds/coverage", w.Code, w.Body.String())
	}
	if repo.project != "chat" || repo.id != id || repo.limit != 3 || repo.to.Sub(repo.from) != time.Hour {
		t.Fatalf("incorrect filters %+v", repo)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("events cached")
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/events?standalone=true", nil))
	if !repo.standalone || repo.project != "" {
		t.Fatal("standalone scope lost")
	}
}
