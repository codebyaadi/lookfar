package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	sqlfiles "github.com/codebyaadi/lookfar/sql"
)

// Migrations come from two places: lookfar's own goose files (sql/schema) and
// River's migrator, which owns the river_* tables. River's schema is installed
// through rivermigrate rather than vendored SQL so it always matches the
// River version in go.mod. Up runs goose then River; "down --all" runs River
// then goose. A single-step Down only touches lookfar's own migrations.

// MigrationStatus describes one migration and whether it has been applied.
type MigrationStatus struct {
	Name      string // "00001_ingest.sql" or "river/006"
	Applied   bool
	AppliedAt time.Time
}

func gooseProvider(pool *pgxpool.Pool) (*goose.Provider, error) {
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

func riverMigrator(pool *pgxpool.Pool) (*rivermigrate.Migrator[pgx.Tx], error) {
	m, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return nil, fmt.Errorf("create river migrator: %w", err)
	}
	return m, nil
}

// MigrateUp applies all pending migrations and returns labels of what was applied.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	p, err := gooseProvider(pool)
	if err != nil {
		return nil, err
	}
	res, err := p.Up(ctx)
	out := gooseLabels(res)
	if err != nil {
		return out, err
	}
	rm, err := riverMigrator(pool)
	if err != nil {
		return out, err
	}
	rres, err := rm.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return out, fmt.Errorf("river migrate up: %w", err)
	}
	return append(out, riverLabels(rres)...), nil
}

// MigrateDown rolls back the most recent lookfar migration (River is left alone).
func MigrateDown(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	p, err := gooseProvider(pool)
	if err != nil {
		return nil, err
	}
	res, err := p.Down(ctx)
	if err != nil && !errors.Is(err, goose.ErrNoNextVersion) {
		return nil, err
	}
	return gooseLabels([]*goose.MigrationResult{res}), nil
}

// MigrateDownAll rolls back River's schema and then every lookfar migration.
func MigrateDownAll(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rm, err := riverMigrator(pool)
	if err != nil {
		return nil, err
	}
	rres, err := rm.Migrate(ctx, rivermigrate.DirectionDown, &rivermigrate.MigrateOpts{TargetVersion: -1})
	if err != nil {
		return nil, fmt.Errorf("river migrate down: %w", err)
	}
	out := riverLabels(rres)

	p, err := gooseProvider(pool)
	if err != nil {
		return out, err
	}
	res, err := p.DownTo(ctx, 0)
	return append(out, gooseLabels(res)...), err
}

// MigrationsStatus lists all known migrations with their applied state.
func MigrationsStatus(ctx context.Context, pool *pgxpool.Pool) ([]MigrationStatus, error) {
	p, err := gooseProvider(pool)
	if err != nil {
		return nil, err
	}
	st, err := p.Status(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]MigrationStatus, 0, len(st))
	for _, s := range st {
		out = append(out, MigrationStatus{
			Name:      s.Source.Path,
			Applied:   s.State == goose.StateApplied,
			AppliedAt: s.AppliedAt,
		})
	}

	rm, err := riverMigrator(pool)
	if err != nil {
		return nil, err
	}
	applied := map[int]bool{}
	existing, err := rm.ExistingVersions(ctx)
	if err != nil {
		return nil, fmt.Errorf("river migration status: %w", err)
	}
	for _, m := range existing {
		applied[m.Version] = true
	}
	for _, m := range rm.AllVersions() {
		ok := applied[m.Version]
		out = append(out, MigrationStatus{Name: fmt.Sprintf("river/%03d", m.Version), Applied: ok})
	}
	return out, nil
}

func gooseLabels(res []*goose.MigrationResult) []string {
	var out []string
	for _, r := range res {
		if r != nil && r.Source != nil {
			out = append(out, fmt.Sprintf("%05d", r.Source.Version))
		}
	}
	return out
}

func riverLabels(res *rivermigrate.MigrateResult) []string {
	if res == nil {
		return nil
	}
	var out []string
	for _, v := range res.Versions {
		out = append(out, fmt.Sprintf("river/%03d", v.Version))
	}
	return out
}
