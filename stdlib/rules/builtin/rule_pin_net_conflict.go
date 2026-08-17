package builtin

import "github.com/panyam/agni/core/check"

// pinNetConflict reports malformed input: a pin that appears in more than one net's
// connections. Pins-to-net is many-to-one by definition, a net being the equivalence class
// of joined pins, so a finding means a reader gap or a corrupt export: fix the read, not
// the design. It also guards PinNetName's first-net-wins contract, whose arbitrary pick is
// safe only because a conflict is reported. Its first run over real designs caught two
// reader gaps within minutes: unannotated REF** placeholders merged by the PCB reader
// (WS1-024), and duplicate "GND" port designators collapsed by the EDIF reader (WS1-025).
var pinNetConflict = (&check.Spec{
	Over:    "pin_net_conflicts",
	Message: "pin appears in more than one net ({pin_conflict.nets}); a pin belongs to exactly one",
}).Rule(check.Rule{
	Name:     "pin-net-conflict",
	Severity: "info",
	Summary:  "A pin appears in more than one net's connections, which is malformed input.",
	Impact:   "Every consumer that asks which net a pin is on (rules, diff keying, highlights) gets an arbitrary answer for this pin. The netlist is not internally consistent, and anything derived from it inherits the ambiguity silently.",
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryIntegrity,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("pin-net-conflict"),
})
