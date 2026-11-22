package main

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/codebyaadi/lookfar/internal/config"
	"github.com/codebyaadi/lookfar/internal/server"
)

func newServeCmd() *cobra.Command {
	var addr, dbURL, cfgPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the API, dashboard, scheduler and workers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dbURL == "" {
				return errors.New("database url is not set: pass --database-url or set DATABASE_URL")
			}
			if err := checkConfig(cfgPath); err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return server.Run(ctx, server.Options{
				Addr:        addr,
				DatabaseURL: dbURL,
				Version:     version,
				Logger:      slog.Default(),
			})
		},
	}
	cmd.Flags().StringVar(&addr, "addr", envOr("LOOKFAR_ADDR", ":8080"), "listen address (default: $LOOKFAR_ADDR or :8080)")
	cmd.Flags().StringVar(&dbURL, "database-url", os.Getenv("DATABASE_URL"), "Postgres URL (default: $DATABASE_URL)")
	cmd.Flags().StringVar(&cfgPath, "config", "", "path to lookfar.yaml (default: $"+config.EnvVar+" or ./"+config.DefaultPath+")")
	return cmd
}

// checkConfig validates the config file so a bad file stops startup. A missing
// file is only an error when the path was given explicitly; otherwise defaults apply.
func checkConfig(flagPath string) error {
	path := config.ResolvePath(flagPath, os.Getenv)
	explicit := path != config.DefaultPath
	cfg, err := config.Load(path, os.LookupEnv)
	switch {
	case errors.Is(err, fs.ErrNotExist) && !explicit:
		slog.Warn("no config file found, running with defaults", "path", path)
		return nil
	case err != nil:
		return err
	}
	slog.Info("config loaded", "path", path, "domains", len(cfg.Domains), "components", len(cfg.Stack.Components))
	return nil
}
