package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// regSpec hand-builds a regulator spec whose OUTPUT voltage is stated the way a real one is: a
// recommended-operating row, not an absolute maximum. A rule filtering outputs to ABSOLUTE_MAX would
// find nothing on a real part, which is why OutputVoltageLimits does not constrain the kind.
func regSpec(mpn string, vout float64, method string, confidence float64) *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	return &parampb.PartSpec{
		Mpn:          mpn,
		Manufacturer: "Acme",
		Docs:         []*parampb.SourceDoc{{Id: "ds", Title: "ACME-REG Rev A", Vendor: "Acme"}},
		Parameters: []*parampb.Parameter{{
			Name: "Output voltage", Symbol: "VOUT",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING,
			Value:             &parampb.RangeValue{Max: f(vout)},
			Unit:              "V",
			Conditions:        []*parampb.Condition{{Symbol: "TA", Eq: f(25), Unit: "C", Raw: "TA = 25C"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{
				DocRef: "ds", Page: 6, TableOrFigure: "Electrical Characteristics",
				Method: method, Confidence: confidence,
			},
		}},
	}
}

// railDesign wires a regulator U1 to a load U2 over the given net. When beadRef is non-empty a
// two-terminal part of that ref-des sits between them on a second net, so the pair is one series
// crossing apart rather than sharing a net.
func railDesign(beadRef string) *ir.Design {
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			{Name: "REG", Pins: []*ir.Pin{{Name: "VOUT", Designator: "1"}}},
			{Name: "LOAD", Pins: []*ir.Pin{{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN}}},
		}}},
		Components: []*ir.Component{
			{RefDes: "U1", Sections: []*ir.ComponentSection{{PartRef: "REG", LibraryRef: "lib"}},
				Attributes: map[string]string{"MPN": "ACME-REG"}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U2", Sections: []*ir.ComponentSection{{PartRef: "LOAD", LibraryRef: "lib"}},
				Attributes: map[string]string{"MPN": "ACME-33"}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
	}
	if beadRef == "" {
		d.Nets = []*ir.Net{{Name: "VRAIL", Prov: &ir.Provenance{SourceFile: "t"},
			Connections: []*ir.Connection{
				{ComponentRef: "U1", PinRef: "1", Prov: &ir.Provenance{SourceFile: "t"}},
				{ComponentRef: "U2", PinRef: "1", Prov: &ir.Provenance{SourceFile: "t"}},
			}}}
		return d
	}
	d.Components = append(d.Components, &ir.Component{
		RefDes: beadRef, Attributes: map[string]string{}, Prov: &ir.Provenance{SourceFile: "t"},
	})
	d.Nets = []*ir.Net{
		{Name: "VRAIL", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
			{ComponentRef: "U1", PinRef: "1", Prov: &ir.Provenance{SourceFile: "t"}},
			{ComponentRef: beadRef, PinRef: "1", Prov: &ir.Provenance{SourceFile: "t"}},
		}},
		{Name: "VRAIL_F", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
			{ComponentRef: beadRef, PinRef: "2", Prov: &ir.Provenance{SourceFile: "t"}},
			{ComponentRef: "U2", PinRef: "1", Prov: &ir.Provenance{SourceFile: "t"}},
		}},
	}
	return d
}

func regModel(t *testing.T, d *ir.Design, vout, absMax float64) check.Model {
	t.Helper()
	return check.NewModelWithParams(d, nil, param.ParamSet{
		"ACME-REG": regSpec("ACME-REG", vout, "hand", 1),
		"ACME-33":  ldoSpec("ACME-33", absMax),
	})
}

// TestRegulatorOutputExceedsAbsMax is the WS3-028 acceptance: a param on one part compared against a
// param on ANOTHER, across the net joining them, citing both documents. The subject is the endangered
// part, because that is the one a reviewer opens the datasheet for.
func TestRegulatorOutputExceedsAbsMax(t *testing.T) {
	m := regModel(t, railDesign(""), 5.0, 3.6)
	fs := regulatorOutputExceedsAbsMax.Eval(m)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (5V regulator into a 3.6V abs-max part), got %d: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Subject != "U2" {
		t.Errorf("subject = %q, want U2 (the endangered part)", f.Subject)
	}
	for _, want := range []string{"U1", "VRAIL", "5", "U2", "3.6"} {
		if !strings.Contains(f.Message, want) {
			t.Errorf("message missing %q: %s", want, f.Message)
		}
	}
	// BOTH datasheets, which is the point of the rule and the reason DatasheetProv is a slice.
	if len(f.DatasheetProv) != 2 {
		t.Fatalf("want 2 citations (load and source), got %d: %+v", len(f.DatasheetProv), f.DatasheetProv)
	}
	if f.DatasheetProv[0].Doc != "ACME-33 Rev B" {
		t.Errorf("first citation = %q, want the endangered part's doc", f.DatasheetProv[0].Doc)
	}
	if f.DatasheetProv[1].Doc != "ACME-REG Rev A" {
		t.Errorf("second citation = %q, want the supplying part's doc", f.DatasheetProv[1].Doc)
	}
}

// TestRegulatorOutputWithinRating: the same topology with the regulator inside the load's rating is
// silent. Guards the comparison direction, which a sign error would invert while every other
// assertion still passed.
func TestRegulatorOutputWithinRating(t *testing.T) {
	m := regModel(t, railDesign(""), 3.3, 3.6)
	if fs := regulatorOutputExceedsAbsMax.Eval(m); len(fs) != 0 {
		t.Errorf("3.3V into a 3.6V abs-max part must be silent, got %+v", fs)
	}
}

// TestRegulatorOutputAcrossSeriesElement: a ferrite between regulator and load is ordinary layout and
// must not hide the connection, so the rule walks one series crossing (check.SupplyPathReachHops).
func TestRegulatorOutputAcrossSeriesElement(t *testing.T) {
	d := railDesign("FB1")
	d.Components[2].Sections = []*ir.ComponentSection{{Attributes: map[string]string{"kind": "ferrite"}}}
	d.Components[2].DeviceClasses = []string{"ferrite"}
	m := regModel(t, d, 5.0, 3.6)
	if fs := regulatorOutputExceedsAbsMax.Eval(m); len(fs) != 1 {
		t.Errorf("want the finding to survive one series crossing, got %d: %+v", len(fs), fs)
	}
}

// TestRegulatorOutputSilentWithoutParams: the params tier is a per-run injection, so with no seeded
// set there is nothing to compare. check.Available gates the rule to not-applicable, which is what
// makes an unseeded design read unevaluable rather than clean (the WS3-097 posture).
func TestRegulatorOutputSilentWithoutParams(t *testing.T) {
	m := check.NewModel(railDesign(""))
	if fs := regulatorOutputExceedsAbsMax.Eval(m); len(fs) != 0 {
		t.Errorf("want no findings with no seeded params, got %+v", fs)
	}
	if ok, reason := check.Available(regulatorOutputExceedsAbsMax, m); ok || reason == "" {
		t.Errorf("Available = (%v, %q), want not-applicable with a reason", ok, reason)
	}
}

// TestRegulatorOutputOneSidedSeed: a regulator with no spec, or a load with no spec, yields no
// comparison. Skip, never pass — half a join is not evidence.
func TestRegulatorOutputOneSidedSeed(t *testing.T) {
	for _, c := range []struct {
		name string
		set  param.ParamSet
	}{
		{"load unseeded", param.ParamSet{"ACME-REG": regSpec("ACME-REG", 5.0, "hand", 1)}},
		{"source unseeded", param.ParamSet{"ACME-33": ldoSpec("ACME-33", 3.6)}},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := check.NewModelWithParams(railDesign(""), nil, c.set)
			if fs := regulatorOutputExceedsAbsMax.Eval(m); len(fs) != 0 {
				t.Errorf("want no findings, got %+v", fs)
			}
		})
	}
}

// TestRegulatorOutputCarriesBothEntitiesAsContext is the two-entity case of agni issue 349, and the
// one that shows why context is a LIST with roles rather than a single extra subject.
//
// The message names three entities: the endangered part (the subject), the regulator supplying it,
// and the net between them. A reader who wants to look at the regulator, or at the rail, had to read
// the ref des out of the sentence and find it by hand.
//
// The ORDER assertion matters as much as the presence: entries come in the order the message names
// them, so a panel's chips read like the sentence above them.
func TestRegulatorOutputCarriesBothEntitiesAsContext(t *testing.T) {
	m := regModel(t, railDesign(""), 5.0, 3.6)
	fs := regulatorOutputExceedsAbsMax.Eval(m)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding, got %d", len(fs))
	}
	ctx := fs[0].Context
	if len(ctx) != 2 {
		t.Fatalf("want 2 context entities (the source and the rail), got %d: %+v", len(ctx), ctx)
	}
	if ctx[0].Subject != "U1" || ctx[0].Role != "source" || ctx[0].Kind != check.KindComponent {
		t.Errorf("first context = %+v, want U1 as the component playing source", ctx[0])
	}
	if ctx[1].Subject != "VRAIL" || ctx[1].Role != "rail" || ctx[1].Kind != check.KindNet {
		t.Errorf("second context = %+v, want VRAIL as the net playing rail", ctx[1])
	}
	// The subject is the endangered part and must not also appear as its own context, or the panel
	// offers a chip that navigates to where the reader already is.
	for _, c := range ctx {
		if c.Subject == fs[0].Subject {
			t.Errorf("context repeats the subject %q", fs[0].Subject)
		}
		if !strings.Contains(fs[0].Message, c.Subject) {
			t.Errorf("context %q is not named in the message %q", c.Subject, fs[0].Message)
		}
	}
}
