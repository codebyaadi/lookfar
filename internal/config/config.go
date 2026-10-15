// Package config loads runtime configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

type Config struct {
	Env         string // "development" or "production"
	LogLevel    slog.Level
	DatabaseURL string

	// API
	APIAddr     string
	CORSOrigins []string
	// AuthSecret is better-auth's BETTER_AUTH_SECRET, used to verify the
	// signature on the session cookie.
	AuthSecret string

	// AllowPrivateFeeds disables the SSRF guard so local test feeds work.
	// Never enable in production.
	AllowPrivateFeeds bool

	// Worker
	WorkerTick time.Duration
}

func (c Config) IsProduction() bool { return c.Env == "production" }

// Load reads configuration from environment variables. Values from a .env
// file must be exported by the caller (the Makefile does this).
func Load() (Config, error) {
	c := Config{
		Env:         getenv("APP_ENV", "development"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		APIAddr:     getenv("API_ADDR", ":8080"),
		CORSOrigins: splitList(getenv("CORS_ORIGINS", "http://localhost:3000")),
		AuthSecret:  os.Getenv("BETTER_AUTH_SECRET"),

		AllowPrivateFeeds: getenv("ALLOW_PRIVATE_FEEDS", "false") == "true",
	}

	if err := c.LogLevel.UnmarshalText([]byte(getenv("LOG_LEVEL", "info"))); err != nil {
		return c, fmt.Errorf("LOG_LEVEL: %w", err)
	}

	tick, err := time.ParseDuration(getenv("WORKER_TICK", "30s"))
	if err != nil {
		return c, fmt.Errorf("WORKER_TICK: %w", err)
	}
	c.WorkerTick = tick

	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if c.AuthSecret == "" {
		errs = append(errs, errors.New("BETTER_AUTH_SECRET is required"))
	}
	return c, errors.Join(errs...)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
