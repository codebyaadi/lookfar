package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/codebyaadi/lookfar/internal/advisory"
)

// DedupeArgs is the dedupe_advisories job: merge advisory rows that describe
// the same vulnerability. DryRun only counts the groups.
type DedupeArgs struct {
	DryRun bool `json:"dry_run,omitempty"`
}

// Kind implements river.JobArgs.
func (DedupeArgs) Kind() string { return "dedupe_advisories" }

// DedupeWorker runs the merge and records its counters.
type DedupeWorker struct {
	river.WorkerDefaults[DedupeArgs]
	Logger *slog.Logger
	Pool   *pgxpool.Pool
}

// Work implements river.Worker.
func (w *DedupeWorker) Work(ctx context.Context, job *river.Job[DedupeArgs]) error {
	st, err := advisory.Dedupe(ctx, w.Pool, job.Args.DryRun)
	RecordStats(ctx, st.Stats())
	if err != nil {
		return err
	}
	w.Logger.Info("deduped advisories", "groups", st.Groups, "merged", st.RowsMerged,
		"repointed", st.Repointed, "dry_run", job.Args.DryRun)
	return nil
}

var dedupeJob = Job{
	Name: "dedupe_advisories",
	Args: func(cliArgs []string) (river.JobArgs, error) {
		switch {
		case len(cliArgs) == 0:
			return DedupeArgs{}, nil
		case len(cliArgs) == 1 && cliArgs[0] == "dry-run":
			return DedupeArgs{DryRun: true}, nil
		}
		return nil, errors.New("usage: run dedupe_advisories [dry-run]")
	},
}

// enqueueDedupe queues a dedupe run after a collector finished (serve only).
// It is unique among unfinished jobs, so a burst of collector runs queues one.
func enqueueDedupe(ctx context.Context, logger *slog.Logger) {
	c, err := river.ClientFromContextSafely[pgx.Tx](ctx)
	if err != nil {
		return
	}
	if _, err := c.Insert(ctx, DedupeArgs{}, chainOpts()); err != nil {
		logger.Warn("could not enqueue dedupe_advisories", "error", fmt.Sprint(err))
	}
}
