package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestCheckMarkdown pins the report contract: a severity summary table, sections ordered
// error -> warning -> info (empty severities omitted), rule headings carrying the catalog
// Summary, and one line per finding with subject, message, and source provenance. fires.edn
// yields one finding of each severity, so the full ordering is exercised on a real reader path.
func TestCheckMarkdown(t *testing.T) {
	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "markdown", "testdata/conformance/fires.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check --format markdown: %v", err)
	}
	got := out.String()

	for _, want := range []string{
		"| severity | findings |",
		"| error | 1 |",
		"| warning | 1 |",
		"| info | 1 |",
		"## error",
		"## warning",
		"## info",
		"### i2c-pull-up — An I2C net (SDA/SCL) reaches no rail through a pull-up resistor.",
		"### unconnected-component — A component appears on no net (none of its pins land on any signal).",
		"### single-pin-net — A net connects to fewer than two pins (a floating stub), and is not an intentional no-connect.",
		"- `SDA` — I2C net has no pull-up resistor",
		"- `X1` — component has no net connections",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report missing %q\n%s", want, got)
		}
	}
	// Severity sections in order: error before warning before info.
	e, w, i := strings.Index(got, "## error"), strings.Index(got, "## warning"), strings.Index(got, "## info")
	if !(e < w && w < i) {
		t.Errorf("severity sections out of order: error@%d warning@%d info@%d", e, w, i)
	}
}

// TestCheckFailOn covers the CI-gate exit matrix: the run errors exactly when a finding at or
// above the threshold exists, and the default (no flag) stays exit-0 so existing callers are
// unaffected. fires.edn has error+warning+info findings; passes.edn has none.
func TestCheckFailOn(t *testing.T) {
	run := func(args ...string) error {
		cmd := checkCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		return cmd.Execute()
	}
	if err := run("testdata/conformance/fires.edn"); err != nil {
		t.Errorf("no --fail-on must not error, got %v", err)
	}
	if err := run("--fail-on", "error", "testdata/conformance/fires.edn"); err == nil {
		t.Error("--fail-on error with an error finding should exit non-zero")
	}
	if err := run("--fail-on", "info", "testdata/conformance/fires.edn"); err == nil {
		t.Error("--fail-on info with any finding should exit non-zero")
	}
	if err := run("--fail-on", "error", "testdata/conformance/passes.edn"); err != nil {
		t.Errorf("--fail-on error with no findings must pass, got %v", err)
	}
	if err := run("--fail-on", "bogus", "testdata/conformance/fires.edn"); err == nil {
		t.Error("unknown --fail-on value should be rejected")
	}
	// The gate composes with the report formats, not just text.
	if err := run("--format", "markdown", "--fail-on", "error", "testdata/conformance/fires.edn"); err == nil {
		t.Error("--fail-on should also gate --format markdown runs")
	}
}

// TestCheckReportSheets pins WS3-023 for the report path: `check --format report` annotates the
// findings nested in the severity report with their sheet membership, via the same
// service.AnnotateReport the web GetCheckReport RPC uses. The U1 component finding on the
// multi-sheet sheetnav fixture locates on the root sheet.
func TestCheckReportSheets(t *testing.T) {
	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "report", "testdata/conformance/sheetnav.fires.kicad_sch"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check --format report: %v", err)
	}
	var got struct {
		Report struct {
			Sections []struct {
				Rules []struct {
					Findings []sheetFinding `json:"findings"`
				} `json:"rules"`
			} `json:"sections"`
		} `json:"report"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	var u1 []string
	var found bool
	for _, s := range got.Report.Sections {
		for _, g := range s.Rules {
			for _, f := range g.Findings {
				if f.Subject.Kind == "component" && f.Subject.Ref == "U1" {
					u1, found = f.Sheets, true
				}
			}
		}
	}
	if !found {
		t.Fatal("expected a U1 component finding in the report")
	}
	if len(u1) != 1 || u1[0] != "/" {
		t.Errorf("U1 report sheets = %v, want [\"/\"]", u1)
	}
}
