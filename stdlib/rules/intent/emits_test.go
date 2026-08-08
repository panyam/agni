package intent

import "testing"

// TestEmits covers the vocabulary Emits recognizes and, load-bearingly, the names it must REJECT: a
// not-yet-shipped intent rule name (the WS3-098 pre-bind case) and a non-intent name.
func TestEmits(t *testing.T) {
	known := []string{
		"module-missing", "module-count", "voltage-domain-mismatch",
		"intent/module-missing", // composed catalog name accepted too
		"subsystem-power-tree", "intent/subsystem-clock",
		"protection-ovp", "protection-discharge", "intent/protection-discharge",
		"rail-current-capacity", "rail-current-margin", "intent/rail-current-margin",
		// WS3-092. A manifest that pre-bound these names read not-automated until the rule kind
		// shipped; missing this line is the one wiring step that fails nothing at build time and
		// leaves every bound item quiet forever.
		"sequence-soc-power-tree", "intent/sequence-modem-rails",
	}
	for _, n := range known {
		if !Emits(n) {
			t.Errorf("Emits(%q) = false, want true", n)
		}
	}
	unknown := []string{
		"thermal-budget", "intent/thermal-budget", // a future intent rule kind, not yet shipped
		"reset-polarity", "intent/reset-polarity",
		"single-pin-net", "", "intent/",
	}
	for _, n := range unknown {
		if Emits(n) {
			t.Errorf("Emits(%q) = true, want false", n)
		}
	}
}

// TestEmitsCoversCompiler holds Emits to Compile's actual output: every rule the compiler produces from
// a declaration exercising all four kinds must satisfy Emits. A new intent rule KIND added to Compile
// without updating Emits fails here, so the two cannot drift.
func TestEmitsCoversCompiler(t *testing.T) {
	decl := Declaration{
		Name:           "t",
		Modules:        []Module{{Name: "soc", Class: "soc", Count: 2}},
		VoltageDomains: []VoltageDomain{{Name: "io", Nominal: 3.3, Rails: []string{"3V3"}}},
		Subsystems:     []Subsystem{{Name: "power tree", Nets: []string{"VBAT"}}},
		Protections:    []Protection{{Rail: "VBAT", Kind: ProtectionOVP}, {Rail: "3V3", Kind: ProtectionDischarge}},
		NetProperties: []NetProperty{
			{Net: "SYS_RESET_N", Property: PropResetPolarity, Value: "low"},
			{Net: "PCIE_TX0_P", Property: PropACCoupled},
			{Net: "BOOT_MODE0", Property: PropStrap, Value: "high"},
		},
		RailBudgets:  []RailBudget{{Rail: "3V3", Peak: 0.8}},
		MarginFactor: 1.2,
		Sequences: []Sequence{{
			Name:     "SoC power tree",
			Relation: SequenceEnableGated,
			Order:    []SequenceStage{{Rail: "VDD_CORE", Good: "PG_CORE"}, {Rail: "VDD_IO", Enable: "EN_IO"}},
		}},
	}
	rules := Compile(decl)
	if len(rules) == 0 {
		t.Fatal("Compile produced no rules for a full declaration")
	}
	for _, r := range rules {
		if !Emits(r.Name) {
			t.Errorf("Compile emitted %q but Emits(%q) = false — Emits must cover every compiler kind", r.Name, r.Name)
		}
	}
}
