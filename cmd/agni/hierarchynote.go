package main

import (
	"fmt"
	"io"

	rpt "github.com/panyam/agni/core/report"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// hierarchyNote is the stderr line saying a read left out hierarchical blocks, or "" when it left out
// none (agni issue 707). The sentence comes from report.HierarchyNote, and the HTML reports and the
// viewer's strip show the same sentence (agni issue 717). This function adds only the CLI's framing.
//
// It is in the register of resolutionNote because it is the same kind of statement, about what the
// READ covered rather than about the design. On a read of the top level alone it also names the
// flag. The shared sentence cannot name it, since the viewer and the reports have no flags (agni
// issue 718). A flattened read lists only cycles, whether the flag or a design.yaml asked for the
// flattening. The flag cannot help with a cycle.
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

// noteRemoteHierarchy writes hierarchyNote for a run a server answered, naming the design as the
// command named it. A local run reads the design through localLoader, which writes the note as it
// reads, so this writes nothing unless remote is set. A server's result carries the blocks its read
// left out, so a remote run warns as a local one does (agni issue 1887).
func noteRemoteHierarchy(w io.Writer, remote bool, design string, blocks []*ir.UnexpandedHierarchy) {
	if remote {
		fmt.Fprint(w, hierarchyNote(design, blocks))
	}
}
