package check

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// The built-in SpecFuncs are the Go escape hatches the rule specs call. Each wraps an existing
// rule helper rather than reimplementing it, so the Go rules and the specs share one
// implementation of every heuristic, and each declares the reads/primitives it consumes so
// derivation stays accurate through the FFI boundary (see SpecFunc).
func init() { registerBuiltinSpecFuncs() }

// registerBuiltinSpecFuncs is idempotent (map assignment) and ALSO called from rule var
// initializers that bind Specs referencing these funcs, because package vars initialize before
// init functions and Spec.Rule validates Call targets at bind time.
func registerBuiltinSpecFuncs() {
	RegisterSpecFunc("intentionally_unconnected", &SpecFunc{
		// The no-connect heuristic, over tool marker names and NO_CONNECT-typed pins.
		Reads:      []string{"net.names", "pin.no_connect"},
		Primitives: []string{"traverse", "exists", "pin-role"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return IntentionallyUnconnected(m, ents["net"].(*ir.Net))
		},
	})
	RegisterSpecFunc("unprotected_power_reach", &SpecFunc{
		// The WS3-011 reach walk behind input-protection. True when, from the in-scope
		// connector net, SOME series-reachable net carries a real power-input pin with
		// neither a fuse crossed on its path nor a TVS on a path net.
		Reads:      []string{"component.class", "on_net", "pin.electrical_type"},
		Primitives: []string{"exists", "pin-role", "reach", "traverse"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return UnprotectedPowerReach(m, ents["net"].(*ir.Net))
		},
	})
	RegisterSpecFunc("power_pin_reach", &SpecFunc{
		// The esd/input-protection turf split under reach (see PowerPinReachable).
		Reads:      []string{"component.class", "on_net", "pin.electrical_type"},
		Primitives: []string{"exists", "pin-role", "reach", "traverse"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return PowerPinReachable(m, ents["net"].(*ir.Net))
		},
	})
	RegisterSpecFunc("tvs_reach", &SpecFunc{
		// The WS3-011 clamp-existence walk behind esd-protection (see TVSReachable).
		Reads:      []string{"component.class", "on_net"},
		Primitives: []string{"exists", "reach", "traverse"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return TVSReachable(m, ents["net"].(*ir.Net))
		},
	})
	RegisterSpecFunc("zener_reach", &SpecFunc{
		// The WS3-078 Zener clamp walk (see ZenerReachable). Same shape as tvs_reach, and
		// kept distinct because a Zener is not a fast ESD TVS.
		Reads:      []string{"component.class", "on_net"},
		Primitives: []string{"exists", "reach", "traverse"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return ZenerReachable(m, ents["net"].(*ir.Net))
		},
	})
	RegisterSpecFunc("ic_esd_rated", &SpecFunc{
		// The WS3-073 credit for IC-integrated ESD on the net or its 2-hop reach (see ICESDRated).
		Reads:      []string{"param.esd_rating", "on_net"},
		Primitives: []string{"exists", "reach", "traverse", "param-join"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return ICESDRated(m, ents["net"].(*ir.Net))
		},
	})
	RegisterSpecFunc("ground_name", &SpecFunc{
		Reads:      []string{"net.names"},
		Primitives: []string{"pattern"},
		Fn: func(m Model, _ map[string]any, args []any) any {
			return m.IsGroundName(args[0].(string))
		},
	})
	RegisterSpecFunc("pullup_reaches_rail", &SpecFunc{
		// The i2c-pull-up walk (see PullUpReachesRail).
		//
		// It is an FFI rather than a composition of collections because the spec language can
		// reach a net's connections but not a connection's component's OTHER nets, so the
		// second hop has nowhere to come from. See agni issue 374 for the surface that would
		// make this expressible.
		Reads:      []string{"component.class", "net.names", "on_net"},
		Primitives: []string{"exists", "reach", "traverse"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return PullUpReachesRail(m, ents["net"].(*ir.Net))
		},
	})
	RegisterSpecFunc("rail_name", &SpecFunc{
		Reads:      []string{"net.names"},
		Primitives: []string{"pattern"},
		Fn: func(m Model, _ map[string]any, args []any) any {
			return m.IsPowerRailName(args[0].(string))
		},
	})
	RegisterSpecFunc("feedback_name", &SpecFunc{
		Reads:      []string{"net.names"},
		Primitives: []string{"pattern"},
		Fn: func(m Model, _ map[string]any, args []any) any {
			return m.IsFeedbackName(args[0].(string))
		},
	})
	RegisterSpecFunc("switching_name", &SpecFunc{
		Reads:      []string{"net.names"},
		Primitives: []string{"pattern"},
		Fn: func(m Model, _ map[string]any, args []any) any {
			return m.IsSwitchingName(args[0].(string))
		},
	})
	RegisterSpecFunc("control_name", &SpecFunc{
		Reads:      []string{"net.names"},
		Primitives: []string{"pattern"},
		Fn: func(m Model, _ map[string]any, args []any) any {
			return m.IsControlName(args[0].(string))
		},
	})
	RegisterSpecFunc("gate_drive_name", &SpecFunc{
		Reads:      []string{"net.names"},
		Primitives: []string{"pattern"},
		Fn: func(m Model, _ map[string]any, args []any) any {
			return m.IsGateDriveName(args[0].(string))
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
		// The pairing primitive, true when a net with this name exists (case-insensitive).
		Reads:      []string{"net.names"},
		Primitives: []string{"pair"},
		Fn: func(m Model, _ map[string]any, args []any) any {
			return m.HasNetName(args[0].(string))
		},
	})
	RegisterSpecFunc("diff_convention_present", &SpecFunc{
		// Design-level pair-population evidence that gates diff-pair orphan reporting (see
		// DiffConventionPresent).
		Reads:      []string{"net.names"},
		Primitives: []string{"pair"},
		Fn: func(m Model, _ map[string]any, _ []any) any {
			return DiffConventionPresent(m)
		},
	})
}
