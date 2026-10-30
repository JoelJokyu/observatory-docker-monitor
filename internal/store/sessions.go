package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrSessionActive = errors.New("a session is already active for this project")
var ErrSessionNotFound = errors.New("session not found")
var ErrSessionScope = errors.New("project has no currently observed containers")

type Session struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Project        string     `json:"project"`
	Standalone     bool       `json:"standalone"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	SampleInterval int        `json:"sample_interval_seconds"`
}

const sessionColumns = `id,name,project,standalone,started_at,finished_at,sample_interval_seconds`

func scanSession(row pgx.Row) (Session, error) {
	var v Session
	err := row.Scan(&v.ID, &v.Name, &v.Project, &v.Standalone, &v.StartedAt, &v.FinishedAt, &v.SampleInterval)
	return v, err
}

func (s *Store) StartSession(ctx context.Context, name, project string, standalone bool, interval int) (Session, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return Session{}, err
	}
	id := hex.EncodeToString(random[:])
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	// Serialize with inventory commits: a start snapshot cannot miss a container
	// that is replaced while its inventory transaction is still committing.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(841303)`); err != nil {
		return Session{}, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM containers WHERE present AND project=$1)`, project).Scan(&exists); err != nil {
		return Session{}, err
	}
	if !exists {
		return Session{}, ErrSessionScope
	}
	v, err := scanSession(tx.QueryRow(ctx, `INSERT INTO test_sessions(id,name,project,standalone,sample_interval_seconds) VALUES($1,$2,$3,$4,$5) RETURNING `+sessionColumns, id, name, project, standalone, interval))
	if err != nil {
		var failure *pgconn.PgError
		if errors.As(err, &failure) && failure.Code == "23505" {
			return Session{}, ErrSessionActive
		}
		return Session{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO session_members(session_id,container_id,name,service,first_observed_at)
 SELECT $1,id,name,service,$2 FROM containers WHERE present AND project=$3 ON CONFLICT DO NOTHING`, id, v.StartedAt, project)
	if err != nil {
		return Session{}, err
	}
	return v, tx.Commit(ctx)
}
func (s *Store) FinishSession(ctx context.Context, id string) (Session, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(841303)`); err != nil {
		return Session{}, err
	}
	v, err := scanSession(tx.QueryRow(ctx, `UPDATE test_sessions SET finished_at=COALESCE(finished_at,GREATEST(clock_timestamp(),started_at))
 WHERE id=$1 AND (finished_at IS NULL OR finished_at>=now()-interval '7 days') RETURNING `+sessionColumns, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, err
	}
	return v, tx.Commit(ctx)
}

func (s *Store) Sessions(ctx context.Context, project string, standalone bool, limit int) ([]Session, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+sessionColumns+` FROM test_sessions WHERE (finished_at IS NULL OR finished_at>=now()-interval '7 days')
 AND ($1='' OR project=$1) AND (NOT $2 OR standalone) ORDER BY (finished_at IS NULL) DESC,started_at DESC,id DESC LIMIT $3`, project, standalone, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Session, 0)
	for rows.Next() {
		v, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type SessionContainer struct {
	ID         string     `json:"container_id"`
	Name       string     `json:"name"`
	Service    string     `json:"service"`
	JoinedAt   time.Time  `json:"first_observed_at"`
	Samples    int64      `json:"samples"`
	First      *time.Time `json:"first_sample"`
	Last       *time.Time `json:"last_sample"`
	CPUCount   int64      `json:"cpu_observations"`
	CPUMean    *float64   `json:"cpu_mean"`
	CPUPeak    *float64   `json:"cpu_peak"`
	MemoryPeak *int64     `json:"memory_peak_bytes"`
}
type SessionAction struct {
	Action string `json:"action"`
	Count  int64  `json:"count"`
}
type SessionSummary struct {
	Session           Session            `json:"session"`
	AsOf              time.Time          `json:"as_of"`
	From              time.Time          `json:"available_from"`
	To                time.Time          `json:"to"`
	HistoryExpired    bool               `json:"history_expired"`
	ContainerCount    int64              `json:"container_count"`
	Samples           int64              `json:"samples"`
	EventCount        int64              `json:"event_count"`
	NonzeroExits      int64              `json:"nonzero_exits"`
	Containers        []SessionContainer `json:"containers"`
	ContainersLimited bool               `json:"containers_limited"`
	Actions           []SessionAction    `json:"actions"`
}

func (s *Store) SessionSummary(ctx context.Context, id string) (SessionSummary, error) {
	// All counts use one database snapshot and one clock boundary.
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return SessionSummary{}, err
	}
	defer tx.Rollback(ctx)
	result := SessionSummary{Containers: make([]SessionContainer, 0), Actions: make([]SessionAction, 0)}
	if err = tx.QueryRow(ctx, `SELECT transaction_timestamp()`).Scan(&result.AsOf); err != nil {
		return result, err
	}
	result.Session, err = scanSession(tx.QueryRow(ctx, `SELECT `+sessionColumns+` FROM test_sessions WHERE id=$1 AND (finished_at IS NULL OR finished_at>=now()-interval '7 days')`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrSessionNotFound
	}
	if err != nil {
		return result, err
	}
	result.To = result.AsOf
	if result.Session.FinishedAt != nil {
		result.To = *result.Session.FinishedAt
	}
	result.From = result.Session.StartedAt
	cutoff := result.AsOf.Add(-7 * 24 * time.Hour)
	if result.From.Before(cutoff) {
		result.From = cutoff
		result.HistoryExpired = true
	}
	if result.From.After(result.To) {
		result.From = result.To
	}
	// Membership comes from the start snapshot, later inventory, or in-window
	// events, including late replay after the session has finished.
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM session_members WHERE session_id=$1`, id).Scan(&result.ContainerCount); err != nil {
		return result, err
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM samples s JOIN session_members m ON m.container_id=s.container_id WHERE m.session_id=$1 AND s.at>=$2 AND s.at<$3`, id, result.From, result.To).Scan(&result.Samples); err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, `SELECT m.container_id,m.name,m.service,m.first_observed_at,count(s.at),min(s.at),max(s.at),count(s.cpu_percent),avg(s.cpu_percent),max(s.cpu_percent),max(s.memory_bytes)
 FROM session_members m LEFT JOIN samples s ON s.container_id=m.container_id AND s.at>=$2 AND s.at<$3 WHERE m.session_id=$1
 GROUP BY m.container_id,m.name,m.service,m.first_observed_at ORDER BY m.name,m.container_id LIMIT 501`, id, result.From, result.To)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var v SessionContainer
		if err = rows.Scan(&v.ID, &v.Name, &v.Service, &v.JoinedAt, &v.Samples, &v.First, &v.Last, &v.CPUCount, &v.CPUMean, &v.CPUPeak, &v.MemoryPeak); err != nil {
			rows.Close()
			return result, err
		}
		result.Containers = append(result.Containers, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if len(result.Containers) > 500 {
		result.Containers = result.Containers[:500]
		result.ContainersLimited = true
	}
	rows, err = tx.Query(ctx, `SELECT e.action,count(*),count(*) FILTER(WHERE e.action='die' AND e.exit_code<>'' AND e.exit_code<>'0')
 FROM container_events e JOIN session_members m ON m.container_id=e.container_id WHERE m.session_id=$1 AND e.at>=$2 AND e.at<$3 GROUP BY e.action ORDER BY e.action`, id, result.From, result.To)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var v SessionAction
		var nonzero int64
		if err = rows.Scan(&v.Action, &v.Count, &nonzero); err != nil {
			rows.Close()
			return result, err
		}
		result.Actions = append(result.Actions, v)
		result.EventCount += v.Count
		result.NonzeroExits += nonzero
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}
