package version

import "strings"

// CPEMatch is one cpeMatch entry of an NVD CVE configuration.
type CPEMatch struct {
	Vulnerable            bool   `json:"vulnerable"`
	Criteria              string `json:"criteria"`
	VersionStartIncluding string `json:"versionStartIncluding"`
	VersionStartExcluding string `json:"versionStartExcluding"`
	VersionEndIncluding   string `json:"versionEndIncluding"`
	VersionEndExcluding   string `json:"versionEndExcluding"`
}

// CPE holds the fields of a CPE 2.3 formatted string that matter here.
type CPE struct {
	Part, Vendor, Product, Version, Update string
}

// ParseCPE parses "cpe:2.3:a:vendor:product:version:update:...". Escaped
// colons inside fields are respected; other backslash escapes are removed.
func ParseCPE(s string) (CPE, bool) {
	fields := splitCPE(s)
	if len(fields) < 6 || fields[0] != "cpe" || fields[1] != "2.3" {
		return CPE{}, false
	}
	c := CPE{Part: fields[2], Vendor: unescape(fields[3]), Product: unescape(fields[4]), Version: unescape(fields[5])}
	if len(fields) > 6 {
		c.Update = unescape(fields[6])
	}
	return c, true
}

func splitCPE(s string) []string {
	var out []string
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s):
			b.WriteByte(s[i])
			b.WriteByte(s[i+1])
			i++
		case s[i] == ':':
			out = append(out, b.String())
			b.Reset()
		default:
			b.WriteByte(s[i])
		}
	}
	return append(out, b.String())
}

func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// Matches reports whether a component at version v (vendor/product as in the
// CPE dictionary) is affected according to one cpeMatch entry.
//
//   - not vulnerable, non-application or other vendor/product entries never match;
//   - a wildcard version with bounds matches when v is inside the range
//     (unorderable versions such as "r22" do not match);
//   - a wildcard version without bounds matches every version;
//   - an exact version matches an equal version; "-" (not applicable) and
//     pre-release update fields (rc1, beta) never match a plain version.
func (m CPEMatch) Matches(vendor, product, v string) bool {
	if !m.Vulnerable {
		return false
	}
	c, ok := ParseCPE(m.Criteria)
	if !ok || c.Part != "a" || !strings.EqualFold(c.Vendor, vendor) || !strings.EqualFold(c.Product, product) {
		return false
	}
	r := Range{
		StartIncluding: m.VersionStartIncluding, StartExcluding: m.VersionStartExcluding,
		EndIncluding: m.VersionEndIncluding, EndExcluding: m.VersionEndExcluding,
	}
	switch c.Version {
	case "*":
		if r.IsZero() {
			return true
		}
		in, ok := r.Contains(v)
		return in && ok
	case "-":
		return false
	}
	if !r.IsZero() {
		return false // an exact version never carries bounds; treat as malformed
	}
	if c.Update != "" && c.Update != "*" && c.Update != "-" {
		return false
	}
	if cmp, ok := Compare(v, c.Version); ok {
		return cmp == 0
	}
	return strings.EqualFold(v, c.Version)
}
