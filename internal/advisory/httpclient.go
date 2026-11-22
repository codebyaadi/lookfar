package advisory

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"time"
)

// Fetcher is the shared HTTP client for collectors: a timeout, an honest
// User-Agent without personal data, transparent gzip, and retry with
// exponential backoff on 429 and 5xx.
type Fetcher struct {
	HTTP       *http.Client
	UserAgent  string
	MaxRetries int           // retries after the first attempt
	Backoff    time.Duration // first delay; doubles each retry
	// RetryOn lists extra status codes to retry (NVD answers 403 when rate limited).
	RetryOn []int
}

// NewFetcher returns a Fetcher with production defaults. Go's transport adds
// Accept-Encoding: gzip and decompresses automatically.
func NewFetcher(version string) *Fetcher {
	return &Fetcher{
		HTTP:       &http.Client{Timeout: 60 * time.Second},
		UserAgent:  "lookfar/" + version + " (+https://github.com/codebyaadi/lookfar)",
		MaxRetries: 3,
		Backoff:    2 * time.Second,
	}
}

// Get performs a GET with the extra headers, retrying on 429/5xx and network
// errors. The caller must close the body of the returned response. A Retry-After
// header (seconds) overrides the backoff delay.
func (f *Fetcher) Get(ctx context.Context, url string, header http.Header) (*http.Response, error) {
	return f.Do(ctx, http.MethodGet, url, header, nil)
}

// Do is Get for any method; body (may be nil) is re-sent on every retry.
func (f *Fetcher) Do(ctx context.Context, method, url string, header http.Header, body []byte) (*http.Response, error) {
	delay := f.Backoff
	var lastErr error
	for attempt := 0; ; attempt++ {
		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, rdr)
		if err != nil {
			return nil, err
		}
		for k, vs := range header {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
		req.Header.Set("User-Agent", f.UserAgent)

		resp, err := f.HTTP.Do(req)
		wait := delay
		switch {
		case err != nil:
			lastErr = err
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 || slices.Contains(f.RetryOn, resp.StatusCode):
			lastErr = fmt.Errorf("%s %s: %s", method, url, resp.Status)
			if s, perr := strconv.Atoi(resp.Header.Get("Retry-After")); perr == nil && s >= 0 {
				wait = time.Duration(s) * time.Second
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		default:
			return resp, nil
		}

		if attempt >= f.MaxRetries || ctx.Err() != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("after %d attempts: %w", attempt+1, lastErr)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		delay *= 2
	}
}
