package intent

import (
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// ovpDesign builds a design where net `rail` carries the component D1 of class `d1class`, plus a GND net.
func protDesign(rail, d1class string) *ir.Design {
	return &ir.Design{
		Components: []*ir.Component{{RefDes: "D1", DeviceClasses: classify.Tags(d1class)}},
		Nets: []*ir.Net{
			{Name: rail, Connections: []*ir.Connection{{ComponentRef: "D1", PinRef: "1"}}},
			{Name: "GND", Connections: []*ir.Connection{{ComponentRef: "D1", PinRef: "2"}}},
		},
	}
}

func TestOVPPassesWhenTVSOnRail(t *testing.T) {
	decl := declOf(t, "name: I\nintent:\n  nets:\n    VBATT01:\n      protect: [ovp]\n")
	if fs := check.RunBackground(check.NewModel(protDesign("VBATT01", "tvs")), Compile(decl)); len(fs) != 0 {
		t.Errorf("a TVS on the declared rail should pass, got %+v", fs)
	}
	// zener also clamps.
	if fs := check.RunBackground(check.NewModel(protDesign("VBATT01", "zener")), Compile(decl)); len(fs) != 0 {
		t.Errorf("a zener on the declared rail should pass, got %+v", fs)
	}
}

func TestOVPFiresWhenNoClampOnRail(t *testing.T) {
	decl := declOf(t, "name: I\nintent:\n  nets:\n    VBATT01:\n      protect: [ovp]\n")
	// A resistor on the rail is not a clamp -> fire.
	fs := check.RunBackground(check.NewModel(protDesign("VBATT01", "resistor")), Compile(decl))
	if len(fs) != 1 || fs[0].Rule != "protection-ovp" || check.EntityRef(fs[0].Subject) != "VBATT01" {
		t.Fatalf("want one ovp finding on VBATT01, got %+v", fs)
	}
}

func TestOVPNetScoped(t *testing.T) {
	// The TVS is on a DIFFERENT net than the declared rail -> the declared rail is still unprotected.
	decl := declOf(t, "name: I\nintent:\n  nets:\n    VBATT01:\n      protect: [ovp]\n")
	d := protDesign("SOME_OTHER_NET", "tvs")
	d.Nets = append(d.Nets, &ir.Net{Name: "VBATT01"}) // declared rail exists but has no TVS
	if fs := check.RunBackground(check.NewModel(d), Compile(decl)); len(fs) != 1 {
		t.Errorf("a TVS on another net must not satisfy the declared rail, got %+v", fs)
	}
}

func TestDischargePassesWithBleeder(t *testing.T) {
	// A resistor with one pin on the rail and one on GND is a bleeder.
	decl := declOf(t, "name: I\nintent:\n  nets:\n    5V0:\n      protect: [discharge]\n")
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "R1", DeviceClasses: classify.Tags("resistor")}},
		Nets: []*ir.Net{
			{Name: "5V0", Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "1"}}},
			{Name: "GND", Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "2"}}},
		},
	}
	if fs := check.RunBackground(check.NewModel(d), Compile(decl)); len(fs) != 0 {
		t.Errorf("a bleeder resistor rail->GND should pass, got %+v", fs)
	}
}

func TestDischargeFiresWithoutBleeder(t *testing.T) {
	decl := declOf(t, "name: I\nintent:\n  nets:\n    5V0:\n      protect: [discharge]\n")
	// R1 is on the rail but its other pin is a signal, not ground -> not a bleeder.
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "R1", DeviceClasses: classify.Tags("resistor")}},
		Nets: []*ir.Net{
			{Name: "5V0", Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "1"}}},
			{Name: "SIG", Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "2"}}},
		},
	}
	if fs := check.RunBackground(check.NewModel(d), Compile(decl)); len(fs) != 1 || fs[0].Rule != "protection-discharge" {
		t.Fatalf("want one discharge finding (no rail->GND resistor), got %+v", fs)
	}
}

func TestProtectionsCompileToPerKindRules(t *testing.T) {
	decl := declOf(t, "name: I\nintent:\n  nets:\n    A:\n      protect: [ovp]\n    B:\n      protect: [discharge]\n    C:\n      protect: [ovp]\n")
	names := map[string]bool{}
	for _, r := range Compile(decl) {
		names[r.Name] = true
	}
	if !names["protection-ovp"] || !names["protection-discharge"] {
		t.Errorf("want one rule per kind, got %v", names)
	}
	if len(Compile(decl)) != 2 {
		t.Errorf("two kinds -> two rules, got %d", len(Compile(decl)))
	}
}

func TestParseRejectsProtections(t *testing.T) {
	for label, doc := range map[string]string{
		"no rail":      "name: N\nintent:\n  nets:\n    null:\n      protect: [ovp]\n",
		"unknown kind": "name: N\nintent:\n  nets:\n    X:\n      protect: [crowbar]\n",
		"empty kind":   "name: N\nintent:\n  nets:\n    X:\n      protect: [null]\n",
	} {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: expected a validation error, got nil", label)
		}
	}
}
