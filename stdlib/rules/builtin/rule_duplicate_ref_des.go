package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// duplicateRefDes reports reference-designator collisions the reader detected. See Detail.
var duplicateRefDes = &check.Rule{
	Name:       "duplicate-ref-des",
	Severity:   "error",
	Summary:    "A reference designator is claimed by more than one distinct physical part.",
	Impact:     "Two parts sharing a ref-des collapse into one in the BOM and corrupt the net join key: one part is dropped from the build, or connections meant for two parts land on one. It is an annotation slip that silently falsifies both the BOM and the netlist.",
	Primitives: []string{"select"},
	Reads:      []string{"ref_des_collision"},
	// The rule IS the reader's diagnostic, so a reader that does not compute it leaves this rule
	// with nothing to report — which is exactly what a clean design looks like. Gating on the
	// capability makes the difference visible: not-applicable, with a reason, instead of a pass
	// nobody earned (agni issue 309).
	RequiresCapability: []check.Capability{check.CapRefDesCollisions},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
		check.KeySite:         check.SiteDiagnostic, // the reader decides duplicate vs multi-unit (docsite/content/architecture/rules-and-checks.md)
	},
	Detail: ruleDoc("duplicate-ref-des"),
	Eval: func(m check.Model) []check.Finding {
		return check.Report(m.RefDesCollisions(), func(c *ir.RefDesCollision) check.Finding {
			var prov *ir.Provenance
			if len(c.Instances) > 0 {
				prov = c.Instances[0]
			}
			return check.Finding{
				Kind:    check.KindComponent,
				Subject: c.RefDes,
				Message: fmt.Sprintf("ref-des claimed by %d placements; expected one physical part", len(c.Instances)),
				Prov:    prov,
			}
		})
	},
}

// duplicateRefDesSpec is the rule's declarative twin (WS3-003).
var duplicateRefDesSpec = &check.Spec{
	Over:    "ref_des_collisions",
	Message: "ref-des claimed by {collision.instance_count} placements; expected one physical part",
}
