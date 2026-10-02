package intent

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// declOf is a small helper, since parse never fails on these well-formed literals in-test.
func declOf(t *testing.T, yaml string) Declaration {
	t.Helper()
	d, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return d
}

func TestModuleMissingFiresOnAbsentModule(t *testing.T) {
	decl := declOf(t, `
name: I
intent:
  modules:
  - {name: SoC, class: soc}
  - {name: CAN transceiver, class: can_transceiver}
`)
	// The design has an SoC but NO CAN transceiver. The declared expectation set comes from the
	// declaration, not the netlist, so the absent module must fail.
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", DeviceClasses: classify.Tags("soc")},
		{RefDes: "R1", DeviceClasses: classify.Tags("resistor")},
	}}
	fs := check.RunBackground(check.NewModel(d), Compile(decl))
	if len(fs) != 1 {
		t.Fatalf("want exactly one finding (the absent CAN transceiver), got %d: %+v", len(fs), fs)
	}
	if fs[0].Rule != RuleModuleMissing || fs[0].Subject.Kind != check.KindComponent || check.EntityRef(fs[0].Subject) != "CAN transceiver" {
		t.Errorf("finding shape wrong: %+v", fs[0])
	}
	if !strings.Contains(fs[0].Message, "can_transceiver") {
		t.Errorf("message should name the criterion, got %q", fs[0].Message)
	}
}

func TestModulePresentPasses(t *testing.T) {
	decl := declOf(t, "name: I\nintent:\n  modules:\n  - {name: SoC, class: soc}\n")
	// The classifier tags a TVS as both tvs and diode; HasClass matches a family parent, so a module
	// declared as "diode" would match a tvs. Here the exact class matches directly.
	d := &ir.Design{Components: []*ir.Component{{RefDes: "U1", DeviceClasses: classify.Tags("soc")}}}
	if fs := check.RunBackground(check.NewModel(d), Compile(decl)); len(fs) != 0 {
		t.Errorf("a present module must not fire, got %+v", fs)
	}
}

func TestModuleMatchesByFamilyTag(t *testing.T) {
	decl := declOf(t, "name: I\nintent:\n  modules:\n  - {name: any diode, class: diode}\n")
	// A component classed tvs carries the diode family tag, so a diode-declared module matches it.
	d := &ir.Design{Components: []*ir.Component{{RefDes: "D1", DeviceClasses: classify.Tags("tvs", "diode")}}}
	if fs := check.RunBackground(check.NewModel(d), Compile(decl)); len(fs) != 0 {
		t.Errorf("family-tag match should pass, got %+v", fs)
	}
}

func TestModuleCountFiresOnTooFew(t *testing.T) {
	decl := declOf(t, "name: I\nintent:\n  modules:\n  - {name: CAN, class: can, count: 2}\n")
	// One CAN present and two declared, so module-missing passes (>=1 present) and module-count fires.
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", DeviceClasses: classify.Tags("can")},
		{RefDes: "R1", DeviceClasses: classify.Tags("resistor")},
	}}
	fs := check.RunBackground(check.NewModel(d), Compile(decl))
	if len(fs) != 1 {
		t.Fatalf("want exactly one finding (the count mismatch), got %d: %+v", len(fs), fs)
	}
	if fs[0].Rule != RuleModuleCount || check.EntityRef(fs[0].Subject) != "CAN" {
		t.Errorf("finding shape wrong: %+v", fs[0])
	}
	if !strings.Contains(fs[0].Message, "expects 2, found 1") {
		t.Errorf("message should state expected vs found, got %q", fs[0].Message)
	}
}

func TestModuleCountFiresOnTooMany(t *testing.T) {
	decl := declOf(t, "name: I\nintent:\n  modules:\n  - {name: CAN, class: can, count: 1}\n")
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", DeviceClasses: classify.Tags("can")},
		{RefDes: "U2", DeviceClasses: classify.Tags("can")},
	}}
	fs := check.RunBackground(check.NewModel(d), Compile(decl))
	if len(fs) != 1 || fs[0].Rule != RuleModuleCount {
		t.Fatalf("want one module-count finding, got %+v", fs)
	}
	if !strings.Contains(fs[0].Message, "expects 1, found 2") {
		t.Errorf("message should state expected vs found, got %q", fs[0].Message)
	}
}

func TestModuleCountPassesOnExact(t *testing.T) {
	decl := declOf(t, "name: I\nintent:\n  modules:\n  - {name: CAN, class: can, count: 2}\n")
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", DeviceClasses: classify.Tags("can")},
		{RefDes: "U2", DeviceClasses: classify.Tags("can")},
	}}
	if fs := check.RunBackground(check.NewModel(d), Compile(decl)); len(fs) != 0 {
		t.Errorf("exact count must not fire, got %+v", fs)
	}
}

func TestModuleCountUnspecifiedEmitsNoRule(t *testing.T) {
	// A declaration with modules but no counts must compile to NO count rule (empty-set-is-silent), so
	// an item bound to intent/module-count reads not-automated rather than silently passing.
	decl := declOf(t, "name: I\nintent:\n  modules:\n  - {name: SoC, class: soc}\n")
	for _, r := range Compile(decl) {
		if r.Name == RuleModuleCount {
			t.Fatalf("no count declared, but a module-count rule was emitted")
		}
	}
}

func TestNegativeCountIsALoadError(t *testing.T) {
	if _, err := Parse([]byte("name: I\nintent:\n  modules:\n  - {name: CAN, class: can, count: -1}\n")); err == nil {
		t.Fatal("a negative count should be a load error")
	}
}

func TestModuleMatchesByMPN(t *testing.T) {
	decl := declOf(t, "name: I\nintent:\n  modules:\n  - {name: flash, mpn: W25Q128}\n")
	// Every model joins the design's MPNs, with or without a datasheet provider (agni issue 748), so
	// the module matches on a plain model. A part carrying a DIFFERENT MPN leaves it unmatched (fires).
	other := &ir.Design{Components: []*ir.Component{
		{RefDes: "U2", Mpn: "MX25L128"},
	}}
	if fs := check.RunBackground(check.NewModel(other), Compile(decl)); len(fs) != 1 {
		t.Errorf("MPN module should be unmatched when no part carries its MPN, got %+v", fs)
	}
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U2", Mpn: "W25Q128"},
	}}
	if fs := check.RunBackground(check.NewModel(d), Compile(decl)); len(fs) != 0 {
		t.Errorf("MPN module should match on a model built with no provider, got %+v", fs)
	}
}
