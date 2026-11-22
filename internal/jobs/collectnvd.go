package jobs

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/codebyaadi/lookfar/internal/advisory"
)

func newNVDCollector(pool *pgxpool.Pool, opts Options) *advisory.NVD {
	version := opts.Version
	if version == "" {
		version = "dev"
	}
	f := advisory.NewFetcher(version)
	f.HTTP.Timeout = 120 * time.Second // CPE queries can return several MB
	f.RetryOn = []int{403}             // NVD signals rate limiting with 403
	f.Backoff = 6 * time.Second
	f.MaxRetries = 4
	key := opts.NVDAPIKey
	if key == "" {
		key = os.Getenv("NVD_API_KEY")
	}
	return &advisory.NVD{Pool: pool, Fetcher: f, BaseURL: opts.NVDURL, APIKey: key, Limiter: opts.NVDLimiter}
}

// CollectNVDArgs is the collect_nvd job: CPE matches for server components plus severity enrichment.
type CollectNVDArgs struct{}

// Kind implements river.JobArgs.
func (CollectNVDArgs) Kind() string { return "collect_nvd" }

// NVDCollector is what the worker needs; advisory.NVD implements it.
type NVDCollector interface {
	CollectStats(ctx context.Context, since time.Time) (advisory.NVDStats, error)
}

// CollectNVDWorker runs the NVD collector and records its counters.
type CollectNVDWorker struct {
	river.WorkerDefaults[CollectNVDArgs]
	Logger    *slog.Logger
	Collector NVDCollector
}

// Work implements river.Worker. A long run is expected without an API key
// (about six seconds per request), so the job gets a generous timeout.
func (w *CollectNVDWorker) Work(ctx context.Context, _ *river.Job[CollectNVDArgs]) error {
	st, err := w.Collector.CollectStats(ctx, time.Time{})
	RecordStats(ctx, st.Stats())
	if err != nil {
		return err
	}
	w.Logger.Info("collected NVD", "servers", st.ServersQueried, "cves", st.CVEsScanned, "matches", st.Matches,
		"enriched", st.Enriched, "skipped_rate_limited", st.SkippedRateLimited)
	return nil
}

// Timeout implements river.Worker.
func (w *CollectNVDWorker) Timeout(*river.Job[CollectNVDArgs]) time.Duration { return 30 * time.Minute }

var collectNVDJob = Job{
	Name: "collect_nvd",
	Args: func(cliArgs []string) (river.JobArgs, error) {
		if len(cliArgs) > 0 {
			return nil, errors.New("collect_nvd takes no arguments")
		}
		return CollectNVDArgs{}, nil
	},
}
