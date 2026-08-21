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
// tweak runs against the built spec before the model is constructed, for the cases that need a
// document revision or a verification record rather than just a method and a confidence.
func supplyModel(method string, confidence float64, tweak ...func(*parampb.PartSpec)) check.Model {
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
	for _, fn := range tweak {
		fn(spec)
	}
	return check.NewModelWithParams(d, nil, param.ParamSet{"ACME-33": spec})
}

// --- provisional: a fail on a verification the document has outrun ---

// A human verification is pinned to the revision it was performed against, so when the vendor ships a
// new one the confirmation stops being evidence about the document in hand. The value itself is
// unchanged and its confidence is 1.0, which is exactly what makes this dangerous: every signal that
// existed before the verification record reports a stale value as the most trustworthy data in the
// system, because a person really did check it once.
//
// This is the case the ratified floor could not see on its own. param.MarkVerified raises confidence
// to 1.0 and nothing lowers it afterwards, so a floor test alone rates a superseded verification as a
// hard Fail forever.
func TestStaleVerificationIsNotRatified(t *testing.T) {
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "abs-max", Title: "supply within abs max", Binding: Binding{Rule: "supply-exceeds-abs-max"}},
	}}}}
	outcome := func(m check.Model) Outcome {
		return Run(RunParams{Model: m, Catalog: check.DefaultCatalog(), Manifest: man, Design: "d", RatifiedFloor: 0.9}).Areas[0].Items[0].Outcome
	}

	// Verified against the revision the corpus holds: a trustworthy fail.
	// checkedRev is verified against first, then the corpus is moved to specHash: the same sequence a
	// re-seed produces, and the only one that leaves a verification pinned to a revision nobody holds.
	verifiedAgainst := func(specHash, checkedHash string) func(*parampb.PartSpec) {
		return func(s *parampb.PartSpec) {
			param.MarkVerified(s.Parameters[0], "sri",
				&parampb.SourceDoc{Id: "ds", Title: "ACME-33 " + checkedHash, ContentHash: checkedHash},
				"2026-08-13", "")
			s.Docs[0].ContentHash = specHash
		}
	}
	if got := outcome(supplyModel("hand", 0.95, verifiedAgainst("sha256:relB", "sha256:relB"))); got != Fail {
		t.Errorf("verified against the revision in hand: want %s, got %s", Fail, got)
	}

	// The vendor ships rev C and the corpus re-seeds. Nothing about the verification changed, and its
	// confidence is still 1.0, but it now describes a document nobody has.
	if got := outcome(supplyModel("hand", 0.95, verifiedAgainst("sha256:relC", "sha256:relB"))); got != Provisional {
		t.Errorf("verification of a superseded revision: want %s, got %s", Provisional, got)
	}

	// The corpus never recorded which revision it holds, so drift cannot be ruled out. A caller that
	// cannot check must not be told the answer is fine.
	if got := outcome(supplyModel("hand", 0.95, verifiedAgainst("", "sha256:relB"))); got != Provisional {
		t.Errorf("no revision to compare against: want %s, got %s", Provisional, got)
	}
}

// The converse, and the reason the fix is additive: a value nobody ever verified is judged exactly as
// it was before verification records existed. If this regressed, every hand-seeded fixture in the
// corpus would drop to Provisional at once.
func TestUnverifiedDataIsJudgedOnConfidenceAsBefore(t *testing.T) {
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "abs-max", Title: "supply within abs max", Binding: Binding{Rule: "supply-exceeds-abs-max"}},
	}}}}
	movedOn := func(s *parampb.PartSpec) { s.Docs[0].ContentHash = "sha256:relC" }
	got := Run(RunParams{
		Model: supplyModel("hand", 1.0, movedOn), Catalog: check.DefaultCatalog(),
		Manifest: man, Design: "d", RatifiedFloor: 0.9,
	}).Areas[0].Items[0].Outcome
	if got != Fail {
		t.Errorf("a document revision must not demote a value nobody claimed to have verified: got %s, want %s", got, Fail)
	}
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

// TestMultiCitationRatifiedOnlyIfEveryCitationIs pins the semantics WS3-028 forced when
// Finding.DatasheetProv became a slice: a finding is unratified if ANY of its citations fails the
// floor, because the conclusion rests on every value it joined and is only as trustworthy as the
// weakest one.
//
// The mixed case is the one that matters. A connection-aware finding whose abs-max was hand-read but
// whose output voltage came from a low-confidence extraction is exactly half-evidenced; rating it a
// hard Fail on the strength of the good citation alone is the false-fail this axis exists to prevent.
//
// Note this quantifier is the OPPOSITE of allUnratified's, deliberately: across findings one
// trustworthy finding is enough to make an item a real Fail, because they are independent claims.
func TestMultiCitationRatifiedOnlyIfEveryCitationIs(t *testing.T) {
	hand := &check.DatasheetCitation{Doc: "A", Method: "hand", Confidence: 1.0}
	weak := &check.DatasheetCitation{Doc: "B", Method: "derive/v0", Confidence: 0.3}
	mock := &check.DatasheetCitation{Doc: "C", Method: "mock", Confidence: 1.0}

	for _, c := range []struct {
		name  string
		cites []*check.DatasheetCitation
		want  bool
	}{
		{"no citations is a netlist finding, trustworthy by construction", nil, false},
		{"one ratified", []*check.DatasheetCitation{hand}, false},
		{"one below the floor", []*check.DatasheetCitation{weak}, true},
		{"both ratified", []*check.DatasheetCitation{hand, hand}, false},
		{"mixed: one weak citation taints the finding", []*check.DatasheetCitation{hand, weak}, true},
		{"mixed: order does not matter", []*check.DatasheetCitation{weak, hand}, true},
		{"mock alongside a hand-read value still taints", []*check.DatasheetCitation{hand, mock}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := check.Finding{Subject: check.Entity{Ref: "U1"}, Rule: "r", DatasheetProv: c.cites}
			if got := isUnratified(f, 0.8); got != c.want {
				t.Errorf("isUnratified = %v, want %v", got, c.want)
			}
		})
	}
}
