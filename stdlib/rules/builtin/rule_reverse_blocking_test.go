package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// revDesign builds a connector-fed power path: J1 (connector) -> VIN -> [series part] -> VOUT -> U1's
// power-input pin. seriesKind names the part between the two nets ("" for a direct connection), and
// flip reverses the series part's pin order so a diode faces the other way.
func revDesign(seriesKind string, flip bool) *ir.Design {
	load := &ir.PartType{Name: "LOAD", Pins: []*ir.Pin{
		{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
	}}
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{load}}},
		Components: []*ir.Component{
			{RefDes: "J1", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U1", Sections: []*ir.ComponentSection{{PartRef: "LOAD", LibraryRef: "lib"}},
				Prov: &ir.Provenance{SourceFile: "t"}},
		},
	}
	if seriesKind == "" {
		d.Nets = []*ir.Net{{Name: "VIN", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
			{ComponentRef: "J1", PinRef: "1"}, {ComponentRef: "U1", PinRef: "1"},
		}}}
		return d
	}

	// The series part: a diode declares anode/cathode pins so orientation is derivable; anything else
	// just needs two terminals.
	part := &ir.PartType{Name: "SER"}
	if seriesKind == "diode" {
		part.Pins = []*ir.Pin{
			{Name: "A", Designator: "1"}, {Name: "K", Designator: "2"},
		}
	} else {
		part.Pins = []*ir.Pin{{Name: "1", Designator: "1"}, {Name: "2", Designator: "2"}}
	}
	d.Libraries[0].Parts = append(d.Libraries[0].Parts, part)

	ref := map[string]string{"diode": "D1", "transistor": "Q1", "resistor": "R1"}[seriesKind]
	d.Components = append(d.Components, &ir.Component{
		RefDes: ref, Sections: []*ir.ComponentSection{{PartRef: "SER", LibraryRef: "lib"}},
		Prov: &ir.Provenance{SourceFile: "t"},
	})

	inPin, outPin := "1", "2" // anode on VIN (source side) when not flipped
	if flip {
		inPin, outPin = "2", "1"
	}
	d.Nets = []*ir.Net{
		{Name: "VIN", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
			{ComponentRef: "J1", PinRef: "1"}, {ComponentRef: ref, PinRef: inPin},
		}},
		{Name: "VOUT", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
			{ComponentRef: ref, PinRef: outPin}, {ComponentRef: "U1", PinRef: "1"},
		}},
	}
	return d
}

func revFindings(d *ir.Design) []check.Finding {
	return reverseBlockingAbsent.Findings(check.NewModel(d))
}

// TestReverseBlockingFiresOnBarePath (WS3-094): a connector feeding a power input directly has nothing
// blocking reverse flow.
func TestReverseBlockingFiresOnBarePath(t *testing.T) {
	fs := revFindings(revDesign("", false))
	if len(fs) != 1 || check.EntityRef(fs[0].Subject) != "VIN" {
		t.Fatalf("want 1 finding on VIN, got %+v", fs)
	}
}

// TestReverseBlockingSilentWithCorrectDiode: a series diode with its anode on the source side is the
// blocking element the rule looks for.
func TestReverseBlockingSilentWithCorrectDiode(t *testing.T) {
	if fs := revFindings(revDesign("diode", false)); len(fs) != 0 {
		t.Errorf("anode-toward-source diode blocks reverse flow, want silence: %+v", fs)
	}
}

// TestReverseBlockingFiresOnBackwardsDiode is the assertion that makes orientation load-bearing. A
// diode fitted the other way round blocks the SUPPLY rather than the fault, so it must not satisfy the
// check — a rule that only asked "is a diode present" would pass this board.
func TestReverseBlockingFiresOnBackwardsDiode(t *testing.T) {
	if fs := revFindings(revDesign("diode", true)); len(fs) != 1 {
		t.Errorf("a reversed diode does not block reverse flow, want the finding: %+v", fs)
	}
}

// TestReverseBlockingReportsUnclassifiableTransistor: a P-FET ideal diode is a transistor plus a bias
// network that no netlist labels, and it is the correct modern answer to reverse protection, so a path
// crossing a transistor is unclassifiable rather than unprotected. Firing a DEFECT here would
// false-fail every ORing-FET design.
//
// This asserted SILENCE until agni issue 74 gave a rule somewhere to put "I could not decide". Silence
// was the wrong answer for a reason invisible at this layer: a review item bound to the rule read it
// as PASS, so the report claimed protection on a path nothing had verified. The finding must be
// Inconclusive, so it is neither a defect nor a pass.
func TestReverseBlockingReportsUnclassifiableTransistor(t *testing.T) {
	fs := revFindings(revDesign("transistor", false))
	if len(fs) != 1 {
		t.Fatalf("want 1 inconclusive finding for an unidentifiable transistor, got %d: %+v", len(fs), fs)
	}
	if !fs[0].Inconclusive {
		t.Error("an unidentifiable transistor is not a defect; the design may well be correct")
	}
	if !strings.Contains(fs[0].Message, "ideal_diode_controller") {
		t.Errorf("the message must say what would resolve it: %s", fs[0].Message)
	}
}

// TestReverseBlockingSilentOnIdentifiedController is the capability the class was added for: when a
// datasheet IDENTIFIES the part as an ideal-diode / ORing / power-mux controller, the path carries a
// directional element and the rule is genuinely silent.
//
// It is the other half of the test above, and the pair is the point. Before this, "verified protected"
// and "could not tell" were the same silence, so classification could not have changed any observable
// behaviour. Now they are distinguishable, which is what lets a bound review item read a real pass.
func TestReverseBlockingSilentOnIdentifiedController(t *testing.T) {
	// The controller is a SEPARATE part from the FET it drives, which is both the real topology and
	// what makes this test able to fail. Stamping the class onto the FET itself would stop it being
	// classified a transistor at all, so the path would go silent by falling through rather than by
	// the controller being credited, and deleting the credit branch would not fail this test.
	d := revDesign("transistor", false)
	d.Components = append(d.Components, &ir.Component{
		RefDes:        "U9",
		DeviceClasses: []string{string(check.ClassIdealDiodeController)},
		Prov:          &ir.Provenance{SourceFile: "t"},
	})
	d.Nets[0].Connections = append(d.Nets[0].Connections, &ir.Connection{ComponentRef: "U9", PinRef: "1"})

	if fs := revFindings(d); len(fs) != 0 {
		t.Errorf("an identified ideal-diode controller IS a directional element, want silence: %+v", fs)
	}
}

// TestReverseBlockingFiresThroughPassive: a resistor is not directional, so a path carrying only
// passives is still unblocked. Guards against the walk treating any crossed part as protection.
func TestReverseBlockingFiresThroughPassive(t *testing.T) {
	if fs := revFindings(revDesign("resistor", false)); len(fs) != 1 {
		t.Errorf("a resistor blocks nothing directionally, want the finding: %+v", fs)
	}
}

// TestReverseBlockingIgnoresConnectorlessPath: the rule is about what enters the board, so an internal
// rail feeding a power input is not its business.
func TestReverseBlockingIgnoresConnectorlessPath(t *testing.T) {
	d := revDesign("", false)
	d.Components[0].RefDes = "U9" // no longer a connector
	d.Nets[0].Connections[0].ComponentRef = "U9"
	if fs := revFindings(d); len(fs) != 0 {
		t.Errorf("no connector on the net, want silence: %+v", fs)
	}
}

// TestReverseBlockingTransistorBeatsBackwardsDiode is what makes the transistor branch load-bearing
// rather than decorative. Silence on a transistor is otherwise just the default — a transistor is not
// a diode, so it never sets the finding — and a test with only a transistor passes whether the guard
// exists or not (verified by deleting it).
//
// The guard earns its place when BOTH bridge the neighbourhood: a backwards diode on one leg and a
// FET on another. The FET may be an ideal diode protecting the board, so the honest report is silence,
// not a finding derived from the leg we happened to understand. Unclassifiable beats partially
// classified.
func TestReverseBlockingTransistorBeatsBackwardsDiode(t *testing.T) {
	d := revDesign("diode", true) // J1 -> VIN -> D1 (backwards) -> VOUT -> U1 power input
	// A second leg off VIN: a transistor to its own load.
	fet := &ir.PartType{Name: "FET", Pins: []*ir.Pin{{Name: "1", Designator: "1"}, {Name: "2", Designator: "2"}}}
	load2 := &ir.PartType{Name: "LOAD2", Pins: []*ir.Pin{
		{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
	}}
	d.Libraries[0].Parts = append(d.Libraries[0].Parts, fet, load2)
	d.Components = append(d.Components,
		&ir.Component{RefDes: "Q1", Sections: []*ir.ComponentSection{{PartRef: "FET", LibraryRef: "lib"}},
			Prov: &ir.Provenance{SourceFile: "t"}},
		&ir.Component{RefDes: "U2", Sections: []*ir.ComponentSection{{PartRef: "LOAD2", LibraryRef: "lib"}},
			Prov: &ir.Provenance{SourceFile: "t"}})
	d.Nets[0].Connections = append(d.Nets[0].Connections, &ir.Connection{ComponentRef: "Q1", PinRef: "1"})
	d.Nets = append(d.Nets, &ir.Net{
		Name: "VOUT_B", Prov: &ir.Provenance{SourceFile: "t"},
		Connections: []*ir.Connection{{ComponentRef: "Q1", PinRef: "2"}, {ComponentRef: "U2", PinRef: "1"}},
	})

	fs := revFindings(d)
	if len(fs) != 1 || !fs[0].Inconclusive {
		t.Fatalf("a transistor on any leg makes the path unclassifiable, want one inconclusive finding: %+v", fs)
	}
}

// revShuntClampDesign models the false fire in issue 63's first mechanism: a high-side switch output
// drives a load connector and carries a FREEWHEEL diode, anode on ground, cathode on the switched
// output. That diode is a shunt clamp beside the path, not a series element in it, so it says nothing
// about reverse blocking. U1's ground pin is declared power_in, which is ordinary, and is what made the
// far side of the clamp look like a load.
func revShuntClampDesign() *ir.Design {
	load := &ir.PartType{Name: "LOAD", Pins: []*ir.Pin{
		{Name: "GND", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
	}}
	diode := &ir.PartType{Name: "SER", Pins: []*ir.Pin{{Name: "A", Designator: "1"}, {Name: "K", Designator: "2"}}}
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{load, diode}}},
		Components: []*ir.Component{
			{RefDes: "J2", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U1", Sections: []*ir.ComponentSection{{PartRef: "LOAD", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "D1", Sections: []*ir.ComponentSection{{PartRef: "SER", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			{Name: "SW_OUT", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
				{ComponentRef: "J2", PinRef: "1"}, {ComponentRef: "D1", PinRef: "2"}, // cathode
			}},
			{Name: "GND", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
				{ComponentRef: "D1", PinRef: "1"}, {ComponentRef: "U1", PinRef: "1"}, // anode, and a power_in ground pin
			}},
		},
	}
}

// TestReverseBlockingIgnoresShuntClamp (issue 63): a diode whose far terminal lands on GROUND is a
// shunt clamp, and the rule must not evaluate it as a reverse-blocking candidate at all.
//
// Before the fix the orientation test asked only "is the anode on the near net", so a ground-anode
// freewheel diode read as "fitted backwards" and the output was reported unblocked. That produced 20 of
// the 34 false FAILs observed on a real board, every one of them on a correctly-protected switch output.
func TestReverseBlockingIgnoresShuntClamp(t *testing.T) {
	if fs := revFindings(revShuntClampDesign()); len(fs) != 0 {
		t.Errorf("a freewheel diode to ground is a shunt, not a backwards series blocker, want silence: %+v", fs)
	}
}

// revMultiTerminalFETDesign models issue 63's second mechanism: a real 3-terminal MOSFET (gate, source,
// drain) on the path, alongside a backwards diode on the same node.
//
// The transistor guard reached the part only through farNet, which returns nil for anything touching
// more than one net outside the reach set. A 3-terminal FET touches two, so the guard was skipped and
// the diode drove the finding. A 2-terminal stand-in cannot catch this, which is why the existing
// TestReverseBlockingTransistorBeatsBackwardsDiode passed throughout.
func revMultiTerminalFETDesign() *ir.Design {
	load := &ir.PartType{Name: "LOAD", Pins: []*ir.Pin{
		{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
	}}
	fet := &ir.PartType{Name: "FET", Pins: []*ir.Pin{
		{Name: "G", Designator: "1"}, {Name: "S", Designator: "2"}, {Name: "D", Designator: "3"},
	}}
	diode := &ir.PartType{Name: "SER", Pins: []*ir.Pin{{Name: "A", Designator: "1"}, {Name: "K", Designator: "2"}}}
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{load, fet, diode}}},
		Components: []*ir.Component{
			{RefDes: "J1", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U1", Sections: []*ir.ComponentSection{{PartRef: "LOAD", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "Q1", Sections: []*ir.ComponentSection{{PartRef: "FET", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "D1", Sections: []*ir.ComponentSection{{PartRef: "SER", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			{Name: "VIN", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
				{ComponentRef: "J1", PinRef: "1"},
				{ComponentRef: "Q1", PinRef: "2"}, // source
				{ComponentRef: "D1", PinRef: "2"}, // cathode on the near side: fitted backwards
			}},
			{Name: "VOUT", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
				{ComponentRef: "Q1", PinRef: "3"}, // drain
				{ComponentRef: "D1", PinRef: "1"}, // anode
				{ComponentRef: "U1", PinRef: "1"},
			}},
			{Name: "GATE_CTRL", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{
				{ComponentRef: "Q1", PinRef: "1"},
			}},
		},
	}
}

// TestReverseBlockingGuardCountsMultiTerminalTransistor (issue 63): a transistor ANYWHERE on the
// neighbourhood makes the arrangement unclassifiable, however many terminals it has.
//
// This is the 14-net half of the real-board false fires. The rule's whole safety posture is that it
// stays silent on a possible ORing FET or ideal diode rather than false-failing a correct design, and
// a guard that only recognises 2-terminal parts does not deliver it for any real MOSFET.
func TestReverseBlockingGuardCountsMultiTerminalTransistor(t *testing.T) {
	fs := revFindings(revMultiTerminalFETDesign())
	if len(fs) != 1 || !fs[0].Inconclusive {
		t.Fatalf("a 3-terminal FET on the path is unclassifiable, want one inconclusive finding: %+v", fs)
	}
}
