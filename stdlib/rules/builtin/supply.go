package builtin

import (
	"regexp"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// Vendor symbols type pins power_in that take no supply, and the supply rules read the type (agni
// issue 935). The two shapes here are recognised by structure, never by the symbol alone, so a pin
// that really is a supply input still fails.

// exposedPadName matches the names vendors give a package's exposed thermal pad.
var exposedPadName = regexp.MustCompile(`(?i)^(ep|epad|exp|pad|exposed_?pad|thermal_?pad)$`)

// floatingExposedPad reports why a net is a lone exposed pad rather than a supply, and "" when it is
// not one. KiCad names an unwired pin's net `unconnected-(U29-PadEP)`, and many parts' datasheets let
// the exposed pad float, so a power_in pad alone on its net is not a rail missing its capacitor or its
// driver. Only the pad name exempts it: a lone VDD pin is the forgotten wire these rules exist for.
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
	if !exposedPadName.MatchString(m.PinName(only.GetComponentRef(), only.GetPinRef())) {
		return ""
	}
	return "the net is " + only.GetComponentRef() + "'s exposed pad, left unconnected, so it is not a supply rail"
}

// dividerTap reports why a net is the midpoint of a resistive divider feeding a sense input rather
// than a supply, and "" when it is not one. A USB hub's VBUS-sense pin sits between a resistor to VBUS
// and one to ground, and its symbol types the pin power_in. A divider needs BOTH legs, so a supply fed
// through a single series resistor or a 0 Ohm link, which still wants its decoupling, is not one.
//
// A divider tap still reads as a rail to Model.IsRailNet when its name matches the rail vocabulary
// (VBUS_MON_UP does). Subtracting it there would change every rail-quantified rule and query, and was
// left until someone measures how many such taps a real corpus holds.
func dividerTap(m check.Model, n *ir.Net) string {
	self := map[string]bool{n.GetName(): true}
	var up, down string
	sense := false
	for _, c := range n.GetConnections() {
		ref := c.GetComponentRef()
		if check.IsVirtualRef(ref) {
			continue
		}
		if check.ConnDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_IN {
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
	return "the net is a divider tap, fed through " + up + " and returned through " + down + " to ground, so its power-input pin senses a voltage rather than drawing a supply"
}

// notASupply is the reason a net carrying a power-input pin is not a supply rail, or "".
func notASupply(m check.Model, n *ir.Net) string {
	if r := floatingExposedPad(m, n); r != "" {
		return r
	}
	return dividerTap(m, n)
}
