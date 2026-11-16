package altium

import (
	"testing"
)

// A three-layer Board6 runs top, mid 1, bottom. Its lengths come in mils and millimetres, it states
// no top mask, and its dielectrics are a core and a prepreg (agni 1601).
func TestTheStackFollowsBoard6sCopperChain(t *testing.T) {
	b := props{
		"LAYER1NAME": "Top", "LAYER1NEXT": "2", "LAYER1COPTHICK": "1.4mil", "LAYER1DIELHEIGHT": "0.2mm", "LAYER1DIELMATERIAL": "FR-4", "LAYER1DIELTYPE": "1", "LAYER1DIELCONST": "4.6",
		"LAYER2NAME": "Mid", "LAYER2NEXT": "32", "LAYER2COPTHICK": "0.035mm", "LAYER2DIELHEIGHT": "10mil", "LAYER2DIELMATERIAL": "PP", "LAYER2DIELTYPE": "2",
		"LAYER32NAME": "Bottom", "LAYER32COPTHICK": "35um",
		"TOPHEIGHT": "0mil", "BOTTOMHEIGHT": "0.5mil", "BOTTOMMATERIAL": "Solder Resist", "BOTTOMCONST": "3.5",
	}
	stack := copperStack(b)
	s := stackup(b, stack, map[int]string{1: "F.Cu", 2: "In1.Cu", 32: "B.Cu"})
	type row struct {
		name, kind, layer string
		nm                int64
		material, typ     string
	}
	want := []row{
		{"Top", "copper", "F.Cu", 35_560, "", ""},
		{"Dielectric 1", "dielectric", "", 200_000, "FR-4", "core"},
		{"Mid", "copper", "In1.Cu", 35_000, "", ""},
		{"Dielectric 2", "dielectric", "", 254_000, "PP", "prepreg"},
		{"Bottom", "copper", "B.Cu", 35_000, "", ""},
		{"B.Mask", "mask", "B.Mask", 12_700, "Solder Resist", ""},
	}
	if len(s.GetLayers()) != len(want) {
		t.Fatalf("%d rows %v, want %d", len(s.GetLayers()), s.GetLayers(), len(want))
	}
	for i, l := range s.GetLayers() {
		got := row{l.GetName(), l.GetKind(), l.GetLayer(), l.GetThickness(), l.GetMaterial(), l.GetDielectricType()}
		if got != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got, want[i])
		}
	}
	if e := s.GetLayers()[1].EpsilonR; e == nil || *e != 4.6 {
		t.Errorf("dielectric 1's constant = %v, want 4.6", e)
	}
	if s.Thickness != nil {
		t.Errorf("stated thickness = %d, want none, as Board6 states none", s.GetThickness())
	}
}
