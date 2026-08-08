package intent

import "testing"

// TestEmits covers the vocabulary Emits recognizes and, load-bearingly, the names it must REJECT: a
// not-yet-shipped intent rule name (the WS3-098 pre-bind case) and a non-intent name.
func TestEmits(t *testing.T) {
	known := []string{
		"module-missing", "module-count", "voltage-domain-mismatch",
		"intent/module-missing",       // composed catalog name accepted too
		"subsystem-power-tree", "intent/subsystem-clock",
		"protection-ovp", "protection-discharge", "intent/protection-discharge",
	}
	for _, n := range known {
		if !Emits(n) {
			t.Errorf("Emits(%q) = false, want true", n)
		}
	}
	unknown := []string{
		"power-sequence", "intent/power-sequence", // a future intent rule kind, not yet shipped
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
