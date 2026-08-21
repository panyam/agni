package intent

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

func lsF(v float64) *float64 { return &v }

func lsPartType(name string, pins ...string) *ir.PartType {
	p := &ir.PartType{Name: name}
	for i, n := range pins {
		p.Pins = append(p.Pins, &ir.Pin{Name: n, Designator: string(rune('1' + i))})
	}
	return p
}

func lsComp(refDes, part, mpn string) *ir.Component {
	c := &ir.Component{
		RefDes:   refDes,
		Sections: []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}},
		Prov:     &ir.Provenance{SourceFile: "t"},
	}
	if mpn != "" {
		c.Attributes = map[string]string{"MPN": mpn}
	}
	return c
}

func lsConn(ref, pin string) *ir.Connection { return &ir.Connection{ComponentRef: ref, PinRef: pin} }

func lsNet(name string, conns ...*ir.Connection) *ir.Net {
	return &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}, Connections: conns}
}

// lsSwitch describes one controller-based high-side switch to place on a test design: the controller
// U<n> drives Q<n>'s gate and senses across R<n>, whose value the design states in ohms. The trip
// current is the controller's threshold over senseOhms.
type lsSwitch struct {
	n         string  // ref-des suffix, so several switches coexist on one design
	senseOhms float64 // the shunt's value, from the DESIGN
	in, out   string  // the nets on either side of the pass element
	ctrlMPN   string  // the controller's MPN, joined to a seeded spec
	fetMPN    string  // the pass FET's MPN
}

// lsDesign wires one switch per spec. Each switch's shunt sits between two nets the controller also
// touches, which is the Kelvin-sensing signature the resolver looks for.
//
// Nets are merged BY NAME across specs, because a name is a net's identity in every format this reads
// and two switches fed from one input share that input. Appending a second net called VIN would build
// a design no reader can produce, and it would quietly weaken the multi-switch test: the rule would
// resolve the first VIN, see one switch on it, and agree with an assertion about picking between two.
func lsDesign(specs ...lsSwitch) *ir.Design {
	d := &ir.Design{Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
		lsPartType("CTRL", "GATE", "SNSP", "SNSN"),
		lsPartType("FET", "G", "D", "S"),
		lsPartType("RES", "1", "2"),
	}}}}
	byName := map[string]*ir.Net{}
	add := func(name string, conns ...*ir.Connection) {
		if n, ok := byName[name]; ok {
			n.Connections = append(n.Connections, conns...)
			return
		}
		n := lsNet(name, conns...)
		byName[name] = n
		d.Nets = append(d.Nets, n)
	}
	for _, s := range specs {
		ctrl, fet, sense := "U"+s.n, "Q"+s.n, "R"+s.n
		r := lsComp(sense, "RES", "")
		r.Value = &ir.Quantity{Input: "sense", Value: lsF(s.senseOhms), Unit: classify.UnitOhm}
		d.Components = append(d.Components, lsComp(ctrl, "CTRL", s.ctrlMPN), lsComp(fet, "FET", s.fetMPN), r)
		add("GATE_DRV"+s.n, lsConn(ctrl, "1"), lsConn(fet, "1"))
		add(s.in, lsConn(ctrl, "2"), lsConn(sense, "1"))
		add("VSW"+s.n, lsConn(ctrl, "3"), lsConn(sense, "2"), lsConn(fet, "2"))
		add(s.out, lsConn(fet, "3"))
	}
	return d
}

// oneSwitch is the ordinary design under test: one switch between VIN and VOUT.
func oneSwitch(senseOhms float64) *ir.Design {
	return lsDesign(lsSwitch{n: "1", senseOhms: senseOhms, in: "VIN", out: "VOUT",
		ctrlMPN: "ACME-HSS", fetMPN: "ACME-NFET"})
}

// hssSpec is a switch controller stating an overcurrent sense threshold in volts. That row is what
// identifies a part as a controller (the resolver reads the datasheet, not a class keyword) and it is
// the only datasheet value the verdict rests on.
func hssSpec(mpn string, ocpVolts float64) *parampb.PartSpec {
	return &parampb.PartSpec{
		Mpn: mpn, Manufacturer: "Acme",
		Docs: []*parampb.SourceDoc{{Id: "hss", Title: mpn + " Rev A", Vendor: "Acme"}},
		Parameters: []*parampb.Parameter{{
			Name: "Overcurrent Protection Threshold", Symbol: "V(OCP)",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC,
			Value:             &parampb.RangeValue{Max: lsF(ocpVolts)},
			Unit:              "V",
			Conditions:        []*parampb.Condition{{Symbol: "TA", Eq: lsF(25), Unit: "C", Raw: "TA = 25C"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{DocRef: "hss", Page: 6,
				TableOrFigure: "Electrical Characteristics", Method: "hand", Confidence: 1},
		}},
	}
}

// fetSpec is the external pass MOSFET. rdsOhms > 0 adds the on-resistance row the sizing clause reads;
// 0 leaves the part seeded but silent on it, which is the same gap as an unseeded part.
func fetSpec(mpn string, rdsOhms float64) *parampb.PartSpec {
	s := &parampb.PartSpec{
		Mpn: mpn, Manufacturer: "Acme",
		Docs: []*parampb.SourceDoc{{Id: "nfet", Title: mpn + " Rev C", Vendor: "Acme"}},
	}
	if rdsOhms > 0 {
		s.Parameters = append(s.Parameters, &parampb.Parameter{
			Name: "Static Drain-Source On-Resistance", Symbol: "RDS(on)",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC,
			Value:             &parampb.RangeValue{Max: lsF(rdsOhms)},
			Unit:              "Ohm",
			Conditions:        []*parampb.Condition{{Symbol: "VGS", Eq: lsF(10), Unit: "V", Raw: "VGS = 10 V"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{DocRef: "nfet", Page: 2,
				TableOrFigure: "Electrical Characteristics", Method: "hand", Confidence: 1},
		})
	}
	return s
}

// lsModel seeds the ordinary one-switch design.
func lsModel(d *ir.Design, ocpVolts, rdsOhms float64) check.Model {
	return check.NewModelWithParams(d, nil, param.ParamSet{
		"ACME-HSS":  hssSpec("ACME-HSS", ocpVolts),
		"ACME-NFET": fetSpec("ACME-NFET", rdsOhms),
	})
}

// lsDecl is a one-rail declaration carrying only a budget.
func lsDecl(rail string, peak float64) Declaration {
	return Declaration{Name: "t", RailBudgets: []RailBudget{{Rail: rail, Peak: peak}}}
}

func lsEval(d Declaration, m check.Model) []check.Finding {
	return loadSwitchTripBelowBudgetRule(d).Findings(m)
}

// TestLoadSwitchTripBelowBudgetFires is the WS3-085 acceptance for the sizing lower bound. A 50mV
// threshold across a 25mOhm shunt limits at 2A, on a rail the intent declares draws up to 5A: the
// switch opens under the load the design was drawn for.
//
// The finding has to name all three inputs. A message carrying only the verdict leaves a reviewer
// unable to tell which of the budget, the threshold and the shunt is the wrong one.
func TestLoadSwitchTripBelowBudgetFires(t *testing.T) {
	fs := lsEval(lsDecl("VOUT", 5), lsModel(oneSwitch(0.025), 0.05, 0.02))
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (a 2A limit on a 5A rail), got %d: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Kind != check.KindNet || f.Subject != "VOUT" {
		t.Errorf("subject = (%s, %q), want the declared rail net VOUT", f.Kind, f.Subject)
	}
	for _, want := range []string{"5A", "2A", "V(OCP)", "R1", "U1", "0.025"} {
		if !strings.Contains(f.Message, want) {
			t.Errorf("message missing %q: %s", want, f.Message)
		}
	}
	if f.Prov == nil {
		t.Error("a net finding must carry the rail's provenance so the viewer can locate it")
	}
	if len(f.DatasheetProv) != 1 || f.DatasheetProv[0].Doc != "ACME-HSS Rev A" {
		t.Errorf("want exactly the controller's threshold cited, got %+v", f.DatasheetProv)
	}
}

// TestLoadSwitchTripAboveBudgetSilent guards the comparison DIRECTION, which a sign error would invert
// while every assertion in the firing test still passed. The same board with a 10mOhm shunt limits at
// 5A, above the 2A the rail is declared to draw.
func TestLoadSwitchTripAboveBudgetSilent(t *testing.T) {
	if fs := lsEval(lsDecl("VOUT", 2), lsModel(oneSwitch(0.01), 0.05, 0.02)); len(fs) != 0 {
		t.Errorf("a 5A limit on a 2A rail must be silent, got %+v", fs)
	}
}

// TestLoadSwitchTripAtExactlyTheBudget is the float-comparison guard, and the values are chosen so the
// mutation shows.
//
// 22mV across a 4.4mOhm shunt is 5A. In float64 the DIVISION lands on 4.999999999999999, so a rail
// declared at 5.0 reads as under-limited and fires unless below() carries its relative tolerance. The
// arithmetic has to happen at runtime for the test to mean anything: 0.022/0.0044 written as a constant
// expression folds at arbitrary precision and is exactly 5, so a check spelled that way would disagree
// with the running code. Here both sides arrive as float64 values, one from a seeded parameter and one
// from a stamped component value.
//
// A pair that happens to round the other way (50mV across 10mOhm is one) would make this test pass with
// the tolerance deleted, which is the wrong reason to be green.
func TestLoadSwitchTripAtExactlyTheBudget(t *testing.T) {
	m := lsModel(oneSwitch(0.0044), 0.022, 0.02)
	if fs := lsEval(lsDecl("VOUT", 5), m); len(fs) != 0 {
		t.Errorf("a switch limiting at exactly the declared peak must be silent, got %+v", fs)
	}
	// One milliamp of genuine shortfall still fires, so the tolerance has not blunted the check.
	if fs := lsEval(lsDecl("VOUT", 5.001), m); len(fs) != 1 {
		t.Errorf("a switch genuinely short of the declared peak must still fire, got %+v", fs)
	}
}

// TestLoadSwitchSilentOnUndeclaredAndAbsentRails holds the direction of the iteration. The rule walks
// the DECLARATION and probes the design: a rail the design carries but the intent does not budget is not
// its business, and a budgeted rail the design does not carry is a missing-rail defect the
// voltage-domain and subsystem forms report, so firing here would report one defect twice.
func TestLoadSwitchSilentOnUndeclaredAndAbsentRails(t *testing.T) {
	m := lsModel(oneSwitch(0.025), 0.05, 0.02)
	if fs := lsEval(lsDecl("NOT_A_RAIL", 5), m); len(fs) != 0 {
		t.Errorf("a budgeted rail the design does not carry must be silent, got %+v", fs)
	}
	if fs := lsEval(Declaration{Name: "t"}, m); len(fs) != 0 {
		t.Errorf("a design rail nobody budgeted must be silent, got %+v", fs)
	}
}

// TestLoadSwitchSilentWhereNoSwitchReachesTheRail: the gate net of the switch is budgeted, which no
// series path connects to the pass element's power terminals. A rule that associated any switch on the
// design with any declared rail would fire here, and on every unswitched rail of a board that has one
// switch somewhere.
func TestLoadSwitchSilentWhereNoSwitchReachesTheRail(t *testing.T) {
	m := lsModel(oneSwitch(0.025), 0.05, 0.02)
	if fs := lsEval(lsDecl("GATE_DRV1", 5), m); len(fs) != 0 {
		t.Errorf("a rail no pass element reaches must be silent, got %+v", fs)
	}
}

// TestLoadSwitchBudgetOnTheInputSide: a series element carries the same current on both sides, so a
// budget declared on the rail feeding the switch is judged by the same limit as one declared on its
// output. Requiring the output side would make the rule depend on which end an author happened to
// budget.
func TestLoadSwitchBudgetOnTheInputSide(t *testing.T) {
	m := lsModel(oneSwitch(0.025), 0.05, 0.02)
	if fs := lsEval(lsDecl("VIN", 5), m); len(fs) != 1 {
		t.Errorf("a budget on the switch's input rail must be judged too, got %d: %+v", len(fs), fs)
	}
}

// TestLoadSwitchHighestLimitBinds: two switches reach one rail and the rule reads the HIGHER limit. The
// reach radius can pull in a switch that gates a different branch, and reporting the smaller one would
// be a nuisance-trip finding the design does not have. Every fail must be a genuine defect, so ambiguous
// evidence takes the reading that does not fire.
func TestLoadSwitchHighestLimitBinds(t *testing.T) {
	d := lsDesign(
		lsSwitch{n: "1", senseOhms: 0.025, in: "VIN", out: "VOUT", ctrlMPN: "ACME-HSS", fetMPN: "ACME-NFET"},
		lsSwitch{n: "2", senseOhms: 0.005, in: "VIN", out: "VOUT", ctrlMPN: "ACME-HSS2", fetMPN: "ACME-NFET"},
	)
	m := check.NewModelWithParams(d, nil, param.ParamSet{
		"ACME-HSS":  hssSpec("ACME-HSS", 0.05),  // 2A
		"ACME-HSS2": hssSpec("ACME-HSS2", 0.05), // 10A
		"ACME-NFET": fetSpec("ACME-NFET", 0.02),
	})
	if fs := lsEval(lsDecl("VOUT", 5), m); len(fs) != 0 {
		t.Errorf("the 10A switch covers the 5A budget, so the rail must be silent, got %+v", fs)
	}
	// The same pair against a budget above BOTH limits still fires, so taking the highest has not
	// silenced the rule outright.
	fs := lsEval(lsDecl("VOUT", 12), m)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding against a 12A budget, got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "10A") {
		t.Errorf("the finding must judge against the 10A limit, not the 2A one: %s", fs[0].Message)
	}
}

// beadChain hangs n ferrites off VOUT in series, VOUT -FB1- VOUT_FILT1 -FB2- VOUT_FILT2, and returns
// the design. The far end is the rail a budget gets declared on.
func beadChain(n int) *ir.Design {
	d := oneSwitch(0.025)
	prev := "VOUT"
	for i := 1; i <= n; i++ {
		bead, far := fmt.Sprintf("FB%d", i), fmt.Sprintf("VOUT_FILT%d", i)
		d.Components = append(d.Components, lsComp(bead, "", ""))
		for _, net := range d.Nets {
			if net.Name == prev {
				net.Connections = append(net.Connections, lsConn(bead, "1"))
			}
		}
		d.Nets = append(d.Nets, lsNet(far, lsConn(bead, "2")))
		prev = far
	}
	return d
}

// TestLoadSwitchAcrossSeriesElement pins the association RADIUS from both sides, which is the pair of
// errors a single-distance test cannot tell apart.
//
// One crossing must be reached: a ferrite between the pass element and the rail is ordinary layout and
// must not hide the switch. Two must NOT be, and that is the review-integrity half. Voltage does not
// degrade along a series path, so a radius set wide enough makes every rail on the board look like it
// is behind every switch, and each of those associations is a fail the design does not have. The rule
// borrows check.SupplyPathReachHops rather than choosing its own number, so the two sizing rules cannot
// drift to different answers about what "on this rail" means.
func TestLoadSwitchAcrossSeriesElement(t *testing.T) {
	if fs := lsEval(lsDecl("VOUT_FILT1", 5), lsModel(beadChain(1), 0.05, 0.02)); len(fs) != 1 {
		t.Errorf("want the finding to survive one series crossing, got %d: %+v", len(fs), fs)
	}
	if fs := lsEval(lsDecl("VOUT_FILT2", 5), lsModel(beadChain(2), 0.05, 0.02)); len(fs) != 0 {
		t.Errorf("two series crossings is past the supply radius and must be silent, got %+v", fs)
	}
}

// TestLoadSwitchReportsDissipationAtTheDeclaredCurrent is the RDS(on) half of sizing. The fix for a
// limit set too low is a smaller shunt, and that only helps if the pass element can carry the budgeted
// current, so the finding reports what it dissipates at the declared draw: 5A through 20mOhm is 0.5W.
//
// The figure is reported, never judged, and it adds NO citation. A finding is rated by its weakest
// citation, so listing a value the conclusion never used could drag a genuine failure to provisional.
func TestLoadSwitchReportsDissipationAtTheDeclaredCurrent(t *testing.T) {
	fs := lsEval(lsDecl("VOUT", 5), lsModel(oneSwitch(0.025), 0.05, 0.02))
	if len(fs) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(fs), fs)
	}
	for _, want := range []string{"RDS(on)", "0.5W", "Q1"} {
		if !strings.Contains(fs[0].Message, want) {
			t.Errorf("the sizing clause must report %q: %s", want, fs[0].Message)
		}
	}
	if len(fs[0].DatasheetProv) != 1 {
		t.Errorf("the on-resistance must not add a citation: want 1, got %d", len(fs[0].DatasheetProv))
	}
}

// TestLoadSwitchWithoutOnResistanceSaysNothing: a pass FET stating no comparable RDS(on) must produce no
// sizing clause at all. A zero or an omitted figure would read as a FET that dissipates nothing.
func TestLoadSwitchWithoutOnResistanceSaysNothing(t *testing.T) {
	fs := lsEval(lsDecl("VOUT", 5), lsModel(oneSwitch(0.025), 0.05, 0))
	if len(fs) != 1 {
		t.Fatalf("want the finding regardless of the FET's seeding, got %d: %+v", len(fs), fs)
	}
	if strings.Contains(fs[0].Message, "Sizing the pass element") {
		t.Errorf("no RDS(on) row means no sizing clause: %s", fs[0].Message)
	}
	if len(fs[0].DatasheetProv) != 1 {
		t.Errorf("want the controller's citation, got %+v", fs[0].DatasheetProv)
	}
}

// TestLoadSwitchWithUnusableOnResistanceSaysNothing is the other half of the sizing clause's refusal.
// A seeded RDS(on) row is not automatically a usable one: nothing in the parameter layer gates a max
// bound for finiteness, so a bad extraction can leave a row present and stating infinity. The clause
// has to drop on the ARITHMETIC refusing, not only on the row being absent, or a reviewer reads
// "dissipating +InfW" and learns nothing about the part.
//
// The finding itself must survive. The dissipation is reported and never judged, so its absence has no
// bearing on whether the trip point sits under the declared draw.
func TestLoadSwitchWithUnusableOnResistanceSaysNothing(t *testing.T) {
	fs := lsEval(lsDecl("VOUT", 5), lsModel(oneSwitch(0.025), 0.05, math.Inf(1)))
	if len(fs) != 1 {
		t.Fatalf("the verdict does not rest on the FET's row, want 1 finding, got %d: %+v", len(fs), fs)
	}
	if strings.Contains(fs[0].Message, "Sizing the pass element") {
		t.Errorf("an unusable on-resistance must produce no sizing clause: %s", fs[0].Message)
	}
}

// TestLoadSwitchSilentWithoutTheControllerDatasheet: with nothing seeded there is no threshold, so no
// switch resolves and there is no limit to compare. It reports the rail as unevaluable rather than
// clean, and the unevaluable half is the review runner's needs-data gate, which the rule feeds by
// declaring ParamSymbols. Without that declaration the item would score a pass on a check that never ran.
func TestLoadSwitchSilentWithoutTheControllerDatasheet(t *testing.T) {
	m := check.NewModel(oneSwitch(0.025))
	r := loadSwitchTripBelowBudgetRule(lsDecl("VOUT", 5))
	if fs := r.Findings(m); len(fs) != 0 {
		t.Errorf("want no findings with no seeded params, got %+v", fs)
	}
	if ok, reason := check.Available(r, m); ok || reason == "" {
		t.Errorf("Available = (%v, %q), want not-applicable with a reason", ok, reason)
	}
	if len(r.ParamSymbols) == 0 {
		t.Fatal("the rule must declare the datasheet symbols it joins on, or a bound item reads pass")
	}
	// The gate has to recognize the spelling a real spec is seeded under, or it fires on every design and
	// the rule never reports anything. V(OCP) is what the controller above states.
	seeded := lsModel(oneSwitch(0.025), 0.05, 0.02)
	if !check.SeedsAnySymbol(seeded, r.ParamSymbols) {
		t.Error("the declared symbols must match a seeded V(OCP) row, else every design reads needs-data")
	}
	if check.SeedsAnySymbol(m, r.ParamSymbols) {
		t.Error("an unseeded design must not satisfy the symbol gate")
	}
}

// TestLoadSwitchSilentWithoutAThreshold: a seeded controller stating no overcurrent threshold is the
// same gap as an unseeded one. Nothing identifies it as a current-limiting part, so no switch resolves
// and there is no verdict. Skip, never pass.
func TestLoadSwitchSilentWithoutAThreshold(t *testing.T) {
	m := check.NewModelWithParams(oneSwitch(0.025), nil, param.ParamSet{
		"ACME-HSS":  {Mpn: "ACME-HSS", Manufacturer: "Acme"},
		"ACME-NFET": fetSpec("ACME-NFET", 0.02),
	})
	if fs := lsEval(lsDecl("VOUT", 5), m); len(fs) != 0 {
		t.Errorf("a controller stating no threshold must yield no verdict, got %+v", fs)
	}
}

// TestLoadSwitchSilentWithoutTheShuntValue: the trip current is the threshold divided by a resistance
// the DESIGN states. A shunt whose value the reader never normalized is not evidence of a milliohm
// part, so the switch does not resolve and the rule says nothing.
func TestLoadSwitchSilentWithoutTheShuntValue(t *testing.T) {
	d := oneSwitch(0.025)
	for _, c := range d.Components {
		if c.RefDes == "R1" {
			c.Value = nil
		}
	}
	if fs := lsEval(lsDecl("VOUT", 5), lsModel(d, 0.05, 0.02)); len(fs) != 0 {
		t.Errorf("a shunt with no stated value must yield no verdict, got %+v", fs)
	}
}

// TestLoadSwitchRuleCompiledWithBudgets: the rule reads rail_budgets and nothing else from the
// declaration, so it compiles exactly when a budget is declared. A declaration about something else must
// not drag a silently-passing sizing rule into the catalog.
func TestLoadSwitchRuleCompiledWithBudgets(t *testing.T) {
	names := func(d Declaration) map[string]bool {
		out := map[string]bool{}
		for _, r := range Compile(d) {
			out[r.Name] = true
		}
		return out
	}
	if !names(lsDecl("VOUT", 5))[RuleLoadSwitchTripBelowBudget] {
		t.Error("a declared rail budget must compile the load-switch sizing rule")
	}
	// No margin factor is declared above, so this rule must NOT inherit the margin rule's gate: the
	// lower bound is a protection threshold against a declared draw, not house headroom policy.
	if names(lsDecl("VOUT", 5))[RuleRailCurrentMargin] {
		t.Error("no margin_factor must still leave the margin rule uncompiled")
	}
	none := names(Declaration{Name: "t", Modules: []Module{{Name: "MCU", Class: "soc"}}})
	if none[RuleLoadSwitchTripBelowBudget] {
		t.Errorf("no rail_budgets must compile no load-switch rule, got %v", none)
	}
}
