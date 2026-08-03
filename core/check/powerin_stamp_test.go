package check

import (
	"testing"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestPowerInStampFlipsConnectivityRule (WS3-072 PR2) is the red-before-green for the EDIF flip: an
// EDIF-style VDD pin (typed INPUT) on an undriven net is invisible to power-input-not-driven today,
// because the rule keys on POWER_IN; the ingestion stamp promotes the pin, so the rule then fires.
// This is exactly the behavior the corpus delta measures at scale.
func TestPowerInStampFlipsConnectivityRule(t *testing.T) {
	mk := func() *ir.Design {
		return &ir.Design{
			Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{{
				Name: "IC", Pins: []*ir.Pin{{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_INPUT}},
			}}}},
			Components: []*ir.Component{{RefDes: "U1", Sections: []*ir.ComponentSection{{PartRef: "IC", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}}},
			Nets:       []*ir.Net{{Name: "VDD_UNDRIVEN", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "t"}}},
		}
	}

	// Before the stamp: the VDD pin reads INPUT, so the rule is silent (the EDIF gap PR2 closes).
	if fs := Run(NewModel(mk()), []*Rule{powerInputNotDriven}); len(fs) != 0 {
		t.Fatalf("pre-stamp (INPUT pin): want silent, got %v", fs)
	}

	// After the stamp: the VDD pin is POWER_IN with no driver on its net. The design's format types
	// power outputs (default), so the rule fires.
	d := mk()
	d.SourceFormat = "kicad-sch"
	classify.StampPowerInPins(d)
	if fs := Run(NewModel(d), []*Rule{powerInputNotDriven}); len(fs) != 1 {
		t.Fatalf("post-stamp (POWER_IN pin): want 1 finding, got %v", fs)
	}
}

// TestPowerInputNotDrivenGatedByFormat (WS3-072 PR2): the rule is gated OFF on a format that does not
// type power OUTPUTS (EDIF/IPC), because "no driver" there means the source is under-typed, not that
// the rail is unpowered — the switched/derived-rail false positive. On an output-typing format the same
// undriven power_in fires.
func TestPowerInputNotDrivenGatedByFormat(t *testing.T) {
	mk := func(format string) *ir.Design {
		return &ir.Design{
			SourceFormat: format,
			Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{{
				Name: "IC", Pins: []*ir.Pin{{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN}},
			}}}},
			Components: []*ir.Component{{RefDes: "U1", Sections: []*ir.ComponentSection{{PartRef: "IC", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}}},
			Nets:       []*ir.Net{{Name: "VDD", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "t"}}},
		}
	}
	if fs := Run(NewModel(mk("edif-2.0.0")), []*Rule{powerInputNotDriven}); len(fs) != 0 {
		t.Errorf("edif (no power-out typing): rule must be gated off, got %v", fs)
	}
	if fs := Run(NewModel(mk("kicad-sch")), []*Rule{powerInputNotDriven}); len(fs) != 1 {
		t.Errorf("kicad (types power-out): undriven power_in must fire, got %v", fs)
	}
}
