// Package llm talks to OpenAI-compatible chat APIs (Groq, OpenRouter, ...)
// through an ordered fallback chain with per-provider rate limiting.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// ErrRateLimited means every provider is out of quota right now; callers
// should retry later instead of counting it as a failure.
type ErrRateLimited struct{ RetryAfter time.Duration }

func (e *ErrRateLimited) Error() string {
	return fmt.Sprintf("llm: rate limited (retry after %s)", e.RetryAfter)
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Messages  []Message
	MaxTokens int
	// JSON asks the provider for a JSON object response.
	JSON bool
}

type Response struct {
	Text         string
	Model        string
	Provider     string
	InputTokens  int
	OutputTokens int
}

type Provider struct {
	Name    string
	BaseURL string
	APIKey  string
	Model   string
	// RPM caps requests per minute (0 = unlimited).
	RPM int
	// Extra is merged into the request body (e.g. reasoning_effort).
	Extra map[string]any

	limiter *rate.Limiter
}

type Chain struct {
	providers []*Provider
	http      *http.Client
}

func NewChain(providers ...*Provider) *Chain {
	var ps []*Provider
	for _, p := range providers {
		if p == nil || p.APIKey == "" {
			continue
		}
		if p.RPM > 0 {
			p.limiter = rate.NewLimiter(rate.Limit(float64(p.RPM)/60), min(p.RPM, 5))
		}
		ps = append(ps, p)
	}
	return &Chain{providers: ps, http: &http.Client{Timeout: 90 * time.Second}}
}

// Enabled reports whether at least one provider is configured.
func (c *Chain) Enabled() bool { return len(c.providers) > 0 }

// Complete tries each provider in order. Rate limits, 5xx responses and
// network errors move on to the next provider.
func (c *Chain) Complete(ctx context.Context, req Request) (*Response, error) {
	if !c.Enabled() {
		return nil, errors.New("llm: no providers configured")
	}
	var (
		lastErr    error
		limited    int
		retryAfter = 30 * time.Second
	)
	for _, p := range c.providers {
		if p.limiter != nil && !p.limiter.Allow() {
			// Short wait for a token before falling through.
			wait, cancel := context.WithTimeout(ctx, 8*time.Second)
			err := p.limiter.Wait(wait)
			cancel()
			if err != nil {
				limited++
				lastErr = fmt.Errorf("%s: local rate limit", p.Name)
				continue
			}
		}
		resp, err := c.call(ctx, p, req)
		if err == nil {
			return resp, nil
		}
		var rl *ErrRateLimited
		switch {
		case errors.As(err, &rl):
			limited++
			retryAfter = max(rl.RetryAfter, 5*time.Second)
			slog.Warn("llm provider rate limited", "provider", p.Name)
		case ctx.Err() != nil:
			return nil, ctx.Err()
		case isRetryable(err):
			slog.Warn("llm provider failed, trying next", "provider", p.Name, "err", err)
		default:
			return nil, fmt.Errorf("%s: %w", p.Name, err)
		}
		lastErr = fmt.Errorf("%s: %w", p.Name, err)
	}
	if limited == len(c.providers) {
		return nil, &ErrRateLimited{RetryAfter: retryAfter}
	}
	return nil, lastErr
}

type statusError struct {
	code int
	body string
}

func (e *statusError) Error() string { return fmt.Sprintf("status %d: %s", e.code, e.body) }

func isRetryable(err error) bool {
	var se *statusError
	if errors.As(err, &se) {
		// 400/404/422 usually mean this model rejects a parameter; the next
		// provider may accept it.
		return se.code >= 500 || se.code == 400 || se.code == 404 || se.code == 422
	}
	return true // network error / timeout
}

func (c *Chain) call(ctx context.Context, p *Provider, req Request) (*Response, error) {
	body := map[string]any{
		"model":      p.Model,
		"messages":   req.Messages,
		"max_tokens": req.MaxTokens,
	}
	if req.JSON {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	for k, v := range p.Extra {
		body[k] = v
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.BaseURL, "/")+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Authorization", "Bearer "+p.APIKey)
	hreq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		after := 30 * time.Second
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			after = time.Duration(s) * time.Second
		}
		return nil, &ErrRateLimited{RetryAfter: after}
	}
	if resp.StatusCode/100 != 2 {
		return nil, &statusError{code: resp.StatusCode, body: truncate(string(data), 300)}
	}

	var out struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			Prompt     int `json:"prompt_tokens"`
			Completion int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return nil, &statusError{code: 502, body: "empty completion"}
	}
	model := out.Model
	if model == "" {
		model = p.Model
	}
	return &Response{
		Text: out.Choices[0].Message.Content, Model: model, Provider: p.Name,
		InputTokens: out.Usage.Prompt, OutputTokens: out.Usage.Completion,
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
