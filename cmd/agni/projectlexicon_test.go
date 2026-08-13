package main

import (
	"strings"
	"testing"
)

// TestProjectLexiconReachesTheRead is the assertion nothing made.
//
// A project's naming convention carries two halves that land in different places: its RULES extend
// the catalog, and its LEXICON has to reach the design READ, because net roles are resolved once at
// ingestion. Every existing test covers the first, or covers the second for a convention supplied on
// a REQUEST. None asserted that a design belonging to a project is read under that project's
// vocabulary, which is the half that decides what a rail is.
func TestProjectLexiconReachesTheRead(t *testing.T) {
	const design = "../../examples/tutorial-project/designs/gateway/gateway.edn"
	out := runCLI(t, queryCmd(), design, "rail(?n) => ?n")
	for _, want := range []string{"PMIC_MAIN_12V0", "PMIC_CORE_3V3", "PMIC_IO_1V8"} {
		if !strings.Contains(out, want) {
			t.Errorf("%s is a rail under the project's declared vocabulary, but the read did not see it:\n%s", want, out)
		}
	}
}
