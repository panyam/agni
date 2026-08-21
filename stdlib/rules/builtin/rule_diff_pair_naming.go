package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
)

// diffPairNaming flags a differential-pair positive net that has no complementary negative net.
// See Detail.
var diffPairNaming = &check.Rule{
	Name:       "diff-pair-naming",
	Severity:   "warning",
	Summary:    "A differential-pair positive net (_P / _DP / trailing +) has no complementary negative net.",
	Impact:     "A broken pair means the layout tool never treats the two nets as coupled, so they route like ordinary signals and the high-speed link fails signal integrity. Caught at capture it is a one-line fix; caught after layout it means re-routing.",
	Remedy:     "Add the missing complementary net, or rename the net if it was never half of a pair. Left as it is, the layout tool routes the two as ordinary signals and the link is not a differential pair at all.",
	Primitives: []string{"select", "pattern", "pair"},
	Reads:      []string{"net.names"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryNaming,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistPublicReference,
	},
	Detail:              ruleDoc("diff-pair-naming"),
	Eval:                diffPairNamingVerdicts,
	StatesConsideredSet: true,
}

// diffPairNamingVerdicts decides every net whose NAME claims to be the positive half of a pair, and
// that set is the considered set. A net carrying no such suffix is not a subject of a naming rule
// about pairs, so it yields no verdict at all: reporting it as a pass would claim the rule checked
// something it never asked about.
//
// THE POPULATION GATE BECOMES NotConsidered, which is the part worth having. `DiffConventionPresent`
// asks whether the design uses the convention anywhere, and it exists because a netlist where
// nothing is differential can still end nets in `_P` by coincidence (the LGSynth benchmarks do).
// Under the old shape a coincidental `_P` on such a design took the same silent path as a net whose
// complement was found, so the two were indistinguishable. They are not the same answer: one is
// "the pair is complete", the other is "this design states no pair anywhere, so its suffixes are not
// evidence of one". The second is the rule declining to judge, and saying so is what stops a reader
// from reading the silence as a clean bill.
//
// The witness names the complement it looked for, so it tracks the fact rather than restating the
// outcome. Rename the negative half and a pass becomes a failure naming the name it could not find.
func diffPairNamingVerdicts(m check.Model) []check.Verdict {
	uses := check.DiffConventionPresent(m)

	var out []check.Verdict
	for _, n := range m.Nets() {
		neg, ok := check.ExpectedDiffNegative(n.Name)
		if !ok {
			continue // not a differential-positive name, so not this rule's subject
		}
		v := check.Verdict{
			Kind:    check.KindNet,
			Subject: n.Name,
			NetID:   n.GetId(),
		}
		switch {
		case !uses:
			v.Outcome = check.NotConsidered
			v.Reason = "the design states no complete differential pair anywhere, so this suffix is not evidence that the net is half of one"
		case m.HasNetName(neg):
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("the design carries the complementary net %q", neg),
				Terms:     []check.WitnessTerm{{Label: "complement", Value: neg}},
			}
		default:
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("the design carries no net named %q", neg),
				Terms:     []check.WitnessTerm{{Label: "complement", Value: neg}},
			}
			v.Finding = &check.Finding{
				Kind:    check.KindNet,
				Subject: n.Name,
				NetID:   n.GetId(),
				Message: fmt.Sprintf("differential net has no complementary %q", neg),
				Prov:    n.Prov,
			}
		}
		out = append(out, v)
	}
	return out
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
