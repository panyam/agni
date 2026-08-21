package intent

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// regCurrentSpec hand-builds a regulator spec stating an output current the way a real one does: a
// recommended-operating row, not an absolute maximum. A rule filtering to ABSOLUTE_MAX would find
// nothing on a real part, which is why OutputCurrentLimits does not constrain the kind.
func regCurrentSpec(mpn, symbol string, iout float64) *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	return &parampb.PartSpec{
		Mpn:          mpn,
		Manufacturer: "Acme",
		Docs:         []*parampb.SourceDoc{{Id: "ds", Title: mpn + " Rev A", Vendor: "Acme"}},
		Parameters: []*parampb.Parameter{{
			Name: "Output current", Symbol: symbol,
			LimitKind:         parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING,
			Value:             &parampb.RangeValue{Max: f(iout)},
			Unit:              "A",
			Conditions:        []*parampb.Condition{{Symbol: "TA", Eq: f(25), Unit: "C", Raw: "TA = 25C"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{
				DocRef: "ds", Page: 4, TableOrFigure: "Recommended Operating Conditions",
				Method: "hand", Confidence: 1,
			},
		}},
	}
}

// budgetDesign wires a regulator U1 and a load U2 onto the rail. When beadRef is non-empty a
// two-terminal part of that ref-des sits between them, so the regulator is one series crossing from
// the rail the load sits on.
func budgetDesign(rail, beadRef string) *ir.Design {
	d := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "U1", Attributes: map[string]string{"MPN": "ACME-REG"}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U2", Attributes: map[string]string{"MPN": "ACME-LOAD"}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
	}
	if beadRef == "" {
		d.Nets = []*ir.Net{{Name: rail, Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
			{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: "U2", PinRef: "1"},
		}}}
		return d
	}
	d.Components = append(d.Components, &ir.Component{RefDes: beadRef, Prov: &ir.Provenance{SourceFile: "t"}})
	d.Nets = []*ir.Net{
		{Name: rail + "_PRE", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
			{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: beadRef, PinRef: "1"},
		}},
		{Name: rail, Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
			{ComponentRef: beadRef, PinRef: "2"}, {ComponentRef: "U2", PinRef: "1"},
		}},
	}
	return d
}

// budgetModel attaches a seeded spec for the regulator rating iout amps under the given symbol.
func budgetModel(d *ir.Design, symbol string, iout float64) check.Model {
	return check.NewModelWithParams(d, nil, param.ParamSet{
		"ACME-REG": regCurrentSpec("ACME-REG", symbol, iout),
	})
}

// budgetDecl is a one-rail declaration with the given peak and margin factor (0 to omit the factor).
func budgetDecl(rail string, peak, factor float64) Declaration {
	return Declaration{
		Name:         "t",
		RailBudgets:  []RailBudget{{Rail: rail, Peak: peak}},
		MarginFactor: factor,
	}
}

// TestRailCapacityFiresWhenUnderRated is the WS3-095 acceptance for the capacity half: a rail declared
// to draw 0.8A peak, supplied by a regulator rated 0.5A, is over-subscribed by the design's own
// numbers. The finding names both figures and cites the datasheet the rating came from.
func TestRailCapacityFiresWhenUnderRated(t *testing.T) {
	m := budgetModel(budgetDesign("3V3", ""), "IOUT", 0.5)
	fs := railBudgetCapacityRule(budgetDecl("3V3", 0.8, 0)).Findings(m)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (0.5A supply on a 0.8A rail), got %d: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Subject != "3V3" || f.Kind != check.KindNet {
		t.Errorf("subject = (%s, %q), want the rail net 3V3", f.Kind, f.Subject)
	}
	for _, want := range []string{"0.8", "U1", "0.5", "IOUT"} {
		if !strings.Contains(f.Message, want) {
			t.Errorf("message missing %q: %s", want, f.Message)
		}
	}
	if len(f.DatasheetProv) != 1 || f.DatasheetProv[0].Doc != "ACME-REG Rev A" {
		t.Errorf("want one citation of the supplying part's datasheet, got %+v", f.DatasheetProv)
	}
}

// TestRailCapacitySilentWhenRated guards the comparison direction. A sign error would invert the rule
// while every assertion in the firing test still passed.
func TestRailCapacitySilentWhenRated(t *testing.T) {
	m := budgetModel(budgetDesign("3V3", ""), "IOUT", 1.0)
	if fs := railBudgetCapacityRule(budgetDecl("3V3", 0.8, 0)).Findings(m); len(fs) != 0 {
		t.Errorf("a 1.0A supply on a 0.8A rail must be silent, got %+v", fs)
	}
}

// TestRailMarginFiresInTheHeadroomBand is the acceptance for the margin half: 0.9A clears the 0.8A
// peak but falls short of the 0.96A the declared 1.2 factor asks for.
func TestRailMarginFiresInTheHeadroomBand(t *testing.T) {
	m := budgetModel(budgetDesign("3V3", ""), "IOUT", 0.9)
	fs := railBudgetMarginRule(budgetDecl("3V3", 0.8, 1.2)).Findings(m)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (0.9A against a 0.96A margin), got %d: %+v", len(fs), fs)
	}
	for _, want := range []string{"0.8", "1.2", "0.96", "0.9"} {
		if !strings.Contains(fs[0].Message, want) {
			t.Errorf("message missing %q: %s", want, fs[0].Message)
		}
	}
}

// TestRailUnderPeakFiresCapacityOnlyNotMargin is the reason these are TWO rules and not one at two
// thresholds. A supply below the peak is below the margin too, so a naive margin rule would fire as
// well and a reviewer would work two items for one defect. Capacity owns that range; margin declines
// it. The named case the plan calls out, held by name so a refactor cannot quietly merge them.
func TestRailUnderPeakFiresCapacityOnlyNotMargin(t *testing.T) {
	m := budgetModel(budgetDesign("3V3", ""), "IOUT", 0.5)
	d := budgetDecl("3V3", 0.8, 1.2)
	if fs := railBudgetCapacityRule(d).Findings(m); len(fs) != 1 {
		t.Fatalf("capacity must fire below the peak, got %d: %+v", len(fs), fs)
	}
	if fs := railBudgetMarginRule(d).Findings(m); len(fs) != 0 {
		t.Errorf("margin must stay silent below the peak (capacity's range), got %+v", fs)
	}
}

// TestRailMarginSilentAtExactlyTheMargin: a supply rated at exactly the margin meets it and must not
// fire. The values matter. 0.1 x 1.5 is 0.15000000000000002 in float64 while the literal 0.15 is
// 0.1499999999999999944, so a 150mA part on a 100mA rail at a 1.5 factor fails a margin it meets
// exactly unless below() carries its relative tolerance. Picking a pair that happens to round the
// other way (0.8 x 1.2 is one) would make this test pass with the tolerance deleted, which is the
// wrong reason to be green.
func TestRailMarginSilentAtExactlyTheMargin(t *testing.T) {
	m := budgetModel(budgetDesign("3V3", ""), "IOUT", 0.15)
	if fs := railBudgetMarginRule(budgetDecl("3V3", 0.1, 1.5)).Findings(m); len(fs) != 0 {
		t.Errorf("a supply rated at exactly the margin must be silent, got %+v", fs)
	}
	// The same rail one milliamp short still fires, so the tolerance has not blunted the check.
	short := budgetModel(budgetDesign("3V3", ""), "IOUT", 0.149)
	if fs := railBudgetMarginRule(budgetDecl("3V3", 0.1, 1.5)).Findings(short); len(fs) != 1 {
		t.Errorf("a supply genuinely short of the margin must still fire, got %+v", fs)
	}
}

// TestRailBudgetAcrossSeriesElement: a ferrite between the regulator and the rail is ordinary layout
// and must not hide the supply, so the association walks one series crossing.
func TestRailBudgetAcrossSeriesElement(t *testing.T) {
	m := budgetModel(budgetDesign("3V3", "FB1"), "IOUT", 0.5)
	if fs := railBudgetCapacityRule(budgetDecl("3V3", 0.8, 0)).Findings(m); len(fs) != 1 {
		t.Errorf("want the finding to survive one series crossing, got %d: %+v", len(fs), fs)
	}
}

// TestRailBudgetSymbolAliases: the vendor spelling lives in the model layer, so a spec printing I_OUT
// or IOUT(MAX) is read exactly like one printing IOUT. A rule that matched one spelling would go
// silent on most real datasheets, which reads as a clean design.
func TestRailBudgetSymbolAliases(t *testing.T) {
	for _, sym := range []string{"IOUT", "I_OUT", "IOUT(MAX)", "IO", "ICONT"} {
		t.Run(sym, func(t *testing.T) {
			m := budgetModel(budgetDesign("3V3", ""), sym, 0.5)
			if fs := railBudgetCapacityRule(budgetDecl("3V3", 0.8, 0)).Findings(m); len(fs) != 1 {
				t.Errorf("symbol %q should be read as an output current, got %d findings", sym, len(fs))
			}
		})
	}
	// A current the rule must NOT credit as capacity: a quiescent current is not what the part
	// delivers, and reading it would compare a rail's demand against an unrelated number.
	m := budgetModel(budgetDesign("3V3", ""), "IQ", 0.5)
	if fs := railBudgetCapacityRule(budgetDecl("3V3", 0.8, 0)).Findings(m); len(fs) != 0 {
		t.Errorf("a quiescent-current row must not be read as output capacity, got %+v", fs)
	}
}

// TestRailBudgetReadsMilliampRatings is the rule half of agni issue 148, and the failure it closes is
// the one the whole outcome vocabulary exists to prevent: a wrong PASS on a power rail.
//
// MILLIAMPS ARE THE ORDINARY SPELLING for a sub-amp regulator, so a spec transcribed as the sheet
// prints it hit this without doing anything unusual. The row used to fail the extractor's unit gate,
// which left the rule with no supply for the rail, nothing to compare, and no finding. Neither guard
// caught it: check.Available saw a params tier attached, and the needs-data gate saw the symbol seeded.
// The item scored a clean pass on a rail the design over-subscribes by 60%.
//
// Both directions are asserted. A 500mA part under an 0.8A budget must fire, and the same part under a
// 0.3A budget must not, because a conversion that fired unconditionally would look identical to a
// correct one on the first case alone.
func TestRailBudgetReadsMilliampRatings(t *testing.T) {
	milliamps := func() *parampb.PartSpec {
		spec := regCurrentSpec("ACME-REG", "IOUT", 500)
		spec.Parameters[0].Unit = "mA"
		return spec
	}

	d := budgetDesign("3V3", "")
	m := check.NewModelWithParams(d, nil, param.ParamSet{"ACME-REG": milliamps()})
	fs := railBudgetCapacityRule(budgetDecl("3V3", 0.8, 0)).Findings(m)
	if len(fs) != 1 {
		t.Fatalf("a 500mA supply under an 0.8A budget must fire exactly once, got %d: %+v", len(fs), fs)
	}
	// The message quotes the rating in the base unit the extractor returns, so a reviewer reads the
	// same number the comparison used rather than converting it back themselves.
	if !strings.Contains(fs[0].Message, "0.5A") {
		t.Errorf("finding must quote the converted rating: %s", fs[0].Message)
	}

	// The volt-spelled twin of the same part must reach the identical verdict, which is the property
	// that makes the conversion a normalization rather than a new comparison.
	inAmps := check.NewModelWithParams(d, nil, param.ParamSet{"ACME-REG": regCurrentSpec("ACME-REG", "IOUT", 0.5)})
	amps := railBudgetCapacityRule(budgetDecl("3V3", 0.8, 0)).Findings(inAmps)
	if len(amps) != 1 || amps[0].Message != fs[0].Message {
		t.Errorf("the mA and A spellings of one row must report identically:\n  mA: %+v\n   A: %+v", fs, amps)
	}

	// A budget the part genuinely covers stays silent, so the conversion is not simply firing always.
	if within := railBudgetCapacityRule(budgetDecl("3V3", 0.3, 0)).Findings(m); len(within) != 0 {
		t.Errorf("a 500mA supply covers an 0.3A budget and must stay silent, got %+v", within)
	}
}

// TestRailBudgetSkipsUnrecognizedUnits: converting the units the layer knows must not soften its
// refusal of the ones it does not. An unrecognized unit still yields no supply, and the residual the
// old milliamp case documented survives here unchanged: the symbol IS seeded, so needs-data does not
// cover it and the item reads pass. That is now a genuinely narrow gap (a vendor unit no SI scale
// applies to) rather than the everyday milliamp row it used to be.
func TestRailBudgetSkipsUnrecognizedUnits(t *testing.T) {
	d := budgetDesign("3V3", "")
	spec := regCurrentSpec("ACME-REG", "IOUT", 0.5)
	spec.Parameters[0].Unit = "dBm"
	m := check.NewModelWithParams(d, nil, param.ParamSet{"ACME-REG": spec})
	if fs := railBudgetCapacityRule(budgetDecl("3V3", 0.8, 0)).Findings(m); len(fs) != 0 {
		t.Errorf("an unrecognized unit must be skipped, never scaled by a guess, got %+v", fs)
	}
	if !check.SeedsAnySymbol(m, check.OutputCurrentSymbols()) {
		t.Error("the symbol IS seeded here, so needs-data is not what covers this case")
	}
}

// TestRailBudgetSilentWithoutTheDatasheet: with no seeded spec there is no rating, so the rule has
// nothing to compare and must not fire. It reports the rail as unevaluable rather than clean, and the
// unevaluable half is the review runner's needs-data gate (TestRuleBoundDatasheetItemNeedsData in
// core/review), which these rules feed by declaring ParamSymbols.
func TestRailBudgetSilentWithoutTheDatasheet(t *testing.T) {
	m := check.NewModel(budgetDesign("3V3", ""))
	if fs := railBudgetCapacityRule(budgetDecl("3V3", 0.8, 0)).Findings(m); len(fs) != 0 {
		t.Errorf("want no findings with no seeded params, got %+v", fs)
	}
	if ok, reason := check.Available(railBudgetCapacityRule(budgetDecl("3V3", 0.8, 0)), m); ok || reason == "" {
		t.Errorf("Available = (%v, %q), want not-applicable with a reason", ok, reason)
	}
	// ParamSymbols is what lets the review runner tell "within budget" from "nothing states a current".
	if len(railBudgetCapacityRule(budgetDecl("3V3", 0.8, 0)).ParamSymbols) == 0 {
		t.Error("the capacity rule must declare the datasheet symbols it joins on")
	}
	if len(railBudgetMarginRule(budgetDecl("3V3", 0.8, 1.2)).ParamSymbols) == 0 {
		t.Error("the margin rule must declare the datasheet symbols it joins on")
	}
}

// TestRailBudgetSilentOnUndeclaredAndAbsentRails holds the direction of the iteration. The rule walks
// the DECLARATION and probes the design: a rail the design carries but the intent does not budget is
// not its business, and a budgeted rail the design does not carry is a missing-rail defect the
// voltage-domain and subsystem forms report, so firing here would report one defect twice.
func TestRailBudgetSilentOnUndeclaredAndAbsentRails(t *testing.T) {
	m := budgetModel(budgetDesign("3V3", ""), "IOUT", 0.5)
	if fs := railBudgetCapacityRule(budgetDecl("1V8", 0.8, 0)).Findings(m); len(fs) != 0 {
		t.Errorf("a budgeted rail the design does not carry must be silent, got %+v", fs)
	}
	if fs := railBudgetCapacityRule(Declaration{Name: "t"}).Findings(m); len(fs) != 0 {
		t.Errorf("a design rail nobody budgeted must be silent, got %+v", fs)
	}
}

// TestRailBudgetTakesTheBestSupply: with two seeded parts reaching one rail the rule reads the
// highest rating. The reach radius can pull in a part that does not feed this rail, and a
// multi-channel regulator states a rating per channel with no way to say which channel this net is, so
// reading the smallest would report a shortfall the design does not have. Every fail must be a genuine
// defect, so ambiguous evidence takes the reading that does not fire.
func TestRailBudgetTakesTheBestSupply(t *testing.T) {
	d := budgetDesign("3V3", "")
	d.Components = append(d.Components, &ir.Component{
		RefDes: "U3", Attributes: map[string]string{"MPN": "ACME-REG2"}, Prov: &ir.Provenance{SourceFile: "t"},
	})
	d.Nets[0].Connections = append(d.Nets[0].Connections, &ir.Connection{ComponentRef: "U3", PinRef: "1"})
	m := check.NewModelWithParams(d, nil, param.ParamSet{
		"ACME-REG":  regCurrentSpec("ACME-REG", "IOUT", 0.5),
		"ACME-REG2": regCurrentSpec("ACME-REG2", "IOUT", 1.5),
	})
	if fs := railBudgetCapacityRule(budgetDecl("3V3", 0.8, 0)).Findings(m); len(fs) != 0 {
		t.Errorf("the 1.5A supply covers the 0.8A budget, so the rail must be silent, got %+v", fs)
	}
}

// TestMarginRuleNotCompiledWithoutAFactor is the no-default guard. margin_factor is house policy, so
// with none declared the rule is not compiled at all and a bound review item reads needs-design-intent
// rather than passing against a number nobody stated. Compiling it with a default would be the silent
// false pass this family of checks exists to prevent.
func TestMarginRuleNotCompiledWithoutAFactor(t *testing.T) {
	names := func(d Declaration) map[string]bool {
		out := map[string]bool{}
		for _, r := range Compile(d) {
			out[r.Name] = true
		}
		return out
	}
	no := names(budgetDecl("3V3", 0.8, 0))
	if !no[RuleRailCurrentCapacity] {
		t.Error("budgets alone must still compile the capacity rule")
	}
	if no[RuleRailCurrentMargin] {
		t.Error("no margin_factor must leave the margin rule uncompiled (no default policy in a rule literal)")
	}
	with := names(budgetDecl("3V3", 0.8, 1.2))
	if !with[RuleRailCurrentCapacity] || !with[RuleRailCurrentMargin] {
		t.Errorf("a declared factor must compile both rules, got %v", with)
	}
	// No budgets at all compiles neither, so a declaration about something else entirely does not drag
	// a silently-passing sizing rule into the catalog.
	none := names(Declaration{Name: "t", Modules: []Module{{Name: "MCU", Class: "soc"}}})
	if none[RuleRailCurrentCapacity] || none[RuleRailCurrentMargin] {
		t.Errorf("no rail_budgets must compile no sizing rule, got %v", none)
	}
}
