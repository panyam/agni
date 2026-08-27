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
		c.Mpn = mpn
	}
	return c
}

func lsConn(ref, pin string) *ir.Connection { return &ir.Connection{ComponentRef: ref, PinRef: pin} }

// loadSwitchBoard is a controller-based high-side switch: U1 drives Q1's gate and senses across R1,
// whose value the design states in ohms. senseOhms sets the trip current against the controller's
// threshold.
func loadSwitchBoard(senseOhms float64) *ir.Design {
	r1 := lsComp("R1", "RES", "")
	r1.Value = &ir.Quantity{Input: "sense", Value: lsF(senseOhms), Unit: classify.UnitOhm}
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			lsPartType("CTRL", "GATE", "SNSP", "SNSN"),
			lsPartType("FET", "G", "D", "S"),
			lsPartType("RES", "1", "2"),
		}}},
		Components: []*ir.Component{
			lsComp("U1", "CTRL", "DEMO-HSS-CTRL"),
			lsComp("Q1", "FET", "DEMO-NFET-40"),
			r1,
		},
		Nets: []*ir.Net{
			{Name: "GATE_DRV", Prov: &ir.Provenance{SourceFile: "t"},
				Connections: []*ir.Connection{lsConn("U1", "1"), lsConn("Q1", "1")}},
			{Name: "VIN", Prov: &ir.Provenance{SourceFile: "t"},
				Connections: []*ir.Connection{lsConn("U1", "2"), lsConn("R1", "1")}},
			{Name: "VSW", Prov: &ir.Provenance{SourceFile: "t"},
				Connections: []*ir.Connection{lsConn("U1", "3"), lsConn("R1", "2"), lsConn("Q1", "2")}},
			{Name: "VOUT", Prov: &ir.Provenance{SourceFile: "t"},
				Connections: []*ir.Connection{lsConn("Q1", "3")}},
		},
	}
}

func hssCtrlSpec(ocpVolts float64) *parampb.PartSpec {
	return &parampb.PartSpec{
		Mpn: "DEMO-HSS-CTRL", Manufacturer: "Agni Conformance Works",
		DeviceClass: "high-side switch controller",
		Docs:        []*parampb.SourceDoc{{Id: "hss", Title: "DEMO-HSS-CTRL Rev A", Vendor: "Agni Conformance Works"}},
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

// passFetSpec builds the external MOSFET's spec. Each entry of idAmps becomes a continuous drain
// rating row; rdsOhms > 0 adds an on-resistance row.
func passFetSpec(rdsOhms float64, idAmps ...float64) *parampb.PartSpec {
	s := &parampb.PartSpec{
		Mpn: "DEMO-NFET-40", Manufacturer: "Agni Conformance Works",
		DeviceClass: "nfet",
		Docs:        []*parampb.SourceDoc{{Id: "nfet", Title: "DEMO-NFET-40 Rev C", Vendor: "Agni Conformance Works"}},
	}
	for _, a := range idAmps {
		s.Parameters = append(s.Parameters, &parampb.Parameter{
			Name: "Drain Current - Continuous", Symbol: "ID",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
			Value:             &parampb.RangeValue{Max: lsF(a)},
			Unit:              "A",
			Conditions:        []*parampb.Condition{{Symbol: "TA", Eq: lsF(25), Unit: "C", Raw: "TA = 25C"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{DocRef: "nfet", Page: 1,
				TableOrFigure: "Absolute Maximum Ratings", Method: "hand", Confidence: 1},
		})
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

func runLoadSwitchRule(d *ir.Design, set param.ParamSet) []check.Finding {
	return loadSwitchTripAboveFetRating.Findings(check.NewModelWithParams(d, nil, set))
}

// TestLoadSwitchTripAboveFetRatingFires: 50mV across a 10mOhm shunt trips at 5A, above the external
// FET's 3A continuous rating, so the current limit never protects the pass element.
func TestLoadSwitchTripAboveFetRatingFires(t *testing.T) {
	fs := runLoadSwitchRule(loadSwitchBoard(0.01), param.ParamSet{
		"DEMO-HSS-CTRL": hssCtrlSpec(0.05),
		"DEMO-NFET-40":  passFetSpec(0.02, 3),
	})
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (5A trip on a 3A FET), got %d: %+v", len(fs), fs)
	}
	f := fs[0]
	if check.EntityRef(f.Subject) != "Q1" {
		t.Errorf("subject = %q, want Q1 (the part whose rating is exceeded)", f.Subject)
	}
	if f.Subject.Kind != check.KindComponent {
		t.Errorf("kind = %q, want component", f.Subject.Kind)
	}
	// The three inputs must all be visible: the computed trip current, the shunt it came from, and
	// the rating it was judged against. A message naming only the verdict leaves a reviewer unable to
	// tell which of the three numbers is the wrong one.
	for _, want := range []string{"5A", "R1", "V(OCP)", "3A", "U1"} {
		if !strings.Contains(f.Message, want) {
			t.Errorf("message must mention %q: %s", want, f.Message)
		}
	}
	if len(f.DatasheetProv) != 2 {
		t.Fatalf("want 2 citations (the FET's rating and the controller's threshold), got %d: %+v",
			len(f.DatasheetProv), f.DatasheetProv)
	}
	if f.DatasheetProv[0].Doc != "DEMO-NFET-40 Rev C" || f.DatasheetProv[1].Doc != "DEMO-HSS-CTRL Rev A" {
		t.Errorf("citations = %+v, want the endangered FET's doc first then the controller's", f.DatasheetProv)
	}
	if f.Prov == nil {
		t.Error("a component finding must carry the component's provenance")
	}
}

// TestLoadSwitchTripWithinFetRating guards the comparison DIRECTION, which a sign error would invert
// while every other assertion in this file still passed. 50mV across 50mOhm trips at 1A, under the
// FET's 3A rating.
func TestLoadSwitchTripWithinFetRating(t *testing.T) {
	fs := runLoadSwitchRule(loadSwitchBoard(0.05), param.ParamSet{
		"DEMO-HSS-CTRL": hssCtrlSpec(0.05),
		"DEMO-NFET-40":  passFetSpec(0.02, 3),
	})
	if len(fs) != 0 {
		t.Errorf("a 1A trip on a 3A FET must be silent, got %+v", fs)
	}
}

// TestLoadSwitchTripEqualsRatingIsSilent: at exactly the rating the design is at the vendor's own
// number, not past it. The rule claims only the unambiguous half, so the boundary is not a finding.
func TestLoadSwitchTripEqualsRatingIsSilent(t *testing.T) {
	fs := runLoadSwitchRule(loadSwitchBoard(0.01), param.ParamSet{
		"DEMO-HSS-CTRL": hssCtrlSpec(0.05),
		"DEMO-NFET-40":  passFetSpec(0.02, 5),
	})
	if len(fs) != 0 {
		t.Errorf("a 5A trip on a 5A FET must be silent, got %+v", fs)
	}
}

// TestLoadSwitchLowestRatingBinds: a part is endangered at its weakest rating, so a second, higher
// drain-current row must not excuse the over-current. Taking the highest would silence this design.
func TestLoadSwitchLowestRatingBinds(t *testing.T) {
	fs := runLoadSwitchRule(loadSwitchBoard(0.01), param.ParamSet{
		"DEMO-HSS-CTRL": hssCtrlSpec(0.05),
		"DEMO-NFET-40":  passFetSpec(0.02, 8, 3),
	})
	if len(fs) != 1 {
		t.Fatalf("want 1 finding: the LOWEST rating binds, got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "3A") {
		t.Errorf("message must judge against the 3A row, not the 8A one: %s", fs[0].Message)
	}
}

// TestLoadSwitchReportsEffectiveOnResistance: the effective on-resistance of a controller-based switch
// is the external FET's RDS(on), which is what item-26-style sizing needs and what no number on the
// controller's sheet answers.
//
// It is quoted in the message but deliberately NOT added to DatasheetProv: the verdict does not rest
// on it, and the review's data-trust gate rates a finding by its WEAKEST citation, so listing an
// unused value could drag a genuine failure to provisional.
func TestLoadSwitchReportsEffectiveOnResistance(t *testing.T) {
	fs := runLoadSwitchRule(loadSwitchBoard(0.01), param.ParamSet{
		"DEMO-HSS-CTRL": hssCtrlSpec(0.05),
		"DEMO-NFET-40":  passFetSpec(0.02, 3),
	})
	if len(fs) != 1 {
		t.Fatalf("want 1 finding, got %d", len(fs))
	}
	if !strings.Contains(fs[0].Message, "effective on-resistance") || !strings.Contains(fs[0].Message, "RDS(on)") {
		t.Errorf("message must report the switch's effective on-resistance: %s", fs[0].Message)
	}
	if len(fs[0].DatasheetProv) != 2 {
		t.Errorf("the on-resistance must not add a citation: want 2, got %d", len(fs[0].DatasheetProv))
	}
}

// TestLoadSwitchWithoutOnResistanceSaysNothing: an unseeded RDS(on) must produce no clause at all.
// Reporting a zero or an empty figure would read as a perfect switch.
func TestLoadSwitchWithoutOnResistanceSaysNothing(t *testing.T) {
	fs := runLoadSwitchRule(loadSwitchBoard(0.01), param.ParamSet{
		"DEMO-HSS-CTRL": hssCtrlSpec(0.05),
		"DEMO-NFET-40":  passFetSpec(0, 3),
	})
	if len(fs) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(fs), fs)
	}
	if strings.Contains(fs[0].Message, "on-resistance") {
		t.Errorf("no RDS(on) row means no on-resistance clause: %s", fs[0].Message)
	}
	if len(fs[0].DatasheetProv) != 2 {
		t.Errorf("want 2 citations, got %d", len(fs[0].DatasheetProv))
	}
}

// TestLoadSwitchUnseededFetIsSilent: the pass element carries the rating being exceeded, so without it
// there is nothing to judge. Skip, never pass.
func TestLoadSwitchUnseededFetIsSilent(t *testing.T) {
	fs := runLoadSwitchRule(loadSwitchBoard(0.01), param.ParamSet{
		"DEMO-HSS-CTRL": hssCtrlSpec(0.05),
	})
	if len(fs) != 0 {
		t.Errorf("an unseeded pass FET must yield no verdict, got %+v", fs)
	}
}

// TestLoadSwitchFetWithNoDrainRatingIsSilent: a seeded FET whose spec states no continuous drain
// current is the same gap as an unseeded one.
func TestLoadSwitchFetWithNoDrainRatingIsSilent(t *testing.T) {
	fs := runLoadSwitchRule(loadSwitchBoard(0.01), param.ParamSet{
		"DEMO-HSS-CTRL": hssCtrlSpec(0.05),
		"DEMO-NFET-40":  passFetSpec(0.02),
	})
	if len(fs) != 0 {
		t.Errorf("a FET with no drain-current row must yield no verdict, got %+v", fs)
	}
}

// TestLoadSwitchSilentWithoutParams: the params tier is a per-run injection, so an unseeded design has
// no threshold to compute from and Available gates the rule to not-applicable rather than clean.
func TestLoadSwitchSilentWithoutParams(t *testing.T) {
	m := check.NewModel(loadSwitchBoard(0.01))
	if fs := loadSwitchTripAboveFetRating.Findings(m); len(fs) != 0 {
		t.Errorf("want no findings with no seeded params, got %+v", fs)
	}
	if ok, reason := check.Available(loadSwitchTripAboveFetRating, m); ok || reason == "" {
		t.Errorf("Available = (%v, %q), want not-applicable with a reason", ok, reason)
	}
}
