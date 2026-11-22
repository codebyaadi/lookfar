package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug": slog.LevelDebug, "WARN": slog.LevelWarn, "error": slog.LevelError,
		"info": slog.LevelInfo, "": slog.LevelInfo, "nonsense": slog.LevelInfo,
	}
	for in, want := range tests {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestNewFiltersByLevel(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, "warn")
	l.Info("hidden")
	l.Warn("shown")
	out := buf.String()
	if strings.Contains(out, "hidden") || !strings.Contains(out, "shown") {
		t.Fatalf("unexpected output: %s", out)
	}
}
