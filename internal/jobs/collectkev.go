package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/codebyaadi/lookfar/internal/advisory"
)

func newKEVCollector(pool *pgxpool.Pool, opts Options) *advisory.KEV {
	version := opts.Version
	if version == "" {
		version = "dev"
	}
	return &advisory.KEV{Pool: pool, Fetcher: advisory.NewFetcher(version), URL: opts.KEVURL}
}

// CollectKEVArgs is the collect_kev job: fetch the CISA KEV catalog.
type CollectKEVArgs struct{}

// Kind implements river.JobArgs.
func (CollectKEVArgs) Kind() string { return "collect_kev" }

// CollectKEVWorker runs the KEV collector and records its counters.
type CollectKEVWorker struct {
	river.WorkerDefaults[CollectKEVArgs]
	Logger    *slog.Logger
	Collector advisory.Collector
	// ChainDedupe queues dedupe_advisories after a successful run (serve only).
	ChainDedupe bool
}

// Work implements river.Worker.
func (w *CollectKEVWorker) Work(ctx context.Context, _ *river.Job[CollectKEVArgs]) error {
	res, err := w.Collector.Collect(ctx, time.Time{})
	RecordStats(ctx, res.Stats())
	if err != nil {
		return err
	}
	w.Logger.Info("collected KEV", "fetched", res.Fetched, "new", res.New, "updated", res.Updated,
		"unchanged", res.Unchanged, "not_modified", res.NotModified)
	if w.ChainDedupe {
		enqueueDedupe(ctx, w.Logger)
	}
	return nil
}

var collectKEVJob = Job{
	Name: "collect_kev",
	Args: func(cliArgs []string) (river.JobArgs, error) {
		if len(cliArgs) > 0 {
			return nil, errors.New("collect_kev takes no arguments")
		}
		return CollectKEVArgs{}, nil
	},
}
