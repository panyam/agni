package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// fetSpec hand-builds a MOSFET spec with a drain-source breakdown row, the shape a real FET datasheet
// prints it in (an absolute maximum, unlike a regulator's output).
func fetSpec(mpn string, vdss float64) *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	return &parampb.PartSpec{
		Mpn: mpn, Manufacturer: "Acme",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "ACME-FET Rev D", Vendor: "Acme"}},
		Parameters: []*parampb.Parameter{{
			Name: "Drain-Source Voltage", Symbol: "VDSS",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
			Value:             &parampb.RangeValue{Max: f(vdss)},
			Unit:              "V",
			Conditions:        []*parampb.Condition{{Symbol: "TA", Eq: f(25), Unit: "C", Raw: "TA = 25C"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{
				DocRef: "ds", Page: 1, TableOrFigure: "Absolute Maximum Ratings",
				Method: "hand", Confidence: 1,
			},
		}},
	}
}

// fetDesign puts Q1 (a FET) on the named rail. When regRef is non-empty a regulator of that ref-des
// also sits on the rail, so the rail's voltage has DATASHEET evidence rather than only its name.
func fetDesign(railName, regRef string) *ir.Design {
	d := &ir.Design{
		Components: []*ir.Component{{
			RefDes: "Q1", Attributes: map[string]string{"MPN": "ACME-FET"},
			Prov: &ir.Provenance{SourceFile: "t"},
		}},
		Nets: []*ir.Net{{
			Name: railName, Prov: &ir.Provenance{SourceFile: "t"},
			Connections: []*ir.Connection{{ComponentRef: "Q1", PinRef: "1"}},
		}},
	}
	if regRef != "" {
		d.Components = append(d.Components, &ir.Component{
			RefDes: regRef, Attributes: map[string]string{"MPN": "ACME-REG"},
			Prov: &ir.Provenance{SourceFile: "t"},
		})
		d.Nets[0].Connections = append(d.Nets[0].Connections,
			&ir.Connection{ComponentRef: regRef, PinRef: "1"})
	}
	return d
}

// TestFetVdssBelowRailFromNetName: the name-derived path. A 50V FET on a +60V rail is over its
// breakdown, and with no driving part on the rail the only evidence is the net name — so exactly ONE
// citation (the FET's), because a naming convention is not a document.
func TestFetVdssBelowRailFromNetName(t *testing.T) {
	m := check.NewModelWithParams(fetDesign("+60V", ""), nil,
		param.ParamSet{"ACME-FET": fetSpec("ACME-FET", 50)})
	fs := fetVdssBelowRail.Eval(m)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (50V FET on a 60V rail), got %d: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Subject != "Q1" {
		t.Errorf("subject = %q, want Q1", f.Subject)
	}
	if !strings.Contains(f.Message, "from the net name") {
		t.Errorf("message must say the rail voltage came from the name: %s", f.Message)
	}
	if len(f.DatasheetProv) != 1 {
		t.Errorf("a name-derived rail earns no citation: want 1, got %d", len(f.DatasheetProv))
	}
}

// TestFetVdssBelowRailFromDatasheet: with a regulator on the rail declaring its output, the voltage is
// a VENDOR value, so it earns a second citation and the message says which part it came from. This is
// the WS3-028 evidence upgrade applied to a second rule.
func TestFetVdssBelowRailFromDatasheet(t *testing.T) {
	m := check.NewModelWithParams(fetDesign("VBUS", "U1"), nil, param.ParamSet{
		"ACME-FET": fetSpec("ACME-FET", 50),
		"ACME-REG": regSpec("ACME-REG", 60, "hand", 1),
	})
	fs := fetVdssBelowRail.Eval(m)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(fs), fs)
	}
	f := fs[0]
	if !strings.Contains(f.Message, "U1 datasheet output") {
		t.Errorf("message must name the part the rail voltage came from: %s", f.Message)
	}
	// Two citations: the FET's breakdown and the regulator's output. VBUS is a rail by name but
	// carries no voltage TOKEN, so this finding exists ONLY because of the datasheet path.
	if len(f.DatasheetProv) != 2 {
		t.Fatalf("want 2 citations (FET and the driving regulator), got %d: %+v", len(f.DatasheetProv), f.DatasheetProv)
	}
	if f.DatasheetProv[0].Doc != "ACME-FET Rev D" || f.DatasheetProv[1].Doc != "ACME-REG Rev A" {
		t.Errorf("citations = %+v, want the FET's doc then the regulator's", f.DatasheetProv)
	}
}

// TestFetVdssWithinRating: a 50V FET on a 12V rail is silent. Guards the comparison direction, which a
// sign error would invert while every other assertion still passed.
func TestFetVdssWithinRating(t *testing.T) {
	m := check.NewModelWithParams(fetDesign("+12V", ""), nil,
		param.ParamSet{"ACME-FET": fetSpec("ACME-FET", 50)})
	if fs := fetVdssBelowRail.Eval(m); len(fs) != 0 {
		t.Errorf("50V FET on a 12V rail must be silent, got %+v", fs)
	}
}

// TestFetVdssUnknownRailVoltage: a rail whose name carries no voltage token and which no seeded part
// drives yields no number, so the rule skips. Silence here is "I could not tell", and the rule must
// not invent a voltage to compare against.
//
// VBUS specifically, because it IS a rail by the naming lexicon while carrying no voltage token. An
// earlier draft used VSYS, which the lexicon does not read as a rail at all — so the test passed by
// being skipped one step earlier and proved nothing about the unknown-voltage path.
func TestFetVdssUnknownRailVoltage(t *testing.T) {
	m := check.NewModelWithParams(fetDesign("VBUS", ""), nil,
		param.ParamSet{"ACME-FET": fetSpec("ACME-FET", 50)})
	if fs := fetVdssBelowRail.Eval(m); len(fs) != 0 {
		t.Errorf("unknown rail voltage must yield no finding, got %+v", fs)
	}
}

// TestFetVdssSilentWithoutParams: the params tier is a per-run injection, so an unseeded design has
// nothing to compare and Available gates the rule to not-applicable rather than letting it read clean.
func TestFetVdssSilentWithoutParams(t *testing.T) {
	m := check.NewModel(fetDesign("+60V", ""))
	if fs := fetVdssBelowRail.Eval(m); len(fs) != 0 {
		t.Errorf("want no findings with no seeded params, got %+v", fs)
	}
	if ok, reason := check.Available(fetVdssBelowRail, m); ok || reason == "" {
		t.Errorf("Available = (%v, %q), want not-applicable with a reason", ok, reason)
	}
}

// TestFetVdssIgnoresGround: ground is a rail by the engine's definition but carries no voltage to
// compare, so it is excluded explicitly. Without the guard a FET's ground connection would reach the
// name-derived path and be judged against whatever that returned.
func TestFetVdssIgnoresGround(t *testing.T) {
	d := fetDesign("+60V", "")
	d.Nets = append(d.Nets, &ir.Net{
		Name: "GND", Prov: &ir.Provenance{SourceFile: "t"},
		Connections: []*ir.Connection{{ComponentRef: "Q1", PinRef: "2"}},
	})
	m := check.NewModelWithParams(d, nil, param.ParamSet{"ACME-FET": fetSpec("ACME-FET", 50)})
	fs := fetVdssBelowRail.Eval(m)
	if len(fs) != 1 {
		t.Fatalf("want exactly 1 finding (the +60V rail, not GND), got %d: %+v", len(fs), fs)
	}
	if strings.Contains(fs[0].Message, "GND") {
		t.Errorf("ground must not be compared: %s", fs[0].Message)
	}
}
