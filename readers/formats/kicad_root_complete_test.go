package formats

import (
	"os"
	"testing"
	"testing/fstest"
)

// A root schematic read beside its own same-stem .kicad_pro is the project's root, so a walk that
// opened every sub-sheet covers the design and its by-name nets resolve from external to global,
// exactly as a .kicad_pro read does (agni issue 1013). A schematic with no project beside it may
// be someone else's sub-sheet, and a walk that missed a sub-sheet is partial, so both keep the
// marking.
func TestKicadRootBesideItsProjectResolvesExternalNets(t *testing.T) {
	root, err := os.ReadFile("../kicad/testdata/hier_root.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	child, err := os.ReadFile("../kicad/testdata/hier_child.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name         string
		project      bool
		child        bool
		wantExternal bool
	}{
		{"root beside its project, every sheet opened", true, true, false},
		{"no project beside the schematic", false, true, true},
		{"project present, a sub-sheet missing", true, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := fstest.MapFS{"proj/hier_root.kicad_sch": {Data: root}}
			if tc.child {
				m["proj/hier_child.kicad_sch"] = &fstest.MapFile{Data: child}
			}
			if tc.project {
				m["proj/hier_root.kicad_pro"] = &fstest.MapFile{Data: []byte("{}")}
			}
			d, err := (&Loader{FS: m}).ReadDesign("proj/hier_root.kicad_sch")
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range d.GetNets() {
				if n.GetName() != "VCC" {
					continue
				}
				a := n.GetAttributes()
				if got := a["external"] == "true"; got != tc.wantExternal {
					t.Errorf("VCC external = %v, want %v (attributes %v)", got, tc.wantExternal, a)
				}
				if got := a["global"] == "true"; got == tc.wantExternal {
					t.Errorf("VCC global = %v, want %v (attributes %v)", got, !tc.wantExternal, a)
				}
				return
			}
			t.Fatal("VCC net not found")
		})
	}
}
