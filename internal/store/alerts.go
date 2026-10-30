package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"observatory.local/platform/internal/alerts"
	"observatory.local/platform/internal/model"
)

var ErrAlertNotFound = errors.New("alert not found or expired")

const ruleColumns = `project,enabled,cpu_enabled,memory_enabled,cpu_threshold,memory_threshold,nonzero_exit,oom,unhealthy,enabled_since`

func readRule(row pgx.Row) (alerts.Rule, error) {
	var r alerts.Rule
	err := row.Scan(&r.Project, &r.Enabled, &r.CPUEnabled, &r.MemoryEnabled, &r.CPUThreshold, &r.MemoryThreshold, &r.NonzeroExit, &r.OOM, &r.Unhealthy, &r.EnabledSince)
	return r, err
}
func (s *Store) AlertRule(ctx context.Context, project string) (alerts.Rule, error) {
	r, err := readRule(s.Pool.QueryRow(ctx, `SELECT `+ruleColumns+` FROM alert_rules WHERE project=$1`, project))
	if errors.Is(err, pgx.ErrNoRows) {
		return alerts.DefaultRule(project), nil
	}
	return r, err
}
func (s *Store) SaveAlertRule(ctx context.Context, r alerts.Rule) (alerts.Rule, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return r, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(841304)`); err != nil {
		return r, err
	}
	// Reconfiguration retires old evidence without claiming the workload recovered.
	if _, err = tx.Exec(ctx, `UPDATE local_alerts SET resolved_at=clock_timestamp(),resolution='rule_changed' WHERE project=$1 AND resolved_at IS NULL AND kind IN ('cpu','memory','unhealthy')`, r.Project); err != nil {
		return r, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM alert_conditions WHERE project=$1`, r.Project); err != nil {
		return r, err
	}
	r, err = readRule(tx.QueryRow(ctx, `INSERT INTO alert_rules(project,enabled,cpu_enabled,memory_enabled,cpu_threshold,memory_threshold,nonzero_exit,oom,unhealthy)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(project) DO UPDATE SET enabled=excluded.enabled,cpu_enabled=excluded.cpu_enabled,memory_enabled=excluded.memory_enabled,cpu_threshold=excluded.cpu_threshold,memory_threshold=excluded.memory_threshold,nonzero_exit=excluded.nonzero_exit,oom=excluded.oom,unhealthy=excluded.unhealthy,enabled_since=clock_timestamp() RETURNING `+ruleColumns, r.Project, r.Enabled, r.CPUEnabled, r.MemoryEnabled, r.CPUThreshold, r.MemoryThreshold, r.NonzeroExit, r.OOM, r.Unhealthy))
	if err != nil {
		return r, err
	}
	return r, tx.Commit(ctx)
}
func newAlertID() (string, error) {
	var b [16]byte
	_, err := rand.Read(b[:])
	return hex.EncodeToString(b[:]), err
}
func insertAlert(ctx context.Context, tx pgx.Tx, a alerts.Alert, source *string) error {
	_, err := tx.Exec(ctx, `INSERT INTO local_alerts(id,project,container_id,name,service,kind,evidence_from,at,last_observed_at,value,threshold,detail,source_event_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8,$9,$10,$11,$12) ON CONFLICT DO NOTHING`, a.ID, a.Project, a.ContainerID, a.Name, a.Service, a.Kind, a.EvidenceFrom, a.At, a.Value, a.Threshold, a.Detail, source)
	return err
}
func conditionObservation(ctx context.Context, tx pgx.Tx, project, id, name, service, kind string, at time.Time, value *float64, threshold, duration float64, maxGap time.Duration) error {
	var state alerts.Condition
	var raw []byte
	var alertID *string
	err := tx.QueryRow(ctx, `SELECT state,alert_id FROM alert_conditions WHERE project=$1 AND container_id=$2 AND kind=$3`, project, id, kind).Scan(&raw, &alertID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &state); err != nil {
			return err
		}
	}
	if state.Last != nil && !at.After(*state.Last) {
		return nil
	}
	high := value != nil && *value >= threshold
	transition := state.Observe(at, high, value != nil, time.Duration(duration)*time.Second, maxGap)
	if transition.Recovered && alertID != nil {
		if _, err = tx.Exec(ctx, `UPDATE local_alerts SET resolved_at=$2,resolution='recovered' WHERE id=$1 AND resolved_at IS NULL`, *alertID, at); err != nil {
			return err
		}
		alertID = nil
	}
	if transition.Fired {
		generated, err := newAlertID()
		if err != nil {
			return err
		}
		alertID = &generated
		since := at
		if state.Since != nil {
			since = *state.Since
		}
		detail := "Recorded CPU readings met the threshold over at least 30 seconds (100% is one core)."
		if kind == "memory" {
			detail = "Recorded memory readings met the threshold over at least 30 seconds, as a percentage of Docker's reported limit. That limit may be inherited from the Engine."
		}
		if kind == "unhealthy" {
			detail = "Docker reported an unhealthy health-check state."
		}
		if err = insertAlert(ctx, tx, alerts.Alert{ID: generated, Project: project, ContainerID: id, Name: name, Service: service, Kind: kind, EvidenceFrom: since, At: at, Value: value, Threshold: &threshold, Detail: detail}, nil); err != nil {
			return err
		}
	}
	if state.Active && value != nil && high && alertID != nil {
		if _, err = tx.Exec(ctx, `UPDATE local_alerts SET last_observed_at=$2,value=$3 WHERE id=$1`, *alertID, at, *value); err != nil {
			return err
		}
	}
	raw, err = json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO alert_conditions(project,container_id,kind,state,alert_id,updated_at) VALUES($1,$2,$3,$4,$5,now()) ON CONFLICT(project,container_id,kind) DO UPDATE SET state=excluded.state,alert_id=excluded.alert_id,updated_at=now()`, project, id, kind, raw, alertID)
	return err
}
func (s *Store) evaluateSample(ctx context.Context, tx pgx.Tx, v model.Sample) error {
	// Rule, state and notification are serialized and committed with the sample.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(841304)`); err != nil {
		return err
	}
	var project, name, service string
	if err := tx.QueryRow(ctx, `SELECT project,name,service FROM containers WHERE id=$1`, v.ContainerID).Scan(&project, &name, &service); err != nil {
		return err
	}
	rule, err := readRule(tx.QueryRow(ctx, `SELECT `+ruleColumns+` FROM alert_rules WHERE project=$1`, project))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !rule.Enabled || v.At.Before(rule.EnabledSince) {
		return nil
	}
	// More than 1.5 expected intervals is a missing-observation gap. The caller's
	// configured interval is supplied through the Store for live collection.
	interval := s.SampleInterval
	if interval == 0 {
		interval = 10 * time.Second
	}
	gap := interval * 3 / 2
	if rule.CPUEnabled {
		if err = conditionObservation(ctx, tx, project, v.ContainerID, name, service, "cpu", v.At, v.CPU, rule.CPUThreshold, 30, gap); err != nil {
			return err
		}
	}
	if rule.MemoryEnabled {
		var percent *float64
		if v.MemoryLimit > 0 {
			n := float64(v.Memory) / float64(v.MemoryLimit) * 100
			percent = &n
		}
		if err = conditionObservation(ctx, tx, project, v.ContainerID, name, service, "memory", v.At, percent, rule.MemoryThreshold, 30, gap); err != nil {
			return err
		}
	}
	return nil
}
func evaluateEvent(ctx context.Context, tx pgx.Tx, e model.Event) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(841304)`); err != nil {
		return err
	}
	r, err := readRule(tx.QueryRow(ctx, `SELECT `+ruleColumns+` FROM alert_rules WHERE project=$1`, e.Project))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !r.Enabled || e.At.Before(r.EnabledSince) {
		return nil
	}
	kind, detail := "", ""
	if e.Action == "die" && r.NonzeroExit && e.ExitCode != "" && e.ExitCode != "0" {
		kind = "nonzero_exit"
		detail = fmt.Sprintf("Docker reported exit code %s. An intentional stop can also produce a nonzero exit.", e.ExitCode)
	}
	if e.Action == "oom" && r.OOM {
		kind = "oom"
		detail = "Docker reported an out-of-memory event."
	}
	if kind != "" {
		id, err := newAlertID()
		if err != nil {
			return err
		}
		if err = insertAlert(ctx, tx, alerts.Alert{ID: id, Project: e.Project, ContainerID: e.ContainerID, Name: e.Name, Service: e.Service, Kind: kind, EvidenceFrom: e.At, At: e.At, Detail: detail}, &e.ID); err != nil {
			return err
		}
	}
	if r.Unhealthy {
		switch e.Action {
		case "health_status: unhealthy", "health_status: healthy":
			value := 0.0
			if e.Action == "health_status: unhealthy" {
				value = 1
			}
			return conditionObservation(ctx, tx, e.Project, e.ContainerID, e.Name, e.Service, "unhealthy", e.At, &value, 1, 0, time.Hour)
		case "start", "die", "destroy":
			var raw []byte
			var id *string
			err := tx.QueryRow(ctx, `SELECT state,alert_id FROM alert_conditions WHERE project=$1 AND container_id=$2 AND kind='unhealthy'`, e.Project, e.ContainerID).Scan(&raw, &id)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			var state alerts.Condition
			if err = json.Unmarshal(raw, &state); err != nil {
				return err
			}
			if state.Last != nil && !e.At.After(*state.Last) {
				return nil
			}
			if id != nil {
				if _, err = tx.Exec(ctx, `UPDATE local_alerts SET resolved_at=$2,resolution='container_lifecycle_changed' WHERE id=$1 AND resolved_at IS NULL`, *id, e.At); err != nil {
					return err
				}
			}
			// Retain the last event time to reject older replay after lifecycle changes.
			state = alerts.Condition{Last: &e.At}
			raw, err = json.Marshal(state)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE alert_conditions SET state=$3,alert_id=NULL,updated_at=now() WHERE project=$1 AND container_id=$2 AND kind='unhealthy'`, e.Project, e.ContainerID, raw)
			return err
		}
	}
	return nil
}
func (s *Store) Alerts(ctx context.Context, project string, standalone, includeDismissed bool) ([]alerts.Alert, int64, error) {
	var unread int64
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM local_alerts WHERE last_observed_at>=now()-interval '7 days' AND dismissed_at IS NULL AND ($1='' OR project=$1) AND (NOT $2 OR project='')`, project, standalone).Scan(&unread); err != nil {
		return nil, 0, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT id,project,container_id,name,service,kind,evidence_from,at,last_observed_at,value,threshold,detail,resolved_at,resolution,dismissed_at FROM local_alerts
 WHERE last_observed_at>=now()-interval '7 days' AND ($1='' OR project=$1) AND (NOT $2 OR project='') AND ($3 OR dismissed_at IS NULL) ORDER BY last_observed_at DESC,id DESC LIMIT 101`, project, standalone, includeDismissed)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]alerts.Alert, 0)
	for rows.Next() {
		var a alerts.Alert
		if err := rows.Scan(&a.ID, &a.Project, &a.ContainerID, &a.Name, &a.Service, &a.Kind, &a.EvidenceFrom, &a.At, &a.LastObserved, &a.Value, &a.Threshold, &a.Detail, &a.ResolvedAt, &a.Resolution, &a.DismissedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, unread, rows.Err()
}
func (s *Store) DismissAlert(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE local_alerts SET dismissed_at=COALESCE(dismissed_at,clock_timestamp()) WHERE id=$1 AND last_observed_at>=now()-interval '7 days'`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrAlertNotFound
	}
	return err
}
