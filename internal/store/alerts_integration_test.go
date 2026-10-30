package store

import (
	"errors"
	"observatory.local/platform/internal/alerts"
	"observatory.local/platform/internal/model"
	"testing"
	"time"
)

func TestAlertThresholdEpisodesAndDismissal(t *testing.T) {
	ctx, s := integrationStore(t)
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	c := seedSessionProject(t, ctx, s, "alert-test")
	r, err := s.AlertRule(ctx, c.Project)
	if err != nil || r.Enabled {
		t.Fatal("project did not default off", r, err)
	}
	r.Enabled = true
	r.MemoryEnabled = false
	r, err = s.SaveAlertRule(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Add(-2 * time.Minute)
	if _, err = s.Pool.Exec(ctx, `UPDATE alert_rules SET enabled_since=$2 WHERE project=$1`, r.Project, start.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	save := func(seconds int, cpu *float64) {
		t.Helper()
		if err := s.Save(ctx, model.Sample{ContainerID: c.ID, At: start.Add(time.Duration(seconds) * time.Second), CPU: cpu, Memory: 100, MemoryLimit: 1000}); err != nil {
			t.Fatal(err)
		}
	}
	high, low := 90.0, 20.0
	for i := 0; i <= 3; i++ {
		save(i*10, &high)
	}
	rows, count, err := s.Alerts(ctx, c.Project, false, false)
	if err != nil || len(rows) != 1 || count != 1 || rows[0].Kind != "cpu" {
		t.Fatal("sustained alert missing", rows, count, err)
	}
	first := rows[0].ID
	if err = s.DismissAlert(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err = s.DismissAlert(ctx, first); err != nil {
		t.Fatal("dismiss not retry-safe", err)
	}
	// Reopening the adapter has no in-memory suppression or pending state dependency.
	s = &Store{Pool: s.Pool}
	save(40, &high)
	save(40, &high)
	rows, count, err = s.Alerts(ctx, c.Project, false, false)
	if err != nil || len(rows) != 0 || count != 0 {
		t.Fatal("dismissed condition alerted repeatedly", rows, count, err)
	}
	save(50, nil)
	rows, _, err = s.Alerts(ctx, c.Project, false, true)
	if err != nil || rows[0].ResolvedAt != nil {
		t.Fatal("unknown resolved active episode", rows, err)
	}
	save(60, &low)
	rows, _, err = s.Alerts(ctx, c.Project, false, true)
	if err != nil || rows[0].Resolution != "recovered" {
		t.Fatal("known recovery not recorded", rows, err)
	}
	for _, n := range []int{70, 80, 90, 100} {
		save(n, &high)
	}
	rows, count, err = s.Alerts(ctx, c.Project, false, true)
	if err != nil || len(rows) != 2 || count != 1 || rows[0].ID == first {
		t.Fatal("new breach did not create distinct episode", rows, count, err)
	}
	// Settings changes retire evidence rather than invent a workload recovery.
	r.Enabled = false
	if _, err = s.SaveAlertRule(ctx, r); err != nil {
		t.Fatal(err)
	}
	rows, _, err = s.Alerts(ctx, c.Project, false, true)
	if err != nil || rows[0].Resolution != "rule_changed" {
		t.Fatal("configuration retirement", rows, err)
	}
	save(110, &high)
	rows, _, err = s.Alerts(ctx, c.Project, false, true)
	if err != nil || len(rows) != 2 {
		t.Fatal("disabled rule fired", rows, err)
	}
	if err = s.DismissAlert(ctx, "missing"); !errors.Is(err, ErrAlertNotFound) {
		t.Fatal("unknown dismiss", err)
	}
}
func TestAlertEventReplayRecoveryAndOptIn(t *testing.T) {
	ctx, s := integrationStore(t)
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	r := alerts.DefaultRule("events")
	r.Enabled = true
	r.CPUEnabled = false
	r.MemoryEnabled = false
	rule, err := s.SaveAlertRule(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	base := rule.EnabledSince.Add(time.Millisecond)
	event := func(id, action, code, project string, seconds int) {
		t.Helper()
		if err := s.SaveEvent(ctx, model.Event{ID: id, ContainerID: "event-only", Name: "brief", Project: project, Action: action, ExitCode: code, At: base.Add(time.Duration(seconds) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	event("old", "die", "3", "events", -10)
	event("other", "die", "3", "not-opted-in", 0)
	event("exit", "die", "3", "events", 1)
	event("exit", "die", "3", "events", 1)
	event("normal", "die", "0", "events", 2)
	event("oom", "oom", "", "events", 3)
	event("bad", "health_status: unhealthy", "", "events", 4)
	event("bad-again", "health_status: unhealthy", "", "events", 5)
	rows, count, err := s.Alerts(ctx, "events", false, false)
	if err != nil || len(rows) != 3 || count != 3 {
		t.Fatal("event opt-in/replay/health grouping", rows, count, err)
	}
	var healthID string
	for _, a := range rows {
		if a.Kind == "unhealthy" {
			healthID = a.ID
		}
	}
	if healthID == "" {
		t.Fatal("health episode missing")
	}
	if err = s.DismissAlert(ctx, healthID); err != nil {
		t.Fatal(err)
	}
	event("healthy", "health_status: healthy", "", "events", 6)
	event("bad-new", "health_status: unhealthy", "", "events", 7)
	// Out-of-order replay cannot recover a newer health episode.
	event("old-healthy", "health_status: healthy", "", "events", 6)
	rows, _, err = s.Alerts(ctx, "events", false, true)
	if err != nil || len(rows) != 4 {
		t.Fatal("new health episode missing", rows, err)
	}
	for _, a := range rows {
		if a.Kind == "unhealthy" && a.ID != healthID && a.ResolvedAt != nil {
			t.Fatal("old replay recovered newer state")
		}
	}
	event("ended", "die", "0", "events", 8)
	rows, _, err = s.Alerts(ctx, "events", false, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range rows {
		if a.Kind == "unhealthy" && a.ID != healthID && a.Resolution != "container_lifecycle_changed" {
			t.Fatal("lifecycle changed marked healthy", a)
		}
	}
	solo := alerts.DefaultRule("")
	solo.Enabled = true
	if _, err = s.SaveAlertRule(ctx, solo); err != nil {
		t.Fatal(err)
	}
	event("solo", "die", "2", "", 9)
	rows, _, err = s.Alerts(ctx, "", true, false)
	if err != nil || len(rows) != 1 || rows[0].Project != "" {
		t.Fatal("standalone scoping", rows, err)
	}
}
func TestAlertGapsMemoryAndRetention(t *testing.T) {
	ctx, s := integrationStore(t)
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	c := seedSessionProject(t, ctx, s, "memory")
	r := alerts.DefaultRule(c.Project)
	r.Enabled = true
	r.CPUEnabled = false
	if _, err := s.SaveAlertRule(ctx, r); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(-time.Minute)
	if _, err := s.Pool.Exec(ctx, `UPDATE alert_rules SET enabled_since=$2 WHERE project=$1`, r.Project, base.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	save := func(n int, limit int64) {
		t.Helper()
		if err := s.Save(ctx, model.Sample{ContainerID: c.ID, At: base.Add(time.Duration(n) * time.Second), Memory: 900, MemoryLimit: limit}); err != nil {
			t.Fatal(err)
		}
	}
	save(0, 1000)
	save(10, 1000)
	save(30, 1000)
	save(40, 0)
	save(50, 1000)
	save(60, 1000)
	rows, _, err := s.Alerts(ctx, c.Project, false, false)
	if err != nil || len(rows) != 0 {
		t.Fatal("gap/unknown counted as sustained evidence", rows, err)
	}
	save(70, 1000)
	save(80, 1000)
	rows, _, err = s.Alerts(ctx, c.Project, false, false)
	if err != nil || len(rows) != 1 || rows[0].Kind != "memory" || rows[0].Value == nil || *rows[0].Value != 90 {
		t.Fatal("memory percent", rows, err)
	}
	old := time.Now().UTC().Add(-8 * 24 * time.Hour)
	if _, err = s.Pool.Exec(ctx, `UPDATE local_alerts SET last_observed_at=$1`, old); err != nil {
		t.Fatal(err)
	}
	rows, _, err = s.Alerts(ctx, "", false, true)
	if err != nil || len(rows) != 0 {
		t.Fatal("expired alerts exposed", rows, err)
	}
	if _, err = s.Pool.Exec(ctx, `UPDATE alert_conditions SET updated_at=$1`, old); err != nil {
		t.Fatal(err)
	}
	if err = s.Prune(ctx, time.Now().UTC().Add(-7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.Pool.QueryRow(ctx, `SELECT count(*) FROM local_alerts`).Scan(&count); err != nil || count != 0 {
		t.Fatal("alert retention", count, err)
	}
}
