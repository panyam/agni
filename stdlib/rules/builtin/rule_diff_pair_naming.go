package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// diffPairNaming flags a differential-pair positive net that has no complementary negative net.
// See Detail.
var diffPairNaming = &check.Rule{
	Name:       "diff-pair-naming",
	Severity:   "warning",
	Summary:    "A differential-pair positive net (_P / _DP / trailing +) has no complementary negative net.",
	Impact:     "A broken pair means the layout tool never treats the two nets as coupled, so they route like ordinary signals and the high-speed link fails signal integrity. Caught at capture it is a one-line fix; caught after layout it means re-routing.",
	Primitives: []string{"select", "pattern", "pair"},
	Reads:      []string{"net.names"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryNaming,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistPublicReference,
	},
	Detail: ruleDoc("diff-pair-naming"),
	Eval: func(m check.Model) []check.Finding {
		// Gate: only claim a broken pair on a design that USES the convention (some complete
		// X_P/X_N pair exists). Without this, any coincidental _P suffix is a finding, so a
		// combinational netlist where nothing is differential (e.g. LGSynth benchmarks whose
		// nets end in _P) sprays a warning per net — the profile of a rule operators disable.
		// Folded into the predicate (not an early nil return) to stay structurally identical
		// to the Spec twin's Where, which the parity test compares element-for-element.
		uses := check.DiffConventionPresent(m)
		orphans := check.Select(m.Nets(), func(n *ir.Net) bool {
			neg, ok := check.ExpectedDiffNegative(n.Name)
			return uses && ok && !m.HasNetName(neg)
		})
		return check.Report(orphans, func(n *ir.Net) check.Finding {
			neg, _ := check.ExpectedDiffNegative(n.Name)
			return check.Finding{
				Kind:    check.KindNet,
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
var diffPairNamingSpec = &check.Spec{
	Over: "nets",
	Let:  map[string]check.Term{"neg": check.Call{Fn: "diff_negative", Args: []check.Term{check.Fact{Name: "net.names"}}}},
	Where: check.And{Xs: []check.Expr{
		check.IsTrue{T: check.Call{Fn: "diff_convention_present"}},
		check.Cmp{L: check.Var{Name: "neg"}, Op: "!=", R: check.Lit{V: ""}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "has_net", Args: []check.Term{check.Var{Name: "neg"}}}}},
	}},
	Message: "differential net has no complementary {neg:q}",
}
