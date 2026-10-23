// Command api serves the Lookfar HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codebyaadi/lookfar/internal/auth"
	"github.com/codebyaadi/lookfar/internal/config"
	"github.com/codebyaadi/lookfar/internal/httpapi"
	"github.com/codebyaadi/lookfar/internal/logging"
	"github.com/codebyaadi/lookfar/internal/postgres"
	"github.com/codebyaadi/lookfar/internal/source"
	"github.com/codebyaadi/lookfar/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	logging.Setup(cfg.IsProduction(), cfg.LogLevel, "api")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	queries := store.New(pool)
	router := httpapi.NewRouter(httpapi.Deps{
		Auth:        auth.NewAuthenticator(cfg.AuthSecret, queries),
		Queries:     queries,
		Sources:     source.NewService(pool, source.NewChecker(source.NewFetcher(cfg.AllowPrivateSources), cfg.GitHubToken)),
		DB:          pool,
		CORSOrigins: cfg.CORSOrigins,
		AIEnabled:   cfg.AIEnabled(),
	})

	srv := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.APIAddr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		slog.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
