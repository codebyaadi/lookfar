// Command worker runs Feedoku's background jobs (feed polling, AI enrichment).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codebyaadi/feedoku/internal/config"
	"github.com/codebyaadi/feedoku/internal/enrich"
	"github.com/codebyaadi/feedoku/internal/feed"
	"github.com/codebyaadi/feedoku/internal/ingest"
	"github.com/codebyaadi/feedoku/internal/jobs"
	"github.com/codebyaadi/feedoku/internal/llm"
	"github.com/codebyaadi/feedoku/internal/logging"
	"github.com/codebyaadi/feedoku/internal/postgres"
	"github.com/codebyaadi/feedoku/internal/store"
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

	q := store.New(pool)
	opts := worker.Options{
		DB:     pool,
		Poller: ingest.New(q, feed.NewFetcher(cfg.AllowPrivateFeeds)),
		Tick:   cfg.WorkerTick,
	}

	chain := llm.NewChain(
		&llm.Provider{Name: "groq", BaseURL: cfg.GroqBaseURL, APIKey: cfg.GroqAPIKey, Model: cfg.GroqModel, RPM: cfg.LLMRPM,
			Extra: map[string]any{"reasoning_effort": "low"}},
		&llm.Provider{Name: "openrouter", BaseURL: cfg.OpenRouterBaseURL, APIKey: cfg.OpenRouterAPIKey, Model: cfg.OpenRouterModel, RPM: min(cfg.LLMRPM, 18)},
	)
	if chain.Enabled() {
		runner := jobs.NewRunner(q, 2*time.Second)
		runner.Register(enrich.KindEnrichArticle, enrich.New(q, chain).Handle)
		opts.Jobs, opts.JobConcurrency = runner, cfg.EnrichConcurrency
	} else {
		slog.Warn("no LLM provider configured (GROQ_API_KEY / OPENROUTER_API_KEY): summaries disabled")
	}
	if embedder := enrich.NewEmbedder(q, llm.NewVoyage(cfg.VoyageBaseURL, cfg.VoyageAPIKey, cfg.VoyageModel, 1024), cfg.EmbedBatch); embedder.Enabled() {
		opts.Embedder, opts.EmbedBatch = embedder, cfg.EmbedBatch
	} else {
		slog.Warn("VOYAGE_API_KEY not set: embeddings disabled")
	}

	return worker.New(opts).Run(ctx)
}
