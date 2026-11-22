// Package advisory holds the advisory model and the collectors that fill it
// (CISA KEV now; OSV and NVD later).
package advisory

import (
	"context"
	"encoding/json"
	"time"
)

// Advisory mirrors a row of the advisories table.
type Advisory struct {
	ID             string // canonical id, for example CVE-2026-1234
	Aliases        []string
	Summary        string
	Severity       string // unknown, low, medium, high, critical
	CVSS           *float64
	Exploited      bool
	ExploitedSince *time.Time
	Affected       json.RawMessage // jsonb array; elements carry a "source" tag
	References     json.RawMessage // jsonb array; stored in the refs column
	ModifiedAt     *time.Time
}

// Result summarizes one collector run.
type Result struct {
	Fetched     int  // entries in the feed
	New         int  // entries not seen before
	Updated     int  // entries whose content changed
	Unchanged   int  // entries identical to what is stored (all of them on a 304)
	NotModified bool // the server answered 304
}

// Stats returns the result as run-log stats.
func (r Result) Stats() map[string]any {
	return map[string]any{
		"fetched": r.Fetched, "new": r.New, "updated": r.Updated,
		"unchanged": r.Unchanged, "not_modified": r.NotModified,
	}
}

// Collector is implemented by every advisory source.
type Collector interface {
	// Name identifies the source, for example "cisa-kev".
	Name() string
	// Collect fetches and stores new data. since is the time of the last
	// successful run (zero if never); sources with their own cursor may ignore it.
	Collect(ctx context.Context, since time.Time) (Result, error)
}
