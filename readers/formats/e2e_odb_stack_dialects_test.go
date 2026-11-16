package formats

import (
	"path/filepath"
	"testing"
)

// TestAllegroODBStacksSumToTheirStatedThickness holds each Allegro-written ODB++ job's stack to the
// board thickness the job states (agni 1660). pc069 writes its copper weight in micrometres, and
// zaius lists no dielectric, putting each one in the copper above it. Read either way wrongly, a
// stack misses its total by far: pc069 read 7.56 mm against 1.67. The masks and finishes the jobs
// leave out of their rows allow 5%.
func TestAllegroODBStacksSumToTheirStatedThickness(t *testing.T) {
	jobs, _ := filepath.Glob("../../tools/samples/odb/allegro/*/*.odb.tgz")
	if len(jobs) < 3 {
		t.Fatalf("found %d Allegro ODB++ jobs, want 3; run `make samples-oracle`", len(jobs))
	}
	for _, j := range jobs {
		_, g := readBoth(t, j)
		s := g.GetStackup()
		if s.Thickness == nil {
			t.Errorf("%s states no board thickness", filepath.Base(j))
			continue
		}
		sum, dielectrics := int64(0), 0
		for _, l := range s.GetLayers() {
			sum += l.GetThickness()
			if l.GetKind() == "dielectric" && l.GetThickness() > 0 {
				dielectrics++
			}
		}
		if d := abs(sum - s.GetThickness()); d*20 > s.GetThickness() || dielectrics == 0 {
			t.Errorf("%s: %d dielectric rows summing with the rest to %d nm, the job states %d", filepath.Base(j), dielectrics, sum, s.GetThickness())
		}
	}
}

// TestODXStatesItsBoardThickness reads the total parasdr's ODX states in its header, the only stack
// thickness an ODX file gives (agni 1660).
func TestODXStatesItsBoardThickness(t *testing.T) {
	g, err := (&Loader{}).BoardGeometry(parasdr + "parasdr_odb++.xml")
	if err != nil {
		t.Fatal(err)
	}
	if s := g.GetStackup(); s.Thickness == nil || s.GetThickness() != 1_520_000 {
		t.Errorf("stated thickness %v, want the header's 1.52 mm", s.Thickness)
	}
}
