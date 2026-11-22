package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/codebyaadi/lookfar/internal/config"
)

func newConfigCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Work with lookfar.yaml",
	}
	cmd.PersistentFlags().StringVar(&path, "config", "", "path to lookfar.yaml (default: $"+config.EnvVar+" or ./"+config.DefaultPath+")")

	cmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate lookfar.yaml and print a summary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved := config.ResolvePath(path, os.Getenv)
			cfg, err := config.Load(resolved, os.LookupEnv)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "config ok (%s)\n%s\n", resolved, cfg.Summary())
			return nil
		},
	})
	return cmd
}
