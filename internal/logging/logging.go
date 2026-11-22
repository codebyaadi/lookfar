// Package logging configures the process-wide slog logger.
package logging

import (
	"io"
	"log/slog"
	"strings"
)

// New returns a JSON logger writing to w at the given level
// (debug, info, warn, error). Unknown levels fall back to info.
func New(w io.Writer, level string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: parseLevel(level)}))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
