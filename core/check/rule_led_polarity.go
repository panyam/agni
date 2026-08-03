package check

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// ledPolarity flags an LED wired so it can never conduct: anode on a ground-named net,
// cathode on a power-rail-named net. The first consumer of the derived pin.role fact
// (WS1-009) and deliberately LED-only: for zeners and TVS diodes the reverse-biased
// topology is correct usage — flagging it is the false positive that makes engineers
// mute orientation rules. The general diode-orientation rule waits on net-polarity facts
// (which way current is meant to flow), not just rail names.
// The FFI is registered inside the rule var's own initializer (not an init func): package
// vars initialize before init() runs, and Spec.Rule validates Call targets at bind time.
var ledPolarity = func() *Rule {
	registerLedReversed()
	return ledPolaritySpec.Rule(ledPolarityMeta)
}()

var ledPolaritySpec = &Spec{
	Over: "components",
	Where: And{Xs: []Expr{
		Cmp{L: Fact{"component.class"}, Op: "==", R: Lit{"led"}},
		IsTrue{T: Call{Fn: "led_reversed"}},
	}},
	Message: "LED anode is on ground and cathode on a power rail; it can never conduct",
}

var ledPolarityMeta = Rule{
	Name:     "led-polarity",
	Severity: "error",
	Summary:  "An LED's anode sits on ground and its cathode on a power rail — mounted backwards.",
	Impact:   "A reversed LED simply never lights. It passes every connectivity check (both pins are wired), survives assembly, and shows up as a dead indicator at bring-up — a part-level rework for a capture-time slip.",
	Tags: map[string]string{
		KeyCategory:     CategoryConnectivity,
		KeyTier:         "R",
		KeyDistribution: DistOpen,
	},
	Detail: ruleDoc("led-polarity"),
}

func registerLedReversed() {
	RegisterSpecFunc("led_reversed", &SpecFunc{
		// Resolves the component's anode and cathode by derived role, then tests the
		// nets they land on against the rail-name conventions. Multi-clause and
		// entity-shaped, so it lives behind the FFI seam; the pin.role fact it rides is
		// unit-tested directly.
		Reads:      []string{"net.names", "on_net", "pin.role"},
		Primitives: []string{"pin-role", "traverse", "pattern"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			ref := ents["component"].(*ir.Component).RefDes
			anodeNet, cathodeNet := "", ""
			for _, p := range m.Pins() {
				if p.Component.RefDes != ref {
					continue
				}
				switch m.PinRole(ref, p.Designator) {
				case RoleAnode:
					anodeNet = m.PinNetName(ref, p.Designator)
				case RoleCathode:
					cathodeNet = m.PinNetName(ref, p.Designator)
				}
			}
			return anodeNet != "" && cathodeNet != "" &&
				IsGroundName(anodeNet) && IsPowerRailName(cathodeNet)
		},
	})
}
