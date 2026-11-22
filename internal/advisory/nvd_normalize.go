package advisory

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/codebyaadi/lookfar/internal/stack"
	"github.com/codebyaadi/lookfar/internal/version"
)

// nvdCVE is the part of an NVD CVE API 2.0 record lookfar uses.
type nvdCVE struct {
	ID           string `json:"id"`
	Published    string `json:"published"`
	LastModified string `json:"lastModified"`
	Descriptions []struct {
		Lang  string `json:"lang"`
		Value string `json:"value"`
	} `json:"descriptions"`
	Metrics struct {
		V40 []nvdMetric `json:"cvssMetricV40"`
		V31 []nvdMetric `json:"cvssMetricV31"`
		V30 []nvdMetric `json:"cvssMetricV30"`
	} `json:"metrics"`
	Configurations []struct {
		Nodes []struct {
			Operator string             `json:"operator"`
			Negate   bool               `json:"negate"`
			CPEMatch []version.CPEMatch `json:"cpeMatch"`
		} `json:"nodes"`
	} `json:"configurations"`
	References []struct {
		URL  string   `json:"url"`
		Tags []string `json:"tags"`
	} `json:"references"`
}

type nvdMetric struct {
	Source   string `json:"source"`
	Type     string `json:"type"` // Primary (NVD) or Secondary (a CNA)
	CVSSData struct {
		Version      string  `json:"version"`
		VectorString string  `json:"vectorString"`
		BaseScore    float64 `json:"baseScore"`
	} `json:"cvssData"`
}

// nvdScore picks the CVSS score: v4.0, then v3.1, then v3.0; within a version
// NVD's own (Primary) score wins over a CNA's (Secondary). ok is false when
// the record has no usable CVSS metric (for example still awaiting analysis).
func nvdScore(c nvdCVE) (score float64, ok bool) {
	for _, list := range [][]nvdMetric{c.Metrics.V40, c.Metrics.V31, c.Metrics.V30} {
		if len(list) == 0 {
			continue
		}
		best := list[0]
		for _, m := range list {
			if m.Type == "Primary" {
				best = m
				break
			}
		}
		return best.CVSSData.BaseScore, true
	}
	return 0, false
}

// nvdAdvisory is the NVD view of an advisory, ready to upsert.
type nvdAdvisory struct {
	Advisory
	HasScore bool
}

// normalizeNVD maps a CVE record to the NVD-owned fields of an advisory.
// Elements are tagged source=nvd; severity and cvss are only set when the
// record carries a CVSS score.
func normalizeNVD(c nvdCVE) nvdAdvisory {
	summary := ""
	for _, d := range c.Descriptions {
		if d.Lang == "en" {
			summary = strings.TrimSpace(d.Value)
			break
		}
	}
	if len(summary) > 300 {
		summary = summary[:300]
	}

	severity := "unknown"
	var cvss *float64
	score, ok := nvdScore(c)
	if ok {
		severity = severityFromScore(score)
		s := score
		cvss = &s
	}

	var matches []map[string]string
	for _, cfg := range c.Configurations {
		for _, n := range cfg.Nodes {
			for _, m := range n.CPEMatch {
				if !m.Vulnerable {
					continue
				}
				el := map[string]string{"criteria": m.Criteria}
				for k, v := range map[string]string{
					"versionStartIncluding": m.VersionStartIncluding, "versionStartExcluding": m.VersionStartExcluding,
					"versionEndIncluding": m.VersionEndIncluding, "versionEndExcluding": m.VersionEndExcluding,
				} {
					if v != "" {
						el[k] = v
					}
				}
				matches = append(matches, el)
			}
		}
	}
	affected := []map[string]any{}
	if len(matches) > 0 {
		affected = append(affected, map[string]any{"source": "nvd", "cpeMatches": matches})
	}
	refs := []map[string]any{{"source": "nvd", "type": "ADVISORY", "url": "https://nvd.nist.gov/vuln/detail/" + c.ID}}
	for _, r := range c.References {
		if r.URL != "" {
			refs = append(refs, map[string]any{"source": "nvd", "type": "WEB", "url": r.URL, "tags": r.Tags})
		}
	}
	affectedJSON, _ := json.Marshal(affected)
	refsJSON, _ := json.Marshal(refs)

	return nvdAdvisory{
		HasScore: ok,
		Advisory: Advisory{
			ID: c.ID, Summary: summary, Severity: severity, CVSS: cvss,
			Affected: affectedJSON, References: refsJSON, ModifiedAt: parseNVDTime(c.LastModified),
		},
	}
}

const nvdTimeLayout = "2006-01-02T15:04:05.000"

// parseNVDTime reads NVD timestamps, which are UTC without a zone suffix.
func parseNVDTime(s string) *time.Time {
	for _, layout := range []string{nvdTimeLayout, "2006-01-02T15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return &t
		}
	}
	return nil
}

// affects reports whether the CVE's configurations say that a component of
// the given server at the given version is affected. AND-nodes (software
// running on a platform) are treated like OR: any vulnerable entry for the
// product that matches the version counts, which can over-match slightly.
func (c nvdCVE) affects(pairs []stack.CPEPair, v string) bool {
	for _, cfg := range c.Configurations {
		for _, n := range cfg.Nodes {
			for _, m := range n.CPEMatch {
				for _, p := range pairs {
					if m.Matches(p.Vendor, p.Product, v) {
						return true
					}
				}
			}
		}
	}
	return false
}
