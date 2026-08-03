package check

import (
	"fmt"
	"strings"

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
		uses := diffConventionPresent(m)
		orphans := Select(m.Nets(), func(n *ir.Net) bool {
			neg, ok := expectedDiffNegative(n.Name)
			return uses && ok && !m.HasNetName(neg)
		})
		return Report(orphans, func(n *ir.Net) Finding {
			neg, _ := expectedDiffNegative(n.Name)
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

// diffConventionPresent reports whether the design uses differential-pair naming at all: at
// least one net has its expected complement present (a complete X_P/X_N pair). It is the
// pair-population evidence that gates orphan reporting, so a design with no differential pairs
// stays silent even when names coincidentally carry a _P/_DP/+ suffix.
func diffConventionPresent(m Model) bool {
	for _, n := range m.Nets() {
		if neg, ok := expectedDiffNegative(n.Name); ok && m.HasNetName(neg) {
			return true
		}
	}
	return false
}

// expectedDiffNegative returns the complementary negative net name for a differential-pair
// positive member, and ok=false when the name is not a positive member. Suffix families:
// "_P"/"_N", "_DP"/"_DN", and trailing "+"/"-". Matching is case-insensitive; the returned name
// preserves the source casing so the message reads naturally.
func expectedDiffNegative(name string) (string, bool) {
	up := strings.ToUpper(name)
	switch {
	case strings.HasSuffix(up, "_DP"), strings.HasSuffix(up, "_P"):
		// Both end in P; flip only the trailing P/p to N/n, preserving case.
		last := name[len(name)-1]
		flip := "N"
		if last == 'p' {
			flip = "n"
		}
		return name[:len(name)-1] + flip, true
	case strings.HasSuffix(name, "+"):
		return name[:len(name)-1] + "-", true
	}
	return "", false
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
