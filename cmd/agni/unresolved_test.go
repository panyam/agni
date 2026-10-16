package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestTheReportFilesAnInconclusiveFindingAsUnresolved: the markdown report counts an inconclusive
// finding apart from the defects, as the viewer's Checks panel does, rather than under the severity
// the rule would have reported (agni issue 1045).
func TestTheReportFilesAnInconclusiveFindingAsUnresolved(t *testing.T) {
	out := runCLI(t, checkCmd(), filepath.Join("testdata", "conformance", "reverse-blocking.undecided.kicad_sch"), "--format", "markdown")
	for _, want := range []string{"| unresolved | 1 |", "3 finding(s) and 1 unresolved", "\n## unresolved\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "## unresolved") < strings.Index(out, "## warning") {
		t.Errorf("the unresolved section comes before a severity:\n%s", out)
	}
}
