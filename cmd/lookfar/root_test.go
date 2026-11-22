package main

import (
	"bytes"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
	return strings.TrimSpace(out.String())
}

func TestVersion(t *testing.T) {
	if got := run(t, "version"); got != version {
		t.Fatalf("got %q, want %q", got, version)
	}
}

func TestPlaceholders(t *testing.T) {
	for _, name := range []string{"serve", "migrate", "config", "run", "mcp", "token"} {
		if got := run(t, name); got != "not implemented yet" {
			t.Errorf("%s: got %q", name, got)
		}
	}
}
