package advisory

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func f64(v float64) *float64 { return &v }

func day(s string) *time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return &t
}

func els(source string, extra ...string) json.RawMessage {
	m := map[string]any{"source": source}
	if len(extra)%2 == 0 {
		for i := 0; i < len(extra); i += 2 {
			m[extra[i]] = extra[i+1]
		}
	}
	b, _ := json.Marshal([]map[string]any{m})
	return b
}

func kevRow(id string) Advisory {
	return Advisory{
		ID: id, Summary: "KEV: name", Severity: "unknown", Exploited: true, ExploitedSince: day("2023-05-01"),
		Affected: els("kev", "product", "Redis"), References: els("kev", "type", "kev"),
	}
}

func osvRow(id string, aliases []string, sev string, score float64) Advisory {
	return Advisory{
		ID: id, Aliases: aliases, Summary: "OSV summary", Severity: sev, CVSS: f64(score),
		Affected: els("osv", "osvId", id), References: els("osv", "osvId", id), ModifiedAt: day("2024-01-01"),
	}
}

func nvdRow(id string, sev string, score *float64) Advisory {
	return Advisory{
		ID: id, Summary: "NVD summary", Severity: sev, CVSS: score,
		Affected: els("nvd"), References: els("nvd"), ModifiedAt: day("2025-01-01"),
	}
}

func sources(arr json.RawMessage) map[string]int {
	var l []map[string]any
	_ = json.Unmarshal(arr, &l)
	out := map[string]int{}
	for _, e := range l {
		out[e["source"].(string)]++
	}
	return out
}

func TestCanonicalFor(t *testing.T) {
	tests := []struct {
		name           string
		rowIDs, idents []string
		want           string
	}{
		{"cve among idents wins over ghsa row", []string{"GHSA-b"}, []string{"GHSA-b", "CVE-2020-5"}, "CVE-2020-5"},
		{"smallest cve", []string{"GHSA-b", "CVE-2020-9"}, []string{"GHSA-b", "CVE-2020-9", "CVE-2020-10"}, "CVE-2020-10"},
		{"no cve: smallest row id", []string{"GO-2024-1", "GHSA-b"}, []string{"GO-2024-1", "GHSA-b", "PYSEC-1"}, "GHSA-b"},
		{"lowercase cve prefix", []string{"X"}, []string{"cve-2020-1"}, "cve-2020-1"},
	}
	for _, tc := range tests {
		if got := canonicalFor(tc.rowIDs, tc.idents); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestMergeKEVAndOSV(t *testing.T) {
	m := mergeAdvisories("CVE-2023-1", []Advisory{
		kevRow("CVE-2023-1"),
		osvRow("GHSA-aaaa", []string{"CVE-2023-1"}, "high", 8.1),
	})
	if m.ID != "CVE-2023-1" || !reflect.DeepEqual(m.Aliases, []string{"GHSA-aaaa"}) {
		t.Errorf("id/aliases: %s %v", m.ID, m.Aliases)
	}
	if !m.Exploited || m.ExploitedSince == nil || !m.ExploitedSince.Equal(*day("2023-05-01")) {
		t.Errorf("exploited fields lost: %+v", m)
	}
	if m.Severity != "high" || m.CVSS == nil || *m.CVSS != 8.1 {
		t.Errorf("OSV severity should win over KEV's unknown: %s %v", m.Severity, m.CVSS)
	}
	if m.Summary != "KEV: name" { // canonical row's summary first
		t.Errorf("summary %q", m.Summary)
	}
	if s := sources(m.Affected); s["kev"] != 1 || s["osv"] != 1 {
		t.Errorf("affected elements: %v", s)
	}
	if s := sources(m.References); s["kev"] != 1 || s["osv"] != 1 {
		t.Errorf("refs elements: %v", s)
	}
	if m.ModifiedAt == nil || !m.ModifiedAt.Equal(*day("2024-01-01")) {
		t.Errorf("modified_at: %v", m.ModifiedAt)
	}
}

func TestMergeOSVAndNVDPrecedence(t *testing.T) {
	// OSV scored 7.1 (high); NVD says 9.8 (critical). OSV wins, as the collectors do.
	m := mergeAdvisories("CVE-2022-2", []Advisory{
		nvdRow("CVE-2022-2", "critical", f64(9.8)),
		osvRow("GHSA-bbbb", []string{"CVE-2022-2"}, "high", 7.1),
	})
	if m.Severity != "high" || *m.CVSS != 7.1 {
		t.Errorf("OSV must win: %s %v", m.Severity, *m.CVSS)
	}
	if !m.ModifiedAt.Equal(*day("2025-01-01")) { // newest modified_at
		t.Errorf("modified_at: %v", m.ModifiedAt)
	}
	if s := sources(m.Affected); s["osv"] != 1 || s["nvd"] != 1 {
		t.Errorf("affected: %v", s)
	}
}

func TestMergeNVDFillsGaps(t *testing.T) {
	// OSV knows a label but no score; NVD supplies the score only.
	osv := osvRow("GHSA-cccc", []string{"CVE-2021-3"}, "medium", 0)
	osv.CVSS = nil
	m := mergeAdvisories("CVE-2021-3", []Advisory{osv, nvdRow("CVE-2021-3", "high", f64(7.5))})
	if m.Severity != "medium" || m.CVSS == nil || *m.CVSS != 7.5 {
		t.Errorf("severity stays OSV's label, cvss filled from NVD: %s %v", m.Severity, m.CVSS)
	}
	// Nobody has a severity but one row has a score: derive the label from it.
	m = mergeAdvisories("CVE-2021-4", []Advisory{kevRow("CVE-2021-4"), nvdRow("CVE-2021-4", "unknown", f64(9.1))})
	if m.Severity != "critical" || *m.CVSS != 9.1 {
		t.Errorf("derive severity: %s %v", m.Severity, m.CVSS)
	}
	// Nothing known at all.
	m = mergeAdvisories("CVE-2021-5", []Advisory{kevRow("CVE-2021-5"), kevRow("GHSA-x")})
	if m.Severity != "unknown" || m.CVSS != nil {
		t.Errorf("unknown stays unknown: %s %v", m.Severity, m.CVSS)
	}
}

func TestMergeThreeSources(t *testing.T) {
	m := mergeAdvisories("CVE-2020-1", []Advisory{
		kevRow("CVE-2020-1"),
		osvRow("GHSA-1111", []string{"CVE-2020-1", "PYSEC-9"}, "critical", 9.8),
		osvRow("GO-2020-1", []string{"CVE-2020-1", "GHSA-1111"}, "unknown", 0),
		nvdRow("CVE-2020-1", "high", f64(7.5)),
	})
	want := []string{"GHSA-1111", "GO-2020-1", "PYSEC-9"}
	if !reflect.DeepEqual(m.Aliases, want) {
		t.Errorf("aliases %v, want %v", m.Aliases, want)
	}
	if m.Severity != "critical" || *m.CVSS != 9.8 || !m.Exploited {
		t.Errorf("merged: %+v", m)
	}
	if s := sources(m.Affected); s["kev"] != 1 || s["osv"] != 2 || s["nvd"] != 1 {
		t.Errorf("affected: %v", s)
	}
}

func TestMergeEarliestExploitedAndIdempotence(t *testing.T) {
	a, b := kevRow("CVE-1"), kevRow("GHSA-z")
	b.ExploitedSince = day("2021-01-01")
	m := mergeAdvisories("CVE-1", []Advisory{a, b})
	if !m.ExploitedSince.Equal(*day("2021-01-01")) {
		t.Errorf("earliest date should win: %v", m.ExploitedSince)
	}
	// Merging an already merged row with itself changes nothing.
	again := mergeAdvisories("CVE-1", []Advisory{m})
	if !reflect.DeepEqual(again.Aliases, m.Aliases) || again.Severity != m.Severity ||
		string(again.Affected) != string(m.Affected) || again.Exploited != m.Exploited {
		t.Errorf("not idempotent:\n%+v\n%+v", m, again)
	}
}

func TestUnionElementsDedupesEqualObjects(t *testing.T) {
	a := json.RawMessage(`[{"source":"osv","osvId":"X","n":1}]`)
	b := json.RawMessage(`[{"n":1,"osvId":"X","source":"osv"},{"source":"kev"}]`) // same object, different key order
	var got []map[string]any
	if err := json.Unmarshal(unionElements(a, b, json.RawMessage(`not json`)), &got); err != nil || len(got) != 2 {
		t.Errorf("got %v %v", got, err)
	}
}

func TestWithAlias(t *testing.T) {
	got := withAlias([]string{"B", "A", "CVE-1"}, "CVE-1", "GHSA-x", "A", "")
	if !reflect.DeepEqual(got, []string{"A", "B", "GHSA-x"}) {
		t.Errorf("got %v", got)
	}
}
