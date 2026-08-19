package main

import (
	"bytes"
	"os"
	"path/filepath"
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

// TestRenderResolvesProjectSymbolLibrary is the geometry twin of TestReadDesignResolvesProjectConfig.
//
// agni issue 228 gave the netlist commands a choke point (readDesign) where a design's project config
// enters the read. `agni render` never went through it, because readDesign returns a netlist and a
// render wants geometry, so the render kept building its loader with no options at all. A project's
// declared symbol library therefore reached every analysis of the design and none of its drawings,
// and --symbol-path was the only route to one (agni issue 347).
//
// Asserted on the drawn ENTITY KEYS rather than on the shape count, because the keys are what the
// failure destroys. An unresolved symbol keeps its placement's reference designator and loses its
// pins, so the sheet still draws all 19 ref-des labels from the annotation pass and draws no bodies.
// Counting elements would call that "a smaller drawing"; counting keys calls it what it is, which is
// a sheet where nothing can be clicked.
func TestRenderResolvesProjectSymbolLibrary(t *testing.T) {
	const design = "../../examples/tutorial-project/designs/gateway/gateway.kicad_sch"
	out := filepath.Join(t.TempDir(), "sheet.svg")
	runCLI(t, renderCmd(), design, "--format", "svg", "--symbols", "faithful", "-o", out)
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("render wrote no file: %v", err)
	}
	svg := string(b)
	if n := strings.Count(svg, `data-kind="component"`); n == 0 {
		t.Errorf("the project declares a symbol library beside its descriptor, so a faithful render "+
			"must draw component bodies carrying entity keys; got none in %d bytes of SVG", len(svg))
	}
	// The positive control for the assertion above. Every reference designator draws from the
	// annotation pass whether or not its symbol resolved, so a render that lost every body still
	// LOOKS complete. If this stops holding, the test above is no longer measuring what it claims.
	if !strings.Contains(svg, "C1") {
		t.Error("expected the sheet's reference designators to draw; the fixture or the annotation pass changed")
	}
}

// TestRenderNotesWhatItCouldNotDraw is agni issue 354 at the CLI.
//
// A render that lost its symbols does not look broken. Every reference designator, every wire and the
// title block still draw, so the sheet reads as complete and only the component bodies are missing.
// Nothing on the page says so, and the honest reading of a sheet showing C1 that will not respond to a
// click is "the tool knows nothing about C1", which is false.
//
// Asserted on stderr rather than the SVG: the note is deliberately not IN the drawing, so that a
// redirected document stays clean.
func TestRenderNotesWhatItCouldNotDraw(t *testing.T) {
	// The same schematic, away from the symbol library its project declares. This is what a checkout
	// without the library, or a design opened outside its project, looks like.
	dir := t.TempDir()
	src, err := os.ReadFile("../../examples/tutorial-project/designs/gateway/gateway.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(dir, "gateway.kicad_sch")
	if err := os.WriteFile(orphan, src, 0o644); err != nil {
		t.Fatal(err)
	}

	var errOut bytes.Buffer
	cmd := renderCmd()
	cmd.SetErr(&errOut)
	runCLI(t, cmd, orphan, "--format", "svg", "--symbols", "faithful", "-o", filepath.Join(dir, "out.svg"))
	got := errOut.String()
	if !strings.Contains(got, "not drawn") {
		t.Errorf("a render that drew no bodies must say so on stderr, got:\n%s", got)
	}
	// The blast radius, which is what tells a reader whether the drawing is worth reading at all.
	if !strings.Contains(got, "19 of 19") {
		t.Errorf("the note must count what was lost against the total, got:\n%s", got)
	}
	// What to do about it. A warning naming no remedy is a warning a reader cannot act on.
	if !strings.Contains(got, "--symbol-path") {
		t.Errorf("the note must name how to resolve it, got:\n%s", got)
	}
}

// The positive control, and the one that keeps the warning worth reading. The SAME schematic in its
// project resolves every symbol, and a note on a complete render would appear on every render.
func TestRenderIsSilentWhenEverythingDraws(t *testing.T) {
	var errOut bytes.Buffer
	cmd := renderCmd()
	cmd.SetErr(&errOut)
	runCLI(t, cmd, "../../examples/tutorial-project/designs/gateway/gateway.kicad_sch",
		"--format", "svg", "--symbols", "faithful", "-o", filepath.Join(t.TempDir(), "out.svg"))
	if strings.Contains(errOut.String(), "not drawn") {
		t.Errorf("a complete render must say nothing about undrawn placements, got:\n%s", errOut.String())
	}
}
