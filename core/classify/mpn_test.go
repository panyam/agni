package classify

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func mpnDesign(comp map[string]string, partAttrs map[string]string) *ir.Design {
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{{Name: "P1", Attributes: partAttrs}}}},
		Components: []*ir.Component{{
			RefDes:     "U1",
			Attributes: comp,
			Sections:   []*ir.ComponentSection{{LibraryRef: "lib", PartRef: "P1"}},
		}},
	}
}

func mpnOf(d *ir.Design) string { return d.Components[0].GetMpn() }

// TestStampMPNPrefersTheComponent. A part number stated on the placement is more specific than one
// stated on the part type, so it wins. This is the case where a shared part type is placed as several
// different orderable parts.
func TestStampMPNPrefersTheComponent(t *testing.T) {
	d := mpnDesign(map[string]string{"Manufacturer PN": "ON-THE-PART"}, map[string]string{"mpn": "ON-THE-TYPE"})
	StampMPN(d)
	if got := mpnOf(d); got != "ON-THE-PART" {
		t.Errorf("MPN = %q, want the component's own value to win", got)
	}
}

// TestStampMPNNeverOverwrites. A reader that already wrote the canonical key has made a confident
// statement, and C9's fill variant promotes only where the reader was under-specified.
func TestStampMPNNeverOverwrites(t *testing.T) {
	d := mpnDesign(map[string]string{"Manufacturer PN": "ALIAS"}, map[string]string{"mpn": "TYPE"})
	d.Components[0].Mpn = "EXPLICIT"
	StampMPN(d)
	if got := mpnOf(d); got != "EXPLICIT" {
		t.Errorf("MPN = %q, want the existing canonical value untouched", got)
	}
}

// TestStampMPNFallsBackToThePartType is the Telesis shape and the reason agni issue 519 existed: the
// number is on the type, the consumer reads the component, and before this pass the two never met.
func TestStampMPNFallsBackToThePartType(t *testing.T) {
	d := mpnDesign(nil, map[string]string{"mpn": "FROM-THE-TYPE"})
	StampMPN(d)
	if got := mpnOf(d); got != "FROM-THE-TYPE" {
		t.Errorf("MPN = %q, want the part type's value promoted onto the placement", got)
	}
}

// TestStampMPNInventsNothing. A design that states no part number anywhere must come back with none.
// An invented value is worse than an absent one: it joins to a real datasheet and produces confident
// findings about a part that is not on the board.
func TestStampMPNInventsNothing(t *testing.T) {
	d := mpnDesign(map[string]string{"Description": "a resistor"}, map[string]string{"Package": "0603"})
	StampMPN(d)
	if got := mpnOf(d); got != "" {
		t.Errorf("MPN = %q, want empty: nothing in this design states a part number", got)
	}
}

// TestStampMPNKeepsTheSourceSpelling. The canonical key is ADDED, never a rename, so a reader or a
// report inspecting the attribute its format actually wrote still finds it.
func TestStampMPNKeepsTheSourceSpelling(t *testing.T) {
	d := mpnDesign(map[string]string{"Manufacturer PN": "PARTX"}, nil)
	StampMPN(d)
	if got := d.Components[0].GetAttributes()["Manufacturer PN"]; got != "PARTX" {
		t.Errorf("source attribute = %q, want it left in place", got)
	}
}

// TestStampMPNIsIdempotent. Ingestion passes can run more than once over one design (a re-read, a
// test harness), and a second run must be a no-op rather than a second interpretation.
func TestStampMPNIsIdempotent(t *testing.T) {
	d := mpnDesign(nil, map[string]string{"mpn": "FROM-THE-TYPE"})
	StampMPN(d)
	first := mpnOf(d)
	StampMPN(d)
	if second := mpnOf(d); second != first {
		t.Errorf("second run changed the MPN from %q to %q", first, second)
	}
}

// TestStampMPNDegradesWhenThePassNeverRan is C9 requirement (c) stated as a test. A hand-authored IR
// that never went through ingestion keeps whatever it was built with, and the absence of the
// canonical key is not read as a fact about the design.
func TestStampMPNDegradesWhenThePassNeverRan(t *testing.T) {
	d := mpnDesign(nil, nil)
	d.Components[0].Mpn = "HAND-AUTHORED"
	if got := mpnOf(d); got != "HAND-AUTHORED" {
		t.Fatalf("MPN = %q before any pass, want the value the IR was built with", got)
	}
}

// TestStampMPNResolvesThroughTheSharedPartIndex. The reader-local version this replaced joined on the
// first section's bare PartRef, so a part whose DISPLAY NAME differs from its native id never
// resolved and its components silently kept no part number. PartIndex already handles that case for
// the class pass, and using it here means a component's class and its part number can never disagree
// about which part type it has.
func TestStampMPNResolvesThroughTheSharedPartIndex(t *testing.T) {
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{{
			Name:       "DisplayName",
			Attributes: map[string]string{"mpn": "BY-NATIVE-ID"},
			Prov:       &ir.Provenance{NativeId: "&NATIVEID"},
		}}}},
		Components: []*ir.Component{{
			RefDes:   "U1",
			Sections: []*ir.ComponentSection{{PartRef: "NATIVEID"}},
		}},
	}
	StampMPN(d)
	if got := mpnOf(d); got != "BY-NATIVE-ID" {
		t.Errorf("MPN = %q, want the part resolved by its native id alias", got)
	}
}
