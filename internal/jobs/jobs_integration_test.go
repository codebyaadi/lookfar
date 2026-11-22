//go:build integration

package jobs_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/codebyaadi/lookfar/internal/jobs"
	"github.com/codebyaadi/lookfar/internal/postgres"
	"github.com/codebyaadi/lookfar/internal/store"
)

func TestRunHeartbeatRecordsRun(t *testing.T) {
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
		t.Fatalf("migrate up: %v", err)
	}

	client, err := jobs.NewClient(pool, slog.Default(), jobs.Options{Serve: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Stop(context.Background()) })

	if err := jobs.RunNow(ctx, client, jobs.HeartbeatArgs{}); err != nil {
		t.Fatalf("run heartbeat: %v", err)
	}

	runs, err := store.New(pool).ListRuns(ctx, store.ListRunsParams{Job: "heartbeat", MaxRows: 5})
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs: %v %v", runs, err)
	}
	r := runs[0]
	if r.Status != jobs.StatusSucceeded || r.FinishedAt == nil || r.Error.Valid || len(r.Stats) < 3 {
		t.Errorf("unexpected run row: %+v", r)
	}

	// Migrating down everything (River included) and up again must work.
	_ = client.Stop(ctx)
	if _, err := postgres.MigrateDownAll(ctx, pool); err != nil {
		t.Fatalf("down all: %v", err)
	}
	if applied, err := postgres.MigrateUp(ctx, pool); err != nil || len(applied) < 4 {
		t.Fatalf("up again: %v %v", applied, err)
	}
}
