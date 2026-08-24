package main

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

func runDiffCSV(t *testing.T) ([][]string, string) {
	t.Helper()
	cmd := diffCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "csv", "testdata/rev-a.edn", "testdata/rev-b.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff --format csv: %v", err)
	}
	recs, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v\n%s", err, out.String())
	}
	return recs, out.String()
}

// TestDiffCSVOneTablePerChange is the load-bearing assertion for the shape decision: a diff report
// is four collections of different kinds, and they arrive as one table discriminated by
// change_class. Every row is the full width, so a component row and a net row parse the same.
func TestDiffCSVOneTablePerChange(t *testing.T) {
	recs, raw := runDiffCSV(t)

	if got := recs[0]; !equalStrings(got, diffCSVColumns) {
		t.Errorf("header = %v, want %v", got, diffCSVColumns)
	}
	for i, rec := range recs {
		if len(rec) != len(diffCSVColumns) {
			t.Errorf("record %d has %d fields, want %d: %v", i, len(rec), len(diffCSVColumns), rec)
		}
	}

	byClass := map[string][]string{}
	for _, rec := range recs[1:] {
		byClass[rec[0]] = rec
	}
	for _, want := range []string{"component-added", "net-new", "net-deleted", "net-renamed", "net-hard"} {
		if _, ok := byClass[want]; !ok {
			t.Errorf("no %s row; the fixture pair exercises it\n%s", want, raw)
		}
	}

	if got := byClass["component-added"][1]; got != "R4" {
		t.Errorf("component-added subject = %q, want R4", got)
	}
	if row := byClass["net-renamed"]; row[1] != "DATA" || row[2] != "SIG" {
		t.Errorf("net-renamed = (subject %q, old_name %q), want (DATA, SIG)", row[1], row[2])
	}
}

// TestDiffCSVProvenanceIsPerSide checks the two source-file columns carry the asymmetry that makes
// them worth having: a new net exists only on the right, a deleted one only on the left.
func TestDiffCSVProvenanceIsPerSide(t *testing.T) {
	recs, _ := runDiffCSV(t)
	byClass := map[string][]string{}
	for _, rec := range recs[1:] {
		byClass[rec[0]] = rec
	}

	if row := byClass["net-new"]; row[8] != "" || !strings.HasSuffix(row[9], "rev-b.edn") {
		t.Errorf("net-new provenance = (%q, %q), want (empty, .../rev-b.edn)", row[8], row[9])
	}
	if row := byClass["net-deleted"]; !strings.HasSuffix(row[8], "rev-a.edn") || row[9] != "" {
		t.Errorf("net-deleted provenance = (%q, %q), want (.../rev-a.edn, empty)", row[8], row[9])
	}
}

// TestDiffCSVMultiValueCell pins the intra-cell separator for a net's changed endpoints. It must
// not be a comma, or a reader splitting the cell cannot tell it from the field delimiter.
func TestDiffCSVMultiValueCell(t *testing.T) {
	recs, _ := runDiffCSV(t)
	for _, rec := range recs[1:] {
		if rec[0] != "net-hard" {
			continue
		}
		if rec[6] == "" && rec[7] == "" {
			t.Error("net-hard row reports neither added nor removed endpoints")
		}
		if strings.Contains(rec[6], ",") || strings.Contains(rec[7], ",") {
			t.Errorf("endpoint cell uses a comma separator: added=%q removed=%q", rec[6], rec[7])
		}
		return
	}
	t.Fatal("no net-hard row to check")
}

func TestDiffCSVIsDeterministic(t *testing.T) {
	_, first := runDiffCSV(t)
	_, second := runDiffCSV(t)
	if first != second {
		t.Error("two runs over the same pair produced different bytes, so two exports cannot be diffed")
	}
}

// TestDiffRejectsUnknownFormat covers the behaviour change alongside the new format: this command
// used to accept any --format value and silently render text, so a misspelling produced a human
// summary that a script then failed to parse for reasons nothing explained.
func TestDiffRejectsUnknownFormat(t *testing.T) {
	cmd := diffCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--format", "jsn", "testdata/rev-a.edn", "testdata/rev-b.edn"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("diff --format jsn succeeded; an unknown format must not fall through to text")
	}
	if !strings.Contains(err.Error(), "unknown --format") {
		t.Errorf("error = %v, want it to name the unknown format", err)
	}
}

// TestDiffRenameApproxIsOptIn drives the near-match pass through the CLI, which is the seam the flag
// actually crosses: the engine's default is disabled, so a missing flag here would silently produce
// today's output and every unit test in core/diff would still pass.
func TestDiffRenameApproxIsOptIn(t *testing.T) {
	run := func(args ...string) string {
		var out bytes.Buffer
		cmd := diffCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("diff %v: %v", args, err)
		}
		return out.String()
	}

	off := run("testdata/rev-a.edn", "testdata/rev-c.edn")
	if !strings.Contains(off, "[deleted] CLK") || !strings.Contains(off, "[new]     OSC") {
		t.Errorf("without the flag CLK and OSC are unpaired:\n%s", off)
	}
	if strings.Contains(off, "renamed-approx") {
		t.Errorf("the pass must not run without the flag:\n%s", off)
	}

	on := run("--rename-approx", "testdata/rev-a.edn", "testdata/rev-c.edn")
	if !strings.Contains(on, "[renamed?] CLK -> OSC") {
		t.Errorf("with the flag the pairing is recovered:\n%s", on)
	}
	if strings.Contains(on, "[deleted] CLK") || strings.Contains(on, "[new]     OSC") {
		t.Errorf("a recovered pairing must not ALSO report as new and deleted:\n%s", on)
	}
	// The exact pass is unaffected either way, which is what keeps a recovered fact from being
	// downgraded to a guess when the flag is on.
	for _, out := range []string{off, on} {
		if !strings.Contains(out, "[renamed] SIG -> DATA") {
			t.Errorf("the exact rename survives in both modes:\n%s", out)
		}
	}
}

// TestDiffCSVCarriesRenameEvidence: the spreadsheet is where a reviewer triages a near match, so the
// numbers that decided it have to be in columns rather than only in the text form's prose.
func TestDiffCSVCarriesRenameEvidence(t *testing.T) {
	var out bytes.Buffer
	cmd := diffCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "csv", "--rename-approx", "testdata/rev-a.edn", "testdata/rev-c.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff csv: %v", err)
	}
	recs, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	idx := map[string]int{}
	for i, h := range recs[0] {
		idx[h] = i
	}
	for _, col := range []string{"match_old_coverage", "match_old_coverage_significant", "match_new_coverage_significant"} {
		if _, ok := idx[col]; !ok {
			t.Fatalf("column %q missing from header %v", col, recs[0])
		}
	}
	var row []string
	for _, r := range recs[1:] {
		if r[idx["change_class"]] == "net-renamed-approx" {
			row = r
		}
	}
	if row == nil {
		t.Fatalf("no net-renamed-approx row in:\n%s", out.String())
	}
	if got := row[idx["match_old_coverage_significant"]]; got != "1.000" {
		t.Errorf("significant coverage = %q, want 1.000", got)
	}
	if got := row[idx["added"]]; got != "TP1.1" {
		t.Errorf("added = %q, want the gained probe", got)
	}
	// Every other row leaves the columns empty, so a reader sorting on them sees only the rows the
	// pass actually judged.
	for _, r := range recs[1:] {
		if r[idx["change_class"]] != "net-renamed-approx" && r[idx["match_old_coverage"]] != "" {
			t.Errorf("%s row carries match evidence: %v", r[idx["change_class"]], r)
		}
	}
}
