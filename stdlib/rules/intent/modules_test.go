package intent

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// declOf is a small helper: parse never fails on these well-formed literals in-test.
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
modules:
  - {name: SoC, class: soc}
  - {name: CAN transceiver, class: can_transceiver}
`)
	// The design has an SoC but NO CAN transceiver: the declared expectation set comes from the
	// declaration, not the netlist, so the absent module must fail.
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", DeviceClasses: []string{"soc"}},
		{RefDes: "R1", DeviceClasses: []string{"resistor"}},
	}}
	fs := check.Run(check.NewModel(d), Compile(decl))
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
	decl := declOf(t, "name: I\nmodules:\n  - {name: SoC, class: soc}")
	// The classifier tags a TVS as both tvs and diode; HasClass matches a family parent, so a module
	// declared as "diode" would match a tvs. Here the exact class matches directly.
	d := &ir.Design{Components: []*ir.Component{{RefDes: "U1", DeviceClasses: []string{"soc"}}}}
	if fs := check.Run(check.NewModel(d), Compile(decl)); len(fs) != 0 {
		t.Errorf("a present module must not fire, got %+v", fs)
	}
}

func TestModuleMatchesByFamilyTag(t *testing.T) {
	decl := declOf(t, "name: I\nmodules:\n  - {name: any diode, class: diode}")
	// A component classed tvs carries the diode family tag, so a diode-declared module matches it.
	d := &ir.Design{Components: []*ir.Component{{RefDes: "D1", DeviceClasses: []string{"tvs", "diode"}}}}
	if fs := check.Run(check.NewModel(d), Compile(decl)); len(fs) != 0 {
		t.Errorf("family-tag match should pass, got %+v", fs)
	}
}

func TestModuleCountFiresOnTooFew(t *testing.T) {
	decl := declOf(t, "name: I\nmodules:\n  - {name: CAN, class: can, count: 2}")
	// One CAN present, two declared: module-missing passes (>=1 present), module-count fires.
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", DeviceClasses: []string{"can"}},
		{RefDes: "R1", DeviceClasses: []string{"resistor"}},
	}}
	fs := check.Run(check.NewModel(d), Compile(decl))
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
	decl := declOf(t, "name: I\nmodules:\n  - {name: CAN, class: can, count: 1}")
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", DeviceClasses: []string{"can"}},
		{RefDes: "U2", DeviceClasses: []string{"can"}},
	}}
	fs := check.Run(check.NewModel(d), Compile(decl))
	if len(fs) != 1 || fs[0].Rule != RuleModuleCount {
		t.Fatalf("want one module-count finding, got %+v", fs)
	}
	if !strings.Contains(fs[0].Message, "expects 1, found 2") {
		t.Errorf("message should state expected vs found, got %q", fs[0].Message)
	}
}

func TestModuleCountPassesOnExact(t *testing.T) {
	decl := declOf(t, "name: I\nmodules:\n  - {name: CAN, class: can, count: 2}")
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", DeviceClasses: []string{"can"}},
		{RefDes: "U2", DeviceClasses: []string{"can"}},
	}}
	if fs := check.Run(check.NewModel(d), Compile(decl)); len(fs) != 0 {
		t.Errorf("exact count must not fire, got %+v", fs)
	}
}

func TestModuleCountUnspecifiedEmitsNoRule(t *testing.T) {
	// A declaration with modules but no counts must compile to NO count rule (empty-set-is-silent), so
	// an item bound to intent/module-count reads not-automated rather than silently passing.
	decl := declOf(t, "name: I\nmodules:\n  - {name: SoC, class: soc}")
	for _, r := range Compile(decl) {
		if r.Name == RuleModuleCount {
			t.Fatalf("no count declared, but a module-count rule was emitted")
		}
	}
}

func TestNegativeCountIsALoadError(t *testing.T) {
	if _, err := Parse([]byte("name: I\nmodules:\n  - {name: CAN, class: can, count: -1}")); err == nil {
		t.Fatal("a negative count should be a load error")
	}
}

func TestModuleMatchesByMPN(t *testing.T) {
	decl := declOf(t, "name: I\nmodules:\n  - {name: flash, mpn: W25Q128}")
	// MPN resolves only on a params-built model; without one, ComponentMPN is empty and the module is
	// unmatched (fires). With the params-built model (empty specs suffices to build the mpn map from
	// attributes), it matches.
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U2", Mpn: "W25Q128"},
	}}
	if fs := check.Run(check.NewModel(d), Compile(decl)); len(fs) != 1 {
		t.Errorf("MPN module should be unmatched without a params model, got %+v", fs)
	}
	pm := check.NewModelWithParams(d, nil, nil)
	if fs := check.Run(pm, Compile(decl)); len(fs) != 0 {
		t.Errorf("MPN module should match on a params-built model, got %+v", fs)
	}
}
