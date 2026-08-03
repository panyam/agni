package check

import (
	"fmt"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// diffPairNaming flags a differential-pair positive net that has no complementary negative net.
// See Detail.
var diffPairNaming = &Rule{
	Name:       "diff-pair-naming",
	Severity:   "warning",
	Summary:    "A differential-pair positive net (_P / _DP / trailing +) has no complementary negative net.",
	Impact:     "A broken pair means the layout tool never treats the two nets as coupled, so they route like ordinary signals and the high-speed link fails signal integrity. Caught at capture it is a one-line fix; caught after layout it means re-routing.",
	Primitives: []string{"select", "pattern", "pair"},
	Reads:      []string{"net.names"},
	Tags: map[string]string{
		KeyCategory:     CategoryNaming,
		KeyTier:         "R",
		KeyDistribution: DistPublicReference,
	},
	Detail: ruleDoc("diff-pair-naming"),
	Eval: func(m Model) []Finding {
		// Gate: only claim a broken pair on a design that USES the convention (some complete
		// X_P/X_N pair exists). Without this, any coincidental _P suffix is a finding, so a
		// combinational netlist where nothing is differential (e.g. LGSynth benchmarks whose
		// nets end in _P) sprays a warning per net — the profile of a rule operators disable.
		// Folded into the predicate (not an early nil return) to stay structurally identical
		// to the Spec twin's Where, which the parity test compares element-for-element.
		uses := DiffConventionPresent(m)
		orphans := Select(m.Nets(), func(n *ir.Net) bool {
			neg, ok := ExpectedDiffNegative(n.Name)
			return uses && ok && !m.HasNetName(neg)
		})
		return Report(orphans, func(n *ir.Net) Finding {
			neg, _ := ExpectedDiffNegative(n.Name)
			return Finding{
				Kind:    KindNet,
				Subject: n.Name,
				NetID:   n.GetId(),
				Message: fmt.Sprintf("differential net has no complementary %q", neg),
				Prov:    n.Prov,
			}
		})
	},
}

// diffPairNamingSpec is the rule's declarative twin (WS3-003): the complement name is a Let
// binding shared by the Where clause and the message, computed once per net. The leading
// diff_convention_present gate mirrors the Go Eval's pair-population guard (design-level, so
// the same value for every net); the interpreter evaluates it per net, which is harmless.
var diffPairNamingSpec = &Spec{
	Over: "nets",
	Let:  map[string]Term{"neg": Call{Fn: "diff_negative", Args: []Term{Fact{"net.names"}}}},
	Where: And{Xs: []Expr{
		IsTrue{T: Call{Fn: "diff_convention_present"}},
		Cmp{L: Var{"neg"}, Op: "!=", R: Lit{""}},
		Not{X: IsTrue{T: Call{Fn: "has_net", Args: []Term{Var{"neg"}}}}},
	}},
	Message: "differential net has no complementary {neg:q}",
}
