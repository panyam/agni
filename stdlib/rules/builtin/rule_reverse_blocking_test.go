package builtin

import (
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

func revFindings(d *ir.Design) []check.Finding { return reverseBlockingAbsent.Eval(check.NewModel(d)) }

// TestReverseBlockingFiresOnBarePath (WS3-094): a connector feeding a power input directly has nothing
// blocking reverse flow.
func TestReverseBlockingFiresOnBarePath(t *testing.T) {
	fs := revFindings(revDesign("", false))
	if len(fs) != 1 || fs[0].Subject != "VIN" {
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

// TestReverseBlockingSilentOnTransistorPath is the option-A guard and the reason this rule is safe to
// ship. A P-FET ideal diode is a transistor plus a bias network that no netlist labels, and it is the
// correct modern answer to reverse protection — so a path crossing a transistor is unclassifiable, not
// unprotected. Firing here would false-fail every ORing-FET design.
func TestReverseBlockingSilentOnTransistorPath(t *testing.T) {
	if fs := revFindings(revDesign("transistor", false)); len(fs) != 0 {
		t.Errorf("a transistor on the path is unclassifiable, want silence: %+v", fs)
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

	if fs := revFindings(d); len(fs) != 0 {
		t.Errorf("a transistor on any leg makes the path unclassifiable, want silence: %+v", fs)
	}
}
