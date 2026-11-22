//go:build integration

package jobs_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/codebyaadi/lookfar/internal/jobs"
	"github.com/codebyaadi/lookfar/internal/postgres"
	"github.com/codebyaadi/lookfar/internal/store"
)

func TestCollectKEVJobTwice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	body, err := os.ReadFile("../advisory/testdata/kev.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"e1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"e1"`)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	ctr, err := tcpostgres.Run(ctx, "postgres:18",
		tcpostgres.WithDatabase("lookfar"), tcpostgres.WithUsername("lookfar"), tcpostgres.WithPassword("lookfar"),
		tcpostgres.BasicWaitStrategies(),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(ctr) })
	url, _ := ctr.ConnectionString(ctx, "sslmode=disable")
	pool, err := postgres.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := postgres.MigrateUp(ctx, pool); err != nil {
		t.Fatal(err)
	}

	client, err := jobs.NewClient(pool, slog.Default(), jobs.Options{KEVURL: srv.URL, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Stop(context.Background()) })

	for range 2 {
		if err := jobs.RunNow(ctx, client, jobs.CollectKEVArgs{}); err != nil {
			t.Fatal(err)
		}
	}

	runs, err := store.New(pool).ListRuns(ctx, store.ListRunsParams{Job: "collect_kev", MaxRows: 5})
	if err != nil || len(runs) != 2 {
		t.Fatalf("runs: %v %v", runs, err)
	}
	// ListRuns is newest first.
	var second, first map[string]any
	_ = json.Unmarshal(runs[0].Stats, &second)
	_ = json.Unmarshal(runs[1].Stats, &first)
	if first["new"] != float64(5) || first["fetched"] != float64(5) {
		t.Errorf("first run stats: %v", first)
	}
	if second["not_modified"] != true || second["unchanged"] != float64(5) || second["new"] != float64(0) {
		t.Errorf("second run stats: %v", second)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM advisories WHERE exploited`).Scan(&n); err != nil || n != 5 {
		t.Errorf("exploited advisories = %d (%v)", n, err)
	}
}
