// Package server runs the long-lived lookfar process: migrations, River and HTTP.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/codebyaadi/lookfar/internal/httpapi"
	"github.com/codebyaadi/lookfar/internal/jobs"
	"github.com/codebyaadi/lookfar/internal/postgres"
)

// Options configure Run.
type Options struct {
	Addr        string
	DatabaseURL string
	Version     string
	Logger      *slog.Logger
	// Stack feeds the stack_sync job; nil means no config file.
	Stack jobs.StackSource
	// Ready, if set, receives the bound address once the server is listening.
	Ready func(addr string)
}

// Run starts everything and blocks until ctx is cancelled, then shuts down
// gracefully: stop accepting HTTP requests, let running jobs finish, close the pool.
func Run(ctx context.Context, o Options) error {
	pool, err := postgres.Connect(ctx, o.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	applied, err := postgres.MigrateUp(ctx, pool)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	o.Logger.Info("migrations up to date", "applied", len(applied))

	client, err := jobs.NewClient(pool, o.Logger, jobs.Options{Serve: true, Stack: o.Stack})
	if err != nil {
		return err
	}
	// River is started with a background context so that cancelling ctx lets
	// Stop drain running jobs instead of aborting them.
	if err := client.Start(context.WithoutCancel(ctx)); err != nil {
		return fmt.Errorf("start river: %w", err)
	}

	handler := httpapi.New(httpapi.Deps{
		Version: o.Version,
		Ping:    pool.Ping,
		RiverRunning: func() bool {
			select {
			case <-client.Stopped():
				return false
			default:
				return true
			}
		},
	})
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", o.Addr)
	if err != nil {
		stopClient(client, o.Logger)
		return fmt.Errorf("listen on %s: %w", o.Addr, err)
	}
	o.Logger.Info("listening", "addr", ln.Addr().String())
	if o.Ready != nil {
		o.Ready(ln.Addr().String())
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	var runErr error
	select {
	case <-ctx.Done():
		o.Logger.Info("shutting down")
	case runErr = <-serveErr:
	}

	shutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil && runErr == nil {
		runErr = err
	}
	stopClient(client, o.Logger)
	if errors.Is(runErr, http.ErrServerClosed) {
		runErr = nil
	}
	return runErr
}

func stopClient(c *jobs.Client, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Stop(ctx); err != nil {
		logger.Error("stop river", "error", err)
	}
}
