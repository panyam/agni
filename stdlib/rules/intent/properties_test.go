package intent

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// propDesign wires NET to a part U1, plus optional bias/coupling parts. biasTo names the net a
// resistor R1 bridges NET to (a rail or a ground); capTo names the net a capacitor C1 bridges it to.
// Either may be empty.
func propDesign(net, biasTo, capTo string) *ir.Design {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}}},
		Nets: []*ir.Net{{
			Name: net, Prov: &ir.Provenance{SourceFile: "t"},
			Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}},
		}},
	}
	add := func(ref, other string) {
		d.Components = append(d.Components, &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}})
		d.Nets[0].Connections = append(d.Nets[0].Connections, &ir.Connection{ComponentRef: ref, PinRef: "1"})
		d.Nets = append(d.Nets, &ir.Net{
			Name: other, Prov: &ir.Provenance{SourceFile: "t"},
			Connections: []*ir.Connection{{ComponentRef: ref, PinRef: "2"}},
		})
	}
	if biasTo != "" {
		add("R1", biasTo)
	}
	if capTo != "" {
		add("C1", capTo)
	}
	return d
}

func propFindings(t *testing.T, d *ir.Design, p NetProperty) []check.Finding {
	t.Helper()
	return propertyRule(p.Property, []NetProperty{p}).Findings(check.NewModel(d))
}

// TestResetPolarityFiresOnContradiction (WS3-088): a net declared active-low that is biased LOW is
// held in reset from power-up. That contradiction is the whole check.
func TestResetPolarityFiresOnContradiction(t *testing.T) {
	fs := propFindings(t, propDesign("SYS_RESET_N", "GND", ""),
		NetProperty{Net: "SYS_RESET_N", Property: PropResetPolarity, Value: "low"})
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (active-low reset pulled down), got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "biased LOW") {
		t.Errorf("message should name the contradiction: %s", fs[0].Message)
	}

	// The mirror image: active-high pulled up is equally held asserted.
	fs = propFindings(t, propDesign("PHY_EN", "+3V3", ""),
		NetProperty{Net: "PHY_EN", Property: PropResetPolarity, Value: "high"})
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (active-high pulled up), got %d: %+v", len(fs), fs)
	}
}

// TestResetPolarityAgreesSilently: the correct bias produces nothing. Guards the direction, which an
// inverted comparison would flip while every other assertion still passed.
func TestResetPolarityAgreesSilently(t *testing.T) {
	if fs := propFindings(t, propDesign("SYS_RESET_N", "+3V3", ""),
		NetProperty{Net: "SYS_RESET_N", Property: PropResetPolarity, Value: "low"}); len(fs) != 0 {
		t.Errorf("active-low reset pulled UP is correct, want silence: %+v", fs)
	}
}

// TestResetPolarityReportsWhatItCannotDecide (agni issue 74): a reset driven by a supervisor with an
// internal pull carries no bias resistor, so the netlist cannot state its resting level.
//
// This test previously asserted SILENCE, and the rule's doc card, its declaration comment and the
// test's own name all had to explain that a resulting pass meant "no contradiction found" rather than
// "polarity confirmed". The rule now says that itself, as an inconclusive finding, so a bound review
// item reads inconclusive instead of pass and the caveat does not have to survive in prose.
//
// The finding must be Inconclusive, not a defect: the design is not wrong, it is unverifiable from
// this evidence, and failing it would report a non-defect on every correct board whose reset is
// driven by a part with an internal pull.
func TestResetPolarityReportsWhatItCannotDecide(t *testing.T) {
	fs := propFindings(t, propDesign("SYS_RESET_N", "", ""),
		NetProperty{Net: "SYS_RESET_N", Property: PropResetPolarity, Value: "low"})
	if len(fs) != 1 {
		t.Fatalf("want 1 inconclusive finding (no bias evidence), got %d: %+v", len(fs), fs)
	}
	if !fs[0].Inconclusive {
		t.Error("no bias evidence is not a defect; the design is unverifiable, not wrong")
	}
	if !strings.Contains(fs[0].Message, "no bias") {
		t.Errorf("the message must name what could not be resolved: %s", fs[0].Message)
	}
}

// TestResetPolarityDividerSaysSoSpecifically: check.NetBias answers "neither" for two different
// designs, and the message must tell them apart. A net with no bias resistor sends a reviewer looking
// for a driver; a DIVIDER sends them to check a ratio against the part's input thresholds. Reporting
// "carries no bias" on a board that visibly has two resistors would waste that trip and read as a bug
// in the tool.
func TestResetPolarityDividerSaysSoSpecifically(t *testing.T) {
	d := propDesign("SYS_RESET_N", "GND", "")
	d.Components = append(d.Components, &ir.Component{RefDes: "R2", Prov: &ir.Provenance{SourceFile: "t"}})
	d.Nets[0].Connections = append(d.Nets[0].Connections, &ir.Connection{ComponentRef: "R2", PinRef: "1"})
	d.Nets = append(d.Nets, &ir.Net{
		Name: "+3V3", Prov: &ir.Provenance{SourceFile: "t"},
		Connections: []*ir.Connection{{ComponentRef: "R2", PinRef: "2"}},
	})
	for _, v := range []string{"low", "high"} {
		fs := propFindings(t, d, NetProperty{Net: "SYS_RESET_N", Property: PropResetPolarity, Value: v})
		if len(fs) != 1 || !fs[0].Inconclusive {
			t.Fatalf("a divider is undecidable, not a contradiction (value=%s): %+v", v, fs)
		}
		if !strings.Contains(fs[0].Message, "divider") {
			t.Errorf("value=%s: the message must name the divider, not claim there is no bias: %s", v, fs[0].Message)
		}
	}
}

func TestACCoupledFiresWhenDCConnected(t *testing.T) {
	fs := propFindings(t, propDesign("PCIE_TX0_P", "", ""),
		NetProperty{Net: "PCIE_TX0_P", Property: PropACCoupled})
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (declared AC-coupled, no series cap), got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "series capacitor") {
		t.Errorf("message should name what is missing: %s", fs[0].Message)
	}
}

// TestACCoupledDistinguishesCouplingFromDecoupling is the assertion that makes this rule worth
// shipping. Both uses are "a capacitor on the net"; the difference is the far side. A cap to ground
// decouples and the signal does not pass through it, so it must NOT satisfy an AC-coupling
// declaration — a rule that counted it would pass every net with a bypass cap on it.
func TestACCoupledDistinguishesCouplingFromDecoupling(t *testing.T) {
	coupling := propFindings(t, propDesign("PCIE_TX0_P", "", "PCIE_TX0_P_C"),
		NetProperty{Net: "PCIE_TX0_P", Property: PropACCoupled})
	if len(coupling) != 0 {
		t.Errorf("a cap to another SIGNAL is a coupling cap, want silence: %+v", coupling)
	}

	for _, farSide := range []string{"GND", "+3V3"} {
		fs := propFindings(t, propDesign("PCIE_TX0_P", "", farSide),
			NetProperty{Net: "PCIE_TX0_P", Property: PropACCoupled})
		if len(fs) != 1 {
			t.Errorf("a cap to %s decouples, it does not couple: want the finding, got %+v", farSide, fs)
		}
	}
}

// TestPropertyRuleIgnoresUndeclaredNets: the rule iterates the DECLARATION and probes the design,
// never the reverse. A design net the intent is silent about is not this rule's business, which is the
// posture every intent rule shares.
func TestPropertyRuleIgnoresUndeclaredNets(t *testing.T) {
	d := propDesign("SOME_OTHER_NET", "GND", "")
	if fs := propFindings(t, d, NetProperty{Net: "SYS_RESET_N", Property: PropResetPolarity, Value: "low"}); len(fs) != 0 {
		t.Errorf("a declared net absent from the design is not a contradiction: %+v", fs)
	}
}

// TestResetPolarityFindsMultiHopBias is the gap that motivated moving these predicates into
// core/check. A bias resistor does not always sit directly between the net and its rail — it can
// reach the rail through further passives, and a direct-only check silently misses that.
//
// profiles.pullupRule already carried both clauses (WS3-108 forced the second). The first draft of
// this rule reimplemented only the direct half, so a design biased through a series element read as
// "no evidence" and the contradiction went unreported.
func TestResetPolarityFindsMultiHopBias(t *testing.T) {
	// PHY_EN -- R1 -- MID -- R2 -- +3V3 : biased HIGH, two hops out.
	d := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "R1", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "R2", Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			{Name: "PHY_EN", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
				{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: "R1", PinRef: "1"}}},
			{Name: "MID", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
				{ComponentRef: "R1", PinRef: "2"}, {ComponentRef: "R2", PinRef: "1"}}},
			{Name: "+3V3", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
				{ComponentRef: "R2", PinRef: "2"}}},
		},
	}
	fs := propFindings(t, d, NetProperty{Net: "PHY_EN", Property: PropResetPolarity, Value: "high"})
	if len(fs) != 1 {
		t.Fatalf("want 1 finding: active-high declared, biased high through a series element; got %+v", fs)
	}
}

// TestStrapFiresOnOppositeBias (WS3-086): a strap declared to latch HIGH that is pulled DOWN comes up
// in the opposite configuration. That contradiction is the check.
func TestStrapFiresOnOppositeBias(t *testing.T) {
	fs := propFindings(t, propDesign("BOOT_MODE0", "GND", ""),
		NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: "high"})
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (strap declared high, pulled down), got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "strap HIGH") || !strings.Contains(fs[0].Message, "biased LOW") {
		t.Errorf("message should name BOTH the declared and the observed level: %s", fs[0].Message)
	}
}

// TestStrapFiresWhenDeclaredLowButPulledHigh is the direction property-reset-polarity structurally
// cannot catch, and the reason strap is not that rule under another name.
//
// reset-polarity's value is the level that ASSERTS reset, so only a bias toward it is a defect and a
// bias away from it is correct. strap's value is the level the pin should LATCH, so a bias away from it
// is the defect. Both ways round are wrong, and this pins the second one.
func TestStrapFiresWhenDeclaredLowButPulledHigh(t *testing.T) {
	fs := propFindings(t, propDesign("PHYAD1", "+3V3", ""),
		NetProperty{Net: "PHYAD1", Property: PropStrap, Value: "low"})
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (strap declared low, pulled up), got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "strap LOW") || !strings.Contains(fs[0].Message, "biased HIGH") {
		t.Errorf("message should name BOTH levels: %s", fs[0].Message)
	}

	// And the same design declared the other way round is correct, so an inverted comparison cannot
	// pass this test by firing on everything.
	if fs := propFindings(t, propDesign("PHYAD1", "+3V3", ""),
		NetProperty{Net: "PHYAD1", Property: PropStrap, Value: "high"}); len(fs) != 0 {
		t.Errorf("a strap pulled to the level it declares is correct, want silence: %+v", fs)
	}
}

// TestStrapSilentWithoutBias pins the limit this rule is honest about. Strap pins carry internal pulls,
// and the standard datasheet instruction is to fit an external resistor ONLY for the non-default state,
// so a design declaring the default level with no resistor on the net is correct and common.
//
// Silence here means "no contradiction found", NOT "strap confirmed".
func TestStrapSilentWithoutBias(t *testing.T) {
	for _, v := range []string{"low", "high"} {
		if fs := propFindings(t, propDesign("BOOT_MODE0", "", ""),
			NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: v}); len(fs) != 0 {
			t.Errorf("no bias resistor is no evidence, not a wrong strap (value=%s): %+v", v, fs)
		}
	}
}

// TestStrapDividerIsNeither: some parts read a tri-level strap from a divider, and which level it
// selects depends on a ratio of two resistances the engine cannot read. Reporting a direction here
// would be guessing.
func TestStrapDividerIsNeither(t *testing.T) {
	d := propDesign("BOOT_MODE0", "GND", "")
	d.Components = append(d.Components, &ir.Component{RefDes: "R2", Prov: &ir.Provenance{SourceFile: "t"}})
	d.Nets[0].Connections = append(d.Nets[0].Connections, &ir.Connection{ComponentRef: "R2", PinRef: "1"})
	d.Nets = append(d.Nets, &ir.Net{
		Name: "+3V3", Prov: &ir.Provenance{SourceFile: "t"},
		Connections: []*ir.Connection{{ComponentRef: "R2", PinRef: "2"}},
	})
	for _, v := range []string{"low", "high"} {
		if fs := propFindings(t, d, NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: v}); len(fs) != 0 {
			t.Errorf("a divider selects neither level (value=%s): %+v", v, fs)
		}
	}
}

// strapDesign is propDesign plus a stamped VALUE on the pull resistor. The value has to be set
// directly rather than as an attribute: check.NewModel re-stamps nothing, so a hand-built design gets
// its Quantity the way the ingestion pass would have left it.
func strapDesign(net, biasTo, input string, ohms float64, unit string) *ir.Design {
	d := propDesign(net, biasTo, "")
	for _, c := range d.Components {
		if c.RefDes == "R1" {
			c.Value = &ir.Quantity{Input: input, Value: &ohms, Unit: unit}
		}
	}
	return d
}

// TestStrapValueBelowMinimum (WS3-119): a strap pulled the RIGHT way by too strong a resistor. The
// direction half is satisfied, which is exactly what makes this easy to miss at review.
func TestStrapValueBelowMinimum(t *testing.T) {
	fs := propFindings(t, strapDesign("BOOT_MODE0", "+3V3", "100R", 100, check.UnitOhm),
		NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: "high", MinOhms: 1000, MaxOhms: 100000})
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (100R below the declared 1k minimum), got %d: %+v", len(fs), fs)
	}
	for _, want := range []string{"R1", "100R", "1k"} {
		if !strings.Contains(fs[0].Message, want) {
			t.Errorf("message must name the resistor, its SOURCE TEXT, and the bound; missing %q: %s", want, fs[0].Message)
		}
	}
}

// TestStrapValueAboveMaximum: the other end of the band.
func TestStrapValueAboveMaximum(t *testing.T) {
	fs := propFindings(t, strapDesign("PHYAD1", "GND", "1M", 1e6, check.UnitOhm),
		NetProperty{Net: "PHYAD1", Property: PropStrap, Value: "low", MinOhms: 1000, MaxOhms: 100000})
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (1M above the declared 100k maximum), got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "100k") {
		t.Errorf("message must render the bound the way an engineer writes it: %s", fs[0].Message)
	}
}

// TestStrapValueInsideBandIsSilent: a correct strap produces nothing, both halves passing.
func TestStrapValueInsideBandIsSilent(t *testing.T) {
	fs := propFindings(t, strapDesign("BOOT_MODE0", "+3V3", "10k", 10000, check.UnitOhm),
		NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: "high", MinOhms: 1000, MaxOhms: 100000})
	if len(fs) != 0 {
		t.Errorf("a strap inside its declared band must be silent: %+v", fs)
	}
}

// TestStrapValueUnreadableIsSilent is the case the ticket calls out by name: UNREADABLE IS NOT
// ACCEPTABLE, but it is also not a defect. A resistor whose value the parser could not read, or which
// carries none at all, is skipped rather than guessed — the params-tier posture. Firing here would
// report a defect on no evidence.
func TestStrapValueUnreadableIsSilent(t *testing.T) {
	// No value stamped at all (a design read before the value pass, or a part with no value field).
	none := propFindings(t, propDesign("BOOT_MODE0", "+3V3", ""),
		NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: "high", MinOhms: 1000, MaxOhms: 100000})
	if len(none) != 0 {
		t.Errorf("a strap resistor with no value must be silent, not flagged: %+v", none)
	}
	// Present-but-unparsed: the source text survived, the number did not.
	d := propDesign("BOOT_MODE0", "+3V3", "")
	for _, c := range d.Components {
		if c.RefDes == "R1" {
			c.Value = &ir.Quantity{Input: "DNP"} // no Value, no Unit
		}
	}
	unparsed := propFindings(t, d, NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: "high", MinOhms: 1000, MaxOhms: 100000})
	if len(unparsed) != 0 {
		t.Errorf("a strap resistor whose value did not parse must be silent: %+v", unparsed)
	}
}

// TestStrapValueWrongUnitIsSilent: a stamped value in the wrong unit is not a resistance. Comparing a
// farad count against an ohm band would be the unlike-units coercion ComponentValueIn exists to refuse.
func TestStrapValueWrongUnitIsSilent(t *testing.T) {
	fs := propFindings(t, strapDesign("BOOT_MODE0", "+3V3", "100n", 1e-7, "F"),
		NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: "high", MinOhms: 1000, MaxOhms: 100000})
	if len(fs) != 0 {
		t.Errorf("a value in farads must not be compared against an ohm band: %+v", fs)
	}
}

// TestStrapValueSilentWithoutDeclaredBand: no band declared, no value check. The direction half still
// runs, which is the degradation the field's doc promises.
func TestStrapValueSilentWithoutDeclaredBand(t *testing.T) {
	// 100R would be outside any sensible band, but none was declared.
	fs := propFindings(t, strapDesign("BOOT_MODE0", "+3V3", "100R", 100, check.UnitOhm),
		NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: "high"})
	if len(fs) != 0 {
		t.Errorf("no declared band means no value check: %+v", fs)
	}
	// ... and the direction half is unaffected by the value machinery.
	wrong := propFindings(t, strapDesign("BOOT_MODE0", "GND", "100R", 100, check.UnitOhm),
		NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: "high"})
	if len(wrong) != 1 || !strings.Contains(wrong[0].Message, "biased LOW") {
		t.Errorf("the direction half must still fire with no band declared: %+v", wrong)
	}
}

// TestStrapValueOnlyOneBoundDeclared: min and max are independent, so a one-sided band checks one side
// and stays silent on the other.
func TestStrapValueOnlyOneBoundDeclared(t *testing.T) {
	lowOnly := propFindings(t, strapDesign("BOOT_MODE0", "+3V3", "100R", 100, check.UnitOhm),
		NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: "high", MinOhms: 1000})
	if len(lowOnly) != 1 {
		t.Errorf("a min-only band must still catch too-strong: %+v", lowOnly)
	}
	// The same 100R is fine when only a maximum was declared.
	maxOnly := propFindings(t, strapDesign("BOOT_MODE0", "+3V3", "100R", 100, check.UnitOhm),
		NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: "high", MaxOhms: 100000})
	if len(maxOnly) != 0 {
		t.Errorf("a max-only band must not judge the low side: %+v", maxOnly)
	}
}

// TestStrapValueNoResistorIsSilent: a band is declared but nothing biases the net, so there is no
// resistor to judge. The direction half already covers "no bias" (silently, for the internal-pull
// reason), and the value half must not reach for a resistor that is not there.
func TestStrapValueNoResistorIsSilent(t *testing.T) {
	fs := propFindings(t, propDesign("BOOT_MODE0", "", ""),
		NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: "high", MinOhms: 1000, MaxOhms: 100000})
	if len(fs) != 0 {
		t.Errorf("a declared band on an unbiased net must be silent: %+v", fs)
	}
}

// TestStrapValueDividerIsSilent: a divider's two resistors set the level together, so neither is "the"
// strap pull. Judging one of them would name an arbitrary part and could flag a correct divider, so
// the value half declines — the same honesty the direction half applies by reporting neither rail.
func TestStrapValueDividerIsSilent(t *testing.T) {
	d := propDesign("BOOT_MODE0", "+3V3", "")
	// A second resistor to ground makes it a divider.
	d.Components = append(d.Components, &ir.Component{RefDes: "R2", Prov: &ir.Provenance{SourceFile: "t"}})
	d.Nets[0].Connections = append(d.Nets[0].Connections, &ir.Connection{ComponentRef: "R2", PinRef: "1"})
	d.Nets = append(d.Nets, &ir.Net{
		Name: "GND", Prov: &ir.Provenance{SourceFile: "t"},
		Connections: []*ir.Connection{{ComponentRef: "R2", PinRef: "2"}},
	})
	for _, c := range d.Components {
		if c.RefDes == "R1" || c.RefDes == "R2" {
			ohms := 100.0
			c.Value = &ir.Quantity{Input: "100R", Value: &ohms, Unit: check.UnitOhm}
		}
	}
	fs := propFindings(t, d, NetProperty{Net: "BOOT_MODE0", Property: PropStrap, Value: "high", MinOhms: 1000, MaxOhms: 100000})
	if len(fs) != 0 {
		t.Errorf("a divider has no single strap resistor to judge; want silence, got %+v", fs)
	}
}
