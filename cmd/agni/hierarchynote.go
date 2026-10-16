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
// READ covered rather than about the design.
func hierarchyNote(file string, blocks []*ir.UnexpandedHierarchy) string {
	s := rpt.HierarchyNote(file, blocks)
	if s == "" {
		return ""
	}
	return "note: " + s + "\n"
}
