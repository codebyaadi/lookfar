package config

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func envMap(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func load(t *testing.T, file string, env map[string]string) (*Config, error) {
	t.Helper()
	return Load("testdata/"+file, envMap(env))
}

func TestDefaults(t *testing.T) {
	cfg, err := load(t, "minimal.yaml", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SeverityThreshold != "high" || cfg.Triage.Provider != "none" {
		t.Errorf("bad defaults: %+v", cfg)
	}
	d := cfg.Delivery.Digest
	if d.Day != "monday" || d.Hour != 9 || d.Timezone != "UTC" {
		t.Errorf("bad digest default: %+v", d)
	}
	if cfg.Delivery.Email != nil || cfg.Delivery.Webhook != nil {
		t.Error("channels should be unset")
	}
}

func TestFullConfig(t *testing.T) {
	cfg, err := load(t, "full.yaml", map[string]string{"WH_SECRET": "whs", "LLM_KEY": "llmk"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SeverityThreshold != "critical" || len(cfg.Stack.Components) != 2 || cfg.Stack.Components[1].Kind != "saas" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if cfg.Delivery.Digest.Hour != 0 || cfg.Delivery.Digest.Day != "friday" {
		t.Errorf("digest: %+v", cfg.Delivery.Digest)
	}
	if cfg.Delivery.Webhook.Secret.Reveal() != "whs" || cfg.Triage.APIKey.Reveal() != "llmk" {
		t.Error("env interpolation failed")
	}
}

func TestSecretMasking(t *testing.T) {
	cfg, err := load(t, "full.yaml", map[string]string{"WH_SECRET": "topsecret-wh", "LLM_KEY": "topsecret-llm"})
	if err != nil {
		t.Fatal(err)
	}
	outputs := []string{
		fmt.Sprintf("%v", cfg), fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg),
		fmt.Sprint(cfg.Delivery.Email), cfg.Summary(),
	}
	for _, out := range outputs {
		for _, leak := range []string{"topsecret", "pw@mail"} {
			if strings.Contains(out, leak) {
				t.Errorf("output leaks %q: %s", leak, out)
			}
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		env  map[string]string
		want []string // each must appear in the error text
	}{
		{"empty", "  \n", nil, []string{"config is empty"}},
		{"bad yaml", "domains: [a\n", nil, []string{"invalid YAML"}},
		{"unknown field", "domainz: [a.com]\n", nil, []string{"invalid config", "domainz"}},
		{"severity", "severity_threshold: urgent\n", nil,
			[]string{"t.yaml:1:21: severity_threshold: must be one of: low medium high critical"}},
		{"domain scheme", "domains:\n  - https://a.com\n", nil,
			[]string{"t.yaml:2:5: domains[0]:", "no scheme"}},
		{"domain uppercase", "domains: [A.com]\n", nil, []string{"t.yaml:1:11: domains[0]:"}},
		{"domain no tld", "domains: [localhost]\n", nil, []string{"domains[0]"}},
		{"brand term", "brand_terms: [\"Bad Term\"]\n", nil, []string{"brand_terms[0]"}},
		{"digest day", "delivery:\n  digest: { day: someday }\n", nil, []string{"t.yaml:2:", "delivery.digest.day: must be one of"}},
		{"digest hour", "delivery:\n  digest: { hour: 24 }\n", nil, []string{"delivery.digest.hour: must be at most 23"}},
		{"timezone", "delivery:\n  digest: { timezone: Mars/Base }\n", nil, []string{"delivery.digest.timezone: unknown timezone"}},
		{"provider enum", "triage: { provider: gemini }\n", nil, []string{"triage.provider: must be one of: none anthropic openai"}},
		{"api key required", "triage:\n  provider: anthropic\n  model: m\n", nil, []string{"triage.api_key: is required"}},
		{"api key env unset", "triage:\n  provider: anthropic\n  model: m\n  api_key: ${K}\n", nil, []string{"t.yaml:4:", "triage.api_key: is required"}},
		{"missing env required", "delivery:\n  webhook: { url: \"https://h.com\", secret: \"${S}\" }\n", nil,
			[]string{"t.yaml:2:", "delivery.webhook.secret: environment variable S is not set"}},
		{"email invalid", "delivery:\n  email: { to: nope, smtp_url: \"http://x\" }\n", nil,
			[]string{"delivery.email.to", "delivery.email.smtp_url: must be an smtp"}},
		{"component name", "stack:\n  components:\n    - version: \"1\"\n", nil, []string{"stack.components[0].name: is required"}},
		{"component kind", "stack:\n  components:\n    - { name: a, kind: weird }\n", nil, []string{"stack.components[0].kind"}},
		{"budget", "triage: { daily_budget: 0 }\n", nil, []string{"triage.daily_budget: must be at least 1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse("t.yaml", []byte(tc.yaml), envMap(tc.env))
			if err == nil {
				t.Fatal("expected error")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q missing %q", err.Error(), w)
				}
			}
		})
	}
}

func TestInvalidFixtureReportsAllWithPositions(t *testing.T) {
	_, err := load(t, "invalid.yaml", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{
		"invalid.yaml:1:", "severity_threshold",
		"invalid.yaml:4:5: domains[1]", "invalid.yaml:5:5: domains[2]",
		"brand_terms[1]", "delivery.digest.day", "delivery.digest.hour", "delivery.digest.timezone",
		"triage.model: is required", "triage.api_key: is required",
	} {
		if !strings.Contains(err.Error(), "testdata/"+want) && !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in:\n%s", want, err)
		}
	}
}

func TestOptionalEnvExpandsEmpty(t *testing.T) {
	cfg, err := Parse("t.yaml", []byte("mission: \"hi ${NOPE}\"\ntriage: { provider: none, api_key: \"${K}\" }\n"), envMap(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mission != "hi " || cfg.Triage.APIKey != "" {
		t.Errorf("got %q %q", cfg.Mission, cfg.Triage.APIKey)
	}
}

func TestEnvInterpolationInList(t *testing.T) {
	cfg, err := Parse("t.yaml", []byte("domains: [\"${D}\"]\n"), envMap(map[string]string{"D": "example.org"}))
	if err != nil || cfg.Domains[0] != "example.org" {
		t.Fatalf("got %v %v", cfg, err)
	}
}

func TestResolvePath(t *testing.T) {
	env := func(v string) func(string) string {
		return func(string) string { return v }
	}
	tests := []struct{ flag, env, want string }{
		{"a.yaml", "b.yaml", "a.yaml"},
		{"", "b.yaml", "b.yaml"},
		{"", "", "lookfar.yaml"},
	}
	for _, tc := range tests {
		if got := ResolvePath(tc.flag, env(tc.env)); got != tc.want {
			t.Errorf("ResolvePath(%q,%q) = %q, want %q", tc.flag, tc.env, got, tc.want)
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load("testdata/nope.yaml", os.LookupEnv); err == nil {
		t.Fatal("expected error")
	}
}
