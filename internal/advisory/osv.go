package advisory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/codebyaadi/lookfar/internal/store"
)

// OSVBaseURL is the public OSV API.
const OSVBaseURL = "https://api.osv.dev"

// osvBatchSize is OSV's documented limit for one querybatch call.
const osvBatchSize = 1000

// OSV collects advisories for the active components from OSV. OSV does the
// version matching: lookfar sends purl@version and gets back the IDs that apply.
type OSV struct {
	Pool        *pgxpool.Pool
	Fetcher     *Fetcher
	BaseURL     string // defaults to OSVBaseURL
	Concurrency int    // parallel detail fetches; defaults to 4
}

// Name implements Collector.
func (*OSV) Name() string { return "osv-dev" }

// OSVStats extends Result with OSV-specific counters.
type OSVStats struct {
	Result
	ComponentsQueried int
	ComponentsSkipped int
	VulnsFound        int
	DetailsFetched    int
	AdvisoriesNew     int
	AdvisoriesUpdated int
}

// Stats returns run-log stats.
func (s OSVStats) Stats() map[string]any {
	return map[string]any{
		"components_queried": s.ComponentsQueried, "components_skipped": s.ComponentsSkipped,
		"vulns_found": s.VulnsFound, "details_fetched": s.DetailsFetched,
		"advisories_new": s.AdvisoriesNew, "advisories_updated": s.AdvisoriesUpdated,
	}
}

// Collect implements Collector (the Result only carries the generic counters).
func (o *OSV) Collect(ctx context.Context, since time.Time) (Result, error) {
	st, err := o.CollectStats(ctx, since)
	return st.Result, err
}

type batchQuery struct {
	Package   *batchPackage `json:"package"`
	PageToken string        `json:"page_token,omitempty"`
}
type batchPackage struct {
	Purl string `json:"purl"`
}
type batchResponse struct {
	Results []struct {
		Vulns []struct {
			ID       string `json:"id"`
			Modified string `json:"modified"`
		} `json:"vulns"`
		NextPageToken string `json:"next_page_token"`
	} `json:"results"`
}

// CollectStats runs the collector and returns the detailed counters.
func (o *OSV) CollectStats(ctx context.Context, _ time.Time) (OSVStats, error) {
	var st OSVStats
	q := store.New(o.Pool)
	src, err := q.UpsertSource(ctx, store.UpsertSourceParams{Kind: "osv", Name: o.Name(), Schedule: "@every 12h"})
	if err != nil {
		return st, fmt.Errorf("register source: %w", err)
	}
	err = o.collect(ctx, q, src, &st)
	finish := store.FinishSourceRunParams{ID: src.ID, Ok: err == nil}
	if err != nil {
		finish.Error = pgtype.Text{String: err.Error(), Valid: true}
	}
	if ferr := q.FinishSourceRun(context.WithoutCancel(ctx), finish); ferr != nil && err == nil {
		err = fmt.Errorf("record source run: %w", ferr)
	}
	return st, err
}

func (o *OSV) collect(ctx context.Context, q *store.Queries, src store.Source, st *OSVStats) error {
	comps, err := q.ListOSVComponents(ctx)
	if err != nil {
		return fmt.Errorf("list components: %w", err)
	}
	total, err := q.CountActiveComponents(ctx)
	if err != nil {
		return err
	}
	st.ComponentsSkipped = int(total) - len(comps)

	// Several component rows can share a purl (same package in two manifests).
	byPurl := map[string][]int64{}
	var purls []string
	for _, c := range comps {
		if _, ok := byPurl[c.Purl]; !ok {
			purls = append(purls, c.Purl)
		}
		byPurl[c.Purl] = append(byPurl[c.Purl], c.ID)
	}
	st.ComponentsQueried = len(purls)
	if len(purls) == 0 {
		return nil
	}

	hits, modified, err := o.queryAll(ctx, purls)
	if err != nil {
		return err
	}
	st.VulnsFound = len(modified)

	// Decide which records need a detail fetch: new ones, or a changed modified time.
	ids := make([]string, 0, len(modified))
	for id := range modified {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	stored, err := q.ListRawItemsByIDs(ctx, store.ListRawItemsByIDsParams{SourceID: src.ID, Ids: ids})
	if err != nil {
		return fmt.Errorf("load stored items: %w", err)
	}
	payloads := map[string]json.RawMessage{}
	var toFetch []string
	storedModified := map[string]string{}
	for _, s := range stored {
		var m struct {
			Modified string `json:"modified"`
		}
		_ = json.Unmarshal(s.Payload, &m)
		storedModified[s.ExternalID] = m.Modified
		payloads[s.ExternalID] = s.Payload
	}
	for _, id := range ids {
		if m, ok := storedModified[id]; !ok || !sameModified(m, modified[id]) {
			toFetch = append(toFetch, id)
		}
	}

	fetched, err := o.fetchDetails(ctx, toFetch)
	if err != nil {
		return err
	}
	st.DetailsFetched = len(fetched)
	for id, p := range fetched {
		payloads[id] = p
	}

	return o.write(ctx, src.ID, st, payloads, fetched, hits, byPurl)
}

// queryAll posts every purl to querybatch in chunks, following pagination.
// It returns purl -> OSV ids and id -> modified.
func (o *OSV) queryAll(ctx context.Context, purls []string) (map[string][]string, map[string]string, error) {
	hits := map[string][]string{}
	modified := map[string]string{}

	pending := make([]batchQuery, len(purls))
	for i, p := range purls {
		pending[i] = batchQuery{Package: &batchPackage{Purl: p}}
	}
	for len(pending) > 0 {
		n := min(len(pending), osvBatchSize)
		chunk := pending[:n]
		pending = pending[n:]

		resp, err := o.post(ctx, "/v1/querybatch", map[string]any{"queries": chunk})
		if err != nil {
			return nil, nil, err
		}
		var br batchResponse
		if err := json.Unmarshal(resp, &br); err != nil {
			return nil, nil, fmt.Errorf("parse querybatch response: %w", err)
		}
		if len(br.Results) != len(chunk) {
			return nil, nil, fmt.Errorf("querybatch returned %d results for %d queries", len(br.Results), len(chunk))
		}
		for i, r := range br.Results {
			purl := chunk[i].Package.Purl
			for _, v := range r.Vulns {
				hits[purl] = append(hits[purl], v.ID)
				modified[v.ID] = v.Modified
			}
			if r.NextPageToken != "" {
				pending = append(pending, batchQuery{Package: chunk[i].Package, PageToken: r.NextPageToken})
			}
		}
	}
	return hits, modified, nil
}

func (o *OSV) post(ctx context.Context, path string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := o.Fetcher.Do(ctx, http.MethodPost, o.base()+path, http.Header{"Content-Type": {"application/json"}}, b)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("POST %s: %s: %s", path, resp.Status, bytes.TrimSpace(msg))
	}
	return io.ReadAll(io.LimitReader(resp.Body, 256<<20))
}

func (o *OSV) base() string {
	if o.BaseURL != "" {
		return o.BaseURL
	}
	return OSVBaseURL
}

// fetchDetails downloads full records with limited concurrency.
func (o *OSV) fetchDetails(ctx context.Context, ids []string) (map[string]json.RawMessage, error) {
	out := make(map[string]json.RawMessage, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	limit := o.Concurrency
	if limit <= 0 {
		limit = 4
	}
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)
	for _, id := range ids {
		g.Go(func() error {
			resp, err := o.Fetcher.Get(gctx, o.base()+"/v1/vulns/"+url.PathEscape(id), nil)
			if err != nil {
				return fmt.Errorf("fetch %s: %w", id, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusNotFound {
				return nil // withdrawn between the batch and now; skip
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("fetch %s: %s", id, resp.Status)
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
			if err != nil {
				return fmt.Errorf("read %s: %w", id, err)
			}
			if !json.Valid(body) {
				return fmt.Errorf("fetch %s: invalid JSON", id)
			}
			mu.Lock()
			out[id] = body
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

// write stores raw items and advisories for fetched records, then records
// every (component, advisory) hit, all in one transaction.
func (o *OSV) write(ctx context.Context, sourceID int64, st *OSVStats, payloads, fetched map[string]json.RawMessage,
	hits map[string][]string, byPurl map[string][]int64) error {

	tx, err := o.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := store.New(tx)

	canonical := map[string]string{} // OSV id -> advisory id
	for id, p := range payloads {
		var v osvVuln
		if err := json.Unmarshal(p, &v); err != nil {
			return fmt.Errorf("parse OSV record %s: %w", id, err)
		}
		adv := normalizeOSV(v)
		// Write-time guard: another row may already hold this vulnerability under a
		// different id (for example the CVE row that lists this GHSA as an alias).
		target, others, err := ResolveCanonical(ctx, q, adv.ID, adv.Aliases)
		if err != nil {
			return err
		}
		canonical[id] = target

		raw, isFetched := fetched[id]
		if !isFetched {
			if _, err := Absorb(ctx, q, target, others); err != nil {
				return err
			}
			continue
		}
		if _, err := q.UpsertRawItem(ctx, store.UpsertRawItemParams{
			SourceID: sourceID, ExternalID: id, Payload: raw, PublishedAt: adv.ModifiedAt,
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store raw item %s: %w", id, err)
		}
		cv := pgtype.Float8{}
		if adv.CVSS != nil {
			cv = pgtype.Float8{Float64: *adv.CVSS, Valid: true}
		}
		inserted, err := q.UpsertOSVAdvisory(ctx, store.UpsertOSVAdvisoryParams{
			ID: target, Aliases: withAlias(adv.Aliases, target, adv.ID), Summary: adv.Summary, Severity: adv.Severity, Cvss: cv,
			Affected: adv.Affected, Refs: adv.References, ModifiedAt: adv.ModifiedAt, OsvID: id,
		})
		if err != nil {
			return fmt.Errorf("store advisory %s: %w", adv.ID, err)
		}
		if _, err := Absorb(ctx, q, target, others); err != nil {
			return err
		}
		if inserted {
			st.AdvisoriesNew++
		} else {
			st.AdvisoriesUpdated++
		}
	}

	var compIDs []int64
	var advIDs []string
	// Two OSV records can share a canonical id, so de-duplicate the pairs: one
	// INSERT .. ON CONFLICT cannot touch the same row twice.
	seenPair := map[[2]string]bool{}
	for purl, osvIDs := range hits {
		for _, osvID := range osvIDs {
			adv, ok := canonical[osvID]
			if !ok {
				continue // record vanished (404 on detail fetch)
			}
			for _, cid := range byPurl[purl] {
				key := [2]string{fmt.Sprint(cid), adv}
				if seenPair[key] {
					continue
				}
				seenPair[key] = true
				compIDs = append(compIDs, cid)
				advIDs = append(advIDs, adv)
			}
		}
	}
	if len(compIDs) > 0 {
		if _, err := q.UpsertComponentAdvisories(ctx, store.UpsertComponentAdvisoriesParams{
			Source: "osv", ComponentIds: compIDs, AdvisoryIds: advIDs,
		}); err != nil {
			return fmt.Errorf("record component advisories: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// sameModified compares OSV timestamps at microsecond precision: querybatch
// truncates them while some detail records carry nanoseconds.
func sameModified(a, b string) bool {
	if a == b {
		return true
	}
	ta, errA := time.Parse(time.RFC3339Nano, a)
	tb, errB := time.Parse(time.RFC3339Nano, b)
	if errA != nil || errB != nil {
		return false
	}
	return ta.Truncate(time.Microsecond).Equal(tb.Truncate(time.Microsecond))
}
