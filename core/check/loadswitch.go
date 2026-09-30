package check

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// The external-FET load switch (WS3-085). An integrated load switch is one part, and everything a
// rule wants to know about it is in that part's datasheet: its on-resistance, its current limit. A
// CONTROLLER-based switch is three parts, and neither number is a constant of any of them. The
// on-resistance belongs to whichever MOSFET the designer chose, and the current limit is set by an
// external sense resistor the designer also chose, divided into a threshold voltage the controller
// states. So the numbers only exist once the netlist and the datasheet set are read together.
//
// The resolution lives in the model layer because a sizing rule, a fuse-selection rule, and the
// trip-rating rule all need it first.

// senseResistorCeilingOhms is the largest resistance this resolver will accept as a CURRENT SHUNT.
//
// The structural signature of a Kelvin-sensed shunt (a resistor whose every terminal lands on a net
// the controller also touches) is shared by one other arrangement, a feedback or programming divider
// hung between a controller's output pin and its sense pin. Nothing in a netlist distinguishes them.
// Magnitude does, by orders of magnitude. A shunt sized to drop tens of millivolts at amperes is a
// milliohm-class part, while a divider that must not waste current is kilohms. One ohm sits in the
// empty middle, so the filter is not a tuned threshold.
//
// It fails toward silence. A shunt above an ohm (a very low-current sense) is skipped and the switch
// yields no verdict.
const senseResistorCeilingOhms = 1.0

// ExternalFetLoadSwitch is a load switch resolved from a switch CONTROLLER, the EXTERNAL MOSFET it
// drives, and the external SENSE RESISTOR that sets the current limit, joined to the seeded datasheet
// set. Every field is populated; a switch that could not be resolved completely is not returned at
// all, rather than returned with holes a caller might read as zeroes.
type ExternalFetLoadSwitch struct {
	// Controller is the ref-des of the part driving the gate, identified by its datasheet stating an
	// overcurrent threshold. Fet is the pass element it drives. Sense is the shunt across the
	// controller's sense pins.
	Controller string
	Fet        string
	Sense      string
	// GateNet is the net carrying the controller's drive and the FET's gate terminal.
	GateNet string
	// SenseOhms is the shunt's value in ohms, read from the DESIGN (the component's value attribute),
	// not from any datasheet. It is the half of the trip current the vendor does not know.
	SenseOhms float64
	// TripAmps is the current at which the switch limits, Ocp's threshold divided by SenseOhms.
	TripAmps float64
	// Ocp is the controller's overcurrent-threshold row the trip current was computed from, kept so a
	// caller can cite the document it came from. Where a controller states several, the HIGHEST binds,
	// because it is the most current the switch will pass before it acts and so the worst case for
	// everything downstream of it.
	Ocp *parampb.Parameter
	// OnResistance is the FET's worst-case RDS(on) row, which IS this switch's effective on-resistance.
	// Nil when the FET is unseeded or states no comparable row; a caller must treat nil as "not known"
	// and never as zero, since a zero on-resistance is a perfect switch.
	OnResistance *parampb.Parameter
}

// ExternalFetLoadSwitches resolves every controller-plus-external-FET load switch in the design.
//
// An ambiguous switch yields no entry. That covers a FET whose gate net carries two candidate
// controllers, a controller with two candidate shunts, and a shunt whose value the design does not
// state in ohms. An over-current verdict computed from the wrong resistor is worse than no verdict,
// because it looks as authoritative as a correct one.
func ExternalFetLoadSwitches(m Model) []ExternalFetLoadSwitch {
	nets := m.Nets()
	var out []ExternalFetLoadSwitch
	for _, comp := range m.Components() {
		fet := comp.GetRefDes()
		if m.ComponentClass(fet) != ClassTransistor {
			continue
		}
		gate := soleGateNet(m, nets, fet)
		if gate == nil {
			continue
		}
		ctrl, ocp := soleController(m, gate, fet)
		if ctrl == "" {
			continue
		}
		sense, ohms, ok := soleSenseResistor(m, nets, ctrl)
		if !ok {
			continue
		}
		amps, ok := OhmsLawCurrent(ocp.Value.GetMax(), ohms)
		if !ok {
			continue
		}
		out = append(out, ExternalFetLoadSwitch{
			Controller: ctrl, Fet: fet, Sense: sense, GateNet: gate.GetName(),
			SenseOhms: ohms, TripAmps: amps,
			Ocp: ocp, OnResistance: worstOnResistance(m.PartSpec(fet)),
		})
	}
	return out
}

// soleGateNet returns the single net carrying fet's GATE terminal, or nil when the part declares no
// gate pin, its gate pin is unconnected, or its gate pins land on more than one net. The role comes
// from the naming lexicon's RoleVocab (WS3-117) and never from a pin name matched here (C20; see
// pinrole.go).
func soleGateNet(m Model, nets []*ir.Net, fet string) *ir.Net {
	var found *ir.Net
	for _, n := range nets {
		for _, c := range n.GetConnections() {
			if c.GetComponentRef() != fet || m.PinRole(fet, c.GetPinRef()) != RoleGate {
				continue
			}
			if found != nil && found != n {
				return nil // two gate nets, so not a single pass element this resolver can reason about
			}
			found = n
		}
	}
	return found
}

// soleController returns the ref-des of the single switch controller driving the gate net, with the
// binding overcurrent-threshold row; "" when there is none or more than one.
//
// THE CONTROLLER IS IDENTIFIED BY WHAT ITS DATASHEET STATES, not by a device-class keyword. A part
// that declares an overcurrent sense threshold is a current-limiting controller, and a part that does
// not is not, whatever its description string says. That is stronger evidence than a class label.
// It also settles integrated versus controller, since an integrated switch has no external FET on a
// gate net to be found from.
//
// The connection must be DIRECT. A series gate resistor between controller and FET is common and
// defeats this, a known limit. Crossing passives here would also cross a gate pull-down into the
// source net's neighbourhood, and a wrong controller yields a wrong ampere figure. Refusing is the
// cheaper error.
func soleController(m Model, gate *ir.Net, fet string) (string, *parampb.Parameter) {
	var ref string
	var binding *parampb.Parameter
	for _, c := range gate.GetConnections() {
		r := c.GetComponentRef()
		if r == fet || r == ref || IsVirtualRef(r) || m.ComponentClass(r) == ClassTransistor {
			continue
		}
		spec := m.PartSpec(r)
		if spec == nil {
			continue
		}
		rows := OcpThresholdLimits(spec)
		if len(rows) == 0 {
			continue
		}
		if ref != "" {
			return "", nil // two controllers on one gate are unresolvable, never a guess
		}
		ref, binding = r, rows[0]
		for _, p := range rows[1:] {
			if p.Value.GetMax() > binding.Value.GetMax() {
				binding = p
			}
		}
	}
	return ref, binding
}

// soleSenseResistor returns the single shunt across the controller's sense pins and its value in
// ohms, or ok=false when there is none or more than one.
//
// THE SIGNATURE IS KELVIN SENSING, expressed structurally as a resistor EVERY one of whose nets is also
// touched by the controller. A shunt is measured by two dedicated pins landing on its two terminals,
// so both of its nets are the controller's. A series element that happens to sit near the
// controller has a far side the controller does not touch, so the structure excludes it with no name
// match.
//
// The value comes from the design and must be stated IN OHMS (WS3-118). A component whose value the
// reader never normalized, or normalized without a unit, is not evidence that it is a milliohm shunt.
func soleSenseResistor(m Model, nets []*ir.Net, ctrl string) (string, float64, bool) {
	onCtrl := map[*ir.Net]bool{}
	for _, n := range nets {
		if netTouches(n, ctrl) {
			onCtrl[n] = true
		}
	}
	var ref string
	var ohms float64
	for _, comp := range m.Components() {
		r := comp.GetRefDes()
		if m.ComponentClass(r) != ClassResistor {
			continue
		}
		touched, allOnCtrl := 0, true
		for _, n := range nets {
			if !netTouches(n, r) {
				continue
			}
			touched++
			if !onCtrl[n] {
				allOnCtrl = false
			}
		}
		if touched < 2 || !allOnCtrl {
			continue
		}
		// A zero or negative reading is not filtered here on purpose. OhmsLawCurrent is the single
		// place that decides what resistance admits a current, and duplicating the test would leave a
		// branch no test could distinguish from it.
		v, ok := ComponentValueIn(m, r, UnitOhm)
		if !ok || v > senseResistorCeilingOhms {
			continue
		}
		if ref != "" {
			return "", 0, false // two candidate shunts are unresolvable, never a guess
		}
		ref, ohms = r, v
	}
	if ref == "" {
		return "", 0, false
	}
	return ref, ohms, true
}

// worstOnResistance returns the highest-RDS(on) row a FET's spec states, or nil when it is unseeded or
// states none comparably. A real sheet gives several rows under different gate drives and junction
// temperatures; the highest is the one a thermal argument has to survive, so it is the one reported.
// An unseeded FET needs no separate branch, because the proto getters are nil-tolerant and a nil spec
// states no rows.
func worstOnResistance(spec *parampb.PartSpec) *parampb.Parameter {
	rows := OnResistanceLimits(spec)
	if len(rows) == 0 {
		return nil
	}
	worst := rows[0]
	for _, p := range rows[1:] {
		if p.Value.GetMax() > worst.Value.GetMax() {
			worst = p
		}
	}
	return worst
}

// netTouches reports whether refDes has any connection on n.
func netTouches(n *ir.Net, refDes string) bool {
	for _, c := range n.GetConnections() {
		if c.GetComponentRef() == refDes {
			return true
		}
	}
	return false
}
