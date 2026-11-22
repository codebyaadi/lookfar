package advisory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/codebyaadi/lookfar/internal/store"
)

// KEVURL is CISA's official Known Exploited Vulnerabilities catalog.
const KEVURL = "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"

// KEV collects the CISA Known Exploited Vulnerabilities catalog.
type KEV struct {
	Pool    *pgxpool.Pool
	Fetcher *Fetcher
	URL     string // defaults to KEVURL
}

// Name implements Collector.
func (*KEV) Name() string { return "cisa-kev" }

// kevEntry is one catalog item. Unknown fields are kept in the raw payload.
type kevEntry struct {
	CVEID                      string   `json:"cveID"`
	VendorProject              string   `json:"vendorProject"`
	Product                    string   `json:"product"`
	VulnerabilityName          string   `json:"vulnerabilityName"`
	DateAdded                  string   `json:"dateAdded"`
	ShortDescription           string   `json:"shortDescription"`
	RequiredAction             string   `json:"requiredAction"`
	DueDate                    string   `json:"dueDate"`
	KnownRansomwareCampaignUse string   `json:"knownRansomwareCampaignUse"`
	Notes                      string   `json:"notes"`
	CWEs                       []string `json:"cwes"`
}

type kevCursor struct {
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
}

// Collect fetches the catalog (conditionally) and stores it. It implements Collector.
func (k *KEV) Collect(ctx context.Context, _ time.Time) (Result, error) {
	q := store.New(k.Pool)
	src, err := q.UpsertSource(ctx, store.UpsertSourceParams{Kind: "kev", Name: k.Name(), Schedule: "@daily"})
	if err != nil {
		return Result{}, fmt.Errorf("register source: %w", err)
	}

	res, cursor, err := k.collect(ctx, q, src)
	finish := store.FinishSourceRunParams{ID: src.ID, Ok: err == nil}
	if err != nil {
		finish.Error = pgtype.Text{String: err.Error(), Valid: true}
	} else if cursor != nil {
		finish.Cursor = cursor
	}
	// Record the outcome even if the run context is already done.
	if ferr := q.FinishSourceRun(context.WithoutCancel(ctx), finish); ferr != nil && err == nil {
		err = fmt.Errorf("record source run: %w", ferr)
	}
	return res, err
}

func (k *KEV) collect(ctx context.Context, q *store.Queries, src store.Source) (Result, []byte, error) {
	var cur kevCursor
	_ = json.Unmarshal(src.Cursor, &cur)

	url := k.URL
	if url == "" {
		url = KEVURL
	}
	hdr := http.Header{}
	if cur.ETag != "" {
		hdr.Set("If-None-Match", cur.ETag)
	}
	if cur.LastModified != "" {
		hdr.Set("If-Modified-Since", cur.LastModified)
	}
	resp, err := k.Fetcher.Get(ctx, url, hdr)
	if err != nil {
		return Result{}, nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		n, err := q.CountRawItems(ctx, src.ID)
		if err != nil {
			return Result{}, nil, err
		}
		return Result{NotModified: true, Unchanged: int(n)}, nil, nil
	case http.StatusOK:
	default:
		return Result{}, nil, fmt.Errorf("GET %s: unexpected status %s", url, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return Result{}, nil, fmt.Errorf("read feed: %w", err)
	}
	entries, err := parseKEV(body)
	if err != nil {
		return Result{}, nil, err
	}

	res, err := k.store(ctx, src.ID, entries)
	if err != nil {
		return res, nil, err
	}
	next, _ := json.Marshal(kevCursor{ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified")})
	return res, next, nil
}

// kevRaw pairs a parsed entry with its untouched JSON, which is what raw_items stores.
type kevRaw struct {
	Entry kevEntry
	JSON  json.RawMessage
}

// parseKEV decodes the catalog JSON, skipping entries without a CVE id.
func parseKEV(data []byte) ([]kevRaw, error) {
	var doc struct {
		Vulnerabilities []json.RawMessage `json:"vulnerabilities"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse KEV feed: %w", err)
	}
	if doc.Vulnerabilities == nil {
		return nil, errors.New("parse KEV feed: no vulnerabilities array")
	}
	out := make([]kevRaw, 0, len(doc.Vulnerabilities))
	for _, raw := range doc.Vulnerabilities {
		var e kevEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("parse KEV entry: %w", err)
		}
		if strings.TrimSpace(e.CVEID) == "" {
			continue
		}
		out = append(out, kevRaw{Entry: e, JSON: raw})
	}
	return out, nil
}

// store writes every entry in one transaction.
func (k *KEV) store(ctx context.Context, sourceID int64, entries []kevRaw) (Result, error) {
	res := Result{Fetched: len(entries)}
	tx, err := k.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := store.New(tx)

	for _, r := range entries {
		added := parseDate(r.Entry.DateAdded)
		inserted, err := q.UpsertRawItem(ctx, store.UpsertRawItemParams{
			SourceID: sourceID, ExternalID: r.Entry.CVEID, Payload: r.JSON, PublishedAt: added,
		})
		switch {
		case errors.Is(err, pgx.ErrNoRows): // payload identical: nothing to do
			res.Unchanged++
			continue
		case err != nil:
			return res, fmt.Errorf("store raw item %s: %w", r.Entry.CVEID, err)
		case inserted:
			res.New++
		default:
			res.Updated++
		}

		adv := normalizeKEV(r.Entry)
		if err := q.UpsertKEVAdvisory(ctx, store.UpsertKEVAdvisoryParams{
			ID: adv.ID, Summary: adv.Summary, ExploitedSince: adv.ExploitedSince,
			Affected: adv.Affected, Refs: adv.References,
		}); err != nil {
			return res, fmt.Errorf("store advisory %s: %w", adv.ID, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return res, err
	}
	return res, nil
}

// normalizeKEV maps a catalog entry to the KEV-owned view of an advisory.
// Array elements are tagged source=kev so other sources can merge beside them.
func normalizeKEV(e kevEntry) Advisory {
	summary := strings.TrimSpace(e.VulnerabilityName)
	if d := strings.TrimSpace(e.ShortDescription); d != "" {
		if summary != "" {
			summary += ": "
		}
		summary += d
	}

	affected, _ := json.Marshal([]map[string]any{{
		"source": "kev", "vendorProject": e.VendorProject, "product": e.Product,
	}})
	ref := map[string]any{
		"source": "kev", "type": "kev",
		"url":                        "https://nvd.nist.gov/vuln/detail/" + e.CVEID,
		"requiredAction":             e.RequiredAction,
		"dueDate":                    e.DueDate,
		"knownRansomwareCampaignUse": e.KnownRansomwareCampaignUse,
		"notes":                      e.Notes,
	}
	if len(e.CWEs) > 0 {
		ref["cwes"] = e.CWEs
	}
	refs, _ := json.Marshal([]map[string]any{ref})

	return Advisory{
		ID: e.CVEID, Summary: summary, Exploited: true,
		ExploitedSince: parseDate(e.DateAdded), Affected: affected, References: refs,
	}
}

func parseDate(s string) *time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}
