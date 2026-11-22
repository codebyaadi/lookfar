package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/codebyaadi/lookfar/internal/postgres"
)

func newMigrateCmd() *cobra.Command {
	var dbURL string
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations (up, down, status)",
	}
	cmd.PersistentFlags().StringVar(&dbURL, "database-url", os.Getenv("DATABASE_URL"), "Postgres URL (default: $DATABASE_URL)")

	// connect runs fn with a pool; it keeps the subcommand bodies short.
	connect := func(cmd *cobra.Command, fn func(context.Context, *pgxpool.Pool) error) error {
		if dbURL == "" {
			return errors.New("database url is not set: pass --database-url or set DATABASE_URL")
		}
		pool, err := postgres.Connect(cmd.Context(), dbURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		return fn(cmd.Context(), pool)
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "up",
			Short: "Apply all pending migrations",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return connect(cmd, func(ctx context.Context, pool *pgxpool.Pool) error {
					vs, err := postgres.MigrateUp(ctx, pool)
					if err != nil {
						return err
					}
					if len(vs) == 0 {
						fmt.Fprintln(cmd.OutOrStdout(), "no pending migrations")
					}
					for _, v := range vs {
						fmt.Fprintf(cmd.OutOrStdout(), "applied %s\n", v)
					}
					return nil
				})
			},
		},
		newMigrateDownCmd(connect),
		&cobra.Command{
			Use:   "status",
			Short: "Show which migrations are applied",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return connect(cmd, func(ctx context.Context, pool *pgxpool.Pool) error {
					st, err := postgres.MigrationsStatus(ctx, pool)
					if err != nil {
						return err
					}
					for _, s := range st {
						state := "pending"
						if s.Applied {
							state = "applied"
							if !s.AppliedAt.IsZero() {
								state += " " + s.AppliedAt.Format("2006-01-02 15:04:05Z07:00")
							}
						}
						fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", s.Name, state)
					}
					return nil
				})
			},
		},
	)
	return cmd
}

func newMigrateDownCmd(connect func(*cobra.Command, func(context.Context, *pgxpool.Pool) error) error) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Roll back the latest migration (--all for every migration)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return connect(cmd, func(ctx context.Context, pool *pgxpool.Pool) error {
				down := postgres.MigrateDown
				if all {
					down = postgres.MigrateDownAll
				}
				vs, err := down(ctx, pool)
				if err != nil {
					return err
				}
				if len(vs) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "nothing to roll back")
				}
				for _, v := range vs {
					fmt.Fprintf(cmd.OutOrStdout(), "rolled back %s\n", v)
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "roll back every applied migration")
	return cmd
}
