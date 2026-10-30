package store

import (
	"observatory.local/platform/internal/model"
	"testing"
	"time"
)

func TestProjectsEventsLifecycle(t *testing.T) {
	ctx, s := integrationStore(t)
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	c := model.Container{ID: "container", Name: "api", Image: "chat", State: "running", SeenAt: now, Project: "chat", Service: "api"}
	if err := s.Inventory(ctx, []model.Container{c}); err != nil {
		t.Fatal(err)
	}
	items, err := s.Containers(ctx)
	if err != nil || len(items) != 1 || items[0].Project != "chat" || items[0].Service != "api" {
		t.Fatalf("project roundtrip: %+v %v", items, err)
	}
	for _, e := range []model.Event{
		{ID: "one", ContainerID: "container", Name: "api", Project: "chat", Service: "api", Action: "start", At: now.Add(-time.Second)},
		{ID: "two", ContainerID: "container", Name: "api", Project: "chat", Service: "api", Action: "die", ExitCode: "1", At: now},
		{ID: "three", ContainerID: "transient", Project: "only-events", Action: "destroy", At: now},
		{ID: "four", ContainerID: "solo", Action: "die", At: now},
		{ID: "old", ContainerID: "expired", Project: "expired", Action: "start", At: now.Add(-8 * 24 * time.Hour)},
	} {
		if err := s.SaveEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
		if err := s.SaveEvent(ctx, e); err != nil {
			t.Fatal("replay duplicate", err)
		}
	}
	events, err := s.Events(ctx, now.Add(-time.Minute), now.Add(time.Second), "chat", "", false, 10)
	if err != nil || len(events) != 2 || events[0].ID != "two" || events[0].ExitCode != "1" {
		t.Fatalf("scope/order/dedup: %+v %v", events, err)
	}
	events, err = s.Events(ctx, now.Add(-time.Minute), now, "chat", "", false, 10)
	if err != nil || len(events) != 1 {
		t.Fatal("half-open time range", events, err)
	}
	events, err = s.Events(ctx, now.Add(-time.Minute), now.Add(time.Second), "", "solo", true, 1)
	if err != nil || len(events) != 1 || events[0].ContainerID != "solo" {
		t.Fatal("standalone/id/limit filter", events, err)
	}
	latest, err := s.LatestEvent(ctx)
	if err != nil || latest == nil || !latest.Equal(now) {
		t.Fatal("latest cursor", latest, err)
	}
	if err := s.Inventory(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Prune(ctx, now.Add(-7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	projects, err := s.Projects(ctx)
	if err != nil || len(projects) != 2 || projects[0] != "chat" || projects[1] != "only-events" {
		t.Fatal("project discovery/retention", projects, err)
	}
	events, err = s.Events(ctx, now.Add(-9*24*time.Hour), now.Add(time.Second), "", "", false, 20)
	if err != nil || len(events) != 4 {
		t.Fatal("event retention/removal", events, err)
	}
}
