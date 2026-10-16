package httpapi_test

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

	"github.com/codebyaadi/feedoku/internal/auth"
	"github.com/codebyaadi/feedoku/internal/enrich"
	"github.com/codebyaadi/feedoku/internal/feed"
	"github.com/codebyaadi/feedoku/internal/httpapi"
	"github.com/codebyaadi/feedoku/internal/ingest"
	"github.com/codebyaadi/feedoku/internal/jobs"
	"github.com/codebyaadi/feedoku/internal/llm"
	"github.com/codebyaadi/feedoku/internal/postgres"
	"github.com/codebyaadi/feedoku/internal/store"
)

func longText(word string) string { return strings.Repeat(word+" ", 120) }

func aiFeed() string {
	item := func(n int, body string) string {
		return fmt.Sprintf(`<item><title>Story %d</title><link>https://blog.test/s%d</link><guid>ai-%d</guid>
<pubDate>%s</pubDate><description>%s</description></item>`, n, n, n, time.Now().Add(-time.Duration(n)*time.Hour).Format(time.RFC1123Z), body)
	}
	return `<?xml version="1.0"?><rss version="2.0"><channel><title>AI Test Blog</title><link>https://blog.test/</link>` +
		item(1, longText("alpha")) + item(2, longText("beta")) + item(3, "tiny") + `</channel></rss>`
}

// TestEnrichmentPipeline runs the real queue against Postgres with fake LLM
// and embedding servers. Use a throwaway TEST_DATABASE_URL: it claims jobs.
func TestEnrichmentPipeline(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	// The runner claims every job of its kind, so start from an empty queue.
	if _, err := pool.Exec(ctx, `UPDATE jobs SET done_at = now() WHERE kind = $1 AND done_at IS NULL`, enrich.KindEnrichArticle); err != nil {
		t.Fatal(err)
	}

	suffix := fmt.Sprint(time.Now().UnixNano())
	userID, token := "ai"+suffix, "aitok"+suffix
	if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, name, email) VALUES ($1, 'T', $2)`, userID, userID+"@test.dev"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, userID) })
	if _, err := pool.Exec(ctx, `INSERT INTO session (id, "expiresAt", token, "userId") VALUES ($1, now() + interval '1 hour', $2, $1)`, userID, token); err != nil {
		t.Fatal(err)
	}

	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, "<html></html>")
		case "/rss.xml":
			fmt.Fprint(w, aiFeed())
		default:
			http.NotFound(w, r)
		}
	}))
	defer feedSrv.Close()

	// Fake LLM: the very first completion is malformed to exercise the retry.
	var llmCalls atomic.Int32
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := llmCalls.Add(1)
		content := `{"tldr":"A short summary.","key_points":["one","two"],"topics":["programming","ai & ml"],"entities":["Go"],"sentiment":"positive"}`
		if n == 1 {
			content = "Sorry, here you go: not json"
		}
		out, _ := json.Marshal(map[string]any{
			"model":   "fake-model",
			"choices": []any{map[string]any{"message": map[string]string{"content": content}}},
			"usage":   map[string]int{"prompt_tokens": 100, "completion_tokens": 20},
		})
		w.Write(out)
	}))
	defer llmSrv.Close()

	voyageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Input []string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		data := make([]map[string]any, len(in.Input))
		for i := range in.Input {
			vec := make([]float32, 1024)
			vec[i%1024] = 1
			data[i] = map[string]any{"index": i, "embedding": vec}
		}
		out, _ := json.Marshal(map[string]any{"data": data, "usage": map[string]int{"total_tokens": 7}})
		w.Write(out)
	}))
	defer voyageSrv.Close()

	q := store.New(pool)
	svc := ingest.New(q, feed.NewFetcher(true))
	f, err := svc.AddFeed(ctx, userID, feedSrv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE dedupe_key IN (SELECT 'enrich:' || id FROM articles WHERE feed_id = $1)`, f.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM feeds WHERE id = $1`, f.ID)
	})

	var pending int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM jobs j WHERE done_at IS NULL AND dedupe_key IN (SELECT 'enrich:' || id FROM articles WHERE feed_id = $1)`, f.ID).Scan(&pending)
	if pending != 3 {
		t.Fatalf("expected 3 enrichment jobs enqueued, got %d", pending)
	}
	// Re-polling must not enqueue duplicates.
	svc.Poll(ctx, f)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE dedupe_key IN (SELECT 'enrich:' || id FROM articles WHERE feed_id = $1)`, f.ID).Scan(&pending)
	if pending != 3 {
		t.Fatalf("re-poll created duplicate jobs: %d", pending)
	}

	chain := llm.NewChain(&llm.Provider{Name: "fake", BaseURL: llmSrv.URL, APIKey: "k", Model: "m"})
	runner := jobs.NewRunner(q, 50*time.Millisecond)
	runner.Register(enrich.KindEnrichArticle, enrich.New(q, chain).Handle)
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { runner.Run(runCtx, 2); close(done) }()
	defer func() { stop(); <-done }()

	waitFor(t, 10*time.Second, "all articles enriched", func() bool {
		var n int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM article_ai ai JOIN articles a ON a.id = ai.article_id WHERE a.feed_id = $1 AND ai.status = 'done'`, f.ID).Scan(&n)
		return n == 3
	})
	// 2 long articles + 1 retry; the tiny article must not cost an LLM call.
	if got := llmCalls.Load(); got != 3 {
		t.Errorf("LLM calls = %d, want 3 (2 articles + 1 retry, tiny article skipped)", got)
	}
	var reqs, inTok int
	_ = pool.QueryRow(ctx, `SELECT COALESCE(sum(requests),0), COALESCE(sum(input_tokens),0) FROM ai_usage WHERE scope = 'enrich' AND day = current_date`).Scan(&reqs, &inTok)
	if reqs < 3 || inTok < 300 {
		t.Errorf("usage not recorded: requests=%d input=%d", reqs, inTok)
	}

	// Embeddings
	emb := enrich.NewEmbedder(q, llm.NewVoyage(voyageSrv.URL, "k", "voyage-4-lite", 1024), 16)
	n, err := emb.Sweep(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var embedded int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM article_ai ai JOIN articles a ON a.id = ai.article_id WHERE a.feed_id = $1 AND ai.embedding IS NOT NULL`, f.ID).Scan(&embedded)
	if n < 3 || embedded != 3 {
		t.Fatalf("sweep embedded %d, db has %d", n, embedded)
	}

	// API surface
	srv := httptest.NewServer(httpapi.NewRouter(httpapi.Deps{
		Auth: auth.NewAuthenticator(secret, q), DB: pool, Queries: q, Ingest: svc, AIEnabled: true,
	}))
	defer srv.Close()
	get := func(path string, out any) {
		t.Helper()
		req, _ := http.NewRequest("GET", srv.URL+path, bytes.NewReader(nil))
		req.AddCookie(&http.Cookie{Name: "better-auth.session_token", Value: auth.SignValue(token, []byte(secret))})
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}

	var list struct {
		Articles []struct {
			ID     int64
			Title  string
			TLDR   string   `json:"tldr"`
			Topics []string `json:"topics"`
		}
	}
	get("/v1/articles", &list)
	withTLDR := 0
	for _, a := range list.Articles {
		if a.TLDR != "" {
			withTLDR++
			if len(a.Topics) != 2 || a.Topics[0] != "Programming" || a.Topics[1] != "AI & ML" {
				t.Errorf("topics = %v", a.Topics)
			}
		}
	}
	if len(list.Articles) != 3 || withTLDR != 2 {
		t.Fatalf("articles=%d withTLDR=%d", len(list.Articles), withTLDR)
	}

	var detail struct {
		Article struct {
			AI struct {
				Status         string
				TLDR           string   `json:"tldr"`
				KeyPoints      []string `json:"keyPoints"`
				ReadingMinutes int      `json:"readingMinutes"`
			} `json:"ai"`
		}
	}
	get(fmt.Sprintf("/v1/articles/%d", list.Articles[0].ID), &detail)
	if d := detail.Article.AI; d.Status != "done" || len(d.KeyPoints) != 2 || d.ReadingMinutes < 1 {
		t.Errorf("detail ai = %+v", d)
	}

	var topics struct {
		Topics []struct {
			Name  string
			Count int
		}
	}
	get("/v1/topics", &topics)
	if len(topics.Topics) < 2 {
		t.Errorf("topics = %+v", topics)
	}
	var filtered struct{ Articles []struct{ ID int64 } }
	get("/v1/articles?topic=AI%20%26%20ML", &filtered)
	if len(filtered.Articles) != 2 {
		t.Errorf("topic filter returned %d, want 2", len(filtered.Articles))
	}
	var me struct{ Features map[string]bool }
	get("/v1/me", &me)
	if !me.Features["ai"] {
		t.Error("features.ai should be true")
	}
}

// A provider rate limit must defer the job without consuming an attempt.
func TestRateLimitedJobIsDeferredNotFailed(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	// Isolate: this test owns the queue for the kind it uses.
	const kind = "test_defer_kind"
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO jobs (kind) VALUES ($1) RETURNING id`, kind).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id = $1`, id) })

	q := store.New(pool)
	runner := jobs.NewRunner(q, 20*time.Millisecond)
	runner.Register(kind, func(context.Context, store.Job) error {
		return jobs.Defer(time.Hour, &llm.ErrRateLimited{RetryAfter: time.Hour})
	})
	runCtx, stop := context.WithTimeout(ctx, 600*time.Millisecond)
	defer stop()
	runner.Run(runCtx, 1)

	var attempts int
	var runAt time.Time
	var doneAt *time.Time
	_ = pool.QueryRow(ctx, `SELECT attempts, run_at, done_at FROM jobs WHERE id = $1`, id).Scan(&attempts, &runAt, &doneAt)
	if attempts != 0 || doneAt != nil || time.Until(runAt) < 50*time.Minute {
		t.Fatalf("attempts=%d doneAt=%v runAt=%v", attempts, doneAt, runAt)
	}
}

func waitFor(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
