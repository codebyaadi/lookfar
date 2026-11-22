package stack

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	packageurl "github.com/package-url/packageurl-go"
)

// parsePackageJSON reads the dependency maps of a package.json. It is the
// fallback when no lockfile exists, so versions are ranges, not exact versions.
// (osv-scalibr's packagejson extractor describes installed packages, not declared
// dependencies, so this small parser is used instead.)
func parsePackageJSON(data []byte) ([]Component, error) {
	var pj struct {
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if err := json.Unmarshal(data, &pj); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}
	var out []Component
	for _, deps := range []map[string]string{pj.Dependencies, pj.DevDependencies, pj.OptionalDependencies} {
		names := make([]string, 0, len(deps))
		for n := range deps {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, name := range names {
			ns, n := splitScope(name)
			out = append(out, Component{
				Ecosystem:  "npm",
				Name:       name,
				Constraint: deps[name],
				PURL:       packageurl.NewPackageURL("npm", ns, n, "", nil, "").ToString(),
				Unresolved: true,
			})
		}
	}
	return out, nil
}

// splitScope splits "@scope/name" into ("@scope", "name").
func splitScope(name string) (string, string) {
	if strings.HasPrefix(name, "@") {
		if i := strings.Index(name, "/"); i > 0 {
			return name[:i], name[i+1:]
		}
	}
	return "", name
}
