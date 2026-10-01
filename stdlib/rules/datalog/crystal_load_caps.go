package datalog

import (
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
)

// crystalLoadCapsQ is the datalog form of the built-in crystal-load-caps rule (WS3-074), compiled
// into crystalLoadCapsDL below. It is the PARITY TWIN of builtin.crystalLoadCaps, held
// finding-for-finding equal by TestCrystalDatalogParity, and it is NOT registered (absent from
// dlRules) because the conformance harness runs the Go rules only and would lose this rule's
// coverage (twin discipline in docsite/content/build/check-rule.md).
//
// The program mirrors the Go rule over component.class, net.ground and net.external (the Go rule's
// external-net read-gap skip):
//
//   - a clock part is the CLOCK FAMILY minus the subtypes that take no external caps (WS10-015). Net
//     membership reads component.net rather than pin.net, the same data the Go rule reads, so it
//     needs no resolved part types.
//   - term is a crystal's non-rail terminal net. net.rail() covers power AND ground, so a grounded case
//     pin drops out here.
//   - powered is a crystal with a pin on a SUPPLY rail (rail but not ground), i.e. an active
//     oscillator with a Vdd pin, which takes no external load caps.
//   - "exactly two terminals" is two AND not three, since the evaluator aggregates only in the goal
//     and not in an IDB head. It also excludes a 3+-pin active oscillator whose Vcc net is not
//     recognizable as a rail by name (the real-corpus false positive PR 265 fixed).
var crystalLoadCapsQ = query.FindingQuery{
	Rule: check.Rule{
		Name:     "crystal-load-caps",
		Severity: "warning",
		Summary:  "A passive crystal has an oscillator terminal with no load capacitor to ground.",
		Impact:   "A quartz crystal oscillates at its rated frequency only with the specified load capacitance on each terminal. Omit a load cap and the oscillator either will not start, starts intermittently over temperature, or runs off-frequency, which corrupts every timed peripheral downstream (UART baud, USB, CAN bit timing). Expressed in datalog over the device-class and net relations.",
		Remedy:   "Fit a load capacitor from each oscillator terminal to ground, sized from the crystal's specified load capacitance and the stray capacitance of the layout rather than copied from another design.",
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryConnectivity,
			check.KeyTier:         "R",
			check.KeyDistribution: check.DistOpen,
		},
	},
	Query: query.MustParse(`
		cap_on(?net)   :- component.net(?c, ?net), component.class(?c, "capacitor");
		clockpart(?y)  :- component.class(?y, "clock"), not component.class(?y, "oscillator"), not component.class(?y, "ceramic_resonator");
		term(?y, ?net) :- clockpart(?y), component.net(?y, ?net), not net.rail(?net);
		powered(?y)    :- clockpart(?y), component.net(?y, ?r), net.rail(?r), not net.ground(?r);
		two(?y)        :- term(?y, ?a), term(?y, ?b), ?a != ?b;
		three(?y)      :- term(?y, ?a), term(?y, ?b), term(?y, ?c), ?a != ?b, ?a != ?c, ?b != ?c;
		bad(?y, ?net)  :- term(?y, ?net), two(?y), not three(?y), not powered(?y), not cap_on(?net), not net.external(?net);
		bad(?y, ?net)  => ?y, ?net`),
	Kind:       check.KindComponent,
	SubjectVar: "y",
	Message:    "crystal terminal net {net} has no load capacitor",
	// The terminal net the message names. The subject is the crystal, the part a reader changes, but
	// both terminals sit inside its symbol, so only the net says which one is at fault (agni issue 349).
	ContextVars: []query.ContextVar{{Var: "net", Kind: check.KindNet, Role: "terminal"}},
}

var crystalLoadCapsDL = query.MustRuleFromQuery(crystalLoadCapsQ)
