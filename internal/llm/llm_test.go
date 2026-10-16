package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func okServer(t *testing.T, text string, hits *int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		if r.Header.Get("Authorization") != "Bearer key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["response_format"] == nil {
			t.Error("expected response_format for JSON requests")
		}
		fmt.Fprintf(w, `{"model":"m","choices":[{"message":{"content":%q}}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`, text)
	}))
}

func status(code int, hits *int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*hits++
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(code)
	}))
}

func req() Request {
	return Request{Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 10, JSON: true}
}

func TestFallsBackWhenFirstProviderRateLimited(t *testing.T) {
	var a, b int
	first, second := status(http.StatusTooManyRequests, &a), okServer(t, `{"ok":true}`, &b)
	defer first.Close()
	defer second.Close()

	c := NewChain(
		&Provider{Name: "p1", BaseURL: first.URL, APIKey: "key", Model: "m1"},
		&Provider{Name: "p2", BaseURL: second.URL, APIKey: "key", Model: "m2"},
	)
	resp, err := c.Complete(context.Background(), req())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Provider != "p2" || resp.InputTokens != 10 || resp.OutputTokens != 5 || a != 1 || b != 1 {
		t.Fatalf("resp=%+v a=%d b=%d", resp, a, b)
	}
}

func TestFallsBackOnServerError(t *testing.T) {
	var a, b int
	first, second := status(http.StatusBadGateway, &a), okServer(t, `{}`, &b)
	defer first.Close()
	defer second.Close()
	c := NewChain(
		&Provider{Name: "p1", BaseURL: first.URL, APIKey: "key"},
		&Provider{Name: "p2", BaseURL: second.URL, APIKey: "key"},
	)
	if resp, err := c.Complete(context.Background(), req()); err != nil || resp.Provider != "p2" {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
}

func TestAllRateLimitedReturnsErrRateLimited(t *testing.T) {
	var a int
	s := status(http.StatusTooManyRequests, &a)
	defer s.Close()
	c := NewChain(&Provider{Name: "p1", BaseURL: s.URL, APIKey: "key"})
	_, err := c.Complete(context.Background(), req())
	var rl *ErrRateLimited
	if !errors.As(err, &rl) || rl.RetryAfter.Seconds() != 7 {
		t.Fatalf("err = %v", err)
	}
}

func TestSkipsProvidersWithoutKeys(t *testing.T) {
	c := NewChain(&Provider{Name: "p1", BaseURL: "http://x", APIKey: ""}, nil)
	if c.Enabled() {
		t.Fatal("chain with no keys must be disabled")
	}
}

func TestClientErrorIsNotRetriedOnNextProvider(t *testing.T) {
	var a, b int
	first, second := status(http.StatusUnauthorized, &a), okServer(t, `{}`, &b)
	defer first.Close()
	defer second.Close()
	c := NewChain(
		&Provider{Name: "p1", BaseURL: first.URL, APIKey: "key"},
		&Provider{Name: "p2", BaseURL: second.URL, APIKey: "key"},
	)
	// A bad API key is a configuration error worth surfacing, not hiding.
	if _, err := c.Complete(context.Background(), req()); err == nil || b != 0 {
		t.Fatalf("err=%v b=%d", err, b)
	}
}

func TestVoyageEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Input []string `json:"input"`
			Dim   int      `json:"output_dimension"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		// return out of order to prove indexes are honoured
		fmt.Fprint(w, `{"data":[{"index":1,"embedding":[0,1,0]},{"index":0,"embedding":[1,0,0]}],"usage":{"total_tokens":42}}`)
	}))
	defer srv.Close()
	v := NewVoyage(srv.URL, "key", "voyage-4-lite", 3)
	vecs, tokens, err := v.Embed(context.Background(), []string{"a", "b"}, "document")
	if err != nil || tokens != 42 || vecs[0][0] != 1 || vecs[1][1] != 1 {
		t.Fatalf("vecs=%v tokens=%d err=%v", vecs, tokens, err)
	}
	if _, _, err := NewVoyage(srv.URL, "key", "m", 5).Embed(context.Background(), []string{"a", "b"}, "document"); err == nil {
		t.Fatal("expected dimension mismatch error")
	}
}
