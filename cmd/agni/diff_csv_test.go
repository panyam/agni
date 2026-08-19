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
