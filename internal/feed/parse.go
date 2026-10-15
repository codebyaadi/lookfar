package feed

import (
	"bytes"
	"errors"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/mmcdole/gofeed"
)

type Item struct {
	GUID        string
	URL         string
	Title       string
	Author      string
	PublishedAt time.Time
	Summary     string // plain text excerpt
	ContentHTML string // sanitized
	ContentText string
	ImageURL    string
}

type Parsed struct {
	Title       string
	Description string
	SiteURL     string
	Items       []Item
}

var (
	policy  = newPolicy()
	tagRe   = regexp.MustCompile(`<[^>]*>`)
	spaceRe = regexp.MustCompile(`\s+`)
)

func newPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("loading").OnElements("img")
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}

// Parse parses RSS, Atom or JSON Feed bytes. base is the feed URL, used to
// resolve relative links.
func Parse(body []byte, base string) (*Parsed, error) {
	f, err := gofeed.NewParser().Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if len(f.Items) == 0 && f.Title == "" {
		return nil, errors.New("feed is empty")
	}
	p := &Parsed{
		Title:       strings.TrimSpace(f.Title),
		Description: PlainText(f.Description),
		SiteURL:     resolve(base, f.Link),
	}
	for _, it := range f.Items {
		if item, ok := convertItem(it, base); ok {
			p.Items = append(p.Items, item)
		}
	}
	return p, nil
}

func convertItem(it *gofeed.Item, base string) (Item, bool) {
	link := resolve(base, it.Link)
	guid := strings.TrimSpace(it.GUID)
	if guid == "" {
		guid = link
	}
	title := strings.TrimSpace(html.UnescapeString(it.Title))
	if guid == "" || (link == "" && title == "") {
		return Item{}, false
	}
	if title == "" {
		title = link
	}

	raw := it.Content
	if raw == "" {
		raw = it.Description
	}
	clean := SanitizeHTML(raw)
	text := PlainText(clean)

	published := time.Now().UTC()
	switch {
	case it.PublishedParsed != nil:
		published = it.PublishedParsed.UTC()
	case it.UpdatedParsed != nil:
		published = it.UpdatedParsed.UTC()
	}
	// Feeds sometimes emit dates in the future; never sort ahead of "now".
	if published.After(time.Now().Add(time.Hour)) {
		published = time.Now().UTC()
	}

	author := ""
	if it.Author != nil {
		author = strings.TrimSpace(it.Author.Name)
	}
	summary := PlainText(it.Description)
	if summary == "" {
		summary = text
	}
	return Item{
		GUID:        guid,
		URL:         link,
		Title:       title,
		Author:      author,
		PublishedAt: published,
		Summary:     truncate(summary, 600),
		ContentHTML: clean,
		ContentText: text,
		ImageURL:    imageFor(it, base),
	}, true
}

func imageFor(it *gofeed.Item, base string) string {
	if it.Image != nil && it.Image.URL != "" {
		return resolve(base, it.Image.URL)
	}
	for _, e := range it.Enclosures {
		if strings.HasPrefix(e.Type, "image/") && e.URL != "" {
			return resolve(base, e.URL)
		}
	}
	if m, ok := it.Extensions["media"]; ok {
		for _, key := range []string{"thumbnail", "content"} {
			for _, ext := range m[key] {
				if u := ext.Attrs["url"]; u != "" {
					return resolve(base, u)
				}
			}
		}
	}
	return ""
}

func SanitizeHTML(s string) string { return strings.TrimSpace(policy.Sanitize(s)) }

// PlainText strips tags and collapses whitespace.
func PlainText(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func resolve(base, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	r, err := b.Parse(ref)
	if err != nil || (r.Scheme != "http" && r.Scheme != "https") {
		return ""
	}
	return r.String()
}
