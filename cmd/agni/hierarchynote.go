package main

import (
	rpt "github.com/panyam/agni/core/report"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// hierarchyNote is the stderr line saying a read left out hierarchical blocks, or "" when it left out
// none (agni issue 707). The sentence is report.HierarchyNote, the one the HTML reports and the
// viewer's strip also show (agni issue 717); this adds only the CLI's framing.
//
// It is in the register of resolutionNote because it is the same kind of statement, about what the
// READ covered rather than about the design. On a read of the top level alone it also names the
// flag, which the shared sentence cannot, since the viewer and the reports have no flags (agni issue
// 718). A flattened read lists only cycles, which the flag cannot help with, whether the flag or a
// design.yaml asked for it.
func hierarchyNote(file string, blocks []*ir.UnexpandedHierarchy) string {
	s := rpt.HierarchyNote(file, blocks)
	if s == "" {
		return ""
	}
	if !rpt.HierarchyCycles(blocks) {
		s += " Pass --flatten-hierarchy to read every block."
	}
	return "note: " + s + "\n"
}
