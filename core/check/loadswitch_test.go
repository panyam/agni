package check

import (
	"math"
	"testing"

	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

func lsPtr(v float64) *float64 { return &v }

// TestOhmsLawCurrent covers the named physical operation the trip current is computed with (WS3-085).
// The refusals matter more than the arithmetic: a zero-ohm shunt divides to +Inf, which every
// comparison downstream reads as an enormous current and reports as a defect.
func TestOhmsLawCurrent(t *testing.T) {
	for _, c := range []struct {
		name   string
		volts  float64
		ohms   float64
		want   float64
		wantOK bool
	}{
		{"50mV across 10mOhm trips at 5A", 0.05, 0.01, 5, true},
		{"50mV across 50mOhm trips at 1A", 0.05, 0.05, 1, true},
		{"zero ohms is refused, not infinity", 0.05, 0, 0, false},
		{"a negative resistance is refused", 0.05, -0.01, 0, false},
		{"an infinite resistance is refused", 0.05, math.Inf(1), 0, false},
		{"a NaN voltage is refused", math.NaN(), 0.01, 0, false},
		{"an infinite voltage is refused", math.Inf(1), 0.01, 0, false},
		{"a negative threshold is legal and yields a negative current", -0.05, 0.01, -5, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, ok := OhmsLawCurrent(c.volts, c.ohms)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if ok && !QuantityEqual(got, c.want) {
				t.Errorf("amps = %g, want %g", got, c.want)
			}
			if !ok && got != 0 {
				t.Errorf("a refused answer must return 0, got %g", got)
			}
		})
	}
}

// TestResistivePowerWatts covers OhmsLawCurrent's sibling, the sizing half of a load switch (WS3-085).
//
// Two behaviours differ from OhmsLawCurrent and both are deliberate. ZERO ohms is allowed here: a
// zero-ohm link dissipates nothing, which is a true answer rather than an unanswerable one, whereas a
// zero divisor has no current to report. And a NEGATIVE current is allowed and squares to the same
// positive power, because a resistor heats the same whichever way the current runs. A refusal on
// either would push the sizing clause off a legal design.
func TestResistivePowerWatts(t *testing.T) {
	for _, c := range []struct {
		name   string
		amps   float64
		ohms   float64
		want   float64
		wantOK bool
	}{
		{"5A through 20mOhm dissipates 0.5W", 5, 0.02, 0.5, true},
		{"12A through 20mOhm dissipates 2.88W", 12, 0.02, 2.88, true},
		{"a zero-ohm link dissipates nothing, and that is an answer", 5, 0, 0, true},
		{"no current dissipates nothing", 0, 0.02, 0, true},
		{"a negative current dissipates the same as a positive one", -5, 0.02, 0.5, true},
		{"a negative resistance is refused", 5, -0.02, 0, false},
		{"a NaN current is refused", math.NaN(), 0.02, 0, false},
		{"an infinite current is refused", math.Inf(1), 0.02, 0, false},
		{"a NaN resistance is refused", 5, math.NaN(), 0, false},
		{"an infinite resistance is refused", 5, math.Inf(1), 0, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ResistivePowerWatts(c.amps, c.ohms)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if ok && !QuantityEqual(got, c.want) {
				t.Errorf("watts = %g, want %g", got, c.want)
			}
			if !ok && got != 0 {
				t.Errorf("a refused answer must return 0, got %g", got)
			}
		})
	}
}

// TestOcpThresholdSymbolsCoverTheAliasSet holds the two spellings of the overcurrent alias set to each
// other. OcpThresholdLimits matches a seeded row against ocpThresholdSymbols; a rule declares
// OcpThresholdSymbols() so the review runner can tell needs-data from a clean design. If those drift,
// the failure is silent and one-directional: a symbol the extractor reads but the gate does not
// declare makes a seeded design read needs-data forever, and a symbol the gate declares but the
// extractor never matches makes an unseeded design look seeded, which scores a pass on a check that
// never ran.
func TestOcpThresholdSymbolsCoverTheAliasSet(t *testing.T) {
	declared := map[string]bool{}
	for _, s := range OcpThresholdSymbols() {
		n := alnumUpper(s)
		if declared[n] {
			t.Errorf("%q is declared twice after normalization (%q); an author would see it twice", s, n)
		}
		declared[n] = true
	}
	// Every symbol the extractor matches must be declarable, or a seeded design reads needs-data.
	for sym := range ocpThresholdSymbols {
		if !declared[alnumUpper(sym)] {
			t.Errorf("OcpThresholdLimits matches %q but no declared symbol normalizes to it", sym)
		}
	}
	// And nothing may be declared that the extractor would never match.
	matched := map[string]bool{}
	for sym := range ocpThresholdSymbols {
		matched[alnumUpper(sym)] = true
	}
	for _, s := range OcpThresholdSymbols() {
		if !matched[alnumUpper(s)] {
			t.Errorf("%q is declared but OcpThresholdLimits matches no such symbol", s)
		}
	}
	// The accessor hands out a copy: a rule storing it in Rule.ParamSymbols must not be able to
	// corrupt the alias set for every other design in the process.
	got := OcpThresholdSymbols()
	got[0] = "CLOBBERED"
	if OcpThresholdSymbols()[0] == "CLOBBERED" {
		t.Error("OcpThresholdSymbols returns the package slice itself, so a caller can corrupt it")
	}
}

// lsPart builds a part type with the named pins, designators "1".."n".
func lsPart(name string, pins ...string) *ir.PartType {
	p := &ir.PartType{Name: name}
	for i, n := range pins {
		p.Pins = append(p.Pins, &ir.Pin{Name: n, Designator: string(rune('1' + i))})
	}
	return p
}

func lsComponent(refDes, part, mpn string) *ir.Component {
	c := &ir.Component{
		RefDes:   refDes,
		Sections: []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}},
		Prov:     &ir.Provenance{SourceFile: "t"},
	}
	if mpn != "" {
		c.Mpn = mpn
	}
	return c
}

func lsNet(name string, conns ...*ir.Connection) *ir.Net {
	return &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}, Connections: conns}
}

func conn(ref, pin string) *ir.Connection { return &ir.Connection{ComponentRef: ref, PinRef: pin} }

// loadSwitchDesign is the canonical external-FET high-side switch: controller U1 drives Q1's gate and
// senses across R1, which sits in series in the power path with both of its terminals on U1's sense
// pins (Kelvin sensing). R1 is stamped at 10mOhm, the shape ingestion produces from "0R01".
func loadSwitchDesign() *ir.Design {
	r1 := lsComponent("R1", "RES", "")
	r1.Value = &ir.Quantity{Input: "0R01", Value: lsPtr(0.01), Unit: classify.UnitOhm}
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			lsPart("CTRL", "GATE", "SNSP", "SNSN"),
			lsPart("FET", "G", "D", "S"),
			lsPart("RES", "1", "2"),
		}}},
		Components: []*ir.Component{
			lsComponent("U1", "CTRL", "DEMO-HSS"),
			lsComponent("Q1", "FET", "DEMO-NFET"),
			r1,
		},
		Nets: []*ir.Net{
			lsNet("GATE_DRV", conn("U1", "1"), conn("Q1", "1")),
			lsNet("VIN", conn("U1", "2"), conn("R1", "1")),
			lsNet("VSW", conn("U1", "3"), conn("R1", "2"), conn("Q1", "2")),
			lsNet("VOUT", conn("Q1", "3")),
		},
	}
}

// ctrlSpec is a synthetic switch-controller spec: an overcurrent sense threshold and nothing else a
// load-switch rule reads.
func ctrlSpec(mpn string, ocpVolts float64) *parampb.PartSpec {
	return &parampb.PartSpec{
		Mpn: mpn, Manufacturer: "Acme",
		DeviceClass: "high-side switch controller",
		Docs:        []*parampb.SourceDoc{{Id: "ctrl", Title: "ACME-HSS Rev A", Vendor: "Acme"}},
		Parameters: []*parampb.Parameter{{
			Name: "Overcurrent Protection Threshold", Symbol: "V(OCP)",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC,
			Value:             &parampb.RangeValue{Max: lsPtr(ocpVolts)},
			Unit:              "V",
			Conditions:        []*parampb.Condition{{Symbol: "TA", Eq: lsPtr(25), Unit: "C", Raw: "TA = 25C"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{DocRef: "ctrl", Page: 5,
				TableOrFigure: "Electrical Characteristics", Method: "hand", Confidence: 1},
		}},
	}
}

// nfetSpec is a synthetic pass-FET spec: a continuous drain rating and an on-resistance row.
func nfetSpec(mpn string, idAmps, rdsOhms float64) *parampb.PartSpec {
	s := &parampb.PartSpec{
		Mpn: mpn, Manufacturer: "Acme",
		DeviceClass: "nfet",
		Docs:        []*parampb.SourceDoc{{Id: "fet", Title: "ACME-NFET Rev B", Vendor: "Acme"}},
		Parameters: []*parampb.Parameter{{
			Name: "Drain Current - Continuous", Symbol: "ID",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
			Value:             &parampb.RangeValue{Max: lsPtr(idAmps)},
			Unit:              "A",
			Conditions:        []*parampb.Condition{{Symbol: "TA", Eq: lsPtr(25), Unit: "C", Raw: "TA = 25C"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{DocRef: "fet", Page: 1,
				TableOrFigure: "Absolute Maximum Ratings", Method: "hand", Confidence: 1},
		}},
	}
	if rdsOhms > 0 {
		s.Parameters = append(s.Parameters, &parampb.Parameter{
			Name: "Static Drain-Source On-Resistance", Symbol: "RDS(on)",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC,
			Value:             &parampb.RangeValue{Max: lsPtr(rdsOhms)},
			Unit:              "Ohm",
			Conditions:        []*parampb.Condition{{Symbol: "VGS", Eq: lsPtr(10), Unit: "V", Raw: "VGS = 10 V"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{DocRef: "fet", Page: 2,
				TableOrFigure: "Electrical Characteristics", Method: "hand", Confidence: 1},
		})
	}
	return s
}

func loadSwitchSet() param.ParamSet {
	return param.ParamSet{
		"DEMO-HSS":  ctrlSpec("DEMO-HSS", 0.05),
		"DEMO-NFET": nfetSpec("DEMO-NFET", 3, 0.02),
	}
}

func resolveOne(t *testing.T, d *ir.Design) (ExternalFetLoadSwitch, bool) {
	t.Helper()
	m := NewModelWithParams(d, nil, loadSwitchSet())
	sw := ExternalFetLoadSwitches(m)
	if len(sw) == 0 {
		return ExternalFetLoadSwitch{}, false
	}
	if len(sw) > 1 {
		t.Fatalf("want at most one resolved switch, got %d: %+v", len(sw), sw)
	}
	return sw[0], true
}

// TestExternalFetLoadSwitchResolves is the happy path: all three parts found, and the trip current
// computed from the DESIGN's shunt against the CONTROLLER's threshold. 50mV over 10mOhm is 5A.
func TestExternalFetLoadSwitchResolves(t *testing.T) {
	sw, ok := resolveOne(t, loadSwitchDesign())
	if !ok {
		t.Fatal("want a resolved load switch, got none")
	}
	if sw.Controller != "U1" || sw.Fet != "Q1" || sw.Sense != "R1" {
		t.Errorf("resolved %q/%q/%q, want U1/Q1/R1", sw.Controller, sw.Fet, sw.Sense)
	}
	if sw.GateNet != "GATE_DRV" {
		t.Errorf("gate net = %q, want GATE_DRV", sw.GateNet)
	}
	if !QuantityEqual(sw.SenseOhms, 0.01) {
		t.Errorf("sense = %gOhm, want 0.01", sw.SenseOhms)
	}
	if !QuantityEqual(sw.TripAmps, 5) {
		t.Errorf("trip = %gA, want 5 (50mV / 10mOhm)", sw.TripAmps)
	}
	if sw.Ocp == nil || sw.Ocp.Symbol != "V(OCP)" {
		t.Errorf("Ocp = %+v, want the V(OCP) row", sw.Ocp)
	}
	// The effective on-resistance of a controller-based switch IS the external FET's RDS(on). Nothing
	// on the controller's sheet answers this, which is the whole reason the ticket exists.
	if sw.OnResistance == nil || !QuantityEqual(sw.OnResistance.Value.GetMax(), 0.02) {
		t.Errorf("OnResistance = %+v, want the FET's 0.02Ohm RDS(on) row", sw.OnResistance)
	}
}

// TestLoadSwitchNeedsTheGateRole: the topology hangs off the FET's GATE terminal, resolved through the
// naming lexicon (WS3-117). A transistor whose pins carry no recognized terminal name yields no
// switch, rather than the resolver falling back to some other pin.
// The FET is left on the drive net ALONE, so that a resolver ignoring the role would find exactly one
// net and resolve a switch. Leaving its other terminals connected would make the test pass for the
// wrong reason: any resolver would then see several nets and refuse on count rather than on role.
func TestLoadSwitchNeedsTheGateRole(t *testing.T) {
	d := loadSwitchDesign()
	d.Libraries[0].Parts[1] = lsPart("FET", "P1", "P2", "P3")
	d.Nets[2].Connections = []*ir.Connection{conn("U1", "3"), conn("R1", "2")}
	d.Nets[3].Connections = nil
	if _, ok := resolveOne(t, d); ok {
		t.Error("a transistor with no gate terminal must resolve no load switch")
	}
}

// TestLoadSwitchSenseValueMustBeInOhms is the EDIF-shaped case, and the one that decides what this
// rule family may claim on a real board. WS3-118 normalizes the value attribute on KiCad, IPC-2581
// and gEDA; EDIF and xschem carry it under the exporting tool's own spelling, so the shunt commonly
// arrives with no parsed number at all. Silence is the only honest answer: a resistor whose value is
// unknown is not evidence of a milliohm shunt.
func TestLoadSwitchSenseValueMustBeInOhms(t *testing.T) {
	for _, c := range []struct {
		name string
		q    *ir.Quantity
	}{
		{"no value stamped at all", nil},
		{"source text present but unparsed", &ir.Quantity{Input: "0R01"}},
		{"a number with no unit", &ir.Quantity{Input: "0.01", Value: lsPtr(0.01)}},
		{"a number in the wrong unit", &ir.Quantity{Input: "10u", Value: lsPtr(1e-5), Unit: "F"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := loadSwitchDesign()
			d.Components[2].Value = c.q
			if _, ok := resolveOne(t, d); ok {
				t.Error("want no resolved switch: the shunt's resistance is not known")
			}
		})
	}
}

// TestLoadSwitchZeroOhmShuntIsSilent: a shunt read as 0 would divide to infinity, and an infinite
// trip current exceeds every rating, so the rule would report a defect on every such design.
func TestLoadSwitchZeroOhmShuntIsSilent(t *testing.T) {
	d := loadSwitchDesign()
	d.Components[2].Value = &ir.Quantity{Input: "0R", Value: lsPtr(0), Unit: classify.UnitOhm}
	if _, ok := resolveOne(t, d); ok {
		t.Error("a zero-ohm shunt must resolve no switch, not an infinite trip current")
	}
}

// TestLoadSwitchRejectsDividerSizedResistor: a feedback or programming divider between a controller's
// output and its sense pin has the same STRUCTURE as a Kelvin-sensed shunt. Magnitude separates them,
// and the resolver must not take a kilohm part for a current shunt.
func TestLoadSwitchRejectsDividerSizedResistor(t *testing.T) {
	d := loadSwitchDesign()
	d.Components[2].Value = &ir.Quantity{Input: "10k", Value: lsPtr(10000), Unit: classify.UnitOhm}
	if _, ok := resolveOne(t, d); ok {
		t.Error("a 10k resistor must not be read as a current shunt")
	}
}

// TestLoadSwitchAmbiguousShuntIsSilent: two resistors both sitting entirely on the controller's nets
// are indistinguishable, and a trip current computed from the wrong one looks exactly as authoritative
// as a correct one.
func TestLoadSwitchAmbiguousShuntIsSilent(t *testing.T) {
	d := loadSwitchDesign()
	r2 := lsComponent("R2", "RES", "")
	r2.Value = &ir.Quantity{Input: "0R02", Value: lsPtr(0.02), Unit: classify.UnitOhm}
	d.Components = append(d.Components, r2)
	d.Nets[1].Connections = append(d.Nets[1].Connections, conn("R2", "1"))
	d.Nets[2].Connections = append(d.Nets[2].Connections, conn("R2", "2"))
	if _, ok := resolveOne(t, d); ok {
		t.Error("two candidate shunts must resolve no switch")
	}
}

// TestLoadSwitchExcludesResistorReachingOffTheController: the Kelvin signature is that EVERY one of
// the resistor's nets is one the controller touches. A series part with a far side the controller
// cannot see is not being measured, so it is not the shunt.
func TestLoadSwitchExcludesResistorReachingOffTheController(t *testing.T) {
	d := loadSwitchDesign()
	// R1's far terminal moves off VSW (which U1 senses) onto VOUT (which it does not).
	d.Nets[2].Connections = []*ir.Connection{conn("U1", "3"), conn("Q1", "2")}
	d.Nets[3].Connections = append(d.Nets[3].Connections, conn("R1", "2"))
	if _, ok := resolveOne(t, d); ok {
		t.Error("a resistor whose far net the controller does not touch must not be read as the shunt")
	}
}

// TestLoadSwitchOneTerminalResistorIsNotAShunt: a shunt is MEASURED, so both of its terminals are on
// the controller. A resistor with one terminal connected has nothing across it, and counting it would
// make it a second candidate and suppress a switch that is perfectly resolvable.
func TestLoadSwitchOneTerminalResistorIsNotAShunt(t *testing.T) {
	d := loadSwitchDesign()
	r9 := lsComponent("R9", "RES", "")
	r9.Value = &ir.Quantity{Input: "0R02", Value: lsPtr(0.02), Unit: classify.UnitOhm}
	d.Components = append(d.Components, r9)
	d.Nets[1].Connections = append(d.Nets[1].Connections, conn("R9", "1"))
	sw, ok := resolveOne(t, d)
	if !ok {
		t.Fatal("a half-connected resistor must not compete for the shunt role")
	}
	if sw.Sense != "R1" {
		t.Errorf("sense = %q, want R1", sw.Sense)
	}
}

// TestLoadSwitchAmbiguousControllerIsSilent: two parts on the gate net both declaring an overcurrent
// threshold cannot be told apart, so no verdict.
func TestLoadSwitchAmbiguousControllerIsSilent(t *testing.T) {
	d := loadSwitchDesign()
	d.Components = append(d.Components, lsComponent("U2", "CTRL", "DEMO-HSS"))
	d.Nets[0].Connections = append(d.Nets[0].Connections, conn("U2", "1"))
	if _, ok := resolveOne(t, d); ok {
		t.Error("two controllers on one gate net must resolve no switch")
	}
}

// TestLoadSwitchControllerIsIdentifiedByItsThreshold: the controller is whatever part on the gate net
// declares an overcurrent threshold, not whatever part is an IC. A gate driver seeded with a spec that
// states no threshold is not a current-limiting controller.
func TestLoadSwitchControllerIsIdentifiedByItsThreshold(t *testing.T) {
	m := NewModelWithParams(loadSwitchDesign(), nil, param.ParamSet{
		"DEMO-HSS":  nfetSpec("DEMO-HSS", 3, 0), // seeded, but states no V(OCP)
		"DEMO-NFET": nfetSpec("DEMO-NFET", 3, 0.02),
	})
	if sw := ExternalFetLoadSwitches(m); len(sw) != 0 {
		t.Errorf("a part with no overcurrent threshold is not a controller, got %+v", sw)
	}
}

// TestLoadSwitchSilentWithoutParams: with no params tier there is no threshold, so no controller can
// be identified and no trip current exists. Skip, never a clean pass.
func TestLoadSwitchSilentWithoutParams(t *testing.T) {
	if sw := ExternalFetLoadSwitches(NewModel(loadSwitchDesign())); len(sw) != 0 {
		t.Errorf("want no switches with no seeded params, got %+v", sw)
	}
}

// TestLoadSwitchGateOnTwoNetsIsSilent: a transistor whose gate terminals land on two different nets is
// not the single pass element this resolver reasons about.
func TestLoadSwitchGateOnTwoNetsIsSilent(t *testing.T) {
	d := loadSwitchDesign()
	d.Libraries[0].Parts[1] = lsPart("FET", "G", "GATE", "S")
	d.Nets[2].Connections = []*ir.Connection{conn("U1", "3"), conn("R1", "2")}
	d.Nets = append(d.Nets, lsNet("GATE2", conn("Q1", "2")))
	if _, ok := resolveOne(t, d); ok {
		t.Error("a FET with two gate nets must resolve no switch")
	}
}

// TestLoadSwitchOnResistanceAbsentIsNil: an unseeded on-resistance must read as NOT KNOWN. Zero is a
// perfect switch, so a caller defaulting to it would report a dissipation of nothing.
func TestLoadSwitchOnResistanceAbsentIsNil(t *testing.T) {
	m := NewModelWithParams(loadSwitchDesign(), nil, param.ParamSet{
		"DEMO-HSS":  ctrlSpec("DEMO-HSS", 0.05),
		"DEMO-NFET": nfetSpec("DEMO-NFET", 3, 0),
	})
	sw := ExternalFetLoadSwitches(m)
	if len(sw) != 1 {
		t.Fatalf("want 1 switch, got %d", len(sw))
	}
	if sw[0].OnResistance != nil {
		t.Errorf("OnResistance = %+v, want nil when the FET states none", sw[0].OnResistance)
	}
}

// TestLoadSwitchHighestThresholdBinds: where a controller states several thresholds, the highest is
// the most current the switch passes before it acts, which is the worst case for everything
// downstream. Taking the lowest would under-report the trip current and hide a real over-current.
func TestLoadSwitchHighestThresholdBinds(t *testing.T) {
	spec := ctrlSpec("DEMO-HSS", 0.05)
	low := ctrlSpec("DEMO-HSS", 0.02).Parameters[0]
	spec.Parameters = append([]*parampb.Parameter{low}, spec.Parameters...)
	m := NewModelWithParams(loadSwitchDesign(), nil, param.ParamSet{
		"DEMO-HSS":  spec,
		"DEMO-NFET": nfetSpec("DEMO-NFET", 3, 0.02),
	})
	sw := ExternalFetLoadSwitches(m)
	if len(sw) != 1 {
		t.Fatalf("want 1 switch, got %d", len(sw))
	}
	if !QuantityEqual(sw[0].TripAmps, 5) {
		t.Errorf("trip = %gA, want 5 (the HIGHEST threshold, 50mV, binds)", sw[0].TripAmps)
	}
}

// TestLoadSwitchWorstOnResistanceBinds: a real sheet states RDS(on) several times under different
// gate drives and junction temperatures. The HIGHEST is the one a thermal argument has to survive, so
// reporting the typical row would understate what the FET dissipates.
func TestLoadSwitchWorstOnResistanceBinds(t *testing.T) {
	spec := nfetSpec("DEMO-NFET", 3, 0.02)
	// The worst row is listed LAST, so the selection has to walk past a lower one to reach it.
	hot := nfetSpec("DEMO-NFET", 3, 0.031).Parameters[1]
	spec.Parameters = append(spec.Parameters, hot)
	m := NewModelWithParams(loadSwitchDesign(), nil, param.ParamSet{
		"DEMO-HSS":  ctrlSpec("DEMO-HSS", 0.05),
		"DEMO-NFET": spec,
	})
	sw := ExternalFetLoadSwitches(m)
	if len(sw) != 1 {
		t.Fatalf("want 1 switch, got %d", len(sw))
	}
	if !QuantityEqual(sw[0].OnResistance.Value.GetMax(), 0.031) {
		t.Errorf("on-resistance = %gOhm, want the worst row at 0.031",
			sw[0].OnResistance.Value.GetMax())
	}
}

// TestLoadSwitchTransistorIsNotAController: a second transistor on the gate net is never a candidate
// controller, whatever its spec says. Without that exclusion it would count as a second candidate and
// the ambiguity guard would suppress a switch that is perfectly resolvable.
func TestLoadSwitchTransistorIsNotAController(t *testing.T) {
	d := loadSwitchDesign()
	d.Components = append(d.Components, lsComponent("Q9", "RES", "DEMO-HSS"))
	d.Nets[0].Connections = append(d.Nets[0].Connections, conn("Q9", "1"))
	sw, ok := resolveOne(t, d)
	if !ok {
		t.Fatal("a transistor on the gate net must not compete for the controller role")
	}
	if sw.Controller != "U1" {
		t.Errorf("controller = %q, want U1", sw.Controller)
	}
}

// TestOcpThresholdLimitsConvertsMillivolts is the extractor half of agni issue 148. Real controller
// sheets print this row in MILLIVOLTS, and it used to fail the unit gate: no threshold row, so no load
// switch resolved, so the item scored a PASS on a check that never ran.
//
// The two spellings are ONE datasheet row written twice, so the assertion is equality rather than mere
// presence. 50 mV and 0.05 V must produce the identical double, which is what lets the rest of the
// engine compare a converted row against a threshold without a tolerance.
func TestOcpThresholdLimitsConvertsMillivolts(t *testing.T) {
	volts := ctrlSpec("X", 0.05)
	inVolts := OcpThresholdLimits(volts)
	if len(inVolts) != 1 {
		t.Fatalf("want 1 threshold row, got %d", len(inVolts))
	}

	millivolts := ctrlSpec("X", 0.05)
	millivolts.Parameters[0].Unit = "mV"
	millivolts.Parameters[0].Value.Max = lsPtr(50)
	inMillivolts := OcpThresholdLimits(millivolts)
	if len(inMillivolts) != 1 {
		t.Fatalf("a millivolt row must convert, not vanish: got %d rows", len(inMillivolts))
	}
	if got := inMillivolts[0].Unit; got != "V" {
		t.Errorf("returned unit = %q, want V; the extractor's contract is that its rows are in base units", got)
	}
	if got, want := inMillivolts[0].Value.GetMax(), inVolts[0].Value.GetMax(); got != want {
		t.Errorf("50mV converted to %v, want exactly the %v the volt-spelled row carries", got, want)
	}
	if millivolts.Parameters[0].Unit != "mV" {
		t.Error("the seeded spec was rewritten; a citation and the params panel must keep showing mV")
	}
}

// TestOcpThresholdLimitsRefusesUnrecognizedUnits: converting the units it knows must not soften the
// refusal of the ones it does not. An unrecognized unit is still no row, because a guessed scale on a
// number that gets divided into an ampere rating is exactly the confident wrong answer this rule
// family costs the most on.
func TestOcpThresholdLimitsRefusesUnrecognizedUnits(t *testing.T) {
	for _, unit := range []string{"dBm", "", "KV"} {
		spec := ctrlSpec("X", 0.05)
		spec.Parameters[0].Unit = unit
		if got := OcpThresholdLimits(spec); len(got) != 0 {
			t.Errorf("unit %q must be refused, got %+v", unit, got)
		}
	}
}

// TestOcpThresholdLimitsSymbolGate: the alias set is narrow because the number is divided by a
// resistance and reported as an ampere rating, so a symbol that could mean something else would
// produce a confident wrong current.
func TestOcpThresholdLimitsSymbolGate(t *testing.T) {
	spec := ctrlSpec("X", 0.05)
	spec.Parameters[0].Symbol = "VREF"
	if got := OcpThresholdLimits(spec); len(got) != 0 {
		t.Errorf("VREF is not an overcurrent threshold: got %+v", got)
	}
}

// TestDrainCurrentLimitsExcludesPulsed: pulsed drain current is a much larger number under a
// duty-cycle condition. Crediting it against a steady trip current turns a real over-current into a
// pass, which is the silent direction.
func TestDrainCurrentLimitsExcludesPulsed(t *testing.T) {
	spec := nfetSpec("X", 3, 0)
	if got := DrainCurrentLimits(spec); len(got) != 1 {
		t.Fatalf("want 1 continuous rating, got %d", len(got))
	}
	spec.Parameters[0].Symbol = "IDM"
	if got := DrainCurrentLimits(spec); len(got) != 0 {
		t.Errorf("IDM (pulsed) must not be read as a continuous rating: got %+v", got)
	}
}

// TestDrainCurrentLimitsRequiresAbsoluteMax: continuous drain current IS an absolute maximum on a real
// sheet, so a characteristic row carrying the same symbol is not the rating.
func TestDrainCurrentLimitsRequiresAbsoluteMax(t *testing.T) {
	spec := nfetSpec("X", 3, 0)
	spec.Parameters[0].LimitKind = parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC
	if got := DrainCurrentLimits(spec); len(got) != 0 {
		t.Errorf("a non-abs-max row must be skipped: got %+v", got)
	}
}

// TestOnResistanceAcceptsBothOhmSpellings: "Ohm" (what the hand-encoded corpus writes) and the symbol
// are two spellings of ONE unit, so accepting both is normalization rather than conversion. The
// returned unit is the canonical symbol whichever way the spec spelled it, so a rule comparing a
// datasheet resistance against a design-side one compares two identical strings.
func TestOnResistanceAcceptsBothOhmSpellings(t *testing.T) {
	// "Ω" is the deprecated OHM SIGN, byte-different from and visually identical to the canonical
	// U+03A9. Written as an escape because a literal here would be indistinguishable by eye from the
	// entry beside it, so a duplicated row would look like coverage.
	for _, u := range []string{"Ohm", "ohm", param.UnitOhm, "\u2126"} {
		spec := nfetSpec("X", 3, 0.02)
		spec.Parameters[1].Unit = u
		got := OnResistanceLimits(spec)
		if len(got) != 1 {
			t.Errorf("unit %q: want 1 on-resistance row, got %d", u, len(got))
			continue
		}
		if got[0].Unit != param.UnitOhm {
			t.Errorf("unit %q: returned %q, want the canonical ohm symbol", u, got[0].Unit)
		}
	}
}

// TestOnResistanceConvertsMilliohms: a MILLIOHM is the ordinary way a modern FET sheet prints
// RDS(on), so the spelling that used to read as no row at all was the common one. 20 mΩ and 0.02 Ω are
// one row written twice and must land on the same double (agni issue 148).
func TestOnResistanceConvertsMilliohms(t *testing.T) {
	ohms := OnResistanceLimits(nfetSpec("X", 3, 0.02))
	if len(ohms) != 1 {
		t.Fatalf("want 1 on-resistance row, got %d", len(ohms))
	}
	for _, u := range []string{"mOhm", "mΩ"} {
		spec := nfetSpec("X", 3, 20)
		spec.Parameters[1].Unit = u
		got := OnResistanceLimits(spec)
		if len(got) != 1 {
			t.Errorf("unit %q: a milliohm row must convert, got %d rows", u, len(got))
			continue
		}
		if got[0].Value.GetMax() != ohms[0].Value.GetMax() {
			t.Errorf("unit %q: 20%s converted to %v, want exactly the %v the ohm-spelled row carries",
				u, u, got[0].Value.GetMax(), ohms[0].Value.GetMax())
		}
	}
}
