package intent

import (
	"strings"
	"testing"

	"github.com/panyam/agni/check"
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
	if fs[0].Rule != RuleModuleMissing || fs[0].Kind != check.KindComponent || fs[0].Subject != "CAN transceiver" {
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

func TestModuleMatchesByMPN(t *testing.T) {
	decl := declOf(t, "name: I\nmodules:\n  - {name: flash, mpn: W25Q128}")
	// MPN resolves only on a params-built model; without one, ComponentMPN is empty and the module is
	// unmatched (fires). With the params-built model (empty specs suffices to build the mpn map from
	// attributes), it matches.
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U2", Attributes: map[string]string{"MPN": "W25Q128"}},
	}}
	if fs := check.Run(check.NewModel(d), Compile(decl)); len(fs) != 1 {
		t.Errorf("MPN module should be unmatched without a params model, got %+v", fs)
	}
	pm := check.NewModelWithParams(d, nil, nil)
	if fs := check.Run(pm, Compile(decl)); len(fs) != 0 {
		t.Errorf("MPN module should match on a params-built model, got %+v", fs)
	}
}
