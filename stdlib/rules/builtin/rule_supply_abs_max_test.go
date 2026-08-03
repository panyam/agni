package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// ldoSpec hand-builds a seeded spec for the fake part: abs-max VDD with a structured
// TA condition (machine-comparable by docs/20 rules).
func ldoSpec(mpn string, absMax float64) *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	return &parampb.PartSpec{
		Mpn:          mpn,
		Manufacturer: "Acme",
		Docs: []*parampb.SourceDoc{{
			Id: "ds", Title: "ACME-33 Rev B", Vendor: "Acme",
		}},
		Parameters: []*parampb.Parameter{{
			Name: "Supply voltage", Symbol: "VDD",
			LimitKind: parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
			Value:     &parampb.RangeValue{Max: f(absMax)},
			Unit:      "V",
			Conditions: []*parampb.Condition{
				{Symbol: "TA", Eq: f(25), Unit: "C", Raw: "TA = 25C"},
			},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{
				DocRef: "ds", Page: 4, TableOrFigure: "Absolute Maximum Ratings",
				Method: "hand", Confidence: 1,
			},
		}},
	}
}

// supplyDesign builds a one-part design: U1 (part LDO, pin 1 = VDD power_in) with its
// VDD pin on the given net, joined to a spec via the given identity channel. It is the
// builtin-package copy of the fixture (the check package keeps its own for the Available gate).
func supplyDesign(netName string, viaBomLine bool, mpn string) *ir.Design {
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{{
			Name: "LDO",
			Pins: []*ir.Pin{{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN}},
		}}}},
		Components: []*ir.Component{{
			RefDes:     "U1",
			Sections:   []*ir.ComponentSection{{PartRef: "LDO", LibraryRef: "lib"}},
			Attributes: map[string]string{},
			Prov:       &ir.Provenance{SourceFile: "t"},
		}},
		Nets: []*ir.Net{{
			Name:        netName,
			Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}},
			Prov:        &ir.Provenance{SourceFile: "t"},
		}},
	}
	if mpn != "" {
		if viaBomLine {
			d.Bom = []*ir.BomLine{{RefDes: []string{"U1"}, Mpn: mpn, Manufacturer: "Acme"}}
		} else {
			d.Components[0].Attributes["MPN"] = mpn
		}
	}
	return d
}

func runSupplyRule(t *testing.T, d *ir.Design, set param.ParamSet) []check.Finding {
	t.Helper()
	m := check.NewModelWithParams(d, nil, set)
	return check.Run(m, []*check.Rule{supplyExceedsAbsMax})
}

func TestSupplyExceedsAbsMax(t *testing.T) {
	set := param.ParamSet{"ACME-33": ldoSpec("ACME-33", 4.6)}

	fires := runSupplyRule(t, supplyDesign("+5V", false, "ACME-33"), set)
	if len(fires) != 1 {
		t.Fatalf("5V rail into 4.6V abs-max part: want 1 finding, got %d: %v", len(fires), fires)
	}
	f := fires[0]
	if f.Kind != check.KindComponent || f.Subject != "U1" {
		t.Errorf("finding subject = %s/%s, want component/U1", f.Kind, f.Subject)
	}
	for _, want := range []string{"+5V", "4.6", "ACME-33 Rev B", "page 4", "Absolute Maximum Ratings", "confidence 1"} {
		if !strings.Contains(f.Message, want) {
			t.Errorf("finding message missing %q: %s", want, f.Message)
		}
	}
	// The same Citation also travels structured, so a renderer need not parse the message (WS10-012).
	if dp := f.DatasheetProv; dp == nil {
		t.Fatal("supply-exceeds-abs-max finding must carry structured DatasheetProv")
	} else if dp.Doc != "ACME-33 Rev B" || dp.DocRef != "ds" || dp.Page != 4 ||
		dp.Section != "Absolute Maximum Ratings" || dp.Method != "hand" || dp.Confidence != 1 {
		t.Errorf("DatasheetProv not populated from the spec: %+v", dp)
	}

	if fs := runSupplyRule(t, supplyDesign("+3V3", false, "ACME-33"), set); len(fs) != 0 {
		t.Errorf("3.3V rail within abs-max: want silent, got %v", fs)
	}
}

func TestSupplyJoinIdentityChannels(t *testing.T) {
	set := param.ParamSet{"ACME-33": ldoSpec("ACME-33", 4.6)}
	if fs := runSupplyRule(t, supplyDesign("+5V", true, "ACME-33"), set); len(fs) != 1 {
		t.Errorf("BomLine identity channel: want 1 finding, got %v", fs)
	}
	if fs := runSupplyRule(t, supplyDesign("+5V", false, "acme-33"), set); len(fs) != 1 {
		t.Errorf("attribute identity is case-insensitive on mpn: want 1 finding, got %v", fs)
	}
}

func TestSupplySkipsNotFalsePasses(t *testing.T) {
	set := param.ParamSet{"ACME-33": ldoSpec("ACME-33", 4.6)}

	if fs := runSupplyRule(t, supplyDesign("+5V", false, ""), set); len(fs) != 0 {
		t.Errorf("component with no MPN: skip, got %v", fs)
	}
	if fs := runSupplyRule(t, supplyDesign("+5V", false, "OTHER-1"), set); len(fs) != 0 {
		t.Errorf("unseeded MPN: skip, got %v", fs)
	}
	if fs := runSupplyRule(t, supplyDesign("+5V", false, "ACME-33"), nil); len(fs) != 0 {
		t.Errorf("no seeded set at all: silent by construction, got %v", fs)
	}

	underspec := ldoSpec("ACME-33", 4.6)
	underspec.Parameters[0].ConditionCoverage = parampb.ConditionCoverage_CONDITION_COVERAGE_PARTIAL
	if fs := runSupplyRule(t, supplyDesign("+5V", false, "ACME-33"), param.ParamSet{"ACME-33": underspec}); len(fs) != 0 {
		t.Errorf("under-specified limit row: skip, got %v", fs)
	}

	rawCond := ldoSpec("ACME-33", 4.6)
	rawCond.Parameters[0].Conditions = []*parampb.Condition{{Symbol: "TA", Raw: "over operating range"}}
	if fs := runSupplyRule(t, supplyDesign("+5V", false, "ACME-33"), param.ParamSet{"ACME-33": rawCond}); len(fs) != 0 {
		t.Errorf("machine-incomparable limit row (text-only condition): skip, got %v", fs)
	}

	milli := ldoSpec("ACME-33", 4600)
	milli.Parameters[0].Unit = "mV"
	if fs := runSupplyRule(t, supplyDesign("+5V", false, "ACME-33"), param.ParamSet{"ACME-33": milli}); len(fs) != 0 {
		t.Errorf("non-V unit: skip (never ad-hoc convert), got %v", fs)
	}
}

func TestSupplyDedupsPerNet(t *testing.T) {
	d := supplyDesign("+5V", false, "ACME-33")
	d.Libraries[0].Parts[0].Pins = append(d.Libraries[0].Parts[0].Pins,
		&ir.Pin{Name: "VDDA", Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN})
	d.Nets[0].Connections = append(d.Nets[0].Connections, &ir.Connection{ComponentRef: "U1", PinRef: "2"})
	set := param.ParamSet{"ACME-33": ldoSpec("ACME-33", 4.6)}
	if fs := runSupplyRule(t, d, set); len(fs) != 1 {
		t.Errorf("two power-in pins on one rail: one finding per (component, net), got %v", fs)
	}
}

// TestParamProviderMockBackend proves the datasheet source is pluggable behind the
// ParamProvider seam: the model reaches specs through Lookup only, so an in-memory
// ProviderFunc mock (no textproto directory) drives a datasheet rule exactly as a ParamSet
// does. This is the seam a shared datasheet service later slots into.
func TestParamProviderMockBackend(t *testing.T) {
	spec := ldoSpec("ACME-33", 4.6)
	var lookups int
	mock := param.ProviderFunc(func(mpn string) *parampb.PartSpec {
		lookups++
		if mpn == "ACME-33" {
			return spec
		}
		return nil
	})
	m := check.NewModelWithParams(supplyDesign("+5V", false, "ACME-33"), nil, mock)
	if fs := check.Run(m, []*check.Rule{supplyExceedsAbsMax}); len(fs) != 1 {
		t.Fatalf("mock ParamProvider backend: want 1 finding, got %v", fs)
	}
	if lookups == 0 {
		t.Error("the model never consulted the provider")
	}
	// A nil provider (a model built without params) is silent, not a panic.
	if fs := check.Run(check.NewModel(supplyDesign("+5V", false, "ACME-33")), []*check.Rule{supplyExceedsAbsMax}); len(fs) != 0 {
		t.Errorf("nil provider: want silent, got %v", fs)
	}
}

// TestSupplyInputPinViaIngestionStamp pins the format-neutral supply-pin detection (WS3-072 PR2): a
// source that does not classify pin electrical type — EDIF marks a VDD pin plain INPUT — is recovered
// by the ingestion stamp (classify.StampPowerInPins), which promotes an under-typed supply-named input
// pin to POWER_IN. So the datasheet rail rules are not silently KiCad-only, but now via a plain
// PinDir == POWER_IN check (the WS3-036 name-role fallback is gone). The stamp is gated away from
// confident directions (OUTPUT) and non-supply names, so those stay silent.
func TestSupplyInputPinViaIngestionStamp(t *testing.T) {
	set := param.ParamSet{"ACME-33": ldoSpec("ACME-33", 4.6)}

	// EDIF-style: the VDD pin is typed INPUT (EDIF has no POWER_IN); the ingestion stamp promotes it
	// to POWER_IN, so a +5V rail over the 4.6V abs-max fires.
	edif := supplyDesign("+5V", false, "ACME-33")
	edif.Libraries[0].Parts[0].Pins[0].Direction = ir.PinDirection_PIN_DIRECTION_INPUT
	classify.StampPowerInPins(edif)
	if fs := runSupplyRule(t, edif, set); len(fs) != 1 {
		t.Errorf("INPUT-typed VDD pin (EDIF-style): want 1 finding after the POWER_IN stamp, got %v", fs)
	}

	// A rail the part SOURCES (OUTPUT-typed, even with a supply-ish name) is a confident direction the
	// stamp never overwrites, so it is not a consumed supply pin.
	out := supplyDesign("+5V", false, "ACME-33")
	out.Libraries[0].Parts[0].Pins[0].Direction = ir.PinDirection_PIN_DIRECTION_OUTPUT
	classify.StampPowerInPins(out)
	if fs := runSupplyRule(t, out, set); len(fs) != 0 {
		t.Errorf("OUTPUT-typed power pin: not a consumed rail, want silent, got %v", fs)
	}

	// A non-supply pin name (IO1) is not promoted even when typed INPUT, so it stays a plain input.
	sig := supplyDesign("+5V", false, "ACME-33")
	sig.Libraries[0].Parts[0].Pins[0].Name = "IO1"
	sig.Libraries[0].Parts[0].Pins[0].Direction = ir.PinDirection_PIN_DIRECTION_INPUT
	classify.StampPowerInPins(sig)
	if fs := runSupplyRule(t, sig, set); len(fs) != 0 {
		t.Errorf("INPUT-typed non-supply pin (IO1): not promoted, want silent, got %v", fs)
	}
}
