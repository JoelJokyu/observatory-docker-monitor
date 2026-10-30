package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"observatory.local/platform/internal/model"
)

func integrationStore(t *testing.T) (context.Context, *Store) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run real PostgreSQL integration checks")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := pgx.Identifier{fmt.Sprintf("observatory_test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close(ctx)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
		admin.Close(cleanup)
	})
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return ctx, &Store{Pool: pool}
}
func TestPostgresLifecycle(t *testing.T) {
	ctx, s := integrationStore(t)
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.migrate(ctx); err != nil {
		t.Fatal("migration not repeatable", err)
	}
	now := time.Now().UTC()
	c := model.Container{ID: "old-id", Name: "chat", Image: "chat:test", State: "running", SeenAt: now}
	if err := s.Inventory(ctx, []model.Container{c}); err != nil {
		t.Fatal(err)
	}
	cpu, netRate, blockRate := 50.0, 1024.0, 512.0
	total, pids := int64(50000), int64(7)
	for _, at := range []time.Time{now.Add(-8 * 24 * time.Hour), now.Add(-20 * time.Second), now.Add(-10 * time.Second)} {
		v := model.Sample{ContainerID: c.ID, At: at, CPU: &cpu, Memory: 100, MemoryLimit: 1000, NetworkRx: &total, NetworkTx: &total, BlockRead: &total, BlockWrite: &total, PIDs: &pids, NetworkRxRate: &netRate, NetworkTxRate: &netRate, BlockReadRate: &blockRate, BlockWriteRate: &blockRate}
		if err := s.Save(ctx, v); err != nil {
			t.Fatal(err)
		}
		if err := s.Save(ctx, v); err != nil {
			t.Fatal("duplicate sample", err)
		}
	}
	items, err := s.Containers(ctx)
	if err != nil || len(items) != 1 || items[0].Latest == nil {
		t.Fatalf("latest sample failed: %+v %v", items, err)
	}
	latest := items[0].Latest
	if latest.NetworkRx == nil || *latest.NetworkRx != total || latest.PIDs == nil || *latest.PIDs != pids || latest.BlockWriteRate == nil || *latest.BlockWriteRate != blockRate {
		t.Fatalf("I/O and PID roundtrip failed: %+v", latest)
	}
	history, err := s.History(ctx, c.ID, now.Add(-time.Minute), now, 60)
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	for _, b := range history {
		count += b.Count
		if b.CPU == nil || *b.CPU != 50 || b.NetworkRxRate == nil || *b.NetworkRxRate != netRate || b.PIDs == nil || *b.PIDs != 7 {
			t.Fatalf("incorrect bucket means: %+v", b)
		}
	}
	if count != 2 {
		t.Fatalf("history or dedup failed: %+v", history)
	}
	if err = s.Inventory(ctx, nil); err != nil {
		t.Fatal(err)
	}
	items, err = s.Containers(ctx)
	if err != nil || len(items) != 1 || items[0].Present {
		t.Fatal("removed container should retain history", err)
	}
	if err = s.Prune(ctx, now.Add(-7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM samples").Scan(&remaining); err != nil || remaining != 2 {
		t.Fatal("retention failed", remaining, err)
	}
	c.ID = "new-id"
	if err = s.Inventory(ctx, []model.Container{c}); err != nil {
		t.Fatal(err)
	}
	items, err = s.Containers(ctx)
	if err != nil || len(items) != 2 {
		t.Fatal("recreation merged identities", err)
	}
}
func TestUpgradePreservesHistory(t *testing.T) {
	ctx, s := integrationStore(t)
	initial, err := migrations.ReadFile("migrations/001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, string(initial)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, `CREATE TABLE schema_migrations(version integer PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT now()); INSERT INTO schema_migrations(version) VALUES(1); INSERT INTO containers(id,name,image,state,seen_at,present) VALUES('legacy','chat','chat:v1','running',now(),true); INSERT INTO samples(container_id,at,cpu_percent,memory_bytes,memory_limit_bytes) VALUES('legacy',now(),50,100,1000)`); err != nil {
		t.Fatal(err)
	}
	if err = s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := s.Containers(ctx)
	if err != nil || len(items) != 1 || items[0].Latest == nil {
		t.Fatal("upgrade lost existing sample", err)
	}
	old := items[0].Latest
	if old.CPU == nil || *old.CPU != 50 || old.Memory != 100 || old.NetworkRx != nil || old.PIDs != nil || old.BlockReadRate != nil {
		t.Fatalf("old values changed or new fields fabricated: %+v", old)
	}
	var versions int
	if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&versions); err != nil || versions != 5 {
		t.Fatal("migration versions not recorded", err)
	}
}
