// Package jobs wires River: the client, the worker registry, periodic jobs
// and the run log that records every job execution in the runs table.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/codebyaadi/lookfar/internal/store"
)

// Run statuses stored in runs.status.
const (
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// RunLog persists job executions.
type RunLog interface {
	StartRun(ctx context.Context, job string) (id int64, err error)
	FinishRun(ctx context.Context, id int64, status string, stats json.RawMessage, errMsg string) error
}

// StoreRunLog is a RunLog backed by the runs table.
type StoreRunLog struct{ Q *store.Queries }

// StartRun inserts a running row.
func (l StoreRunLog) StartRun(ctx context.Context, job string) (int64, error) {
	run, err := l.Q.CreateRun(ctx, store.CreateRunParams{Job: job})
	if err != nil {
		return 0, err
	}
	return run.ID, nil
}

// FinishRun marks the run finished.
func (l StoreRunLog) FinishRun(ctx context.Context, id int64, status string, stats json.RawMessage, errMsg string) error {
	return l.Q.FinishRun(ctx, store.FinishRunParams{
		ID: id, Status: status, Stats: stats,
		Error: pgtype.Text{String: errMsg, Valid: errMsg != ""},
	})
}

type statsKey struct{}

type statsHolder struct {
	mu sync.Mutex
	m  map[string]any
}

// RecordStats lets a worker attach counters to its run row. It is a no-op
// outside a job that is wrapped by the run log.
func RecordStats(ctx context.Context, stats map[string]any) {
	h, ok := ctx.Value(statsKey{}).(*statsHolder)
	if !ok {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.m == nil {
		h.m = map[string]any{}
	}
	for k, v := range stats {
		h.m[k] = v
	}
}

// RunLogMiddleware records a runs row around every job. A failure to write the
// log never fails the job, but a panic is logged as failed and re-raised.
// logf receives slog-style key/value arguments.
func RunLogMiddleware(log RunLog, logf func(msg string, args ...any)) river.WorkerMiddlewareFunc {
	return river.WorkerMiddlewareFunc(func(ctx context.Context, job *rivertype.JobRow, doInner func(context.Context) error) error {
		id, startErr := log.StartRun(ctx, job.Kind)
		if startErr != nil {
			logf("run log: start failed", "job", job.Kind, "error", startErr)
		}
		holder := &statsHolder{}
		ctx = context.WithValue(ctx, statsKey{}, holder)

		finish := func(status, msg string) {
			if startErr != nil {
				return
			}
			holder.mu.Lock()
			stats, _ := json.Marshal(holder.m)
			empty := len(holder.m) == 0
			holder.mu.Unlock()
			if empty {
				stats = json.RawMessage("{}")
			}
			// The job context may already be cancelled (shutdown); use a detached one.
			if ferr := log.FinishRun(context.WithoutCancel(ctx), id, status, stats, msg); ferr != nil {
				logf("run log: finish failed", "job", job.Kind, "run_id", id, "error", ferr)
			}
		}

		defer func() {
			if r := recover(); r != nil {
				finish(StatusFailed, fmt.Sprintf("panic: %v", r))
				panic(r)
			}
		}()

		if err := doInner(ctx); err != nil {
			finish(StatusFailed, err.Error())
			return err
		}
		finish(StatusSucceeded, "")
		return nil
	})
}
