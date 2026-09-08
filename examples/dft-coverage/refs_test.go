package main

import (
	"strings"
	"testing"
)

func refRows(n int) [][]string {
	out := make([][]string, n)
	for i := range out {
		out[i] = []string{string(rune('A'+i%26)) + string(rune('0'+i/26))}
	}
	return out
}

// TestRefsCapsALongList: pointed at a real board these buckets run to several hundred parts, and an
// uncapped list buries the lines around it. The count stays exact; only the naming is trimmed.
func TestRefsCapsALongList(t *testing.T) {
	got := refs(refRows(400))
	if n := strings.Count(got, ",") + 1 - 1; n > refsCap+1 {
		t.Errorf("named %d refs, want at most %d plus the summary", n, refsCap)
	}
	if !strings.Contains(got, "(388 more)") {
		t.Errorf("summary does not say how many were withheld: %q", got[max(0, len(got)-40):])
	}
}

// TestRefsLeavesAShortListWhole: the bundled fixture's buckets are small enough to name in full, and
// truncating those would lose the detail the walkthrough is built on.
func TestRefsLeavesAShortListWhole(t *testing.T) {
	got := refs([][]string{{"R2"}, {"C1"}, {"C2"}})
	if got != "C1, C2, R2" {
		t.Errorf("refs = %q, want the full sorted list", got)
	}
}
