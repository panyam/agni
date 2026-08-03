package diff

import (
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// net builds an ir.Net named name, sourced from src (recorded in provenance), with the
// given "refdes.pin" connections.
func net(name, src string, conns ...string) *ir.Net {
	n := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: src}}
	for _, c := range conns {
		p := strings.SplitN(c, ".", 2)
		n.Connections = append(n.Connections, &ir.Connection{ComponentRef: p[0], PinRef: p[1]})
	}
	return n
}

// TestNetTaxonomy exercises the full net-change classification, rename detection, and
// provenance annotation on a hand-built revision pair.
func TestNetTaxonomy(t *testing.T) {
	a := &ir.Design{Nets: []*ir.Net{
		net("GND", "old", "R1.2", "R2.2"),     // unchanged  -> Equal (not reported)
		net("VCC", "old", "R1.1", "R2.1"),     // gains C1.1 -> Hard
		net("SIG_OLD", "old", "U1.1", "U2.1"), // renamed    -> SIG_NEW
		net("DEAD", "old", "R3.1"),            // removed    -> Deleted
		net("CLK", "old", "U1.5"),             // net_class change only -> Soft
	}}
	// CLK carries a net_class in old; changing only that (conns identical) is a Soft change.
	a.Nets[4].NetClass = "signal"

	b := &ir.Design{Nets: []*ir.Net{
		net("GND", "new", "R1.2", "R2.2"),
		net("VCC", "new", "R1.1", "R2.1", "C1.1"), // Hard: +C1.1
		net("SIG_NEW", "new", "U1.1", "U2.1"),     // same conns as SIG_OLD -> rename
		net("NEWNET", "new", "C1.2"),              // New
		net("CLK", "new", "U1.5"),                 // Soft: net_class differs
	}}
	b.Nets[4].NetClass = "power"

	r := Designs(a, b)

	byName := map[string]NetChange{}
	for _, nc := range r.Nets {
		byName[nc.Name] = nc
	}

	if len(r.Nets) != 5 {
		t.Fatalf("net changes = %d, want 5 (GND equal is excluded); got %+v", len(r.Nets), r.Nets)
	}
	if nc := byName["VCC"]; nc.Kind != NetHard || !eq(nc.Added, []string{"C1.1"}) {
		t.Errorf("VCC = %+v, want Hard +[C1.1]", nc)
	}
	if nc := byName["SIG_NEW"]; nc.Kind != NetRenamed || nc.OldName != "SIG_OLD" {
		t.Errorf("SIG_NEW = %+v, want Renamed from SIG_OLD", nc)
	}
	if nc := byName["NEWNET"]; nc.Kind != NetNew {
		t.Errorf("NEWNET = %+v, want New", nc)
	}
	if nc := byName["DEAD"]; nc.Kind != NetDeleted {
		t.Errorf("DEAD = %+v, want Deleted", nc)
	}
	if nc := byName["CLK"]; nc.Kind != NetSoft {
		t.Errorf("CLK = %+v, want Soft (net_class change)", nc)
	}
	if _, reported := byName["GND"]; reported {
		t.Error("GND is unchanged and must not be reported")
	}

	// Provenance is attached from both sides where each exists.
	if nc := byName["VCC"]; nc.OldProv.GetSourceFile() != "old" || nc.NewProv.GetSourceFile() != "new" {
		t.Errorf("VCC prov = old:%q new:%q, want old/new", nc.OldProv.GetSourceFile(), nc.NewProv.GetSourceFile())
	}
	if nc := byName["SIG_NEW"]; nc.OldProv.GetSourceFile() != "old" || nc.NewProv.GetSourceFile() != "new" {
		t.Errorf("rename prov = old:%q new:%q, want old/new", nc.OldProv.GetSourceFile(), nc.NewProv.GetSourceFile())
	}
	if nc := byName["NEWNET"]; nc.OldProv != nil || nc.NewProv.GetSourceFile() != "new" {
		t.Errorf("NEWNET prov = %+v, want old nil / new set", nc)
	}
}

// TestComponentFabricationDiff covers WS1-037: a dnp flip (populated -> not) and a footprint
// swap surface as component field changes, so a review diff shows a part dropped from the build
// or re-footprinted, not just a value edit.
func TestComponentFabricationDiff(t *testing.T) {
	comp := func(dnp, fp string) *ir.Component {
		return &ir.Component{
			RefDes:     "R1",
			Sections:   []*ir.ComponentSection{{LibraryRef: "Device", PartRef: "R"}},
			Attributes: map[string]string{"Value": "10k", "dnp": dnp, "Footprint": fp},
		}
	}
	a := &ir.Design{Components: []*ir.Component{comp("no", "R_0603")}}
	b := &ir.Design{Components: []*ir.Component{comp("yes", "R_0805")}}

	got := map[string]string{}
	for _, c := range Designs(a, b).ComponentsChanged {
		got[c.Field] = c.Old + "->" + c.New
	}
	if got["dnp"] != "no->yes" {
		t.Errorf("dnp change = %q, want no->yes (all=%v)", got["dnp"], got)
	}
	if got["Footprint"] != "R_0603->R_0805" {
		t.Errorf("Footprint change = %q, want R_0603->R_0805", got["Footprint"])
	}
	if _, ok := got["Value"]; ok {
		t.Errorf("Value must not be reported (unchanged); got %v", got)
	}
}

func eq(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
