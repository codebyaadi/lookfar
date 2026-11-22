package stack

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/codebyaadi/lookfar/internal/store"
)

// SyncSummary reports what Sync changed.
type SyncSummary struct {
	Added       int
	Updated     int // changed in place or reactivated
	Unchanged   int
	Deactivated int
	// FileErrors lists manifests that could not be read. Their existing
	// components are left untouched rather than deactivated.
	FileErrors []string
}

type rowKey struct{ purl, origin, manifest string }

// Sync makes the components table match the resolved stack, in one
// transaction: upsert on (purl, origin, manifest_path), deactivate rows that
// are no longer present, reactivate rows that came back.
func Sync(ctx context.Context, pool *pgxpool.Pool, res Resolved) (SyncSummary, error) {
	var sum SyncSummary
	failed := map[string]bool{}
	for _, f := range res.Files {
		if f.Err != nil {
			failed[f.Path] = true
			sum.FileErrors = append(sum.FileErrors, f.Err.Error())
		}
	}
	for _, e := range res.ManualErrors {
		sum.FileErrors = append(sum.FileErrors, e.Error())
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return sum, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := store.New(tx)

	existing, err := q.ListComponents(ctx)
	if err != nil {
		return sum, fmt.Errorf("list components: %w", err)
	}
	byKey := make(map[rowKey]store.Component, len(existing))
	for _, e := range existing {
		byKey[rowKey{e.Purl, e.Origin, e.ManifestPath}] = e
	}

	present := map[rowKey]bool{}
	for _, c := range res.Components {
		k := rowKey{c.PURL, c.Origin, c.Manifest}
		if c.Origin == OriginManual {
			k.manifest = ""
		}
		if present[k] {
			continue
		}
		present[k] = true

		row, found := byKey[k]
		switch {
		case !found:
			_, err = q.InsertComponent(ctx, store.InsertComponentParams{
				Ecosystem: c.Ecosystem, Name: c.Name, Version: c.Version, Purl: c.PURL,
				Origin: c.Origin, ManifestPath: k.manifest, Kind: kindOf(c), Cpe: c.CPE,
			})
			sum.Added++
		case !row.Active || row.Ecosystem != c.Ecosystem || row.Name != c.Name ||
			row.Version != c.Version || row.Kind != kindOf(c) || row.Cpe != c.CPE:
			err = q.UpdateComponent(ctx, store.UpdateComponentParams{
				ID: row.ID, Ecosystem: c.Ecosystem, Name: c.Name, Version: c.Version, Kind: kindOf(c), Cpe: c.CPE,
			})
			sum.Updated++
		default:
			sum.Unchanged++
		}
		if err != nil {
			return sum, fmt.Errorf("upsert %s: %w", c.PURL, err)
		}
	}

	var stale []int64
	for k, row := range byKey {
		if !row.Active || present[k] {
			continue
		}
		if k.origin == OriginManifest && failed[k.manifest] {
			continue // could not read the file this time: keep what we knew
		}
		stale = append(stale, row.ID)
	}
	if len(stale) > 0 {
		n, err := q.DeactivateComponents(ctx, stale)
		if err != nil {
			return sum, fmt.Errorf("deactivate: %w", err)
		}
		sum.Deactivated = int(n)
	}

	if err := tx.Commit(ctx); err != nil {
		return sum, fmt.Errorf("commit: %w", err)
	}
	return sum, nil
}

func kindOf(c Component) string {
	if c.Kind == "" {
		return KindPackage
	}
	return c.Kind
}
