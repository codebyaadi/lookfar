package source

import (
	"fmt"
	"strings"
	"testing"
)

func lines(n int, prefix string) string {
	var b []string
	for i := range n {
		b = append(b, fmt.Sprintf("%s %d", prefix, i))
	}
	return strings.Join(b, "\n")
}

func TestLineDiffIdentical(t *testing.T) {
	d := LineDiff("a\nb\nc", "a\nb\nc")
	if d.Added != 0 || d.Removed != 0 || len(d.Hunks) != 0 {
		t.Fatalf("identical text should produce an empty diff, got %+v", d)
	}
}

func TestLineDiffReplaceAddRemove(t *testing.T) {
	d := LineDiff("Pro plan $49\nTeam plan $99", "Pro plan $59\nTeam plan $99\nEnterprise: contact us")
	if d.Added != 2 || d.Removed != 1 {
		t.Fatalf("added=%d removed=%d, want 2 and 1: %+v", d.Added, d.Removed, d.Hunks)
	}
	var del, add []string
	for _, h := range d.Hunks {
		switch h.Op {
		case OpDel:
			del = append(del, h.Lines...)
		case OpAdd:
			add = append(add, h.Lines...)
		}
	}
	if len(del) != 1 || del[0] != "Pro plan $49" {
		t.Errorf("deleted = %v", del)
	}
	if len(add) != 2 || add[0] != "Pro plan $59" || add[1] != "Enterprise: contact us" {
		t.Errorf("added = %v", add)
	}
	want := len("Pro plan $49") + len("Pro plan $59") + len("Enterprise: contact us")
	if got := d.ChangedChars(); got != want {
		t.Errorf("ChangedChars = %d, want %d", got, want)
	}
}

func TestLineDiffFromEmpty(t *testing.T) {
	d := LineDiff("", "one\ntwo")
	if d.Added != 2 || d.Removed != 0 {
		t.Fatalf("%+v", d)
	}
}

func TestLineDiffCollapsesContext(t *testing.T) {
	old := lines(100, "line")
	updated := strings.Replace(old, "line 50", "CHANGED", 1)
	d := LineDiff(old, updated)
	if d.Added != 1 || d.Removed != 1 {
		t.Fatalf("%+v", d)
	}
	var skips, eqLines int
	for _, h := range d.Hunks {
		switch h.Op {
		case OpSkip:
			skips++
			if h.Skipped < 40 {
				t.Errorf("expected a large skip, got %d", h.Skipped)
			}
		case OpEqual:
			eqLines += len(h.Lines)
		}
	}
	if skips != 2 {
		t.Errorf("want a skip on each side of the change, got %d", skips)
	}
	if eqLines != 2*diffContext {
		t.Errorf("want %d context lines, got %d", 2*diffContext, eqLines)
	}
}

func TestLineDiffMovedLineStaysSmall(t *testing.T) {
	d := LineDiff("a\nb\nc\nd\ne", "a\nc\nd\nb\ne")
	if d.Added+d.Removed > 2 {
		t.Fatalf("moving one line should cost one add and one delete at most: %+v", d)
	}
}

func TestLineDiffHugeFallsBackToReplace(t *testing.T) {
	d := LineDiff(lines(2500, "old"), lines(2500, "new"))
	if d.Added != 2500 || d.Removed != 2500 {
		t.Fatalf("added=%d removed=%d", d.Added, d.Removed)
	}
}
