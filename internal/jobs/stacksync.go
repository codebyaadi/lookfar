package jobs

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/codebyaadi/lookfar/internal/config"
	"github.com/codebyaadi/lookfar/internal/stack"
)

// StackSpec is what stack_sync needs from the config: manifest paths
// (already resolved against the config directory) and manual components.
type StackSpec struct {
	Manifests []string
	Manual    []stack.ManualSpec
}

// StackSource returns the current stack spec. ok is false when there is no
// config, in which case stack_sync does nothing rather than wiping the table.
type StackSource func() (spec StackSpec, ok bool)

// StackFromConfig builds a StackSource from a loaded config. A nil config
// (no config file) yields ok=false.
func StackFromConfig(cfg *config.Config) StackSource {
	return func() (StackSpec, bool) {
		if cfg == nil {
			return StackSpec{}, false
		}
		var spec StackSpec
		for _, m := range cfg.Stack.Manifests {
			p := m.Path
			if !filepath.IsAbs(p) && cfg.Dir != "" {
				p = filepath.Join(cfg.Dir, p)
			}
			spec.Manifests = append(spec.Manifests, p)
		}
		for _, c := range cfg.Stack.Components {
			spec.Manual = append(spec.Manual, stack.ManualSpec{
				Name: c.Name, Version: c.Version, Kind: c.Kind, Ecosystem: c.Ecosystem,
			})
		}
		return spec, true
	}
}

// StackSyncArgs is the stack_sync job: resolve the config stack into the components table.
type StackSyncArgs struct{}

// Kind implements river.JobArgs.
func (StackSyncArgs) Kind() string { return "stack_sync" }

// StackSyncWorker syncs components. Per-file extraction errors are recorded
// in the run stats; only a database error fails the job.
type StackSyncWorker struct {
	river.WorkerDefaults[StackSyncArgs]
	Logger *slog.Logger
	Pool   *pgxpool.Pool
	Source StackSource
}

// Work implements river.Worker.
func (w *StackSyncWorker) Work(ctx context.Context, _ *river.Job[StackSyncArgs]) error {
	if w.Source == nil {
		return errors.New("stack_sync: no stack source configured")
	}
	spec, ok := w.Source()
	if !ok {
		w.Logger.Warn("stack_sync skipped: no config file")
		RecordStats(ctx, map[string]any{"skipped": "no config file"})
		return nil
	}
	res := stack.Resolve(ctx, spec.Manifests, spec.Manual)
	sum, err := stack.Sync(ctx, w.Pool, res)
	if sum.FileErrors == nil {
		sum.FileErrors = []string{}
	}
	RecordStats(ctx, map[string]any{
		"components":  len(res.Components),
		"added":       sum.Added,
		"updated":     sum.Updated,
		"unchanged":   sum.Unchanged,
		"deactivated": sum.Deactivated,
		"file_errors": sum.FileErrors,
	})
	if err != nil {
		return err
	}
	w.Logger.Info("stack synced", "components", len(res.Components), "added", sum.Added,
		"updated", sum.Updated, "deactivated", sum.Deactivated, "file_errors", len(sum.FileErrors))
	return nil
}

var stackSyncJob = Job{
	Name: "stack_sync",
	Args: func(cliArgs []string) (river.JobArgs, error) {
		if len(cliArgs) > 0 {
			return nil, errors.New("stack_sync takes no arguments")
		}
		return StackSyncArgs{}, nil
	},
}
