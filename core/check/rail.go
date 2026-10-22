package check

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// A zero-ohm link is a net boundary in the netlist and not in the circuit. A rail joined to its
// converter's output node through a 0 Ohm resistor is one rail with two names, and a rule asking what
// sits on the rail asks across the link (agni issue 1015).

// ZeroOhmLink reports whether ref is a fitted resistor whose value is zero ohms. A library that writes
// its part name as the Value (`R_0R_22.4A_0603`) has that value read out of the name at ingestion
// (agni issue 1115), so the name needs no reading here.
func ZeroOhmLink(m Model, ref string) bool {
	if m.ComponentClass(ref) != ClassResistor || !m.IsFitted(ref) {
		return false
	}
	v, ok := ComponentValueIn(m, ref, UnitOhm)
	return ok && v == 0
}

// JoinedNet is one net of a rail and the zero-ohm link that joined it, which is "" for the net the
// question started from.
type JoinedNet struct {
	Net *ir.Net
	Via string
}

// JoinedRail returns n and every net joined to it through zero-ohm links, transitively, with n first
// and the rest in the order the walk reached them. It never crosses into a ground, so a 0 Ohm link
// tying a rail's return to a chassis net does not make the ground part of the rail.
func JoinedRail(m Model, n *ir.Net) []JoinedNet {
	out := []JoinedNet{{Net: n}}
	if n == nil || !hasZeroOhmLink(m, n) {
		return out
	}
	netsOf := map[string][]*ir.Net{}
	for _, o := range m.Nets() {
		seen := map[string]bool{}
		for _, c := range o.Connections {
			if !seen[c.ComponentRef] {
				seen[c.ComponentRef] = true
				netsOf[c.ComponentRef] = append(netsOf[c.ComponentRef], o)
			}
		}
	}
	visited := map[string]bool{n.Name: true}
	for i := 0; i < len(out); i++ {
		for _, c := range out[i].Net.Connections {
			if !ZeroOhmLink(m, c.ComponentRef) || len(netsOf[c.ComponentRef]) != 2 {
				continue
			}
			for _, o := range netsOf[c.ComponentRef] {
				if visited[o.Name] || m.IsGroundNet(o) {
					continue
				}
				visited[o.Name] = true
				out = append(out, JoinedNet{Net: o, Via: c.ComponentRef})
			}
		}
	}
	return out
}

// hasZeroOhmLink reports whether a zero-ohm link touches n, so JoinedRail indexes the design only
// when there is a link to cross.
func hasZeroOhmLink(m Model, n *ir.Net) bool {
	for _, c := range n.Connections {
		if ZeroOhmLink(m, c.ComponentRef) {
			return true
		}
	}
	return false
}

// FirstOnRail names the first fitted part of the class on n or on a net joined to it through
// zero-ohm links, with the joined net it sits on (whose Via is "" when that is n itself). The ref is
// "" when the rail carries none.
func FirstOnRail(m Model, n *ir.Net, class ComponentClass) (string, JoinedNet) {
	for _, j := range JoinedRail(m, n) {
		for _, c := range j.Net.Connections {
			if m.HasClass(c.ComponentRef, class) && m.IsFitted(c.ComponentRef) {
				return c.ComponentRef, j
			}
		}
	}
	return "", JoinedNet{}
}

// BoardSource names what drives n from the board itself, and "" when nothing does. A connector on
// such a net is an outlet the board powers, so the rules asking how power ENTERS through a connector
// leave the net out (agni issue 1016). Jetson's +3V3 feeds an SFP cage and an M.2 socket from the
// board's own 3.3 V buck through a 0 Ohm link.
//
// A converter's output counts outright, read from structure because a vendor symbol often types
// every pin passive: a switching net reached through an inductor and a feedback net, together, within
// the power-entry walk's radius. A boost converter's INPUT reaches its switch node through the
// inductor as well, and asking for the feedback divider too keeps that input a supply entry, since a
// boost's divider sits on its output side.
//
// A power-output pin on a fitted part other than a connector counts only when nothing on the walk is
// the input of another part with a power output. A dual-role USB-C port is the shape that rule keeps:
// a load switch sources VBUS and a PD controller's power path sinks it, so power can still arrive
// through the connector and the entry rules still judge it.
func BoardSource(m Model, n *ir.Net) string {
	r := m.Reach(n, PowerPathReachHops)
	var sw, coil, fb, out string
	var sinks []string
	for _, rn := range r.Nets {
		for _, c := range rn.Connections {
			ref := c.ComponentRef
			if IsVirtualRef(ref) || !m.IsFitted(ref) || m.HasClass(ref, ClassConnector) {
				continue
			}
			switch ConnDir(m, c) {
			case ir.PinDirection_PIN_DIRECTION_POWER_OUT:
				if out == "" {
					out = ref + "'s power output on " + rn.GetName()
				}
			case ir.PinDirection_PIN_DIRECTION_POWER_IN:
				sinks = append(sinks, ref)
			}
		}
		if step, ok := r.Parent[rn.GetName()]; ok && sw == "" && m.HasClass(step.Through, ClassInductor) && m.HasAnyRole(rn, ir.Role_ROLE_SWITCHING) {
			sw, coil = rn.GetName(), step.Through
		}
		if fb == "" && m.HasAnyRole(rn, ir.Role_ROLE_FEEDBACK) {
			fb = rn.GetName()
		}
	}
	switch {
	case sw != "" && fb != "":
		return "a converter's output, its switch node " + sw + " behind inductor " + coil + " and its feedback on " + fb
	case out != "" && !anyPowerSource(m, sinks):
		return out
	}
	return ""
}

// anyPowerSource reports whether any of refs has a power-output pin anywhere in the design, which
// makes its power input on a net a path power can take onward rather than a load.
func anyPowerSource(m Model, refs []string) bool {
	if len(refs) == 0 {
		return false
	}
	want := map[string]bool{}
	for _, r := range refs {
		want[r] = true
	}
	for _, n := range m.Nets() {
		for _, c := range n.Connections {
			if want[c.ComponentRef] && ConnDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_OUT {
				return true
			}
		}
	}
	return false
}
