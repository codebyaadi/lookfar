package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
)

// HeartbeatArgs is a trivial job that proves the scheduler and run log work.
type HeartbeatArgs struct{}

// Kind implements river.JobArgs.
func (HeartbeatArgs) Kind() string { return "heartbeat" }

// HeartbeatWorker logs a line and records a run.
type HeartbeatWorker struct {
	river.WorkerDefaults[HeartbeatArgs]
	Logger *slog.Logger
}

// Work implements river.Worker.
func (w *HeartbeatWorker) Work(ctx context.Context, _ *river.Job[HeartbeatArgs]) error {
	now := time.Now().UTC()
	w.Logger.Info("heartbeat", "at", now)
	RecordStats(ctx, map[string]any{"beat_at": now.Format(time.RFC3339)})
	return nil
}

var heartbeatJob = Job{
	Name: "heartbeat",
	Args: func(cliArgs []string) (river.JobArgs, error) {
		if len(cliArgs) > 0 {
			return nil, errors.New("heartbeat takes no arguments")
		}
		return HeartbeatArgs{}, nil
	},
}
