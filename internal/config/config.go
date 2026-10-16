// Package config loads runtime configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
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

	// AI providers. Everything is optional: with no keys the app runs as a
	// plain reader and the worker skips enrichment.
	GroqAPIKey        string
	GroqModel         string
	GroqBaseURL       string
	OpenRouterAPIKey  string
	OpenRouterModel   string
	OpenRouterBaseURL string
	LLMRPM            int // per-provider requests per minute
	VoyageAPIKey      string
	VoyageModel       string
	VoyageBaseURL     string
	EmbedBatch        int
	EnrichConcurrency int
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

		GroqAPIKey:        os.Getenv("GROQ_API_KEY"),
		GroqModel:         getenv("GROQ_MODEL", "openai/gpt-oss-20b"),
		GroqBaseURL:       getenv("GROQ_BASE_URL", "https://api.groq.com/openai/v1"),
		OpenRouterAPIKey:  os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:   getenv("OPENROUTER_MODEL", "openai/gpt-oss-20b:free"),
		OpenRouterBaseURL: getenv("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
		VoyageAPIKey:      os.Getenv("VOYAGE_API_KEY"),
		VoyageModel:       getenv("VOYAGE_MODEL", "voyage-4-lite"),
		VoyageBaseURL:     getenv("VOYAGE_BASE_URL", "https://api.voyageai.com/v1"),
	}

	if err := c.LogLevel.UnmarshalText([]byte(getenv("LOG_LEVEL", "info"))); err != nil {
		return c, fmt.Errorf("LOG_LEVEL: %w", err)
	}

	tick, err := time.ParseDuration(getenv("WORKER_TICK", "30s"))
	if err != nil {
		return c, fmt.Errorf("WORKER_TICK: %w", err)
	}
	c.WorkerTick = tick

	if c.LLMRPM, err = getenvInt("LLM_RPM", 25); err != nil {
		return c, err
	}
	if c.EmbedBatch, err = getenvInt("EMBED_BATCH", 16); err != nil {
		return c, err
	}
	if c.EnrichConcurrency, err = getenvInt("ENRICH_CONCURRENCY", 2); err != nil {
		return c, err
	}

	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if c.AuthSecret == "" {
		errs = append(errs, errors.New("BETTER_AUTH_SECRET is required"))
	}
	return c, errors.Join(errs...)
}

// AIEnabled reports whether any chat-completion provider is configured.
func (c Config) AIEnabled() bool { return c.GroqAPIKey != "" || c.OpenRouterAPIKey != "" }

func getenvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return n, nil
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
