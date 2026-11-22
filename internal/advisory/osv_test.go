package advisory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func loadVuln(t *testing.T, id string) osvVuln {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "osv", id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var v osvVuln
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestNormalizeOSVFixtures(t *testing.T) {
	tests := []struct {
		osvID     string
		canonical string
		aliases   []string
		severity  string
		cvss      float64 // 0 = none
		summary   bool
	}{
		// GHSA with a CVE alias: the CVE is canonical, the GHSA id becomes an alias.
		{"GHSA-29mw-wpgm-hmr9", "CVE-2020-28500", []string{"GHSA-29mw-wpgm-hmr9"}, "medium", 5.3, true},
		// Several aliases: CVE canonical, everything else sorted into aliases; CVSS 3.1 wins by score.
		{"GHSA-2gwj-7jmv-h26r", "CVE-2022-28346", []string{"BIT-django-2022-28346", "GHSA-2gwj-7jmv-h26r", "PYSEC-2022-190"}, "critical", 9.8, true},
		// Go vulnerability database record for the standard library: no severity data.
		{"GO-2024-2598", "CVE-2024-24783", []string{"BIT-golang-2024-24783", "GO-2024-2598"}, "unknown", 0, true},
		{"PYSEC-2021-109", "CVE-2021-35042", []string{"BIT-django-2021-35042", "GHSA-xpfp-f569-q3p2", "PYSEC-2021-109"}, "unknown", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.osvID, func(t *testing.T) {
			a := normalizeOSV(loadVuln(t, tc.osvID))
			if a.ID != tc.canonical || a.OSVID != tc.osvID {
				t.Errorf("id = %s (osv %s), want %s", a.ID, a.OSVID, tc.canonical)
			}
			if len(a.Aliases) != len(tc.aliases) {
				t.Fatalf("aliases = %v, want %v", a.Aliases, tc.aliases)
			}
			for i := range tc.aliases {
				if a.Aliases[i] != tc.aliases[i] {
					t.Errorf("aliases = %v, want %v", a.Aliases, tc.aliases)
				}
			}
			if a.Severity != tc.severity {
				t.Errorf("severity = %s, want %s", a.Severity, tc.severity)
			}
			switch {
			case tc.cvss == 0 && a.CVSS != nil:
				t.Errorf("unexpected cvss %v", *a.CVSS)
			case tc.cvss != 0 && (a.CVSS == nil || *a.CVSS != tc.cvss):
				t.Errorf("cvss = %v, want %v", a.CVSS, tc.cvss)
			}
			if tc.summary && a.Summary == "" {
				t.Error("empty summary")
			}
			if a.ModifiedAt == nil || a.Exploited || a.ExploitedSince != nil {
				t.Errorf("modified/exploited fields wrong: %+v", a)
			}
			var affected []map[string]any
			if err := json.Unmarshal(a.Affected, &affected); err != nil || len(affected) == 0 {
				t.Fatalf("affected: %s (%v)", a.Affected, err)
			}
			for _, el := range affected {
				if el["source"] != "osv" || el["osvId"] != tc.osvID || el["package"] == nil || el["ranges"] == nil && el["versions"] == nil {
					t.Errorf("affected element: %v", el)
				}
			}
			var refs []map[string]any
			_ = json.Unmarshal(a.References, &refs)
			if len(refs) < 2 || refs[0]["url"] != "https://osv.dev/vulnerability/"+tc.osvID || refs[0]["source"] != "osv" {
				t.Errorf("refs: %s", a.References)
			}
		})
	}
}

func TestCanonicalIDRules(t *testing.T) {
	tests := []struct {
		name string
		v    osvVuln
		want string
	}{
		{"no cve", osvVuln{ID: "GHSA-aaaa", Aliases: []string{"PYSEC-1"}}, "GHSA-aaaa"},
		{"cve alias", osvVuln{ID: "GHSA-aaaa", Aliases: []string{"CVE-2020-2"}}, "CVE-2020-2"},
		{"smallest of several cves", osvVuln{ID: "X", Aliases: []string{"CVE-2020-9", "CVE-2020-10"}}, "CVE-2020-10"},
		{"id itself is a cve", osvVuln{ID: "CVE-2021-1", Aliases: []string{"GHSA-b"}}, "CVE-2021-1"},
	}
	for _, tc := range tests {
		if got := normalizeOSV(tc.v).ID; got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
	a := normalizeOSV(osvVuln{ID: "CVE-2021-1", Aliases: []string{"CVE-2021-1", "GHSA-b"}})
	if len(a.Aliases) != 1 || a.Aliases[0] != "GHSA-b" {
		t.Errorf("canonical id must not appear in aliases: %v", a.Aliases)
	}
}

func TestOSVSeverityFallbacks(t *testing.T) {
	label := func(l string) osvVuln {
		var v osvVuln
		v.DatabaseSpecific.Severity = l
		return v
	}
	for in, want := range map[string]string{"LOW": "low", "MODERATE": "medium", "HIGH": "high", "CRITICAL": "critical", "": "unknown", "weird": "unknown"} {
		if got, _ := osvSeverity(label(in)); got != want {
			t.Errorf("label %q -> %s, want %s", in, got, want)
		}
	}
	// A garbage vector falls back to the label.
	v := label("HIGH")
	v.Severity = []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	}{{Type: "CVSS_V3", Score: "CVSS:3.1/garbage"}}
	if got, sc := osvSeverity(v); got != "high" || sc != nil {
		t.Errorf("fallback: %s %v", got, sc)
	}
}

func TestCVSSScoring(t *testing.T) {
	tests := []struct {
		vector string
		want   float64
		ok     bool
	}{
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", 9.8, true},
		{"CVSS:3.0/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", 9.8, true},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:L", 5.3, true},
		{"CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N", 9.3, true},
		{"7.5", 7.5, true},
		{"CVSS:3.1/nope", 0, false},
		{"", 0, false},
	}
	for _, tc := range tests {
		got, ok := cvssScore(tc.vector)
		if ok != tc.ok || ok && got != tc.want {
			t.Errorf("cvssScore(%q) = %v, %v; want %v, %v", tc.vector, got, ok, tc.want, tc.ok)
		}
	}
	for score, want := range map[float64]string{0: "unknown", 0.1: "low", 3.9: "low", 4: "medium", 6.9: "medium", 7: "high", 8.9: "high", 9: "critical", 10: "critical"} {
		if got := severityFromScore(score); got != want {
			t.Errorf("severityFromScore(%v) = %s, want %s", score, got, want)
		}
	}
}

func TestSameModified(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"2026-09-10T03:50:11.106451165Z", "2026-09-10T03:50:11.106451Z", true}, // nanoseconds vs microseconds
		{"2026-02-04T03:12:08.722840Z", "2026-02-04T03:12:08.722840Z", true},
		{"2026-02-04T03:12:08Z", "2026-02-04T03:12:09Z", false},
		{"2026-02-04T03:12:08Z", "not a time", false},
		{"", "2026-02-04T03:12:08Z", false},
	}
	for _, tc := range tests {
		if got := sameModified(tc.a, tc.b); got != tc.want {
			t.Errorf("sameModified(%q, %q) = %v", tc.a, tc.b, got)
		}
	}
}
