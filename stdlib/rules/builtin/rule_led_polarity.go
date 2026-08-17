package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// ledPolarity flags an LED wired so it can never conduct: anode on a ground-named net,
// cathode on a power-rail-named net. It rides the derived pin.role fact (WS1-009), and is
// deliberately LED-only: for zeners and TVS diodes the reverse-biased topology is correct
// usage, and flagging it is the false positive that makes engineers mute orientation
// rules. Generalizing to all diodes needs net-polarity facts (which way current is meant
// to flow), not rail names.
//
// The FFI is registered inside the rule var's own initializer rather than an init func:
// package vars initialize before init() runs, and Spec.Rule validates Call targets at bind
// time. See docsite/content/build/check-rule.md.
var ledPolarity = func() *check.Rule {
	registerLedReversed()
	return ledPolaritySpec.Rule(ledPolarityMeta)
}()

var ledPolaritySpec = &check.Spec{
	Over: "components",
	Where: check.And{Xs: []check.Expr{
		check.Cmp{L: check.Fact{Name: "component.class"}, Op: "==", R: check.Lit{V: "led"}},
		check.IsTrue{T: check.Call{Fn: "led_reversed"}},
	}},
	Message: "LED anode is on ground and cathode on a power rail; it can never conduct",
}

var ledPolarityMeta = check.Rule{
	Name:     "led-polarity",
	Severity: "error",
	Summary:  "An LED's anode sits on ground and its cathode on a power rail, mounted backwards.",
	Impact:   "A reversed LED simply never lights. It passes every connectivity check (both pins are wired), survives assembly, and shows up as a dead indicator at bring-up — a part-level rework for a capture-time slip.",
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("led-polarity"),
}

func registerLedReversed() {
	check.RegisterSpecFunc("led_reversed", &check.SpecFunc{
		// Resolves the anode and cathode by derived role, then tests the nets they land
		// on against the rail-name conventions. Multi-clause and entity-shaped, so it
		// lives behind the FFI seam.
		Reads:      []string{"net.names", "on_net", "pin.role"},
		Primitives: []string{"pin-role", "traverse", "pattern"},
		Fn: func(m check.Model, ents map[string]any, _ []any) any {
			ref := ents["component"].(*ir.Component).RefDes
			anodeNet, cathodeNet := "", ""
			for _, p := range m.Pins() {
				if p.Component.RefDes != ref {
					continue
				}
				switch m.PinRole(ref, p.Designator) {
				case check.RoleAnode:
					anodeNet = m.PinNetName(ref, p.Designator)
				case check.RoleCathode:
					cathodeNet = m.PinNetName(ref, p.Designator)
				}
			}
			return anodeNet != "" && cathodeNet != "" &&
				m.IsGroundName(anodeNet) && m.IsPowerRailName(cathodeNet)
		},
	})
}
