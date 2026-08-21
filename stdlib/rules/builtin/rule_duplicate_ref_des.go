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
	Remedy:     "Re-annotate the schematic so each physical part holds its own designator, then re-check the BOM, since whichever part was silently merged is the one to look at first.",
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
	Detail:              ruleDoc("duplicate-ref-des"),
	Eval:                duplicateRefDesVerdicts,
	StatesConsideredSet: true,
}

// duplicateRefDesVerdicts decides every designator the design carries, which is a wider set than the
// reader's collision list and is the whole reason for stating it. A collision list alone answers
// "which designators are claimed twice"; the considered set answers "which designators were checked
// for that", and those differ by every part on the board.
//
// THE CAPABILITY GATE IS RESTATED HERE, and it is restated over the PASSES ONLY. `Run` does not
// consult RequiresCapability — that is the review layer's call — so on a format whose reader never
// looks for collisions this Eval still runs and still finds none, which is what a clean design looks
// like. That was agni issue 309, and the rule-level gate fixed it only for callers that ask
// Available first. What a caller that skipped Available must not be told is "R7 has no collision",
// so those subjects come back NotConsidered instead. A reported collision needs no such hedge: the
// reader cannot report one without having looked.
//
// The fails come first and in the reader's own order so the findings projection is byte-identical to
// what this rule has always reported. Passes follow in design order, deduplicated by designator,
// because the subject is the DESIGNATOR rather than the placement: a verdict is keyed by
// (rule, kind, ref), so a second verdict about the same ref-des would be a duplicate identity.
func duplicateRefDesVerdicts(m check.Model) []check.Verdict {
	collided := map[string]bool{}
	out := make([]check.Verdict, 0, len(m.Components()))
	for _, c := range m.RefDesCollisions() {
		collided[c.RefDes] = true
		var prov *ir.Provenance
		if len(c.Instances) > 0 {
			prov = c.Instances[0]
		}
		out = append(out, check.Verdict{
			Kind:    check.KindComponent,
			Subject: c.RefDes,
			Outcome: check.Fail,
			Witness: &check.Witness{
				Statement: fmt.Sprintf("%d placements claim the designator %q", len(c.Instances), c.RefDes),
				Terms:     []check.WitnessTerm{{Label: "placements", Value: fmt.Sprint(len(c.Instances))}},
			},
			Finding: &check.Finding{
				Kind:    check.KindComponent,
				Subject: c.RefDes,
				Message: fmt.Sprintf("ref-des claimed by %d placements; expected one physical part", len(c.Instances)),
				Prov:    prov,
			},
		})
	}

	looked := m.SuppliesDiagnostic(string(check.CapRefDesCollisions))
	for _, ref := range dedupedRefDes(m) {
		if collided[ref] {
			continue
		}
		v := check.Verdict{Kind: check.KindComponent, Subject: ref}
		if !looked {
			v.Outcome = check.NotConsidered
			v.Reason = "this format's reader does not detect ref-des collisions, so nothing was compared"
		} else {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				// The reader is the authority on multi-unit versus duplicate (one part drawn as
				// three sections is not a collision), so the pass rests on the same judgement the
				// failure does rather than on a second count taken here that could disagree with it.
				Statement: fmt.Sprintf("the reader found no second physical part claiming %q", ref),
			}
		}
		out = append(out, v)
	}
	return out
}

// dedupedRefDes yields the design's designators once each, in design order, skipping the empty one.
// A part with no designator has no identity to collide on and nothing to name in a report, so it is
// not a subject of this rule at all; unannotated-components is the rule that has something to say
// about it.
func dedupedRefDes(m check.Model) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range m.Components() {
		if c.RefDes == "" || seen[c.RefDes] {
			continue
		}
		seen[c.RefDes] = true
		out = append(out, c.RefDes)
	}
	return out
}

// duplicateRefDesSpec is the rule's declarative twin (WS3-003).
var duplicateRefDesSpec = &check.Spec{
	Over:    "ref_des_collisions",
	Message: "ref-des claimed by {collision.instance_count} placements; expected one physical part",
}
