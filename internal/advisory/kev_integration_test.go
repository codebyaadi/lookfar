//go:build integration

package advisory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/codebyaadi/lookfar/internal/postgres"
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

// feedServer serves the fixture with an ETag and honours If-None-Match.
type feedServer struct {
	*httptest.Server
	body  atomic.Value // []byte
	etag  atomic.Value // string
	calls atomic.Int32
}

func newFeedServer(t *testing.T, body []byte) *feedServer {
	t.Helper()
	fs := &feedServer{}
	fs.body.Store(body)
	fs.etag.Store(`v1`)
	fs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.calls.Add(1)
		etag := fs.etag.Load().(string)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", "Fri, 20 Nov 2026 17:48:39 GMT")
		_, _ = w.Write(fs.body.Load().([]byte))
	}))
	t.Cleanup(fs.Close)
	return fs
}

func TestKEVCollectIdempotentWithConditionalGet(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	body, err := os.ReadFile("testdata/kev.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := newFeedServer(t, body)
	k := &KEV{Pool: pool, Fetcher: testFetcher(), URL: srv.URL}

	r1, err := k.Collect(ctx, time.Time{})
	if err != nil || r1.Fetched != 5 || r1.New != 5 || r1.NotModified {
		t.Fatalf("first run: %+v %v", r1, err)
	}

	// Second run: the stored ETag makes the server answer 304.
	r2, err := k.Collect(ctx, time.Time{})
	if err != nil || !r2.NotModified || r2.Unchanged != 5 || r2.New != 0 {
		t.Fatalf("second run: %+v %v", r2, err)
	}

	// The feed changes (new ETag) but entries are identical: all unchanged, nothing rewritten.
	srv.etag.Store(`"v2"`)
	r3, err := k.Collect(ctx, time.Time{})
	if err != nil || r3.NotModified || r3.Unchanged != 5 || r3.New != 0 || r3.Updated != 0 {
		t.Fatalf("third run: %+v %v", r3, err)
	}

	// One entry changes upstream: exactly one update.
	changed := strings.Replace(string(body), `"requiredAction": "`, `"requiredAction": "UPDATED `, 1)
	srv.body.Store([]byte(changed))
	srv.etag.Store(`v3`)
	r4, err := k.Collect(ctx, time.Time{})
	if err != nil || r4.Updated != 1 || r4.Unchanged != 4 {
		t.Fatalf("fourth run: %+v %v", r4, err)
	}

	q := store.New(pool)
	src, err := q.UpsertSource(ctx, store.UpsertSourceParams{Kind: "kev", Name: "cisa-kev", Schedule: "@daily"})
	if err != nil {
		t.Fatal(err)
	}
	if src.LastRunAt == nil || src.LastOkAt == nil || src.LastError.Valid || !strings.Contains(string(src.Cursor), `v3`) {
		t.Errorf("source bookkeeping: %+v cursor=%s", src, src.Cursor)
	}
	if n, _ := q.CountRawItems(ctx, src.ID); n != 5 {
		t.Errorf("raw items = %d", n)
	}
	entries, _ := parseKEV(body)
	adv, err := q.GetAdvisory(ctx, entries[0].Entry.CVEID)
	if err != nil || !adv.Exploited || adv.ExploitedSince == nil || adv.Summary == "" {
		t.Errorf("advisory: %+v %v", adv, err)
	}
}

func TestKEVFailureRecordedAndKeepsCursor(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	body, _ := os.ReadFile("testdata/kev.json")
	srv := newFeedServer(t, body)
	k := &KEV{Pool: pool, Fetcher: testFetcher(), URL: srv.URL}
	if _, err := k.Collect(ctx, time.Time{}); err != nil {
		t.Fatal(err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer bad.Close()
	f := testFetcher()
	f.MaxRetries = 1
	k2 := &KEV{Pool: pool, Fetcher: f, URL: bad.URL}
	if _, err := k2.Collect(ctx, time.Time{}); err == nil {
		t.Fatal("expected failure")
	}

	src, _ := store.New(pool).UpsertSource(ctx, store.UpsertSourceParams{Kind: "kev", Name: "cisa-kev", Schedule: "@daily"})
	if !src.LastError.Valid || src.LastOkAt == nil || !strings.Contains(string(src.Cursor), `v1`) {
		t.Errorf("failure bookkeeping: err=%v ok=%v cursor=%s", src.LastError, src.LastOkAt, src.Cursor)
	}
}

// Another source may have written the advisory first; KEV must add to it, not replace it.
func TestKEVMergesWithExistingAdvisory(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	body, _ := os.ReadFile("testdata/kev.json")
	entries, _ := parseKEV(body)
	id := entries[0].Entry.CVEID

	_, err := pool.Exec(ctx, `INSERT INTO advisories (id, aliases, summary, severity, cvss, affected, refs)
		VALUES ($1, ARRAY['GHSA-xxxx-yyyy-zzzz'], 'OSV summary', 'critical', 9.8,
		        '[{"source":"osv","package":"left-pad"}]', '[{"source":"osv","url":"https://osv.dev/x"}]')`, id)
	if err != nil {
		t.Fatal(err)
	}

	srv := newFeedServer(t, body)
	k := &KEV{Pool: pool, Fetcher: testFetcher(), URL: srv.URL}
	for range 2 { // twice: the kev-tagged elements must not accumulate
		srv.etag.Store(srv.etag.Load().(string) + "x") // defeat the 304
		if _, err := k.Collect(ctx, time.Time{}); err != nil {
			t.Fatal(err)
		}
	}

	var (
		summary, severity, affected, refs string
		exploited                         bool
		aliases                           []string
		sinceSet                          bool
	)
	err = pool.QueryRow(ctx, `SELECT summary, severity, affected::text, refs::text, exploited, aliases, exploited_since IS NOT NULL
		FROM advisories WHERE id = $1`, id).Scan(&summary, &severity, &affected, &refs, &exploited, &aliases, &sinceSet)
	if err != nil {
		t.Fatal(err)
	}
	if summary != "OSV summary" || severity != "critical" || len(aliases) != 1 {
		t.Errorf("KEV clobbered other sources: summary=%q severity=%q aliases=%v", summary, severity, aliases)
	}
	if !exploited || !sinceSet {
		t.Error("KEV must set the exploited fields")
	}
	for name, v := range map[string]string{"affected": affected, "refs": refs} {
		if strings.Count(v, `"source": "osv"`) != 1 || strings.Count(v, `"source": "kev"`) != 1 {
			t.Errorf("%s should hold one osv and one kev element: %s", name, v)
		}
	}
}
