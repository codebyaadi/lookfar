package config

import (
	"fmt"
	"strings"

	"github.com/codebyaadi/lookfar/internal/stack"
)

type pathError struct{ path, msg string }

// checkComponents applies the cross-field rules for manual components and
// fills in an inferred kind (server) for names in the curated list.
func checkComponents(cfg *Config) []pathError {
	var errs []pathError
	for i := range cfg.Stack.Components {
		c := &cfg.Stack.Components[i]
		base := fmt.Sprintf("stack.components[%d]", i)

		kind, ok := stack.InferKind(c.Name, c.Kind)
		if !ok {
			errs = append(errs, pathError{base + ".kind", fmt.Sprintf(
				"%q is not known server software, so kind is required: package, server, image or saas", c.Name)})
			continue
		}
		c.Kind = kind

		switch kind {
		case stack.KindPackage:
			if c.Ecosystem == "" {
				errs = append(errs, pathError{base + ".ecosystem", "is required for kind package: npm, PyPI or Go"})
			}
		case stack.KindServer:
			if _, ok := stack.LookupServer(c.Name); !ok {
				errs = append(errs, pathError{base + ".name", fmt.Sprintf(
					"unknown server software %q (known: %s)", c.Name, strings.Join(stack.ServerNames(), ", "))})
			}
		}
	}
	return errs
}
