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
