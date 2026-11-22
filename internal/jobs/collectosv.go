package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/codebyaadi/lookfar/internal/advisory"
)

func newOSVCollector(pool *pgxpool.Pool, opts Options) *advisory.OSV {
	version := opts.Version
	if version == "" {
		version = "dev"
	}
	return &advisory.OSV{Pool: pool, Fetcher: advisory.NewFetcher(version), BaseURL: opts.OSVURL}
}

// CollectOSVArgs is the collect_osv job: ask OSV about every active package component.
type CollectOSVArgs struct{}

// Kind implements river.JobArgs.
func (CollectOSVArgs) Kind() string { return "collect_osv" }

// OSVCollector is what the worker needs; advisory.OSV implements it.
type OSVCollector interface {
	CollectStats(ctx context.Context, since time.Time) (advisory.OSVStats, error)
}

// CollectOSVWorker runs the OSV collector and records its counters.
type CollectOSVWorker struct {
	river.WorkerDefaults[CollectOSVArgs]
	Logger    *slog.Logger
	Collector OSVCollector
	// ChainNVD enqueues collect_nvd after a successful run (serve only).
	ChainNVD bool
}

// Work implements river.Worker.
func (w *CollectOSVWorker) Work(ctx context.Context, _ *river.Job[CollectOSVArgs]) error {
	st, err := w.Collector.CollectStats(ctx, time.Time{})
	RecordStats(ctx, st.Stats())
	if err != nil {
		return err
	}
	w.Logger.Info("collected OSV", "queried", st.ComponentsQueried, "skipped", st.ComponentsSkipped,
		"vulns", st.VulnsFound, "details", st.DetailsFetched, "new", st.AdvisoriesNew, "updated", st.AdvisoriesUpdated)
	if w.ChainNVD {
		if c, cerr := river.ClientFromContextSafely[pgx.Tx](ctx); cerr == nil {
			if _, ierr := c.Insert(ctx, CollectNVDArgs{}, chainOpts()); ierr != nil {
				w.Logger.Warn("could not enqueue collect_nvd", "error", ierr)
			}
		}
	}
	return nil
}

var collectOSVJob = Job{
	Name: "collect_osv",
	Args: func(cliArgs []string) (river.JobArgs, error) {
		if len(cliArgs) > 0 {
			return nil, errors.New("collect_osv takes no arguments")
		}
		return CollectOSVArgs{}, nil
	},
}

// chainOpts makes a chained enqueue (stack_sync -> collect_osv -> collect_nvd) unique among jobs
// that have not finished, so a burst of syncs queues one collection.
func chainOpts() *river.InsertOpts {
	return &river.InsertOpts{UniqueOpts: river.UniqueOpts{
		ByArgs: true,
		ByState: []rivertype.JobState{
			rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
			rivertype.JobStateRetryable, rivertype.JobStateScheduled,
		},
	}}
}
