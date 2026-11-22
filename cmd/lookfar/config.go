package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/codebyaadi/lookfar/internal/config"
	"github.com/codebyaadi/lookfar/internal/stack"
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
		Short: "Validate lookfar.yaml and print the summary and resolved stack",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved := config.ResolvePath(path, os.Getenv)
			cfg, err := config.Load(resolved, os.LookupEnv)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "config ok (%s)\n%s\n", resolved, cfg.Summary())
			printStack(cmd.Context(), out, cfg)
			return nil
		},
	})
	return cmd
}

// printStack resolves the declared stack (no database needed) and prints it.
// Manifest problems are reported but do not fail validation: the file may
// simply not exist on the machine that validates the config.
func printStack(ctx context.Context, out io.Writer, cfg *config.Config) {
	manual := make([]stack.ManualSpec, len(cfg.Stack.Components))
	for i, c := range cfg.Stack.Components {
		manual[i] = stack.ManualSpec{Name: c.Name, Version: c.Version, Kind: c.Kind, Ecosystem: c.Ecosystem}
	}
	manifests := make([]string, len(cfg.Stack.Manifests))
	for i, m := range cfg.Stack.Manifests {
		manifests[i] = m.Path
		if !filepath.IsAbs(m.Path) {
			manifests[i] = filepath.Join(cfg.Dir, m.Path)
		}
	}
	res := stack.Resolve(ctx, manifests, manual)

	fmt.Fprintln(out, "\nstack:")
	failed := 0
	for i, f := range res.Files {
		if f.Err != nil {
			failed++
			fmt.Fprintf(out, "  manifest %s: ERROR %v\n", cfg.Stack.Manifests[i].Path, f.Err)
			continue
		}
		fmt.Fprintf(out, "  manifest %s: %d components\n", cfg.Stack.Manifests[i].Path, f.Count)
	}
	for _, c := range res.Components {
		if c.Origin == stack.OriginManual {
			fmt.Fprintf(out, "  manual %s %s (%s) %s\n", c.Name, c.Version, c.Kind, c.PURL)
		}
	}
	for _, e := range res.ManualErrors {
		fmt.Fprintf(out, "  manual: ERROR %v\n", e)
	}
	if failed > 0 {
		fmt.Fprintf(out, "warning: %d manifest(s) could not be read\n", failed)
	}
}
