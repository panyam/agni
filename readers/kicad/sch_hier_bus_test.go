package kicad

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// pinNet maps (ref,pin) -> net name over a design's nets.
func pinNet(d *ir.Design, ref, pin string) string {
	for _, n := range d.GetNets() {
		for _, c := range n.GetConnections() {
			if c.GetComponentRef() == ref && c.GetPinRef() == pin {
				return n.GetName()
			}
		}
	}
	return ""
}

// TestHierBusMembersDoNotCross pins what `DATA[1:0]` means to KiCad, which is: nothing in
// particular. KiCad's vector-bus syntax is `PREFIX[first..last]`, so a label spelled with a colon is
// an ordinary scalar net name and the sheet pin carrying it is a scalar port. `DATA0` and `DATA1`
// are then unrelated local labels, kicad-cli keeps `/DATA0` (R1) and `/sub/DATA0` (R101) apart, and
// so do we.
//
// It reads as a claim about buses and is not one. This fixture was once taken as evidence that bus
// members never cross a sheet boundary, and that reading held up the fix for agni issue 561 for a
// while: change nothing here but the spelling, to `DATA[0..1]`, and kicad-cli joins the two halves
// into one net. TestBusVectorCrossesSheetBoundary is that fixture, against kicad-cli's own answer.
// The pair is only meaningful together — this one is the control.
func TestHierBusMembersDoNotCross(t *testing.T) {
	d, _, err := ReadSchematicHierarchyNets("hier_bus_root.kicad_sch", readFixture(t, "hier_bus_root.kicad_sch"), hierOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	parent := pinNet(d, "R1", "1")  // root member tap DATA0
	child := pinNet(d, "R101", "1") // sub-sheet member tap DATA0
	if parent != "DATA0" {
		t.Errorf("parent R1.1 net = %q, want bare DATA0 (root local)", parent)
	}
	if child != "/sub/DATA0" {
		t.Errorf("child R101.1 net = %q, want qualified /sub/DATA0", child)
	}
	if parent == child {
		t.Errorf("a colon-spelled label is not a KiCad bus, so these must stay separate, got both %q", parent)
	}
}

// TestHierBusMemberQualification pins fix (b): a bus's member names are qualified into the same
// net-name space as the sheet's member NETS, so bus-not-modeled resolves correctly per instance. On
// this fully-tapped fixture every member of every bus is an actual net, so the finding is silent; the
// sub-sheet bus's members are the qualified `/sub/DATAn`. Before the fix, collectBuses emitted bare
// members that never matched the sub-sheet's `/sub/DATAn` nets, so the flag false-fired.
func TestHierBusMemberQualification(t *testing.T) {
	d, _, err := ReadSchematicHierarchyNets("hier_bus_root.kicad_sch", readFixture(t, "hier_bus_root.kicad_sch"), hierOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	netSet := map[string]bool{}
	for _, n := range d.GetNets() {
		netSet[n.GetName()] = true
	}
	buses := d.GetInputDiagnostics().GetUnmodeledBuses()
	if len(buses) != 2 {
		t.Fatalf("want 2 detected buses (root + sub), got %d", len(buses))
	}
	sawQualified := false
	for _, b := range buses {
		for _, m := range b.GetMembers() {
			if !netSet[m] {
				t.Errorf("bus %q member %q is not a net; bus-not-modeled would false-fire (nets=%v)", b.GetLabel(), m, keys(netSet))
			}
			if m == "/sub/DATA0" || m == "/sub/DATA1" {
				sawQualified = true
			}
		}
	}
	if !sawQualified {
		t.Error("the sub-sheet bus's members should be qualified /sub/DATAn (fix b)")
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
