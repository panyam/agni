package check

import (
	"sort"

	"github.com/panyam/agni/core/ident"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// PinPartners is one pin of a device, with the pins on other devices that its net reaches through
// series parts. A pin map would declare those pins as its far ends (agni issue 1238).
//
// Reason says why no far end was looked for, and is empty when the walk ran. With Reason empty and
// no Partners, the pin reached nothing that could be a far end. With several Partners it reached more
// than one, and the netlist alone cannot decide between them.
type PinPartners struct {
	Pin      string
	Name     string
	Net      string
	Partners []Endpoint
	Reason   string
}

// Reasons PartnersOf gives for not walking from a pin.
const (
	PartnerReasonNoNet  = "on no net"
	PartnerReasonGround = "ground"
	PartnerReasonRail   = "a supply rail"
)

// partnerClass reports whether a class's pins can be a signal's far end. Such a part drives or
// receives the signal, or is a connector that carries it off the board. Passives and protection
// parts sit on a signal rather than terminate it, so a trace reports them as stubs and they are
// never a partner.
func partnerClass(c ComponentClass) bool {
	switch c {
	case ClassIC, ClassConnector, ClassIdealDiodeController, ClassCurrentLimitSwitch, ClassClock, ClassOscillator:
		return true
	}
	return false
}

// PartnersOf walks from every pin of refDes, as TracePins walks, and lists the pins of fitted
// devices it reaches, in pin order. It drafts a pin map from the netlist, so what it finds agrees
// with the netlist by construction. A reviewer still checks each row against the datasheet or the
// interface it implements, and a later revision is then checked against the reviewed map.
//
// A pin on ground or a supply rail is not walked, since every part on the board reaches it. Rails
// reached from a signal pin are left out for the same reason, so a pull-up never makes the rail's
// loads a signal's partners.
func PartnersOf(m Model, refDes string, hops int) []PinPartners {
	if hops <= 0 {
		hops = DefaultTraceHops
	}
	// One pass over the nets finds every pin of the device. Calling netOfPin per pin would scan every
	// net once per pin. A module connector has hundreds of pins, so that cost shows on a large board.
	netOf := map[string]*ir.Net{}
	for _, n := range m.Nets() {
		for _, c := range n.Connections {
			if c.ComponentRef == refDes {
				if _, ok := netOf[c.PinRef]; !ok {
					netOf[c.PinRef] = n
				}
			}
		}
	}
	// First the declared pins, including the ones on no net. Then any connected pin that the part
	// type does not declare, since a netlist with no part types gives the model no pin list at all.
	var pins []string
	listed := map[string]bool{}
	for _, p := range m.Pins() {
		if p.Component.GetRefDes() == refDes && !listed[p.Designator] {
			listed[p.Designator] = true
			pins = append(pins, p.Designator)
		}
	}
	for pin := range netOf {
		if !listed[pin] {
			pins = append(pins, pin)
		}
	}
	var out []PinPartners
	for _, pin := range pins {
		pp := PinPartners{Pin: pin, Name: m.PinName(refDes, pin)}
		n := netOf[pin]
		switch {
		case n == nil:
			pp.Reason = PartnerReasonNoNet
		case m.IsGroundNet(n):
			pp.Net, pp.Reason = n.Name, PartnerReasonGround
		case m.IsRailNet(n) || IsBusLike(m, n):
			pp.Net, pp.Reason = n.Name, PartnerReasonRail
		default:
			pp.Net = n.Name
			pp.Partners = reachedDevicePins(m, refDes, m.ReachToTerminus(n, hops))
		}
		out = append(out, pp)
	}
	sort.SliceStable(out, func(i, j int) bool { return ident.CompareNatural(out[i].Pin, out[j].Pin) < 0 })
	return out
}

// reachedDevicePins lists the pins of partner-class parts other than self on the nets a walk
// reached, skipping a reached rail or bus-like net.
func reachedDevicePins(m Model, self string, r Reach) []Endpoint {
	seen := map[Endpoint]bool{}
	var out []Endpoint
	for i, n := range r.Nets {
		if i > 0 && (m.IsRailNet(n) || IsBusLike(m, n)) {
			continue
		}
		for _, c := range n.Connections {
			ref := c.ComponentRef
			if ref == self || IsVirtualRef(ref) || !m.IsFitted(ref) || !partnerClass(m.ComponentClass(ref)) {
				continue
			}
			e := Endpoint{RefDes: ref, Pin: c.PinRef}
			if !seen[e] {
				seen[e] = true
				out = append(out, e)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RefDes != out[j].RefDes {
			return ident.CompareNatural(out[i].RefDes, out[j].RefDes) < 0
		}
		return ident.CompareNatural(out[i].Pin, out[j].Pin) < 0
	})
	return out
}
