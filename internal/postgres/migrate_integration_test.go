//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/codebyaadi/lookfar/internal/postgres"
	"github.com/codebyaadi/lookfar/internal/store"
)

func TestMigrationsAndStarterQueries(t *testing.T) {
	ctx := context.Background()
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

	applied, err := postgres.MigrateUp(ctx, pool)
	if err != nil || len(applied) < 4 {
		t.Fatalf("up: %v %v", applied, err)
	}
	if again, err := postgres.MigrateUp(ctx, pool); err != nil || len(again) != 0 {
		t.Fatalf("second up should be a no-op: %v %v", again, err)
	}

	q := store.New(pool)
	src, err := q.CreateSource(ctx, store.CreateSourceParams{Kind: "kev", Name: "cisa-kev", Enabled: true, Schedule: "@daily"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := q.GetSource(ctx, src.ID); err != nil || got.Name != "cisa-kev" {
		t.Fatalf("get source: %v %v", got, err)
	}
	if _, err := q.CreateSource(ctx, store.CreateSourceParams{Kind: "kev", Name: "cisa-kev"}); err == nil {
		t.Error("duplicate (kind, name) should fail")
	}
	if _, err := q.CreateSource(ctx, store.CreateSourceParams{Kind: "bogus", Name: "x"}); err == nil {
		t.Error("invalid kind should fail the check constraint")
	}
	if list, err := q.ListSources(ctx); err != nil || len(list) != 1 {
		t.Fatalf("list sources: %v %v", list, err)
	}

	run, err := q.CreateRun(ctx, store.CreateRunParams{Job: "collect kev", SourceID: &src.ID})
	if err != nil || run.Status != "running" {
		t.Fatalf("create run: %+v %v", run, err)
	}
	if got, err := q.GetRun(ctx, run.ID); err != nil || got.Job != "collect kev" {
		t.Fatalf("get run: %v %v", got, err)
	}
	if runs, err := q.ListRuns(ctx, store.ListRunsParams{Job: "collect kev", MaxRows: 10}); err != nil || len(runs) != 1 {
		t.Fatalf("list runs: %v %v", runs, err)
	}

	st, err := postgres.MigrationsStatus(ctx, pool)
	if err != nil || len(st) < 4 || !st[2].Applied {
		t.Fatalf("status: %+v %v", st, err)
	}

	if down, err := postgres.MigrateDownAll(ctx, pool); err != nil || len(down) < 4 {
		t.Fatalf("down all: %v %v", down, err)
	}
	if _, err := q.ListSources(ctx); err == nil {
		t.Error("sources table should be gone after down")
	}
	if applied, err := postgres.MigrateUp(ctx, pool); err != nil || len(applied) < 4 {
		t.Fatalf("up after down: %v %v", applied, err)
	}
}
