// Package config loads, interpolates and validates lookfar.yaml.
package config

import (
	"fmt"
	"log/slog"
)

// Config is the full lookfar.yaml schema.
type Config struct {
	Mission           string   `yaml:"mission"`
	SeverityThreshold string   `yaml:"severity_threshold" validate:"oneof=low medium high critical"`
	Stack             Stack    `yaml:"stack"`
	Domains           []string `yaml:"domains" validate:"dive,domain"`
	BrandTerms        []string `yaml:"brand_terms" validate:"dive,brandterm"`
	Delivery          Delivery `yaml:"delivery"`
	Triage            Triage   `yaml:"triage"`
}

// Stack declares the software the user runs.
type Stack struct {
	Manifests  []Manifest  `yaml:"manifests" validate:"dive"`
	Components []Component `yaml:"components" validate:"dive"`
}

// Manifest points at a lockfile, manifest, Dockerfile or SBOM.
type Manifest struct {
	Path string `yaml:"path" validate:"required"`
}

// Component is a manually declared piece of software.
type Component struct {
	Name    string `yaml:"name" validate:"required"`
	Version string `yaml:"version"`
	Kind    string `yaml:"kind" validate:"omitempty,oneof=package server image saas"`
}

// Delivery configures where findings are sent.
type Delivery struct {
	Email   *Email   `yaml:"email"`
	Webhook *Webhook `yaml:"webhook"`
	Digest  Digest   `yaml:"digest"`
}

// Email is the SMTP digest and alert channel.
type Email struct {
	To      string `yaml:"to" validate:"required,email"`
	SMTPURL Secret `yaml:"smtp_url" validate:"required,smtpurl"`
}

// Webhook is the signed webhook channel.
type Webhook struct {
	URL    string `yaml:"url" validate:"required,http_url"`
	Secret Secret `yaml:"secret" validate:"required"`
}

// Digest is the weekly digest schedule.
type Digest struct {
	Day      string `yaml:"day" validate:"oneof=monday tuesday wednesday thursday friday saturday sunday"`
	Hour     int    `yaml:"hour" validate:"min=0,max=23"`
	Timezone string `yaml:"timezone" validate:"timezone"`
}

// Triage configures optional LLM triage.
type Triage struct {
	Provider    string `yaml:"provider" validate:"oneof=none anthropic openai"`
	Model       string `yaml:"model" validate:"required_unless=Provider none"`
	APIKey      Secret `yaml:"api_key" validate:"required_unless=Provider none"`
	DailyBudget int    `yaml:"daily_budget" validate:"min=1"`
}

// Defaults returns a Config with all default values applied.
func Defaults() Config {
	return Config{
		SeverityThreshold: "high",
		Delivery:          Delivery{Digest: Digest{Day: "monday", Hour: 9, Timezone: "UTC"}},
		Triage:            Triage{Provider: "none", DailyBudget: 100},
	}
}

// Secret is a string that never prints its value. Use Reveal to read it.
type Secret string

// Reveal returns the underlying value.
func (s Secret) Reveal() string { return string(s) }

// String implements fmt.Stringer.
func (s Secret) String() string { return s.mask() }

// GoString implements fmt.GoStringer.
func (s Secret) GoString() string { return `"` + s.mask() + `"` }

// MarshalText implements encoding.TextMarshaler (covers JSON and YAML output).
func (s Secret) MarshalText() ([]byte, error) { return []byte(s.mask()), nil }

// LogValue implements slog.LogValuer.
func (s Secret) LogValue() slog.Value { return slog.StringValue(s.mask()) }

func (s Secret) mask() string {
	if s == "" {
		return ""
	}
	return "***"
}

// Summary returns a short, secret-free description of the config.
func (c *Config) Summary() string {
	channels := "none"
	switch {
	case c.Delivery.Email != nil && c.Delivery.Webhook != nil:
		channels = "email, webhook"
	case c.Delivery.Email != nil:
		channels = "email"
	case c.Delivery.Webhook != nil:
		channels = "webhook"
	}
	return fmt.Sprintf(
		"manifests: %d\ncomponents: %d\ndomains: %d\nbrand terms: %d\nseverity threshold: %s\ntriage provider: %s\ndelivery channels: %s\ndigest: %s %02d:00 %s",
		len(c.Stack.Manifests), len(c.Stack.Components), len(c.Domains), len(c.BrandTerms),
		c.SeverityThreshold, c.Triage.Provider, channels,
		c.Delivery.Digest.Day, c.Delivery.Digest.Hour, c.Delivery.Digest.Timezone,
	)
}
