package source

import "strings"

// Hunk operations.
const (
	OpEqual = "eq"
	OpAdd   = "add"
	OpDel   = "del"
	OpSkip  = "skip" // unchanged lines collapsed out of the diff
)

// Hunk is a run of lines with the same operation. For OpSkip, Skipped holds
// the number of collapsed lines and Lines is empty.
type Hunk struct {
	Op      string   `json:"op"`
	Lines   []string `json:"lines,omitempty"`
	Skipped int      `json:"skipped,omitempty"`
}

// Diff is a line-level difference with a few lines of context around changes.
type Diff struct {
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Hunks   []Hunk `json:"hunks"`
}

const (
	diffContext   = 3
	maxDiffCells  = 4_000_000 // cap on len(old)*len(new) for the LCS table
	maxLineLength = 2000
)

// ChangedChars is the number of characters in added and removed lines. It is
// what a minimum-change threshold is compared against.
func (d Diff) ChangedChars() int {
	n := 0
	for _, h := range d.Hunks {
		if h.Op == OpAdd || h.Op == OpDel {
			for _, l := range h.Lines {
				n += len(l)
			}
		}
	}
	return n
}

// LineDiff compares two texts line by line.
func LineDiff(oldText, newText string) Diff {
	return diffLines(splitLines(oldText), splitLines(newText))
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func diffLines(a, b []string) Diff {
	// Trim the common prefix and suffix; most diffs are small.
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	midA, midB := a[pre:len(a)-suf], b[pre:len(b)-suf]

	var ops []Hunk
	push := func(op string, lines []string) {
		if len(lines) == 0 {
			return
		}
		if n := len(ops); n > 0 && ops[n-1].Op == op {
			ops[n-1].Lines = append(ops[n-1].Lines, lines...)
			return
		}
		ops = append(ops, Hunk{Op: op, Lines: append([]string(nil), lines...)})
	}

	push(OpEqual, a[:pre])
	switch {
	case len(midA) == 0:
		push(OpAdd, midB)
	case len(midB) == 0:
		push(OpDel, midA)
	case len(midA)*len(midB) > maxDiffCells:
		// Too big to align cheaply: report the middle as one replacement.
		push(OpDel, midA)
		push(OpAdd, midB)
	default:
		lcsDiff(midA, midB, push)
	}
	push(OpEqual, a[len(a)-suf:])

	return finish(ops)
}

// lcsDiff aligns two line slices with a longest-common-subsequence table.
func lcsDiff(a, b []string, push func(op string, lines []string)) {
	n, m := len(a), len(b)
	w := m + 1
	t := make([]uint16, (n+1)*w)
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				t[i*w+j] = t[(i+1)*w+j+1] + 1
			} else {
				t[i*w+j] = max(t[(i+1)*w+j], t[i*w+j+1])
			}
		}
	}
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			push(OpEqual, a[i:i+1])
			i++
			j++
		case t[(i+1)*w+j] >= t[i*w+j+1]:
			push(OpDel, a[i:i+1])
			i++
		default:
			push(OpAdd, b[j:j+1])
			j++
		}
	}
	push(OpDel, a[i:])
	push(OpAdd, b[j:])
}

// finish counts changes and collapses long runs of unchanged lines to a few
// lines of context.
func finish(ops []Hunk) Diff {
	d := Diff{Hunks: make([]Hunk, 0, len(ops))}
	for idx, h := range ops {
		switch h.Op {
		case OpAdd:
			d.Added += len(h.Lines)
			d.Hunks = append(d.Hunks, h)
		case OpDel:
			d.Removed += len(h.Lines)
			d.Hunks = append(d.Hunks, h)
		default:
			d.Hunks = append(d.Hunks, collapse(h, idx == 0, idx == len(ops)-1)...)
		}
	}
	return d
}

// collapse trims an equal run to context lines. The first run keeps only its
// tail and the last only its head, since only one side touches a change.
func collapse(h Hunk, first, last bool) []Hunk {
	n := len(h.Lines)
	if first && last {
		return nil // nothing changed at all
	}
	head, tail := diffContext, diffContext
	if first {
		head = 0
	}
	if last {
		tail = 0
	}
	if n <= head+tail+1 {
		return []Hunk{h}
	}
	var out []Hunk
	if head > 0 {
		out = append(out, Hunk{Op: OpEqual, Lines: h.Lines[:head]})
	}
	out = append(out, Hunk{Op: OpSkip, Skipped: n - head - tail})
	if tail > 0 {
		out = append(out, Hunk{Op: OpEqual, Lines: h.Lines[n-tail:]})
	}
	return out
}
