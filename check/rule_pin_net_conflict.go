package check

// pinNetConflict reports malformed input: a pin that appears in more than one net's
// connections. Pins-to-net is many-to-one by definition — a net IS the equivalence class
// of joined pins — so this state cannot come from a correct capture + read; it means a
// reader gap or a corrupt export. The rule exists so PinNetName's first-net-wins
// contract stays honest: the arbitrary pick is safe because the condition that makes it
// arbitrary is loudly reported. Its first run over the real corpus caught two reader
// gaps within minutes (unannotated REF** placeholders merged by the PCB reader, WS1-024;
// duplicate "GND" port designators collapsed by the EDIF reader, WS1-025) — the tripwire
// is not hypothetical, and its findings mean "fix the read", not "fix the design".
var pinNetConflict = (&Spec{
	Over:    "pin_net_conflicts",
	Message: "pin appears in more than one net ({pin_conflict.nets}); a pin belongs to exactly one",
}).Rule(Rule{
	Name:     "pin-net-conflict",
	Severity: "info",
	Summary:  "A pin appears in more than one net's connections — malformed input.",
	Impact:   "Every consumer that asks which net a pin is on (rules, diff keying, highlights) gets an arbitrary answer for this pin. The netlist is not internally consistent, and anything derived from it inherits the ambiguity silently.",
	Tags: map[string]string{
		KeyCategory:     CategoryIntegrity,
		KeyTier:         "P",
		KeyDistribution: DistOpen,
	},
	Detail: ruleDoc("pin-net-conflict"),
})
