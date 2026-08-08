package formats

import "testing"

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
