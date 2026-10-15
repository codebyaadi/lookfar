package feed

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const rssSample = `<?xml version="1.0"?>
<rss version="2.0"><channel>
<title>Example Blog</title><link>https://example.com/</link><description>Posts</description>
<item><title>Hello &amp; welcome</title><link>/hello</link><guid>post-1</guid>
<pubDate>Mon, 02 Jan 2006 15:04:05 GMT</pubDate>
<description><![CDATA[<p>Hi <script>alert(1)</script><b>there</b></p>]]></description></item>
</channel></rss>`

const atomSample = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>Atom Blog</title>
<link href="https://atom.example/"/>
<entry><title>Entry</title><id>urn:1</id><link href="https://atom.example/e1"/>
<updated>2024-05-01T10:00:00Z</updated><content type="html">&lt;p&gt;body&lt;/p&gt;</content></entry>
</feed>`

func TestParseRSSSanitizesAndResolves(t *testing.T) {
	p, err := Parse([]byte(rssSample), "https://example.com/feed")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Example Blog" || len(p.Items) != 1 {
		t.Fatalf("unexpected parse: %+v", p)
	}
	it := p.Items[0]
	if it.URL != "https://example.com/hello" {
		t.Errorf("url not resolved: %q", it.URL)
	}
	if it.Title != "Hello & welcome" {
		t.Errorf("title = %q", it.Title)
	}
	if strings.Contains(it.ContentHTML, "script") || !strings.Contains(it.ContentHTML, "<b>there</b>") {
		t.Errorf("html not sanitized: %q", it.ContentHTML)
	}
}

func TestParseAtom(t *testing.T) {
	p, err := Parse([]byte(atomSample), "https://atom.example/feed")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Items) != 1 || p.Items[0].GUID != "urn:1" || p.Items[0].PublishedAt.Year() != 2024 {
		t.Fatalf("unexpected: %+v", p.Items)
	}
}

func TestParseRejectsHTML(t *testing.T) {
	if _, err := Parse([]byte("<html><body>nope</body></html>"), "https://x.test"); err == nil {
		t.Fatal("expected error for non-feed")
	}
}

func TestDiscoverViaLinkTag(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><link rel="alternate" type="application/rss+xml" href="/custom.xml"></head></html>`)
	})
	mux.HandleFunc("/custom.xml", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, rssSample) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d, err := NewFetcher(true).Discover(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if d.URL != srv.URL+"/custom.xml" {
		t.Errorf("url = %s", d.URL)
	}
}

func TestDiscoverCommonPathFallback(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/feed.xml" {
			fmt.Fprint(w, atomSample)
			return
		}
		if r.URL.Path == "/" {
			fmt.Fprint(w, "<html></html>")
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	d, err := NewFetcher(true).Discover(context.Background(), srv.URL)
	if err != nil || !strings.HasSuffix(d.URL, "/feed.xml") {
		t.Fatalf("got %v, %v", d, err)
	}
}

func TestConditionalGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		fmt.Fprint(w, rssSample)
	}))
	defer srv.Close()
	f := NewFetcher(true)
	r, err := f.Get(context.Background(), srv.URL, "", "")
	if err != nil || r.ETag != `"v1"` {
		t.Fatalf("first fetch: %v %v", r, err)
	}
	if _, err := f.Get(context.Background(), srv.URL, r.ETag, ""); err != ErrNotModified {
		t.Fatalf("want ErrNotModified, got %v", err)
	}
}

func TestSSRFGuardBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, rssSample) }))
	defer srv.Close()
	if _, err := NewFetcher(false).Get(context.Background(), srv.URL, "", ""); err == nil {
		t.Fatal("expected loopback to be refused")
	}
	if _, err := NewFetcher(false).Get(context.Background(), "file:///etc/passwd", "", ""); err == nil {
		t.Fatal("expected file scheme to be refused")
	}
}
