package classify

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestPartIndexAliasesNativeId (WS1-045): a section that references its part by the source's NATIVE ID
// (not the PartType's display name) still resolves, via the id alias PartIndex adds. This is the shape
// an EDIF `(rename ID "Display")` cell produces — the PartType is named by Display, the section's
// PartRef is the ID — and without the alias the part's pins were silently dropped. The display-name key
// still wins, and an empty/equal native id is a no-op.
func TestPartIndexAliasesNativeId(t *testing.T) {
	d := &ir.Design{Libraries: []*ir.PartLibrary{{Name: "Oscillator", Parts: []*ir.PartType{{
		Name: "OSC.0000C1",                              // display
		Prov: &ir.Provenance{NativeId: "OSC00500000C1"}, // id (differs; no & escape)
		Pins: []*ir.Pin{{Name: "Vcc", Designator: "4"}},
	}}}}}
	idx := PartIndex(d)

	// Resolves by the display name (unchanged) AND by the native id (the new alias), in both the
	// qualified and loose forms.
	for _, key := range []string{"Oscillator/OSC.0000C1", "/OSC.0000C1", "Oscillator/OSC00500000C1", "/OSC00500000C1"} {
		if idx[key] == nil {
			t.Errorf("PartIndex missing key %q", key)
		}
	}
	// FirstPart resolves a component whose section names the part by its native id.
	c := &ir.Component{RefDes: "X1", Sections: []*ir.ComponentSection{{LibraryRef: "Oscillator", PartRef: "OSC00500000C1"}}}
	if pt := FirstPart(idx, c); pt == nil || len(pt.Pins) != 1 {
		t.Fatalf("FirstPart via native id = %+v, want the Vcc part", pt)
	}
}

// TestPartIndexNativeIdNeverClobbersName (WS1-045): the id alias is a guarded FALLBACK — if one part's
// native id collides with another part's display name, the real display-name key wins.
func TestPartIndexNativeIdNeverClobbersName(t *testing.T) {
	d := &ir.Design{Libraries: []*ir.PartLibrary{{Name: "L", Parts: []*ir.PartType{
		{Name: "SHARED", Prov: &ir.Provenance{NativeId: "other"}, Pins: []*ir.Pin{{Name: "A"}}},
		{Name: "P2", Prov: &ir.Provenance{NativeId: "SHARED"}, Pins: []*ir.Pin{{Name: "B"}}},
	}}}}
	idx := PartIndex(d)
	if pt := idx["/SHARED"]; pt == nil || pt.Name != "SHARED" {
		t.Errorf("/SHARED resolved to %v, want the part actually NAMED SHARED (display wins the alias)", pt)
	}
}
