package main

import (
	"regexp"
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

// TestReadDesignResolvesProjectConfig covers the commands that never touch a service: stats, diff,
// emit, render, intake and profilediag all read through readDesign, which built its loader with no
// options at all. So every one of them read under the BUILT-IN naming vocabulary however their
// project was configured, and saw none of its declared symbol libraries (agni issue 228).
//
// Asserted on the stamped roles rather than through a command's output, because that is what the
// read produces and what every one of those commands then derives from. A command-level assertion
// would test one derivation and leave the other five uncovered.
func TestReadDesignResolvesProjectConfig(t *testing.T) {
	d, err := readDesign("../../examples/tutorial-project/designs/gateway/gateway.edn")
	if err != nil {
		t.Fatalf("readDesign: %v", err)
	}
	// The tutorial project names rails function-first, which the start-anchored built-in vocabulary
	// does not match. Its conventions.yaml says so: without that file the only rail is GND.
	var rails int
	for _, n := range d.GetNets() {
		for _, r := range n.GetRoles() {
			if r.GetRole() == "rail" {
				rails++
			}
		}
	}
	if rails < 3 {
		t.Errorf("expected the project's declared rails to be stamped, got %d rail-stamped nets (the built-in vocabulary alone stamps none of them)", rails)
	}
}

// TestReadDesignResolvesSymbolPaths is the other half, and the acceptance criterion agni issue 229
// left unmet: a design's declared symbol library reaches a read that no service mediates, so the
// tutorial project's own Makefile no longer passes --symbol-path.
//
// A schematic whose external library does not resolve reads SHORT rather than failing, so the count
// is the assertion: `libraries` is 0 without the library and 1 with it.
func TestReadDesignResolvesSymbolPaths(t *testing.T) {
	// as-named reads exactly the schematic rather than the netlist its design.yaml declares as the
	// entry. Set directly because it is a ROOT persistent flag.
	saved := readAsNamed
	readAsNamed = true
	defer func() { readAsNamed = saved }()

	d, err := readDesign("../../examples/tutorial-project/designs/gateway/gateway.kicad_sch")
	if err != nil {
		t.Fatalf("readDesign: %v", err)
	}
	if n := len(d.GetLibraries()); n != 1 {
		t.Errorf("the design declares symbols/, so its external library should resolve with no --symbol-path; got %d libraries", n)
	}
}

// TestStatsOutputIsCapturable is the test agni issue 253 existed to make possible, and it doubles as
// the observable half of issue 228: `stats` on a schematic reports the library its design declares,
// with no --symbol-path.
//
// Before, `stats` printed with fmt.Printf, so a caller setting cmd.SetOut got an empty buffer while
// the text still reached the process stdout. An assertion then failed with the expected string
// visibly printed a few lines above it, which is a confusing enough failure that the earlier test for
// this behaviour was rewritten to assert on readDesign instead.
func TestStatsOutputIsCapturable(t *testing.T) {
	saved := readAsNamed
	readAsNamed = true
	defer func() { readAsNamed = saved }()

	out := runCLI(t, statsCmd(), "../../examples/tutorial-project/designs/gateway/gateway.kicad_sch")
	if out == "" {
		t.Fatal("stats wrote nothing to the command's writer; it is printing to the process stdout again")
	}
	if !regexp.MustCompile(`libraries:\s+1\b`).MatchString(out) {
		t.Errorf("the design declares symbols/, so its library should resolve with no --symbol-path:\n%s", out)
	}
	if !regexp.MustCompile(`components:\s+19\b`).MatchString(out) {
		t.Errorf("expected the schematic's 19 components:\n%s", out)
	}
}
