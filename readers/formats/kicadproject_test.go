package formats

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/readers/kicad"
)

// kicadProjectDir copies the two-sheet fixture into a temp dir beside a .kicad_pro that assigns every
// net to a class and declares board-wide minimums, so a read can be checked for what only the project
// carries.
func kicadProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{"twosheet.fires.kicad_sch", "twosheet_sub.kicad_sch"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "cmd", "agni", "testdata", "conformance", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pro := `{
  "board": {"design_settings": {"rules": {"min_track_width": 0.0969, "min_clearance": 0.1}}},
  "net_settings": {
    "classes": [{"name": "Default", "track_width": 0.2}, {"name": "Power", "track_width": 0.5}],
    "netclass_patterns": [{"netclass": "Power", "pattern": "*"}]
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "twosheet.fires.kicad_pro"), []byte(pro), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A design named by its schematic reads what its project declares, as one named by the project does
// (agni issue 933). Before, only the .kicad_pro entry read net classes, and nothing read the
// board-wide minimums.
func TestASchematicEntryReadsItsProject(t *testing.T) {
	dir := kicadProjectDir(t)
	for _, entry := range []string{"twosheet.fires.kicad_sch", "twosheet.fires.kicad_pro"} {
		d, err := (&Loader{}).ReadDesign(filepath.Join(dir, entry))
		if err != nil {
			t.Fatalf("%s: %v", entry, err)
		}
		kinds := map[string]int{}
		for _, c := range d.GetConstraints() {
			kinds[c.GetKind()]++
		}
		if kinds[kicad.ConstraintKindBoardRules] != 1 || kinds[kicad.ConstraintKindNetClass] != 2 {
			t.Errorf("%s: constraint kinds %v, want one board_rules and two netclass", entry, kinds)
		}
		classed := 0
		for _, n := range d.GetNets() {
			if len(n.GetNetClasses()) > 0 {
				classed++
			}
		}
		if classed == 0 || classed != len(d.GetNets()) {
			t.Errorf("%s: %d of %d nets carry the Power class", entry, classed, len(d.GetNets()))
		}
		if r := check.NewModel(d).BoardRules(); r.TrackWidthNm != 96_900 || r.ClearanceNm != 100_000 || r.DrillNm != 0 {
			t.Errorf("%s: board rules %+v", entry, r)
		}
	}
}

// A sub-sheet read on its own has a stem of its own, so the root's project is not its project.
func TestASubSheetDoesNotReadTheRootsProject(t *testing.T) {
	d, err := (&Loader{}).ReadDesign(filepath.Join(kicadProjectDir(t), "twosheet_sub.kicad_sch"))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.GetConstraints()) != 0 {
		t.Errorf("a sub-sheet read picked up constraints %v", d.GetConstraints())
	}
}

// core/check mirrors the reader's constraint kinds rather than importing a reader (C1), so this holds
// the two spellings together.
func TestConstraintKindsAgree(t *testing.T) {
	if kicad.ConstraintKindNetClass != check.NetClassConstraintKind {
		t.Errorf("net class kind: reader %q, model %q", kicad.ConstraintKindNetClass, check.NetClassConstraintKind)
	}
	if kicad.ConstraintKindBoardRules != check.BoardRulesConstraintKind {
		t.Errorf("board rules kind: reader %q, model %q", kicad.ConstraintKindBoardRules, check.BoardRulesConstraintKind)
	}
}
