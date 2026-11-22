package advisory

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	cvss30 "github.com/pandatix/go-cvss/30"
	cvss31 "github.com/pandatix/go-cvss/31"
	cvss40 "github.com/pandatix/go-cvss/40"
)

// osvVuln is the part of an OSV record lookfar uses.
type osvVuln struct {
	ID       string   `json:"id"`
	Modified string   `json:"modified"`
	Aliases  []string `json:"aliases"`
	Summary  string   `json:"summary"`
	Details  string   `json:"details"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
	Affected []struct {
		Package struct {
			Name      string `json:"name"`
			Ecosystem string `json:"ecosystem"`
			Purl      string `json:"purl"`
		} `json:"package"`
		Ranges   json.RawMessage `json:"ranges"`
		Versions json.RawMessage `json:"versions"`
	} `json:"affected"`
	References []struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"references"`
	DatabaseSpecific struct {
		Severity string `json:"severity"`
	} `json:"database_specific"`
}

// osvAdvisory is the OSV view of an advisory, ready to upsert.
type osvAdvisory struct {
	Advisory
	OSVID string
}

// normalizeOSV maps an OSV record to an advisory. The canonical id is the CVE
// alias when there is one (the smallest if several), otherwise the OSV id; every
// other identifier goes into Aliases.
func normalizeOSV(v osvVuln) osvAdvisory {
	canonical := v.ID
	var cves []string
	for _, a := range v.Aliases {
		if strings.HasPrefix(strings.ToUpper(a), "CVE-") {
			cves = append(cves, a)
		}
	}
	if strings.HasPrefix(strings.ToUpper(v.ID), "CVE-") {
		cves = append(cves, v.ID)
	}
	if len(cves) > 0 {
		sort.Strings(cves)
		canonical = cves[0]
	}

	seen := map[string]bool{canonical: true}
	aliases := []string{}
	for _, id := range append([]string{v.ID}, v.Aliases...) {
		if id != "" && !seen[id] {
			seen[id] = true
			aliases = append(aliases, id)
		}
	}
	sort.Strings(aliases)

	severity, score := osvSeverity(v)

	affected := make([]map[string]any, 0, len(v.Affected))
	for _, a := range v.Affected {
		el := map[string]any{
			"source": "osv", "osvId": v.ID,
			"package": map[string]any{"name": a.Package.Name, "ecosystem": a.Package.Ecosystem, "purl": a.Package.Purl},
		}
		if len(a.Ranges) > 0 {
			el["ranges"] = a.Ranges
		}
		if len(a.Versions) > 0 {
			el["versions"] = a.Versions
		}
		affected = append(affected, el)
	}
	refs := []map[string]any{{"source": "osv", "osvId": v.ID, "type": "ADVISORY", "url": "https://osv.dev/vulnerability/" + v.ID}}
	for _, r := range v.References {
		if r.URL != "" {
			refs = append(refs, map[string]any{"source": "osv", "osvId": v.ID, "type": r.Type, "url": r.URL})
		}
	}
	affectedJSON, _ := json.Marshal(affected)
	refsJSON, _ := json.Marshal(refs)

	var modified *time.Time
	if t, err := time.Parse(time.RFC3339, v.Modified); err == nil {
		modified = &t
	}
	return osvAdvisory{
		OSVID: v.ID,
		Advisory: Advisory{
			ID: canonical, Aliases: aliases, Summary: osvSummary(v), Severity: severity, CVSS: score,
			Affected: affectedJSON, References: refsJSON, ModifiedAt: modified,
		},
	}
}

func osvSummary(v osvVuln) string {
	if s := strings.TrimSpace(v.Summary); s != "" {
		return s
	}
	first, _, _ := strings.Cut(strings.TrimSpace(v.Details), "\n")
	if len(first) > 300 {
		first = first[:300]
	}
	return first
}

// osvSeverity prefers a score computed from a CVSS vector, then the database
// label (GHSA: LOW, MODERATE, HIGH, CRITICAL), else "unknown".
func osvSeverity(v osvVuln) (string, *float64) {
	var best *float64
	for _, s := range v.Severity {
		if sc, ok := cvssScore(s.Score); ok && (best == nil || sc > *best) {
			sc := sc
			best = &sc
		}
	}
	if best != nil {
		return severityFromScore(*best), best
	}
	switch strings.ToUpper(v.DatabaseSpecific.Severity) {
	case "LOW":
		return "low", nil
	case "MODERATE", "MEDIUM":
		return "medium", nil
	case "HIGH":
		return "high", nil
	case "CRITICAL":
		return "critical", nil
	}
	return "unknown", nil
}

// cvssScore scores a CVSS 3.0, 3.1 or 4.0 vector, or reads a plain number.
func cvssScore(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "CVSS:3.1/"):
		if v, err := cvss31.ParseVector(s); err == nil {
			return v.BaseScore(), true
		}
	case strings.HasPrefix(s, "CVSS:3.0/"):
		if v, err := cvss30.ParseVector(s); err == nil {
			return v.BaseScore(), true
		}
	case strings.HasPrefix(s, "CVSS:4.0/"):
		if v, err := cvss40.ParseVector(s); err == nil {
			return v.Score(), true
		}
	default:
		if f, err := strconv.ParseFloat(s, 64); err == nil && f >= 0 && f <= 10 {
			return f, true
		}
	}
	return 0, false
}

func severityFromScore(score float64) string {
	switch {
	case score >= 9:
		return "critical"
	case score >= 7:
		return "high"
	case score >= 4:
		return "medium"
	case score > 0:
		return "low"
	}
	return "unknown"
}
