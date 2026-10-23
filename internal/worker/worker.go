// Package worker runs Lookfar's background loop: it drains the job queue.
// Later phases register source checks, agent runs and notifications as jobs.
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/codebyaadi/lookfar/internal/jobs"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Options struct {
	DB   Pinger
	Tick time.Duration
	// Jobs runs queued jobs; nil until a phase registers handlers.
	Jobs           *jobs.Runner
	JobConcurrency int
}

type Worker struct {
	Options
}

func New(o Options) *Worker { return &Worker{o} }

// Run blocks until ctx is cancelled, checking database health every tick.
func (w *Worker) Run(ctx context.Context) error {
	slog.Info("worker started", "tick", w.Tick, "jobs", w.Jobs != nil)
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

func (w *Worker) pass(ctx context.Context) {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := w.DB.Ping(pingCtx); err != nil && ctx.Err() == nil {
		slog.Error("database unreachable", "err", err)
	}
}
