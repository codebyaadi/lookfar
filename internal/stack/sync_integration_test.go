//go:build integration

package stack_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/codebyaadi/lookfar/internal/postgres"
	"github.com/codebyaadi/lookfar/internal/stack"
	"github.com/codebyaadi/lookfar/internal/store"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ctr, err := tcpostgres.Run(ctx, "postgres:18",
		tcpostgres.WithDatabase("lookfar"), tcpostgres.WithUsername("lookfar"), tcpostgres.WithPassword("lookfar"),
		tcpostgres.BasicWaitStrategies(),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(ctr) })
	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := postgres.MigrateUp(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func copyFixture(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func activeCount(t *testing.T, pool *pgxpool.Pool) (active, total int) {
	t.Helper()
	rows, err := store.New(pool).ListComponents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Active {
			active++
		}
	}
	return active, len(rows)
}

func TestSyncIdempotentDeactivateReactivate(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	dir := t.TempDir()
	gomod := copyFixture(t, dir, "go.mod")          // 3 components
	reqs := copyFixture(t, dir, "requirements.txt") // 3 components
	manual := []stack.ManualSpec{{Name: "postgresql", Version: "16.4"}, {Name: "stripe", Kind: "saas"}}

	sync := func(manifests ...string) stack.SyncSummary {
		t.Helper()
		sum, err := stack.Sync(ctx, pool, stack.Resolve(ctx, manifests, manual))
		if err != nil {
			t.Fatal(err)
		}
		return sum
	}

	// First run adds everything: 3 + 3 + 2 manual.
	if s := sync(gomod, reqs); s.Added != 8 || s.Updated != 0 || s.Deactivated != 0 {
		t.Fatalf("first sync: %+v", s)
	}
	// Second run changes nothing.
	if s := sync(gomod, reqs); s.Added != 0 || s.Updated != 0 || s.Deactivated != 0 || s.Unchanged != 8 {
		t.Fatalf("second sync should be a no-op: %+v", s)
	}

	// Removing a manifest from the config deactivates its components, keeping the rows.
	if s := sync(gomod); s.Deactivated != 3 || s.Added != 0 {
		t.Fatalf("after removal: %+v", s)
	}
	if a, total := activeCount(t, pool); a != 5 || total != 8 {
		t.Fatalf("active %d total %d", a, total)
	}

	// Adding it back reactivates the same rows instead of inserting new ones.
	if s := sync(gomod, reqs); s.Updated != 3 || s.Added != 0 || s.Deactivated != 0 {
		t.Fatalf("after re-adding: %+v", s)
	}
	if a, total := activeCount(t, pool); a != 8 || total != 8 {
		t.Fatalf("active %d total %d", a, total)
	}

	// A manifest that cannot be read this time keeps its components active.
	if err := os.Remove(reqs); err != nil {
		t.Fatal(err)
	}
	s := sync(gomod, reqs)
	if s.Deactivated != 0 || len(s.FileErrors) != 1 {
		t.Fatalf("unreadable manifest: %+v", s)
	}
	if a, _ := activeCount(t, pool); a != 8 {
		t.Fatalf("components of an unreadable manifest must stay active, active=%d", a)
	}

	// Dropping the unreadable manifest from the list and one manual component deactivates
	// both (3 requirements rows + stripe).
	sum, err := stack.Sync(ctx, pool, stack.Resolve(ctx, []string{gomod}, manual[:1]))
	if err != nil || sum.Deactivated != 4 {
		t.Fatalf("manual removal: %+v %v", sum, err)
	}

	// Server components carry their CPE.
	rows, _ := store.New(pool).ListComponents(ctx)
	found := false
	for _, r := range rows {
		if r.Purl == "pkg:generic/postgresql@16.4" {
			found = r.Cpe == "cpe:2.3:a:postgresql:postgresql:16.4:*:*:*:*:*:*:*" && r.Kind == "server" && r.Origin == "manual"
		}
	}
	if !found {
		t.Error("postgresql component missing CPE or kind")
	}
}
