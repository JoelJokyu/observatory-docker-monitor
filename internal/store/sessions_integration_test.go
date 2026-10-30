package store

import (
	"context"
	"errors"
	"fmt"
	"observatory.local/platform/internal/model"
	"sync"
	"testing"
	"time"
)

func seedSessionProject(t *testing.T, ctx context.Context, s *Store, project string) model.Container {
	t.Helper()
	c := model.Container{ID: project + "-original", Name: "api", Image: "chat", Project: project, Service: "api", State: "running", SeenAt: time.Now().UTC()}
	if err := s.Inventory(ctx, []model.Container{c}); err != nil {
		t.Fatal(err)
	}
	return c
}
func TestSessionLifecycleMembershipAndBoundaries(t *testing.T) {
	ctx, s := integrationStore(t)
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	c := seedSessionProject(t, ctx, s, "chat")
	v, err := s.StartSession(ctx, "baseline", "chat", false, 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.StartSession(ctx, "duplicate", "chat", false, 10); !errors.Is(err, ErrSessionActive) {
		t.Fatal("duplicate active session accepted", err)
	}
	start := v.StartedAt.Add(-time.Minute).Truncate(time.Microsecond)
	if _, err = s.Pool.Exec(ctx, `UPDATE test_sessions SET started_at=$2 WHERE id=$1`, v.ID, start); err != nil {
		t.Fatal(err)
	}
	// A replacement is discovered during the same session; the old identity stays.
	c2 := c
	c2.ID = "replacement"
	c2.SeenAt = start.Add(20 * time.Second)
	if err = s.Inventory(ctx, []model.Container{c2}); err != nil {
		t.Fatal(err)
	}
	c2.Name = "api-final-name"
	c2.SeenAt = start.Add(25 * time.Second)
	if err = s.Inventory(ctx, []model.Container{c2}); err != nil {
		t.Fatal(err)
	}
	cpu := 40.0
	for _, sample := range []model.Sample{
		{ContainerID: c.ID, At: start.Add(-time.Second), Memory: 9999, CPU: &cpu},
		{ContainerID: c.ID, At: start, Memory: 100, CPU: nil},
		{ContainerID: c.ID, At: start.Add(10 * time.Second), Memory: 200, CPU: &cpu},
		{ContainerID: c2.ID, At: start.Add(30 * time.Second), Memory: 300, CPU: &cpu},
	} {
		if err = s.Save(ctx, sample); err != nil {
			t.Fatal(err)
		}
	}
	event := model.Event{ID: "short-event", ContainerID: "transient", Project: "chat", Name: "brief", Service: "worker", At: start.Add(40 * time.Second), Action: "die", ExitCode: "3"}
	if err = s.SaveEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	done, err := s.FinishSession(ctx, v.ID)
	if err != nil || done.FinishedAt == nil {
		t.Fatal("finish", done, err)
	}
	repeated, err := s.FinishSession(ctx, v.ID)
	if err != nil || !repeated.FinishedAt.Equal(*done.FinishedAt) {
		t.Fatal("finish retry changed boundary", err)
	}
	// Late replay with an in-session timestamp joins even after finish.
	late := event
	late.ID = "late"
	late.ContainerID = "late-container"
	late.Action = "create"
	late.ExitCode = ""
	late.At = start.Add(45 * time.Second)
	if err = s.SaveEvent(ctx, late); err != nil {
		t.Fatal(err)
	}
	outside := event
	outside.ID = "after"
	outside.ContainerID = "outside"
	outside.At = *done.FinishedAt
	if err = s.SaveEvent(ctx, outside); err != nil {
		t.Fatal(err)
	}
	unrelated := event
	unrelated.ID = "other-project"
	unrelated.ContainerID = "other"
	unrelated.Project = "other"
	if err = s.SaveEvent(ctx, unrelated); err != nil {
		t.Fatal(err)
	}
	if err = s.Save(ctx, model.Sample{ContainerID: c.ID, At: *done.FinishedAt, CPU: &cpu, Memory: 9999}); err != nil {
		t.Fatal(err)
	}
	summary, err := s.SessionSummary(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ContainerCount != 4 || summary.Samples != 3 || summary.EventCount != 2 || summary.NonzeroExits != 1 || summary.HistoryExpired {
		t.Fatalf("incorrect session evidence %+v", summary)
	}
	for _, m := range summary.Containers {
		if m.ID == c2.ID && m.Name != "api-final-name" {
			t.Fatal("replacement kept temporary create name", m.Name)
		}
		if m.ID == c.ID && (m.Samples != 2 || m.CPUCount != 1 || m.CPUMean == nil || *m.CPUMean != 40 || m.MemoryPeak == nil || *m.MemoryPeak != 200) {
			t.Fatalf("incorrect mean/bounds/peak %+v", m)
		}
	}
	// Reopening through a new adapter doesn't rely on an in-memory session map.
	reopened := &Store{Pool: s.Pool}
	list, err := reopened.Sessions(ctx, "chat", false, 101)
	if err != nil || len(list) != 1 || list[0].FinishedAt == nil {
		t.Fatal("session persistence", list, err)
	}
	next, err := s.StartSession(ctx, "second run", "chat", false, 10)
	if err != nil || next.ID == v.ID {
		t.Fatal("new completed-project session", err)
	}
	if _, err = s.FinishSession(ctx, "unknown"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("unknown finish", err)
	}
}
func TestSessionConcurrencyAndScope(t *testing.T) {
	ctx, s := integrationStore(t)
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	seedSessionProject(t, ctx, s, "parallel")
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := s.StartSession(ctx, fmt.Sprint("run", n), "parallel", false, 10)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrSessionActive) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatal("active-session race", success, conflicts)
	}
	if _, err := s.StartSession(ctx, "missing", "absent", false, 10); !errors.Is(err, ErrSessionScope) {
		t.Fatal("missing scope accepted", err)
	}
	seedSessionProject(t, ctx, s, "")
	solo, err := s.StartSession(ctx, "solo", "", true, 10)
	if err != nil || !solo.Standalone {
		t.Fatal("standalone start", err)
	}
	list, err := s.Sessions(ctx, "", true, 101)
	if err != nil || len(list) != 1 || list[0].ID != solo.ID {
		t.Fatal("standalone list scope", list, err)
	}
}
func TestSessionRetentionAndMissingValues(t *testing.T) {
	ctx, s := integrationStore(t)
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	seedSessionProject(t, ctx, s, "retention")
	v, err := s.StartSession(ctx, "long run", "retention", false, 10)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-8 * 24 * time.Hour)
	if _, err = s.Pool.Exec(ctx, `UPDATE test_sessions SET started_at=$2 WHERE id=$1`, v.ID, old); err != nil {
		t.Fatal(err)
	}
	summary, err := s.SessionSummary(ctx, v.ID)
	if err != nil || !summary.HistoryExpired || summary.Samples != 0 || len(summary.Containers) != 1 || summary.Containers[0].CPUMean != nil || summary.Containers[0].MemoryPeak != nil {
		t.Fatal("expired/unknown values", summary, err)
	}
	if err = s.Prune(ctx, time.Now().UTC().Add(-7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SessionSummary(ctx, v.ID); err != nil {
		t.Fatal("active session was expired", err)
	}
	if _, err = s.Pool.Exec(ctx, `UPDATE test_sessions SET finished_at=$2 WHERE id=$1`, v.ID, old.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	list, err := s.Sessions(ctx, "retention", false, 101)
	if err != nil || len(list) != 0 {
		t.Fatal("expired session listed", err)
	}
	if _, err = s.SessionSummary(ctx, v.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("expired session detail exposed", err)
	}
	if err = s.Prune(ctx, time.Now().UTC().Add(-7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.Pool.QueryRow(ctx, `SELECT count(*) FROM session_members WHERE session_id=$1`, v.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("session member retention", count, err)
	}
}
