package advisory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/codebyaadi/lookfar/internal/stack"
)

// loadNVD reads a recorded API page and returns its first CVE.
func loadNVD(t *testing.T, id string) (nvdCVE, json.RawMessage) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "nvd", id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var page nvdPage
	if err := json.Unmarshal(b, &page); err != nil || len(page.Vulnerabilities) != 1 {
		t.Fatalf("fixture %s: %v", id, err)
	}
	var c nvdCVE
	if err := json.Unmarshal(page.Vulnerabilities[0].CVE, &c); err != nil {
		t.Fatal(err)
	}
	return c, page.Vulnerabilities[0].CVE
}

func pairsOf(name string) []stack.CPEPair {
	s, _ := stack.LookupServer(name)
	return s.CPEs
}

func TestNVDAffectsFixtures(t *testing.T) {
	nginxCVE, _ := loadNVD(t, "CVE-2022-41741") // range 1.1.3..1.22.0 plus exact 1.23.0 and 1.23.1
	pgCVE, _ := loadNVD(t, "CVE-2023-5868")     // ranges per branch (11.0-11.22 ... 15.0-15.5) plus exact 16.0
	tests := []struct {
		name string
		cve  nvdCVE
		srv  string
		ver  string
		want bool
	}{
		{"nginx at the range end (inclusive)", nginxCVE, "nginx", "1.22.0", true},
		{"nginx inside the range", nginxCVE, "nginx", "1.18.0", true},
		{"nginx below the range start", nginxCVE, "nginx", "1.0.0", false},
		{"nginx exact version 1.23.1", nginxCVE, "nginx", "1.23.1", true},
		{"nginx exact miss 1.23.2", nginxCVE, "nginx", "1.23.2", false},
		{"nginx later release", nginxCVE, "nginx", "1.25.3", false},
		{"nginx 1.10.0 is inside the range (ingress-controller entries are another product)", nginxCVE, "nginx", "1.10.0", true}, // via the nginx range, not ingress
		{"postgres 11.5 in branch range", pgCVE, "postgresql", "11.5", true},
		{"postgres 11.22 at exclusive end", pgCVE, "postgresql", "11.22", false},
		{"postgres 16.0 exact", pgCVE, "postgresql", "16.0", true},
		{"postgres 16.4 fixed", pgCVE, "postgresql", "16.4", false},
		{"postgres 10.1 older branch not listed", pgCVE, "postgresql", "10.1", false},
		{"wrong product", pgCVE, "nginx", "1.22.0", false},
	}
	for _, tc := range tests {
		if got := tc.cve.affects(pairsOf(tc.srv), tc.ver); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestNormalizeNVDFixtures(t *testing.T) {
	tests := []struct {
		id       string
		severity string
		cvss     float64
		hasCfg   bool
	}{
		{"CVE-2022-41741", "high", 7.8, true},  // NVD primary 7.8 beats the CNA's 7.0
		{"CVE-2023-5868", "medium", 4.3, true}, // only a secondary (CNA) score exists
		{"CVE-2021-23017", "high", 7.7, true},  // v3.1 is used although a v2 metric exists
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			c, _ := loadNVD(t, tc.id)
			a := normalizeNVD(c)
			if a.ID != tc.id || !a.HasScore || a.CVSS == nil {
				t.Fatalf("unexpected: %+v", a)
			}
			if a.Severity != severityFromScore(tc.cvss) || *a.CVSS != tc.cvss {
				t.Errorf("severity %s cvss %v, want %v", a.Severity, *a.CVSS, tc.cvss)
			}
			if a.Summary == "" || a.ModifiedAt == nil || a.Exploited || a.ExploitedSince != nil {
				t.Errorf("fields: %+v", a)
			}
			var affected, refs []map[string]any
			if err := json.Unmarshal(a.Affected, &affected); err != nil || len(affected) != 1 || affected[0]["source"] != "nvd" {
				t.Errorf("affected: %s", a.Affected)
			}
			if err := json.Unmarshal(a.References, &refs); err != nil || len(refs) < 2 || refs[0]["source"] != "nvd" {
				t.Errorf("refs: %s", a.References)
			}
		})
	}
}

func TestNVDScorePreference(t *testing.T) {
	m := func(typ string, score float64) nvdMetric {
		var x nvdMetric
		x.Type = typ
		x.CVSSData.BaseScore = score
		return x
	}
	tests := []struct {
		name string
		c    nvdCVE
		want float64
		ok   bool
	}{
		{"v4.0 wins over v3.1", withMetrics([]nvdMetric{m("Primary", 9.3)}, []nvdMetric{m("Primary", 5.0)}, nil), 9.3, true},
		{"v3.1 wins over v3.0", withMetrics(nil, []nvdMetric{m("Primary", 6.1)}, []nvdMetric{m("Primary", 9.0)}), 6.1, true},
		{"v3.0 as last resort", withMetrics(nil, nil, []nvdMetric{m("Primary", 8.8)}), 8.8, true},
		{"primary beats secondary within a version", withMetrics(nil, []nvdMetric{m("Secondary", 9.8), m("Primary", 7.5)}, nil), 7.5, true},
		{"secondary used when it is all there is", withMetrics(nil, []nvdMetric{m("Secondary", 4.3)}, nil), 4.3, true},
		{"no metrics", nvdCVE{}, 0, false},
	}
	for _, tc := range tests {
		got, ok := nvdScore(tc.c)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s: got %v, %v; want %v, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
	if a := normalizeNVD(nvdCVE{ID: "CVE-1"}); a.HasScore || a.Severity != "unknown" || a.CVSS != nil {
		t.Errorf("unscored CVE: %+v", a)
	}
}

func withMetrics(v40, v31, v30 []nvdMetric) nvdCVE {
	var c nvdCVE
	c.Metrics.V40, c.Metrics.V31, c.Metrics.V30 = v40, v31, v30
	return c
}

func TestParseNVDTime(t *testing.T) {
	for _, in := range []string{"2026-06-17T05:03:44.713", "2026-06-17T05:03:44", "2026-06-17T05:03:44Z"} {
		if parseNVDTime(in) == nil {
			t.Errorf("parseNVDTime(%q) = nil", in)
		}
	}
	if parseNVDTime("garbage") != nil || parseNVDTime("") != nil {
		t.Error("bad input must give nil")
	}
}
