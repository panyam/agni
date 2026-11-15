package formats

import (
	"slices"
	"testing"

	"github.com/panyam/agni/core/stackup"
)

// TestParasdrsIPC2581Stackup reads the stack Allegro wrote into parasdr's IPC-2581 export (agni
// 1592): six copper layers between five dielectrics, each with its Spec's material and electrical
// values, and the 1.52 mm board those rows add up to. The air rows at either surface are left out.
// Allegro marks every dielectric DIELCORE, its prepregs included, and the read keeps its word.
func TestParasdrsIPC2581Stackup(t *testing.T) {
	g, err := (&Loader{}).BoardGeometry(parasdr + "parasdr_ipc2581.xml")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range g.GetStackup().GetLayers() {
		got = append(got, stackup.Describe(l, g.GetUnitNm()))
	}
	want := []string{
		"F.Cu copper 0.04 mm CU_40UM_PLAT_EXT er=2.8 tand=0",
		"DIELECTRIC_BELOW_TOP dielectric 0.1 mm HFFR4_GENERIC_PREG core er=4.6 tand=0",
		"In1.Cu copper 0.035 mm CU_35UM_NPLAT_INT er=4.5 tand=0",
		"DIELECTRIC_BELOW_L2-GND dielectric 0.5 mm HFFR4_500UM_LMT core er=4.6 tand=0",
		"In2.Cu copper 0.035 mm CU_35UM_NPLAT_INT er=4.5 tand=0",
		"DIELECTRIC_BELOW_L3-X dielectric 0.1 mm HFFR4_GENERIC_PREG core er=4.6 tand=0",
		"In3.Cu copper 0.035 mm CU_35UM_NPLAT_INT er=4.5 tand=0",
		"DIELECTRIC_BELOW_L4-Y dielectric 0.5 mm HFFR4_500UM_LMT core er=4.6 tand=0",
		"In4.Cu copper 0.035 mm CU_35UM_NPLAT_INT er=4.5 tand=0",
		"DIELECTRIC_BELOW_L5-GND dielectric 0.1 mm HFFR4_GENERIC_PREG core er=4.6 tand=0",
		"B.Cu copper 0.04 mm CU_40UM_PLAT_EXT er=2.8 tand=0",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("stack rows:\n got %q\nwant %q", got, want)
	}
	if s := g.GetStackup(); s.Thickness == nil || s.GetThickness() != 1_520_000 {
		t.Fatalf("board thickness = %v, want 1520000 nm", s.Thickness)
	}
	if msg := stackup.Check(g.GetStackup(), g.GetUnitNm()); msg != "" {
		t.Errorf("Check = %q, want agreement", msg)
	}
}
