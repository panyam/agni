package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// Vendor symbols type some pins power_in even though those pins take no supply, and the supply
// rules read the type (agni issue 935). The two shapes here are recognised by structure, never by
// the symbol alone, so a pin that really is a supply input still fails.

// floatingExposedPad reports why a net is a lone exposed pad rather than a supply, and "" when it is
// not one. KiCad names an unwired pin's net `unconnected-(U29-PadEP)`, and many parts' datasheets let
// the exposed pad float, so a power_in pad alone on its net is not a rail missing its capacitor or its
// driver. Only the pad name exempts it. A lone VDD pin is the forgotten wire these rules exist to
// catch.
func floatingExposedPad(m check.Model, n *ir.Net) string {
	var only *ir.Connection
	for _, c := range n.GetConnections() {
		if check.IsVirtualRef(c.GetComponentRef()) {
			continue
		}
		if only != nil {
			return ""
		}
		only = c
	}
	if only == nil || check.ConnDir(m, only) != ir.PinDirection_PIN_DIRECTION_POWER_IN {
		return ""
	}
	if !m.IsExposedPadName(m.PinName(only.GetComponentRef(), only.GetPinRef())) {
		return ""
	}
	return "the net is " + only.GetComponentRef() + "'s exposed pad, left unconnected, so it is not a supply rail"
}

// dividerTap reports why a net is the midpoint of a resistive divider feeding a sense input rather
// than a supply, and "" when it is not one. A USB hub's VBUS-sense pin sits between a resistor to VBUS
// and one to ground, and a symbol types such a pin power_in, input or bidirectional (Jetson's hub
// draws it bidirectional, agni issue 1241). A divider needs BOTH legs. So a supply fed through a
// single series resistor or a 0 Ohm link is not a divider tap, and it still wants its decoupling.
//
// A divider tap still reads as a rail to Model.IsRailNet when its name matches the rail vocabulary
// (VBUS_MON_UP does). Subtracting it there would change every rail-quantified rule and query. That
// change waits until someone measures how many such taps a real corpus holds.
func dividerTap(m check.Model, n *ir.Net) string {
	self := map[string]bool{n.GetName(): true}
	var up, down string
	sense := false
	for _, c := range n.GetConnections() {
		ref := c.GetComponentRef()
		if check.IsVirtualRef(ref) {
			continue
		}
		if senseDir(check.ConnDir(m, c)) {
			sense = true
			continue
		}
		if m.ComponentClass(ref) != check.ClassResistor {
			return ""
		}
		far := farNet(m, ref, self)
		switch {
		case far == nil:
			return ""
		case m.IsGroundNet(far):
			down = ref
		case m.IsRailNet(far) || far.GetAttributes()[netgraph.AttrPowerDriven] == "true" || check.CountDir(check.NetDirs(m, far), check.IsDriver) > 0:
			if up == "" {
				up = ref + " from " + far.GetName()
			}
		}
	}
	// A divider feeding no power_in pin is no sense input, and the rules asking about one have their
	// own reasons (a resistor-only rail in a test-point fixture is still a rail to probe).
	if !sense || up == "" || down == "" {
		return ""
	}
	return "the net is a divider tap, fed through " + up + " and returned through " + down + " to ground, so its input pin senses a voltage rather than drawing a supply"
}

// senseDir reports whether a pin typed d can be the input a divider tap feeds.
func senseDir(d ir.PinDirection) bool {
	switch d {
	case ir.PinDirection_PIN_DIRECTION_POWER_IN, ir.PinDirection_PIN_DIRECTION_INPUT, ir.PinDirection_PIN_DIRECTION_INOUT:
		return true
	}
	return false
}

// ownBiasSupply reports why a net is one part's own internal supply rather than a board rail, and ""
// when it is not one (agni issue 1914). A SIC431 buck makes its VDD bias from an internal regulator
// and wants only a capacitor on the pin, so Jetson names six such nets `12V_VDD` and the like after
// the converter. The shape is one fitted part's supply-named pin, typed passive or power_out, with
// nothing else fitted on the net but capacitors. Nothing on the board feeds that net, so the part
// itself does.
func ownBiasSupply(m check.Model, n *ir.Net) string {
	owner, pin, caps := "", "", []string{}
	for _, c := range n.GetConnections() {
		ref := c.GetComponentRef()
		if check.IsVirtualRef(ref) || !m.IsFitted(ref) {
			continue
		}
		if m.ComponentClass(ref) == check.ClassCapacitor {
			caps = append(caps, ref)
			continue
		}
		if owner != "" || check.IsPassiveClass(m.ComponentClass(ref)) || m.HasClass(ref, check.ClassConnector) {
			return ""
		}
		// A power_in pin takes a supply from the board, and alone with its capacitor it is the
		// forgotten wire the supply rules exist to catch. A bias output reads passive or power_out.
		if check.ConnDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_IN {
			return ""
		}
		owner, pin = ref, m.PinName(ref, c.GetPinRef())
	}
	if owner == "" || len(caps) == 0 || !m.IsSupplyPinName(pin) {
		return ""
	}
	return "the net is " + owner + "'s own internal supply on its " + pin + " pin, decoupled by " + caps[0] + ", and nothing else on the board draws from it or feeds it"
}

// controlSignal reports why a rail-named net is a control signal rather than a supply, and "" when it
// is not one (agni issue 1914). Jetson's `VIN_PWR_ON` drives three FET gates and a buck's enable, and
// its `VIN` prefix reads as a rail. Every part pin on such a net is an input, and none is a power
// input or a passive-typed pin that could be a supply.
func controlSignal(m check.Model, n *ir.Net) string {
	inputs := 0
	for _, c := range n.GetConnections() {
		ref := c.GetComponentRef()
		if check.IsVirtualRef(ref) || !m.IsFitted(ref) || check.IsPassiveClass(m.ComponentClass(ref)) {
			continue
		}
		if check.ConnDir(m, c) != ir.PinDirection_PIN_DIRECTION_INPUT {
			return ""
		}
		inputs++
	}
	if inputs == 0 {
		return ""
	}
	return "every part pin on the net is an input, so it carries a control signal rather than a supply"
}

// notASupply returns the reason a net with a power-input pin is not a supply rail, or "".
func notASupply(m check.Model, n *ir.Net) string {
	for _, f := range []func(check.Model, *ir.Net) string{floatingExposedPad, dividerTap, ownBiasSupply, controlSignal} {
		if r := f(m, n); r != "" {
			return r
		}
	}
	return ""
}
