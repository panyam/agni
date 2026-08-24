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

// TestAvailableNetClassCapability (WS3-105): a rule SCOPED by net class selects nothing where the
// design assigns no classes, so it would report clean without ever checking anything. The gate is
// content-derived, not format-derived — a KiCad project that declares no classes is as unanswerable
// as an EDIF netlist that cannot declare any, so both must read not-applicable.
func TestAvailableNetClassCapability(t *testing.T) {
	rule := &Rule{Reads: []string{"net.netclass"}, RequiresCapability: []Capability{CapNetClass}}

	classless := NewModel(&ir.Design{SourceFormat: "kicad-sch", Nets: []*ir.Net{{Name: "USB_D+"}}})
	if ok, reason := Available(rule, classless); ok || reason == "" {
		t.Errorf("netclass rule on a class-free KiCad design: got (%v, %q), want (false, non-empty)", ok, reason)
	}
	edif := NewModel(&ir.Design{SourceFormat: "edif-2.0.0", Nets: []*ir.Net{{Name: "USB_D+"}}})
	if ok, reason := Available(rule, edif); ok || reason == "" {
		t.Errorf("netclass rule on EDIF: got (%v, %q), want (false, non-empty)", ok, reason)
	}

	classed := NewModel(&ir.Design{SourceFormat: "kicad-sch", Nets: []*ir.Net{{Name: "USB_D+", NetClasses: []string{"HighSpeed"}}}})
	if ok, _ := Available(rule, classed); !ok {
		t.Error("netclass rule on a design with classes: want available")
	}
	if ok, _ := Available(rule, nil); !ok {
		t.Error("netclass rule at catalog listing (m==nil): want available")
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
		Eval: FailuresOnly(func(Model) []Finding { return nil }),
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

// TestCatalogWithKeepsWhatTheBaseCarried is the property WS3-107 turned on: extending a catalog must
// ADD to it. A *Catalog holds composed rules rather than its inputs, so a caller holding one it did
// not build cannot rebuild it — and the thing it did instead, recomposing from the standard sources,
// silently discarded whatever else the base carried.
func TestCatalogWithKeepsWhatTheBaseCarried(t *testing.T) {
	base, err := NewCatalog(NewSource("alpha", []*Rule{{Name: "a1"}}), NewSource("beta", []*Rule{{Name: "b1"}}))
	if err != nil {
		t.Fatal(err)
	}
	got, err := base.With(NewSource("gamma", []*Rule{{Name: "g1"}}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"alpha/a1", "beta/b1", "gamma/g1"} {
		if got.Lookup(want) == nil {
			t.Errorf("%s missing after With; the base was dropped", want)
		}
	}
	if base.Lookup("gamma/g1") != nil {
		t.Error("With mutated the base catalog")
	}
	if n := len(base.Rules()); n != 2 {
		t.Errorf("base rule count changed to %d", n)
	}
}

// TestCatalogWithNamespacesOnlyTheExtras pins that base rules cross over verbatim. They are already
// namespaced, so re-composing them would be rejected for containing the separator; only the extras go
// through composition.
func TestCatalogWithNamespacesOnlyTheExtras(t *testing.T) {
	base, err := NewCatalog(NewSource("alpha", []*Rule{{Name: "a1"}}))
	if err != nil {
		t.Fatal(err)
	}
	got, err := base.With(NewSource("beta", []*Rule{{Name: "b1"}}))
	if err != nil {
		t.Fatal(err)
	}
	if r := got.Lookup("alpha/a1"); r == nil {
		t.Fatal("base rule lost")
	} else if r.Tags[KeySource] != "alpha" {
		t.Errorf("base rule source tag = %q, want alpha", r.Tags[KeySource])
	}
	if got.Lookup("alpha/alpha/a1") != nil {
		t.Error("a base rule was namespaced a second time")
	}
	if r := got.Lookup("beta/b1"); r == nil || r.Tags[KeySource] != "beta" {
		t.Errorf("extra rule not composed: %v", r)
	}
}

// TestCatalogWithRejectsACollision pins that an extension cannot shadow what the base carried, and
// that re-adding a source the base already has is the same duplicate error a single composition would
// give rather than a confusing per-rule collision.
func TestCatalogWithRejectsACollision(t *testing.T) {
	base, err := NewCatalog(NewSource("alpha", []*Rule{{Name: "a1"}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := base.With(NewSource("alpha", []*Rule{{Name: "a2"}})); err == nil {
		t.Error("re-adding an existing source should be a duplicate-source error")
	}
	if _, err := base.With(NewSource("beta", []*Rule{{Name: "b/1"}})); err == nil {
		t.Error("a rule name containing the namespace separator should be rejected")
	}
}

// TestAvailableRefDesCollisionsCapability (agni issue 309): a rule whose entire subject is a reader
// diagnostic has nothing to report when the reader never computed it, and "nothing to report" is
// exactly what a clean design looks like. So the gate reads the reader's own declaration rather than
// the emptiness of the list.
//
// The two designs below are the whole bug: identical empty collision lists, opposite meanings. Before
// the declaration existed, both read as a pass.
func TestAvailableRefDesCollisionsCapability(t *testing.T) {
	rule := &Rule{Reads: []string{"ref_des_collision"}, RequiresCapability: []Capability{CapRefDesCollisions}}

	looked := NewModel(&ir.Design{SourceFormat: "kicad-sch",
		InputDiagnostics: &ir.InputDiagnostics{Supplied: []string{"ref_des_collisions"}}})
	if ok, _ := Available(rule, looked); !ok {
		t.Error("a reader that computed the diagnostic and found none: want available (it is a real clean pass)")
	}

	didNot := NewModel(&ir.Design{SourceFormat: "edif-2.0.0", InputDiagnostics: &ir.InputDiagnostics{}})
	ok, reason := Available(rule, didNot)
	if ok {
		t.Error("a reader that cannot detect collisions: want not-applicable, not a silent pass")
	}
	if reason == "" {
		t.Error("not-applicable with no reason tells a reviewer nothing")
	}

	// A design with no InputDiagnostics at all is the same case: nobody looked.
	if ok, _ := Available(rule, NewModel(&ir.Design{SourceFormat: "edif-2.0.0"})); ok {
		t.Error("no diagnostics block at all: want not-applicable")
	}

	// The catalog listing (m == nil) keeps the rule available, mirroring the other capabilities:
	// the engine can run it, on a design that supplies the diagnostic.
	if ok, _ := Available(rule, nil); !ok {
		t.Error("capability rule at catalog listing (m==nil): want available")
	}
}
