package source

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

var commonPaths = []string{"/feed", "/feed.xml", "/rss", "/rss.xml", "/atom.xml", "/index.xml"}

// Discovered is a resolved feed together with its already-fetched body.
type Discovered struct {
	URL    string
	Parsed *Parsed
	Resp   *Response
}

// Discover resolves any URL to a feed: the URL itself if it is a feed,
// otherwise a <link rel="alternate"> from the page, otherwise common paths.
func (f *Fetcher) Discover(ctx context.Context, rawURL string) (*Discovered, error) {
	rawURL = normalizeURL(rawURL)
	resp, err := f.Get(ctx, rawURL, "", "")
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	if p, err := Parse(resp.Body, resp.FinalURL); err == nil {
		return &Discovered{URL: resp.FinalURL, Parsed: p, Resp: resp}, nil
	}

	candidates := linkAlternates(resp.Body, resp.FinalURL)
	if base, err := url.Parse(resp.FinalURL); err == nil {
		for _, p := range commonPaths {
			candidates = append(candidates, base.ResolveReference(&url.URL{Path: p}).String())
		}
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		if seen[c] {
			continue
		}
		seen[c] = true
		r, err := f.Get(ctx, c, "", "")
		if err != nil {
			continue
		}
		if p, err := Parse(r.Body, r.FinalURL); err == nil {
			return &Discovered{URL: r.FinalURL, Parsed: p, Resp: r}, nil
		}
	}
	return nil, errors.New("no feed found at that address")
}

func normalizeURL(s string) string {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	return s
}

func linkAlternates(body []byte, base string) []string {
	var out []string
	z := html.NewTokenizer(bytes.NewReader(body))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return out
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if t.Data != "link" {
				continue
			}
			var rel, typ, href string
			for _, a := range t.Attr {
				switch a.Key {
				case "rel":
					rel = strings.ToLower(a.Val)
				case "type":
					typ = strings.ToLower(a.Val)
				case "href":
					href = a.Val
				}
			}
			if rel == "alternate" && href != "" &&
				(strings.Contains(typ, "rss") || strings.Contains(typ, "atom") || typ == "application/feed+json") {
				if u := resolve(base, href); u != "" {
					out = append(out, u)
				}
			}
		}
	}
}
