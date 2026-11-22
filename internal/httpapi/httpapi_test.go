package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	return rec
}

func TestHealth(t *testing.T) {
	okPing := func(context.Context) error { return nil }
	badPing := func(context.Context) error { return errors.New("down") }
	tests := []struct {
		name     string
		deps     Deps
		wantCode int
		want     []string
	}{
		{"ok", Deps{Version: "v1", Ping: okPing, RiverRunning: func() bool { return true }},
			200, []string{`"status":"ok"`, `"database":"ok"`, `"river":"running"`, `"version":"v1"`}},
		{"db down", Deps{Ping: badPing, RiverRunning: func() bool { return true }},
			503, []string{`"status":"degraded"`, `"database":"error"`}},
		{"river stopped", Deps{Ping: okPing, RiverRunning: func() bool { return false }},
			503, []string{`"river":"stopped"`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, New(tc.deps), "/api/health")
			if rec.Code != tc.wantCode {
				t.Fatalf("code %d: %s", rec.Code, rec.Body)
			}
			for _, w := range tc.want {
				if !strings.Contains(rec.Body.String(), w) {
					t.Errorf("body %s missing %s", rec.Body, w)
				}
			}
		})
	}
}

func TestRouting(t *testing.T) {
	h := New(Deps{})
	if rec := get(t, h, "/api/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown api path: %d", rec.Code)
	}
	// The SPA fallback answers other paths (404 only because dist is not built in tests).
	if rec := get(t, h, "/findings"); rec.Code == http.StatusInternalServerError {
		t.Errorf("spa path: %d", rec.Code)
	}
}
