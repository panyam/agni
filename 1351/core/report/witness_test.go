package report

import (
	"bytes"
	"strings"
	"testing"
)

func probedWitness() []Witness {
	return []Witness{{
		Relation: "component.probed_one", Values: []string{"R1", "VBUS", "SENSE"},
		Rule: "component.probed_one(?r, ?p, ?u) :-\n  net.has_test_point(?p), not net.has_test_point(?u)",
		Children: []Witness{
			{Relation: "net.has_test_point", Values: []string{"VBUS"}, Cites: []string{"x.edn:VBUS", "x.edn:TP1"}},
			{Relation: "net.has_test_point", Values: []string{""}, Negated: true},
		},
	}}
}

// A witness tree reads one fact per line, each body fact a level under the fact it proves, a derived
// fact with its rule on one line, a leaf with its citations and a negated leaf as an absence.
func TestWitnessLines(t *testing.T) {
	got := strings.Join(WitnessLines(probedWitness()), "\n")
	want := "component.probed_one(R1, VBUS, SENSE)  via component.probed_one(?r, ?p, ?u) :- net.has_test_point(?p), not net.has_test_point(?u)\n" +
		"  net.has_test_point(VBUS)  [x.edn:VBUS ; x.edn:TP1]\n" +
		"  not net.has_test_point(_)  (absent)"
	if got != want {
		t.Errorf("WitnessLines =\n%s\nwant\n%s", got, want)
	}
}

// The text table keeps its rows aligned with a tree under one of them, and a row with no witnesses
// prints as it always did.
func TestTableTextPutsTheTreeUnderItsRow(t *testing.T) {
	tab := Table{Columns: []string{"r", "u"}, Rows: []TableRow{
		{Cells: []string{"R1", "SENSE"}, Cites: []string{"x.edn"}, Witnesses: probedWitness()},
		{Cells: []string{"R22", "GND"}, Cites: []string{"x.edn"}},
	}}
	var b bytes.Buffer
	if err := TableText(&b, tab); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(b.String(), "\n")
	if len(lines) < 6 {
		t.Fatalf("%d lines:\n%s", len(lines), b.String())
	}
	if !strings.HasPrefix(lines[1], "R1   SENSE  x.edn") || !strings.HasPrefix(lines[5], "R22  GND    x.edn") {
		t.Errorf("rows not aligned across the tree:\n%s", b.String())
	}
	if lines[2] != "    component.probed_one(R1, VBUS, SENSE)  via component.probed_one(?r, ?p, ?u) :- net.has_test_point(?p), not net.has_test_point(?u)" ||
		lines[4] != "      not net.has_test_point(_)  (absent)" {
		t.Errorf("the tree is not under its row:\n%s", b.String())
	}
}

// A cell holding a newline spans two lines, so the tree cannot be placed by line, and it goes after
// the table under its row number rather than under the wrong row.
func TestTableTextPutsTheTreeAfterATableWithAMultilineCell(t *testing.T) {
	tab := Table{Columns: []string{"r"}, Rows: []TableRow{
		{Cells: []string{"A\nB"}},
		{Cells: []string{"R1"}, Witnesses: probedWitness()},
	}}
	var b bytes.Buffer
	if err := TableText(&b, tab); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "\nrow 2:\n    component.probed_one(R1, VBUS, SENSE)") {
		t.Errorf("the tree is not after the table under its row number:\n%s", b.String())
	}
}

// html puts the tree in a row spanning the table, under the row it explains.
func TestTableHTMLPutsTheTreeUnderItsRow(t *testing.T) {
	tab := Table{Columns: []string{"r"}, Rows: []TableRow{{Cells: []string{"R1"}, Witnesses: probedWitness()}, {Cells: []string{"R2"}}}}
	var b bytes.Buffer
	if err := TableHTML(&b, tab); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	tree := strings.Index(out, `<tr class="how"><td colspan="2"><pre>component.probed_one(R1, VBUS, SENSE)`)
	if tree < 0 || strings.Index(out, ">R1<") > tree || strings.Index(out, ">R2<") < tree {
		t.Errorf("the tree is not between R1 and R2:\n%s", out)
	}
	if strings.Count(out, `class="how"`) != 1 {
		t.Errorf("want one tree row, for the one witnessed row:\n%s", out)
	}
}
