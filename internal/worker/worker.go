// Package worker runs Feedoku's background loop. Phase 1 adds feed polling;
// later phases add AI enrichment and daily briefings as jobs.
package worker

import (
	"context"
	"log/slog"
	"time"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

const (
	pollBatch       = 20
	pollConcurrency = 5
)

type Poller interface {
	PollDue(ctx context.Context, batch, concurrency int) (int, error)
}

type Worker struct {
	db     Pinger
	poller Poller
	tick   time.Duration
}

func New(db Pinger, poller Poller, tick time.Duration) *Worker {
	return &Worker{db: db, poller: poller, tick: tick}
}

// Run blocks until ctx is cancelled, running one pass per tick.
func (w *Worker) Run(ctx context.Context) error {
	slog.Info("worker started", "tick", w.tick)
	t := time.NewTicker(w.tick)
	defer t.Stop()

	for {
		w.pass(ctx)
		select {
		case <-ctx.Done():
			slog.Info("worker stopping")
			return nil
		case <-t.C:
		}
	}
}

func (w *Worker) pass(ctx context.Context) {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := w.db.Ping(pingCtx); err != nil && ctx.Err() == nil {
		slog.Error("database unreachable", "err", err)
		return
	}
	// Keep draining while there is a backlog of due feeds.
	for ctx.Err() == nil {
		n, err := w.poller.PollDue(ctx, pollBatch, pollConcurrency)
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("poll feeds", "err", err)
			}
			return
		}
		if n > 0 {
			slog.Info("polled feeds", "count", n)
		}
		if n < pollBatch {
			return
		}
	}
}
