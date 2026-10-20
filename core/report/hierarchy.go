package report

import (
	"cmp"
	"fmt"
	"path"
	"slices"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// hierarchyNoteListed caps how many blocks the note names. A real board can hold dozens of sub-cells,
// and the totals already say how much is missing.
const hierarchyNoteListed = 5

// HierarchyNote is the sentence saying a read left out hierarchical blocks, or "" when it left out
// none (agni issues 707, 717). file names the design, and only its last path element is printed, so
// a mount URI and a host path read alike.
//
// It is ONE sentence for every surface that says it: the CLI's stderr note, the check and checklist
// HTML reports, and the viewer's canvas strip, which GetDesignResponse.hierarchy_note carries. Each
// surface frames it (the CLI prefixes "note: ") and none rewords it, so a reader who has seen it in
// one place recognises it in the next.
//
// It takes the diagnostic rather than the design, so it reads nothing else. A hierarchical design is
// not a defect, so this is never a finding. The largest blocks are named first, because that is where
// most of the missing design is.
//
// A flattened read lists only the blocks that place themselves (UNEXPANDED_REASON_CYCLE), and saying
// "only its top cell was read" there would be false, so those get a sentence of their own (agni issue
// 1102).
func HierarchyNote(file string, blocks []*ir.UnexpandedHierarchy) string {
	if len(blocks) == 0 {
		return ""
	}
	named := hierarchyNamed(blocks)
	noun := HierarchyNoun(blocks)
	if HierarchyCycles(blocks) {
		verb, its := "each places itself", "their"
		if len(blocks) == 1 {
			verb, its = "it places itself", "its"
		}
		return fmt.Sprintf("%s is hierarchical and %s could not be expanded, because %s (%s). Counts and checks leave %s contents out.",
			path.Base(file), HierarchySummary(blocks), verb, named, its)
	}
	verb := "were"
	if len(blocks) == 1 {
		verb = "was"
	}
	return fmt.Sprintf("%s is hierarchical and only its top %s was read. %s %s not extracted (%s). Counts and checks cover the top %s alone.",
		path.Base(file), noun, HierarchySummary(blocks), verb, named, noun)
}

// HierarchyCycles reports whether every listed block was left out because it places itself, which is
// all a flattened read leaves out.
func HierarchyCycles(blocks []*ir.UnexpandedHierarchy) bool {
	for _, b := range blocks {
		if b.GetReason() != ir.UnexpandedReason_UNEXPANDED_REASON_CYCLE {
			return false
		}
	}
	return len(blocks) > 0
}

// hierarchyNamed is the note's parenthetical, the largest blocks first with their instance counts.
func hierarchyNamed(blocks []*ir.UnexpandedHierarchy) string {
	sorted := slices.Clone(blocks)
	slices.SortStableFunc(sorted, func(a, b *ir.UnexpandedHierarchy) int {
		return cmp.Or(cmp.Compare(b.GetInstanceCount(), a.GetInstanceCount()), cmp.Compare(a.GetName(), b.GetName()))
	})
	var named []string
	for _, b := range sorted[:min(len(sorted), hierarchyNoteListed)] {
		named = append(named, fmt.Sprintf("%s: %d", b.GetName(), b.GetInstanceCount()))
	}
	if extra := len(sorted) - hierarchyNoteListed; extra > 0 {
		named = append(named, fmt.Sprintf("%d more", extra))
	}
	return strings.Join(named, ", ")
}

// HierarchyNoun names the construct the way its format's users do, falling back to "block".
func HierarchyNoun(blocks []*ir.UnexpandedHierarchy) string {
	if len(blocks) > 0 && blocks[0].GetKind() == "edif_cell" {
		return "cell"
	}
	return "block"
}

// HierarchySummary is "2 sub-cells holding 13 instances", the count half of the note and of the
// `agni stats` line.
func HierarchySummary(blocks []*ir.UnexpandedHierarchy) string {
	noun := HierarchyNoun(blocks)
	instances := 0
	for _, b := range blocks {
		instances += int(b.GetInstanceCount())
	}
	cells := fmt.Sprintf("%d sub-%ss", len(blocks), noun)
	if len(blocks) == 1 {
		cells = "1 sub-" + noun
	}
	held := fmt.Sprintf("%d instances", instances)
	if instances == 1 {
		held = "1 instance"
	}
	return cells + " holding " + held
}
