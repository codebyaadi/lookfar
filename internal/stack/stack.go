// Package stack turns manifest files (lockfiles, SBOMs, Dockerfiles) into a
// normalized list of components identified by package URL.
package stack

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/osv-scalibr/extractor"
	"github.com/google/osv-scalibr/extractor/filesystem"
	"github.com/google/osv-scalibr/extractor/filesystem/language/golang/gomod"
	"github.com/google/osv-scalibr/extractor/filesystem/language/javascript/bunlock"
	"github.com/google/osv-scalibr/extractor/filesystem/language/javascript/packagelockjson"
	"github.com/google/osv-scalibr/extractor/filesystem/language/python/poetrylock"
	"github.com/google/osv-scalibr/extractor/filesystem/language/python/requirements"
	"github.com/google/osv-scalibr/extractor/filesystem/language/python/uvlock"
	"github.com/google/osv-scalibr/extractor/filesystem/sbom/cdx"
	"github.com/google/osv-scalibr/extractor/filesystem/sbom/spdx"
	scalibrfs "github.com/google/osv-scalibr/fs"
	packageurl "github.com/package-url/packageurl-go"
)

// Component is one piece of software found in a manifest.
type Component struct {
	// Ecosystem is the OSV-style ecosystem (npm, PyPI, Go, ...) or "oci" for container images.
	Ecosystem string
	Name      string
	// Version is the exact version, or empty when the manifest only gives a range.
	Version string
	// Constraint is the declared version range when Version is unknown (package.json).
	Constraint string
	// CPE is the CPE 2.3 string for curated server software (empty otherwise).
	CPE string
	// Kind is package, server, image or saas.
	Kind string
	// Origin is "manifest" or "manual".
	Origin string
	// PURL is the package URL, the common component identity.
	PURL string
	// Manifest is the path the component came from, as passed to Extract.
	Manifest string
	// Unresolved is true when part of the identity is unknown: no exact version,
	// or a Dockerfile ARG that could not be resolved.
	Unresolved bool
}

// ErrUnsupported is returned (wrapped) for files lookfar cannot extract from.
var ErrUnsupported = errors.New("unsupported manifest")

// FileError is a failure to extract one file.
type FileError struct {
	Path string
	Err  error
}

func (e FileError) Error() string { return e.Path + ": " + e.Err.Error() }

// Unwrap returns the underlying error.
func (e FileError) Unwrap() error { return e.Err }

// Extract returns the deduplicated components declared in one manifest file.
func Extract(ctx context.Context, path string) ([]Component, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	kind, err := detect(path, data)
	if err != nil {
		return nil, err
	}

	var comps []Component
	switch kind {
	case kindDockerfile:
		comps, err = parseDockerfile(data)
	case kindPackageJSON:
		comps, err = parsePackageJSON(data)
	default:
		comps, err = runScalibr(ctx, kind, path)
	}
	if err != nil {
		return nil, fmt.Errorf("extract %s: %w", path, err)
	}
	return finalize(comps, path), nil
}

// ExtractAll extracts every path, collecting per-file errors instead of stopping.
func ExtractAll(ctx context.Context, paths []string) ([]Component, []FileError) {
	var all []Component
	var errs []FileError
	for _, p := range paths {
		comps, err := Extract(ctx, p)
		if err != nil {
			errs = append(errs, FileError{Path: p, Err: err})
			continue
		}
		all = append(all, comps...)
	}
	return all, errs
}

type kind int

const (
	kindPackageLock kind = iota + 1
	kindPackageJSON
	kindBunLock
	kindRequirements
	kindPoetryLock
	kindUVLock
	kindGoMod
	kindCycloneDX
	kindSPDX
	kindDockerfile
)

// detect picks the parser from the file name and, for JSON SBOMs, the content.
func detect(path string, data []byte) (kind, error) {
	base := strings.ToLower(filepath.Base(path))
	switch {
	case base == "package-lock.json" || base == "npm-shrinkwrap.json":
		return kindPackageLock, nil
	case base == "package.json":
		return kindPackageJSON, nil
	case base == "bun.lock":
		return kindBunLock, nil
	case base == "bun.lockb":
		return 0, fmt.Errorf("%w: binary bun.lockb; use bun.lock (bun install --save-text-lockfile)", ErrUnsupported)
	case strings.HasSuffix(base, ".txt") && strings.Contains(base, "requirements"):
		return kindRequirements, nil
	case base == "poetry.lock":
		return kindPoetryLock, nil
	case base == "uv.lock":
		return kindUVLock, nil
	case base == "go.mod":
		return kindGoMod, nil
	case base == "dockerfile" || strings.HasPrefix(base, "dockerfile.") || strings.HasSuffix(base, ".dockerfile"):
		return kindDockerfile, nil
	case strings.HasSuffix(base, ".json"):
		head := string(data[:min(len(data), 8192)])
		switch {
		case strings.Contains(head, `"bomFormat"`) && strings.Contains(head, "CycloneDX"):
			return kindCycloneDX, nil
		case strings.Contains(head, `"spdxVersion"`):
			return kindSPDX, nil
		}
	}
	return 0, fmt.Errorf("%w: %s", ErrUnsupported, filepath.Base(path))
}

func newExtractor(k kind) (filesystem.Extractor, error) {
	switch k {
	case kindPackageLock:
		return packagelockjson.New(nil)
	case kindBunLock:
		return bunlock.New(nil)
	case kindRequirements:
		return requirements.New(nil)
	case kindPoetryLock:
		return poetrylock.New(nil)
	case kindUVLock:
		return uvlock.New(nil)
	case kindGoMod:
		return gomod.New(nil)
	case kindCycloneDX:
		return cdx.New(nil)
	case kindSPDX:
		return spdx.New(nil)
	}
	return nil, fmt.Errorf("no extractor for kind %d", k)
}

// runScalibr runs one osv-scalibr extractor directly on the file (no directory walk).
func runScalibr(ctx context.Context, k kind, path string) ([]Component, error) {
	ex, err := newExtractor(k)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	dir, name := filepath.Split(abs)
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	inv, err := ex.Extract(ctx, &filesystem.ScanInput{
		FS:     scalibrfs.DirFS(dir),
		Path:   name,
		Root:   dir,
		Info:   info,
		Reader: f,
	})
	if err != nil {
		return nil, err
	}
	return fromPackages(inv.Packages), nil
}

func fromPackages(pkgs []*extractor.Package) []Component {
	var out []Component
	for _, p := range pkgs {
		u := p.PURL()
		if u == nil || p.Name == "" {
			continue
		}
		out = append(out, Component{
			Ecosystem:  p.Ecosystem().String(),
			Name:       p.Name,
			Version:    p.Version,
			PURL:       cleanPURL(u.String()),
			Unresolved: p.Version == "",
		})
	}
	return out
}

// finalize stamps the manifest path, dedupes by purl and sorts for stable output.
func finalize(comps []Component, manifest string) []Component {
	seen := make(map[string]bool, len(comps))
	out := make([]Component, 0, len(comps))
	for _, c := range comps {
		c.Manifest = manifest
		c.Origin = OriginManifest
		if c.Kind == "" {
			c.Kind = imageOrPackage(c)
		}
		key := c.PURL
		if c.PURL == "" || c.Unresolved && c.Version == "" {
			key = c.PURL + "|" + c.Constraint
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PURL < out[j].PURL })
	return out
}

// cleanPURL drops scalibr-specific qualifiers (such as source=PUBLIC_REGISTRY)
// so the purl is a stable identity for the same package.
func cleanPURL(s string) string {
	p, err := packageurl.FromString(s)
	if err != nil {
		return s
	}
	kept := p.Qualifiers[:0]
	for _, q := range p.Qualifiers {
		if q.Key != "source" {
			kept = append(kept, q)
		}
	}
	p.Qualifiers = kept
	return p.ToString()
}

func imageOrPackage(c Component) string {
	if c.Ecosystem == "oci" {
		return KindImage
	}
	return KindPackage
}
