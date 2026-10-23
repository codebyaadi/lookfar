package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/andybalholm/cascadia"
)

// Source kinds.
const (
	KindRSS    = "rss"
	KindPage   = "page"
	KindGitHub = "github"
)

const (
	maxListEntries = 20
	maxEntryLength = 200
	maxItemsCap    = 200
	defaultItems   = 50
)

// Config holds per-source options, stored as JSON. Fields that do not apply to
// a source's kind are ignored.
type Config struct {
	// Page: only watch the elements matching this CSS selector.
	Selector string `json:"selector,omitempty"`
	// Page: drop elements matching these CSS selectors before extracting text.
	IgnoreSelectors []string `json:"ignoreSelectors,omitempty"`
	// Page: drop any line matching one of these regular expressions.
	IgnorePatterns []string `json:"ignorePatterns,omitempty"`
	// Page: keep dates, clock times and "5 minutes ago" text. By default they
	// are stripped so a ticking timestamp does not count as a change.
	KeepTimestamps bool `json:"keepTimestamps,omitempty"`
	// Ignore changes smaller than this many changed characters.
	MinChangeChars int `json:"minChangeChars,omitempty"`
	// RSS and GitHub: how many newest items to track (default 50).
	MaxItems int `json:"maxItems,omitempty"`
	// GitHub: include pre-releases.
	IncludePrereleases bool `json:"includePrereleases,omitempty"`
}

// ParseConfig decodes stored config JSON. Empty input is the zero Config.
func ParseConfig(raw []byte) (Config, error) {
	var c Config
	if len(raw) == 0 {
		return c, nil
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return Config{}, fmt.Errorf("invalid config: %w", err)
	}
	return c, nil
}

// Validate checks that selectors and patterns compile and are within limits.
func (c Config) Validate(kind string) error {
	switch kind {
	case KindRSS, KindPage, KindGitHub:
	default:
		return fmt.Errorf("unknown kind %q", kind)
	}
	if c.MinChangeChars < 0 || c.MinChangeChars > 100_000 {
		return errors.New("minChangeChars must be between 0 and 100000")
	}
	if c.MaxItems < 0 || c.MaxItems > maxItemsCap {
		return fmt.Errorf("maxItems must be between 0 and %d", maxItemsCap)
	}
	if kind != KindPage {
		return nil
	}
	if c.Selector != "" {
		if len(c.Selector) > maxEntryLength {
			return errors.New("selector is too long")
		}
		if _, err := cascadia.ParseGroup(c.Selector); err != nil {
			return fmt.Errorf("invalid selector: %w", err)
		}
	}
	if len(c.IgnoreSelectors) > maxListEntries || len(c.IgnorePatterns) > maxListEntries {
		return fmt.Errorf("at most %d ignore selectors and %d ignore patterns", maxListEntries, maxListEntries)
	}
	for _, s := range c.IgnoreSelectors {
		if len(s) > maxEntryLength {
			return errors.New("ignore selector is too long")
		}
		if _, err := cascadia.ParseGroup(s); err != nil {
			return fmt.Errorf("invalid ignore selector %q: %w", s, err)
		}
	}
	for _, p := range c.IgnorePatterns {
		if len(p) > maxEntryLength {
			return errors.New("ignore pattern is too long")
		}
		if _, err := regexp.Compile(p); err != nil {
			return fmt.Errorf("invalid ignore pattern %q: %w", p, err)
		}
	}
	return nil
}

func (c Config) itemLimit() int {
	if c.MaxItems > 0 {
		return c.MaxItems
	}
	return defaultItems
}

// CanonicalURL validates rawURL for the kind and returns the form to store.
func CanonicalURL(kind, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if kind == KindGitHub && !strings.Contains(rawURL, "://") && strings.Count(rawURL, "/") == 1 {
		rawURL = "https://github.com/" + rawURL // "owner/repo" shorthand
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.New("invalid url")
	}
	if err := checkScheme(u); err != nil {
		return "", err
	}
	if len(rawURL) > 2048 {
		return "", errors.New("url is too long")
	}
	if kind == KindGitHub {
		owner, repo, err := githubRepo(u)
		if err != nil {
			return "", err
		}
		return "https://github.com/" + owner + "/" + repo, nil
	}
	u.Fragment = ""
	return u.String(), nil
}

// githubRepo extracts owner and repository from a github.com URL.
func githubRepo(u *url.URL) (owner, repo string, err error) {
	if h := strings.ToLower(u.Hostname()); h != "github.com" && h != "www.github.com" {
		return "", "", errors.New("GitHub sources must use a github.com URL")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("GitHub URL must look like https://github.com/owner/repo")
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}
