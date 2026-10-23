package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/codebyaadi/lookfar/internal/auth"
	"github.com/codebyaadi/lookfar/internal/source"
	"github.com/codebyaadi/lookfar/internal/store"
)

type apiEnv struct {
	router http.Handler
	token  string
	user   string
	pool   *pgxpool.Pool
}

func newAPIEnv(t *testing.T) *apiEnv {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var ok bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.snapshots') IS NOT NULL`).Scan(&ok); err != nil || !ok {
		t.Skip("migrations not applied")
	}

	user := fmt.Sprintf("api-test-%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, name, email) VALUES ($1, 'T', $2)`, user, user+"@example.test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, user) })

	token, hash, err := auth.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO api_tokens (user_id, name, token_hash) VALUES ($1, 'test', $2)`, user, hash); err != nil {
		t.Fatal(err)
	}

	q := store.New(pool)
	return &apiEnv{
		token: token, user: user, pool: pool,
		router: NewRouter(Deps{
			Auth:    auth.NewAuthenticator("secret", q),
			DB:      pool,
			Queries: q,
			Sources: source.NewService(pool, source.NewChecker(source.NewFetcher(true), "")),
		}),
	}
}

func (e *apiEnv) call(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: bad JSON %q: %v", method, path, rec.Body.String(), err)
		}
	}
	return rec.Code, out
}

func field(t *testing.T, m map[string]any, path ...string) any {
	t.Helper()
	var cur any = m
	for _, p := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("no object at %v in %v", path, m)
		}
		cur = mm[p]
	}
	return cur
}

func TestSourcesAPI(t *testing.T) {
	env := newAPIEnv(t)

	var price atomic.Value
	price.Store("$49")
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><head><title>Rival</title></head><body><p>Pro %s</p><p>Team $99</p></body></html>`, price.Load())
	}))
	defer target.Close()

	// Unauthenticated requests are refused.
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/sources", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list = %d", rec.Code)
	}

	// Validation.
	for name, body := range map[string]map[string]any{
		"bad url":        {"url": "ftp://x"},
		"bad kind":       {"kind": "ftp", "url": target.URL},
		"bad selector":   {"url": target.URL, "config": map[string]any{"selector": "div[["}},
		"bad interval":   {"url": target.URL, "intervalSecs": 5},
		"github non-gh":  {"kind": "github", "url": "https://gitlab.com/a/b"},
		"unknown fields": {"url": target.URL, "bogus": true},
	} {
		if code, _ := env.call(t, "POST", "/v1/sources", body); code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, code)
		}
	}

	// Create runs the first check and returns a baseline.
	code, created := env.call(t, "POST", "/v1/sources", map[string]any{"url": target.URL, "intervalSecs": 300})
	if code != http.StatusCreated {
		t.Fatalf("create = %d %v", code, created)
	}
	id := fmt.Sprint(int64(field(t, created, "source", "id").(float64)))
	if field(t, created, "check", "ok") != true || field(t, created, "check", "baseline") != true {
		t.Fatalf("first check should be a baseline: %v", created["check"])
	}
	if field(t, created, "source", "title") != "Rival" {
		t.Errorf("title should come from the page: %v", created["source"])
	}

	// Duplicate URL.
	if code, _ := env.call(t, "POST", "/v1/sources", map[string]any{"url": target.URL}); code != http.StatusConflict {
		t.Errorf("duplicate = %d, want 409", code)
	}

	// Checking again straight away is rate limited.
	if code, _ := env.call(t, "POST", "/v1/sources/"+id+"/check", nil); code != http.StatusTooManyRequests {
		t.Errorf("immediate re-check = %d, want 429", code)
	}
	// Age the last check so a manual check is allowed, then change the page.
	if _, err := env.pool.Exec(context.Background(), `UPDATE sources SET last_checked_at = now() - interval '1 minute' WHERE id = $1::bigint`, id); err != nil {
		t.Fatal(err)
	}
	price.Store("$59")
	code, checked := env.call(t, "POST", "/v1/sources/"+id+"/check", nil)
	if code != http.StatusOK || field(t, checked, "check", "changed") != true {
		t.Fatalf("check now = %d %v", code, checked)
	}

	// Snapshots: newest first, baseline flagged, diff available.
	code, list := env.call(t, "GET", "/v1/sources/"+id+"/snapshots", nil)
	snaps, _ := list["snapshots"].([]any)
	if code != http.StatusOK || len(snaps) != 2 {
		t.Fatalf("snapshots = %d %v", code, list)
	}
	newest := snaps[0].(map[string]any)
	if newest["baseline"] != false || newest["addedLines"] != float64(1) || newest["removedLines"] != float64(1) || snaps[1].(map[string]any)["baseline"] != true {
		t.Errorf("snapshot summaries: %v", snaps)
	}
	snapID := fmt.Sprint(int64(newest["id"].(float64)))
	code, detail := env.call(t, "GET", "/v1/sources/"+id+"/snapshots/"+snapID, nil)
	if code != http.StatusOK || !strings.Contains(fmt.Sprint(field(t, detail, "snapshot", "content")), "Pro $59") {
		t.Fatalf("snapshot detail = %d %v", code, detail)
	}
	hunks, _ := field(t, detail, "snapshot", "diff", "hunks").([]any)
	if len(hunks) == 0 {
		t.Errorf("diff has no hunks: %v", detail)
	}

	// Update: pause and retitle.
	code, upd := env.call(t, "PATCH", "/v1/sources/"+id, map[string]any{"paused": true, "title": "Rival pricing"})
	if code != http.StatusOK || field(t, upd, "source", "paused") != true || field(t, upd, "source", "title") != "Rival pricing" {
		t.Fatalf("update = %d %v", code, upd)
	}
	if code, _ := env.call(t, "PATCH", "/v1/sources/"+id, map[string]any{"config": map[string]any{"ignorePatterns": []string{"(bad"}}}); code != http.StatusBadRequest {
		t.Errorf("invalid config patch = %d, want 400", code)
	}

	// List shows the snapshot count.
	code, all := env.call(t, "GET", "/v1/sources", nil)
	srcs, _ := all["sources"].([]any)
	if code != http.StatusOK || len(srcs) != 1 || srcs[0].(map[string]any)["snapshotCount"] != float64(2) {
		t.Fatalf("list = %d %v", code, all)
	}

	// Another user cannot see, check or delete it.
	other := &apiEnv{router: env.router, pool: env.pool}
	otherUser := env.user + "-other"
	if _, err := env.pool.Exec(context.Background(), `INSERT INTO "user" (id, name, email) VALUES ($1, 'O', $2)`, otherUser, otherUser+"@example.test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { env.pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, otherUser) })
	tok, hash, _ := auth.NewToken()
	env.pool.Exec(context.Background(), `INSERT INTO api_tokens (user_id, name, token_hash) VALUES ($1, 'o', $2)`, otherUser, hash)
	other.token = tok
	for _, c := range [][2]string{{"GET", "/v1/sources/" + id}, {"POST", "/v1/sources/" + id + "/check"}, {"DELETE", "/v1/sources/" + id}, {"GET", "/v1/sources/" + id + "/snapshots/" + snapID}} {
		if code, _ := other.call(t, c[0], c[1], nil); code != http.StatusNotFound {
			t.Errorf("other user %s %s = %d, want 404", c[0], c[1], code)
		}
	}

	// Delete removes the source and its snapshots.
	if code, _ := env.call(t, "DELETE", "/v1/sources/"+id, nil); code != http.StatusNoContent {
		t.Fatalf("delete = %d", code)
	}
	if code, _ := env.call(t, "GET", "/v1/sources/"+id, nil); code != http.StatusNotFound {
		t.Errorf("get after delete = %d", code)
	}
	var left int
	env.pool.QueryRow(context.Background(), `SELECT count(*) FROM snapshots WHERE source_id = $1::bigint`, id).Scan(&left)
	if left != 0 {
		t.Errorf("%d snapshots left after delete", left)
	}
}
