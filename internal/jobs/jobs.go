// Package jobs is a small Postgres-backed job queue using FOR UPDATE SKIP LOCKED.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/codebyaadi/feedoku/internal/store"
)

type Handler func(ctx context.Context, job store.Job) error

// DeferError asks the runner to retry later without consuming an attempt
// (used for provider rate limits).
type DeferError struct {
	After time.Duration
	Err   error
}

func (e *DeferError) Error() string { return fmt.Sprintf("deferred %s: %v", e.After, e.Err) }
func (e *DeferError) Unwrap() error { return e.Err }

func Defer(after time.Duration, err error) error { return &DeferError{After: after, Err: err} }

type Runner struct {
	q        *store.Queries
	handlers map[string]Handler
	idle     time.Duration
}

func NewRunner(q *store.Queries, idle time.Duration) *Runner {
	return &Runner{q: q, handlers: map[string]Handler{}, idle: idle}
}

func (r *Runner) Register(kind string, h Handler) { r.handlers[kind] = h }

func (r *Runner) kinds() []string {
	ks := make([]string, 0, len(r.handlers))
	for k := range r.handlers {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// Run starts `concurrency` workers and blocks until ctx is cancelled.
func (r *Runner) Run(ctx context.Context, concurrency int) {
	if len(r.handlers) == 0 {
		return
	}
	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.loop(ctx)
		}()
	}
	wg.Wait()
}

func (r *Runner) loop(ctx context.Context) {
	kinds := r.kinds()
	for ctx.Err() == nil {
		job, err := r.q.ClaimJob(ctx, kinds)
		if errors.Is(err, pgx.ErrNoRows) {
			sleep(ctx, r.idle)
			continue
		}
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("claim job", "err", err)
				sleep(ctx, r.idle)
			}
			continue
		}
		r.run(ctx, job)
	}
}

func (r *Runner) run(ctx context.Context, job store.Job) {
	err := r.safeHandle(ctx, job)
	// Finish with a context that survives shutdown so the row isn't left locked.
	fin, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	var de *DeferError
	switch {
	case err == nil:
		if err := r.q.CompleteJob(fin, job.ID); err != nil {
			slog.Error("complete job", "job", job.ID, "err", err)
		}
	case errors.As(err, &de):
		slog.Info("job deferred", "job", job.ID, "kind", job.Kind, "after", de.After)
		_ = r.q.DeferJob(fin, store.DeferJobParams{ID: job.ID, DelaySecs: int32(de.After.Seconds())})
	case ctx.Err() != nil:
		// Shutting down mid-job: release it without burning an attempt.
		_ = r.q.DeferJob(fin, store.DeferJobParams{ID: job.ID, DelaySecs: 0})
	default:
		msg := err.Error()
		slog.Warn("job failed", "job", job.ID, "kind", job.Kind, "attempt", job.Attempts, "err", err)
		if job.Attempts >= job.MaxAttempts {
			_ = r.q.AbandonJob(fin, store.AbandonJobParams{ID: job.ID, LastError: &msg})
			return
		}
		backoff := 30 * time.Second << min(job.Attempts-1, 6)
		_ = r.q.RetryJob(fin, store.RetryJobParams{ID: job.ID, LastError: &msg, DelaySecs: int32(backoff.Seconds())})
	}
}

func (r *Runner) safeHandle(ctx context.Context, job store.Job) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	h, ok := r.handlers[job.Kind]
	if !ok {
		return fmt.Errorf("no handler for %q", job.Kind)
	}
	return h(ctx, job)
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
