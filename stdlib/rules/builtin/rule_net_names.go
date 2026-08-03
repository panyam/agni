package builtin

import (
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// The net-name consistency batch (WS3-016/017): naming is connectivity on schematic
// formats — labels join by name — so a naming slip silently merges or splits nets. Three
// rules over two channels: the per-net alias list (netgraph.AttrAliases: every label the
// naming pass collapsed into one net) and the design's name->net-count index
// (Model.NetNameCount).
//
// The alias RANK tells the rules which scoping class a name came from (the docs/22 FQN
// model): rank 0 is design-wide (a global label or power-symbol rail), anything else is
// sheet-scoped (a local/hierarchical label, an inline wire label). A design-wide name and
// a local alias on one net is NORMAL (a rail with a local nickname); rivalry within one
// class is the hazard.

// netAliases parses the in-scope net's collapsed label list; nil when the net carried at
// most one name.
func netAliases(n *ir.Net) []netgraph.Alias {
	return netgraph.ParseAliases(n.GetAttributes()[netgraph.AttrAliases])
}

// joinNames renders a distinct name set for a finding message, sorted for determinism.
func joinNames(set map[string]bool) string {
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

var duplicateNetName = matrixlessSpecRule(func() *check.Rule {
	check.RegisterSpecFunc("nets_sharing_name", &check.SpecFunc{
		// How many nets in the design carry the in-scope net's exact name. Synthesized
		// stub names (N$, unconnected-()) are the reader's per-net inventions and never
		// collide; empty names carry no claim.
		Reads:      []string{"net.names"},
		Primitives: []string{"count", "pair"},
		Fn: func(m check.Model, ents map[string]any, _ []any) any {
			n := ents["net"].(*ir.Net)
			if n.Name == "" || isSyntheticNetName(n.Name) {
				return 1
			}
			return m.NetNameCount(n.Name)
		},
	})
	spec := &check.Spec{
		Over: "nets",
		Let: map[string]check.Term{
			"claims": check.Call{Fn: "nets_sharing_name"},
		},
		Where:   check.Cmp{L: check.Var{Name: "claims"}, Op: ">=", R: check.Lit{V: 2}},
		Message: "this name is stated by {claims} electrically distinct nets",
	}
	return spec.Rule(check.Rule{
		Name:     "duplicate-net-name",
		Severity: "warning",
		Summary:  "Two electrically distinct nets carry the same name.",
		Impact:   "A name is an identity: reviews, BOM tools, and cross-artifact joins (schematic vs board, revision diffs) assume one name means one net. Two nets under one name silently alias in every one of those views, and on connect-by-name formats the same slip would have MERGED the copper.",
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryConnectivity,
			check.KeyTier:         "R",
			check.KeyDistribution: check.DistOpen,
		},
		Detail: ruleDoc("duplicate-net-name"),
	})
})

var labelAliasConflict = matrixlessSpecRule(func() *check.Rule {
	check.RegisterSpecFunc("scoped_label_clash", &check.SpecFunc{
		// The distinct sheet-scoped (rank != 0) alias names that clash WITHIN one scope,
		// joined for the message; "" when no scope carries two names. A net legitimately
		// carries one name per sheet it crosses (ports qualify by instance path); two
		// names in the SAME scope mean two different labels on one wire.
		Reads:      []string{"net.attributes"},
		Primitives: []string{"pattern", "select"},
		Fn: func(m check.Model, ents map[string]any, _ []any) any {
			byScope := map[string]map[string]bool{}
			for _, a := range netAliases(ents["net"].(*ir.Net)) {
				if a.Rank == 0 {
					continue
				}
				scope, _ := check.ScopeOf(a.Name)
				if byScope[scope] == nil {
					byScope[scope] = map[string]bool{}
				}
				byScope[scope][a.Name] = true
			}
			clash := map[string]bool{}
			for _, names := range byScope {
				if len(names) >= 2 {
					for n := range names {
						clash[n] = true
					}
				}
			}
			return joinNames(clash)
		},
	})
	spec := &check.Spec{
		Over: "nets",
		Let: map[string]check.Term{
			"labels": check.Call{Fn: "scoped_label_clash"},
		},
		Where:   check.Cmp{L: check.Var{Name: "labels"}, Op: "!=", R: check.Lit{V: ""}},
		Message: "one net carries rival labels in the same sheet scope: {labels}",
	}
	return spec.Rule(check.Rule{
		Name:     "label-alias-conflict",
		Severity: "warning",
		Summary:  "One net carries two different sheet-scoped labels in the same scope.",
		Impact:   "Two labels on one wire read as two signals to every human and one net to the tool. Whichever name a reviewer greps for, half the net's story is elsewhere; and if the author MEANT two nets, the wire between the labels is the defect.",
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryConnectivity,
			check.KeyTier:         "R",
			check.KeyDistribution: check.DistOpen,
		},
		Detail: ruleDoc("label-alias-conflict"),
	})
})

var powerTapConflict = matrixlessSpecRule(func() *check.Rule {
	check.RegisterSpecFunc("rail_name_clash", &check.SpecFunc{
		// The distinct design-wide (rank 0) alias names on the in-scope net, joined for
		// the message; "" unless there are at least two. Rank 0 is a global label or a
		// power-symbol rail name — the names that unify across the whole design.
		Reads:      []string{"net.attributes"},
		Primitives: []string{"pattern", "select"},
		Fn: func(m check.Model, ents map[string]any, _ []any) any {
			rails := map[string]bool{}
			for _, a := range netAliases(ents["net"].(*ir.Net)) {
				if a.Rank == 0 {
					rails[a.Name] = true
				}
			}
			if len(rails) < 2 {
				return ""
			}
			return joinNames(rails)
		},
	})
	spec := &check.Spec{
		Over: "nets",
		Let: map[string]check.Term{
			"rails": check.Call{Fn: "rail_name_clash"},
		},
		Where:   check.Cmp{L: check.Var{Name: "rails"}, Op: "!=", R: check.Lit{V: ""}},
		Message: "one net is claimed by rival design-wide names: {rails}",
	}
	return spec.Rule(check.Rule{
		Name:     "power-tap-conflict",
		Severity: "warning",
		Summary:  "One net is tapped by two different design-wide names (power symbols or global labels).",
		Impact:   "A rail tapped +3V3 here and +3.3V there is one net pretending to be two rails. Every OTHER +3.3V tap in the design joins this net too — the classic multi-sheet capture slip where two supplies quietly become one, or one rail's consumers scatter across two names.",
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryPower,
			check.KeyTier:         "R",
			check.KeyDistribution: check.DistOpen,
		},
		Detail: ruleDoc("power-tap-conflict"),
	})
})

// matrixlessSpecRule mirrors the matrix rows' bind-time discipline for standalone
// spec-only rules: the constructor registers the rule's FFIs before Spec.Rule validates
// its Call targets (package vars init before init funcs — the ledPolarity lesson).
func matrixlessSpecRule(build func() *check.Rule) *check.Rule { return build() }

// isSyntheticNetName reports a reader-invented per-net stub name: netgraph's N$<n>, the
// no-connect marker vocabulary, and KiCad's own synthesized pad stubs. These names are
// manufactured per net and carry no authored identity, so naming rules skip them.
func isSyntheticNetName(name string) bool {
	return strings.HasPrefix(name, "N$") ||
		strings.HasPrefix(name, "unconnected-(") ||
		strings.HasPrefix(name, "Net-(")
}
