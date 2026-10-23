package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// Result is what a check found, already normalised to lines of text.
type Result struct {
	Title        string
	Content      string
	ETag         string
	LastModified string
	// NotModified means the server confirmed nothing changed (HTTP 304).
	NotModified bool
}

// Checker fetches a source and normalises what it finds.
type Checker struct {
	fetcher *Fetcher
	// githubAPI is the REST API root; overridable for tests.
	githubAPI   string
	githubWeb   string // github.com, for the tags Atom feed
	githubToken string
}

func NewChecker(f *Fetcher, githubToken string) *Checker {
	return &Checker{fetcher: f, githubAPI: "https://api.github.com", githubWeb: "https://github.com", githubToken: githubToken}
}

// Check fetches the source. Pass the stored validators to allow a 304.
func (c *Checker) Check(ctx context.Context, kind, rawURL string, cfg Config, etag, lastModified string) (*Result, error) {
	var (
		res *Result
		err error
	)
	switch kind {
	case KindPage:
		res, err = c.checkPage(ctx, rawURL, cfg, etag, lastModified)
	case KindRSS:
		res, err = c.checkFeed(ctx, rawURL, cfg, etag, lastModified)
	case KindGitHub:
		res, err = c.checkGitHub(ctx, rawURL, cfg, etag)
	default:
		return nil, fmt.Errorf("unknown kind %q", kind)
	}
	if errors.Is(err, ErrNotModified) {
		return &Result{NotModified: true, ETag: etag, LastModified: lastModified}, nil
	}
	return res, err
}

func (c *Checker) checkPage(ctx context.Context, rawURL string, cfg Config, etag, lastModified string) (*Result, error) {
	resp, err := c.fetcher.Do(ctx, rawURL, GetOptions{ETag: etag, LastModified: lastModified, Accept: "text/html, application/xhtml+xml;q=0.9, */*;q=0.5"})
	if err != nil {
		return nil, err
	}
	title, text, err := NormalizePage(resp.Body, cfg)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("no readable text found on the page")
	}
	return &Result{Title: title, Content: text, ETag: resp.ETag, LastModified: resp.LastModified}, nil
}

func (c *Checker) checkFeed(ctx context.Context, rawURL string, cfg Config, etag, lastModified string) (*Result, error) {
	resp, err := c.fetcher.Do(ctx, rawURL, GetOptions{ETag: etag, LastModified: lastModified})
	if err != nil {
		return nil, err
	}
	parsed, err := Parse(resp.Body, resp.FinalURL)
	if err != nil {
		return nil, fmt.Errorf("not a valid feed: %w", err)
	}

	items := parsed.Items
	// Newest first, then by URL, so a feed that reshuffles its order does not
	// look like a change.
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].PublishedAt.Equal(items[j].PublishedAt) {
			return items[i].PublishedAt.After(items[j].PublishedAt)
		}
		return items[i].URL < items[j].URL
	})
	if n := cfg.itemLimit(); len(items) > n {
		items = items[:n]
	}
	lines := make([]string, 0, len(items))
	for _, it := range items {
		line := oneLine(it.Title)
		if it.URL != "" {
			line += " | " + it.URL
		}
		if !it.PublishedAt.IsZero() {
			line += " | " + it.PublishedAt.UTC().Format("2006-01-02")
		}
		lines = append(lines, line)
	}
	return &Result{Title: parsed.Title, Content: strings.Join(lines, "\n"), ETag: resp.ETag, LastModified: resp.LastModified}, nil
}

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
}

func (c *Checker) checkGitHub(ctx context.Context, rawURL string, cfg Config, etag string) (*Result, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	owner, repo, err := githubRepo(u)
	if err != nil {
		return nil, err
	}

	header := http.Header{"X-GitHub-Api-Version": {"2022-11-28"}}
	if c.githubToken != "" {
		header.Set("Authorization", "Bearer "+c.githubToken)
	}
	api := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d", c.githubAPI, url.PathEscape(owner), url.PathEscape(repo), min(cfg.itemLimit(), 100))
	resp, err := c.fetcher.Do(ctx, api, GetOptions{ETag: etag, Accept: "application/vnd.github+json", Header: header})
	if err != nil {
		return nil, err
	}

	var releases []githubRelease
	if err := json.Unmarshal(resp.Body, &releases); err != nil {
		return nil, fmt.Errorf("unexpected GitHub response: %w", err)
	}
	lines := make([]string, 0, len(releases))
	for _, r := range releases {
		if r.Draft || (r.Prerelease && !cfg.IncludePrereleases) {
			continue
		}
		name := oneLine(r.Name)
		if name == "" || name == r.TagName {
			name = ""
		} else {
			name = " | " + name
		}
		lines = append(lines, fmt.Sprintf("%s%s | %s | %s", r.TagName, name, datePart(r.PublishedAt), r.HTMLURL))
	}
	if len(lines) > 0 {
		return &Result{Title: owner + "/" + repo + " releases", Content: strings.Join(lines, "\n"), ETag: resp.ETag}, nil
	}

	// Many projects (Go, for one) tag versions without publishing releases.
	tags, err := c.githubTags(ctx, owner, repo, cfg)
	if err != nil {
		return nil, err
	}
	if tags == "" {
		return nil, errors.New("this repository has no releases or tags to watch")
	}
	return &Result{Title: owner + "/" + repo + " tags", Content: tags, ETag: resp.ETag}, nil
}

// githubTags lists a repository's newest tags from its tags.atom feed. The
// REST tags endpoint is not ordered by recency, so it cannot answer "what is
// newest"; the Atom feed can, and it does not use API rate limit.
func (c *Checker) githubTags(ctx context.Context, owner, repo string, cfg Config) (string, error) {
	feedURL := fmt.Sprintf("%s/%s/%s/tags.atom", c.githubWeb, url.PathEscape(owner), url.PathEscape(repo))
	resp, err := c.fetcher.Do(ctx, feedURL, GetOptions{Accept: "application/atom+xml, */*;q=0.5"})
	if err != nil {
		return "", err
	}
	parsed, err := Parse(resp.Body, resp.FinalURL)
	if err != nil {
		return "", nil // no tags feed: treat as no tags
	}
	items := parsed.Items
	sort.SliceStable(items, func(i, j int) bool { return items[i].PublishedAt.After(items[j].PublishedAt) })
	if n := cfg.itemLimit(); len(items) > n {
		items = items[:n]
	}
	lines := make([]string, 0, len(items))
	for _, it := range items {
		lines = append(lines, fmt.Sprintf("%s | %s | %s", oneLine(it.Title), it.PublishedAt.UTC().Format("2006-01-02"), it.URL))
	}
	return strings.Join(lines, "\n"), nil
}

func oneLine(s string) string { return strings.TrimSpace(spaceRe.ReplaceAllString(s, " ")) }

func datePart(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}
