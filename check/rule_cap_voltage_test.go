package check

import (
	"slices"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/datasheet/param"
)

// capSpec hand-builds a seeded cap: rated voltage as a machine-comparable
// recommended-operating row (the shape a cap datasheet's ratings table yields).
func capSpec(mpn string, rated float64) *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	return &parampb.PartSpec{
		Mpn:          mpn,
		Manufacturer: "Acme",
		Docs: []*parampb.SourceDoc{{
			Id: "ds", Title: "ACME-CAP Rev C", Vendor: "Acme",
		}},
		Parameters: []*parampb.Parameter{{
			Name: "Rated Voltage", Symbol: "VDC",
			LimitKind: parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING,
			Value:     &parampb.RangeValue{Max: f(rated)},
			Unit:      "V",
			Conditions: []*parampb.Condition{
				{Symbol: "TA", Eq: f(25), Unit: "C", Raw: "TA = 25C"},
			},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{
				DocRef: "ds", Page: 2, TableOrFigure: "Ratings",
				Method: "hand", Confidence: 1,
			},
		}},
	}
}

// capDesign places one capacitor C1 (pins 1/2 passive) with pin 1 on railNet and
// pin 2 on GND, joined via the MPN attribute.
func capDesign(railNet, mpn string) *ir.Design {
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{{
			Name: "C",
			Pins: []*ir.Pin{
				{Name: "~", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE},
				{Name: "~", Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE},
			},
		}}}},
		Components: []*ir.Component{{
			RefDes:     "C1",
			Sections:   []*ir.ComponentSection{{PartRef: "C", LibraryRef: "lib"}},
			Attributes: map[string]string{},
			Prov:       &ir.Provenance{SourceFile: "t"},
		}},
		Nets: []*ir.Net{
			{Name: railNet, Connections: []*ir.Connection{{ComponentRef: "C1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{Name: "GND", Connections: []*ir.Connection{{ComponentRef: "C1", PinRef: "2"}}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
	}
	if mpn != "" {
		d.Components[0].Attributes["MPN"] = mpn
	}
	return d
}

func runCapRule(t *testing.T, d *ir.Design, set param.ParamSet) []Finding {
	t.Helper()
	return Run(NewModelWithParams(d, nil, set), []*Rule{capVoltage})
}

func TestCapVoltageFires(t *testing.T) {
	set := param.ParamSet{"DEMO-CAP-6V3": capSpec("DEMO-CAP-6V3", 6.3)}
	fs := runCapRule(t, capDesign("+10V", "DEMO-CAP-6V3"), set)
	if len(fs) != 1 {
		t.Fatalf("6.3V-rated cap on a 10V rail: want 1 finding, got %v", fs)
	}
	f := fs[0]
	if f.Kind != KindComponent || f.Subject != "C1" {
		t.Errorf("subject = %s/%s, want component/C1", f.Kind, f.Subject)
	}
	for _, want := range []string{"DEMO-CAP-6V3", "+10V", "10V", "6.3V", "1.25", "12.5V",
		"ACME-CAP Rev C", "page 2", "Ratings", "confidence 1"} {
		if !strings.Contains(f.Message, want) {
			t.Errorf("message missing %q: %s", want, f.Message)
		}
	}
}

func TestCapVoltagePassesWithMargin(t *testing.T) {
	set := param.ParamSet{"DEMO-CAP-6V3": capSpec("DEMO-CAP-6V3", 6.3)}
	// 3.3 x 1.25 = 4.125 <= 6.3: comfortable.
	if fs := runCapRule(t, capDesign("+3V3", "DEMO-CAP-6V3"), set); len(fs) != 0 {
		t.Errorf("3V3 rail under a 6.3V rating: want silent, got %v", fs)
	}
	// 5 x 1.25 = 6.25 <= 6.3: the near-miss must still pass (float compare in the FFI).
	if fs := runCapRule(t, capDesign("+5V", "DEMO-CAP-6V3"), set); len(fs) != 0 {
		t.Errorf("5V rail x 1.25 = 6.25 within 6.3 rating: want silent, got %v", fs)
	}
	// Exactly at the derated bound passes (>=, not >): 8 x 1.25 = 10.
	set10 := param.ParamSet{"DEMO-CAP-10V": capSpec("DEMO-CAP-10V", 10)}
	if fs := runCapRule(t, capDesign("+8V", "DEMO-CAP-10V"), set10); len(fs) != 0 {
		t.Errorf("rated exactly rail x derate: want silent, got %v", fs)
	}
}

func TestCapVoltageWorstRailGoverns(t *testing.T) {
	set := param.ParamSet{"DEMO-CAP-6V3": capSpec("DEMO-CAP-6V3", 6.3)}
	d := capDesign("+3V3", "DEMO-CAP-6V3")
	d.Nets[1].Name = "+12V" // pin 2 moves from GND to a hot rail
	fs := runCapRule(t, d, set)
	if len(fs) != 1 || !strings.Contains(fs[0].Message, "+12V") {
		t.Fatalf("the worst connected rail must govern, got %v", fs)
	}
}

func TestCapVoltageNetAttributeBeatsName(t *testing.T) {
	set := param.ParamSet{"DEMO-CAP-6V3": capSpec("DEMO-CAP-6V3", 6.3)}
	d := capDesign("VBOOST", "DEMO-CAP-6V3") // no parseable nominal in the name
	d.Nets[0].Attributes = map[string]string{"max_voltage": "9"}
	fs := runCapRule(t, d, set)
	if len(fs) != 1 || !strings.Contains(fs[0].Message, "VBOOST") {
		t.Fatalf("a declared max_voltage attribute must supply the rail voltage, got %v", fs)
	}
}

func TestCapVoltageSkipsNotFalsePasses(t *testing.T) {
	rated := param.ParamSet{"DEMO-CAP-6V3": capSpec("DEMO-CAP-6V3", 6.3)}
	cases := []struct {
		name string
		d    *ir.Design
		set  param.ParamSet
	}{
		{"no MPN", capDesign("+10V", ""), rated},
		{"unseeded MPN", capDesign("+10V", "OTHER"), rated},
		{"no seeded set", capDesign("+10V", "DEMO-CAP-6V3"), nil},
		{"unknown rail voltage", capDesign("VBOOST", "DEMO-CAP-6V3"), rated},
	}
	for _, tc := range cases {
		if fs := runCapRule(t, tc.d, tc.set); len(fs) != 0 {
			t.Errorf("%s: want skip, got %v", tc.name, fs)
		}
	}

	mut := func(name string, f func(*parampb.PartSpec)) {
		s := capSpec("DEMO-CAP-6V3", 6.3)
		f(s)
		if fs := runCapRule(t, capDesign("+10V", "DEMO-CAP-6V3"), param.ParamSet{"DEMO-CAP-6V3": s}); len(fs) != 0 {
			t.Errorf("%s: want skip, got %v", name, fs)
		}
	}
	mut("no rated-voltage row", func(s *parampb.PartSpec) {
		s.Parameters[0].Symbol, s.Parameters[0].Name = "ESR", "Equivalent Series Resistance"
	})
	mut("under-specified row", func(s *parampb.PartSpec) {
		s.Parameters[0].ConditionCoverage = parampb.ConditionCoverage_CONDITION_COVERAGE_PARTIAL
	})
	mut("text-only condition", func(s *parampb.PartSpec) {
		s.Parameters[0].Conditions = []*parampb.Condition{{Symbol: "TA", Raw: "over operating range"}}
	})
	mut("non-V unit", func(s *parampb.PartSpec) { s.Parameters[0].Unit = "mV"; *s.Parameters[0].Value.Max = 6300 })

	// A non-capacitor with the same seeded MPN never fires (the class gate).
	d := capDesign("+10V", "DEMO-CAP-6V3")
	d.Components[0].RefDes = "U1"
	d.Nets[0].Connections[0].ComponentRef = "U1"
	d.Nets[1].Connections[0].ComponentRef = "U1"
	if fs := runCapRule(t, d, rated); len(fs) != 0 {
		t.Errorf("non-capacitor component: want skip, got %v", fs)
	}
}

// The WS3-004 fact-capture bullet is a tested property: the rule's Reads are DERIVED
// from the spec body plus the FFI's declaration, so the param join and the rail
// voltage appear as named relations without hand-maintained metadata.
func TestCapVoltageDerivedReads(t *testing.T) {
	for _, want := range []string{"param.cap_rated_voltage", "net.max_voltage", "component.mpn", "component.class"} {
		if !slices.Contains(capVoltage.Reads, want) {
			t.Errorf("derived Reads missing %q: %v", want, capVoltage.Reads)
		}
	}
	if !slices.Contains(capVoltage.Primitives, "param-join") {
		t.Errorf("derived Primitives missing param-join: %v", capVoltage.Primitives)
	}
	if ok, reason := Available(capVoltage, nil); ok || !strings.Contains(reason, "--params") {
		t.Errorf("cap-voltage must gate on the params layer at catalog level, got %v %q", ok, reason)
	}
}
