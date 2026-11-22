package advisory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/codebyaadi/lookfar/internal/store"
)

// This file holds the single place that decides which advisories are the same
// vulnerability: the canonical id rule, the merge of several rows into one, and
// the write-time guard every collector calls before it writes.
//
// Canonical id: the CVE if any identifier of any member is a CVE (the smallest
// one if several, matching what the OSV normalizer does). Without a CVE the
// smallest existing row id wins, so the choice is stable.

func isCVE(id string) bool { return strings.HasPrefix(strings.ToUpper(id), "CVE-") }

// canonicalFor applies the canonical id rule. rowIDs are ids of existing or
// incoming rows; idents are every identifier seen (row ids plus aliases).
func canonicalFor(rowIDs, idents []string) string {
	var cves []string
	seen := map[string]bool{}
	for _, id := range idents {
		if isCVE(id) && !seen[id] {
			seen[id] = true
			cves = append(cves, id)
		}
	}
	if len(cves) > 0 {
		sort.Strings(cves)
		return cves[0]
	}
	ids := append([]string(nil), rowIDs...)
	sort.Strings(ids)
	return ids[0]
}

// rowAdvisory converts a stored row to the domain type.
func rowAdvisory(r store.Advisory) Advisory {
	a := Advisory{
		ID: r.ID, Aliases: r.Aliases, Summary: r.Summary, Severity: r.Severity,
		Exploited: r.Exploited, ExploitedSince: r.ExploitedSince, Affected: r.Affected,
		References: r.Refs, ModifiedAt: r.ModifiedAt,
	}
	if r.Cvss.Valid {
		if f, err := r.Cvss.Float64Value(); err == nil && f.Valid {
			v := math.Round(f.Float64*10) / 10
			a.CVSS = &v
		}
	}
	return a
}

// hasSource reports whether any tagged element in a jsonb array has source s.
func hasSource(arr json.RawMessage, s string) bool {
	var els []map[string]any
	if json.Unmarshal(arr, &els) != nil {
		return false
	}
	for _, e := range els {
		if e["source"] == s {
			return true
		}
	}
	return false
}

// unionElements concatenates jsonb arrays, dropping exact duplicates.
func unionElements(arrays ...json.RawMessage) json.RawMessage {
	seen := map[string]bool{}
	out := []json.RawMessage{}
	for _, arr := range arrays {
		var els []json.RawMessage
		if json.Unmarshal(arr, &els) != nil {
			continue
		}
		for _, el := range els {
			key := string(el)
			var generic any
			if json.Unmarshal(el, &generic) == nil {
				if b, err := json.Marshal(generic); err == nil {
					key = string(b) // map keys sorted: equal objects compare equal
				}
			}
			if !seen[key] {
				seen[key] = true
				out = append(out, el)
			}
		}
	}
	b, _ := json.Marshal(out)
	return b
}

// mergeAdvisories combines several rows that describe one vulnerability into
// the row stored under canonical.
//
//   - aliases: every member id and alias, minus the canonical id;
//   - exploited: true if any member says so, with the earliest exploited_since;
//   - severity/cvss: the existing source precedence. A member with OSV elements
//     beats one without (OSV owns the score, NVD only fills gaps); among equals
//     the higher score wins; a missing cvss is filled from any other member;
//   - summary: the first non-empty one, canonical row first;
//   - affected and refs: the union of the per-source tagged elements;
//   - modified_at: newest; created_at: oldest.
func mergeAdvisories(canonical string, rows []Advisory) Advisory {
	out := Advisory{ID: canonical, Severity: "unknown"}

	// Canonical row first so its summary wins ties; otherwise by id for determinism.
	ordered := append([]Advisory(nil), rows...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if (ordered[i].ID == canonical) != (ordered[j].ID == canonical) {
			return ordered[i].ID == canonical
		}
		return ordered[i].ID < ordered[j].ID
	})

	aliasSet := map[string]bool{}
	var affected, refs []json.RawMessage
	for _, r := range ordered {
		aliasSet[r.ID] = true
		for _, a := range r.Aliases {
			aliasSet[a] = true
		}
		if out.Summary == "" {
			out.Summary = r.Summary
		}
		if r.Exploited {
			out.Exploited = true
			if r.ExploitedSince != nil && (out.ExploitedSince == nil || r.ExploitedSince.Before(*out.ExploitedSince)) {
				t := *r.ExploitedSince
				out.ExploitedSince = &t
			}
		}
		if r.ModifiedAt != nil && (out.ModifiedAt == nil || r.ModifiedAt.After(*out.ModifiedAt)) {
			t := *r.ModifiedAt
			out.ModifiedAt = &t
		}
		affected = append(affected, r.Affected)
		refs = append(refs, r.References)
	}
	delete(aliasSet, canonical)
	out.Aliases = make([]string, 0, len(aliasSet))
	for a := range aliasSet {
		out.Aliases = append(out.Aliases, a)
	}
	sort.Strings(out.Aliases)
	out.Affected = unionElements(affected...)
	out.References = unionElements(refs...)

	// Severity: pick the winning member among those that know a severity.
	var best *Advisory
	for i := range ordered {
		r := &ordered[i]
		if r.Severity == "unknown" {
			continue
		}
		if best == nil || better(*r, *best) {
			best = r
		}
	}
	if best != nil {
		out.Severity = best.Severity
		out.CVSS = best.CVSS
	}
	if out.CVSS == nil {
		for _, r := range ordered {
			if r.CVSS != nil {
				v := *r.CVSS
				out.CVSS = &v
				if out.Severity == "unknown" {
					out.Severity = severityFromScore(v)
				}
				break
			}
		}
	}
	return out
}

// better reports whether a's severity should win over b's.
func better(a, b Advisory) bool {
	ao, bo := hasSource(a.Affected, "osv") || hasSource(a.References, "osv"), hasSource(b.Affected, "osv") || hasSource(b.References, "osv")
	if ao != bo {
		return ao
	}
	switch {
	case a.CVSS != nil && b.CVSS == nil:
		return true
	case a.CVSS == nil && b.CVSS != nil:
		return false
	case a.CVSS != nil && b.CVSS != nil && *a.CVSS != *b.CVSS:
		return *a.CVSS > *b.CVSS
	}
	return false
}

// MergeStats counts what a merge did.
type MergeStats struct {
	RowsMerged int // rows deleted into the canonical one
	Repointed  int // component_advisories rows written under the canonical id
}

// mergeInto merges the rows with the given ids into canonical (which may or
// may not exist yet) inside the caller's transaction: it writes the merged row,
// repoints component_advisories and deletes the other rows.
func mergeInto(ctx context.Context, q *store.Queries, canonical string, memberIDs []string) (MergeStats, error) {
	all := append([]string{canonical}, memberIDs...)
	rows, err := q.ListAdvisoriesByIDs(ctx, all)
	if err != nil {
		return MergeStats{}, fmt.Errorf("load advisories: %w", err)
	}
	domain := make([]Advisory, len(rows))
	oldest := time.Now()
	for i, r := range rows {
		domain[i] = rowAdvisory(r)
		if r.CreatedAt.Before(oldest) {
			oldest = r.CreatedAt
		}
	}
	merged := mergeAdvisories(canonical, domain)

	cv := pgtype.Float8{}
	if merged.CVSS != nil {
		cv = pgtype.Float8{Float64: *merged.CVSS, Valid: true}
	}
	if err := q.OverwriteAdvisory(ctx, store.OverwriteAdvisoryParams{
		ID: canonical, Aliases: merged.Aliases, Summary: merged.Summary, Severity: merged.Severity, Cvss: cv,
		Exploited: merged.Exploited, ExploitedSince: merged.ExploitedSince,
		Affected: merged.Affected, Refs: merged.References, ModifiedAt: merged.ModifiedAt, CreatedAt: oldest,
	}); err != nil {
		return MergeStats{}, fmt.Errorf("write merged advisory %s: %w", canonical, err)
	}

	var others []string
	for _, id := range memberIDs {
		if id != canonical {
			others = append(others, id)
		}
	}
	var st MergeStats
	if len(others) == 0 {
		return st, nil
	}
	n, err := q.RepointComponentAdvisories(ctx, store.RepointComponentAdvisoriesParams{Canonical: canonical, Others: others})
	if err != nil {
		return st, fmt.Errorf("repoint component advisories: %w", err)
	}
	st.Repointed = int(n)
	deleted, err := q.DeleteAdvisories(ctx, others)
	if err != nil {
		return st, fmt.Errorf("delete merged advisories: %w", err)
	}
	st.RowsMerged = int(deleted)
	return st, nil
}

// ResolveCanonical is the write-time guard. Given the id and aliases a
// collector is about to write, it finds existing rows that share any
// identifier and returns the id to write to plus the other existing rows that
// must be folded into it (see Absorb). Collectors call this before every
// advisory upsert so a new duplicate is never created.
func ResolveCanonical(ctx context.Context, q *store.Queries, id string, aliases []string) (target string, others []string, err error) {
	idents := append([]string{id}, aliases...)
	existing, err := q.FindOverlappingAdvisories(ctx, idents)
	if err != nil {
		return "", nil, fmt.Errorf("look up overlapping advisories: %w", err)
	}
	rowIDs := []string{id}
	for _, r := range existing {
		rowIDs = append(rowIDs, r.ID)
		idents = append(idents, r.ID)
		idents = append(idents, r.Aliases...)
	}
	target = canonicalFor(rowIDs, idents)
	for _, r := range existing {
		if r.ID != target {
			others = append(others, r.ID)
		}
	}
	return target, others, nil
}

// Absorb folds the other rows found by ResolveCanonical into target. Call it
// after the collector's own upsert, in the same transaction.
func Absorb(ctx context.Context, q *store.Queries, target string, others []string) (MergeStats, error) {
	if len(others) == 0 {
		return MergeStats{}, nil
	}
	return mergeInto(ctx, q, target, others)
}

// withAlias returns aliases plus extra ids, without the canonical id, sorted and unique.
func withAlias(aliases []string, canonical string, extra ...string) []string {
	set := map[string]bool{}
	for _, a := range append(append([]string(nil), aliases...), extra...) {
		if a != "" && a != canonical {
			set[a] = true
		}
	}
	out := make([]string, 0, len(set))
	for a := range set {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}
