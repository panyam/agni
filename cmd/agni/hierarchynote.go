package main

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

// hierarchyNote is the stderr line saying a read left out hierarchical blocks, or "" when it left out
// none (agni issue 707). It takes the diagnostic rather than the design, so it reads nothing else.
//
// It is in the register of resolutionNote because it is the same kind of statement, about what the
// READ covered rather than about the design. A hierarchical design is not a defect, so this is never a
// finding. The largest blocks are named first, because that is where most of the missing design is.
func hierarchyNote(file string, blocks []*ir.UnexpandedHierarchy) string {
	if len(blocks) == 0 {
		return ""
	}
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
	noun := hierarchyNoun(blocks)
	n := len(blocks)
	return fmt.Sprintf("note: %s is hierarchical and only its top %s was read. %s %s not extracted (%s). Counts and checks cover the top %s alone.\n",
		path.Base(file), noun, hierarchySummary(blocks), plural(n, "was", "were"), strings.Join(named, ", "), noun)
}

// hierarchyNoun names the construct the way its format's users do, falling back to "block".
func hierarchyNoun(blocks []*ir.UnexpandedHierarchy) string {
	if len(blocks) > 0 && blocks[0].GetKind() == "edif_cell" {
		return "cell"
	}
	return "block"
}

// hierarchySummary is "2 sub-cells holding 13 instances", the count half of the note and of the
// stats line.
func hierarchySummary(blocks []*ir.UnexpandedHierarchy) string {
	noun := hierarchyNoun(blocks)
	instances := 0
	for _, b := range blocks {
		instances += int(b.GetInstanceCount())
	}
	return plural(len(blocks), "1 sub-"+noun, fmt.Sprintf("%d sub-%ss", len(blocks), noun)) + " holding " +
		plural(instances, "1 instance", fmt.Sprintf("%d instances", instances))
}
