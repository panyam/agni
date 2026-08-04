package relations

import (
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// The fixtures below are duplicated from the check/builtin-rule test suites (the projector tests
// moved here with Facts in issue 10, but the hand-built designs and specs are unexported test
// helpers). Duplicating a proven test builder into the package that needs it is the same call the
// 2b rule extraction made — a test helper is cheaper to copy than to export.

// tnet builds a net with "refdes.pin" connections.
func tnet(name string, conns ...string) *ir.Net {
	n := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}}
	for _, c := range conns {
		p := strings.SplitN(c, ".", 2)
		n.Connections = append(n.Connections, &ir.Connection{ComponentRef: p[0], PinRef: p[1]})
	}
	return n
}

// capSpec hand-builds a seeded cap: rated voltage as a machine-comparable recommended-operating row
// (the shape a cap datasheet's ratings table yields).
func capSpec(mpn string, rated float64) *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	return &parampb.PartSpec{
		Mpn:          mpn,
		Manufacturer: "Acme",
		Docs:         []*parampb.SourceDoc{{Id: "ds", Title: "ACME-CAP Rev C", Vendor: "Acme"}},
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

// capDesign places one capacitor C1 (pins 1/2 passive) with pin 1 on railNet and pin 2 on GND,
// joined via the MPN attribute.
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

// supplyDesign places one part U1 with a POWER_IN pin on netName, joined to a spec by MPN attribute
// (or a BOM line when viaBomLine).
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

// esdSpec hand-builds a part whose datasheet declares a system-level (IEC) ESD absolute-max rating.
func esdSpec(mpn string, volts float64) *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	return &parampb.PartSpec{
		Mpn: mpn, Manufacturer: "Agni",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "DEMO-XCVR Rev A", Vendor: "Agni"}},
		Parameters: []*parampb.Parameter{{
			Name:              "ESD (IEC 61000-4-2, bus pins)",
			Symbol:            "V_ESD",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
			Value:             &parampb.RangeValue{Max: f(volts)},
			Unit:              "V",
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_UNCONDITIONAL,
			Attributes:        map[string]string{"esd_test_model": "iec"}, // system-level (WS3-077)
			Prov:              &parampb.ParamProvenance{DocRef: "ds", Page: 1, TableOrFigure: "ESD Ratings", Method: "hand", Confidence: 1},
		}},
	}
}

// ldoRecommendedSpec hand-builds a supply with a two-sided recommended-operating VDD row.
func ldoRecommendedSpec(mpn string, min, max float64) *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	return &parampb.PartSpec{
		Mpn:          mpn,
		Manufacturer: "Acme",
		Docs:         []*parampb.SourceDoc{{Id: "ds", Title: "ACME-LDO Rev B", Vendor: "Acme"}},
		Parameters: []*parampb.Parameter{{
			Name: "Supply voltage, recommended", Symbol: "VDD",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING,
			Value:             &parampb.RangeValue{Min: f(min), Max: f(max)},
			Unit:              "V",
			Conditions:        []*parampb.Condition{{Symbol: "TA", Eq: f(25), Unit: "C", Raw: "TA = 25C"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{
				DocRef: "ds", Page: 6, TableOrFigure: "Recommended Operating Conditions",
				Method: "hand", Confidence: 1,
			},
		}},
	}
}

// mm builds a nanometer point from millimeter coordinates.
func mm(x, y float64) *geom.Point {
	return &geom.Point{X: int64(x * 1e6), Y: int64(y * 1e6)}
}

// drcBoard: every board relation exercised — a sub-floor trace, a small drill, a thin annular ring,
// and clean copper — reused from the DRC board fixture so the board facts have a real tier to project.
func drcBoard() *geom.BoardGeometry {
	seg := func(x1, y1, x2, y2, wMM float64, layer string) *geom.TrackSegment {
		return &geom.TrackSegment{A: mm(x1, y1), B: mm(x2, y2), Width: int64(wMM * 1e6), Layer: layer}
	}
	via := func(x, y, sizeMM, drillMM float64) *geom.Via {
		return &geom.Via{At: mm(x, y), Size: int64(sizeMM * 1e6), Drill: int64(drillMM * 1e6)}
	}
	return &geom.BoardGeometry{UnitNm: 1, Nets: []*geom.NetCopper{
		{Net: "THIN", Segments: []*geom.TrackSegment{seg(10, 10, 14, 10, 0.05, "F.Cu")}},
		{Net: "CLOSE_A", Segments: []*geom.TrackSegment{seg(10, 12, 14, 12, 0.15, "F.Cu")}},
		{Net: "CLOSE_B", Segments: []*geom.TrackSegment{
			seg(10, 12.19, 14, 12.19, 0.15, "F.Cu"),
			seg(10, 12.19, 14, 12.19, 0.15, "B.Cu"),
		}},
		{Net: "SAMENET", Segments: []*geom.TrackSegment{
			seg(20, 20, 24, 20, 0.15, "F.Cu"),
			seg(20, 20.17, 24, 20.17, 0.15, "F.Cu"),
		}},
		{Net: "SMALLHOLE", Vias: []*geom.Via{via(30, 30, 0.4, 0.1)}},
		{Net: "THINRING", Vias: []*geom.Via{via(32, 30, 0.5, 0.4)}},
		{Net: "CLEAN", Segments: []*geom.TrackSegment{seg(40, 40, 44, 40, 0.25, "F.Cu")},
			Vias: []*geom.Via{via(40, 42, 0.8, 0.4)}},
	}}
}
