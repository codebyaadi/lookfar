package advisory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/kev.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseKEVFixture(t *testing.T) {
	entries, err := parseKEV(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("got %d entries", len(entries))
	}
	first := entries[0].Entry
	if first.CVEID == "" || first.VendorProject == "" || first.Product == "" || first.DateAdded == "" {
		t.Errorf("incomplete entry: %+v", first)
	}
	if !json.Valid(entries[0].JSON) {
		t.Error("raw payload must stay valid JSON")
	}
}

func TestParseKEVErrors(t *testing.T) {
	tests := map[string]string{
		"not json":      "{nope",
		"no array":      `{"title":"x"}`,
		"bad entry":     `{"vulnerabilities":["str"]}`,
		"missing field": `{"vulnerabilities":null}`,
	}
	for name, in := range tests {
		if _, err := parseKEV([]byte(in)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	got, err := parseKEV([]byte(`{"vulnerabilities":[{"cveID":""},{"cveID":"CVE-2020-1"}]}`))
	if err != nil || len(got) != 1 {
		t.Errorf("entries without a CVE id must be skipped: %v %v", got, err)
	}
}

func TestNormalizeKEV(t *testing.T) {
	adv := normalizeKEV(kevEntry{
		CVEID: "CVE-2021-44228", VendorProject: "Apache", Product: "Log4j2",
		VulnerabilityName: "Log4Shell", ShortDescription: "JNDI injection.",
		DateAdded: "2021-12-10", RequiredAction: "Patch", DueDate: "2021-12-24",
		KnownRansomwareCampaignUse: "Known", Notes: "see vendor", CWEs: []string{"CWE-502"},
	})
	if adv.ID != "CVE-2021-44228" || !adv.Exploited || adv.Summary != "Log4Shell: JNDI injection." {
		t.Errorf("basic fields: %+v", adv)
	}
	if adv.ExploitedSince == nil || adv.ExploitedSince.Format("2006-01-02") != "2021-12-10" || adv.ExploitedSince.Location() != time.UTC {
		t.Errorf("exploited_since: %v", adv.ExploitedSince)
	}
	var affected []map[string]any
	if err := json.Unmarshal(adv.Affected, &affected); err != nil || len(affected) != 1 ||
		affected[0]["source"] != "kev" || affected[0]["vendorProject"] != "Apache" || affected[0]["product"] != "Log4j2" {
		t.Errorf("affected: %s (%v)", adv.Affected, err)
	}
	var refs []map[string]any
	if err := json.Unmarshal(adv.References, &refs); err != nil || len(refs) != 1 {
		t.Fatalf("refs: %s (%v)", adv.References, err)
	}
	r := refs[0]
	if r["source"] != "kev" || r["dueDate"] != "2021-12-24" || r["requiredAction"] != "Patch" ||
		r["knownRansomwareCampaignUse"] != "Known" || !strings.HasSuffix(r["url"].(string), "CVE-2021-44228") {
		t.Errorf("ref: %v", r)
	}
}

func TestNormalizeKEVMissingDescription(t *testing.T) {
	if got := normalizeKEV(kevEntry{CVEID: "CVE-1", VulnerabilityName: "Name only"}).Summary; got != "Name only" {
		t.Errorf("summary %q", got)
	}
	if got := normalizeKEV(kevEntry{CVEID: "CVE-1", DateAdded: "garbage"}).ExploitedSince; got != nil {
		t.Errorf("bad date must give nil, got %v", got)
	}
}

func testFetcher() *Fetcher {
	f := NewFetcher("test")
	f.Backoff = time.Millisecond
	return f
}

func TestFetcherRetriesOn5xxThen200(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	resp, err := testFetcher().Get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || calls.Load() != 2 {
		t.Errorf("status %d after %d calls", resp.StatusCode, calls.Load())
	}
}

func TestFetcher429HonoursRetryAfterAndGivesUp(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	f := testFetcher()
	f.MaxRetries = 2
	resp, err := f.Get(context.Background(), srv.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "3 attempts") {
		t.Errorf("want give-up error, got %v", err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	if calls.Load() != 3 {
		t.Errorf("calls = %d, want 3", calls.Load())
	}
}

func TestFetcher4xxIsNotRetriedAndHeadersAreSent(t *testing.T) {
	var calls atomic.Int32
	var gotUA, gotINM string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		gotUA, gotINM = r.Header.Get("User-Agent"), r.Header.Get("If-None-Match")
		http.NotFound(w, r)
	}))
	defer srv.Close()

	resp, err := testFetcher().Get(context.Background(), srv.URL, http.Header{"If-None-Match": {`"abc"`}})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound || calls.Load() != 1 {
		t.Errorf("status %d, calls %d", resp.StatusCode, calls.Load())
	}
	if gotUA != "lookfar/test (+https://github.com/codebyaadi/lookfar)" || gotINM != `"abc"` {
		t.Errorf("headers: UA=%q INM=%q", gotUA, gotINM)
	}
}

func TestFetcherStopsOnCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp, err := testFetcher().Get(ctx, srv.URL, nil)
	if err == nil {
		t.Error("expected an error with a cancelled context")
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
}
