package check

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// regulatorPageDesign is one buck converter's worth of net names, the shape a real regulator page
// produces: every internal node is named after the rail it serves, so every one of them matches the
// rail vocabulary on its prefix.
func regulatorPageDesign() *ir.Design {
	return &ir.Design{Nets: []*ir.Net{
		{Name: "12V_OUT"}, {Name: "12V_FB"}, {Name: "12V_SW"},
		{Name: "12V_BOOT"}, {Name: "12V_PHASE"}, {Name: "+3V3"}, {Name: "SDA"},
	}}
}

// TestRailNetExcludesRegulatorInternals: the rail question has one answer and it is decided here.
// A feedback tap, a switch node and a bootstrap node all read as rails by NAME, and none of them
// carries the rail's voltage, so none is a rail (agni 679, 680).
func TestRailNetExcludesRegulatorInternals(t *testing.T) {
	m := NewModel(regulatorPageDesign())
	byName := map[string]*ir.Net{}
	for _, n := range m.Nets() {
		byName[n.Name] = n
	}

	for _, name := range []string{"12V_FB", "12V_SW", "12V_BOOT", "12V_PHASE"} {
		if m.IsRailNet(byName[name]) {
			t.Errorf("IsRailNet(%q) = true, want false: a regulator internal is not a rail", name)
		}
	}
	// The positive control. Without it a predicate that answered false for everything would pass the
	// half above and read as a working exclusion.
	for _, name := range []string{"12V_OUT", "+3V3"} {
		if !m.IsRailNet(byName[name]) {
			t.Errorf("IsRailNet(%q) = false, want true: the exclusion has eaten a real rail", name)
		}
	}
	if m.IsRailNet(byName["SDA"]) {
		t.Error("IsRailNet(SDA) = true, want false")
	}
}

// TestRailNameStillSeesEveryRailNamedNet: excluding regulator internals from the rail ROLE must not
// change what the name vocabulary answers. A consumer that genuinely wants every rail-named net,
// including the internals, still has IsPowerRailName, and the spec-language rail_name FFI is built
// on it.
func TestRailNameStillSeesEveryRailNamedNet(t *testing.T) {
	m := NewModel(regulatorPageDesign())
	for _, name := range []string{"12V_OUT", "12V_FB", "12V_SW", "12V_BOOT", "12V_PHASE"} {
		if !m.IsPowerRailName(name) {
			t.Errorf("IsPowerRailName(%q) = false, want true", name)
		}
	}
}

// TestSwitchingNameMatchesTheModelLexicon: the model-scoped projection answers the same as the
// package-level helper, so a rule holding a Model and a spec FFI over a bare literal agree.
func TestSwitchingNameMatchesTheModelLexicon(t *testing.T) {
	m := NewModel(regulatorPageDesign())
	for _, name := range []string{"12V_SW", "12V_BOOT", "12V_OUT", "SDA"} {
		if got, want := m.IsSwitchingName(name), IsSwitchingName(name); got != want {
			t.Errorf("IsSwitchingName(%q): model = %v, package = %v", name, got, want)
		}
	}
}
