package builtin

import "github.com/panyam/agni/core/check"

// testPointCoverage is the DFT (design-for-test) presence rule: rails and ground must be
// probe-able. Spec-only (proven vocabulary plus one declared FFI, the matrix-row
// precedent); the channel gate is the design.nc_channel pattern applied to test points.
// This rule is also docs/23's worked example — the rule-authoring guide builds it step by
// step, so changes here should keep the guide's narrative honest.
var testPointCoverage = matrixlessSpecRule(func() *check.Rule {
	// check's own init (spec_funcs.go) already registers the shared FFIs (rail_name/ground_name)
	// before this package's var initializers run, since check is imported here.
	check.RegisterSpecFunc("has_test_points", &check.SpecFunc{
		// The DFT channel: does this design place test points at all. A board with zero
		// TPs has no probe convention to violate (firing on every rail of a TP-less demo
		// board is pure noise); a board with SOME declares the convention, making an
		// uncovered rail an omission.
		Reads:      []string{"component.class"},
		Primitives: []string{"exists", "select"},
		Fn: func(m check.Model, _ map[string]any, _ []any) any {
			for _, c := range m.Components() {
				if m.HasClass(c.RefDes, check.ClassTestPoint) {
					return true
				}
			}
			return false
		},
	})
	spec := &check.Spec{
		Over: "nets",
		Where: check.And{Xs: []check.Expr{
			check.IsTrue{T: check.Call{Fn: "has_test_points"}},
			check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
			check.Or{Xs: []check.Expr{
				check.IsTrue{T: check.Fact{Name: "net.attr.global"}},
				check.IsTrue{T: check.Fact{Name: "net.attr.power_driven"}},
				check.IsTrue{T: check.Call{Fn: "rail_name", Args: []check.Term{check.Fact{Name: "net.names"}}}},
				check.IsTrue{T: check.Call{Fn: "ground_name", Args: []check.Term{check.Fact{Name: "net.names"}}}},
			}},
			// A regulator feedback / sense node reads as a rail by name (VCC..._FB) but is a
			// high-impedance sense point that must NOT be probed; exclude it (WS3-067). The
			// feedback patterns are naming-lexicon config a project extends (WS3-069).
			check.Not{X: check.IsTrue{T: check.Call{Fn: "feedback_name", Args: []check.Term{check.Fact{Name: "net.names"}}}}},
			check.Not{X: check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "component.class"}, Op: "==", R: check.Lit{V: "test_point"}}}},
		}},
		Message: "rail carries no test point; bring-up and factory test cannot probe it",
	}
	return spec.Rule(check.Rule{
		Name:     "test-point-coverage",
		Severity: "info",
		Summary:  "A power rail or ground net has no test point, on a board that uses them.",
		Impact:   "The nets you most need to see at bring-up are the ones you cannot reach with a probe on an assembled board. A rail without a test point means debugging a power problem by touching a component pad the size of a sand grain, and factory test cannot verify the rail at all.",
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryPower,
			check.KeyTier:         "R",
			check.KeyDistribution: check.DistOpen,
		},
		Detail: ruleDoc("test-point-coverage"),
	})
})
