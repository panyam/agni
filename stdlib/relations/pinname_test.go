package relations

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// pinNameDesign places one part whose type declares three pins: a named one, one KiCad marks as
// unnamed with "~", and one the type declares with no name at all.
func pinNameDesign(format string) *ir.Design {
	return &ir.Design{
		SourceFormat: format,
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			{Name: "MCU", Pins: []*ir.Pin{
				{Designator: "41", Name: "PTC11"},
				{Designator: "42", Name: "~"},
				{Designator: "43"},
			}},
		}}},
		Components: []*ir.Component{{
			RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"},
			Sections: []*ir.ComponentSection{{PartRef: "MCU", LibraryRef: "lib"}},
		}},
		Nets: []*ir.Net{tnet("ADC_IN", "U1.41")},
	}
}

func TestPinNameProjectsTheFunctionalName(t *testing.T) {
	rows := factsByRelation(Facts(check.NewModel(pinNameDesign("kicad-sch"))))[RelPinName]
	if len(rows) != 1 {
		t.Fatalf("pin.name = %+v, want one row (only pin 41 is named)", rows)
	}
	r := rows[0]
	if r.Subject != "U1" || r.Object != "41" || r.Value != "PTC11" {
		t.Errorf("pin.name = (%s, %s, %s), want (U1, 41, PTC11)", r.Subject, r.Object, r.Value)
	}
	if len(r.Cites) == 0 {
		t.Error("pin.name row carries no citation")
	}
}

// A pin the source did not name yields no row rather than a row holding "". The distinction is the
// whole reason a query can ask `pin(?r,?p), not pin.name(?r,?p,?_)` and have it mean "the read gave
// this pin no name" instead of matching every pin whose name happens to be empty.
//
// "~" is covered by the same assertion on purpose. It is KiCad's spelling of "this pin has no name"
// and it reaches the IR verbatim, so a projector taking it at face value would publish a pin named
// "~" and every downstream comparison would carry it.
func TestPinNameIsAbsentRatherThanEmpty(t *testing.T) {
	byRel := factsByRelation(Facts(check.NewModel(pinNameDesign("kicad-sch"))))
	// Two positive controls first. The loop below is a negative assertion and passes vacuously on a
	// relation that emitted nothing at all, so it proves something only once we know the fixture
	// declares three pins and the projector named exactly one of them.
	if pins := byRel[RelPin]; len(pins) != 3 {
		t.Fatalf("pin = %d rows, want 3: the fixture must declare the unnamed pins for this to prove anything", len(pins))
	}
	rows := byRel[RelPinName]
	if len(rows) != 1 {
		t.Fatalf("pin.name = %d rows, want exactly 1: with none the check below proves nothing", len(rows))
	}
	for _, r := range rows {
		if r.Value == "" || r.Value == "~" {
			t.Errorf("pin.name emitted a row for an unnamed pin: (%s, %s, %q)", r.Subject, r.Object, r.Value)
		}
	}
}

// The projector reads Name and never Designator, so a pin carrying one and not the other is still
// named. No shipped reader produces this today (the EDIF reader falls back Designator <- Name per
// issue 71), which is exactly why it is worth pinning: the fallback is what makes the two spellings
// agree on EDIF, and a projector that quietly keyed off the designator would look correct for as
// long as that fallback held and go silent the moment a reader stopped applying it.
func TestPinNameReachesAPinWithNoDesignator(t *testing.T) {
	d := &ir.Design{
		SourceFormat: "edif",
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			{Name: "MCU", Pins: []*ir.Pin{{Name: "PTC11"}}}, // a name and no designator
		}}},
		Components: []*ir.Component{{
			RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"},
			Sections: []*ir.ComponentSection{{PartRef: "MCU", LibraryRef: "lib"}},
		}},
	}
	rows := factsByRelation(Facts(check.NewModel(d)))[RelPinName]
	if len(rows) != 1 || rows[0].Value != "PTC11" {
		t.Fatalf("pin.name on a design with no pin designators = %+v, want one row naming PTC11", rows)
	}
	if rows[0].Object != "" {
		t.Errorf("designator = %q, want empty: the fixture is meant to have none, so this test would not prove its point", rows[0].Object)
	}
}

// The name is recorded exactly as the source spells it. Anything comparing a name against a name
// from another document canonicalizes both sides itself, and pinning that here is what stops a
// normalizer being added at the projector, where it would silently rewrite what the file said.
func TestPinNameIsNotNormalized(t *testing.T) {
	d := pinNameDesign("kicad-sch")
	d.Libraries[0].Parts[0].Pins[0].Name = "PTE7"
	rows := factsByRelation(Facts(check.NewModel(d)))[RelPinName]
	if len(rows) != 1 || rows[0].Value != "PTE7" {
		t.Errorf("pin.name = %+v, want the source spelling PTE7 verbatim (not zero-padded)", rows)
	}
}
