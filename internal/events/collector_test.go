package events

import (
	"context"
	"errors"
	dockerevents "github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
	"io"
	"log/slog"
	"observatory.local/platform/internal/model"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixture() dockerevents.Message {
	return dockerevents.Message{Type: dockerevents.ContainerEventType, Action: dockerevents.ActionDie, TimeNano: time.Now().UnixNano(), Actor: dockerevents.Actor{ID: strings.Repeat("a", 64), Attributes: map[string]string{"name": "chat", "exitCode": "137", "com.docker.compose.project": "chat-project", "com.docker.compose.service": "api", "secret": "must not persist"}}}
}
func TestNormalize(t *testing.T) {
	m := fixture()
	e, ok := Normalize(m)
	if !ok || e.Project != "chat-project" || e.Service != "api" || e.ExitCode != "137" || e.At.UnixNano() != m.TimeNano {
		t.Fatalf("lost event metadata: %+v", e)
	}
	duplicate, _ := Normalize(m)
	if e.ID != duplicate.ID {
		t.Fatal("unstable deduplication identity")
	}
	m.TimeNano++
	next, _ := Normalize(m)
	if e.ID == next.ID {
		t.Fatal("distinct nanosecond events collided")
	}
	m = fixture()
	m.Actor.Attributes["secret"] = "changed"
	same, _ := Normalize(m)
	m.Actor.Attributes["secret"] = "other"
	other, _ := Normalize(m)
	if same.ID != other.ID {
		t.Fatal("arbitrary labels affect retained event")
	}
	for _, action := range []dockerevents.Action{dockerevents.ActionExecCreate, "health_status: secret output", "unknown"} {
		m.Action = action
		if _, ok := Normalize(m); ok {
			t.Fatalf("accepted unwanted action %s", action)
		}
	}
	m = fixture()
	m.Actor.ID = "not-a-container"
	if _, ok := Normalize(m); ok {
		t.Fatal("invalid identity")
	}
	m = fixture()
	m.TimeNano = time.Now().Add(-8 * 24 * time.Hour).UnixNano()
	if _, ok := Normalize(m); ok {
		t.Fatal("accepted expired event")
	}
	m = fixture()
	m.TimeNano = 0
	m.Time = time.Now().Unix()
	if _, ok := Normalize(m); !ok {
		t.Fatal("seconds-only timestamp rejected")
	}
}

type fakeStore struct {
	mu     sync.Mutex
	saved  []model.Event
	fail   bool
	latest *time.Time
	writes chan struct{}
}

func (s *fakeStore) LatestEvent(context.Context) (*time.Time, error) { return s.latest, nil }
func (s *fakeStore) SaveEvent(_ context.Context, e model.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case s.writes <- struct{}{}:
	default:
	}
	if s.fail {
		return errors.New("offline")
	}
	s.saved = append(s.saved, e)
	return nil
}

type fakeEngine struct {
	options   chan client.EventsListOptions
	message   dockerevents.Message
	cancelled chan struct{}
}

func (e *fakeEngine) Events(ctx context.Context, o client.EventsListOptions) client.EventsResult {
	e.options <- o
	messages := make(chan dockerevents.Message)
	errs := make(chan error, 1)
	go func() {
		select {
		case messages <- e.message:
		case <-ctx.Done():
		}
		<-ctx.Done()
		errs <- ctx.Err()
		e.cancelled <- struct{}{}
	}()
	return client.EventsResult{Messages: messages, Err: errs}
}
func TestCollectorReplaysDurableCursorAndCancels(t *testing.T) {
	latest := time.Now().Add(-time.Hour).UTC()
	storage := &fakeStore{latest: &latest, writes: make(chan struct{}, 1)}
	engine := &fakeEngine{options: make(chan client.EventsListOptions, 4), message: fixture(), cancelled: make(chan struct{}, 4)}
	c := &Collector{Engine: engine, Store: storage, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); c.Run(ctx) }()
	select {
	case opts := <-engine.options:
		since, err := time.Parse(time.RFC3339Nano, opts.Since)
		if err != nil || !since.Equal(latest.Add(-time.Second)) || !opts.Filters["type"]["container"] {
			t.Fatalf("incorrect replay options %+v", opts)
		}
	case <-time.After(time.Second):
		t.Fatal("no subscription")
	}
	select {
	case <-storage.writes:
	case <-time.After(time.Second):
		t.Fatal("no persisted event")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("collector failed to cancel")
	}
	select {
	case <-engine.cancelled:
	case <-time.After(time.Second):
		t.Fatal("stream context leaked")
	}
	storage.mu.Lock()
	defer storage.mu.Unlock()
	if len(storage.saved) != 1 || storage.saved[0].Action != "die" {
		t.Fatal("event not persisted")
	}
}
func TestFailedWriteDoesNotAdvanceCursor(t *testing.T) {
	latest := time.Now().Add(-time.Hour).UTC()
	storage := &fakeStore{latest: &latest, fail: true, writes: make(chan struct{}, 4)}
	engine := &fakeEngine{options: make(chan client.EventsListOptions, 4), message: fixture(), cancelled: make(chan struct{}, 4)}
	c := &Collector{Engine: engine, Store: storage, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); c.Run(ctx) }()
	var first, second client.EventsListOptions
	select {
	case first = <-engine.options:
	case <-time.After(time.Second):
		t.Fatal("no initial subscription")
	}
	select {
	case second = <-engine.options:
	case <-time.After(4 * time.Second):
		t.Fatal("no retry")
	}
	if first.Since != second.Since {
		t.Fatal("failed event was skipped")
	}
	if !c.Status().Interrupted {
		t.Fatal("missing coverage interruption")
	}
	cancel()
	<-done
}

type recoveringEngine struct {
	calls int
	good  *fakeEngine
}

func (e *recoveringEngine) Events(ctx context.Context, o client.EventsListOptions) client.EventsResult {
	e.calls++
	if e.calls == 1 {
		errs := make(chan error, 1)
		errs <- errors.New("Docker disconnected")
		return client.EventsResult{Messages: make(chan dockerevents.Message), Err: errs}
	}
	return e.good.Events(ctx, o)
}
func TestDockerHandshakeFailureRecoversWithCoverageWarning(t *testing.T) {
	storage := &fakeStore{writes: make(chan struct{}, 1)}
	good := &fakeEngine{options: make(chan client.EventsListOptions, 4), message: fixture(), cancelled: make(chan struct{}, 4)}
	c := &Collector{Engine: &recoveringEngine{good: good}, Store: storage, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); c.Run(ctx) }()
	select {
	case <-storage.writes:
	case <-time.After(4 * time.Second):
		t.Fatal("Docker subscription did not recover")
	}
	status := c.Status()
	if !status.Connected || !status.Interrupted || status.LastConnected == nil {
		t.Fatalf("missing recovered status: %+v", status)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("recovered stream failed to stop")
	}
}
