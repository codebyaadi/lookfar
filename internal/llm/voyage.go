package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Voyage is a client for Voyage AI's embeddings endpoint.
type Voyage struct {
	BaseURL string
	APIKey  string
	Model   string
	Dim     int
	http    *http.Client
}

func NewVoyage(baseURL, apiKey, model string, dim int) *Voyage {
	return &Voyage{BaseURL: baseURL, APIKey: apiKey, Model: model, Dim: dim, http: &http.Client{Timeout: 60 * time.Second}}
}

func (v *Voyage) Enabled() bool { return v != nil && v.APIKey != "" }

// Embed returns one vector per input text. inputType is "document" or "query".
func (v *Voyage) Embed(ctx context.Context, texts []string, inputType string) ([][]float32, int, error) {
	raw, err := json.Marshal(map[string]any{
		"input": texts, "model": v.Model, "input_type": inputType,
		"output_dimension": v.Dim, "truncation": true,
	})
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(v.BaseURL, "/")+"/embeddings", bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+v.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := v.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		after := 60 * time.Second
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			after = time.Duration(s) * time.Second
		}
		return nil, 0, &ErrRateLimited{RetryAfter: after}
	}
	if resp.StatusCode/100 != 2 {
		return nil, 0, &statusError{code: resp.StatusCode, body: truncate(string(data), 300)}
	}
	var out struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, 0, fmt.Errorf("decode embeddings: %w", err)
	}
	if len(out.Data) != len(texts) {
		return nil, 0, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(out.Data))
	}
	vecs := make([][]float32, len(texts))
	for _, d := range out.Data {
		if d.Index < 0 || d.Index >= len(vecs) || len(d.Embedding) != v.Dim {
			return nil, 0, fmt.Errorf("bad embedding at index %d (len %d, want %d)", d.Index, len(d.Embedding), v.Dim)
		}
		vecs[d.Index] = d.Embedding
	}
	return vecs, out.Usage.TotalTokens, nil
}
