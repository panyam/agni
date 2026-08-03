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

func TestQuerySpecLibRequiresParams(t *testing.T) {
	cmd := queryCmd()
	cmd.SetArgs([]string{"--speclib", "param(?m,?s,?x)"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("--speclib without --params must error")
	}
}
