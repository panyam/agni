package builtin

import (
	"fmt"
	"strconv"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// railNotClassified is the tripwire for a project that has not declared its rail naming vocabulary.
// The rail-quantified rules and `net.nominal_voltage` all answer over nets carrying the RAIL ROLE,
// stamped at ingestion from the naming lexicon, and the built-in lexicon is start-anchored (VCC,
// VDD, +3V3). A project naming rails function-first (PMIC_CORE_3V3, SENSOR_5V0) matches none of it
// and must supply its own through `--conventions`. Measured on a real 1700-net board, declaring the
// lexicon moved the rail count from 13 to 91.
//
// It needs a STRUCTURAL signal as well as the name, because `..._3V3` may be a 3.3V rail or a signal
// that SWINGS at 3.3V (agni issue 194 is the same ambiguity from the other side): the net must also
// feed at least one pin the design types as a power INPUT.
//
// CategoryIntegrity because a firing means fix the CONFIG, not the design. Full rationale in
// docs/rail-not-classified.md.

// railCandidate is a net whose NAME declares a voltage: the first of the rule's two channels, before
// the second one is consulted.
type railCandidate struct {
	net       *ir.Net
	volts     float64
	supplyPin string // one ref_des:pin feeding evidence, for the message
	// The same pin as a pair rather than a formatted string, so it can travel as a context entity
	// (agni issue 349). supplyPin stays because the message reads better with the words in it.
	supplyRef   string
	supplyPinNo string
	supplies    int
	railRole    bool // the role stamp already calls this net a rail
}

// eachRailCandidate walks the nets whose NAME declares a voltage and yields one per net, with the
// second channel's count attached rather than applied as a filter. Ground is excluded: a ground net
// carries a role of its own, so it is not a subject of a rail-classification question.
//
// THE COUNT TRAVELS INSTEAD OF GATING, which is the difference the considered set needs. A net naming
// a voltage that types NO power-input pin is not a net the rule cleared; it is a net whose second
// channel is missing, and the caller reports that rather than dropping it. On a source format that
// cannot type power pins at all the second channel is absent everywhere, so this rule was a silent
// no-op on those formats and nothing said so — the exact case the two-channel section of
// docsite/content/build/check-rule.md warns has to be checked.
func eachRailCandidate(m check.Model, yield func(railCandidate)) {
	for _, n := range m.Nets() {
		if m.IsGroundNet(n) {
			continue
		}
		volts, ok := check.NominalVoltageFromName(n.GetName())
		if !ok {
			continue // the first channel is absent: the name says nothing about a voltage
		}
		c := railCandidate{net: n, volts: volts, railRole: m.IsRailNet(n)}
		for _, conn := range n.GetConnections() {
			if !check.SupplyInputPin(m, conn.GetComponentRef(), conn.GetPinRef()) {
				continue
			}
			if c.supplies == 0 {
				c.supplyPin = conn.GetComponentRef() + " pin " + conn.GetPinRef()
				c.supplyRef, c.supplyPinNo = conn.GetComponentRef(), conn.GetPinRef()
			}
			c.supplies++
		}
		yield(c)
	}
}

var railNotClassified = &check.Rule{
	Name:     "rail-not-classified",
	Severity: "warning",
	Summary:  "A net named for a voltage feeds a supply pin but is not classified as a rail, so the rail rules cannot see it.",
	Impact: "Every rail-quantified rule and the net.nominal_voltage fact answer over nets carrying the rail role, " +
		"which is stamped from the naming lexicon. A house convention the built-in vocabulary does not match leaves " +
		"those rails invisible, and the run reports clean because the rules had nothing to quantify over rather than " +
		"because the board is right. Declaring the project's rail patterns under `--conventions` restores them. " +
		"This says nothing about the design being wrong: it reports that the analysis is running with less than it should.",
	Remedy:     "Declare the project's rail naming patterns under `--conventions` so the rail rules can see this net. This does not report a fault in the design, only that the analysis is running with less than it should.",
	Primitives: []string{"select", "traverse", "pin-role"},
	Reads:      []string{"net.name", "net.role", "pin.type", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryIntegrity,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
		check.KeySite:         check.SiteDiagnostic,
	},
	Detail:              ruleDoc("rail-not-classified"),
	Eval:                railNotClassifiedVerdicts,
	StatesConsideredSet: true,
}

// railNotClassifiedVerdicts decides every net whose name declares a voltage, and the three answers it
// can give are the reason this rule is worth converting at all.
//
// A net the role stamp already calls a rail PASSES, and the pass is the useful half here: this rule
// exists to report that the analysis is running with less than it should, so "the rail rules can see
// this net" is exactly what a reader wants counted. Before, a project that declared its lexicon
// correctly got silence, which is what a project with no lexicon at all also got once its rails were
// invisible for a different reason.
//
// A net that names a voltage but types NO power-input pin is NotConsidered. The rule needs two
// channels to agree — a voltage in the name AND a pin the part declares a power input — because
// `..._3V3` is a legitimate name for a signal that swings at 3.3 V as well as for a rail. Where the
// second channel is missing the rule has not judged the net, and on a source format that cannot type
// power pins it is missing on every net, which turns the rule into a no-op that nothing announced.
// That is the case the two-channel section of the authoring doc says to check for, and this is the
// check.
func railNotClassifiedVerdicts(m check.Model) []check.Verdict {
	var out []check.Verdict
	eachRailCandidate(m, func(rc railCandidate) {
		v := check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindNet, Ref: rc.net.GetName(), NetID: rc.net.GetId()}}}
		switch {
		case rc.supplies == 0:
			v.Outcome = check.NotConsidered
			v.Reason = fmt.Sprintf("the name declares %gV but the design types no power-input pin on the net, "+
				"so the second channel this rule needs is absent and the name alone could equally be a signal that swings at %gV",
				rc.volts, rc.volts)
		case rc.railRole:
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("the net carries the rail role, so the rail rules and net.nominal_voltage answer over it (%gV, %d supply pin(s))",
					rc.volts, rc.supplies),
				Terms: []check.WitnessTerm{
					{Label: "nominal", Value: fmt.Sprintf("%gV", rc.volts)},
					{Label: "supply pins", Value: strconv.Itoa(rc.supplies)},
				},
			}
			v.Context = []check.ContextSubject{
				{Entity: check.Entity{Kind: check.KindPin, Ref: rc.supplyRef, Pin: rc.supplyPinNo}, Role: "supply-pin"},
			}
		default:
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("both channels agree the net is a %gV rail (%d supply pin(s)) and the role stamp does not call it one",
					rc.volts, rc.supplies),
				Terms: []check.WitnessTerm{
					{Label: "nominal", Value: fmt.Sprintf("%gV", rc.volts)},
					{Label: "supply pins", Value: strconv.Itoa(rc.supplies)},
				},
			}
			v.Finding = &check.Finding{
				Subject: check.NetEntity(rc.net),
				Prov:    rc.net.GetProv(),
				Message: fmt.Sprintf(
					"net %q declares %gV in its name and feeds %d supply pin(s) (e.g. %s), but carries no rail role, so the rail rules and net.nominal_voltage skip it. If this project names rails off the built-in vocabulary, declare its patterns in a --conventions lexicon",
					rc.net.GetName(), rc.volts, rc.supplies, rc.supplyPin),
				// The supply pin the message offers as evidence. The subject is the net, so the pin
				// that makes it look like a rail was named in prose and reachable nowhere
				// (agni issue 349).
				Context: []check.ContextSubject{
					{Entity: check.Entity{Kind: check.KindPin, Ref: rc.supplyRef, Pin: rc.supplyPinNo}, Role: "supply-pin"},
				},
			}
			v.Context = v.Finding.Context
		}
		out = append(out, v)
	})
	return out
}
