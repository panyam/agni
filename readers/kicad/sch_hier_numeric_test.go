package kicad

import (
	"os"
	"path/filepath"
	"testing"
)

// agni issue 1143: hier_numeric_root is our own hierarchical project whose two sub-sheets under
// different parents are both named "1", the case the Sheets dock has to tell apart. The ids are the
// sheet paths, so they differ where the names do not, and each nests under its own parent.
func TestNumericSubSheetsKeepDistinctPaths(t *testing.T) {
	open := func(rel string) ([]byte, error) { return os.ReadFile(filepath.Join("testdata", rel)) }
	g, err := ReadSchematicHierarchy("hier_numeric_root.kicad_sch", readFixture(t, "hier_numeric_root.kicad_sch"), open)
	if err != nil {
		t.Fatal(err)
	}
	var got [][3]string
	for _, s := range g.GetSheets() {
		got = append(got, [3]string{s.GetId(), s.GetName(), s.GetParentId()})
	}
	want := [][3]string{
		{"/", "Hier numeric root", ""},
		{"/Power", "Power", "/"},
		{"/Power/1", "1", "/Power"},
		{"/IO", "IO", "/"},
		{"/IO/1", "1", "/IO"},
	}
	if len(got) != len(want) {
		t.Fatalf("sheets %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sheet %d is %v, want %v", i, got[i], want[i])
		}
	}
}
