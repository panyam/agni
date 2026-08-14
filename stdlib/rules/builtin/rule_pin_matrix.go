package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// The ERC connection matrix (WS3-014): which pin-type pairings on one net are illegal.
// Every capture tool ships this as a grid (KiCad's pin-conflict matrix, Altium's
// connection matrix). Here each illegal pairing is a matrixRow, metadata plus the Spec
// that detects it, and the rules are GENERATED from the table via Spec.Rule. These rows
// are spec-only: no Go Eval exists, the interpreter is the runtime
// (docsite/content/architecture/rules-and-checks.md, "A rule is a value"; the twin
// discipline is in docsite/content/build/check-rule.md). One file holds every row, a
// deliberate exception to one-file-per-rule, because the table itself is the artifact:
// adding a pairing is adding a row.
//
// Rows deliberately absent, and why:
//   - input-only-with-no-driver: shipped separately as floating-input, whose guards
//     (external skip, >= 2 pins, all-input) are more specific than a pairing.
//   - power-pairing rows beyond the hard-driver conflict: subsumed by
//     output-output-conflict once power-symbol pins became driver connections
//     (WS1-014). power_out ↔ power_out and power_out ↔ output are the same
//     distinct-driver count.

// matrixRow is one illegal pairing: the rule identity/prose and the Spec detecting it.
type matrixRow struct {
	meta check.Rule
	spec *check.Spec
}

// rule binds the row into the registered *Rule (Eval, Reads, and Primitives all come
// from the Spec).
func (r matrixRow) rule() *check.Rule { return r.spec.Rule(r.meta) }

// rowOutputOutput is the matrix's driver conflict: two hard drivers on one net.
var rowOutputOutput = matrixRow{
	meta: check.Rule{
		Name:     "output-output-conflict",
		Severity: "error",
		Summary:  "Two or more driving pins (outputs / power sources) share a net and fight each other.",
		Impact:   "Two outputs on one net drive it to opposite levels at once. The result is a low-impedance path between rails through the two drivers: contention current, brownouts, and cooked output stages. It is the classic short-through-logic bug.",
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryConnectivity,
			check.KeyTier:         "R",
			check.KeyDistribution: check.DistOpen,
		},
		Detail: ruleDoc("output-output-conflict"),
	},
	spec: &check.Spec{
		Over: "nets",
		Let: map[string]check.Term{
			"drivers": check.Call{Fn: "driving_components"},
		},
		// The has_pull guard exempts open-drain wired-OR buses (see isWiredOrBus). Without it,
		// EDIF that types open-drain pins "output" reads a shared interrupt line as N drivers
		// fighting.
		Where: check.And{Xs: []check.Expr{
			check.Cmp{L: check.Var{Name: "drivers"}, Op: ">=", R: check.Lit{V: 2}},
			check.Not{X: check.IsTrue{T: check.Call{Fn: "has_pull"}}},
		}},
		Message: "net has {drivers} driving components; at most one may drive",
	},
}

// isWiredOrBus reports whether a multi-driver net looks like an intentional open-drain WIRED-OR bus (a
// shared interrupt/inhibit/reset line) rather than output contention. Evidence: a resistor member (the
// pull an open-drain bus needs, its idle level) AND NO power-source (POWER_OUT) driver. The resistor is
// keyed on PRESENCE, deliberately not on where it pulls to, because a real pull runs through several
// elements to an auto-named rail or to ground (both defeat a name-based "pull to a rail" test). A
// POWER_OUT driver blocks the exemption: two power sources fighting on a rail is a real conflict even
// with a bleeder/load resistor present.
func isWiredOrBus(m check.Model, n *ir.Net) bool {
	resistor := false
	for _, c := range n.Connections {
		if check.ConnDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_OUT {
			return false
		}
		if m.HasClass(c.ComponentRef, check.ClassResistor) {
			resistor = true
		}
	}
	return resistor
}

func registerHasPull() {
	check.RegisterSpecFunc("has_pull", &check.SpecFunc{
		Reads:      []string{"component.class", "on_net", "pin.electrical_type"},
		Primitives: []string{"exists", "pin-role"},
		Fn: func(m check.Model, ents map[string]any, _ []any) any {
			return isWiredOrBus(m, ents["net"].(*ir.Net))
		},
	})
}

// rowNCConnected is the matrix's no-connect column: a pin the symbol marks "do not
// connect" wired into a real net.
var rowNCConnected = matrixRow{
	meta: check.Rule{
		Name:     "nc-pin-connected",
		Severity: "error",
		Summary:  "A pin marked no-connect is wired into a net with other members.",
		Impact:   "The symbol says the pin must not be used and the schematic uses it anyway. Either the wire is a capture slip landing on the wrong pad, or the part is being asked to do something its maker forbids (an internal test pin, a reserved pad). Both read as working designs until the part misbehaves.",
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryConnectivity,
			check.KeyTier:         "R",
			check.KeyDistribution: check.DistOpen,
		},
		Detail: ruleDoc("nc-pin-connected"),
	},
	spec: &check.Spec{
		Over: "nets",
		Where: check.And{Xs: []check.Expr{
			check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
			check.Cmp{L: check.Fact{Name: "net.pin_count"}, Op: ">=", R: check.Lit{V: 2}},
			check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "no_connect"}}},
		}},
		Message: "net wires in a pin marked no-connect",
	},
}

// rowUnspecifiedWithDriver is the matrix's unspecified column against the driver rows: a
// pin whose symbol declares NO electrical type, on a net something actively drives.
var rowUnspecifiedWithDriver = matrixRow{
	meta: check.Rule{
		Name:     "unspecified-pin-with-driver",
		Severity: "warning",
		Summary:  "A pin with no declared electrical type sits on a driven net.",
		Impact:   "The symbol author never said what the pin is, so no matrix row can clear it: it may be an input (fine), an output (contention with the driver), or a supply pin (a rail short). The ERC is flying blind on exactly the net where a wrong guess costs a driver stage.",
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryConnectivity,
			check.KeyTier:         "R",
			check.KeyDistribution: check.DistOpen,
		},
		Detail: ruleDoc("unspecified-pin-with-driver"),
	},
	spec: &check.Spec{
		Over: "nets",
		Let: map[string]check.Term{
			"drivers": check.Call{Fn: "driving_components"},
		},
		Where: check.And{Xs: []check.Expr{
			check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
			check.Cmp{L: check.Var{Name: "drivers"}, Op: ">=", R: check.Lit{V: 1}},
			check.ExistsIn{Over: "net.connections", Where: check.And{Xs: []check.Expr{
				check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "unspecified"}},
				check.IsTrue{T: check.Fact{Name: "pin.declared"}},
				check.Not{X: check.IsTrue{T: check.Fact{Name: "conn.virtual"}}},
			}}},
		}},
		Message: "a pin with no declared electrical type shares this driven net",
	},
}

// The generated matrix rules, added to the catalog in register.go's rules slice under
// their row names. The FFIs are registered inside the var initializer; see
// rule_led_polarity.go for why.
var (
	outputOutputConflict = func() *check.Rule {
		registerDrivingComponents()
		registerHasPull()
		return rowOutputOutput.rule()
	}()
	ncPinConnected = rowNCConnected.rule()
	// Declared after outputOutputConflict on purpose: same-file var order guarantees the
	// driving_components FFI is registered before this row's bind-time Call validation.
	unspecifiedPinWithDriver = rowUnspecifiedWithDriver.rule()
)

func registerDrivingComponents() {
	check.RegisterSpecFunc("driving_components", &check.SpecFunc{
		// Distinct components with at least one hard-driver pin (output / power_out) on
		// the in-scope net: paralleled driving pins of one part are one driver.
		Reads:      []string{"on_net", "pin.electrical_type"},
		Primitives: []string{"count", "pin-role", "traverse"},
		Fn: func(m check.Model, ents map[string]any, _ []any) any {
			n := ents["net"].(*ir.Net)
			comps := map[string]bool{}
			for _, c := range n.Connections {
				switch check.ConnDir(m, c) { // attribute-aware: virtual power pins drive too (WS1-014)
				case ir.PinDirection_PIN_DIRECTION_OUTPUT, ir.PinDirection_PIN_DIRECTION_POWER_OUT:
					comps[c.ComponentRef] = true
				}
			}
			return len(comps)
		},
	})
}
