package advisory

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/codebyaadi/lookfar/internal/store"
)

// DedupeStats are the counters of one dedupe run.
type DedupeStats struct {
	Groups     int // groups of rows that describe the same vulnerability
	RowsMerged int // rows folded into a canonical row and deleted
	Repointed  int // component_advisories rows written under a canonical id
}

// Stats returns run-log stats.
func (s DedupeStats) Stats() map[string]any {
	return map[string]any{"groups": s.Groups, "rows_merged": s.RowsMerged, "pairs_repointed": s.Repointed}
}

// DuplicateGroups finds groups of advisory rows that share an identifier
// (one row's id is another's id or alias, or their aliases intersect),
// following the overlap transitively. Each group has at least two ids.
//
// The equivalent audit in plain SQL is the ListDuplicatePairs query in
// sql/queries/dedupe.sql.
func DuplicateGroups(ctx context.Context, q *store.Queries) ([][]string, error) {
	pairs, err := q.ListDuplicatePairs(ctx)
	if err != nil {
		return nil, fmt.Errorf("find duplicate pairs: %w", err)
	}
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		if p, ok := parent[x]; ok && p != x {
			parent[x] = find(p)
			return parent[x]
		}
		parent[x] = x
		return x
	}
	for _, p := range pairs {
		ra, rb := find(p.AID), find(p.BID)
		if ra != rb {
			parent[ra] = rb
		}
	}
	byRoot := map[string][]string{}
	for id := range parent {
		r := find(id)
		byRoot[r] = append(byRoot[r], id)
	}
	groups := make([][]string, 0, len(byRoot))
	for _, g := range byRoot {
		if len(g) > 1 {
			sort.Strings(g)
			groups = append(groups, g)
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i][0] < groups[j][0] })
	return groups, nil
}

// Dedupe merges every duplicate group into one row, each group in its own
// transaction. With dryRun it only counts the groups.
func Dedupe(ctx context.Context, pool *pgxpool.Pool, dryRun bool) (DedupeStats, error) {
	var st DedupeStats
	groups, err := DuplicateGroups(ctx, store.New(pool))
	if err != nil {
		return st, err
	}
	st.Groups = len(groups)
	if dryRun {
		return st, nil
	}
	for _, g := range groups {
		ms, err := mergeGroup(ctx, pool, g)
		if err != nil {
			return st, err
		}
		st.RowsMerged += ms.RowsMerged
		st.Repointed += ms.Repointed
	}
	return st, nil
}

func mergeGroup(ctx context.Context, pool *pgxpool.Pool, ids []string) (MergeStats, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return MergeStats{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := store.New(tx)

	rows, err := q.ListAdvisoriesByIDs(ctx, ids)
	if err != nil {
		return MergeStats{}, err
	}
	var rowIDs, idents []string
	for _, r := range rows {
		rowIDs = append(rowIDs, r.ID)
		idents = append(idents, r.ID)
		idents = append(idents, r.Aliases...)
	}
	if len(rowIDs) < 2 {
		return MergeStats{}, nil // already merged by a concurrent writer
	}
	st, err := mergeInto(ctx, q, canonicalFor(rowIDs, idents), rowIDs)
	if err != nil {
		return st, err
	}
	return st, tx.Commit(ctx)
}
