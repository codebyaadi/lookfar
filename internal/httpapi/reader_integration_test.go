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
	"testing"
	"time"

	"github.com/codebyaadi/feedoku/internal/auth"
	"github.com/codebyaadi/feedoku/internal/feed"
	"github.com/codebyaadi/feedoku/internal/httpapi"
	"github.com/codebyaadi/feedoku/internal/ingest"
	"github.com/codebyaadi/feedoku/internal/postgres"
	"github.com/codebyaadi/feedoku/internal/store"
)

const secret = "integration-test-secret-0123456789"

func rssFor(n int) string {
	var items strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&items, `<item><title>Post %d</title><link>https://blog.test/p%d</link><guid>g%d</guid>
<pubDate>%s</pubDate><description>%s</description></item>`,
			i, i, i, time.Now().Add(-time.Duration(i)*time.Hour).Format(time.RFC1123Z), strings.Repeat("word ", 150))
	}
	return `<?xml version="1.0"?><rss version="2.0"><channel><title>Test Blog</title><link>https://blog.test/</link>` + items.String() + `</channel></rss>`
}

func TestReaderFlow(t *testing.T) {
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

	suffix := fmt.Sprint(time.Now().UnixNano())
	userID, token := "u"+suffix, "tok"+suffix
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
			fmt.Fprint(w, "<html><body>home</body></html>")
		case "/rss.xml":
			fmt.Fprint(w, rssFor(5))
		default:
			http.NotFound(w, r)
		}
	}))
	defer feedSrv.Close()

	q := store.New(pool)
	svc := ingest.New(q, feed.NewFetcher(true))
	srv := httptest.NewServer(httpapi.NewRouter(httpapi.Deps{
		Auth: auth.NewAuthenticator(secret, q), DB: pool, Queries: q, Ingest: svc,
	}))
	defer srv.Close()

	cookie := &http.Cookie{Name: "better-auth.session_token", Value: auth.SignValue(token, []byte(secret))}
	do := func(method, path string, body any, out any) int {
		t.Helper()
		var rd *bytes.Reader
		switch b := body.(type) {
		case nil:
			rd = bytes.NewReader(nil)
		case []byte:
			rd = bytes.NewReader(b)
		default:
			raw, _ := json.Marshal(b)
			rd = bytes.NewReader(raw)
		}
		req, _ := http.NewRequest(method, srv.URL+path, rd)
		req.AddCookie(cookie)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if out != nil {
			_ = json.NewDecoder(resp.Body).Decode(out)
		}
		return resp.StatusCode
	}

	// Unauthenticated requests are rejected.
	if resp, _ := http.Get(srv.URL + "/v1/feeds"); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", resp.StatusCode)
	}

	// Add by site root: autodiscovery falls back to /rss.xml.
	var added struct{ Feed struct{ ID int64 } }
	if code := do("POST", "/v1/feeds", map[string]string{"url": feedSrv.URL}, &added); code != http.StatusCreated {
		t.Fatalf("add feed status = %d", code)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM feeds WHERE id = $1`, added.Feed.ID) })

	var feeds struct {
		Feeds []struct{ ID, UnreadCount int64 }
	}
	do("GET", "/v1/feeds", nil, &feeds)
	if len(feeds.Feeds) != 1 || feeds.Feeds[0].UnreadCount != 5 {
		t.Fatalf("feeds = %+v", feeds)
	}

	// Pagination: 2 per page.
	type page struct {
		Articles []struct {
			ID    int64
			Title string
			Read  bool
		}
		NextCursor *string
	}
	var p1, p2, p3 page
	do("GET", "/v1/articles?limit=2", nil, &p1)
	if len(p1.Articles) != 2 || p1.NextCursor == nil || p1.Articles[0].Title != "Post 1" {
		t.Fatalf("page1 = %+v", p1)
	}
	do("GET", "/v1/articles?limit=2&cursor="+*p1.NextCursor, nil, &p2)
	do("GET", "/v1/articles?limit=2&cursor="+*p2.NextCursor, nil, &p3)
	if len(p2.Articles) != 2 || len(p3.Articles) != 1 || p3.NextCursor != nil || p2.Articles[0].ID == p1.Articles[0].ID {
		t.Fatalf("pagination broken: %+v %+v %+v", p1, p2, p3)
	}

	// State: read + save, then filters.
	id := p1.Articles[0].ID
	var st struct{ Read, Saved bool }
	do("POST", fmt.Sprintf("/v1/articles/%d/state", id), map[string]any{"read": true, "saved": true}, &st)
	if !st.Read || !st.Saved {
		t.Fatalf("state = %+v", st)
	}
	var unread, saved page
	do("GET", "/v1/articles?filter=unread", nil, &unread)
	do("GET", "/v1/articles?filter=saved", nil, &saved)
	if len(unread.Articles) != 4 || len(saved.Articles) != 1 {
		t.Fatalf("unread=%d saved=%d", len(unread.Articles), len(saved.Articles))
	}
	// A partial update must not clobber the other fields.
	do("POST", fmt.Sprintf("/v1/articles/%d/state", id), map[string]any{"read": false}, &st)
	if st.Read || !st.Saved {
		t.Fatalf("partial update clobbered state: %+v", st)
	}

	do("POST", "/v1/articles/read-all", nil, nil)
	do("GET", "/v1/articles?filter=unread", nil, &unread)
	if len(unread.Articles) != 0 {
		t.Fatalf("read-all left %d unread", len(unread.Articles))
	}

	// Re-polling is idempotent.
	f, _ := q.GetFeed(ctx, added.Feed.ID)
	svc.Poll(ctx, f)
	var all page
	do("GET", "/v1/articles?limit=100", nil, &all)
	if len(all.Articles) != 5 {
		t.Fatalf("duplicate articles after re-poll: %d", len(all.Articles))
	}

	// OPML import (one good feed, one dead).
	opml := fmt.Sprintf(`<opml version="2.0"><body><outline text="News"><outline text="x" xmlUrl="%s/rss.xml"/><outline text="y" xmlUrl="%s/missing"/></outline></body></opml>`, feedSrv.URL, feedSrv.URL)
	var imp struct {
		Imported int
		Failed   []string
	}
	if code := do("POST", "/v1/feeds/opml", []byte(opml), &imp); code != http.StatusOK || imp.Imported != 1 || len(imp.Failed) != 1 {
		t.Fatalf("opml = %d %+v", code, imp)
	}

	// Unsubscribe.
	if code := do("DELETE", fmt.Sprintf("/v1/feeds/%d", added.Feed.ID), nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete status = %d", code)
	}
	if code := do("GET", fmt.Sprintf("/v1/articles/%d", id), nil, nil); code != http.StatusNotFound {
		t.Fatalf("article visible after unsubscribe: %d", code)
	}
}
