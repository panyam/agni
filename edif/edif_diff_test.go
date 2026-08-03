package edif

import (
	"testing"

	"github.com/panyam/agni/diff"
)

// findChange returns the ComponentChange for (ref, field), or a zero value if absent.
func findChange(cs []diff.ComponentChange, ref, field string) (diff.ComponentChange, bool) {
	for _, c := range cs {
		if c.RefDes == ref && c.Field == field {
			return c, true
		}
	}
	return diff.ComponentChange{}, false
}

// netChangeByName indexes a report's net changes by the net's (new-side) name.
func netChangeByName(r *diff.Report) map[string]diff.NetChange {
	m := map[string]diff.NetChange{}
	for _, nc := range r.Nets {
		m[nc.Name] = nc
	}
	return m
}

// TestEDIFDiffRoundTrip drives the read->diff pipeline end to end over a hand-authored
// before/after EDIF netlist pair (WS6-002). It proves the reader hands diff a correct IR
// and that the full change taxonomy surfaces through the real reader: component
// added/removed/value-changed, and net new/deleted/renamed/hard. The in-package
// TestNetTaxonomy exercises the same taxonomy on a hand-built ir.Design (nets only); this
// closes the gap by covering the component-diff path and the reader->diff seam a
// synthetic Design cannot.
func TestEDIFDiffRoundTrip(t *testing.T) {
	a := readEDN(t, "rev_a.edn")
	b := readEDN(t, "rev_b.edn")
	r := diff.Designs(a, b)

	// Components: C2 added, C1 removed.
	if len(r.ComponentsAdded) != 1 || r.ComponentsAdded[0] != "C2" {
		t.Errorf("ComponentsAdded = %v, want [C2]", r.ComponentsAdded)
	}
	if len(r.ComponentsRemoved) != 1 || r.ComponentsRemoved[0] != "C1" {
		t.Errorf("ComponentsRemoved = %v, want [C1]", r.ComponentsRemoved)
	}

	// The only component-field change is R1's value 10k -> 22k; the multi-section U1 and
	// the unchanged R2 must not appear.
	if len(r.ComponentsChanged) != 1 {
		t.Fatalf("ComponentsChanged = %+v, want exactly one (R1 Value)", r.ComponentsChanged)
	}
	if c, ok := findChange(r.ComponentsChanged, "R1", "Value"); !ok || c.Old != "10k" || c.New != "22k" {
		t.Errorf("R1 Value change = %+v (ok=%v), want 10k -> 22k", c, ok)
	}

	nets := netChangeByName(r)

	// GND is identical in both revisions and must not be reported.
	if _, reported := nets["GND"]; reported {
		t.Error("GND is unchanged and must not be reported")
	}
	// VCC gains C2.1 -> Hard connectivity change.
	if nc := nets["VCC"]; nc.Kind != diff.NetHard || len(nc.Added) != 1 || nc.Added[0] != "C2.1" {
		t.Errorf("VCC = %+v, want Hard +[C2.1]", nc)
	}
	// SIGOLD -> SIGNEW: same connectivity, new name -> rename.
	if nc := nets["SIGNEW"]; nc.Kind != diff.NetRenamed || nc.OldName != "SIGOLD" {
		t.Errorf("SIGNEW = %+v, want Renamed from SIGOLD", nc)
	}
	// DEAD exists only in the old revision -> Deleted.
	if nc := nets["DEAD"]; nc.Kind != diff.NetDeleted {
		t.Errorf("DEAD = %+v, want Deleted", nc)
	}
	// BORN exists only in the new revision -> New.
	if nc := nets["BORN"]; nc.Kind != diff.NetNew {
		t.Errorf("BORN = %+v, want New", nc)
	}

	// Provenance survives the read on both sides of a rename (the seam the round-trip
	// proves that a hand-built Design would assume rather than exercise).
	if nc := nets["SIGNEW"]; nc.OldProv.GetSourceFile() != "rev_a.edn" || nc.NewProv.GetSourceFile() != "rev_b.edn" {
		t.Errorf("SIGNEW prov = old:%q new:%q, want rev_a.edn / rev_b.edn",
			nc.OldProv.GetSourceFile(), nc.NewProv.GetSourceFile())
	}
}
