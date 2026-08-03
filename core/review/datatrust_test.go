package review

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/datasheet/param"
)

// --- provisional: a fail on unratified datasheet data ---

// supplyModel builds a one-part design (U1, VDD power-in pin on a +5V rail) joined to a seeded spec
// whose VDD abs-max is 4.6 V, so supply-exceeds-abs-max fires; method/confidence drive the data-trust.
func supplyModel(method string, confidence float64) check.Model {
	f := func(v float64) *float64 { return &v }
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{{
			Name: "LDO",
			Pins: []*ir.Pin{{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN}},
		}}}},
		Components: []*ir.Component{{
			RefDes: "U1", Sections: []*ir.ComponentSection{{PartRef: "LDO", LibraryRef: "lib"}},
			Attributes: map[string]string{"MPN": "ACME-33"}, Prov: &ir.Provenance{SourceFile: "t"},
		}},
		Nets: []*ir.Net{{Name: "+5V", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "t"}}},
	}
	spec := &parampb.PartSpec{
		Mpn: "ACME-33", Manufacturer: "Acme",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "ACME-33 Rev B", Vendor: "Acme"}},
		Parameters: []*parampb.Parameter{{
			Name: "Supply voltage", Symbol: "VDD",
			LimitKind: parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
			Value:     &parampb.RangeValue{Max: f(4.6)}, Unit: "V",
			Conditions:        []*parampb.Condition{{Symbol: "TA", Eq: f(25), Unit: "C", Raw: "TA = 25C"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov:              &parampb.ParamProvenance{DocRef: "ds", Page: 4, TableOrFigure: "Absolute Maximum Ratings", Method: method, Confidence: confidence},
		}},
	}
	return check.NewModelWithParams(d, nil, param.ParamSet{"ACME-33": spec})
}

// TestProvisionalFromMockData: an item whose datasheet rule fires is Provisional when the seed is mock
// or below the confidence floor (a HITL ratification item, not a trustworthy fail), and a real Fail
// once the value is ratified (method "hand", high confidence).
func TestProvisionalFromMockData(t *testing.T) {
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "abs-max", Title: "supply within abs max", Binding: Binding{Rule: "supply-exceeds-abs-max"}},
	}}}}
	outcome := func(m check.Model, floor float64) Outcome {
		return Run(RunParams{Model: m, Catalog: check.DefaultCatalog(), Manifest: man, Design: "d", RatifiedFloor: floor}).Areas[0].Items[0].Outcome
	}
	if got := outcome(supplyModel("mock", 0.3), 0); got != Provisional {
		t.Errorf("mock-seeded fail: want provisional, got %s", got)
	}
	if got := outcome(supplyModel("derive/v0", 0.5), 0.9); got != Provisional {
		t.Errorf("below-floor fail: want provisional, got %s", got)
	}
	if got := outcome(supplyModel("hand", 1.0), 0); got != Fail {
		t.Errorf("ratified fail: want fail, got %s", got)
	}
}

// --- computed-n/a: a device-class-gated item whose class is absent ---

// TestComputedNAByDeviceClass: an item that applies_to_class a class no component carries resolves to
// computed-n/a (the mechanism determined it does not apply); with a matching part it runs normally.
func TestComputedNAByDeviceClass(t *testing.T) {
	item := Item{ID: "clk", Title: "crystal load caps", Binding: Binding{
		Rule: "crystal-load-caps", AppliesToClass: []string{"crystal", "ceramic_resonator"},
	}}
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{item}}}}
	run := func(d *ir.Design) ItemResult {
		return Run(RunParams{Model: check.NewModel(d), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d"}).Areas[0].Items[0]
	}
	// No clock part at all -> computed-n/a, not a silent pass.
	noClock := &ir.Design{Components: []*ir.Component{{RefDes: "R1"}}}
	if got := run(noClock).Outcome; got != ComputedNA {
		t.Errorf("no crystal/resonator part: want computed-n/a, got %s", got)
	}
	// A crystal is present -> the gate passes and the rule runs (a bare crystal with no caps fails,
	// proving the item is live, not n/a).
	withCrystal := &ir.Design{
		Components: []*ir.Component{{RefDes: "Y1", DeviceClasses: []string{"crystal", "clock"}, Prov: &ir.Provenance{SourceFile: "t"}}},
		Nets: []*ir.Net{
			{Name: "XIN", Connections: []*ir.Connection{{ComponentRef: "Y1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{Name: "XOUT", Connections: []*ir.Connection{{ComponentRef: "Y1", PinRef: "2"}}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
	}
	if got := run(withCrystal).Outcome; got == ComputedNA {
		t.Error("a crystal IS present: the item must run, not read computed-n/a")
	}
}

// --- needs-design-intent: an intent-bound item with no declaration ---

// TestNeedsDesignIntent: an item bound to an intent/ rule with no --intent-path declaration (so the rule
// is absent from the catalog) reads needs-design-intent, not the misleading not-automated.
func TestNeedsDesignIntent(t *testing.T) {
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "arch", Title: "expected modules present", Binding: Binding{Rule: "intent/module-missing"}},
	}}}}
	r := Run(RunParams{Model: check.NewModel(&ir.Design{}), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d"}).Areas[0].Items[0]
	if r.Outcome != NeedsDesignIntent {
		t.Errorf("intent-bound item, no declaration: want needs-design-intent, got %s", r.Outcome)
	}
	// A non-intent unshipped rule stays not-automated (the distinction is real, not a blanket relabel).
	man2 := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "x", Title: "some future check", Binding: Binding{Rule: "not-a-real-rule"}},
	}}}}
	if got := Run(RunParams{Model: check.NewModel(&ir.Design{}), Catalog: check.DefaultCatalog(), Manifest: man2, Design: "d"}).Areas[0].Items[0].Outcome; got != NotAutomated {
		t.Errorf("non-intent unshipped rule: want not-automated, got %s", got)
	}
}

// TestTallyCountsDataTrustStates: the Tally routes the new outcomes into their own buckets and Covered()
// counts everything but not-automated.
func TestTallyCountsDataTrustStates(t *testing.T) {
	var tl Tally
	for _, o := range []Outcome{Pass, Fail, Provisional, NeedsDesignIntent, ComputedNA, NotApplicable, NotAutomated} {
		tl.add(o)
	}
	if tl.Provisional != 1 || tl.NeedsDesignIntent != 1 || tl.ComputedNA != 1 {
		t.Errorf("data-trust counts = prov:%d ndi:%d cna:%d, want 1/1/1", tl.Provisional, tl.NeedsDesignIntent, tl.ComputedNA)
	}
	if tl.Covered() != 6 {
		t.Errorf("Covered() = %d, want 6 (all but not-automated)", tl.Covered())
	}
}
