package stack

import (
	"strings"
	"testing"
)

func TestLookupServerAliases(t *testing.T) {
	tests := map[string]string{
		"postgresql": "postgresql", "PostgreSQL": "postgresql", "postgres": "postgresql",
		"Node.js": "nodejs", "node": "nodejs", "node-js": "nodejs",
		"Apache HTTPD": "apache-httpd", "apache_httpd": "apache-httpd", "httpd": "apache-httpd",
		"nginx": "nginx", "Kafka": "kafka", "apache kafka": "kafka",
	}
	for in, want := range tests {
		s, ok := LookupServer(in)
		if !ok || s.Name != want {
			t.Errorf("LookupServer(%q) = %q, %v; want %q", in, s.Name, ok, want)
		}
	}
	if _, ok := LookupServer("stripe"); ok {
		t.Error("stripe is not curated")
	}
}

func TestCuratedListShape(t *testing.T) {
	if len(servers) != 20 {
		t.Errorf("want 20 curated entries, got %d", len(servers))
	}
	for _, s := range servers {
		if s.Name == "" || s.Vendor == "" || s.Product == "" {
			t.Errorf("incomplete entry: %+v", s)
		}
	}
	// Every alias must resolve to exactly its own entry (no collisions).
	for _, s := range servers {
		for _, n := range append([]string{s.Name}, s.Aliases...) {
			if got, _ := LookupServer(n); got.Name != s.Name {
				t.Errorf("alias %q of %s resolves to %s", n, s.Name, got.Name)
			}
		}
	}
}

func TestFromManual(t *testing.T) {
	tests := []struct {
		name       string
		in         ManualSpec
		purl, cpe  string
		kind, eco  string
		unresolved bool
	}{
		{"server inferred", ManualSpec{Name: "PostgreSQL", Version: "16.4"},
			"pkg:generic/postgresql@16.4", "cpe:2.3:a:postgresql:postgresql:16.4:*:*:*:*:*:*:*", "server", "generic", false},
		{"server alias", ManualSpec{Name: "Node.js", Version: "22.1.0", Kind: "server"},
			"pkg:generic/nodejs@22.1.0", "cpe:2.3:a:nodejs:node.js:22.1.0:*:*:*:*:*:*:*", "server", "generic", false},
		{"server no version", ManualSpec{Name: "nginx"},
			"pkg:generic/nginx", "cpe:2.3:a:f5:nginx:*:*:*:*:*:*:*:*", "server", "generic", true},
		{"saas", ManualSpec{Name: "stripe", Kind: "saas"}, "pkg:generic/stripe", "", "saas", "generic", false},
		{"image", ManualSpec{Name: "redis", Version: "7.2", Kind: "image"}, "pkg:oci/redis@7.2", "", "image", "oci", false},
		{"image registry", ManualSpec{Name: "ghcr.io/org/app", Version: "1.0", Kind: "image"},
			"pkg:oci/org/app@1.0?repository_url=ghcr.io", "", "image", "oci", false},
		{"npm scoped", ManualSpec{Name: "@scope/pkg", Version: "1.0.0", Kind: "package", Ecosystem: "npm"},
			"pkg:npm/%40scope/pkg@1.0.0", "", "package", "npm", false},
		{"pypi normalized", ManualSpec{Name: "Flask_Login", Version: "0.6.3", Kind: "package", Ecosystem: "PyPI"},
			"pkg:pypi/flask-login@0.6.3", "", "package", "PyPI", false},
		{"go module", ManualSpec{Name: "github.com/spf13/cobra", Version: "1.8.0", Kind: "package", Ecosystem: "Go"},
			"pkg:golang/github.com/spf13/cobra@1.8.0", "", "package", "Go", false},
		{"package no version", ManualSpec{Name: "left-pad", Kind: "package", Ecosystem: "npm"},
			"pkg:npm/left-pad", "", "package", "npm", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := FromManual(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if c.PURL != tc.purl || c.CPE != tc.cpe || c.Kind != tc.kind || c.Ecosystem != tc.eco ||
				c.Unresolved != tc.unresolved || c.Origin != OriginManual {
				t.Errorf("got %+v", c)
			}
		})
	}
}

func TestFromManualErrors(t *testing.T) {
	tests := []struct {
		name string
		in   ManualSpec
		want string
	}{
		{"unknown name without kind", ManualSpec{Name: "stripe"}, "needs a kind"},
		{"unknown server", ManualSpec{Name: "oracle-db", Kind: "server"}, "unknown server software"},
		{"package without ecosystem", ManualSpec{Name: "x", Kind: "package"}, "needs an ecosystem"},
		{"bad ecosystem", ManualSpec{Name: "x", Kind: "package", Ecosystem: "rubygems"}, "unsupported ecosystem"},
		{"bad kind", ManualSpec{Name: "x", Kind: "weird"}, "unknown kind"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := FromManual(tc.in)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got %v, want error containing %q", err, tc.want)
			}
		})
	}
}

func TestInferKind(t *testing.T) {
	if k, ok := InferKind("redis", ""); !ok || k != KindServer {
		t.Errorf("redis: %q %v", k, ok)
	}
	if _, ok := InferKind("stripe", ""); ok {
		t.Error("stripe should not infer")
	}
	if k, _ := InferKind("redis", "image"); k != "image" {
		t.Errorf("declared kind must win, got %q", k)
	}
}

func TestResolveCollectsErrorsAndDedupes(t *testing.T) {
	r := Resolve(t.Context(), []string{"testdata/go.mod", "testdata/missing.lock"}, []ManualSpec{
		{Name: "redis", Version: "7"}, {Name: "Redis", Version: "7"}, {Name: "stripe"},
	})
	if len(r.Files) != 2 || r.Files[0].Count != 3 || r.Files[1].Err == nil {
		t.Errorf("files: %+v", r.Files)
	}
	if len(r.ManualErrors) != 1 { // stripe has no kind
		t.Errorf("manual errors: %v", r.ManualErrors)
	}
	if len(r.Components) != 4 { // 3 from go.mod + one deduped redis
		t.Errorf("components: %d", len(r.Components))
	}
}
