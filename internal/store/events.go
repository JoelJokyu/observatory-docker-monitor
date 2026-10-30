package store

import (
	"context"
	"observatory.local/platform/internal/model"
	"time"
)

func (s *Store) SaveEvent(ctx context.Context, e model.Event) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(841303)`); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO container_events(id,container_id,name,project,service,action,at,exit_code,signal)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`, e.ID, e.ContainerID, e.Name, e.Project, e.Service, e.Action, e.At, e.ExitCode, e.Signal)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO session_members(session_id,container_id,name,service,first_observed_at)
 SELECT id,$1,$2,$3,$4 FROM test_sessions WHERE project=$5 AND started_at<=$4 AND (finished_at IS NULL OR finished_at>$4)
 AND (finished_at IS NULL OR finished_at>=now()-interval '7 days') ON CONFLICT DO NOTHING`, e.ContainerID, e.Name, e.Service, e.At, e.Project)
	if err != nil {
		return err
	}
	if err = evaluateEvent(ctx, tx, e); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) LatestEvent(ctx context.Context) (*time.Time, error) {
	var at *time.Time
	err := s.Pool.QueryRow(ctx, `SELECT max(at) FROM container_events`).Scan(&at)
	return at, err
}

// Empty project means all projects; standalone is an explicit boolean to avoid
// colliding with a real Compose project named "standalone".
func (s *Store) Events(ctx context.Context, from, to time.Time, project, id string, standalone bool, limit int) ([]model.Event, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id,container_id,name,project,service,action,at,exit_code,signal FROM container_events
 WHERE at >= $1 AND at < $2 AND ($3='' OR project=$3) AND ($4='' OR container_id=$4) AND (NOT $5 OR project='')
 ORDER BY at DESC,id DESC LIMIT $6`, from, to, project, id, standalone, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Event, 0)
	for rows.Next() {
		var e model.Event
		if err := rows.Scan(&e.ID, &e.ContainerID, &e.Name, &e.Project, &e.Service, &e.Action, &e.At, &e.ExitCode, &e.Signal); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Projects includes event-only containers missed by the inventory poll.
func (s *Store) Projects(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT project FROM containers WHERE project<>'' UNION SELECT project FROM container_events WHERE project<>'' UNION SELECT project FROM test_sessions WHERE project<>'' AND (finished_at IS NULL OR finished_at>=now()-interval '7 days') UNION SELECT project FROM alert_rules WHERE project<>'' ORDER BY project`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
