// Package worker runs Feedoku's background loop. Phase 1 adds feed polling;
// later phases add AI enrichment and daily briefings as jobs.
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/codebyaadi/feedoku/internal/jobs"
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

// Sweeper does one batch of background work (e.g. embedding) and returns how
// many items it handled. Rate-limit errors are logged and retried next tick.
type Sweeper interface {
	Sweep(ctx context.Context) (int, error)
}

type Options struct {
	DB     Pinger
	Poller Poller
	Tick   time.Duration
	// Jobs is optional; when set its handlers run alongside feed polling.
	Jobs           *jobs.Runner
	JobConcurrency int
	Embedder       Sweeper
	EmbedBatch     int
}

type Worker struct {
	Options
}

func New(o Options) *Worker { return &Worker{o} }

// Run blocks until ctx is cancelled, running one pass per tick.
func (w *Worker) Run(ctx context.Context) error {
	slog.Info("worker started", "tick", w.Tick, "jobs", w.Jobs != nil, "embeddings", w.Embedder != nil)
	if w.Jobs != nil {
		go w.Jobs.Run(ctx, w.JobConcurrency)
	}
	t := time.NewTicker(w.Tick)
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

func (w *Worker) sweep(ctx context.Context) {
	if w.Embedder == nil {
		return
	}
	for ctx.Err() == nil {
		n, err := w.Embedder.Sweep(ctx)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("embedding sweep", "err", err)
			}
			return
		}
		if n > 0 {
			slog.Info("embedded articles", "count", n)
		}
		if n < w.EmbedBatch {
			return
		}
	}
}

func (w *Worker) pass(ctx context.Context) {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := w.DB.Ping(pingCtx); err != nil && ctx.Err() == nil {
		slog.Error("database unreachable", "err", err)
		return
	}
	w.sweep(ctx)
	// Keep draining while there is a backlog of due feeds.
	for ctx.Err() == nil {
		n, err := w.Poller.PollDue(ctx, pollBatch, pollConcurrency)
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
