package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
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
	Detail: ruleDoc("unannotated-components"),
	Eval: func(m check.Model) []check.Finding {
		return check.Report(m.UnannotatedComponents(), func(u *ir.UnannotatedComponent) check.Finding {
			var prov *ir.Provenance
			if len(u.GetInstances()) > 0 {
				prov = u.GetInstances()[0]
			}
			return check.Finding{
				Kind:    check.KindComponent,
				Subject: u.GetRefDes(),
				Message: unannotatedMessage(u),
				Prov:    prov,
			}
		})
	},
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
