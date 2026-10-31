package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
)

const lexiconGateway = "../../examples/tutorial-project/designs/gateway"

// The gateway's project adds one rail pattern to the built-ins, so the listing has to tell the two
// apart, and a pattern a run no longer matches has no place in it unmarked.
func TestLexiconSaysWhereEachPatternCameFrom(t *testing.T) {
	out := runCLI(t, lexiconCmd(), lexiconGateway)
	for _, want := range []*regexp.Regexp{
		regexp.MustCompile(`(?m)^net\.rail$`),
		regexp.MustCompile(`(?m)^  pattern +\^\\\+ +built-in$`),
		regexp.MustCompile(`(?m)^  pattern +_\[0-9\]\+V\[0-9\]\$ +project \(gateway\)$`),
		regexp.MustCompile(`(?m)^  switch +infer_rail_from_supply_pins=true +built-in$`),
		regexp.MustCompile(`(?m)^class\.tvs$`),
	} {
		if !want.MatchString(out) {
			t.Errorf("the listing lacks %s:\n%s", want, out)
		}
	}
}

// A role says why it is there, and the listing of what the name matches says which vocabulary and
// which tier gave it.
func TestLexiconExplainsANet(t *testing.T) {
	out := runCLI(t, lexiconCmd(), lexiconGateway, "--net", "PMIC_EN")
	for _, want := range []string{
		"PMIC_EN\n  control (convention): the name matches the control pattern _EN$\n",
		"its name matches\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
	if !regexp.MustCompile(`(?m)^  net\.control +pattern +_EN\$ +built-in$`).MatchString(out) {
		t.Errorf("the match list must name the control pattern and its tier:\n%s", out)
	}

	// A net the datasheet tier also backs reads as the rules read it, with the pin that established it.
	out = runCLI(t, lexiconCmd(), lexiconGateway, "--net", "PMIC_MAIN_12V0")
	if !regexp.MustCompile(`(?m)^  rail \(datasheet\): the datasheet lists \S+ pin \S+ as a power pin$`).MatchString(out) {
		t.Errorf("want the datasheet role with its pin:\n%s", out)
	}

	cmd := lexiconCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{lexiconGateway, "--net", "SCL"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `no net named "SCL"; nets containing it: I2C_SCL`) {
		t.Errorf("a name no net carries must be refused with the nets it is near, got %v", err)
	}
}

// --try reads the design with the candidate in place of the project's convention, rules included, so
// switching the control vocabulary off takes PMIC_EN's role and dropping the house rule takes its two
// findings.
func TestLexiconTryListsWhatMoves(t *testing.T) {
	out := runCLI(t, lexiconCmd(), lexiconGateway, "--try", "testdata/lexicon/no-control.yaml", "--format", "json")
	var resp webapi.CompareLexiconsResponse
	if err := protojson.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("--format json is the wire message: %v\n%s", err, out)
	}
	tables := map[string][][]string{}
	for _, tb := range resp.GetTables() {
		for _, r := range tb.GetRows() {
			tables[tb.GetName()] = append(tables[tb.GetName()], r.GetCells())
		}
	}
	roles := tables["role_changes"]
	if len(roles) != 1 || strings.Join(roles[0], "|") != "PMIC_EN|control (convention)|" {
		t.Errorf("role_changes = %v, want PMIC_EN losing its control role alone", roles)
	}
	findings := tables["finding_changes"]
	if len(findings) != 2 {
		t.Fatalf("finding_changes = %v, want the house rule's two findings", findings)
	}
	for _, f := range findings {
		if f[0] != "gone" || f[1] != "gateway/signal-net-naming" {
			t.Errorf("finding change %v, want the house naming rule's finding gone", f)
		}
	}
}

// The yaml form states the lexicon whole, so trying it against the design changes no role. The
// findings it moves are the house rule's alone, since the file carries the lexicon and no rules.
func TestLexiconYAMLReadsTheDesignAsItWasRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "effective.yaml")
	if err := os.WriteFile(path, []byte(runCLI(t, lexiconCmd(), lexiconGateway, "--format", "yaml")), 0o600); err != nil {
		t.Fatal(err)
	}
	out := runCLI(t, lexiconCmd(), lexiconGateway, "--try", path, "--format", "json")
	var resp webapi.CompareLexiconsResponse
	if err := protojson.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatal(err)
	}
	moved := 0
	for _, tb := range resp.GetTables() {
		for _, r := range tb.GetRows() {
			switch {
			case tb.GetName() == "role_changes":
				t.Errorf("the stated lexicon changed a role: %v", r.GetCells())
			case r.GetCells()[1] != "gateway/signal-net-naming":
				t.Errorf("the stated lexicon moved a finding: %v", r.GetCells())
			default:
				moved++
			}
		}
	}
	// A guard that the comparison ran, since an empty answer would also change no role.
	if moved != 2 {
		t.Errorf("want the house rule's two findings gone, got %d", moved)
	}
}

// A candidate that states no lexicon reads names with the built-ins alone, rather than keeping the
// project's vocabulary it replaces. PMIC_MAIN_12V0 is a rail by its PMIC's datasheet either way, and
// test-point-coverage asks the name, so dropping the house rail pattern is what takes its finding.
func TestLexiconTryWithoutALexiconUsesTheBuiltins(t *testing.T) {
	out := runCLI(t, lexiconCmd(), lexiconGateway, "--try", "testdata/lexicon/rules-only.yaml", "--format", "json")
	var resp webapi.CompareLexiconsResponse
	if err := protojson.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, tb := range resp.GetTables() {
		for _, r := range tb.GetRows() {
			rows = append(rows, tb.GetName()+": "+strings.Join(r.GetCells(), "|"))
		}
	}
	want := "finding_changes: gone|test-point-coverage|net|PMIC_MAIN_12V0|rail carries no test point; bring-up and factory test cannot probe it"
	if len(rows) != 1 || rows[0] != want {
		t.Errorf("rows = %q, want only %q", rows, want)
	}
}
