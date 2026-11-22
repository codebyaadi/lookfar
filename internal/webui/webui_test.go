package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	h := handler(fstest.MapFS{
		"index.html":   {Data: []byte("<html>spa</html>")},
		"_app/main.js": {Data: []byte("console.log(1)")},
	})
	tests := []struct {
		path     string
		wantCode int
		wantBody string
	}{
		{"/", 200, "spa"},
		{"/_app/main.js", 200, "console.log"},
		{"/findings/42", 200, "spa"},
		{"/api/missing", 404, ""},
	}
	for _, tc := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, nil))
		if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantBody) {
			t.Errorf("%s: got %d %q", tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestHandlerWithoutBuild(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(fstest.MapFS{}).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if rec.Code != 404 {
		t.Fatalf("got %d", rec.Code)
	}
}
