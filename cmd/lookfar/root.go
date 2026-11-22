package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/codebyaadi/lookfar/internal/logging"
)

// version is injected at build time: -ldflags "-X main.version=v0.1.0".
var version = "dev"

func newRootCmd() *cobra.Command {
	var logLevel string
	root := &cobra.Command{
		Use:           "lookfar",
		Short:         "Self-hosted security watcher for your stack and your domains",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(*cobra.Command, []string) {
			slog.SetDefault(logging.New(os.Stderr, logLevel))
		},
	}
	root.PersistentFlags().StringVar(&logLevel, "log-level", envOr("LOG_LEVEL", "info"), "log level: debug, info, warn, error")

	root.AddCommand(
		newVersionCmd(),
		newServeCmd(),
		newMigrateCmd(),
		newConfigCmd(),
		newRunCmd(),
		placeholder("mcp", "Run the MCP server over stdio"),
		placeholder("token", "Manage API tokens (create, list, revoke)"),
	)
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the lookfar version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version)
		},
	}
}

func placeholder(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "not implemented yet")
		},
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
