package builtin

import "github.com/panyam/agni/core/check"

// testPointCoverage is the DFT (design-for-test) presence rule: rails and ground must be
// probe-able. Spec-only plus one declared FFI, per the twin discipline in
// docsite/content/build/check-rule.md. That page also builds this rule step by step as its worked
// example, so changes here should keep its narrative honest.
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
		// SCOPE, not the violation: these four decide whether the rule is about this net at all.
		// has_test_points is design-wide, so without the split a board using no test points would
		// report every net as passing, which is a rule that declined to run claiming universal
		// success. The rail clauses matter as much: Over is every net, and this rule is about rails,
		// so on the tutorial gateway design that is 15 nets narrowed to 4.
		Scope: check.And{Xs: []check.Expr{
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
		}},
		// THE VIOLATION, and the only clause that is one: an in-scope rail with no test point on it.
		// A pass here means the rail carries one, which is a claim worth making.
		Where:   check.Not{X: check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "component.class"}, Op: "==", R: check.Lit{V: "test_point"}}}},
		Message: "rail carries no test point; bring-up and factory test cannot probe it",
	}
	return spec.Rule(check.Rule{
		Name:     "test-point-coverage",
		Severity: "info",
		Summary:  "A power rail or ground net has no test point, on a board that uses them.",
		Impact:   "The nets you most need to see at bring-up are the ones you cannot reach with a probe on an assembled board. A rail without a test point means debugging a power problem by touching a component pad the size of a sand grain, and factory test cannot verify the rail at all.",
		Remedy:   "Add a test point to the rail. It costs a pad, and it is the difference between measuring a supply at bring-up and trying to hold a probe on a component lead.",
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryPower,
			check.KeyTier:         "R",
			check.KeyDistribution: check.DistOpen,
		},
		Detail: ruleDoc("test-point-coverage"),
	})
})
