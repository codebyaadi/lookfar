package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/codebyaadi/lookfar/internal/config"
	"github.com/codebyaadi/lookfar/internal/jobs"
	"github.com/codebyaadi/lookfar/internal/postgres"
	"github.com/codebyaadi/lookfar/internal/store"
)

func newRunCmd() *cobra.Command {
	var dbURL, cfgPath string
	cmd := &cobra.Command{
		Use:   "run <job> [args]",
		Short: "Run one job now and wait for it (jobs: " + strings.Join(jobs.Names(), ", ") + ")",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			job, ok := jobs.Registry()[args[0]]
			if !ok {
				return fmt.Errorf("unknown job %q (known: %s)", args[0], strings.Join(jobs.Names(), ", "))
			}
			jobArgs, err := job.Args(args[1:])
			if err != nil {
				return err
			}
			if dbURL == "" {
				return errors.New("database url is not set: pass --database-url or set DATABASE_URL")
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			pool, err := postgres.Connect(ctx, dbURL)
			if err != nil {
				return err
			}
			defer pool.Close()
			if _, err := postgres.MigrateUp(ctx, pool); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}

			opts := jobs.Options{Serve: false, Version: version}
			if job.Name == "stack_sync" {
				cfg, err := loadConfig(cfgPath)
				if err != nil {
					return err
				}
				opts.Stack = jobs.StackFromConfig(cfg)
			}
			client, err := jobs.NewClient(pool, slog.Default(), opts)
			if err != nil {
				return err
			}
			if err := client.Start(ctx); err != nil {
				return fmt.Errorf("start river: %w", err)
			}
			defer func() { _ = client.Stop(cmd.Context()) }()

			runErr := jobs.RunNow(ctx, client, jobArgs)

			out := cmd.OutOrStdout()
			if runs, err := store.New(pool).ListRuns(cmd.Context(), store.ListRunsParams{Job: job.Name, MaxRows: 1}); err == nil && len(runs) == 1 {
				fmt.Fprintf(out, "run %d: %s, stats %s\n", runs[0].ID, runs[0].Status, runs[0].Stats)
			}
			if runErr != nil {
				return fmt.Errorf("job %s failed: %w", job.Name, runErr)
			}
			fmt.Fprintf(out, "job %s ok\n", job.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbURL, "database-url", os.Getenv("DATABASE_URL"), "Postgres URL (default: $DATABASE_URL)")
	cmd.Flags().StringVar(&cfgPath, "config", "", "path to lookfar.yaml for jobs that need it (default: $"+config.EnvVar+" or ./"+config.DefaultPath+")")
	return cmd
}
