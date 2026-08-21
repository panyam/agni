package builtin

import (
	"fmt"

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

// railCandidate is a net that looks like a rail by two independent channels but carries no rail role.
type railCandidate struct {
	net       *ir.Net
	volts     float64
	supplyPin string // one ref_des:pin feeding evidence, for the message
	// The same pin as a pair rather than a formatted string, so it can travel as a context entity
	// (agni issue 349). supplyPin stays because the message reads better with the words in it.
	supplyRef   string
	supplyPinNo string
	supplies    int
}

// eachRailCandidate walks the nets whose NAME declares a voltage and whose CONNECTIONS include a
// power-input pin, and yields those the role stamp does not call a rail. Ground is excluded: a
// ground net carries a role of its own.
func eachRailCandidate(m check.Model, yield func(railCandidate)) {
	for _, n := range m.Nets() {
		if m.IsRailNet(n) || m.IsGroundNet(n) {
			continue
		}
		volts, ok := check.NominalVoltageFromName(n.GetName())
		if !ok {
			continue
		}
		c := railCandidate{net: n, volts: volts}
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
		if c.supplies == 0 {
			continue
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
	Detail: ruleDoc("rail-not-classified"),
	Eval: func(m check.Model) []check.Finding {
		var out []check.Finding
		eachRailCandidate(m, func(rc railCandidate) {
			out = append(out, check.Finding{
				Kind:    check.KindNet,
				Subject: rc.net.GetName(),
				NetID:   rc.net.GetId(),
				Prov:    rc.net.GetProv(),
				Message: fmt.Sprintf(
					"net %q declares %gV in its name and feeds %d supply pin(s) (e.g. %s), but carries no rail role, so the rail rules and net.nominal_voltage skip it. If this project names rails off the built-in vocabulary, declare its patterns in a --conventions lexicon",
					rc.net.GetName(), rc.volts, rc.supplies, rc.supplyPin),
				// The supply pin the message offers as evidence. The subject is the net, so the pin
				// that makes it look like a rail was named in prose and reachable nowhere
				// (agni issue 349).
				Context: []check.ContextSubject{
					{Kind: check.KindPin, Subject: rc.supplyRef, Pin: rc.supplyPinNo, Role: "supply-pin"},
				},
			})
		})
		return out
	},
}
