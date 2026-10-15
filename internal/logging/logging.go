// Package logging configures the process-wide slog logger.
package logging

import (
	"log/slog"
	"os"
)

// Setup installs a JSON logger in production and a text logger otherwise.
func Setup(production bool, level slog.Level, service string) {
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if production {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(h).With("service", service))
}
