package stack

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"

	packageurl "github.com/package-url/packageurl-go"
)

var (
	argRef      = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::?-([^}]*))?\}|\$([A-Za-z_][A-Za-z0-9_]*)`)
	defaultHost = map[string]bool{"docker.io": true, "index.docker.io": true, "registry-1.docker.io": true}
)

// parseDockerfile turns each FROM line into an image component. References to
// earlier build stages and "scratch" are skipped. Global ARGs declared before
// the first FROM are substituted; anything unresolved is kept literally and
// the component is flagged Unresolved.
func parseDockerfile(data []byte) ([]Component, error) {
	args := map[string]string{}
	stages := map[string]bool{}
	var out []Component
	seenFrom := false

	for _, line := range logicalLines(data) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "ARG":
			if !seenFrom {
				for _, a := range fields[1:] {
					name, val, hasVal := strings.Cut(a, "=")
					if hasVal {
						args[name] = strings.Trim(val, `"'`)
					} else if _, ok := args[name]; !ok {
						args[name] = "\x00unset" // declared without default
					}
				}
			}
		case "FROM":
			seenFrom = true
			rest := fields[1:]
			for len(rest) > 0 && strings.HasPrefix(rest[0], "--") {
				rest = rest[1:] // --platform=... and friends
			}
			if len(rest) == 0 {
				continue
			}
			ref := rest[0]
			stageName := ""
			if len(rest) >= 3 && strings.EqualFold(rest[1], "AS") {
				stageName = strings.ToLower(rest[2])
			}
			isStage := stages[strings.ToLower(ref)]
			if stageName != "" {
				stages[stageName] = true
			}
			if isStage || strings.EqualFold(ref, "scratch") {
				continue
			}
			resolved, ok := substitute(ref, args)
			out = append(out, imageComponent(resolved, !ok))
		}
	}
	return out, nil
}

// substitute expands $VAR, ${VAR} and ${VAR:-default}. ok is false if any
// reference stayed unresolved (it is left in the string unchanged).
func substitute(s string, args map[string]string) (string, bool) {
	ok := true
	out := argRef.ReplaceAllStringFunc(s, func(m string) string {
		sub := argRef.FindStringSubmatch(m)
		name, def := sub[1], sub[2]
		if name == "" {
			name = sub[3]
		}
		if v, found := args[name]; found && v != "\x00unset" {
			return v
		}
		if strings.Contains(m, "-") && def != "" {
			return def
		}
		ok = false
		return m
	})
	return out, ok
}

// imageComponent builds an oci component from an image reference such as
// ghcr.io/org/app:1.2@sha256:...
func imageComponent(ref string, unresolved bool) Component {
	name, digest, _ := strings.Cut(ref, "@")
	tag := ""
	// A colon after the last slash separates the tag (earlier colons are registry ports).
	if i := strings.LastIndex(name, ":"); i > strings.LastIndex(name, "/") {
		name, tag = name[:i], name[i+1:]
	}
	registry := ""
	if i := strings.Index(name, "/"); i > 0 {
		if first := name[:i]; strings.ContainsAny(first, ".:") || first == "localhost" {
			registry, name = first, name[i+1:]
		}
	}
	if defaultHost[registry] {
		registry = ""
	}
	version := tag
	if version == "" && digest == "" && !unresolved {
		version = "latest"
	}

	var quals packageurl.Qualifiers
	if registry != "" {
		quals = append(quals, packageurl.Qualifier{Key: "repository_url", Value: registry})
	}
	if digest != "" {
		quals = append(quals, packageurl.Qualifier{Key: "digest", Value: digest})
	}
	ns, n := "", name
	if i := strings.LastIndex(name, "/"); i >= 0 {
		ns, n = name[:i], name[i+1:]
	}
	purl := packageurl.NewPackageURL("oci", ns, n, version, quals, "").ToString()
	return Component{Ecosystem: "oci", Name: name, Version: version, PURL: purl, Unresolved: unresolved}
}

// logicalLines joins backslash continuations and drops comments and blank lines.
func logicalLines(data []byte) []string {
	var lines []string
	var cur strings.Builder
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		t := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasSuffix(t, "\\") {
			cur.WriteString(strings.TrimSuffix(t, "\\") + " ")
			continue
		}
		cur.WriteString(t)
		if s := strings.TrimSpace(cur.String()); s != "" {
			lines = append(lines, s)
		}
		cur.Reset()
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		lines = append(lines, s)
	}
	return lines
}
