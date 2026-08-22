package formats

import (
	"slices"
	"testing"
)

// TestValueStampedOnRealRead is the end-to-end check that the value pass actually reaches a rule: a
// committed KiCad fixture, read through the real loader, must come back with a machine-comparable
// number rather than the vendor's text.
//
// It exists because the unit-tested pass passed while the real read did not. The fixture writes "10u"
// on a capacitor, which states the magnitude and omits the dimension, and an earlier version left the
// unit empty there. ComponentValueIn rejects an empty unit by design, so every capacitor and inductor
// on a real board would have been invisible to the rules that need them while every unit test stayed
// green.
func TestValueStampedOnRealRead(t *testing.T) {
	d, err := (&Loader{}).ReadDesign("../../cmd/agni/testdata/conformance/capcheck.fires.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	var seen int
	for _, c := range d.GetComponents() {
		q := c.GetValue()
		if q == nil {
			continue
		}
		seen++
		if q.GetInput() == "" {
			t.Errorf("%s: a stamped Quantity must keep its source text", c.GetRefDes())
		}
		if q.Value != nil && q.GetUnit() == "" {
			t.Errorf("%s: %q parsed to %v but carries no unit, so no rule can use it",
				c.GetRefDes(), q.GetInput(), q.GetValue())
		}
	}
	if seen == 0 {
		t.Fatal("no component carried a stamped Quantity after a real read")
	}
}

// TestUnresolvedSetImpliesResolvedSet is the wiring guard for the next format reader (agni issue
// 418). A reader that records what FAILED to resolve without recording what did leaves
// symbol-unresolved holding a failure list, and a rule holding only failures can only report
// failures: its silence on a clean design becomes indistinguishable from silence on a design nobody
// looked at. That is precisely the coverage claim the considered set exists to make checkable.
//
// It runs over the fixtures rather than over the reader registry because the omission is a property
// of a real read. A reader could declare the diagnostic in one code path and forget it in another,
// which is the shape both the xschem and gEDA readers have (the no-opener branch deliberately
// declares nothing), and only reading a file exercises the branch that ran.
func TestUnresolvedSetImpliesResolvedSet(t *testing.T) {
	fixtures := []string{
		"../kicad/testdata/extlib.kicad_sch",
		"../kicad/testdata/sch.kicad_sch",
		"../xschem/testdata/divider.sch",
		"../geda/testdata/divider.sch",
		"../../cmd/agni/testdata/conformance/capcheck.passes.kicad_sch",
	}
	var declared int
	for _, f := range fixtures {
		d, err := (&Loader{}).ReadDesign(f)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		diag := d.GetInputDiagnostics()
		supplies := slices.Contains(diag.GetSupplied(), "resolved_symbols")
		if supplies {
			declared++
		}
		if len(diag.GetUnresolvedSymbols()) > 0 && !supplies {
			t.Errorf("%s: reports %d unresolved symbol(s) without declaring resolved_symbols, so the rule "+
				"over them cannot state what it examined", f, len(diag.GetUnresolvedSymbols()))
		}
	}
	// A positive control (build/evidence.md): with no fixture declaring the diagnostic the loop above
	// passes by never testing anything.
	if declared == 0 {
		t.Fatal("no fixture declared resolved_symbols, so the check above proved nothing")
	}
}
