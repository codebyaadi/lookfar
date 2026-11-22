package stack

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type want struct {
	purl       string
	version    string
	constraint string
	unresolved bool
}

func purls(cs []Component) []want {
	out := make([]want, len(cs))
	for i, c := range cs {
		out[i] = want{c.PURL, c.Version, c.Constraint, c.Unresolved}
	}
	return out
}

func TestExtract(t *testing.T) {
	tests := []struct {
		file string
		want []want
	}{
		{"package-lock.json", []want{
			{purl: "pkg:npm/%40types/node@22.1.0", version: "22.1.0"},
			{purl: "pkg:npm/left-pad@1.3.0", version: "1.3.0"}, // nested duplicate is deduped
		}},
		{"package.json", []want{
			{purl: "pkg:npm/%40scope/pkg", constraint: "~2.0.1", unresolved: true},
			{purl: "pkg:npm/left-pad", constraint: "^1.3.0", unresolved: true},
			{purl: "pkg:npm/vitest", constraint: ">=1 <3", unresolved: true},
		}},
		{"bun.lock", []want{
			{purl: "pkg:npm/%40types/node@22.1.0", version: "22.1.0"},
			{purl: "pkg:npm/left-pad@1.3.0", version: "1.3.0"},
		}},
		{"requirements.txt", []want{
			{purl: "pkg:pypi/django", unresolved: true}, // range only, no exact version
			{purl: "pkg:pypi/flask@3.0.0", version: "3.0.0"},
			{purl: "pkg:pypi/requests@2.31.0", version: "2.31.0"}, // listed twice, deduped
		}},
		{"poetry.lock", []want{
			{purl: "pkg:pypi/flask-login@0.6.3", version: "0.6.3"},
			{purl: "pkg:pypi/requests@2.31.0", version: "2.31.0"},
		}},
		{"uv.lock", []want{
			{purl: "pkg:pypi/requests@2.31.0", version: "2.31.0"}, // the virtual root project is skipped
		}},
		{"go.mod", []want{
			{purl: "pkg:golang/github.com/spf13/cobra@1.8.0", version: "1.8.0"},
			{purl: "pkg:golang/golang.org/x/net@0.20.0", version: "0.20.0"},
			{purl: "pkg:golang/stdlib@1.22", version: "1.22"}, // from the go directive
		}},
		{"bom.cdx.json", []want{
			{purl: "pkg:maven/com.fasterxml.jackson.core/jackson-databind@2.15.0", version: "2.15.0"},
			{purl: "pkg:npm/lodash@4.17.21", version: "4.17.21"},
		}},
		{"sbom.spdx.json", []want{
			{purl: "pkg:npm/express@4.18.2", version: "4.18.2"},
		}},
		{"Dockerfile", []want{
			{purl: "pkg:oci/%24%7BBASE%7D", unresolved: true},
			{purl: "pkg:oci/golang@1.22-alpine", version: "1.22-alpine"}, // ARG resolved, --platform skipped
			{purl: "pkg:oci/nginx@latest", version: "latest"},
			{purl: "pkg:oci/node@20?digest=sha256:abcdef", version: "20"},
			{purl: "pkg:oci/org/app@2.1?repository_url=ghcr.io", version: "2.1"},
			{purl: "pkg:oci/redis@%24%7BUNSET_TAG%7D", version: "${UNSET_TAG}", unresolved: true},
			{purl: "pkg:oci/team/tool@v1?repository_url=localhost:5000", version: "v1"},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join("testdata", tc.file)
			got, err := Extract(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, purls(got), cmp.AllowUnexported(want{})); diff != "" {
				t.Errorf("components mismatch (-want +got):\n%s", diff)
			}
			for _, c := range got {
				if c.Manifest != path || c.Ecosystem == "" || c.Name == "" {
					t.Errorf("incomplete component: %+v", c)
				}
			}
		})
	}
}

func TestDockerfileEdgeCases(t *testing.T) {
	src := "ARG V=3\n" +
		"FROM base AS one\n" + // "base" is an external image, "one" a stage
		"FROM one AS two\n" + // stage reference: skipped
		"FROM ONE\n" + // stage names are case-insensitive: skipped
		"FROM \\\n  alpine:${V} \\\n  AS multi\n" + // line continuation
		"FROM --platform=linux/arm64 debian:12 as last\n"
	got, err := parseDockerfile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range got {
		names = append(names, c.PURL)
	}
	wantNames := []string{"pkg:oci/base@latest", "pkg:oci/alpine@3", "pkg:oci/debian@12"}
	if diff := cmp.Diff(wantNames, names); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestExtractErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte{0}, 0o600); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "package-lock.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		path        string
		unsupported bool
		contains    string
	}{
		{"unsupported json", "testdata/bad.json", true, "bad.json"},
		{"unsupported ext", "testdata/notes.md", true, "notes.md"},
		{"binary bun lockfile", filepath.Join(dir, "bun.lockb"), true, "bun.lockb"},
		{"missing file", "testdata/nope.txt", false, "read manifest"},
		{"corrupt lockfile", bad, false, "package-lock.json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Extract(t.Context(), tc.path)
			if err == nil {
				t.Fatal("expected error")
			}
			if errors.Is(err, ErrUnsupported) != tc.unsupported {
				t.Errorf("ErrUnsupported = %v, want %v (%v)", !tc.unsupported, tc.unsupported, err)
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Errorf("error %q missing %q", err, tc.contains)
			}
		})
	}
}

func TestExtractAllCollectsErrors(t *testing.T) {
	comps, errs := ExtractAll(t.Context(), []string{
		"testdata/go.mod", "testdata/notes.md", "testdata/requirements.txt", "testdata/missing.lock",
	})
	if len(errs) != 2 || errs[0].Path != "testdata/notes.md" || !errors.Is(errs[0], ErrUnsupported) {
		t.Fatalf("errors: %+v", errs)
	}
	if len(comps) != 6 { // 3 from go.mod + 3 from requirements.txt
		t.Errorf("got %d components", len(comps))
	}
}
