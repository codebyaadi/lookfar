package source

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testChecker(t *testing.T, apiRoot string) *Checker {
	t.Helper()
	c := NewChecker(NewFetcher(true), "")
	if apiRoot != "" {
		c.githubAPI = apiRoot
	}
	return c
}

func TestCheckPageConditionalGet(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		fmt.Fprint(w, `<html><head><title>T</title></head><body><p>Hello world</p></body></html>`)
	}))
	defer srv.Close()

	c := testChecker(t, "")
	res, err := c.Check(context.Background(), KindPage, srv.URL, Config{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "T" || res.Content != "Hello world" || res.ETag != `"v1"` {
		t.Fatalf("unexpected result: %+v", res)
	}

	res, err = c.Check(context.Background(), KindPage, srv.URL, Config{}, `"v1"`, "")
	if err != nil {
		t.Fatal(err)
	}
	if !res.NotModified || res.ETag != `"v1"` {
		t.Fatalf("want a 304 to surface as NotModified with the etag kept: %+v", res)
	}
}

func TestCheckPageErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/empty":
			fmt.Fprint(w, `<html><body><script>x()</script></body></html>`)
		default:
			http.Error(w, "nope", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c := testChecker(t, "")
	if _, err := c.Check(context.Background(), KindPage, srv.URL+"/boom", Config{}, "", ""); err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("want a status error, got %v", err)
	}
	if _, err := c.Check(context.Background(), KindPage, srv.URL+"/empty", Config{}, "", ""); err == nil || !strings.Contains(err.Error(), "no readable text") {
		t.Errorf("want a no-text error, got %v", err)
	}
}

func TestCheckPageRefusesPrivateAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "secret") }))
	defer srv.Close()

	guarded := NewChecker(NewFetcher(false), "")
	if _, err := guarded.Check(context.Background(), KindPage, srv.URL, Config{}, "", ""); err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("the SSRF guard should block a loopback source, got %v", err)
	}
}

func TestCheckFeedOrderIsStable(t *testing.T) {
	feed := func(items ...string) string {
		return `<?xml version="1.0"?><rss version="2.0"><channel><title>Blog</title>` + strings.Join(items, "") + `</channel></rss>`
	}
	item := func(title, day string) string {
		return fmt.Sprintf(`<item><title>%s</title><link>https://example.com/%s</link><guid>%s</guid><pubDate>%s 2026 10:00:00 GMT</pubDate></item>`,
			title, strings.ToLower(title), title, day)
	}
	body := feed(item("Alpha", "Mon, 05 Jan"), item("Beta", "Tue, 06 Jan"))
	swapped := feed(item("Beta", "Tue, 06 Jan"), item("Alpha", "Mon, 05 Jan"))
	cur := &body
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, *cur) }))
	defer srv.Close()

	c := testChecker(t, "")
	a, err := c.Check(context.Background(), KindRSS, srv.URL, Config{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	cur = &swapped
	b, err := c.Check(context.Background(), KindRSS, srv.URL, Config{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Content != b.Content {
		t.Fatalf("reordering the feed must not change the content:\n%s\n---\n%s", a.Content, b.Content)
	}
	if a.Title != "Blog" || !strings.HasPrefix(a.Content, "Beta | https://example.com/beta | 2026-01-06") {
		t.Fatalf("newest item should come first: %q", a.Content)
	}

	more := feed(item("Alpha", "Mon, 05 Jan"), item("Beta", "Tue, 06 Jan"), item("Gamma", "Wed, 07 Jan"))
	cur = &more
	g, _ := c.Check(context.Background(), KindRSS, srv.URL, Config{MaxItems: 2}, "", "")
	if n := len(strings.Split(g.Content, "\n")); n != 2 {
		t.Fatalf("maxItems=2 should keep two lines, got %d:\n%s", n, g.Content)
	}
}

func TestCheckFeedRejectsNonFeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<html><body>hi</body></html>") }))
	defer srv.Close()
	if _, err := testChecker(t, "").Check(context.Background(), KindRSS, srv.URL, Config{}, "", ""); err == nil || !strings.Contains(err.Error(), "not a valid feed") {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckGitHubReleases(t *testing.T) {
	var gotAuth, gotAccept, gotPath string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotAccept, gotPath = r.Header.Get("Authorization"), r.Header.Get("Accept"), r.URL.Path
		fmt.Fprint(w, `[
		 {"tag_name":"v2.0.0-rc1","name":"RC","html_url":"https://github.com/o/r/releases/tag/v2.0.0-rc1","published_at":"2026-10-20T10:00:00Z","prerelease":true},
		 {"tag_name":"v1.5.0","name":"Big release","html_url":"https://github.com/o/r/releases/tag/v1.5.0","published_at":"2026-10-01T09:00:00Z"},
		 {"tag_name":"v1.4.9","name":"v1.4.9","html_url":"https://github.com/o/r/releases/tag/v1.4.9","published_at":"2026-09-20T09:00:00Z"},
		 {"tag_name":"v1.6.0-draft","name":"","html_url":"x","published_at":"2026-10-25T09:00:00Z","draft":true}
		]`)
	}))
	defer api.Close()

	c := NewChecker(NewFetcher(true), "ghp_test")
	c.githubAPI = api.URL
	res, err := c.Check(context.Background(), KindGitHub, "https://github.com/o/r", Config{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/repos/o/r/releases" || gotAuth != "Bearer ghp_test" || gotAccept != "application/vnd.github+json" {
		t.Errorf("request = path %q auth %q accept %q", gotPath, gotAuth, gotAccept)
	}
	want := "v1.5.0 | Big release | 2026-10-01 | https://github.com/o/r/releases/tag/v1.5.0\nv1.4.9 | 2026-09-20 | https://github.com/o/r/releases/tag/v1.4.9"
	if res.Content != want {
		t.Fatalf("content:\n%s\nwant:\n%s", res.Content, want)
	}
	if res.Title != "o/r releases" {
		t.Errorf("title = %q", res.Title)
	}

	withPre, _ := c.Check(context.Background(), KindGitHub, "https://github.com/o/r", Config{IncludePrereleases: true}, "", "")
	if !strings.HasPrefix(withPre.Content, "v2.0.0-rc1") {
		t.Errorf("prereleases should be included on request:\n%s", withPre.Content)
	}
}

func TestCheckGitHubFallsBackToTags(t *testing.T) {
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>Tags from o/r</title>
<entry><id>t1</id><updated>2026-03-01T10:00:00Z</updated><link rel="alternate" href="https://github.com/o/r/releases/tag/go1.26.2"/><title>go1.26.2</title></entry>
<entry><id>t2</id><updated>2026-04-02T10:00:00Z</updated><link rel="alternate" href="https://github.com/o/r/releases/tag/go1.26.3"/><title>go1.26.3</title></entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/o/r/releases", "/api/repos/o/empty/releases":
			fmt.Fprint(w, `[]`)
		case "/web/o/r/tags.atom":
			fmt.Fprint(w, atom)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewChecker(NewFetcher(true), "")
	c.githubAPI, c.githubWeb = srv.URL+"/api", srv.URL+"/web"
	res, err := c.Check(context.Background(), KindGitHub, "https://github.com/o/r", Config{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	want := "go1.26.3 | 2026-04-02 | https://github.com/o/r/releases/tag/go1.26.3\ngo1.26.2 | 2026-03-01 | https://github.com/o/r/releases/tag/go1.26.2"
	if res.Content != want || res.Title != "o/r tags" {
		t.Fatalf("tags fallback: %q\n%s", res.Title, res.Content)
	}

	if _, err := c.Check(context.Background(), KindGitHub, "https://github.com/o/empty", Config{}, "", ""); err == nil {
		t.Fatalf("a repository with neither releases nor tags should be an error")
	}
}
