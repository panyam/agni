package main

import (
	"bytes"
	"fmt"
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

// TestQuerySpecLibUnitsCLI (agni issue 165) is the end-to-end proof that the query surface's numbers
// are in SI base units, run over the real conformance corpus rather than a hand-built model.
//
// DEMO-HSS-CTRL seeds its overcurrent threshold in MILLIVOLTS, as a real controller sheet prints it.
// Before this, `param(?mpn,"V(OCP)",?max)` yielded 50 and a rule written `?max < 0.1` (a
// hundred-millivolt sanity bound) would have compared 50 against 0.1 and read as wildly over. The
// same query now yields 0.05, and the threshold means what it says.
//
// The companion assertion is the point of the split: param.unit still reports "mV", so normalizing
// the number did not erase what the vendor printed.
func TestQuerySpecLibUnitsCLI(t *testing.T) {
	run := func(q string) string {
		t.Helper()
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		cmd := queryCmd()
		cmd.SetArgs([]string{"--speclib", "--params", "testdata/conformance/params", q})
		err := cmd.Execute()
		w.Close()
		os.Stdout = old
		out, _ := io.ReadAll(r)
		if err != nil {
			t.Fatalf("query %q: %v", q, err)
		}
		return string(out)
	}

	// The threshold is seeded as 50 mV; the query surface must report it in volts.
	got := run(`param(?mpn, "V(OCP)", ?max) => ?mpn, ?max`)
	if !strings.Contains(got, "0.05") {
		t.Errorf("V(OCP) must project as 0.05 V, not its printed 50 mV:\n%s", got)
	}
	if strings.Contains(got, "50") && !strings.Contains(got, "0.05") {
		t.Errorf("V(OCP) still projecting the printed millivolt number:\n%s", got)
	}

	// A numeric comparison against a volt-denominated bound now behaves: 0.05 < 0.1 holds.
	if bounded := run(`param(?mpn, "V(OCP)", ?max), ?max < 0.1 => ?mpn`); !strings.Contains(bounded, "DEMO-HSS-CTRL") {
		t.Errorf("a 50mV threshold must satisfy a 0.1V bound:\n%s", bounded)
	}

	// And the printed spelling is still queryable.
	if units := run(`param.unit(?mpn, "V(OCP)", ?u) => ?mpn, ?u`); !strings.Contains(units, "mV") {
		t.Errorf("param.unit must still report the printed mV:\n%s", units)
	}
}

// TestQueryAbsentPredicateCLI is the end-to-end payoff of making absence representable: selecting the
// rows a datasheet did not state is now a query rather than an apology in a doc.
//
// The conformance controller states V(OCP) as min/typ/max, so it has a lower bound; the MCU's
// absolute-maximum rows are one-sided, so they do not. Before this, both bound their missing side to
// the empty string and nothing could tell them apart, which is why `param.unit.md` shipped a note
// saying there was no way to ask.
func TestQueryAbsentPredicateCLI(t *testing.T) {
	run := func(q string) string {
		t.Helper()
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		cmd := queryCmd()
		cmd.SetArgs([]string{"--speclib", "--params", "testdata/conformance/params", q})
		err := cmd.Execute()
		w.Close()
		os.Stdout = old
		out, _ := io.ReadAll(r)
		if err != nil {
			t.Fatalf("query %q: %v", q, err)
		}
		return string(out)
	}

	// A one-sided row: absolute maximums are stated as a ceiling with no floor.
	oneSided := run(`param.range(?m, ?s, "absolute_max", ?min, ?max), absent(?min) => ?m, ?s, ?max`)
	if !strings.Contains(oneSided, "result(s)") || strings.Contains(oneSided, "no results") {
		t.Errorf("absent(?min) must select the one-sided absolute-max rows:\n%s", oneSided)
	}

	// Its negation is the other half, and the two must partition the same relation.
	twoSided := run(`param.range(?m, ?s, "absolute_max", ?min, ?max), not absent(?min) => ?m, ?s, ?min, ?max`)
	all := run(`param.range(?m, ?s, "absolute_max", ?min, ?max) => ?m, ?s`)
	if countResults(t, oneSided)+countResults(t, twoSided) != countResults(t, all) {
		t.Errorf("absent(?min) and not absent(?min) must partition the relation:\none-sided:\n%s\ntwo-sided:\n%s\nall:\n%s",
			oneSided, twoSided, all)
	}

	// An absent bound must not satisfy a numeric guard, which is what it did by string order before.
	guarded := run(`param.range(?m, ?s, "absolute_max", ?min, ?max), ?min < 0.1 => ?m, ?s`)
	if !strings.Contains(guarded, "no results") {
		t.Errorf("an absent lower bound must not satisfy `?min < 0.1`:\n%s", guarded)
	}
}

// countResults reads the "N result(s)" trailer the query renderer prints.
func countResults(t *testing.T, out string) int {
	t.Helper()
	if strings.Contains(out, "no results") {
		return 0
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), "result(s)") {
			var n int
			if _, err := fmt.Sscanf(strings.TrimSpace(line), "%d result(s)", &n); err == nil {
				return n
			}
		}
	}
	t.Fatalf("no result count in output:\n%s", out)
	return 0
}

// TestQueryRejectsUnknownFormatBeforeReading is about WHERE the check happens, not that it happens.
// A misspelled --format must fail before the design is parsed, because the designs this runs on are
// nine-megabyte netlists and a typo that costs a full read reads as a slow tool. The path given is
// deliberately one that does not exist: if validation ever moves after the read, this stops
// reporting the format error and starts reporting a missing file.
func TestQueryRejectsUnknownFormatBeforeReading(t *testing.T) {
	cmd := queryCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--format", "markdwon", "no/such/design.kicad_sch", "component.class(?r,?c) => ?r"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("a misspelled --format was accepted")
	}
	if !strings.Contains(err.Error(), "unknown --format") {
		t.Errorf("error = %v, want the format complaint rather than a read failure", err)
	}
}

// TestQueryFormatsReachTheRenderers is the end-to-end half: each format is wired to its renderer and
// runs over a real design read. The per-format behaviour is asserted in core/report; this asserts the
// dispatch, which is the part that can silently fall through to text.
func TestQueryFormatsReachTheRenderers(t *testing.T) {
	for _, tc := range []struct{ format, want string }{
		{"csv", "r,provenance"},
		{"json", `"columns"`},
		{"markdown", "| r | provenance |"},
		{"html", "<!doctype html>"},
	} {
		t.Run(tc.format, func(t *testing.T) {
			cmd := queryCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"--format", tc.format, "testdata/conformance/showcase.passes.kicad_sch", `component.class(?r, "capacitor") => ?r`})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("query --format %s: %v", tc.format, err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Errorf("--format %s output missing %q:\n%s", tc.format, tc.want, out.String())
			}
		})
	}
}

// TestQueryViewNamesTheMountNotTheHost is the leak guard, stated as the POSITIVE contract because
// the negative form does not hold under test. A view is committed, mailed and pasted into tickets,
// so its heading must name the design's mount URI. Asserting merely that no host path appears is
// vacuous here: the test passes a RELATIVE path, so the buggy version (which echoes the argument)
// leaks nothing a substring check can see, and the guard reported green with the bug reinstated.
// Requiring the mount URI fails on any source that is not one, including a bare argument.
//
// This is agni issue 501's rule arriving in a new output format, which inherits the rule and not the
// fix.
func TestQueryViewNamesTheMountNotTheHost(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"markdown", "html", "json"} {
		cmd := queryCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--format", format, "testdata/conformance/showcase.passes.kicad_sch", `component.class(?r, "capacitor") => ?r`})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("query --format %s: %v", format, err)
		}
		if !strings.Contains(out.String(), "mount://") {
			t.Errorf("--format %s does not name the design by its mount URI:\n%s", format, out.String())
		}
		if strings.Contains(out.String(), wd) {
			t.Errorf("--format %s published the host path %q", format, wd)
		}
	}
}
