package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
)

const setDesign = "testdata/conformance/showcase.passes.kicad_sch"

func writeSet(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "audit.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const goodSet = `
title: Showcase audit
preamble: |
  res(?r) :- component.class(?r, "resistor");
queries:
  - name: Resistors
    query: res(?r) => ?r
    description: every resistor
  - name: Resistor count
    query: res(?r) => count(?r)
`

func runSet(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := queryCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestQuerySetJSONIsTheWireMessage(t *testing.T) {
	out, err := runSet(t, setDesign, "--set", writeSet(t, goodSet), "--format", "json")
	if err != nil {
		t.Fatalf("query --set: %v\n%s", err, out)
	}
	var resp webapi.RunQueriesResponse
	if err := protojson.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("not protojson of webapi.RunQueriesResponse: %v\n%s", err, out)
	}
	res := resp.GetResults()
	if len(res) != 2 || res[0].GetName() != "Resistors" || res[1].GetName() != "Resistor count" {
		t.Fatalf("results = %v, want both queries in file order", res)
	}
	if len(res[0].GetResult().GetRows()) == 0 || res[1].GetResult().GetRows()[0].GetCells()[0] == "0" {
		t.Errorf("the preamble's relation answered nothing: %v", res)
	}
	if resp.GetTitle() != "Showcase audit" || resp.GetSource() == "" {
		t.Errorf("response does not name its set and design: title %q source %q", resp.GetTitle(), resp.GetSource())
	}
}

func TestQuerySetDocumentsCarryEverySection(t *testing.T) {
	for _, format := range []string{"text", "markdown", "html"} {
		out, err := runSet(t, setDesign, "--set", writeSet(t, goodSet), "--format", format, "--title", "Renamed")
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		for _, want := range []string{"Renamed", "Resistors", "Resistor count"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: missing %q", format, want)
			}
		}
		if strings.Contains(out, "Showcase audit") {
			t.Errorf("%s: --title did not replace the set's own title", format)
		}
	}
}

// Both refusals come before the design is read, which a nonexistent design proves: the error has to
// be about the flags, not the file.
func TestQuerySetRefusesCSVAndSpecLibBeforeReading(t *testing.T) {
	set := writeSet(t, goodSet)
	if _, err := runSet(t, "no/such/design.edn", "--set", set, "--format", "csv"); err == nil || !strings.Contains(err.Error(), "csv") {
		t.Errorf("csv: err = %v, want a refusal naming csv", err)
	}
	if _, err := runSet(t, "--speclib", "--params", "testdata/conformance/params", "--set", set); err == nil || !strings.Contains(err.Error(), "--speclib") {
		t.Errorf("speclib: err = %v, want a refusal naming --speclib", err)
	}
	if _, err := runSet(t, setDesign, "component.class(?r,?c)", "--set", set); err == nil {
		t.Error("a query argument beside --set was accepted")
	}
}

// Every answer that exists is written, and the command still fails, so a script sees both.
func TestQuerySetPartialFailureWritesEverythingThenFails(t *testing.T) {
	body := goodSet + `  - name: Typo
    query: compnent.class(?r, ?c)
`
	out, err := runSet(t, setDesign, "--set", writeSet(t, body), "--format", "markdown")
	if err == nil || !strings.Contains(err.Error(), "1 of 3 queries") {
		t.Fatalf("err = %v, want one of three queries reported unanswered", err)
	}
	for _, want := range []string{"Resistors", "Resistor count", "Could not answer", "did you mean", "compnent.class"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestQuerySetBadFileIsNamedInTheError(t *testing.T) {
	p := writeSet(t, "queries: [{name: a, query: 'entity(?n,?k)'}, {name: a, query: 'entity(?n,?k)'}]")
	_, err := runSet(t, setDesign, "--set", p)
	if err == nil || !strings.Contains(err.Error(), p) || !strings.Contains(err.Error(), "named twice") {
		t.Errorf("err = %v, want the file and the repeated name", err)
	}
}

// A set on stdin is read the same as a file, and JSON is YAML, so a program can send one it built.
func TestQuerySetReadsStdin(t *testing.T) {
	cmd := queryCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader(`{"title": "piped", "queries": [{"name": "r", "query": "component.class(?r, \"resistor\") => ?r"}]}`))
	cmd.SetArgs([]string{setDesign, "--set", "-", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("query --set -: %v", err)
	}
	var resp webapi.RunQueriesResponse
	if err := protojson.Unmarshal(out.Bytes(), &resp); err != nil || resp.GetTitle() != "piped" || len(resp.GetResults()[0].GetResult().GetRows()) == 0 {
		t.Errorf("stdin set answered %v (err %v)", &resp, err)
	}
}
