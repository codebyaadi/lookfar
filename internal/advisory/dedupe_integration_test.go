//go:build integration

package advisory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/codebyaadi/lookfar/internal/store"
)

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
}

func advisoryIDs(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT id FROM advisories ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		out = append(out, id)
	}
	return out
}

func pairsFor(t *testing.T, pool *pgxpool.Pool, advisoryID string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT component_id::text || '/' || source FROM component_advisories WHERE advisory_id = $1 ORDER BY 1`, advisoryID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		out = append(out, s)
	}
	return out
}

func newComponent(t *testing.T, q *store.Queries, purl string) int64 {
	t.Helper()
	id, err := q.InsertComponent(context.Background(), store.InsertComponentParams{
		Ecosystem: "npm", Name: purl, Version: "1.0.0", Purl: purl, Origin: "manual", Kind: "package",
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestDedupeMergesKnownDuplicates(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	q := store.New(pool)
	c1, c2, c3 := newComponent(t, q, "pkg:npm/a@1.0.0"), newComponent(t, q, "pkg:npm/b@1.0.0"), newComponent(t, q, "pkg:npm/c@1.0.0")

	// Case 1: a GHSA written without a CVE, then a CVE row listing it as an alias.
	mustExec(t, pool, `INSERT INTO advisories (id, summary, severity, cvss, affected, refs, modified_at)
		VALUES ('GHSA-dup1', 'ghsa summary', 'high', 7.0, '[{"source":"osv","osvId":"GHSA-dup1"}]', '[{"source":"osv","osvId":"GHSA-dup1"}]', '2024-01-01')`)
	mustExec(t, pool, `INSERT INTO advisories (id, aliases, summary, severity, cvss, affected, refs, modified_at)
		VALUES ('CVE-2030-1', ARRAY['GHSA-dup1'], '', 'critical', 9.8, '[{"source":"nvd"}]', '[{"source":"nvd"}]', '2025-01-01')`)
	mustExec(t, pool, `INSERT INTO component_advisories (component_id, advisory_id, source, first_seen, last_seen) VALUES
		($1, 'GHSA-dup1', 'osv', '2026-01-01', '2026-02-01'),
		($2, 'CVE-2030-1', 'nvd', '2026-01-05', '2026-01-06'),
		($1, 'CVE-2030-1', 'osv', '2026-01-10', '2026-03-01')`, c1, c2)

	// Case 2: a KEV CVE plus an OSV GHSA that aliases it (two rows for one CVE).
	mustExec(t, pool, `INSERT INTO advisories (id, summary, exploited, exploited_since, affected, refs)
		VALUES ('CVE-2030-2', 'KEV summary', true, '2023-05-01', '[{"source":"kev"}]', '[{"source":"kev"}]')`)
	mustExec(t, pool, `INSERT INTO advisories (id, aliases, summary, severity, cvss, affected, refs)
		VALUES ('GHSA-dup2', ARRAY['CVE-2030-2','PYSEC-2030-2'], 'osv summary', 'medium', 5.5, '[{"source":"osv","osvId":"GHSA-dup2"}]', '[]')`)
	mustExec(t, pool, `INSERT INTO component_advisories (component_id, advisory_id, source) VALUES ($1, 'GHSA-dup2', 'osv')`, c3)

	// Control: an unrelated advisory must be left alone.
	mustExec(t, pool, `INSERT INTO advisories (id, summary) VALUES ('CVE-2030-9', 'unrelated')`)

	groups, err := DuplicateGroups(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"CVE-2030-1", "GHSA-dup1"}, {"CVE-2030-2", "GHSA-dup2"}}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups = %v, want %v", groups, want)
	}

	// Dry run reports but changes nothing.
	st, err := Dedupe(ctx, pool, true)
	if err != nil || st.Groups != 2 || st.RowsMerged != 0 || len(advisoryIDs(t, pool)) != 5 {
		t.Fatalf("dry run: %+v %v %v", st, err, advisoryIDs(t, pool))
	}

	st, err = Dedupe(ctx, pool, false)
	if err != nil || st.Groups != 2 || st.RowsMerged != 2 {
		t.Fatalf("dedupe: %+v %v", st, err)
	}
	if got := advisoryIDs(t, pool); !reflect.DeepEqual(got, []string{"CVE-2030-1", "CVE-2030-2", "CVE-2030-9"}) {
		t.Fatalf("rows after dedupe: %v", got)
	}

	// Case 1: aliases unioned, OSV's score wins over NVD's, summary filled from the GHSA, pairs merged.
	var aliases []string
	var summary, severity string
	var cvss float64
	var osvEl, nvdEl int
	if err := pool.QueryRow(ctx, `SELECT aliases, summary, severity, cvss::float8,
		(SELECT count(*) FROM jsonb_array_elements(affected) e WHERE e->>'source'='osv'),
		(SELECT count(*) FROM jsonb_array_elements(affected) e WHERE e->>'source'='nvd')
		FROM advisories WHERE id='CVE-2030-1'`).Scan(&aliases, &summary, &severity, &cvss, &osvEl, &nvdEl); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(aliases, []string{"GHSA-dup1"}) || summary != "ghsa summary" || severity != "high" || cvss != 7.0 || osvEl != 1 || nvdEl != 1 {
		t.Errorf("case 1: %v %q %s %v osv=%d nvd=%d", aliases, summary, severity, cvss, osvEl, nvdEl)
	}
	pairs := pairsFor(t, pool, "CVE-2030-1")
	wantPairs := []string{itoa(c1) + "/osv", itoa(c2) + "/nvd"}
	if !reflect.DeepEqual(pairs, wantPairs) {
		t.Errorf("case 1 pairs %v, want %v", pairs, wantPairs)
	}
	var first, last time.Time
	_ = pool.QueryRow(ctx, `SELECT first_seen, last_seen FROM component_advisories WHERE advisory_id='CVE-2030-1' AND component_id=$1`, c1).Scan(&first, &last)
	if first.UTC().Format("2006-01-02") != "2026-01-01" || last.UTC().Format("2006-01-02") != "2026-03-01" {
		t.Errorf("overlapping pair should keep the widest window: %v .. %v", first, last)
	}

	// Case 2: KEV's exploited flag survives, aliases unioned, KEV and OSV elements both present.
	var exploited bool
	var since *time.Time
	var kevEl int
	if err := pool.QueryRow(ctx, `SELECT aliases, exploited, exploited_since, severity,
		(SELECT count(*) FROM jsonb_array_elements(affected) e WHERE e->>'source'='kev'),
		(SELECT count(*) FROM jsonb_array_elements(affected) e WHERE e->>'source'='osv')
		FROM advisories WHERE id='CVE-2030-2'`).Scan(&aliases, &exploited, &since, &severity, &kevEl, &osvEl); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(aliases, []string{"GHSA-dup2", "PYSEC-2030-2"}) || !exploited || since == nil ||
		since.Format("2006-01-02") != "2023-05-01" || severity != "medium" || kevEl != 1 || osvEl != 1 {
		t.Errorf("case 2: %v exploited=%v since=%v %s kev=%d osv=%d", aliases, exploited, since, severity, kevEl, osvEl)
	}
	if got := pairsFor(t, pool, "CVE-2030-2"); !reflect.DeepEqual(got, []string{itoa(c3) + "/osv"}) {
		t.Errorf("case 2 pairs: %v", got)
	}

	// Running it again finds and changes nothing.
	st, err = Dedupe(ctx, pool, false)
	if err != nil || st.Groups != 0 || st.RowsMerged != 0 || st.Repointed != 0 {
		t.Errorf("second run: %+v %v", st, err)
	}
	if g, _ := DuplicateGroups(ctx, q); len(g) != 0 {
		t.Errorf("duplicates remain: %v", g)
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestDedupeGroupWithoutCVEKeepsSmallestRowID(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	mustExec(t, pool, `INSERT INTO advisories (id, aliases) VALUES ('GO-2030-5', ARRAY['GHSA-nocve'])`)
	mustExec(t, pool, `INSERT INTO advisories (id) VALUES ('GHSA-nocve')`)
	if _, err := Dedupe(ctx, pool, false); err != nil {
		t.Fatal(err)
	}
	if got := advisoryIDs(t, pool); !reflect.DeepEqual(got, []string{"GHSA-nocve"}) {
		t.Errorf("rows: %v", got)
	}
}

func TestResolveCanonicalGuard(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	q := store.New(pool)
	mustExec(t, pool, `INSERT INTO advisories (id, aliases) VALUES ('CVE-2030-3', ARRAY['GHSA-guard1'])`)
	mustExec(t, pool, `INSERT INTO advisories (id) VALUES ('GHSA-guard2')`)

	// An id that is an alias of an existing CVE row resolves to that row.
	target, others, err := ResolveCanonical(ctx, q, "GHSA-guard1", nil)
	if err != nil || target != "CVE-2030-3" || len(others) != 0 {
		t.Errorf("alias resolve: %s %v %v", target, others, err)
	}
	// A new CVE that aliases an existing CVE-less row takes over and absorbs it.
	target, others, err = ResolveCanonical(ctx, q, "CVE-2030-4", []string{"GHSA-guard2"})
	if err != nil || target != "CVE-2030-4" || !reflect.DeepEqual(others, []string{"GHSA-guard2"}) {
		t.Errorf("absorb resolve: %s %v %v", target, others, err)
	}
	// Nothing overlaps: the id is unchanged.
	target, others, err = ResolveCanonical(ctx, q, "GHSA-fresh", []string{"PYSEC-fresh"})
	if err != nil || target != "GHSA-fresh" || len(others) != 0 {
		t.Errorf("fresh resolve: %s %v %v", target, others, err)
	}
}

// fakeOSVRecords serves querybatch/vulns from inline records, so a test can
// control exactly which ids and aliases OSV reports.
func fakeOSVRecords(t *testing.T, purlToIDs map[string][]string, records map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/querybatch":
			var req struct {
				Queries []batchQuery `json:"queries"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			var results []map[string]any
			for _, q := range req.Queries {
				var vulns []map[string]string
				for _, id := range purlToIDs[q.Package.Purl] {
					vulns = append(vulns, map[string]string{"id": id, "modified": "2026-01-01T00:00:00Z"})
				}
				results = append(results, map[string]any{"vulns": vulns})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
		case strings.HasPrefix(r.URL.Path, "/v1/vulns/"):
			rec, ok := records[strings.TrimPrefix(r.URL.Path, "/v1/vulns/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(rec))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// New duplicates must not appear when a collector writes a record whose id is
// already known under another id, or whose CVE alias makes older rows redundant.
func TestWriteTimeGuardPreventsDuplicates(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	q := store.New(pool)
	guard := newComponent(t, q, "pkg:npm/guard@1.0.0")
	legacy := newComponent(t, q, "pkg:npm/legacy@1.0.0")

	// An existing CVE row already lists GHSA-guard1 as an alias.
	mustExec(t, pool, `INSERT INTO advisories (id, aliases, severity, cvss) VALUES ('CVE-2030-3', ARRAY['GHSA-guard1'], 'high', 7.5)`)
	// A legacy GHSA-only row (written before its CVE alias was known) with a hit.
	mustExec(t, pool, `INSERT INTO advisories (id, summary) VALUES ('GHSA-guard6', 'legacy')`)
	mustExec(t, pool, `INSERT INTO component_advisories (component_id, advisory_id, source) VALUES ($1, 'GHSA-guard6', 'osv')`, legacy)

	records := map[string]string{
		// OSV reports GHSA-guard1 without a CVE alias: canonical would be GHSA-guard1.
		"GHSA-guard1": `{"id":"GHSA-guard1","summary":"no alias","modified":"2026-01-01T00:00:00Z","database_specific":{"severity":"LOW"},
			"affected":[{"package":{"name":"guard","ecosystem":"npm"},"versions":["1.0.0"]}]}`,
		// Now the same legacy GHSA arrives with its CVE alias.
		"GHSA-guard6": `{"id":"GHSA-guard6","aliases":["CVE-2030-6"],"summary":"with alias","modified":"2026-01-01T00:00:00Z",
			"affected":[{"package":{"name":"legacy","ecosystem":"npm"},"versions":["1.0.0"]}]}`,
	}
	srv := fakeOSVRecords(t, map[string][]string{
		"pkg:npm/guard@1.0.0":  {"GHSA-guard1"},
		"pkg:npm/legacy@1.0.0": {"GHSA-guard6"},
	}, records)

	o := &OSV{Pool: pool, Fetcher: testFetcher(), BaseURL: srv.URL}
	if _, err := o.CollectStats(ctx, time.Time{}); err != nil {
		t.Fatal(err)
	}

	// Exactly CVE-2030-3 and CVE-2030-6: no GHSA-guard1 row was created, and the
	// legacy GHSA-guard6 row was absorbed into the CVE.
	if got := advisoryIDs(t, pool); !reflect.DeepEqual(got, []string{"CVE-2030-3", "CVE-2030-6"}) {
		t.Fatalf("advisories: %v", got)
	}
	if got := pairsFor(t, pool, "CVE-2030-3"); !reflect.DeepEqual(got, []string{itoa(guard) + "/osv"}) {
		t.Errorf("guard pair: %v", got)
	}
	if got := pairsFor(t, pool, "CVE-2030-6"); !reflect.DeepEqual(got, []string{itoa(legacy) + "/osv"}) {
		t.Errorf("legacy pair should have moved to the CVE: %v", got)
	}
	var aliases []string
	_ = pool.QueryRow(ctx, `SELECT aliases FROM advisories WHERE id='CVE-2030-6'`).Scan(&aliases)
	if !reflect.DeepEqual(aliases, []string{"GHSA-guard6"}) {
		t.Errorf("aliases: %v", aliases)
	}
	// The existing row's higher-precedence score was not replaced by the incoming LOW label.
	var sev string
	_ = pool.QueryRow(ctx, `SELECT severity FROM advisories WHERE id='CVE-2030-3'`).Scan(&sev)
	if sev != "high" { // the incoming bare LOW label must not override the stored 7.5 score
		t.Errorf("severity %q", sev)
	}

	// A second collector run and a dedupe pass both leave it at one row each.
	if _, err := o.CollectStats(ctx, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if st, err := Dedupe(ctx, pool, false); err != nil || st.Groups != 0 {
		t.Errorf("dedupe after collect: %+v %v", st, err)
	}
	if got := advisoryIDs(t, pool); len(got) != 2 {
		t.Errorf("rows after rerun: %v", got)
	}
}
