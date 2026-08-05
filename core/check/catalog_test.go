package check

import (
	"testing"

	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestAvailableCapability gates a capability-requiring rule to not-applicable where the source format
// cannot supply the capability, and leaves it available where it can (WS3-096). An EDIF netlist types
// no power outputs and carries no no-connect channel; a KiCad schematic supplies both. m == nil (the
// catalog listing) leaves the rule available, mirroring the board branch.
func TestAvailableCapability(t *testing.T) {
	powerRule := &Rule{Reads: []string{"on_net"}, RequiresCapability: []Capability{CapTypesPowerOut}}
	ncRule := &Rule{Reads: []string{"net.names"}, RequiresCapability: []Capability{CapNoConnectChannel}}

	edif := NewModel(&ir.Design{SourceFormat: "edif-2.0.0",
		Nets: []*ir.Net{{Name: "N", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}}}})
	if ok, reason := Available(powerRule, edif); ok || reason == "" {
		t.Errorf("power rule on EDIF: got (%v, %q), want (false, non-empty)", ok, reason)
	}
	if ok, reason := Available(ncRule, edif); ok || reason == "" {
		t.Errorf("nc rule on EDIF: got (%v, %q), want (false, non-empty)", ok, reason)
	}

	// KiCad source types power outputs; a nc-marker net name gives the no-connect channel.
	kicad := NewModel(&ir.Design{SourceFormat: "kicad-sch",
		Nets: []*ir.Net{{Name: "unconnected-(U1-PAD1)", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}}}})
	if ok, _ := Available(powerRule, kicad); !ok {
		t.Error("power rule on KiCad: want available")
	}
	if ok, _ := Available(ncRule, kicad); !ok {
		t.Error("nc rule on KiCad: want available")
	}
	if ok, _ := Available(powerRule, nil); !ok {
		t.Error("capability rule at catalog listing (m==nil): want available")
	}
}

func TestAvailableFromReads(t *testing.T) {
	if ok, reason := Available(&Rule{Reads: []string{"net.pin_count", "on_net"}}, nil); !ok || reason != "" {
		t.Errorf("topology-only rule: got (%v, %q), want (true, \"\")", ok, reason)
	}
	if ok, reason := Available(&Rule{Reads: []string{"net.names", "param(mpn, max_voltage)"}}, nil); ok || reason == "" {
		t.Errorf("datasheet-reading rule: got (%v, %q), want (false, non-empty)", ok, reason)
	}
	// A datasheet rule IS applicable once a params tier is attached: the earlier gate returned
	// not-applicable unconditionally, so a seeded ask could never pass/fail in a review even with
	// --params. m == nil (catalog listing) and a params-less model still gate.
	pr := &Rule{Reads: []string{"param.supply_abs_max"}}
	seeded := NewModelWithParams(supplyDesign("+5V", false, "ACME-33"), nil, param.ParamSet{})
	if ok, _ := Available(pr, seeded); !ok {
		t.Error("datasheet rule with a params tier attached: want available")
	}
	if ok, _ := Available(pr, NewModel(supplyDesign("+5V", false, "ACME-33"))); ok {
		t.Error("datasheet rule on a params-less model: want not-available")
	}
}

// testRule builds a minimal named rule for source-composition tests.
func testRule(name string) *Rule {
	return &Rule{
		Name: name, Severity: "info", Summary: "t", Reads: []string{"net.names"},
		Tags: map[string]string{KeyCategory: CategoryNaming},
		Eval: func(Model) []Finding { return nil },
	}
}

func TestCatalogRejections(t *testing.T) {
	cases := []struct {
		name    string
		sources []RuleSource
	}{
		{"duplicate source name", []RuleSource{NewSource("tesla", nil), NewSource("tesla", nil)}},
		{"second anonymous source", []RuleSource{Builtins, NewSource("", nil)}},
		{"bad source name", []RuleSource{NewSource("Tesla Rules", nil)}},
		{"separator in rule name", []RuleSource{NewSource("tesla", []*Rule{testRule("a/b")})}},
		{"duplicate rule in source", []RuleSource{NewSource("tesla", []*Rule{testRule("x"), testRule("x")})}},
	}
	for _, tc := range cases {
		if _, err := NewCatalog(tc.sources...); err == nil {
			t.Errorf("%s: NewCatalog accepted an invalid composition", tc.name)
		}
	}
}
