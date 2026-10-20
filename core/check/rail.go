package check

import (
	"strings"
	"unicode"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// A zero-ohm link is a net boundary in the netlist and not in the circuit. A rail joined to its
// converter's output node through a 0 Ohm resistor is one rail with two names, and a rule asking what
// sits on the rail asks across the link (agni issue 1015).

// ZeroOhmLink reports whether ref is a fitted resistor of zero ohms. The value decides it when it
// parsed to a number, and otherwise a token of the value text that spells zero ohms with its unit
// does, since some libraries name a part `R_0R_22.4A_0603` and that whole string parses to nothing.
// A bare `0` token counts only as the whole value, because inside a name it is as likely a package
// size or a tolerance.
func ZeroOhmLink(m Model, ref string) bool {
	if m.ComponentClass(ref) != ClassResistor || !m.IsFitted(ref) {
		return false
	}
	if v, ok := ComponentValueIn(m, ref, UnitOhm); ok {
		return v == 0
	}
	text, _ := ComponentValueText(m, ref)
	for _, tok := range strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.'
	}) {
		if v, unit, ok := classify.ParseQuantity(tok); ok && v == 0 && unit == UnitOhm {
			return true
		}
	}
	return false
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
