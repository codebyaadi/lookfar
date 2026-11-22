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

	"github.com/riverqueue/river"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/time/rate"

	"github.com/codebyaadi/lookfar/internal/jobs"
	"github.com/codebyaadi/lookfar/internal/postgres"
	"github.com/codebyaadi/lookfar/internal/stack"
)

// stack_sync must lead to collect_osv and then collect_nvd without anyone
// enqueueing them by hand. Periodic jobs are off so only the chain can start them.
func TestChainStackSyncToOSVToNVD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	osv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Queries []json.RawMessage `json:"queries"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		results := make([]map[string]any, len(req.Queries))
		for i := range results {
			results[i] = map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
	}))
	defer osv.Close()

	nvdPage, err := os.ReadFile("../advisory/testdata/nvd/CVE-2023-5868.json")
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Vulnerabilities []json.RawMessage `json:"vulnerabilities"`
	}
	_ = json.Unmarshal(nvdPage, &page)
	nvd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vulns := []json.RawMessage{}
		if r.URL.Query().Get("virtualMatchString") == "cpe:2.3:a:postgresql:postgresql" {
			vulns = page.Vulnerabilities
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"totalResults": len(vulns), "vulnerabilities": vulns})
	}))
	defer nvd.Close()

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

	source := func() (jobs.StackSpec, bool) {
		return jobs.StackSpec{Manual: []stack.ManualSpec{
			{Name: "postgresql", Version: "16.0"},
			{Name: "lodash", Version: "4.17.20", Kind: "package", Ecosystem: "npm"},
		}}, true
	}
	client, err := jobs.NewClient(pool, slog.Default(), jobs.Options{
		Serve: true, NoPeriodic: true, Stack: source, Version: "test",
		OSVURL: osv.URL, NVDURL: nvd.URL, NVDLimiter: rate.NewLimiter(rate.Inf, 1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Stop(context.Background()) })

	if _, err := client.Insert(ctx, jobs.StackSyncArgs{}, &river.InsertOpts{}); err != nil {
		t.Fatal(err)
	}

	// Wait until all three jobs have a succeeded run.
	deadline := time.Now().Add(2 * time.Minute)
	var ids = map[string]int64{}
	for time.Now().Before(deadline) {
		rows, err := pool.Query(ctx, `SELECT job, min(id) FROM runs WHERE status = 'succeeded' GROUP BY job`)
		if err != nil {
			t.Fatal(err)
		}
		ids = map[string]int64{}
		for rows.Next() {
			var job string
			var id int64
			_ = rows.Scan(&job, &id)
			ids[job] = id
		}
		rows.Close()
		if ids["stack_sync"] > 0 && ids["collect_osv"] > 0 && ids["collect_nvd"] > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if ids["stack_sync"] == 0 || ids["collect_osv"] == 0 || ids["collect_nvd"] == 0 {
		t.Fatalf("chain incomplete, succeeded runs: %v", ids)
	}
	if !(ids["stack_sync"] < ids["collect_osv"] && ids["collect_osv"] < ids["collect_nvd"]) {
		t.Errorf("wrong order: %v", ids)
	}

	// The tail of the chain did real work: NVD matched postgresql 16.0.
	var matches int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM component_advisories WHERE source = 'nvd'`).Scan(&matches); err != nil || matches != 1 {
		t.Errorf("nvd matches = %d (%v)", matches, err)
	}
	// Chained enqueues are unique: exactly one run each despite the chain.
	var osvRuns, nvdRuns int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM runs WHERE job = 'collect_osv'`).Scan(&osvRuns)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM runs WHERE job = 'collect_nvd'`).Scan(&nvdRuns)
	if osvRuns != 1 || nvdRuns != 1 {
		t.Errorf("runs: osv %d, nvd %d (want 1 each)", osvRuns, nvdRuns)
	}
}
