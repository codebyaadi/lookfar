package version

import (
	"fmt"
	"testing"
)

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
		ok   bool
	}{
		{"16.4", "16.10", -1, true}, // numeric, not lexical
		{"16.10", "16.4", 1, true},
		{"1.25.3", "1.25.4", -1, true},
		{"1.25.4", "1.25.4", 0, true},
		{"1.0", "1.0.0", 0, true}, // trailing zeros are insignificant
		{"1.0", "1.0.1", -1, true},
		{"2", "10", -1, true},
		{"v1.2.3", "1.2.3", 0, true}, // leading v
		{"1.0.2k", "1.0.2", 1, true}, // openssl letter releases come after
		{"1.0.2k", "1.0.2l", -1, true},
		{"8.9p1", "8.9", 1, true}, // openssh portable suffix
		{"8.9p1", "8.9p2", -1, true},
		{"1.0.0-beta1", "1.0.0", -1, true}, // pre-release before release
		{"1.0.0-rc1", "1.0.0-rc2", -1, true},
		{"1.0.0-alpha", "1.0.0-beta", -1, true},
		{"3.0.0-beta1", "2.9.9", 1, true},
		{"7.2.4-alpine", "7.2.4", 1, true}, // unknown suffix counts as later
		{"1.22.0", "1.22", 0, true},
		{"r22", "1.0", 0, false}, // not comparable
		{"1.0", "Mainline", 0, false},
		{"", "1.0", 0, false},
		{"99999999999999999999999.1", "1.0", 1, true}, // overflow clamps instead of panicking
	}
	for _, tc := range tests {
		got, ok := Compare(tc.a, tc.b)
		if ok != tc.ok || ok && got != tc.want {
			t.Errorf("Compare(%q, %q) = %d, %v; want %d, %v", tc.a, tc.b, got, ok, tc.want, tc.ok)
		}
	}
}

func TestRangeContains(t *testing.T) {
	tests := []struct {
		name   string
		r      Range
		v      string
		in, ok bool
	}{
		{"end excluding, just below", Range{EndExcluding: "1.25.4"}, "1.25.3", true, true},
		{"end excluding, equal", Range{EndExcluding: "1.25.4"}, "1.25.4", false, true},
		{"end including, equal", Range{EndIncluding: "1.25.4"}, "1.25.4", true, true},
		{"start including, equal", Range{StartIncluding: "11.0", EndExcluding: "11.22"}, "11.0", true, true},
		{"start excluding, equal", Range{StartExcluding: "11.0", EndExcluding: "11.22"}, "11.0", false, true},
		{"inside the 16 branch", Range{StartIncluding: "16.0", EndExcluding: "16.5"}, "16.4", true, true},
		{"16.10 is outside 16.0-16.5", Range{StartIncluding: "16.0", EndExcluding: "16.5"}, "16.10", false, true},
		{"below the start", Range{StartIncluding: "12.0", EndExcluding: "12.17"}, "11.22", false, true},
		{"open range", Range{}, "5.0", true, true},
		{"unorderable bound", Range{StartIncluding: "r22", EndIncluding: "r27"}, "1.22", false, false},
		{"unorderable version", Range{EndExcluding: "1.5"}, "latest", false, false},
	}
	for _, tc := range tests {
		in, ok := tc.r.Contains(tc.v)
		if in != tc.in || ok != tc.ok {
			t.Errorf("%s: Contains(%q) = %v, %v; want %v, %v", tc.name, tc.v, in, ok, tc.in, tc.ok)
		}
	}
}

func TestParseCPE(t *testing.T) {
	c, ok := ParseCPE(`cpe:2.3:a:f5:nginx:1.23.0:*:*:*:open_source:*:*:*`)
	if !ok || c.Part != "a" || c.Vendor != "f5" || c.Product != "nginx" || c.Version != "1.23.0" {
		t.Errorf("got %+v %v", c, ok)
	}
	c, ok = ParseCPE(`cpe:2.3:a:nodejs:node\.js:*:*:*:*:*:*:*:*`)
	if !ok || c.Product != "node.js" {
		t.Errorf("escaped dot: %+v %v", c, ok)
	}
	c, ok = ParseCPE(`cpe:2.3:a:v\:x:p:1\:2:*:*:*:*:*:*:*`)
	if !ok || c.Vendor != "v:x" || c.Version != "1:2" {
		t.Errorf("escaped colon: %+v %v", c, ok)
	}
	for _, bad := range []string{"", "cpe:2.2:a:x:y", "not a cpe", "cpe:2.3:a:x"} {
		if _, ok := ParseCPE(bad); ok {
			t.Errorf("ParseCPE(%q) should fail", bad)
		}
	}
}

func TestCPEMatchMatches(t *testing.T) {
	const pg = "cpe:2.3:a:postgresql:postgresql:%s:*:*:*:*:*:*:*"
	tests := []struct {
		name string
		m    CPEMatch
		v    string
		want bool
	}{
		{"range includes 16.4", CPEMatch{Vulnerable: true, Criteria: fmt.Sprintf(pg, "*"), VersionStartIncluding: "16.0", VersionEndExcluding: "16.5"}, "16.4", true},
		{"range excludes 16.10", CPEMatch{Vulnerable: true, Criteria: fmt.Sprintf(pg, "*"), VersionStartIncluding: "16.0", VersionEndExcluding: "16.5"}, "16.10", false},
		{"nginx 1.25.3 below exclusive 1.25.4", CPEMatch{Vulnerable: true, Criteria: "cpe:2.3:a:f5:nginx:*:*:*:*:open_source:*:*:*", VersionEndExcluding: "1.25.4"}, "1.25.3", true},
		{"nginx 1.25.4 at exclusive end", CPEMatch{Vulnerable: true, Criteria: "cpe:2.3:a:f5:nginx:*:*:*:*:open_source:*:*:*", VersionEndExcluding: "1.25.4"}, "1.25.4", false},
		{"exact version", CPEMatch{Vulnerable: true, Criteria: fmt.Sprintf(pg, "16.0")}, "16.0", true},
		{"exact version differs", CPEMatch{Vulnerable: true, Criteria: fmt.Sprintf(pg, "16.0")}, "16.1", false},
		{"exact 16.1 is not 16.10", CPEMatch{Vulnerable: true, Criteria: fmt.Sprintf(pg, "16.1")}, "16.10", false},
		{"wildcard without bounds matches all", CPEMatch{Vulnerable: true, Criteria: fmt.Sprintf(pg, "*")}, "9.6.1", true},
		{"not applicable version", CPEMatch{Vulnerable: true, Criteria: fmt.Sprintf(pg, "-")}, "16.4", false},
		{"not vulnerable (platform) entry", CPEMatch{Vulnerable: false, Criteria: fmt.Sprintf(pg, "16.4")}, "16.4", false},
		{"other product", CPEMatch{Vulnerable: true, Criteria: "cpe:2.3:a:postgresql:pgadmin:*:*:*:*:*:*:*:*"}, "16.4", false},
		{"other vendor", CPEMatch{Vulnerable: true, Criteria: "cpe:2.3:a:other:postgresql:*:*:*:*:*:*:*:*"}, "16.4", false},
		{"operating system part", CPEMatch{Vulnerable: true, Criteria: "cpe:2.3:o:postgresql:postgresql:*:*:*:*:*:*:*:*"}, "16.4", false},
		{"pre-release update does not match release", CPEMatch{Vulnerable: true, Criteria: "cpe:2.3:a:postgresql:postgresql:16.0:beta1:*:*:*:*:*:*"}, "16.0", false},
		{"unorderable bounds never match", CPEMatch{Vulnerable: true, Criteria: "cpe:2.3:a:f5:nginx:*:*:*:*:plus:*:*:*", VersionStartIncluding: "r22", VersionEndIncluding: "r27"}, "1.25.3", false},
		{"case-insensitive vendor", CPEMatch{Vulnerable: true, Criteria: "cpe:2.3:a:PostgreSQL:PostgreSQL:*:*:*:*:*:*:*:*"}, "16.4", true},
	}
	for _, tc := range tests {
		vendor, product := "postgresql", "postgresql"
		if c, _ := ParseCPE(tc.m.Criteria); c.Product == "nginx" {
			vendor, product = "f5", "nginx"
		}
		if got := tc.m.Matches(vendor, product, tc.v); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
