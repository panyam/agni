package report

import (
	"bytes"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestHierarchyNoteIsOneSentenceForEveryFormat pins the shared wording (agni issue 717). A construct
// with no format-specific name reads as a block, and the totals count every block whatever the cap
// names.
func TestHierarchyNoteIsOneSentenceForEveryFormat(t *testing.T) {
	blocks := []*ir.UnexpandedHierarchy{
		{Name: "amp", Kind: "subcircuit", InstanceCount: 4},
		{Name: "bias", Kind: "subcircuit", InstanceCount: 6},
	}
	got := HierarchyNote("/some/dir/top.sch", blocks)
	want := "top.sch is hierarchical and only its top block was read. 2 sub-blocks holding 10 instances were not extracted (bias: 6, amp: 4). Counts and checks cover the top block alone."
	if got != want {
		t.Errorf("note:\n got %q\nwant %q", got, want)
	}
	if got := HierarchyNote("top.sch", nil); got != "" {
		t.Errorf("a read that left nothing out got a note: %q", got)
	}
}

// TestBothPagesStateTheReadLimitAboveTheCounts holds the check report and the checklist page to
// rendering the note when there is one and nothing when there is none.
func TestBothPagesStateTheReadLimitAboveTheCounts(t *testing.T) {
	const note = "hier.edn is hierarchical and only its top cell was read."
	render := map[string]func(string) string{
		"check": func(n string) string {
			var b bytes.Buffer
			if err := HTML(&b, Report{Design: "hier.edn", HierarchyNote: n}); err != nil {
				t.Fatal(err)
			}
			return b.String()
		},
		"checklist": func(n string) string {
			var b bytes.Buffer
			if err := ChecklistHTML(&b, Checklist{Design: "hier.edn", Name: "Review", HierarchyNote: n}); err != nil {
				t.Fatal(err)
			}
			return b.String()
		},
	}
	for name, r := range render {
		page := r(note)
		at := strings.Index(page, `<div class="read-limit">`+note+`</div>`)
		if at < 0 {
			t.Errorf("%s page does not carry the note", name)
		} else if at > strings.Index(page, `<div class="stats">`) {
			t.Errorf("%s page puts the note after the counts it qualifies", name)
		}
		if strings.Contains(r(""), `<div class="read-limit">`) {
			t.Errorf("%s page renders a read-limit band with no note", name)
		}
	}
}

// TestCycleNoteSaysWhatTheWalkLeftOut covers agni issue 1102: a flattened read lists only blocks that
// place themselves, and the top-level sentence would claim a partial read that did not happen.
func TestCycleNoteSaysWhatTheWalkLeftOut(t *testing.T) {
	cycle := ir.UnexpandedReason_UNEXPANDED_REASON_CYCLE
	one := []*ir.UnexpandedHierarchy{{Name: "LOOP", Kind: "edif_cell", InstanceCount: 2, Reason: cycle}}
	if got, want := HierarchyNote("mount://m/hier_cycle.edn", one), "hier_cycle.edn is hierarchical and 1 sub-cell holding 2 instances could not be expanded, because it places itself (LOOP: 2). Counts and checks leave its contents out."; got != want {
		t.Errorf("one cycle:\n got %q\nwant %q", got, want)
	}
	two := []*ir.UnexpandedHierarchy{{Name: "A", Kind: "edif_cell", InstanceCount: 1, Reason: cycle}, {Name: "B", Kind: "edif_cell", InstanceCount: 3, Reason: cycle}}
	if got, want := HierarchyNote("x.edn", two), "x.edn is hierarchical and 2 sub-cells holding 4 instances could not be expanded, because each places itself (B: 3, A: 1). Counts and checks leave their contents out."; got != want {
		t.Errorf("two cycles:\n got %q\nwant %q", got, want)
	}
	for _, r := range []ir.UnexpandedReason{ir.UnexpandedReason_UNEXPANDED_REASON_UNSPECIFIED, ir.UnexpandedReason_UNEXPANDED_REASON_NOT_WALKED} {
		top := []*ir.UnexpandedHierarchy{{Name: "SUB", Kind: "edif_cell", InstanceCount: 1, Reason: r}}
		if got, want := HierarchyNote("hier.edn", top), "hier.edn is hierarchical and only its top cell was read. 1 sub-cell holding 1 instance was not extracted (SUB: 1). Counts and checks cover the top cell alone."; got != want {
			t.Errorf("reason %v:\n got %q\nwant %q", r, got, want)
		}
	}
}
