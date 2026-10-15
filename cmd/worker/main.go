// Command worker runs Feedoku's background jobs (feed polling, AI enrichment).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/codebyaadi/feedoku/internal/config"
	"github.com/codebyaadi/feedoku/internal/logging"
	"github.com/codebyaadi/feedoku/internal/postgres"
	"github.com/codebyaadi/feedoku/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	logging.Setup(cfg.IsProduction(), cfg.LogLevel, "worker")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	return worker.New(pool, cfg.WorkerTick).Run(ctx)
}
