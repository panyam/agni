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
		c.Attributes = map[string]string{"MPN": mpn}
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
func TestLoadSwitchNeedsTheGateRole(t *testing.T) {
	d := loadSwitchDesign()
	d.Libraries[0].Parts[1] = lsPart("FET", "P1", "P2", "P3")
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

// TestOcpThresholdLimitsUnitGate: the threshold must be in volts exactly. A millivolt row reads as no
// row, which is the standing unlike-units posture and fails toward silence rather than toward a
// current a thousand times too large.
func TestOcpThresholdLimitsUnitGate(t *testing.T) {
	spec := ctrlSpec("X", 0.05)
	if got := OcpThresholdLimits(spec); len(got) != 1 {
		t.Fatalf("want 1 threshold row, got %d", len(got))
	}
	spec.Parameters[0].Unit = "mV"
	spec.Parameters[0].Value.Max = lsPtr(50)
	if got := OcpThresholdLimits(spec); len(got) != 0 {
		t.Errorf("a millivolt row must be skipped, not converted: got %+v", got)
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
// are two spellings of ONE unit, so accepting both is normalization. A milliohm row is a different
// unit and stays skipped.
func TestOnResistanceAcceptsBothOhmSpellings(t *testing.T) {
	for _, u := range []string{"Ohm", "Ω"} {
		spec := nfetSpec("X", 3, 0.02)
		spec.Parameters[1].Unit = u
		if got := OnResistanceLimits(spec); len(got) != 1 {
			t.Errorf("unit %q: want 1 on-resistance row, got %d", u, len(got))
		}
	}
	spec := nfetSpec("X", 3, 0.02)
	spec.Parameters[1].Unit = "mOhm"
	if got := OnResistanceLimits(spec); len(got) != 0 {
		t.Errorf("a milliohm row is a different unit and must be skipped: got %+v", got)
	}
}
