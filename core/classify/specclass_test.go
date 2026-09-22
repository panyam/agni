package classify

import (
	"testing"

	"github.com/panyam/agni/core/model"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// tagged reads a component's class tags back as "class:source" pairs, which is what these tests
// assert on: the membership alone was never the half that went missing.
func tagged(c *ir.Component) []string {
	out := []string{}
	for _, t := range c.GetDeviceClasses() {
		src := "?"
		switch t.GetSource() {
		case ir.ClassSource_CLASS_SOURCE_CONVENTION:
			src = "convention"
		case ir.ClassSource_CLASS_SOURCE_DATASHEET:
			src = "datasheet"
		}
		out = append(out, t.GetClass()+":"+src)
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// specDesign is a design already stamped by the convention tier, which is what the Loader hands the
// datasheet pass. Y1 reaches the clock FAMILY from its prefix and no further, which is the state the
// keyword path is documented as unable to improve on.
func specDesign() *ir.Design {
	return &ir.Design{Components: []*ir.Component{
		{RefDes: "Y1", Mpn: "ACME-XTAL", DeviceClasses: TagsOf(ClassClock, ir.ClassSource_CLASS_SOURCE_CONVENTION)},
		{RefDes: "U1", Mpn: "ACME-ORING", DeviceClasses: TagsOf(ClassIC, ir.ClassSource_CLASS_SOURCE_CONVENTION)},
		{RefDes: "R1", Mpn: "", DeviceClasses: TagsOf(ClassResistor, ir.ClassSource_CLASS_SOURCE_CONVENTION)},
	}}
}

// TestStampClassesFromSpecsAddsWhatOnlyADatasheetKnows is the pass doing its job: a seeded
// device_class resolves a subtype the keyword path refuses to guess, and it lands in the IR carrying
// the evidence, beside rather than instead of what the convention tier established.
func TestStampClassesFromSpecsAddsWhatOnlyADatasheetKnows(t *testing.T) {
	d := specDesign()
	StampClassesFromSpecs(d, func(mpn string) string {
		return map[string]string{"ACME-XTAL": "crystal", "ACME-ORING": "ORing Controller"}[mpn]
	})
	// Y1's family tag reads as datasheet-sourced, not convention-sourced, and that is correct rather
	// than a side effect: a spec saying "crystal" says this is a clock source, so the stronger source
	// is recorded for both tags. The convention tier still established the membership first, and
	// nothing about the set changed; only the evidence recorded against it moved up.
	want := map[string][]string{
		"Y1": {"clock:datasheet", "crystal:datasheet"},
		"U1": {"ic:convention", "ideal_diode_controller:datasheet"},
		"R1": {"resistor:convention"},
	}
	for _, c := range d.Components {
		if got := tagged(c); !eq(got, want[c.RefDes]) {
			t.Errorf("%s tags = %v, want %v", c.RefDes, got, want[c.RefDes])
		}
	}
	// The SET keeps the order the tiers wrote it in, so the datasheet's answer arrives last. Which of
	// them is the headline class is BySpecificity's question and not the set's, which is why nothing
	// reads position 0 of this slice directly.
	if got := MostSpecific(ClassNames(d.Components[0])); got != ClassCrystal {
		t.Errorf("Y1 most-specific = %q, want %q", got, ClassCrystal)
	}
}

// TestStampClassesFromSpecsIsAdditiveAndIdempotent pins both halves of the contract two callers
// depend on: the Loader runs this pass, and check.Model runs it again over the same design when it
// is given a corpus of its own.
func TestStampClassesFromSpecsIsAdditiveAndIdempotent(t *testing.T) {
	lookup := func(string) string { return "crystal" }
	once := specDesign()
	StampClassesFromSpecs(once, lookup)
	twice := specDesign()
	StampClassesFromSpecs(twice, lookup)
	StampClassesFromSpecs(twice, lookup)
	for i, c := range twice.Components {
		if got, first := tagged(c), tagged(once.Components[i]); !eq(got, first) {
			t.Errorf("%s: running the pass twice gave %v, once gave %v", c.RefDes, got, first)
		}
	}
}

// TestStampClassesFromSpecsWithoutACorpusChangesNothing is the degrade-safe half of C9 (c). A read
// with no params tier is the ordinary case, and it has to produce exactly what it produced before
// this pass existed.
func TestStampClassesFromSpecsWithoutACorpusChangesNothing(t *testing.T) {
	for name, lookup := range map[string]func(string) string{
		"nil lookup":    nil,
		"empty answers": func(string) string { return "" },
	} {
		d, base := specDesign(), specDesign()
		StampClassesFromSpecs(d, lookup)
		for i, c := range d.Components {
			if got, want := tagged(c), tagged(base.Components[i]); !eq(got, want) {
				t.Errorf("%s: %s tags = %v, want the untouched %v", name, c.RefDes, got, want)
			}
		}
	}
}

// TestAddClassTagRecordsTheStrongerSource covers the additive-only rule where two tiers establish the
// SAME class. Membership never changes; the recorded evidence moves to the stronger one and never
// back, which is what keeps "how do we know this" answerable after any number of tiers have run.
func TestAddClassTagRecordsTheStrongerSource(t *testing.T) {
	up := &ir.Component{RefDes: "Y1"}
	AddClassTag(up, "crystal", ir.ClassSource_CLASS_SOURCE_CONVENTION)
	AddClassTag(up, "crystal", ir.ClassSource_CLASS_SOURCE_DATASHEET)
	if got := tagged(up); !eq(got, []string{"crystal:datasheet"}) {
		t.Errorf("upgrade: %v, want one tag recorded as datasheet", got)
	}
	down := &ir.Component{RefDes: "Y2"}
	AddClassTag(down, "crystal", ir.ClassSource_CLASS_SOURCE_DATASHEET)
	AddClassTag(down, "crystal", ir.ClassSource_CLASS_SOURCE_CONVENTION)
	if got := tagged(down); !eq(got, []string{"crystal:datasheet"}) {
		t.Errorf("downgrade: %v, want the datasheet source to stand", got)
	}
	// Neither the empty class nor the unknown marker is a fact, so neither becomes a tag.
	none := &ir.Component{RefDes: "W9"}
	AddClassTag(none, "", ir.ClassSource_CLASS_SOURCE_DATASHEET)
	AddClassTag(none, string(ClassUnknown), ir.ClassSource_CLASS_SOURCE_DATASHEET)
	if got := tagged(none); len(got) != 0 {
		t.Errorf("unknown/empty became tags: %v", got)
	}
}

// TestUnrecognisedVendorClassSortsBehindTheKeywordClass is the regression this change is most likely
// to cause. A corpus may state a device_class outside the engine's vocabulary ("regulator" is in the
// tutorial project's own corpus today), and it must not displace the class everything else reads.
func TestUnrecognisedVendorClassSortsBehindTheKeywordClass(t *testing.T) {
	d := specDesign()
	StampClassesFromSpecs(d, func(string) string { return "regulator" })
	u1 := d.Components[1]
	if got := tagged(u1); !eq(got, []string{"ic:convention", "regulator:datasheet"}) {
		t.Errorf("U1 tags = %v, want the vendor class kept and added, not dropped", got)
	}
	if got := MostSpecific(ClassNames(u1)); got != ClassIC {
		t.Errorf("MostSpecific(U1) = %q, want %q: an unranked vendor class must not take the headline", got, ClassIC)
	}
}

// TestBySpecificityAgreesWithMostSpecific is the ordering invariant agni issue 710 turns on. The
// drawing takes the head of this list and check.Model takes MostSpecific, so the two are one answer
// only while these agree, for every class the engine ships.
func TestBySpecificityAgreesWithMostSpecific(t *testing.T) {
	for _, cl := range model.ComponentClasses() {
		set := ClassesOf(cl)
		ranked := BySpecificity(set)
		if len(ranked) == 0 {
			t.Fatalf("%s: BySpecificity dropped the whole set %v", cl, set)
		}
		if ranked[0] != string(MostSpecific(set)) {
			t.Errorf("%s: BySpecificity head %q, MostSpecific %q", cl, ranked[0], MostSpecific(set))
		}
		if ranked[0] != string(cl) {
			t.Errorf("%s: ranked head is %q, want the specific class itself", cl, ranked[0])
		}
	}
}
