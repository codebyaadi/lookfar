//go:build integration

package advisory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/codebyaadi/lookfar/internal/store"
)

// fakeNVD serves recorded CVE pages. CPE queries answer from a per-pair list,
// cveId queries from the fixture files, and it can refuse the first request
// with 403 to exercise the retry on rate limiting.
type fakeNVD struct {
	*httptest.Server
	mu       sync.Mutex
	byPair   map[string][]string // virtualMatchString -> CVE ids
	incr     []string            // lastModStartDate values seen
	cveIDs   []string            // cveId queries seen
	refuse   atomic.Int32        // number of 403s still to send
	requests atomic.Int32
}

func newFakeNVD(t *testing.T, byPair map[string][]string) *fakeNVD {
	t.Helper()
	f := &fakeNVD{byPair: byPair}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		if f.refuse.Load() > 0 {
			f.refuse.Add(-1)
			http.Error(w, "rate limited", http.StatusForbidden)
			return
		}
		qs := r.URL.Query()
		var ids []string
		switch {
		case qs.Get("cveId") != "":
			f.mu.Lock()
			f.cveIDs = append(f.cveIDs, qs.Get("cveId"))
			f.mu.Unlock()
			if _, err := os.Stat(filepath.Join("testdata", "nvd", qs.Get("cveId")+".json")); err == nil {
				ids = []string{qs.Get("cveId")}
			}
		case qs.Get("virtualMatchString") != "":
			if v := qs.Get("lastModStartDate"); v != "" {
				f.mu.Lock()
				f.incr = append(f.incr, v)
				f.mu.Unlock()
			}
			ids = f.byPair[qs.Get("virtualMatchString")]
		}

		start, _ := strconv.Atoi(qs.Get("startIndex"))
		size, _ := strconv.Atoi(qs.Get("resultsPerPage"))
		var vulns []json.RawMessage
		for i := start; i < len(ids) && (size == 0 || i < start+size); i++ {
			b, _ := os.ReadFile(filepath.Join("testdata", "nvd", ids[i]+".json"))
			var page nvdPage
			_ = json.Unmarshal(b, &page)
			wrapped, _ := json.Marshal(map[string]json.RawMessage{"cve": page.Vulnerabilities[0].CVE})
			vulns = append(vulns, wrapped)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resultsPerPage": len(vulns), "startIndex": start, "totalResults": len(ids), "vulnerabilities": vulns,
		})
	}))
	t.Cleanup(f.Close)
	return f
}

func newTestNVD(t *testing.T, ctx context.Context, f *fakeNVD) (*NVD, *store.Queries) {
	t.Helper()
	pool := newPool(t)
	fe := NewFetcher("test")
	fe.Backoff = time.Millisecond
	fe.RetryOn = []int{403}
	return &NVD{
		Pool: pool, Fetcher: fe, BaseURL: f.URL,
		Limiter:  rate.NewLimiter(rate.Inf, 1),
		PageSize: 1, // forces pagination
	}, store.New(pool)
}

func addServer(t *testing.T, ctx context.Context, q *store.Queries, name, version string) int64 {
	t.Helper()
	id, err := q.InsertComponent(ctx, store.InsertComponentParams{
		Ecosystem: "generic", Name: name, Version: version, Purl: "pkg:generic/" + name + "@" + version,
		Origin: "manual", Kind: "server",
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestNVDCollectMatchesEnrichesAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	fake := newFakeNVD(t, map[string][]string{
		"cpe:2.3:a:f5:nginx":              {"CVE-2022-41741"},
		"cpe:2.3:a:postgresql:postgresql": {"CVE-2023-5868"},
	})
	n, q := newTestNVD(t, ctx, fake)
	pool := n.Pool

	addServer(t, ctx, q, "nginx", "1.22.0")
	addServer(t, ctx, q, "nginx", "1.25.3")
	pg16 := addServer(t, ctx, q, "postgresql", "16.0")
	addServer(t, ctx, q, "postgresql", "16.4")
	addServer(t, ctx, q, "postgresql", "11.5")

	// An advisory OSV already scored: NVD must not override its severity or cvss.
	if _, err := pool.Exec(ctx, `INSERT INTO advisories (id, summary, severity, cvss, exploited, exploited_since)
		VALUES ('CVE-2023-5868', 'osv summary', 'high', 7.1, true, '2023-11-01')`); err != nil {
		t.Fatal(err)
	}
	// A KEV-only advisory that a component hit but nobody scored (enrichment candidate).
	if _, err := pool.Exec(ctx, `INSERT INTO advisories (id, summary, exploited, exploited_since)
		VALUES ('CVE-2021-23017', 'kev summary', true, '2022-01-01')`); err != nil {
		t.Fatal(err)
	}
	// A candidate NVD does not know (no metrics): remembered, not retried.
	if _, err := pool.Exec(ctx, `INSERT INTO advisories (id, summary) VALUES ('CVE-2099-0001', 'unknown to nvd')`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"CVE-2021-23017", "CVE-2099-0001"} {
		if _, err := pool.Exec(ctx, `INSERT INTO component_advisories (component_id, advisory_id, source) VALUES ($1, $2, 'osv')`, pg16, id); err != nil {
			t.Fatal(err)
		}
	}

	fake.refuse.Store(1) // the first request is rate limited with 403 and retried

	st, err := n.CollectStats(ctx, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if st.ServersQueried != 5 || st.PairsQueried != 3 || st.PairsFailed != 0 { // f5:nginx, nginx:nginx, postgresql:postgresql
		t.Errorf("queries: %+v", st)
	}
	// nginx 1.22.0 -> 41741; postgresql 16.0 and 11.5 -> 5868.
	if st.Matches != 3 || st.CVEsScanned != 2 {
		t.Errorf("matches: %+v", st)
	}
	if st.Enriched != 1 || st.SkippedRateLimited != 0 { // CVE-2021-23017 gets a score; CVE-2099-0001 has none
		t.Errorf("enrichment: %+v", st)
	}

	rows, err := pool.Query(ctx, `SELECT c.version || '>' || ca.advisory_id FROM component_advisories ca
		JOIN components c ON c.id = ca.component_id WHERE ca.source = 'nvd' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		got = append(got, s)
	}
	rows.Close()
	sort.Strings(got)
	want := []string{"1.22.0>CVE-2022-41741", "11.5>CVE-2023-5868", "16.0>CVE-2023-5868"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("nvd pairs = %v, want %v", got, want)
	}

	type adv struct {
		severity  string
		cvss      *float64
		exploited bool
		nvdElems  int
	}
	read := func(id string) adv {
		var a adv
		err := pool.QueryRow(ctx, `SELECT severity, cvss::float8, exploited,
			(SELECT count(*) FROM jsonb_array_elements(affected) e WHERE e->>'source'='nvd')
			FROM advisories WHERE id = $1`, id).Scan(&a.severity, &a.cvss, &a.exploited, &a.nvdElems)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	// OSV's score wins; NVD only added its own elements.
	if a := read("CVE-2023-5868"); a.severity != "high" || a.cvss == nil || *a.cvss != 7.1 || !a.exploited || a.nvdElems != 1 {
		t.Errorf("CVE-2023-5868 must keep OSV's score and KEV flag: %+v", a)
	}
	// NVD created this advisory: it carries NVD's primary score (7.8).
	if a := read("CVE-2022-41741"); a.severity != "high" || a.cvss == nil || *a.cvss != 7.8 || a.exploited {
		t.Errorf("CVE-2022-41741: %+v", a)
	}
	// Enriched KEV-only advisory: severity and cvss filled, exploited untouched.
	if a := read("CVE-2021-23017"); a.severity == "unknown" || a.cvss == nil || !a.exploited {
		t.Errorf("CVE-2021-23017 should be enriched: %+v", a)
	}
	if a := read("CVE-2099-0001"); a.severity != "unknown" || a.cvss != nil {
		t.Errorf("CVE-2099-0001 has no NVD data: %+v", a)
	}

	// Cursor state: per-pair lastModified and the no-metrics mark.
	src, _ := q.UpsertSource(ctx, store.UpsertSourceParams{Kind: "nvd", Name: "nvd-cve", Schedule: "@daily"})
	var cur nvdCursor
	_ = json.Unmarshal(src.Cursor, &cur)
	if cur.Pairs["f5:nginx"] == "" || cur.Pairs["postgresql:postgresql"] == "" || cur.NoMetrics["CVE-2099-0001"] == "" {
		t.Errorf("cursor: %s", src.Cursor)
	}
	if src.LastOkAt == nil || src.LastError.Valid {
		t.Errorf("source bookkeeping: %+v", src)
	}

	// Second run: incremental queries, no repeat lookups for CVEs NVD cannot score,
	// and nothing changes.
	before := len(fake.cveIDs)
	st2, err := n.CollectStats(ctx, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if st2.Matches != 3 || st2.Enriched != 0 || st2.PairsFailed != 0 {
		t.Errorf("run 2: %+v", st2)
	}
	// postgresql was modified recently, so its pair is fetched incrementally; the nginx
	// record is older than NVD's 120-day window and is fetched in full.
	if len(fake.incr) != 1 {
		t.Errorf("want one incremental query (postgresql), saw %v", fake.incr)
	}
	if len(fake.cveIDs) != before {
		t.Errorf("run 2 asked NVD again for %v", fake.cveIDs[before:])
	}
	var nvdPairs, rawItems int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM component_advisories WHERE source='nvd'`).Scan(&nvdPairs)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM raw_items r JOIN sources s ON s.id = r.source_id WHERE s.kind='nvd'`).Scan(&rawItems)
	if nvdPairs != 3 || rawItems != 3 { // 41741, 5868 and the enriched 23017
		t.Errorf("after run 2: %d pairs, %d raw items", nvdPairs, rawItems)
	}
}

func TestNVDEnrichmentCapSkipsTheRest(t *testing.T) {
	ctx := context.Background()
	fake := newFakeNVD(t, nil)
	n, q := newTestNVD(t, ctx, fake)
	n.EnrichCap = 1
	pool := n.Pool
	c := addServer(t, ctx, q, "nginx", "1.0.0")
	for _, id := range []string{"CVE-2021-23017", "CVE-2022-41741"} {
		if _, err := pool.Exec(ctx, `INSERT INTO advisories (id) VALUES ($1)`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO component_advisories (component_id, advisory_id, source) VALUES ($1, $2, 'osv')`, c, id); err != nil {
			t.Fatal(err)
		}
	}
	st, err := n.CollectStats(ctx, time.Time{})
	if err != nil || st.Enriched != 1 || st.SkippedRateLimited != 1 {
		t.Fatalf("%+v %v", st, err)
	}
	// The next run picks up the remainder.
	st, err = n.CollectStats(ctx, time.Time{})
	if err != nil || st.Enriched != 1 || st.SkippedRateLimited != 0 {
		t.Fatalf("run 2: %+v %v", st, err)
	}
}

func TestNVDGivesUpOnPersistentRateLimit(t *testing.T) {
	ctx := context.Background()
	fake := newFakeNVD(t, map[string][]string{"cpe:2.3:a:f5:nginx": {"CVE-2022-41741"}})
	fake.refuse.Store(1000)
	n, q := newTestNVD(t, ctx, fake)
	n.Fetcher.MaxRetries = 1
	addServer(t, ctx, q, "nginx", "1.22.0")

	st, err := n.CollectStats(ctx, time.Time{})
	if err == nil || st.PairsFailed == 0 {
		t.Fatalf("expected a recorded failure: %+v %v", st, err)
	}
	src, _ := q.UpsertSource(ctx, store.UpsertSourceParams{Kind: "nvd", Name: "nvd-cve", Schedule: "@daily"})
	if !src.LastError.Valid || src.LastOkAt != nil {
		t.Errorf("bookkeeping: %+v", src)
	}
}
