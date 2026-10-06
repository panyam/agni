package builtin

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// The supply rules read a power_in pin as a load, and vendor symbols type pins power_in that draw no
// supply (agni issue 935). Each case below pairs the shape the rules now leave alone with the shape
// that must still fail, because the second is what the rules exist for.

// supplyPin is one pin of a test part: its name, designator and electrical direction.
type supplyPin struct {
	name, des string
	dir       ir.PinDirection
}

const (
	pwrIn  = ir.PinDirection_PIN_DIRECTION_POWER_IN
	pwrOut = ir.PinDirection_PIN_DIRECTION_POWER_OUT
	pass   = ir.PinDirection_PIN_DIRECTION_PASSIVE
)

// pinDesign builds a design from parts (ref -> pins) and nets (name -> "ref.des" connections).
func pinDesign(parts map[string][]supplyPin, nets map[string][]string) *ir.Design {
	d := &ir.Design{SourceFormat: "kicad-sch", Libraries: []*ir.PartLibrary{{Name: "lib"}}}
	for ref, pins := range parts {
		pt := &ir.PartType{Name: ref + "_T"}
		for _, p := range pins {
			pt.Pins = append(pt.Pins, &ir.Pin{Name: p.name, Designator: p.des, Direction: p.dir})
		}
		d.Libraries[0].Parts = append(d.Libraries[0].Parts, pt)
		d.Components = append(d.Components, &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
			Sections: []*ir.ComponentSection{{PartRef: ref + "_T", LibraryRef: "lib"}}})
	}
	for name, conns := range nets {
		n := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}}
		for _, c := range conns {
			for i := len(c) - 1; i >= 0; i-- {
				if c[i] == '.' {
					n.Connections = append(n.Connections, &ir.Connection{ComponentRef: c[:i], PinRef: c[i+1:]})
					break
				}
			}
		}
		d.Nets = append(d.Nets, n)
	}
	return d
}

// verdictOn is a rule's outcome on one net, and "" when the rule gave the net no verdict at all.
func verdictOn(t *testing.T, r *check.Rule, d *ir.Design, net string) string {
	t.Helper()
	for _, v := range r.Eval(context.Background(), check.NewModel(d)) {
		for _, s := range v.Subjects {
			if s.Ref == net {
				return string(v.Outcome)
			}
		}
	}
	return ""
}

func twoPin() []supplyPin { return []supplyPin{{"1", "1", pass}, {"2", "2", pass}} }

func TestAFloatingExposedPadIsNotASupply(t *testing.T) {
	d := pinDesign(map[string][]supplyPin{
		"U1": {{"EP", "EP", pwrIn}, {"VDD", "1", pwrIn}},
	}, map[string][]string{
		"unconnected-(U1-PadEP)": {"U1.EP"},
		"unconnected-(U1-Pad1)":  {"U1.1"},
	})
	if got := verdictOn(t, decouplingPresent, d, "unconnected-(U1-PadEP)"); got != "" {
		t.Errorf("decoupling-present on a floating exposed pad = %q, want no verdict", got)
	}
	if got := verdictOn(t, powerInputNotDriven, d, "unconnected-(U1-PadEP)"); got != string(check.NotConsidered) {
		t.Errorf("power-input-not-driven on a floating exposed pad = %q, want not-considered", got)
	}
	// A VDD pin left unwired is the forgotten wire these rules exist for.
	if got := verdictOn(t, decouplingPresent, d, "unconnected-(U1-Pad1)"); got != string(check.Fail) {
		t.Errorf("decoupling-present on a floating VDD pin = %q, want fail", got)
	}
	if got := verdictOn(t, powerInputNotDriven, d, "unconnected-(U1-Pad1)"); got != string(check.Fail) {
		t.Errorf("power-input-not-driven on a floating VDD pin = %q, want fail", got)
	}
}

func TestADividerTapIntoASensePinIsNotASupply(t *testing.T) {
	d := pinDesign(map[string][]supplyPin{
		"U2": {{"OUT", "1", pwrOut}, {"GND", "2", pass}},
		"U1": {{"VBUS_DET", "80", pwrIn}},
		"R1": twoPin(), "R2": twoPin(), "R3": twoPin(),
		"U3":  {{"AVDD", "1", pwrIn}},
		"TP1": {{"1", "1", pass}},
	}, map[string][]string{
		"VBUS":        {"U2.1", "R1.1", "R3.1", "TP1.1"},
		"VBUS_MON_UP": {"R1.2", "R2.1", "U1.80"},
		"GND":         {"R2.2", "U2.2"},
		// A supply fed through one series resistor and no capacitor still wants its decoupling.
		"VBUS_ANA": {"R3.2", "U3.1"},
	})
	if got := verdictOn(t, decouplingPresent, d, "VBUS_MON_UP"); got != "" {
		t.Errorf("decoupling-present on a divider tap = %q, want no verdict", got)
	}
	if got := verdictOn(t, powerInputNotDriven, d, "VBUS_MON_UP"); got != string(check.NotConsidered) {
		t.Errorf("power-input-not-driven on a divider tap = %q, want not-considered", got)
	}
	if got := verdictOn(t, testPointCoverage, d, "VBUS_MON_UP"); got != "" {
		t.Errorf("test-point-coverage on a divider tap = %q, want it outside the rule's scope", got)
	}
	if got := verdictOn(t, decouplingPresent, d, "VBUS_ANA"); got != string(check.Fail) {
		t.Errorf("decoupling-present on a supply fed through one resistor = %q, want fail", got)
	}
}

func TestAConnectorsOwnPinIsNotALoad(t *testing.T) {
	// A source port: a load switch drives VBUS out to the cable, and the receptacle's symbol types
	// its VBUS pin power_in. Nothing on the board draws from VBUS, so there is no power path.
	source := pinDesign(map[string][]supplyPin{
		"J1": {{"VBUS", "A4", pwrIn}},
		"U1": {{"OUT", "1", pwrOut}},
	}, map[string][]string{"VBUS": {"J1.A4", "U1.1"}})
	if got := verdictOn(t, reverseBlockingAbsent, source, "VBUS"); got != "" {
		t.Errorf("reverse-blocking-absent on a source port = %q, want no verdict", got)
	}
	// A sink port feeds a load on the board through passives alone, which is the rule's finding.
	sink := pinDesign(map[string][]supplyPin{
		"J1": {{"VBUS", "A4", pwrIn}},
		"U1": {{"VIN", "1", pwrIn}},
	}, map[string][]string{"VBUS": {"J1.A4", "U1.1"}})
	if got := verdictOn(t, reverseBlockingAbsent, sink, "VBUS"); got != string(check.Fail) {
		t.Errorf("reverse-blocking-absent on a sink port = %q, want fail", got)
	}
}

func TestAPinlessPartIsNotUnconnected(t *testing.T) {
	d := pinDesign(map[string][]supplyPin{
		"H1": nil, // a mounting hole, whose symbol declares no pins
		"U1": {{"VDD", "1", pwrIn}},
		"U2": {{"VDD", "1", pwrIn}},
	}, map[string][]string{"VCC": {"U1.1"}})
	got := map[string]string{}
	for _, v := range unconnectedComponent.Eval(context.Background(), check.NewModel(d)) {
		got[v.Subjects[0].Ref] = string(v.Outcome)
	}
	if got["H1"] != string(check.NotConsidered) {
		t.Errorf("unconnected-component on a pinless part = %q, want not-considered", got["H1"])
	}
	if got["U2"] != string(check.Fail) {
		t.Errorf("unconnected-component on a part whose pins all float = %q, want fail", got["U2"])
	}
}

// A FET whose gate senses VBUS on a source port is not in its power path, so the port has none to
// block. A FET carrying the current between its drain and source still leaves the rule unable to
// tell an ideal diode from a switch.
func TestAGateOnTheConnectorNetIsNotInThePowerPath(t *testing.T) {
	gate := pinDesign(map[string][]supplyPin{
		"J1": {{"VBUS", "A4", pwrIn}},
		"U1": {{"OUT", "1", pwrOut}},
		"Q1": {{"G", "1", ir.PinDirection_PIN_DIRECTION_INPUT}, {"S", "2", pass}, {"D", "3", pass}},
	}, map[string][]string{"VBUS": {"J1.A4", "U1.1", "Q1.1"}, "DET": {"Q1.3"}, "GND": {"Q1.2"}})
	if got := verdictOn(t, reverseBlockingAbsent, gate, "VBUS"); got != "" {
		t.Errorf("reverse-blocking-absent with a FET gate on a source port's VBUS = %q, want no verdict", got)
	}
	series := pinDesign(map[string][]supplyPin{
		"J1": {{"VBUS", "A4", pwrIn}},
		"Q1": {{"G", "1", ir.PinDirection_PIN_DIRECTION_INPUT}, {"S", "2", pass}, {"D", "3", pass}},
		"U2": {{"VIN", "1", pwrIn}},
	}, map[string][]string{"VBUS": {"J1.A4", "Q1.3"}, "VSYS": {"Q1.2", "U2.1"}, "EN": {"Q1.1"}})
	if got := verdictOn(t, reverseBlockingAbsent, series, "VBUS"); got != string(check.Inconclusive) {
		t.Errorf("reverse-blocking-absent with a series FET = %q, want inconclusive", got)
	}
}
