package stack

import (
	"fmt"
	"strings"

	packageurl "github.com/package-url/packageurl-go"
)

// Kinds of manually declared components.
const (
	KindPackage = "package"
	KindServer  = "server"
	KindImage   = "image"
	KindSaaS    = "saas"
)

// Origins of a component.
const (
	OriginManifest = "manifest"
	OriginManual   = "manual"
)

// ManualSpec is a component declared by hand in lookfar.yaml.
type ManualSpec struct {
	Name      string
	Version   string
	Kind      string // package, server, image or saas; empty infers server for curated names
	Ecosystem string // npm, PyPI or Go; required for kind package
}

// InferKind returns the kind to use: the declared one, or server when the name
// is in the curated list. ok is false when the kind cannot be inferred.
func InferKind(name, kind string) (string, bool) {
	if kind != "" {
		return kind, true
	}
	if _, ok := LookupServer(name); ok {
		return KindServer, true
	}
	return "", false
}

// FromManual maps a manually declared component to a Component.
func FromManual(m ManualSpec) (Component, error) {
	kind, ok := InferKind(m.Name, m.Kind)
	if !ok {
		return Component{}, fmt.Errorf("component %q needs a kind (package, server, image or saas)", m.Name)
	}
	c := Component{Name: m.Name, Version: m.Version, Origin: OriginManual, Kind: kind}

	switch kind {
	case KindServer:
		s, ok := LookupServer(m.Name)
		if !ok {
			return Component{}, fmt.Errorf("unknown server software %q (known: %s)", m.Name, strings.Join(ServerNames(), ", "))
		}
		c.Ecosystem, c.Name, c.CPE = "generic", s.Name, s.CPE(m.Version)
		c.PURL = packageurl.NewPackageURL("generic", "", s.Name, m.Version, nil, "").ToString()

	case KindSaaS:
		c.Ecosystem = "generic"
		c.PURL = packageurl.NewPackageURL("generic", "", m.Name, m.Version, nil, "").ToString()

	case KindImage:
		ref := m.Name
		if m.Version != "" {
			ref += ":" + m.Version
		}
		img := imageComponent(ref, false)
		c.Ecosystem, c.Name, c.Version, c.PURL = img.Ecosystem, img.Name, img.Version, img.PURL

	case KindPackage:
		eco, purlType, ns, name, err := packageIdentity(m.Ecosystem, m.Name)
		if err != nil {
			return Component{}, err
		}
		c.Ecosystem = eco
		if purlType == "pypi" {
			c.Name = name // normalized form
		}
		c.PURL = packageurl.NewPackageURL(purlType, ns, name, m.Version, nil, "").ToString()

	default:
		return Component{}, fmt.Errorf("component %q: unknown kind %q", m.Name, kind)
	}
	c.Unresolved = c.Version == "" && kind != KindSaaS
	return c, nil
}

// packageIdentity maps an ecosystem and package name to purl parts.
func packageIdentity(ecosystem, name string) (eco, purlType, namespace, pkg string, err error) {
	switch ecosystem {
	case "npm":
		ns, n := splitScope(name)
		return "npm", "npm", ns, n, nil
	case "PyPI":
		n := strings.ReplaceAll(strings.ToLower(name), "_", "-")
		return "PyPI", "pypi", "", n, nil
	case "Go":
		if i := strings.LastIndex(name, "/"); i > 0 {
			return "Go", "golang", name[:i], name[i+1:], nil
		}
		return "Go", "golang", "", name, nil
	case "":
		return "", "", "", "", fmt.Errorf("package %q needs an ecosystem (npm, PyPI or Go)", name)
	}
	return "", "", "", "", fmt.Errorf("package %q: unsupported ecosystem %q (use npm, PyPI or Go)", name, ecosystem)
}
