// Package httpapi wires the HTTP routes for the Lookfar API.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/codebyaadi/lookfar/internal/auth"
	"github.com/codebyaadi/lookfar/internal/store"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	Auth        *auth.Authenticator
	DB          Pinger
	Queries     *store.Queries
	CORSOrigins []string
	// AIEnabled tells the UI whether a built-in LLM provider is configured.
	AIEnabled bool
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   d.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/healthz", healthz(d.DB))

	r.Route("/v1", func(r chi.Router) {
		r.Use(requireUser(d.Auth))
		r.Get("/me", getMe(d.AIEnabled))
		(&inbox{q: d.Queries}).routes(r)
	})

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	})
	return r
}

func healthz(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			slog.ErrorContext(r.Context(), "healthz: database ping failed", "err", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func getMe(aiEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"user":     userFrom(r.Context()),
			"features": map[string]bool{"ai": aiEnabled},
		})
	}
}

type ctxKey struct{}

func userFrom(ctx context.Context) auth.User {
	u, _ := ctx.Value(ctxKey{}).(auth.User)
	return u
}

func requireUser(a *auth.Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := a.Authenticate(r)
			switch {
			case errors.Is(err, auth.ErrNoSession), errors.Is(err, auth.ErrInvalidSession):
				writeError(w, http.StatusUnauthorized, "unauthorized", "sign in required")
				return
			case err != nil:
				slog.ErrorContext(r.Context(), "authenticate", "err", err)
				writeError(w, http.StatusInternalServerError, "internal", "could not verify session")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, user)))
		})
	}
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.InfoContext(r.Context(), "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration", time.Since(start),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}
