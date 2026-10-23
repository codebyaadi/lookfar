package source

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/codebyaadi/lookfar/internal/store"
)

// testDB connects to DATABASE_URL (with migrations applied) or skips.
func testDB(t *testing.T) (*pgxpool.Pool, *store.Queries) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var ok bool
	if err := pool.QueryRow(context.Background(), `SELECT to_regclass('public.snapshots') IS NOT NULL`).Scan(&ok); err != nil || !ok {
		t.Skip("migrations not applied")
	}
	return pool, store.New(pool)
}

func newTestUser(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	id := fmt.Sprintf("test-%d", time.Now().UnixNano())
	if _, err := pool.Exec(context.Background(), `INSERT INTO "user" (id, name, email) VALUES ($1, 'T', $2)`, id, id+"@example.test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE dedupe_key LIKE 'check:%' AND payload->>'sourceId' IN (SELECT id::text FROM sources WHERE user_id = $1)`, id)
		pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, id)
	})
	return id
}

func TestServiceCheckCycle(t *testing.T) {
	pool, q := testDB(t)
	ctx := context.Background()
	user := newTestUser(t, pool)

	var version atomic.Int32
	var failing atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		price := map[int32]string{1: "$49", 2: "$49", 3: "$59"}[version.Load()]
		// Version 2 only changes a ticking timestamp, which must not count.
		stamp := map[int32]string{1: "5 minutes ago", 2: "9 minutes ago", 3: "1 minute ago"}[version.Load()]
		fmt.Fprintf(w, `<html><head><title>Pricing</title></head><body><h1>Plans</h1><p>Pro %s</p><p>Updated %s</p></body></html>`, price, stamp)
	}))
	defer srv.Close()

	svc := NewService(pool, NewChecker(NewFetcher(true), ""))
	cfgJSON := []byte(`{"minChangeChars":3}`)
	src, err := q.CreateSource(ctx, store.CreateSourceParams{UserID: user, Kind: KindPage, Url: srv.URL, Config: cfgJSON, IntervalSecs: 300})
	if err != nil {
		t.Fatal(err)
	}

	// 1. First check stores the baseline.
	version.Store(1)
	out, err := svc.CheckSource(ctx, src.ID)
	if err != nil || !out.Changed || !out.Baseline {
		t.Fatalf("baseline: %+v, %v", out, err)
	}
	if out.Snapshot.Diff != nil {
		t.Errorf("a baseline has no diff")
	}

	// 2. A ticking timestamp is not a change.
	version.Store(2)
	if out, err = svc.CheckSource(ctx, src.ID); err != nil || out.Changed {
		t.Fatalf("timestamp-only change: %+v, %v", out, err)
	}

	// 3. A real change stores a snapshot with a diff.
	version.Store(3)
	out, err = svc.CheckSource(ctx, src.ID)
	if err != nil || !out.Changed || out.Baseline {
		t.Fatalf("real change: %+v, %v", out, err)
	}
	if out.Snapshot.AddedLines != 1 || out.Snapshot.RemovedLines != 1 || out.Snapshot.Diff == nil {
		t.Errorf("diff counts: %+v", out.Snapshot)
	}

	snaps, err := q.ListSnapshots(ctx, store.ListSnapshotsParams{SourceID: src.ID, RowLimit: 10})
	if err != nil || len(snaps) != 2 {
		t.Fatalf("want 2 snapshots (baseline + change), got %d, %v", len(snaps), err)
	}
	got, err := q.GetSource(ctx, store.GetSourceParams{ID: src.ID, UserID: user})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Pricing" || got.LastChangedAt == nil || got.ErrorCount != 0 || !got.NextCheckAt.After(time.Now()) {
		t.Errorf("source state after checks: %+v", got)
	}

	// 4. A failing site records the error, backs off, and surfaces CheckError.
	failing.Store(true)
	_, err = svc.CheckSource(ctx, src.ID)
	var ce *CheckError
	if !errors.As(err, &ce) {
		t.Fatalf("want CheckError, got %v", err)
	}
	got, _ = q.GetSource(ctx, store.GetSourceParams{ID: src.ID, UserID: user})
	if got.ErrorCount != 1 || got.LastError == nil || !got.NextCheckAt.After(time.Now().Add(5*time.Minute)) {
		t.Errorf("failure state: errors=%d last=%v next=%v", got.ErrorCount, got.LastError, got.NextCheckAt)
	}

	// 5. Recovery clears the error.
	failing.Store(false)
	if _, err := svc.CheckSource(ctx, src.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = q.GetSource(ctx, store.GetSourceParams{ID: src.ID, UserID: user})
	if got.ErrorCount != 0 || got.LastError != nil {
		t.Errorf("error should clear on success: %+v", got)
	}

	// 6. Another user cannot read this source or its snapshots.
	if _, err := q.GetSource(ctx, store.GetSourceParams{ID: src.ID, UserID: "someone-else"}); err == nil {
		t.Errorf("ownership check failed for GetSource")
	}
	if _, err := q.GetSnapshot(ctx, store.GetSnapshotParams{ID: snaps[0].ID, SourceID: src.ID, UserID: "someone-else"}); err == nil {
		t.Errorf("ownership check failed for GetSnapshot")
	}
}

func TestServiceScheduleDueIsIdempotent(t *testing.T) {
	pool, q := testDB(t)
	ctx := context.Background()
	user := newTestUser(t, pool)

	svc := NewService(pool, NewChecker(NewFetcher(true), ""))
	src, err := q.CreateSource(ctx, store.CreateSourceParams{UserID: user, Kind: KindPage, Url: "https://example.com/due", Config: []byte(`{}`), IntervalSecs: 300})
	if err != nil {
		t.Fatal(err)
	}
	// New sources are held off briefly; make this one due.
	if _, err := pool.Exec(ctx, `UPDATE sources SET next_check_at = now() - interval '1 second' WHERE id = $1`, src.ID); err != nil {
		t.Fatal(err)
	}
	paused, err := q.CreateSource(ctx, store.CreateSourceParams{UserID: user, Kind: KindPage, Url: "https://example.com/paused", Config: []byte(`{}`), IntervalSecs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sources SET next_check_at = now() - interval '1 second' WHERE id = $1`, paused.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpdateSource(ctx, store.UpdateSourceParams{ID: paused.ID, UserID: user, Title: "", Config: []byte(`{}`), IntervalSecs: 300, Paused: true}); err != nil {
		t.Fatal(err)
	}

	count := func() int {
		var n int
		pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE kind = $1 AND payload->>'sourceId' = ANY($2)`, JobKind,
			[]string{fmt.Sprint(src.ID), fmt.Sprint(paused.ID)}).Scan(&n)
		return n
	}
	for range 3 {
		if _, err := svc.ScheduleDue(ctx, 1000); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(); n != 1 {
		t.Fatalf("want exactly one job for the due, unpaused source, got %d", n)
	}
}

// The inline first check and the scheduler (or two manual checks) can run at
// the same moment. They must store one baseline, not several.
func TestServiceConcurrentChecksStoreOneSnapshot(t *testing.T) {
	pool, q := testDB(t)
	ctx := context.Background()
	user := newTestUser(t, pool)

	// Hold every request until all six have arrived, then answer together, so
	// the six checks reach the database at the same moment.
	var arrived atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrived.Add(1)
		for deadline := time.Now().Add(3 * time.Second); arrived.Load() < 6 && time.Now().Before(deadline); {
			time.Sleep(time.Millisecond)
		}
		fmt.Fprint(w, `<html><body><p>Same content every time</p></body></html>`)
	}))
	defer srv.Close()

	svc := NewService(pool, NewChecker(NewFetcher(true), ""))
	src, err := q.CreateSource(ctx, store.CreateSourceParams{UserID: user, Kind: KindPage, Url: srv.URL, Config: []byte(`{}`), IntervalSecs: 300})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var baselines atomic.Int32
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := svc.CheckSource(ctx, src.ID)
			if err != nil {
				t.Error(err)
				return
			}
			if out.Baseline {
				baselines.Add(1)
			}
		}()
	}
	wg.Wait()

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM snapshots WHERE source_id = $1`, src.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 || baselines.Load() != 1 {
		t.Fatalf("want exactly one baseline snapshot, got %d snapshots and %d baselines", n, baselines.Load())
	}
}
