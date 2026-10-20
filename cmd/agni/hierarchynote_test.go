package main

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/formats"
)

const (
	hierEDN = "../../readers/edif/testdata/hier.edn"
	flatEDN = "../../readers/edif/testdata/basic.edn"
)

// TestStatsSaysWhatAHierarchicalReadLeftOut exists because stats is what someone runs when a count
// looks wrong, and on a hierarchical .edn its counts are the top cell's alone (agni issue 707). The
// flat design is the control, so a line printed unconditionally fails too.
func TestStatsSaysWhatAHierarchicalReadLeftOut(t *testing.T) {
	out := runCLI(t, statsCmd(), hierEDN)
	if !regexp.MustCompile(`not extracted:\s+1 sub-cell holding 1 instance \(counts above are the top cell only\)`).MatchString(out) {
		t.Errorf("stats on a hierarchical design should say what the read left out:\n%s", out)
	}
	if out := runCLI(t, statsCmd(), flatEDN); strings.Contains(out, "not extracted") {
		t.Errorf("stats on a flat design claims something was left out:\n%s", out)
	}
}

// TestStatsFlattensAHierarchicalEDIF is the stats golden for --flatten-hierarchy (agni issue 718):
// hier_blocks.edn places LED_DRV once and RC twice, and U5 has a gate in TOP and one in LED_DRV.
func TestStatsFlattensAHierarchicalEDIF(t *testing.T) {
	const blocks = "../../readers/edif/testdata/hier_blocks.edn"
	if out := runCLI(t, statsCmd(), blocks); !strings.Contains(out, "not extracted:") {
		t.Errorf("unflattened stats should say what it left out:\n%s", out)
	}
	readFlatten = true
	t.Cleanup(func() { readFlatten = false })
	out := runCLI(t, statsCmd(), blocks)
	for _, want := range []string{
		`components:\s+8 \(unique ref_des\)`,
		`sections:\s+9 \(source instances\)`,
		`multi-section:\s+1 \(`,
		`nets:\s+6\n`,
	} {
		if !regexp.MustCompile(want).MatchString(out) {
			t.Errorf("flattened stats should match %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "not extracted") {
		t.Errorf("flattened stats still reports a partial read:\n%s", out)
	}
}

// TestHierarchyNoteNamesTheFlag holds the CLI's note to naming --flatten-hierarchy while the flag is
// off, and to dropping it once on, where a block still listed was kept out by a cycle.
func TestHierarchyNoteNamesTheFlag(t *testing.T) {
	blocks := []*ir.UnexpandedHierarchy{{Name: "SUB", Kind: "edif_cell", InstanceCount: 1}}
	if got := hierarchyNote("board.edn", blocks); !strings.Contains(got, "Pass --flatten-hierarchy to read every block.") {
		t.Errorf("note without the flag = %q, want it to name the flag", got)
	}
	readFlatten = true
	t.Cleanup(func() { readFlatten = false })
	if got := hierarchyNote("board.edn", blocks); strings.Contains(got, "--flatten-hierarchy") {
		t.Errorf("note with the flag on = %q, want no hint", got)
	}
}

// TestLocalLoaderNotesAHierarchicalRead covers the service-backed commands (check, query, review,
// trace), which read through localLoader rather than readDesign. One command asks for the netlist
// more than once, so the note is written once per file.
func TestLocalLoaderNotesAHierarchicalRead(t *testing.T) {
	var notes bytes.Buffer
	l := &localLoader{loader: &formats.Loader{}, notes: &notes}
	ctx := context.Background()
	for range 2 {
		if _, err := l.Design(ctx, uriOf(t, hierEDN)); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Count(notes.String(), "hier.edn is hierarchical"); got != 1 {
		t.Fatalf("hierarchy note written %d times, want once:\n%s", got, notes.String())
	}
	if !strings.Contains(notes.String(), "(SUB: 1)") {
		t.Errorf("note should name the cell it left out and its instance count:\n%s", notes.String())
	}

	notes.Reset()
	if _, err := l.Design(ctx, uriOf(t, flatEDN)); err != nil {
		t.Fatal(err)
	}
	if notes.Len() != 0 {
		t.Errorf("a flat design wrote a note:\n%s", notes.String())
	}
}

// TestHierarchyNoteNamesTheLargestFirst checks the cap order. The largest blocks hold most of what
// is missing, so they are the ones named when the list is capped, and the totals still count every
// block.
func TestHierarchyNoteNamesTheLargestFirst(t *testing.T) {
	var blocks []*ir.UnexpandedHierarchy
	for i, n := range []int32{3, 40, 1, 7, 12, 2, 9} {
		blocks = append(blocks, &ir.UnexpandedHierarchy{Name: string(rune('A' + i)), Kind: "edif_cell", InstanceCount: n})
	}
	got := hierarchyNote("board.edn", blocks)
	want := "note: board.edn is hierarchical and only its top cell was read. 7 sub-cells holding 74 instances were not extracted (B: 40, E: 12, G: 9, D: 7, A: 3, 2 more). Counts and checks cover the top cell alone. Pass --flatten-hierarchy to read every block.\n"
	if got != want {
		t.Errorf("note:\n got %q\nwant %q", got, want)
	}
	if got := hierarchyNote("flat.edn", nil); got != "" {
		t.Errorf("a read that left nothing out got a note: %q", got)
	}
}
