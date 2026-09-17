package check

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// regulatorPageWithControlsDesign is one converter's full complement of rail-named nets: the output
// it actually produces, and the six pin functions named after it that are not it.
func regulatorPageWithControlsDesign() *ir.Design {
	return &ir.Design{Nets: []*ir.Net{
		{Name: "12V_OUT"}, {Name: "12V_FB"}, {Name: "12V_SW"}, {Name: "12V_BOOT"},
		{Name: "12V_MODE1"}, {Name: "12V_MODE2"}, {Name: "20V_EN"}, {Name: "12V_VDRV"},
		{Name: "+3V3"}, {Name: "SDA"},
		// The objection this vocabulary invites: _EN is a common suffix on nets that have nothing to
		// do with a regulator. These carry no rail token, so they never reach the rail question at
		// all, and the role stamped on them is inert.
		{Name: "USB_EN"}, {Name: "FAN_EN"},
	}}
}

// TestRailNetExcludesEveryRegulatorPinFunction: mode straps, enables and the gate-drive supply are
// named after the converter they belong to, so they read as rails and are not (agni 680).
func TestRailNetExcludesEveryRegulatorPinFunction(t *testing.T) {
	m := NewModel(regulatorPageWithControlsDesign())
	byName := map[string]*ir.Net{}
	for _, n := range m.Nets() {
		byName[n.Name] = n
	}

	for _, name := range []string{"12V_MODE1", "12V_MODE2", "20V_EN", "12V_VDRV"} {
		if m.IsRailNet(byName[name]) {
			t.Errorf("IsRailNet(%q) = true, want false: the voltage token names the converter", name)
		}
	}
	// Still excluded, from the agni 683 half.
	for _, name := range []string{"12V_FB", "12V_SW", "12V_BOOT"} {
		if m.IsRailNet(byName[name]) {
			t.Errorf("IsRailNet(%q) = true, want false", name)
		}
	}
	// The positive control. A predicate answering false for everything passes both halves above.
	for _, name := range []string{"12V_OUT", "+3V3"} {
		if !m.IsRailNet(byName[name]) {
			t.Errorf("IsRailNet(%q) = false, want true: the exclusion has eaten a real rail", name)
		}
	}
}

// TestControlSuffixNeedsARailTokenToMatter: the answer to "does _EN swallow half the board". A net
// with no rail token in its name is not a rail either way, so stamping it control changes nothing.
func TestControlSuffixNeedsARailTokenToMatter(t *testing.T) {
	m := NewModel(regulatorPageWithControlsDesign())
	byName := map[string]*ir.Net{}
	for _, n := range m.Nets() {
		byName[n.Name] = n
	}
	for _, name := range []string{"USB_EN", "FAN_EN"} {
		if !m.IsControlName(name) {
			t.Errorf("IsControlName(%q) = false: the vocabulary should match, harmlessly", name)
		}
		if m.IsRailNet(byName[name]) {
			t.Errorf("IsRailNet(%q) = true, want false", name)
		}
		if m.IsPowerRailName(name) {
			t.Errorf("IsPowerRailName(%q) = true: it carries no rail token, so it never reached the rail question", name)
		}
	}
}

// TestRegulatorInternalHasOneImplementation: IsRailNet and the two voltage relations both subtract
// this predicate, and it spanned four roles across two packages. Asserting them against each other
// here is what stops a fifth role being added to one and not the other.
func TestRegulatorInternalHasOneImplementation(t *testing.T) {
	m := NewModel(regulatorPageWithControlsDesign())
	for _, n := range m.Nets() {
		internal := m.IsRegulatorInternalNet(n)
		railNamed := m.IsPowerRailName(n.Name)
		if internal && railNamed && m.IsRailNet(n) {
			t.Errorf("%q is a regulator internal and still answers IsRailNet", n.Name)
		}
	}
	if !m.IsRegulatorInternalNet(&ir.Net{Name: "12V_VDRV"}) {
		t.Error("gate drive must be one of the four roles the predicate spans")
	}
}
