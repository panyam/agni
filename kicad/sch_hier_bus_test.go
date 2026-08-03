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

// TestHierBusMembersDoNotCross locks the oracle finding (WS1-034 Phase 2): a bus crossing a
// hierarchical sheet boundary via a bus sheet-pin does NOT connect its members across the boundary.
// kicad-cli keeps the parent member `/DATA0` (node R1) and the child member `/sub/DATA0` (node R101)
// as two distinct nets; we match. A prior "fix" that joined them would DISAGREE with kicad-cli, so
// this is a guard against re-introducing the invalidated premise.
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
		t.Errorf("bus members must NOT cross the hierarchical boundary (kicad-cli keeps them separate), got both %q", parent)
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
