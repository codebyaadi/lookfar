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

// Scheduler queues work that has come due and returns how many items it saw.
type Scheduler interface {
	ScheduleDue(ctx context.Context, batch int) (int, error)
}

const scheduleBatch = 100

type Options struct {
	DB   Pinger
	Tick time.Duration
	// Jobs runs queued jobs; nil until a phase registers handlers.
	Jobs           *jobs.Runner
	JobConcurrency int
	// Scheduler is optional; when set, each tick queues due source checks.
	Scheduler Scheduler
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
		return
	}
	w.schedule(ctx)
}

// schedule keeps queuing while there is a backlog of due sources.
func (w *Worker) schedule(ctx context.Context) {
	if w.Scheduler == nil {
		return
	}
	for ctx.Err() == nil {
		n, err := w.Scheduler.ScheduleDue(ctx, scheduleBatch)
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("schedule due sources", "err", err)
			}
			return
		}
		if n > 0 {
			slog.Info("queued source checks", "count", n)
		}
		if n < scheduleBatch {
			return
		}
	}
}
