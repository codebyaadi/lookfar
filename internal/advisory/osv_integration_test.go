//go:build integration

package advisory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/codebyaadi/lookfar/internal/store"
)

// fakeOSV serves querybatch and vulns from the fixtures. django's results are
// split over two pages to exercise next_page_token.
type fakeOSV struct {
	*httptest.Server
	mu        sync.Mutex
	dropLodas bool              // stop reporting the lodash hit
	modified  map[string]string // overrides for the modified timestamp
	detailGET atomic.Int32
	queries   atomic.Int32
}

func (f *fakeOSV) mod(id, def string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if m, ok := f.modified[id]; ok {
		return m
	}
	return def
}

func newFakeOSV(t *testing.T) *fakeOSV {
	t.Helper()
	f := &fakeOSV{modified: map[string]string{}}
	defaults := map[string]string{
		"GHSA-29mw-wpgm-hmr9": "2025-09-29T21:12:31.102523Z",
		"GHSA-2gwj-7jmv-h26r": "2025-02-21T05:41:10.759178Z",
		"PYSEC-2021-109":      "2023-12-06T01:01:17.903404Z",
		"GO-2024-2598":        "2026-02-04T03:12:08.722840Z",
		"GHSA-dupe-0000-test": "2026-01-01T00:00:00Z",
	}
	vuln := func(id string) map[string]string {
		return map[string]string{"id": id, "modified": f.mod(id, defaults[id])}
	}

	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/querybatch":
			var req struct {
				Queries []batchQuery `json:"queries"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			var results []map[string]any
			for _, q := range req.Queries {
				f.queries.Add(1)
				res := map[string]any{}
				purl := q.Package.Purl
				switch {
				case strings.HasPrefix(purl, "pkg:npm/lodash"):
					f.mu.Lock()
					drop := f.dropLodas
					f.mu.Unlock()
					if !drop {
						res["vulns"] = []any{vuln("GHSA-29mw-wpgm-hmr9")}
					}
				case strings.HasPrefix(purl, "pkg:pypi/django") && q.PageToken == "":
					res["vulns"] = []any{vuln("GHSA-2gwj-7jmv-h26r")}
					res["next_page_token"] = "page2"
				case strings.HasPrefix(purl, "pkg:pypi/django") && q.PageToken == "page2":
					res["vulns"] = []any{vuln("PYSEC-2021-109")}
				case strings.HasPrefix(purl, "pkg:golang/stdlib"):
					res["vulns"] = []any{vuln("GO-2024-2598"), vuln("GHSA-dupe-0000-test")}
				}
				results = append(results, res)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/vulns/"):
			f.detailGET.Add(1)
			id := strings.TrimPrefix(r.URL.Path, "/v1/vulns/")
			b, err := os.ReadFile(filepath.Join("testdata", "osv", id+".json"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			// Reflect an overridden modified time in the detail payload too.
			if m, ok := f.modified[id]; ok {
				var v map[string]any
				_ = json.Unmarshal(b, &v)
				v["modified"] = m
				b, _ = json.Marshal(v)
			}
			_, _ = w.Write(b)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func seedComponents(t *testing.T, ctx context.Context, q *store.Queries) {
	t.Helper()
	add := func(eco, name, version, purl, kind string) int64 {
		id, err := q.InsertComponent(ctx, store.InsertComponentParams{
			Ecosystem: eco, Name: name, Version: version, Purl: purl, Origin: "manual", ManifestPath: "", Kind: kind,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	add("npm", "lodash", "4.17.20", "pkg:npm/lodash@4.17.20", "package")
	add("PyPI", "django", "3.2.0", "pkg:pypi/django@3.2.0", "package")
	add("Go", "stdlib", "1.22", "pkg:golang/stdlib@1.22", "package")
	// Not queryable: image, generic server, no exact version.
	add("oci", "nginx", "latest", "pkg:oci/nginx@latest", "image")
	add("generic", "postgresql", "16.4", "pkg:generic/postgresql@16.4", "server")
	add("npm", "left-pad", "", "pkg:npm/left-pad", "package")
	// Inactive components are ignored entirely.
	gone := add("npm", "gone", "1.0.0", "pkg:npm/gone@1.0.0", "package")
	if _, err := q.DeactivateComponents(ctx, []int64{gone}); err != nil {
		t.Fatal(err)
	}
}

func TestOSVCollectEndToEnd(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	q := store.New(pool)
	seedComponents(t, ctx, q)

	// KEV already knows CVE-2022-28346 (the django one).
	if _, err := pool.Exec(ctx, `INSERT INTO advisories (id, summary, exploited, exploited_since, affected, refs)
		VALUES ('CVE-2022-28346', 'KEV: Django SQL injection', true, '2022-08-25',
		        '[{"source":"kev","vendorProject":"Django","product":"Django"}]', '[{"source":"kev","type":"kev"}]')`); err != nil {
		t.Fatal(err)
	}

	fake := newFakeOSV(t)
	o := &OSV{Pool: pool, Fetcher: testFetcher(), BaseURL: fake.URL, Concurrency: 2}

	st, err := o.CollectStats(ctx, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	// 3 queryable components; image, generic and versionless are skipped (inactive row not counted).
	if st.ComponentsQueried != 3 || st.ComponentsSkipped != 3 || st.VulnsFound != 5 || st.DetailsFetched != 5 {
		t.Fatalf("run 1 stats: %+v", st)
	}
	if st.AdvisoriesNew != 3 || st.AdvisoriesUpdated != 2 { // updated: pre-existing KEV row + the second record for CVE-2024-24783
		t.Fatalf("run 1 advisories: %+v", st)
	}
	if fake.queries.Load() != 4 { // 3 queries + the django follow-up page
		t.Errorf("queries sent = %d, want 4 (pagination)", fake.queries.Load())
	}

	// KEV fields survive; OSV fills severity/cvss/aliases and adds its own elements.
	var (
		exploited                   bool
		since                       *time.Time
		summary, severity           string
		cvss                        *float64
		aliases                     []string
		kevElems, osvElems, kevRefs int
	)
	err = pool.QueryRow(ctx, `SELECT exploited, exploited_since, summary, severity, cvss::float8, aliases,
		(SELECT count(*) FROM jsonb_array_elements(affected) e WHERE e->>'source'='kev'),
		(SELECT count(*) FROM jsonb_array_elements(affected) e WHERE e->>'source'='osv'),
		(SELECT count(*) FROM jsonb_array_elements(refs) e WHERE e->>'source'='kev')
		FROM advisories WHERE id='CVE-2022-28346'`).
		Scan(&exploited, &since, &summary, &severity, &cvss, &aliases, &kevElems, &osvElems, &kevRefs)
	if err != nil {
		t.Fatal(err)
	}
	if !exploited || since == nil || since.Format("2006-01-02") != "2022-08-25" {
		t.Errorf("OSV touched exploited fields: %v %v", exploited, since)
	}
	if summary != "KEV: Django SQL injection" {
		t.Errorf("summary must not be overwritten: %q", summary)
	}
	if severity != "critical" || cvss == nil || *cvss != 9.8 || len(aliases) != 3 {
		t.Errorf("OSV fields: %s %v %v", severity, cvss, aliases)
	}
	if kevElems != 1 || osvElems == 0 || kevRefs != 1 {
		t.Errorf("per-source elements: kev affected %d, osv affected %d, kev refs %d", kevElems, osvElems, kevRefs)
	}

	pairs := func() map[string]time.Time {
		rows, err := pool.Query(ctx, `SELECT c.name || '>' || ca.advisory_id, ca.last_seen
			FROM component_advisories ca JOIN components c ON c.id = ca.component_id WHERE ca.source = 'osv'`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]time.Time{}
		for rows.Next() {
			var k string
			var ts time.Time
			if err := rows.Scan(&k, &ts); err != nil {
				t.Fatal(err)
			}
			out[k] = ts
		}
		return out
	}
	first := pairs()
	for _, want := range []string{"lodash>CVE-2020-28500", "django>CVE-2022-28346", "django>CVE-2021-35042", "stdlib>CVE-2024-24783"} {
		if _, ok := first[want]; !ok {
			t.Errorf("missing pair %s in %v", want, first)
		}
	}
	if len(first) != 4 {
		t.Errorf("pairs = %d", len(first))
	}

	// Second run: nothing changed upstream, so no detail fetches and no advisory writes,
	// but last_seen moves forward.
	gets := fake.detailGET.Load()
	st2, err := o.CollectStats(ctx, time.Time{})
	if err != nil || st2.DetailsFetched != 0 || st2.AdvisoriesNew != 0 || st2.AdvisoriesUpdated != 0 || st2.VulnsFound != 5 {
		t.Fatalf("run 2: %+v %v", st2, err)
	}
	if fake.detailGET.Load() != gets {
		t.Error("run 2 fetched details again")
	}
	second := pairs()
	if len(second) != 4 || !second["lodash>CVE-2020-28500"].After(first["lodash>CVE-2020-28500"]) {
		t.Errorf("last_seen should advance: %v vs %v", first, second)
	}

	// A changed modified timestamp triggers exactly one detail fetch and one update.
	fake.mu.Lock()
	fake.modified["GO-2024-2598"] = "2026-03-01T00:00:00Z"
	fake.mu.Unlock()
	st3, err := o.CollectStats(ctx, time.Time{})
	if err != nil || st3.DetailsFetched != 1 || st3.AdvisoriesUpdated != 1 || st3.AdvisoriesNew != 0 {
		t.Fatalf("run 3: %+v %v", st3, err)
	}

	// OSV stops reporting the lodash pair: the row stays, last_seen goes stale.
	beforeDrop := pairs()
	fake.mu.Lock()
	fake.dropLodas = true
	fake.mu.Unlock()
	if _, err := o.CollectStats(ctx, time.Time{}); err != nil {
		t.Fatal(err)
	}
	after := pairs()
	if len(after) != 4 || after["lodash>CVE-2020-28500"] != beforeDrop["lodash>CVE-2020-28500"] {
		t.Errorf("stale pair should be kept untouched: %v", after)
	}

	src, _ := q.UpsertSource(ctx, store.UpsertSourceParams{Kind: "osv", Name: "osv-dev", Schedule: "@every 12h"})
	if src.LastOkAt == nil || src.LastError.Valid {
		t.Errorf("source bookkeeping: %+v", src)
	}
}

func TestOSVServerErrorIsRecorded(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	q := store.New(pool)
	seedComponents(t, ctx, q)

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer bad.Close()
	f := testFetcher()
	f.MaxRetries = 1
	o := &OSV{Pool: pool, Fetcher: f, BaseURL: bad.URL}
	if _, err := o.CollectStats(ctx, time.Time{}); err == nil {
		t.Fatal("expected failure")
	}
	src, _ := q.UpsertSource(ctx, store.UpsertSourceParams{Kind: "osv", Name: "osv-dev", Schedule: "@every 12h"})
	if !src.LastError.Valid || src.LastOkAt != nil {
		t.Errorf("failure bookkeeping: %+v", src)
	}
}
