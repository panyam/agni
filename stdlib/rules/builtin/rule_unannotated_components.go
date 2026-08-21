package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/refdes"
)

// unannotatedComponents reports parts whose designator is still a placeholder. See Detail.
//
// warning, not error: mid-design this is the normal state of a sheet somebody is still drawing, and
// calling it an error would make every work-in-progress read fail. Not info either, because it does
// not survive release — an unannotated part has no BOM line to buy and no designator to find it by
// on the board. duplicate-ref-des earns error because a collision falsifies a netlist that claims to
// be finished; this one says the design is not finished yet.
//
// One finding per PLACEHOLDER rather than per part, because "176 parts are still called R?" is the
// reviewable fact and 176 identical findings is the same sentence 176 times.
var unannotatedComponents = &check.Rule{
	Name:       "unannotated-components",
	Severity:   "warning",
	Summary:    "Parts still carry a placeholder designator instead of an assigned one.",
	Impact:     "An unannotated part has no identity, so nothing downstream can name it: it has no BOM line to order, no designator to locate it by on the board, and no stable key for a diff to track it across revisions. It also silences checks rather than failing them — a placeholder is shared by every unannotated part of its kind, so any rule keying on the designator sees one impossible part instead of many real ones, and pin-net-conflict declines to judge them at all. The parts are drawn and connected; only their names are missing, which is why the design otherwise reads as complete.",
	Remedy:     "Annotate the schematic so every part carries an assigned designator. Until then those parts have no BOM line, no place on the board, and no identity a diff can follow across revisions.",
	Primitives: []string{"select"},
	Reads:      []string{"unannotated_component"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryIntegrity,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
		check.KeySite:         check.SiteDiagnostic, // the reader sees the designator; the IR cannot infer intent
	},
	Detail:              ruleDoc("unannotated-components"),
	Eval:                unannotatedComponentsVerdicts,
	StatesConsideredSet: true,
}

// unannotatedComponentsVerdicts decides every designator the design carries, so a run says which
// parts were checked for annotation and not only which ones failed. The two halves are the same
// question asked of the same set, split by `refdes.IsPlaceholder`, and the rule calls that predicate
// rather than re-deriving one: a layer deriving its own is how the predicate stopped being single in
// the first place (agni issue 311), and here it would let a designator be reported as neither
// assigned nor a placeholder.
//
// THE GROUPING DIFFERS BETWEEN THE TWO OUTCOMES, deliberately. A failure is per PLACEHOLDER, because
// "176 parts are still called R?" is one reviewable fact and 176 copies of it is not; a pass is per
// designator, because an assigned designator names exactly one part. Both are still one verdict per
// subject, since the subject IS the designator on either side.
//
// Failures come first, in the reader's order, so the findings projection stays byte-identical to
// what this rule has always reported.
func unannotatedComponentsVerdicts(m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, u := range m.UnannotatedComponents() {
		var prov *ir.Provenance
		if len(u.GetInstances()) > 0 {
			prov = u.GetInstances()[0]
		}
		n := len(u.GetInstances())
		out = append(out, check.Verdict{
			Kind:    check.KindComponent,
			Subject: u.GetRefDes(),
			Outcome: check.Fail,
			Witness: &check.Witness{
				Statement: fmt.Sprintf("%d placement(s) share the placeholder designator %q, which is annotation state rather than a name", n, u.GetRefDes()),
				Terms:     []check.WitnessTerm{{Label: "placements", Value: fmt.Sprint(n)}},
			},
			Finding: &check.Finding{
				Kind:    check.KindComponent,
				Subject: u.GetRefDes(),
				Message: unannotatedMessage(u),
				Prov:    prov,
			},
		})
	}

	seen := map[string]bool{}
	for _, c := range m.Components() {
		ref := c.GetRefDes()
		if seen[ref] || refdes.IsPlaceholder(ref) {
			continue // the placeholders are already reported above, grouped
		}
		seen[ref] = true
		v := check.Verdict{Kind: check.KindComponent, Subject: ref}
		if ref == "" {
			// Not a placeholder and not a name either. `IsPlaceholder` describes designators a tool
			// WROTE as unassigned ("R?", "REF**"), and an absent designator is a different gap the
			// reader's diagnostic does not carry, so the rule says it cannot judge rather than
			// counting the part as annotated.
			v.Outcome = check.NotConsidered
			v.Reason = "the part carries no designator at all, which is neither an assigned name nor a placeholder the source wrote"
		} else {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("designator %q is assigned, carrying no placeholder mark", ref),
			}
		}
		out = append(out, v)
	}
	return out
}

// unannotatedMessage states the count, because the placeholder alone does not say how much of the
// design is unnamed: one forgotten test point and ninety unnamed decoupling caps read identically
// until the number is there.
func unannotatedMessage(u *ir.UnannotatedComponent) string {
	n := len(u.GetInstances())
	if n == 1 {
		return fmt.Sprintf("1 part still carries the placeholder designator %q; annotate before release.", u.GetRefDes())
	}
	return fmt.Sprintf("%d parts still carry the placeholder designator %q; annotate before release.", n, u.GetRefDes())
}
