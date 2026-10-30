package store

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"observatory.local/platform/internal/model"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	Pool           *pgxpool.Pool
	SampleInterval time.Duration
}

func Open(ctx context.Context, url string) (*Store, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("database configuration: %w", err)
	}
	config.MaxConns = 5
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	s := &Store{Pool: pool}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(841302)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		return err
	}
	for _, migration := range []struct {
		version int
		path    string
	}{{1, "migrations/001_initial.sql"}, {2, "migrations/002_io_pids.sql"}, {3, "migrations/003_projects_events.sql"}, {4, "migrations/004_test_sessions.sql"}, {5, "migrations/005_local_alerts.sql"}} {
		var exists bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", migration.version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sql, readErr := migrations.ReadFile(migration.path)
		if readErr != nil {
			return readErr
		}
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migration %d: %w", migration.version, err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", migration.version); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Inventory is replaced only after a successful Docker listing. Missing targets
// retain their identity and history rather than being silently deleted.
func (s *Store) Inventory(ctx context.Context, containers []model.Container) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(841303)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE containers SET present=false"); err != nil {
		return err
	}
	for _, c := range containers {
		_, err = tx.Exec(ctx, `INSERT INTO containers(id,name,image,state,seen_at,present,project,service)
   VALUES($1,$2,$3,$4,$5,true,$6,$7) ON CONFLICT(id) DO UPDATE SET
   name=excluded.name,image=excluded.image,state=excluded.state,seen_at=excluded.seen_at,present=true,project=excluded.project,service=excluded.service`, c.ID, c.Name, c.Image, c.State, c.SeenAt, c.Project, c.Service)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO session_members(session_id,container_id,name,service,first_observed_at)
 SELECT t.id,c.id,c.name,c.service,c.seen_at FROM test_sessions t JOIN containers c ON c.project=t.project
 WHERE c.present AND c.seen_at>=t.started_at AND (t.finished_at IS NULL OR c.seen_at<t.finished_at)
 AND (t.finished_at IS NULL OR t.finished_at>=now()-interval '7 days') ON CONFLICT (session_id,container_id) DO UPDATE SET name=excluded.name,service=excluded.service`)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Save(ctx context.Context, v model.Sample) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO samples(container_id,at,cpu_percent,memory_bytes,memory_limit_bytes,
 network_rx_bytes,network_tx_bytes,block_read_bytes,block_write_bytes,pids,
 network_rx_bytes_per_second,network_tx_bytes_per_second,block_read_bytes_per_second,block_write_bytes_per_second)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT DO NOTHING`,
		v.ContainerID, v.At, v.CPU, v.Memory, v.MemoryLimit, v.NetworkRx, v.NetworkTx, v.BlockRead, v.BlockWrite, v.PIDs,
		v.NetworkRxRate, v.NetworkTxRate, v.BlockReadRate, v.BlockWriteRate)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		if err = s.evaluateSample(ctx, tx, v); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) Containers(ctx context.Context) ([]model.Container, error) {
	rows, err := s.Pool.Query(ctx, `SELECT c.id,c.name,c.image,c.state,c.seen_at,c.present,c.project,c.service,
 s.at,s.cpu_percent,s.memory_bytes,s.memory_limit_bytes,s.network_rx_bytes,s.network_tx_bytes,
 s.block_read_bytes,s.block_write_bytes,s.pids,s.network_rx_bytes_per_second,s.network_tx_bytes_per_second,
 s.block_read_bytes_per_second,s.block_write_bytes_per_second FROM containers c
 LEFT JOIN LATERAL (SELECT * FROM samples WHERE container_id=c.id ORDER BY at DESC LIMIT 1) s ON true ORDER BY c.name,c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Container, 0)
	for rows.Next() {
		var c model.Container
		var v model.Sample
		var at *time.Time
		var mem, limit *int64
		if err := rows.Scan(&c.ID, &c.Name, &c.Image, &c.State, &c.SeenAt, &c.Present, &c.Project, &c.Service, &at, &v.CPU, &mem, &limit,
			&v.NetworkRx, &v.NetworkTx, &v.BlockRead, &v.BlockWrite, &v.PIDs, &v.NetworkRxRate, &v.NetworkTxRate, &v.BlockReadRate, &v.BlockWriteRate); err != nil {
			return nil, err
		}
		if at != nil {
			v.ContainerID = c.ID
			v.At = *at
			v.Memory = *mem
			v.MemoryLimit = *limit
			c.Latest = &v
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// History caps response size by aggregating into time buckets. The sample count
// and the first/last observation expose the resolution and observed coverage.
type Bucket struct {
	At             time.Time `json:"at"`
	First          time.Time `json:"first"`
	Last           time.Time `json:"last"`
	Count          int64     `json:"count"`
	CPU            *float64  `json:"cpu_percent"`
	Memory         float64   `json:"memory_bytes"`
	MemoryLimit    float64   `json:"memory_limit_bytes"`
	PIDs           *float64  `json:"pids"`
	NetworkRxRate  *float64  `json:"network_rx_bytes_per_second"`
	NetworkTxRate  *float64  `json:"network_tx_bytes_per_second"`
	BlockReadRate  *float64  `json:"block_read_bytes_per_second"`
	BlockWriteRate *float64  `json:"block_write_bytes_per_second"`
}

func (s *Store) History(ctx context.Context, id string, from, to time.Time, bucket int) ([]Bucket, error) {
	rows, err := s.Pool.Query(ctx, `SELECT to_timestamp(floor(extract(epoch FROM at)/$4)*$4) AS bucket,
 min(at),max(at),count(*),avg(cpu_percent),avg(memory_bytes),avg(memory_limit_bytes),avg(pids),avg(network_rx_bytes_per_second),avg(network_tx_bytes_per_second),avg(block_read_bytes_per_second),avg(block_write_bytes_per_second)
 FROM samples WHERE container_id=$1 AND at >= $2 AND at < $3 GROUP BY bucket ORDER BY bucket`, id, from, to, bucket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Bucket, 0)
	for rows.Next() {
		var b Bucket
		if err := rows.Scan(&b.At, &b.First, &b.Last, &b.Count, &b.CPU, &b.Memory, &b.MemoryLimit, &b.PIDs, &b.NetworkRxRate, &b.NetworkTxRate, &b.BlockReadRate, &b.BlockWriteRate); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Bounded batches avoid one unbounded retention transaction.
func (s *Store) Prune(ctx context.Context, cutoff time.Time) error {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM alert_conditions WHERE updated_at<$1 AND (project,container_id,kind) IN (SELECT project,container_id,kind FROM alert_conditions WHERE updated_at<$1 LIMIT 5000)`, cutoff); err != nil {
		return err
	}
	if _, err := s.Pool.Exec(ctx, `DELETE FROM local_alerts WHERE last_observed_at<$1 AND id IN (SELECT id FROM local_alerts WHERE last_observed_at<$1 LIMIT 5000)`, cutoff); err != nil {
		return err
	}
	for i := 0; i < 10; i++ {
		tag, err := s.Pool.Exec(ctx, `DELETE FROM test_sessions WHERE id IN (SELECT id FROM test_sessions WHERE finished_at<$1 ORDER BY finished_at LIMIT 500)`, cutoff)
		if err != nil {
			return err
		}
		if tag.RowsAffected() < 500 {
			break
		}
	}
	for i := 0; i < 10; i++ {
		tag, err := s.Pool.Exec(ctx, `DELETE FROM container_events WHERE id IN (SELECT id FROM container_events WHERE at < $1 ORDER BY at LIMIT 5000)`, cutoff)
		if err != nil {
			return err
		}
		if tag.RowsAffected() < 5000 {
			break
		}
	}
	for i := 0; i < 10; i++ {
		tag, err := s.Pool.Exec(ctx, `DELETE FROM samples WHERE (container_id,at) IN
  (SELECT container_id,at FROM samples WHERE at < $1 ORDER BY at LIMIT 5000)`, cutoff)
		if err != nil {
			return err
		}
		if tag.RowsAffected() < 5000 {
			break
		}
	}
	// Keep stopped containers, but expire removed identities once their history is gone.
	_, err := s.Pool.Exec(ctx, `DELETE FROM containers c WHERE NOT present AND seen_at < $1
 AND NOT EXISTS(SELECT 1 FROM samples s WHERE s.container_id=c.id)`, cutoff)
	return err
}
