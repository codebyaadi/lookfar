// Package version compares software versions and evaluates NVD CPE matches.
// Package ecosystems are matched by OSV on its side; this package covers only
// CPE version ranges (NVD) and, later, container image tags.
package version

import (
	"strconv"
	"strings"
	"unicode"
)

// token is one run of digits or letters in a version string.
type token struct {
	num   bool
	n     uint64
	alpha string
}

// preRelease words sort before the plain release ("1.0.0-beta1" < "1.0.0"),
// while other trailing letters sort after it ("1.0.2k" > "1.0.2", "8.9p1" > "8.9").
var preRelease = map[string]bool{
	"a": true, "alpha": true, "b": true, "beta": true, "rc": true, "pre": true,
	"preview": true, "dev": true, "snapshot": true, "m": true, "milestone": true, "cr": true,
}

func tokenize(s string) []token {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "v")
	var out []token
	for i := 0; i < len(s); {
		r := rune(s[i])
		switch {
		case unicode.IsDigit(r):
			j := i
			for j < len(s) && unicode.IsDigit(rune(s[j])) {
				j++
			}
			n, err := strconv.ParseUint(s[i:j], 10, 64)
			if err != nil {
				n = ^uint64(0) // absurdly long digit run: treat as maximal
			}
			out = append(out, token{num: true, n: n})
			i = j
		case unicode.IsLetter(r):
			j := i
			for j < len(s) && unicode.IsLetter(rune(s[j])) {
				j++
			}
			out = append(out, token{alpha: s[i:j]})
			i = j
		default:
			i++ // separators: . - _ + : and anything else
		}
	}
	return out
}

// Comparable reports whether s looks like a version we can order: it must
// start with a number (so "r22", "Mainline" and "latest" are not comparable).
func Comparable(s string) bool {
	t := tokenize(s)
	return len(t) > 0 && t[0].num
}

// Compare orders two versions: -1, 0 or +1. ok is false if either version is
// not comparable (see Comparable); the result is then meaningless.
func Compare(a, b string) (cmp int, ok bool) {
	if !Comparable(a) || !Comparable(b) {
		return 0, false
	}
	ta, tb := tokenize(a), tokenize(b)
	for i := 0; i < max(len(ta), len(tb)); i++ {
		switch {
		case i >= len(ta):
			return -sideOfExtra(tb[i]), true
		case i >= len(tb):
			return sideOfExtra(ta[i]), true
		}
		if c := compareToken(ta[i], tb[i]); c != 0 {
			return c, true
		}
	}
	return 0, true
}

// sideOfExtra says how a version that has an extra token compares with one
// that ends: +1 if it is greater, -1 if smaller, 0 if the extra token is a zero.
func sideOfExtra(t token) int {
	switch {
	case t.num && t.n == 0:
		return 0 // 1.0 == 1.0.0
	case t.num:
		return 1
	case preRelease[t.alpha]:
		return -1
	default:
		return 1
	}
}

func compareToken(a, b token) int {
	switch {
	case a.num && b.num:
		return cmpUint(a.n, b.n)
	case !a.num && !b.num:
		return strings.Compare(a.alpha, b.alpha)
	case a.num: // number vs letters: 1.0.1 > 1.0.rc1, but 1.0.1 > 1.0.k too
		return 1
	default:
		return -1
	}
}

func cmpUint(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// Range is a version interval as NVD describes it. Empty bounds are open.
type Range struct {
	StartIncluding string
	StartExcluding string
	EndIncluding   string
	EndExcluding   string
}

// IsZero reports whether the range has no bounds at all.
func (r Range) IsZero() bool { return r == Range{} }

// Contains reports whether v is inside the range. ok is false when v or one
// of the bounds is not comparable, in which case callers should treat the
// range as not matching.
func (r Range) Contains(v string) (in bool, ok bool) {
	check := func(bound string, want func(int) bool) (bool, bool) {
		if bound == "" {
			return true, true
		}
		c, ok := Compare(v, bound)
		if !ok {
			return false, false
		}
		return want(c), true
	}
	for _, step := range []struct {
		bound string
		want  func(int) bool
	}{
		{r.StartIncluding, func(c int) bool { return c >= 0 }},
		{r.StartExcluding, func(c int) bool { return c > 0 }},
		{r.EndIncluding, func(c int) bool { return c <= 0 }},
		{r.EndExcluding, func(c int) bool { return c < 0 }},
	} {
		inside, ok := check(step.bound, step.want)
		if !ok {
			return false, false
		}
		if !inside {
			return false, true
		}
	}
	return true, true
}
