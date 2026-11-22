package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	sqlfiles "github.com/codebyaadi/lookfar/sql"
)

// MigrationStatus describes one migration and whether it has been applied.
type MigrationStatus struct {
	Version   int64
	Name      string
	Applied   bool
	AppliedAt time.Time
}

func provider(pool *pgxpool.Pool) (*goose.Provider, error) {
	sub, err := fs.Sub(sqlfiles.Schema, "schema")
	if err != nil {
		return nil, err
	}
	p, err := goose.NewProvider(goose.DialectPostgres, stdlib.OpenDBFromPool(pool), sub)
	if err != nil {
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	return p, nil
}

// MigrateUp applies all pending migrations and returns the versions applied.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool) ([]int64, error) {
	p, err := provider(pool)
	if err != nil {
		return nil, err
	}
	res, err := p.Up(ctx)
	return versions(res), err
}

// MigrateDown rolls back the most recent migration. It returns nil versions if none is applied.
func MigrateDown(ctx context.Context, pool *pgxpool.Pool) ([]int64, error) {
	p, err := provider(pool)
	if err != nil {
		return nil, err
	}
	res, err := p.Down(ctx)
	if err != nil && !errors.Is(err, goose.ErrNoNextVersion) {
		return nil, err
	}
	return versions([]*goose.MigrationResult{res}), nil
}

// MigrateDownAll rolls back every applied migration.
func MigrateDownAll(ctx context.Context, pool *pgxpool.Pool) ([]int64, error) {
	p, err := provider(pool)
	if err != nil {
		return nil, err
	}
	res, err := p.DownTo(ctx, 0)
	return versions(res), err
}

// MigrationsStatus lists all known migrations with their applied state.
func MigrationsStatus(ctx context.Context, pool *pgxpool.Pool) ([]MigrationStatus, error) {
	p, err := provider(pool)
	if err != nil {
		return nil, err
	}
	st, err := p.Status(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]MigrationStatus, len(st))
	for i, s := range st {
		out[i] = MigrationStatus{
			Version:   s.Source.Version,
			Name:      s.Source.Path,
			Applied:   s.State == goose.StateApplied,
			AppliedAt: s.AppliedAt,
		}
	}
	return out, nil
}

func versions(res []*goose.MigrationResult) []int64 {
	var out []int64
	for _, r := range res {
		if r != nil && r.Source != nil {
			out = append(out, r.Source.Version)
		}
	}
	return out
}
