package check

import (
	"fmt"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// duplicateRefDes reports reference-designator collisions the reader detected. See Detail.
var duplicateRefDes = &Rule{
	Name:       "duplicate-ref-des",
	Severity:   "error",
	Summary:    "A reference designator is claimed by more than one distinct physical part.",
	Impact:     "Two parts sharing a ref-des collapse into one in the BOM and corrupt the net join key: one part is dropped from the build, or connections meant for two parts land on one. It is an annotation slip that silently falsifies both the BOM and the netlist.",
	Primitives: []string{"select"},
	Reads:      []string{"ref_des_collision"},
	Tags: map[string]string{
		KeyCategory:     CategoryConnectivity,
		KeyTier:         "P",
		KeyDistribution: DistOpen,
		KeySite:         SiteDiagnostic, // the reader decides duplicate vs multi-unit (docs/19)
	},
	Detail: ruleDoc("duplicate-ref-des"),
	Eval: func(m Model) []Finding {
		return Report(m.RefDesCollisions(), func(c *ir.RefDesCollision) Finding {
			var prov *ir.Provenance
			if len(c.Instances) > 0 {
				prov = c.Instances[0]
			}
			return Finding{
				Kind:    KindComponent,
				Subject: c.RefDes,
				Message: fmt.Sprintf("ref-des claimed by %d placements; expected one physical part", len(c.Instances)),
				Prov:    prov,
			}
		})
	},
}

// duplicateRefDesSpec is the rule's declarative twin (WS3-003).
var duplicateRefDesSpec = &Spec{
	Over:    "ref_des_collisions",
	Message: "ref-des claimed by {collision.instance_count} placements; expected one physical part",
}
