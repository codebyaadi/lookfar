// Package httpapi builds the HTTP handler: the Huma API under /api and the
// embedded dashboard on every other path.
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/codebyaadi/lookfar/internal/webui"
)

// Deps are the live checks the health endpoint reports on.
type Deps struct {
	Version string
	// Ping checks the database connection.
	Ping func(ctx context.Context) error
	// RiverRunning reports whether the job client is still running.
	RiverRunning func() bool
}

// HealthBody is the response of GET /api/health.
type HealthBody struct {
	Status   string `json:"status" enum:"ok,degraded" doc:"ok when every check passes"`
	Version  string `json:"version"`
	Database string `json:"database" enum:"ok,error"`
	River    string `json:"river" enum:"running,stopped"`
}

// HealthOutput carries the HTTP status next to the body.
type HealthOutput struct {
	Status int
	Body   HealthBody
}

// New returns the handler for the whole server.
func New(deps Deps) http.Handler {
	mux := http.NewServeMux()

	cfg := huma.DefaultConfig("lookfar", deps.Version)
	cfg.DocsPath = "" // interactive docs arrive with the public API in a later phase
	api := humago.NewWithPrefix(mux, "/api", cfg)

	huma.Register(api, huma.Operation{
		OperationID: "get-health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Health check",
		Tags:        []string{"system"},
	}, func(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		body := HealthBody{Status: "ok", Version: deps.Version, Database: "ok", River: "running"}
		if deps.Ping == nil || deps.Ping(pingCtx) != nil {
			body.Database = "error"
		}
		if deps.RiverRunning == nil || !deps.RiverRunning() {
			body.River = "stopped"
		}
		out := &HealthOutput{Status: http.StatusOK, Body: body}
		if body.Database != "ok" || body.River != "running" {
			body.Status = "degraded"
			out.Body = body
			out.Status = http.StatusServiceUnavailable
		}
		return out, nil
	})

	// Unknown /api paths must not fall through to the SPA.
	mux.Handle("/api/", http.NotFoundHandler())
	mux.Handle("/", webui.Handler())
	return mux
}
