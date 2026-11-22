package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"github.com/codebyaadi/lookfar/internal/store"
)

// QueueManual is worked only by `lookfar run`, never by `lookfar serve`, so a
// manual run is executed by the process that waits for it.
const QueueManual = "manual"

// Client is the River client type used throughout lookfar.
type Client = river.Client[pgx.Tx]

// Job describes a job that can be started by name from the CLI.
type Job struct {
	Name string
	// Args builds the job arguments from CLI arguments.
	Args func(cliArgs []string) (river.JobArgs, error)
}

// Registry lists every job that `lookfar run` knows about.
func Registry() map[string]Job {
	jobs := []Job{heartbeatJob, stackSyncJob, collectKEVJob}
	m := make(map[string]Job, len(jobs))
	for _, j := range jobs {
		m[j.Name] = j
	}
	return m
}

// Names returns the registered job names, sorted.
func Names() []string {
	var names []string
	for n := range Registry() {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// NewWorkers registers every worker. New jobs add themselves here.
func NewWorkers(logger *slog.Logger, pool *pgxpool.Pool, opts Options) *river.Workers {
	workers := river.NewWorkers()
	river.AddWorker(workers, &HeartbeatWorker{Logger: logger})
	river.AddWorker(workers, &StackSyncWorker{Logger: logger, Pool: pool, Source: opts.Stack})
	river.AddWorker(workers, &CollectKEVWorker{Logger: logger, Collector: newKEVCollector(pool, opts)})
	return workers
}

// PeriodicJobs returns the schedule for `lookfar serve`.
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(5*time.Minute),
			func() (river.JobArgs, *river.InsertOpts) { return HeartbeatArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true},
		),
		river.NewPeriodicJob(
			river.PeriodicInterval(6*time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return StackSyncArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true},
		),
		// KEV is conditional-GET cheap (a 304 when unchanged), so running at start is fine.
		river.NewPeriodicJob(
			river.PeriodicInterval(24*time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return CollectKEVArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true},
		),
	}
}

// Options configure NewClient.
type Options struct {
	// Serve selects the long-running server setup: default queue and periodic
	// jobs. Otherwise only the manual queue is worked (used by `lookfar run`).
	Serve bool
	// Stack supplies the declared stack for stack_sync. Nil means no config.
	Stack StackSource
	// Version is sent in the User-Agent of collectors.
	Version string
	// KEVURL overrides the CISA feed URL (tests only).
	KEVURL string
}

// NewClient builds a River client whose jobs are recorded in the runs table.
func NewClient(pool *pgxpool.Pool, logger *slog.Logger, opts Options) (*Client, error) {
	logf := func(msg string, args ...any) { logger.Error(msg, args...) }
	cfg := &river.Config{
		Logger:  logger,
		Workers: NewWorkers(logger, pool, opts),
	}
	cfg.Plugins = []rivertype.Plugin{RunLogMiddleware(StoreRunLog{Q: store.New(pool)}, logf)}
	if opts.Serve {
		cfg.Queues = map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 10}}
		cfg.PeriodicJobs = PeriodicJobs()
	} else {
		cfg.Queues = map[string]river.QueueConfig{QueueManual: {MaxWorkers: 2}}
	}
	c, err := river.NewClient(riverpgxv5.New(pool), cfg)
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}
	return c, nil
}

// RunNow inserts one job on the manual queue and waits for it to finish. The
// client must have been created with Options{Serve: false} and be started.
// It returns the job's final error (nil on success).
func RunNow(ctx context.Context, c *Client, args river.JobArgs) error {
	events, cancel := c.Subscribe(river.EventKindJobCompleted, river.EventKindJobFailed, river.EventKindJobCancelled)
	defer cancel()

	res, err := c.Insert(ctx, args, &river.InsertOpts{Queue: QueueManual, MaxAttempts: 1})
	if err != nil {
		return fmt.Errorf("enqueue job: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				return errors.New("river stopped before the job finished")
			}
			if ev.Job.ID != res.Job.ID {
				continue
			}
			if ev.Kind == river.EventKindJobCompleted {
				return nil
			}
			if n := len(ev.Job.Errors); n > 0 {
				return errors.New(ev.Job.Errors[n-1].Error)
			}
			return fmt.Errorf("job ended in state %s", ev.Job.State)
		}
	}
}
