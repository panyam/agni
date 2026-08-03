package check

// testPointCoverage is the DFT (design-for-test) presence rule: rails and ground must be
// probe-able. Spec-only (proven vocabulary plus one declared FFI, the matrix-row
// precedent); the channel gate is the design.nc_channel pattern applied to test points.
// This rule is also docs/23's worked example — the rule-authoring guide builds it step by
// step, so changes here should keep the guide's narrative honest.
var testPointCoverage = matrixlessSpecRule(func() *Rule {
	registerBuiltinSpecFuncs() // rail_name/ground_name must exist before bind-time validation
	RegisterSpecFunc("has_test_points", &SpecFunc{
		// The DFT channel: does this design place test points at all. A board with zero
		// TPs has no probe convention to violate (firing on every rail of a TP-less demo
		// board is pure noise); a board with SOME declares the convention, making an
		// uncovered rail an omission.
		Reads:      []string{"component.class"},
		Primitives: []string{"exists", "select"},
		Fn: func(m Model, _ map[string]any, _ []any) any {
			for _, c := range m.Components() {
				if m.HasClass(c.RefDes, ClassTestPoint) {
					return true
				}
			}
			return false
		},
	})
	spec := &Spec{
		Over: "nets",
		Where: And{Xs: []Expr{
			IsTrue{T: Call{Fn: "has_test_points"}},
			Not{X: IsTrue{T: Fact{"net.attr.external"}}},
			Or{Xs: []Expr{
				IsTrue{T: Fact{"net.attr.global"}},
				IsTrue{T: Fact{"net.attr.power_driven"}},
				IsTrue{T: Call{Fn: "rail_name", Args: []Term{Fact{"net.names"}}}},
				IsTrue{T: Call{Fn: "ground_name", Args: []Term{Fact{"net.names"}}}},
			}},
			// A regulator feedback / sense node reads as a rail by name (VCC..._FB) but is a
			// high-impedance sense point that must NOT be probed; exclude it (WS3-067). The
			// feedback patterns are naming-lexicon config a project extends (WS3-069).
			Not{X: IsTrue{T: Call{Fn: "feedback_name", Args: []Term{Fact{"net.names"}}}}},
			Not{X: ExistsIn{Over: "net.connections", Where: Cmp{L: Fact{"component.class"}, Op: "==", R: Lit{"test_point"}}}},
		}},
		Message: "rail carries no test point; bring-up and factory test cannot probe it",
	}
	return spec.Rule(Rule{
		Name:     "test-point-coverage",
		Severity: "info",
		Summary:  "A power rail or ground net has no test point, on a board that uses them.",
		Impact:   "The nets you most need to see at bring-up are the ones you cannot reach with a probe on an assembled board. A rail without a test point means debugging a power problem by touching a component pad the size of a sand grain, and factory test cannot verify the rail at all.",
		Tags: map[string]string{
			KeyCategory:     CategoryPower,
			KeyTier:         "R",
			KeyDistribution: DistOpen,
		},
		Detail: ruleDoc("test-point-coverage"),
	})
})