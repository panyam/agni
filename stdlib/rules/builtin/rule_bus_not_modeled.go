package builtin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// busNotModeled flags a bus whose member signals are NOT resolved into distinct nets (WS1-034). It is a
// read-health tripwire rather than a design defect, so it stays info severity while the reader is the
// known cause. It does NOT fire merely because a bus is present. On a flat sheet KiCad requires a member
// label on every bus tap, so the members already form nets by name (verified against kicad-cli), the
// bus is effectively modeled, and the finding is silent. It fires when the members cannot be confirmed
// as nets (a hierarchical bus port whose members do not cross the sheet boundary, an alias-named bus
// with no member taps), where connectivity really is unmodeled. See Detail.
var busNotModeled = &check.Rule{
	Name:       "bus-not-modeled",
	Severity:   "info",
	Summary:    "A bus's member signals are not resolved into distinct nets.",
	Impact:     "A bus groups many signals under one drawn line. When its members do not each resolve to a net, connectivity for those signals is unmodeled: a diff, a rule, or a highlight over them is unreliable. On a flat sheet the member labels on the taps already form the nets, so this is silent; it fires where the members are genuinely unresolved (e.g. a hierarchical bus port), marking the read as incomplete rather than letting silence read as coverage.",
	Remedy:     "Resolve the bus into its member nets, usually by labelling each member at its tap or by declaring the bus on its hierarchical port. Until then, connectivity over those signals is unmodelled and any diff or highlight across them is unreliable.",
	Primitives: []string{"select"},
	Reads:      []string{"bus.construct", "net.names"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryIntegrity,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
		check.KeySite:         check.SiteDiagnostic, // detected by the reader from the source's bus syntax
	},
	Detail:              ruleDoc("bus-not-modeled"),
	Eval:                busNotModeledVerdicts,
	StatesConsideredSet: true,
}

// busNotModeledVerdicts decides every bus construct the reader detected. This rule can state a
// considered set where the other reader-diagnostic rules cannot because the reader records EVERY bus
// it saw, not only the ones that turned out badly (#400). `unmodeled_buses` is named for what the
// rule reports rather than for what it holds, and the member check partitions it. The reader hands
// over only the dangling endpoints that failed, so dangling-endpoint and wire-no-junction stay
// failures-only.
//
// The pass is a flat-sheet bus whose taps carry member labels, so the members are nets by name and
// connectivity over them is modelled (verified against kicad-cli). That is the ordinary state of most
// buses, and a run that reported nothing about them could not tell it from a design with no bus.
//
// THE TWO FAILING CASES ARE DIFFERENT AND THE WITNESS SAYS WHICH. A bus whose member set the reader
// could not determine at all is a different gap from one whose members are known and are not nets,
// and the second names the member that is missing.
func busNotModeledVerdicts(ctx context.Context, m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, b := range m.UnmodeledBuses() {
		// A bus rather than a net, so its highlight join is its own label (WS7-042b).
		v := check.Verdict{Subjects: []check.Entity{check.BusEntity(busSubject(b))}}
		members := b.GetMembers()
		missing := firstUnmodelledMember(m, members)
		switch {
		case len(members) == 0:
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: "the reader could not determine the bus's member signals, so none of them could be confirmed as a net",
				Terms:     []check.WitnessTerm{{Label: "members named", Value: "0"}},
			}
		case missing != "":
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("the bus names %d member signal(s) and %q is not a net in this design",
					len(members), missing),
				Terms: []check.WitnessTerm{
					{Label: "members named", Value: strconv.Itoa(len(members))},
					{Label: "first member with no net", Value: missing},
				},
			}
		default:
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("all %d member signal(s) the bus names are nets in this design, so connectivity over them is modelled",
					len(members)),
				Terms: []check.WitnessTerm{{Label: "members named", Value: strconv.Itoa(len(members))}},
			}
		}
		if v.Outcome == check.Fail {
			v.Finding = &check.Finding{Subject: check.Entity{Kind: check.KindBus, Ref: busSubject(b)}, Message: busNotModeledMessage, Prov: b.Prov}
		}
		out = append(out, v)
	}
	return out
}

// firstUnmodelledMember names the first member signal that is not a net, or "" when every member is
// one. A reader told a bus is unresolved still has to know WHICH member to go and label.
//
// Member names are matched bare, so a hierarchy read whose member nets are qualified per-sheet
// (`/amp1/DATA0`) does NOT match the bare `DATA0`. That flags a bus whose members do not cross the
// sheet boundary.
func firstUnmodelledMember(m check.Model, members []string) string {
	for _, mem := range members {
		if !m.HasNetName(mem) {
			return mem
		}
	}
	return ""
}

// busNotModeledMessage is the finding message. The specific bus and its unresolved members live in the
// finding's Subject and Prov.
const busNotModeledMessage = "bus members are not modeled as nets (connectivity unresolved)"

// busSubject names the finding after the bus by its source name (a range label `DATA[7:0]` or a
// bus-alias name), else by the construct kind for a nameless one.
func busSubject(b *ir.BusNotModeled) string {
	if b.GetLabel() != "" {
		return b.GetLabel()
	}
	return b.GetKind()
}
