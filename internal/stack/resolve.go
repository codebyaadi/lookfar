package stack

import "context"

// FileResult is the outcome of extracting one manifest.
type FileResult struct {
	Path  string
	Count int
	Err   error
}

// Resolved is the full stack: components from manifests and manual entries.
type Resolved struct {
	Components []Component
	Files      []FileResult
	// ManualErrors holds problems mapping manual components (normally caught by config validation).
	ManualErrors []error
}

// Resolve extracts every manifest and maps the manual components. It never
// stops at the first error.
func Resolve(ctx context.Context, manifests []string, manual []ManualSpec) Resolved {
	var r Resolved
	for _, p := range manifests {
		comps, err := Extract(ctx, p)
		r.Files = append(r.Files, FileResult{Path: p, Count: len(comps), Err: err})
		r.Components = append(r.Components, comps...)
	}
	seen := map[string]bool{}
	for _, m := range manual {
		c, err := FromManual(m)
		if err != nil {
			r.ManualErrors = append(r.ManualErrors, err)
			continue
		}
		if seen[c.PURL] {
			continue
		}
		seen[c.PURL] = true
		r.Components = append(r.Components, c)
	}
	return r
}
