package check

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// The built-in SpecFuncs: the Go escape hatches the rule specs call. Each wraps an existing
// rule helper rather than reimplementing it, so the Go rules and the specs share one
// implementation of every heuristic, and each declares the reads/primitives it consumes so
// derivation stays honest through the FFI boundary (see SpecFunc).
func init() { registerBuiltinSpecFuncs() }

// registerBuiltinSpecFuncs is idempotent (map assignment) and ALSO called from rule var
// initializers that bind Specs referencing these funcs: package vars initialize before
// init functions, and Spec.Rule validates Call targets at bind time (the ledPolarity
// lesson, now shared instead of per-rule).
func registerBuiltinSpecFuncs() {
	RegisterSpecFunc("intentionally_unconnected", &SpecFunc{
		// The no-connect heuristic: tool marker names plus NO_CONNECT-typed pins.
		Reads:      []string{"net.names", "pin.no_connect"},
		Primitives: []string{"traverse", "exists", "pin-role"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return IntentionallyUnconnected(m, ents["net"].(*ir.Net))
		},
	})
	RegisterSpecFunc("unprotected_power_reach", &SpecFunc{
		// The WS3-011 reach walk behind input-protection: from the in-scope connector
		// net, does SOME series-reachable net carry a real power-input pin with neither
		// a fuse crossed on its path nor a TVS on a path net. Wraps the rule helper so
		// the Go Eval and the spec share one traversal.
		Reads:      []string{"component.class", "on_net", "pin.electrical_type"},
		Primitives: []string{"exists", "pin-role", "reach", "traverse"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return UnprotectedPowerReach(m, ents["net"].(*ir.Net))
		},
	})
	RegisterSpecFunc("power_pin_reach", &SpecFunc{
		// The esd/input-protection turf split under reach: a power-direction pin on the
		// in-scope net or any net in its 2-hop series reach.
		Reads:      []string{"component.class", "on_net", "pin.electrical_type"},
		Primitives: []string{"exists", "pin-role", "reach", "traverse"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return PowerPinReachable(m, ents["net"].(*ir.Net))
		},
	})
	RegisterSpecFunc("tvs_reach", &SpecFunc{
		// The WS3-011 clamp-existence walk behind esd-protection: a TVS on the in-scope
		// net or on any net in its 2-hop series reach.
		Reads:      []string{"component.class", "on_net"},
		Primitives: []string{"exists", "reach", "traverse"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return TVSReachable(m, ents["net"].(*ir.Net))
		},
	})
	RegisterSpecFunc("zener_reach", &SpecFunc{
		// WS3-078: a Zener clamp on the in-scope net or its 2-hop series reach. Same shape as
		// tvs_reach; kept distinct because a Zener is not a fast ESD TVS.
		Reads:      []string{"component.class", "on_net"},
		Primitives: []string{"exists", "reach", "traverse"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return ZenerReachable(m, ents["net"].(*ir.Net))
		},
	})
	RegisterSpecFunc("ic_esd_rated", &SpecFunc{
		// WS3-073: a component on the net (or its 2-hop reach) declares a datasheet ESD
		// rating >= the credit floor — IC-integrated ESD that protects the signal.
		Reads:      []string{"param.esd_rating", "on_net"},
		Primitives: []string{"exists", "reach", "traverse", "param-join"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return ICESDRated(m, ents["net"].(*ir.Net))
		},
	})
	RegisterSpecFunc("ground_name", &SpecFunc{
		Reads:      []string{"net.names"},
		Primitives: []string{"pattern"},
		Fn: func(_ Model, _ map[string]any, args []any) any {
			return IsGroundName(args[0].(string))
		},
	})
	RegisterSpecFunc("rail_name", &SpecFunc{
		Reads:      []string{"net.names"},
		Primitives: []string{"pattern"},
		Fn: func(_ Model, _ map[string]any, args []any) any {
			return IsPowerRailName(args[0].(string))
		},
	})
	RegisterSpecFunc("feedback_name", &SpecFunc{
		Reads:      []string{"net.names"},
		Primitives: []string{"pattern"},
		Fn: func(_ Model, _ map[string]any, args []any) any {
			return IsFeedbackName(args[0].(string))
		},
	})
	RegisterSpecFunc("diff_negative", &SpecFunc{
		// The expected complementary net name for a diff-pair positive member, "" when the
		// name is not a positive member (so a Cmp against "" is the ok-check).
		Reads:      []string{"net.names"},
		Primitives: []string{"pattern"},
		Fn: func(_ Model, _ map[string]any, args []any) any {
			neg, ok := ExpectedDiffNegative(args[0].(string))
			if !ok {
				return ""
			}
			return neg
		},
	})
	RegisterSpecFunc("has_net", &SpecFunc{
		// The pairing primitive: does a net with this name exist (case-insensitive).
		Reads:      []string{"net.names"},
		Primitives: []string{"pair"},
		Fn: func(m Model, _ map[string]any, args []any) any {
			return m.HasNetName(args[0].(string))
		},
	})
	RegisterSpecFunc("diff_convention_present", &SpecFunc{
		// Design-level pair-population evidence: does any complete X_P/X_N pair exist? Gates
		// diff-pair orphan reporting so a design with no differential pairs stays silent.
		Reads:      []string{"net.names"},
		Primitives: []string{"pair"},
		Fn: func(m Model, _ map[string]any, _ []any) any {
			return DiffConventionPresent(m)
		},
	})
}
