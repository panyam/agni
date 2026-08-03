package datalog

import (
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
)

// crystalLoadCapsDL is the datalog expression of the built-in crystal-load-caps rule (WS3-074): a
// passive two-terminal crystal whose oscillator terminal carries no load capacitor. It is the
// PARITY TWIN of check.crystalLoadCaps, proven finding-for-finding equal by TestCrystalDatalogParity,
// and it is DELIBERATELY NOT registered (absent from dlRules, so it never enters DefaultCatalog).
//
// Two reasons it stays a twin rather than replacing the Go rule now, mirroring the Spec twin-then-flip
// discipline (docs/19): the conformance harness runs check.Rules only, so a registered datalog rule
// would lose that coverage; and the Go rule is freshly soaked (PR 265). The twin proves the datalog
// surface — extended here with component.class, net.ground, and net.external — can express the rule
// faithfully, including the external-net read-gap skip, leaving a clean flip for later.
//
// The program mirrors the Go rule's structure over the new relations:
//
//   - a clock part is the CLOCK FAMILY minus the subtypes that take no external caps (WS10-015):
//     component.class(?y,"clock") and not "oscillator" and not "ceramic_resonator"; a cap is
//     component.class(?c,"capacitor"). component-on-net (net connections), NOT pin.net (part-type
//     pins), is the right membership relation: it is the same data the Go rule reads and needs no
//     resolved part types.
//   - term = a crystal's non-rail terminal net. rail() covers power AND ground, so a grounded case
//     pin drops out here (it is not a signal terminal).
//   - powered = the crystal has a pin on a SUPPLY rail (rail but not ground) -> an active oscillator
//     with a Vdd pin, which uses no external load caps; exclude it entirely.
//   - the "exactly two terminals" gate is expressed WITHOUT aggregation (the evaluator aggregates
//     only in the goal, not an IDB head): two = has >=2 distinct terminal nets, three = has >=3, so
//     "exactly two" is two AND not three. This also structurally excludes a 3+-pin active oscillator
//     whose Vcc net is not name-recognizable as a rail (the real-corpus false-positive PR 265 fixed).
//   - bad fires on a terminal of an exactly-two-terminal, non-powered crystal that carries no cap and
//     is not an external read-gap net (net.external), the faithful successor to the Go external-skip.
var crystalLoadCapsDL = query.RuleFromQuery(query.FindingQuery{
	Rule: check.Rule{
		Name:     "crystal-load-caps",
		Severity: "warning",
		Summary:  "A passive crystal has an oscillator terminal with no load capacitor to ground.",
		Impact:   "A quartz crystal oscillates at its rated frequency only with the specified load capacitance on each terminal. Omit a load cap and the oscillator either will not start, starts intermittently over temperature, or runs off-frequency, which corrupts every timed peripheral downstream (UART baud, USB, CAN bit timing). Expressed in datalog over the device-class and net relations.",
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryConnectivity,
			check.KeyTier:         "R",
			check.KeyDistribution: check.DistOpen,
		},
	},
	Query: query.MustParse(`
		cap_on(?net)   :- component-on-net(?c, ?net), component.class(?c, "capacitor");
		clockpart(?y)  :- component.class(?y, "clock"), not component.class(?y, "oscillator"), not component.class(?y, "ceramic_resonator");
		term(?y, ?net) :- clockpart(?y), component-on-net(?y, ?net), not rail(?net);
		powered(?y)    :- clockpart(?y), component-on-net(?y, ?r), rail(?r), not net.ground(?r);
		two(?y)        :- term(?y, ?a), term(?y, ?b), ?a != ?b;
		three(?y)      :- term(?y, ?a), term(?y, ?b), term(?y, ?c), ?a != ?b, ?a != ?c, ?b != ?c;
		bad(?y, ?net)  :- term(?y, ?net), two(?y), not three(?y), not powered(?y), not cap_on(?net), not net.external(?net);
		bad(?y, ?net)  => ?y, ?net`),
	Kind:       check.KindComponent,
	SubjectVar: "y",
	Message:    "crystal terminal net {net} has no load capacitor",
})
