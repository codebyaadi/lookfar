package advisory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"

	"github.com/codebyaadi/lookfar/internal/stack"
	"github.com/codebyaadi/lookfar/internal/store"
)

// NVDBaseURL is the NVD CVE API 2.0 endpoint.
const NVDBaseURL = "https://services.nvd.nist.gov/rest/json/cves/2.0"

// nvdMaxIncrementalWindow is NVD's limit for lastModStartDate..lastModEndDate.
const nvdMaxIncrementalWindow = 120 * 24 * time.Hour

// noMetricsRetry is how long a CVE that NVD has not scored yet is left alone.
const noMetricsRetry = 7 * 24 * time.Hour

// NVD collects CVEs for curated server software by CPE and enriches advisories
// that lack a severity or score.
type NVD struct {
	Pool    *pgxpool.Pool
	Fetcher *Fetcher
	BaseURL string // defaults to NVDBaseURL
	APIKey  string // optional; raises the rate limit tenfold
	// Limiter spaces requests. Nil picks NVD's published limits: 5 requests per
	// 30s without a key, 50 with one (kept slightly under).
	Limiter *rate.Limiter
	// PageSize is resultsPerPage for CPE queries (NVD maximum 2000).
	PageSize int
	// EnrichCap bounds CVE lookups per run so a run stays short; the rest
	// wait for the next run. Defaults to 50 without a key and 500 with one.
	EnrichCap int
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// Name implements Collector.
func (*NVD) Name() string { return "nvd-cve" }

// NVDStats are the counters of one run.
type NVDStats struct {
	ServersQueried     int // active server components with a version
	PairsQueried       int // distinct CPE vendor:product queries sent
	PairsFailed        int
	CVEsScanned        int // stored CVE records examined for matching
	Matches            int // component/CVE pairs recorded
	Enriched           int // advisories that gained severity or cvss
	SkippedRateLimited int // enrichment candidates left for the next run
}

// Stats returns run-log stats.
func (s NVDStats) Stats() map[string]any {
	return map[string]any{
		"servers_queried": s.ServersQueried, "pairs_queried": s.PairsQueried, "pairs_failed": s.PairsFailed,
		"cves_scanned": s.CVEsScanned, "matches": s.Matches, "enriched": s.Enriched,
		"skipped_rate_limited": s.SkippedRateLimited,
	}
}

type nvdCursor struct {
	// Pairs maps "vendor:product" to the newest lastModified seen, for incremental fetches.
	Pairs map[string]string `json:"pairs,omitempty"`
	// NoMetrics maps CVE ids NVD could not score to the time we asked.
	NoMetrics map[string]string `json:"no_metrics,omitempty"`
}

// Collect implements Collector.
func (n *NVD) Collect(ctx context.Context, since time.Time) (Result, error) {
	st, err := n.CollectStats(ctx, since)
	return Result{Fetched: st.CVEsScanned, New: st.Matches, Updated: st.Enriched}, err
}

func (n *NVD) now() time.Time {
	if n.Now != nil {
		return n.Now()
	}
	return time.Now()
}

func (n *NVD) limiter() *rate.Limiter {
	if n.Limiter == nil {
		if n.APIKey != "" {
			n.Limiter = rate.NewLimiter(rate.Every(650*time.Millisecond), 1)
		} else {
			n.Limiter = rate.NewLimiter(rate.Every(6200*time.Millisecond), 1)
		}
	}
	return n.Limiter
}

func (n *NVD) enrichCap() int {
	switch {
	case n.EnrichCap > 0:
		return n.EnrichCap
	case n.APIKey != "":
		return 500
	}
	return 50
}

// CollectStats runs the collector and returns detailed counters.
func (n *NVD) CollectStats(ctx context.Context, _ time.Time) (NVDStats, error) {
	var st NVDStats
	q := store.New(n.Pool)
	src, err := q.UpsertSource(ctx, store.UpsertSourceParams{Kind: "nvd", Name: n.Name(), Schedule: "@daily"})
	if err != nil {
		return st, fmt.Errorf("register source: %w", err)
	}
	var cur nvdCursor
	_ = json.Unmarshal(src.Cursor, &cur)
	if cur.Pairs == nil {
		cur.Pairs = map[string]string{}
	}
	if cur.NoMetrics == nil {
		cur.NoMetrics = map[string]string{}
	}

	runErr := n.collect(ctx, q, src, &cur, &st)

	finish := store.FinishSourceRunParams{ID: src.ID, Ok: runErr == nil}
	if runErr != nil {
		finish.Error = pgtype.Text{String: runErr.Error(), Valid: true}
	}
	// Progress (cursors, no-metrics marks) is kept even when part of the run failed.
	finish.Cursor, _ = json.Marshal(cur)
	if ferr := q.FinishSourceRun(context.WithoutCancel(ctx), finish); ferr != nil && runErr == nil {
		runErr = fmt.Errorf("record source run: %w", ferr)
	}
	return st, runErr
}

func (n *NVD) collect(ctx context.Context, q *store.Queries, src store.Source, cur *nvdCursor, st *NVDStats) error {
	comps, err := q.ListNVDServerComponents(ctx)
	if err != nil {
		return fmt.Errorf("list server components: %w", err)
	}
	st.ServersQueried = len(comps)

	// Group components by CPE pair so each pair is queried once.
	type compRef struct {
		id      int64
		version string
		pairs   []stack.CPEPair
	}
	var refs []compRef
	pairSet := map[stack.CPEPair]bool{}
	for _, c := range comps {
		s, ok := stack.LookupServer(c.Name)
		if !ok {
			continue
		}
		refs = append(refs, compRef{id: c.ID, version: c.Version, pairs: s.CPEs})
		for _, p := range s.CPEs {
			pairSet[p] = true
		}
	}
	pairs := make([]stack.CPEPair, 0, len(pairSet))
	for p := range pairSet {
		pairs = append(pairs, p)
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Vendor+":"+pairs[i].Product < pairs[j].Vendor+":"+pairs[j].Product
	})

	var firstErr error
	for _, p := range pairs {
		st.PairsQueried++
		if err := n.syncPair(ctx, q, src.ID, p, cur); err != nil {
			st.PairsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("query %s:%s: %w", p.Vendor, p.Product, err)
			}
		}
	}

	// Match every component against the stored CVEs of its products.
	matched := map[string]nvdCVE{}
	var compIDs []int64
	var advIDs []string
	seenPair := map[[2]string]bool{}
	scanned := map[string]bool{}
	cache := map[stack.CPEPair][]nvdCVE{}
	for _, c := range refs {
		for _, p := range c.pairs {
			cves, ok := cache[p]
			if !ok {
				if cves, err = n.loadCVEs(ctx, q, src.ID, p); err != nil {
					return err
				}
				cache[p] = cves
			}
			for _, cve := range cves {
				scanned[cve.ID] = true
				if !cve.affects([]stack.CPEPair{p}, c.version) {
					continue
				}
				key := [2]string{strconv.FormatInt(c.id, 10), cve.ID}
				if seenPair[key] {
					continue
				}
				seenPair[key] = true
				matched[cve.ID] = cve
				compIDs = append(compIDs, c.id)
				advIDs = append(advIDs, cve.ID)
			}
		}
	}
	st.CVEsScanned = len(scanned)
	st.Matches = len(compIDs)
	if err := n.writeMatches(ctx, matched, compIDs, advIDs); err != nil {
		return err
	}

	if err := n.enrich(ctx, q, src.ID, cur, st); err != nil {
		return err
	}
	return firstErr
}

// syncPair downloads the CVEs for one vendor:product (incrementally when a
// cursor exists) and stores changed ones in raw_items.
func (n *NVD) syncPair(ctx context.Context, q *store.Queries, sourceID int64, p stack.CPEPair, cur *nvdCursor) error {
	key := p.Vendor + ":" + p.Product
	params := url.Values{}
	params.Set("virtualMatchString", "cpe:2.3:a:"+p.Vendor+":"+p.Product)

	now := n.now().UTC()
	if last := parseNVDTime(cur.Pairs[key]); last != nil && now.Sub(*last) < nvdMaxIncrementalWindow {
		params.Set("lastModStartDate", last.Format(nvdTimeLayout))
		params.Set("lastModEndDate", now.Format(nvdTimeLayout))
	}

	records, err := n.fetchAll(ctx, params)
	if err != nil {
		return err
	}
	newest := cur.Pairs[key]
	for _, raw := range records {
		var c nvdCVE
		if err := json.Unmarshal(raw, &c); err != nil {
			return fmt.Errorf("parse CVE record: %w", err)
		}
		if _, err := q.UpsertRawItem(ctx, store.UpsertRawItemParams{
			SourceID: sourceID, ExternalID: c.ID, Payload: raw, PublishedAt: parseNVDTime(c.Published),
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store raw item %s: %w", c.ID, err)
		}
		if c.LastModified > newest { // same fixed-width format, so lexical order is chronological
			newest = c.LastModified
		}
	}
	if newest != "" {
		cur.Pairs[key] = newest
	}
	return nil
}

// loadCVEs returns the stored CVE records that mention a vendor:product.
func (n *NVD) loadCVEs(ctx context.Context, q *store.Queries, sourceID int64, p stack.CPEPair) ([]nvdCVE, error) {
	rows, err := q.ListRawItemsLike(ctx, store.ListRawItemsLikeParams{
		SourceID: sourceID, Pattern: "%cpe:2.3:a:" + p.Vendor + ":" + p.Product + ":%",
	})
	if err != nil {
		return nil, fmt.Errorf("load stored CVEs: %w", err)
	}
	out := make([]nvdCVE, 0, len(rows))
	for _, r := range rows {
		var c nvdCVE
		if err := json.Unmarshal(r.Payload, &c); err != nil {
			return nil, fmt.Errorf("parse stored CVE %s: %w", r.ExternalID, err)
		}
		out = append(out, c)
	}
	return out, nil
}

// writeMatches upserts the matched CVEs as advisories and records the hits.
func (n *NVD) writeMatches(ctx context.Context, matched map[string]nvdCVE, compIDs []int64, advIDs []string) error {
	if len(matched) == 0 {
		return nil
	}
	tx, err := n.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := store.New(tx)

	ids := make([]string, 0, len(matched))
	for id := range matched {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	targets := map[string]string{} // CVE id -> advisory id after the write-time guard
	for _, id := range ids {
		target, others, err := ResolveCanonical(ctx, q, id, nil)
		if err != nil {
			return err
		}
		targets[id] = target
		if err := upsertNVDAdvisory(ctx, q, target, normalizeNVD(matched[id])); err != nil {
			return err
		}
		if _, err := Absorb(ctx, q, target, others); err != nil {
			return err
		}
	}
	// Map to the resolved ids and drop pairs that collapse onto the same row.
	seen := map[[2]string]bool{}
	var cids []int64
	var aids []string
	for i, id := range advIDs {
		key := [2]string{strconv.FormatInt(compIDs[i], 10), targets[id]}
		if seen[key] {
			continue
		}
		seen[key] = true
		cids = append(cids, compIDs[i])
		aids = append(aids, targets[id])
	}
	compIDs, advIDs = cids, aids
	if _, err := q.UpsertComponentAdvisories(ctx, store.UpsertComponentAdvisoriesParams{
		Source: "nvd", ComponentIds: compIDs, AdvisoryIds: advIDs,
	}); err != nil {
		return fmt.Errorf("record component advisories: %w", err)
	}
	return tx.Commit(ctx)
}

func upsertNVDAdvisory(ctx context.Context, q *store.Queries, advisoryID string, a nvdAdvisory) error {
	cv := pgtype.Float8{}
	if a.CVSS != nil {
		cv = pgtype.Float8{Float64: *a.CVSS, Valid: true}
	}
	if _, err := q.UpsertNVDAdvisory(ctx, store.UpsertNVDAdvisoryParams{
		ID: advisoryID, Summary: a.Summary, Severity: a.Severity, Cvss: cv,
		Affected: a.Affected, Refs: a.References, ModifiedAt: a.ModifiedAt,
	}); err != nil {
		return fmt.Errorf("store advisory %s: %w", advisoryID, err)
	}
	return nil
}

// enrich fills severity and cvss for matched advisories that lack them.
func (n *NVD) enrich(ctx context.Context, q *store.Queries, sourceID int64, cur *nvdCursor, st *NVDStats) error {
	cands, err := q.ListEnrichmentCandidates(ctx)
	if err != nil {
		return fmt.Errorf("list enrichment candidates: %w", err)
	}
	now := n.now().UTC()
	var todo []store.ListEnrichmentCandidatesRow
	for _, c := range cands {
		if t, err := time.Parse(time.RFC3339, cur.NoMetrics[c.CveID]); err == nil && now.Sub(t) < noMetricsRetry {
			continue // NVD had no score the last time we asked
		}
		todo = append(todo, c)
	}

	limit := n.enrichCap()
	if len(todo) > limit {
		st.SkippedRateLimited = len(todo) - limit
		todo = todo[:limit]
	}
	for i, c := range todo {
		params := url.Values{"cveId": {c.CveID}}
		records, err := n.fetchAll(ctx, params)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Rate limited beyond the fetcher's retries: stop and leave the rest for next time.
			st.SkippedRateLimited += len(todo) - i
			return fmt.Errorf("enrichment stopped at %s: %w", c.CveID, err)
		}
		if len(records) == 0 {
			cur.NoMetrics[c.CveID] = now.Format(time.RFC3339)
			continue
		}
		var cve nvdCVE
		if err := json.Unmarshal(records[0], &cve); err != nil {
			return fmt.Errorf("parse CVE record %s: %w", c.CveID, err)
		}
		if _, err := q.UpsertRawItem(ctx, store.UpsertRawItemParams{
			SourceID: sourceID, ExternalID: cve.ID, Payload: records[0], PublishedAt: parseNVDTime(cve.Published),
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store raw item %s: %w", cve.ID, err)
		}
		adv := normalizeNVD(cve)
		if !adv.HasScore {
			cur.NoMetrics[c.CveID] = now.Format(time.RFC3339)
			continue
		}
		if err := upsertNVDAdvisory(ctx, q, c.AdvisoryID, adv); err != nil {
			return err
		}
		delete(cur.NoMetrics, c.CveID)
		st.Enriched++
	}
	return nil
}

type nvdPage struct {
	TotalResults    int `json:"totalResults"`
	Vulnerabilities []struct {
		CVE json.RawMessage `json:"cve"`
	} `json:"vulnerabilities"`
}

// fetchAll follows startIndex/resultsPerPage pagination and returns the raw CVE objects.
func (n *NVD) fetchAll(ctx context.Context, params url.Values) ([]json.RawMessage, error) {
	size := n.PageSize
	if size <= 0 {
		size = 2000
	}
	var out []json.RawMessage
	for start := 0; ; {
		p := url.Values{}
		for k, v := range params {
			p[k] = v
		}
		p.Set("resultsPerPage", strconv.Itoa(size))
		p.Set("startIndex", strconv.Itoa(start))

		page, err := n.getPage(ctx, p)
		if err != nil {
			return nil, err
		}
		for _, v := range page.Vulnerabilities {
			out = append(out, v.CVE)
		}
		start += len(page.Vulnerabilities)
		if len(page.Vulnerabilities) == 0 || start >= page.TotalResults {
			return out, nil
		}
	}
}

func (n *NVD) getPage(ctx context.Context, params url.Values) (nvdPage, error) {
	if err := n.limiter().Wait(ctx); err != nil {
		return nvdPage{}, err
	}
	base := n.BaseURL
	if base == "" {
		base = NVDBaseURL
	}
	hdr := http.Header{}
	if n.APIKey != "" {
		hdr.Set("apiKey", n.APIKey)
	}
	// url.Values.Encode escapes ':' which NVD accepts; keep it simple.
	resp, err := n.Fetcher.Get(ctx, base+"?"+params.Encode(), hdr)
	if err != nil {
		return nvdPage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return nvdPage{}, fmt.Errorf("NVD returned %s: %s", resp.Status, msg)
	}
	var page nvdPage
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<20)).Decode(&page); err != nil {
		return nvdPage{}, fmt.Errorf("parse NVD response: %w", err)
	}
	return page, nil
}
