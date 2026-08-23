package builtin

import (
	"fmt"
	"strconv"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// wireNoJunction flags a wire endpoint dropped onto the BODY of another wire with no
// junction dot, the T-tap that looks connected and is not. See Detail.
var wireNoJunction = &check.Rule{
	Name:       "wire-no-junction",
	Severity:   "warning",
	Summary:    "A wire endpoint lands mid-span on another wire with no junction dot.",
	Impact:     "The drawing shows a T-tap; the tool sees two separate nets. The author believes net A and net B are one, every visual review confirms it, and the board ships with the connection missing. It is the most dangerous silent connectivity slip in schematic capture.",
	Remedy:     "Place a junction dot where the wires meet if the connection is intended, or move one wire clear of the other if it is not. Do not leave the crossing to be read by eye.",
	Primitives: []string{"select"},
	Reads:      []string{"wire.endpoint", "wire.junction"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
		check.KeySite:         check.SiteDiagnostic, // reader detects it from wire geometry (docsite/content/architecture/rules-and-checks.md)
	},
	Detail: ruleDoc("wire-no-junction"),
	// Only the KiCad reader examines wire geometry, so on every other format this rule found nothing
	// and a clean pass is what finding nothing looks like. Gating is what makes the considered set
	// below an honest claim rather than one three formats silently fail (agni issue 309's shape,
	// applied to the diagnostic this rule owns).
	RequiresCapability:  []check.Capability{check.CapJunctionTaps},
	Eval:                wireNoJunctionVerdicts,
	StatesConsideredSet: true,
}

// wireNoJunctionVerdicts decides every wire-end-on-wire-body tap, the ones something joins and the
// ones nothing does.
//
// This rule was the most expensive one in the catalog to leave silent, and the silence was total. Its
// own Impact says why: a T-tap with no dot is drawn as a connection the netlist does not have, so the
// author, the reviewer and the plot all agree while the board ships with the connection missing. A
// design whose every tap carried its dot reported exactly what a design with no tap in it reported,
// and exactly what a format that cannot see wire geometry reported, which is nothing.
//
// The reader had the answer and threw it away. `splitWiresAt` runs at every junction dot and mid-span
// label BEFORE the detection pass, so a joined tap is an endpoint of both wires by the time anything
// looks and is indistinguishable from a point where no wire ever crossed. Running the same detection
// once more BEFORE the split recovers the set (agni issue 420), which is the same move that converted
// symbol-unresolved: the reader already decided, and only the good half was being discarded.
//
// THE PASS NAMES THE CONSTRUCT THAT DID THE JOINING, and that is the whole of what makes it evidence.
// "The tap is joined" is a restatement of the outcome, and it reads identically on a tap held by a
// junction dot somebody placed on purpose and one held by a label that happens to sit at the meet.
// The second is correct in KiCad and is a great deal easier to delete by accident, so a reviewer
// checking a T-tap wants to know which they have. The segment count separates an ordinary T from a
// crossing.
func wireNoJunctionVerdicts(m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, e := range m.NoJunctionEndpoints() {
		v := check.Verdict{
			Subjects: []check.Entity{check.EndpointEntity(e.GetX(), e.GetY())},
			Outcome:  check.Fail,
			Witness: &check.Witness{
				Statement: "the endpoint lands on another wire's body and nothing joins them there, so the drawing shows one net where the netlist has two",
				Terms:     []check.WitnessTerm{{Label: "joined by", Value: "nothing"}},
			},
		}
		v.Finding = &check.Finding{
			Subject: check.EndpointEntity(e.GetX(), e.GetY()),
			Message: "wire endpoint touches another wire mid-span with no junction dot",
			Prov:    e.GetProv(),
		}
		out = append(out, v)
	}
	for _, t := range m.JoinedTaps() {
		out = append(out, check.Verdict{
			Subjects: []check.Entity{check.EndpointEntity(t.GetX(), t.GetY())},
			Outcome:  check.Pass,
			Witness: &check.Witness{
				Statement: fmt.Sprintf("the endpoint lands on another wire's body and %s joins them, so the %d wire ends meeting here are one net",
					tapJoinPhrase(t), t.GetSegments()),
				Terms: []check.WitnessTerm{
					{Label: "joined by", Value: tapJoinPhrase(t)},
					{Label: "wire ends", Value: strconv.Itoa(int(t.GetSegments()))},
				},
			},
		})
	}
	return out
}

// tapJoinPhrase names the construct holding a tap together. A label's TEXT is carried because it is
// the net name the tap resolves to, which is the thing a reviewer opens the schematic to confirm; a
// junction dot names no net and has nothing to add. An unrecognised kind falls back to the kind
// string rather than to "something", since guessing would blur the one distinction the witness exists
// to draw.
func tapJoinPhrase(t *ir.JoinedTap) string {
	switch t.GetJoinKind() {
	case "junction":
		return "a junction dot"
	case "label":
		if t.GetLabel() != "" {
			return fmt.Sprintf("label %q", t.GetLabel())
		}
		return "a label"
	}
	return t.GetJoinKind()
}

// wireNoJunctionSpec is the rule's declarative twin (WS3-003); the no_junction_endpoints over-set
// is new vocabulary, so the Go Eval stays canonical (twin discipline:
// docsite/content/build/check-rule.md).
var wireNoJunctionSpec = &check.Spec{
	Over:    "no_junction_endpoints",
	Message: "wire endpoint touches another wire mid-span with no junction dot",
}
