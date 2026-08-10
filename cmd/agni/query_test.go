package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// TestQuerySpecLibCLI exercises the --speclib wiring end to end: no <file>, the corpus (--params) is the
// fact base, and the datasheet relations range over every seeded part (WS10-010). printQueryRows
// writes to os.Stdout, so capture it via a pipe.
func TestQuerySpecLibCLI(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := queryCmd()
	cmd.SetArgs([]string{"--speclib", "--params", "testdata/conformance/params", "param(?mpn, ?sym, ?max)"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("query --speclib: %v", err)
	}
	// The seeded corpus parts appear with no design loaded — the spec library IS the fact base.
	for _, want := range []string{"DEMO-MCU33", "LM1117"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("spec library query missing corpus part %q:\n%s", want, out)
		}
	}
}

// TestQueryDesignCLI exercises the design-query path now that it is a thin client of QueryService
// (WS9-048): the CLI builds the service over a local loader, runs the query, and renders the proto
// rows as the same table as before — columns, a provenance column, and the result count.
func TestQueryDesignCLI(t *testing.T) {
	cmd := queryCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"testdata/conformance/showcase.passes.kicad_sch", `component.class(?r, "capacitor") => ?r`})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("query: %v", err)
	}
	s := out.String()
	for _, want := range []string{"r", "provenance", "C1", "5 result(s)"} {
		if !strings.Contains(s, want) {
			t.Errorf("query output missing %q:\n%s", want, s)
		}
	}
}

// TestQueryNetClassCLI (WS3-105) covers the whole net-class path, which no unit test can: the class
// lives only in the .kicad_pro net_settings, so it reaches the IR through the PROJECT entry point
// (the loader's AnnotateNetClasses call) and is invisible when the same board is opened as a bare
// .kicad_sch. Both halves are asserted, because "the schematic read shows no classes" is the exact
// silence the has_netclass marker exists to make visible.
func TestQueryNetClassCLI(t *testing.T) {
	run := func(path, q string) string {
		t.Helper()
		cmd := queryCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{path, q})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("query %s: %v", path, err)
		}
		return out.String()
	}

	s := run("testdata/conformance/showcase.passes.kicad_pro", "net.netclass(?n, ?c) => ?n, ?c")
	for _, want := range []string{"USB_D+", "HighSpeed", "VBUS_PROT", "Power"} {
		if !strings.Contains(s, want) {
			t.Errorf("project net.netclass query missing %q:\n%s", want, s)
		}
	}
	// WS1-050: the projection is 1:many, so a net in several classes fans out to one row each.
	// +3V3 carries an array-form assignment (Power, Critical) and the USB pair matches two
	// overlapping patterns; every membership must be its own row.
	for _, want := range []string{"Critical", "Differential", "8 result(s)"} {
		if !strings.Contains(s, want) {
			t.Errorf("project net.netclass query missing multi-class row %q:\n%s", want, s)
		}
	}
	// SCL is in no class, so it must not appear — an unclassed net yields no row at all.
	if strings.Contains(s, "SCL") {
		t.Errorf("net.netclass listed the unclassed net SCL:\n%s", s)
	}
	if mk := run("testdata/conformance/showcase.passes.kicad_pro", "has_netclass(?p) => ?p"); !strings.Contains(mk, "1 result(s)") {
		t.Errorf("project has_netclass: want one row:\n%s", mk)
	}

	// The same board read as a bare schematic never sees the project file: no classes, no marker.
	if bare := run("testdata/conformance/showcase.passes.kicad_sch", "net.netclass(?n, ?c) => ?n, ?c"); !strings.Contains(bare, "no results") {
		t.Errorf("schematic-only read must yield no net classes:\n%s", bare)
	}
	if bare := run("testdata/conformance/showcase.passes.kicad_sch", "has_netclass(?p) => ?p"); !strings.Contains(bare, "no results") {
		t.Errorf("schematic-only read must yield no has_netclass marker:\n%s", bare)
	}
}

func TestQuerySpecLibRequiresParams(t *testing.T) {
	cmd := queryCmd()
	cmd.SetArgs([]string{"--speclib", "param(?m,?s,?x)"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("--speclib without --params must error")
	}
}

// TestQueryNetClassDefsCLI (WS3-111) covers the declared-constraint path end to end, which no unit
// test can: the definitions live only in the .kicad_pro, so they reach the IR through the PROJECT
// entry point and are invisible when the same board is opened as a bare .kicad_sch.
func TestQueryNetClassDefsCLI(t *testing.T) {
	run := func(path, q string) string {
		t.Helper()
		cmd := queryCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{path, q})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("query %s: %v", path, err)
		}
		return out.String()
	}
	const pro = "testdata/conformance/showcase.passes.kicad_pro"

	// The per-class rows are the raw declarations: HighSpeed states a clearance and no width.
	raw := run(pro, `netclass.track_width(?c, ?mm) => ?c, ?mm`)
	if strings.Contains(raw, "HighSpeed") {
		t.Errorf("HighSpeed declares no track width and must yield no row:\n%s", raw)
	}

	// The cascade: USB_D+ is in HighSpeed (priority 1, states no width) and Differential
	// (priority 2, states 0.2). It must resolve to Differential's value, NOT to no value and NOT
	// to Default's 0.25 — the per-FIELD fall-through is the whole point.
	d := run(pro, `net.declared_track_width(?n, ?mm) => ?n, ?mm`)
	for _, want := range []string{"USB_D+", "0.2", "net_settings:Differential"} {
		if !strings.Contains(d, want) {
			t.Errorf("declared track width missing %q:\n%s", want, d)
		}
	}
	// A net in no class still gets Default's value, because Default applies to every net.
	if !strings.Contains(d, "net_settings:Default") {
		t.Errorf("an unclassed net must still carry Default's declared width:\n%s", d)
	}
	if !strings.Contains(run(pro, "has_netclass_defs(?p) => ?p"), "1 result(s)") {
		t.Error("project has_netclass_defs: want one row")
	}

	// The same board read as a bare schematic never sees the project file.
	if bare := run("testdata/conformance/showcase.passes.kicad_sch", "has_netclass_defs(?p) => ?p"); !strings.Contains(bare, "no results") {
		t.Errorf("schematic-only read must yield no netclass definitions:\n%s", bare)
	}
}

// runQuery executes one `agni query` invocation and returns its stdout.
func runQuery(t *testing.T, args ...string) string {
	t.Helper()
	cmd := queryCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("query %v: %v", args, err)
	}
	return out.String()
}

// TestQueryConventionsCLI is WS3-113's reproduction, end to end through the command.
//
// `rail(?n)` is not a relation the convention adds; it is one whose ANSWER the convention changes,
// because the lexicon is applied at the design read and net roles are resolved there. On this fixture
// the built-in vocabulary — start-anchored on VCC/VDD/+3V3 — matches none of the project's
// function-first rail names, so the engine reports one rail on a board with more. That is a correct
// answer to a question the project did not ask, and until now there was no way to ask theirs.
func TestQueryConventionsCLI(t *testing.T) {
	design := "testdata/review/conv-demo.edn"
	q := "rail(?n) => ?n"

	builtin := runQuery(t, design, q)
	if strings.Contains(builtin, "PMIC_VDD_LPM_1V8") {
		t.Fatal("the built-in vocabulary already matches this fixture's rail names; it no longer demonstrates the gap")
	}
	if !strings.Contains(builtin, "1 result(s)") {
		t.Errorf("without --conventions, want the one built-in-vocabulary rail:\n%s", builtin)
	}

	house := runQuery(t, design, q, "--conventions", "testdata/review/conventions.yaml")
	if !strings.Contains(house, "PMIC_VDD_LPM_1V8") {
		t.Errorf("--conventions did not reach the design read:\n%s", house)
	}
	if !strings.Contains(house, "2 result(s)") {
		t.Errorf("want both rails under the project's own vocabulary:\n%s", house)
	}
}

// TestQueryConventionsUnreadableCLI: a missing or malformed config is an error at the edge, never a
// silent fall back to the built-in vocabulary. Falling back would answer a different question than
// the one asked and say nothing about it, which on this surface is the whole failure mode: the user
// is asking what the engine believes, and would be told what it believes under somebody else's words.
func TestQueryConventionsUnreadableCLI(t *testing.T) {
	for _, path := range []string{"testdata/review/does-not-exist.yaml", "testdata/review/conv-demo.edn"} {
		cmd := queryCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetArgs([]string{"testdata/review/conv-demo.edn", "rail(?n) => ?n", "--conventions", path})
		if err := cmd.Execute(); err == nil {
			t.Errorf("--conventions %s must error, not answer under the built-in vocabulary", path)
		}
	}
}

// TestQueryBoardPathCLI: the board.* relations range over a board tier the netlist does not carry, so
// without an attached export they are empty — indistinguishable, in a result table, from a board with
// nothing to report.
func TestQueryBoardPathCLI(t *testing.T) {
	design := "testdata/review/can-broken.edn"
	q := "board.track_width(?n,?w) => ?n, ?w"

	if got := runQuery(t, design, q); !strings.Contains(got, "no results") {
		t.Errorf("a netlist with no attached board should yield no board facts:\n%s", got)
	}
	withBoard := runQuery(t, design, q, "--board-path", "testdata/conformance/drc.fires.kicad_pcb")
	if strings.Contains(withBoard, "no results") {
		t.Errorf("--board-path did not attach the board tier:\n%s", withBoard)
	}
	if !strings.Contains(withBoard, "result(s)") {
		t.Errorf("want board facts with the export attached:\n%s", withBoard)
	}
}
