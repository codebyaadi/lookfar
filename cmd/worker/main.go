// Command worker runs Lookfar's background jobs.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codebyaadi/lookfar/internal/config"
	"github.com/codebyaadi/lookfar/internal/jobs"
	"github.com/codebyaadi/lookfar/internal/logging"
	"github.com/codebyaadi/lookfar/internal/postgres"
	"github.com/codebyaadi/lookfar/internal/source"
	"github.com/codebyaadi/lookfar/internal/store"
	"github.com/codebyaadi/lookfar/internal/worker"
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

	q := store.New(pool)
	sources := source.NewService(pool, source.NewChecker(source.NewFetcher(cfg.AllowPrivateSources), cfg.GitHubToken))
	runner := jobs.NewRunner(q, 2*time.Second)
	runner.Register(source.JobKind, sources.Handle)

	return worker.New(worker.Options{
		DB:             pool,
		Tick:           cfg.WorkerTick,
		Jobs:           runner,
		JobConcurrency: cfg.CheckConcurrency,
		Scheduler:      sources,
	}).Run(ctx)
}
