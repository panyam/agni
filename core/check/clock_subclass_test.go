package check

import (
	"testing"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestClockFamilyTagRetention: each clock subtype answers HasClass(clock) (family membership), and an
// oscillator does NOT answer HasClass(crystal) — the family is clock, not crystal (WS10-015).
func TestClockFamilyTagRetention(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "X1", DeviceClasses: classify.Tags(string(ClassOscillator), string(ClassClock))},
		{RefDes: "Y1", DeviceClasses: classify.Tags(string(ClassCrystal), string(ClassClock))},
		{RefDes: "Y2", DeviceClasses: classify.Tags(string(ClassCeramicResonator), string(ClassClock))},
	}}
	m := NewModel(d)
	for _, ref := range []string{"X1", "Y1", "Y2"} {
		if !m.HasClass(ref, ClassClock) {
			t.Errorf("HasClass(%s, clock) should be true (family membership)", ref)
		}
	}
	if m.HasClass("X1", ClassCrystal) {
		t.Error("an oscillator must NOT satisfy HasClass(crystal) — the family is clock, not crystal")
	}
	if got := m.ComponentClass("X1"); got != ClassOscillator {
		t.Errorf("ComponentClass(X1) = %s, want oscillator (most-specific)", got)
	}
}
