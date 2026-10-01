package check

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// passClass reports a component class the walk may cross terminal to terminal, which is the
// series pass elements. Capacitors are absent because a series cap is a DC block, as are diodes
// (polarity) and everything active.
func passClass(c ComponentClass) bool {
	switch c {
	case ClassResistor, ClassInductor, ClassFerrite, ClassFuse:
		return true
	}
	return false
}

// maxWalkFan bounds the fan-out of a net the walk may cross INTO. A series node carries a
// handful of members and a rail or bus carries dozens, so without this guard a name-only rail (an
// EDIF "+5V" with no attributes) turns a pull-up into a doorway to the whole design.
const maxWalkFan = 16

// ProtectionReachHops is the radius every protection guard asks at, so a clamp, fuse or power pin
// counts only within two series crossings of the net. The number is ELECTRICAL (series impedance
// before the clamp conducts), not a search budget, so it is much smaller than query.reachHops, which
// answers a topology question. Widening it credits distant clamps and turns unprotected pins into
// silent passes. See docsite/content/architecture/net-solving.md#how-far-a-walk-crosses-series-parts.
const ProtectionReachHops = 2

// SupplyPathReachHops is the radius a supply-compatibility question asks at (WS3-028). One crossing
// lets a filter bead or series resistor between a regulator and its load through. Wider is wrong
// because voltage does NOT degrade along the path, so every part would look fed by every regulator.
// See docsite/content/architecture/net-solving.md#how-far-a-walk-crosses-series-parts.
const SupplyPathReachHops = 1

// PowerPathReachHops is the radius the power-entry walk asks at (UnprotectedPowerReach), one hop
// wider than ProtectionReachHops. A power entry crosses connector, fuse and bead before the
// regulator's input node, and the signal radius would stop short of the regulator and read an
// unprotected path as having nothing to protect.
const PowerPathReachHops = 3

// IsBusLike reports a shared-DISTRIBUTION net the series-reach walk must not cross INTO. It is a
// ground, a net marked global, or one with more than maxWalkFan connections. A rail-looking NAME is
// NOT bus-like, since protection rules walk through nodes like VBUS and 5V_PROT, and neither is the
// power_driven fact, since a PWR_FLAG marks the power entry nets the walk exists to reach. The reach
// walk and the net.bus_like relation share this ONE definition (WS3-080), and the walk's start net
// is never treated as a stop.
func IsBusLike(m Model, n *ir.Net) bool {
	a := n.GetAttributes()
	return a[netgraph.AttrGlobal] == "true" ||
		m.IsGroundNet(n) || len(n.Connections) > maxWalkFan
}

// Reach runs the bounded BFS over the model's pass-element adjacency, refusing a bus-like net
// outright. See walk for the shared body and admitTerminus for the other admission rule.
func (m *irModel) Reach(start *ir.Net, hops int) Reach {
	return m.walk(start, hops, false)
}

// ReachToTerminus is Reach except that a bus-like net is admitted as a DESTINATION while still
// refused as a transit node. The walk lands on a rail, records how it got there, and does not
// continue through it.
//
// Reach excludes a bus-like net from the result set entirely, not merely from the frontier, so it
// cannot arrive at the rail a pull-up terminates on (PullUpPathToRail carries its own BFS for that
// reason). Use this variant when the question's endpoint may be a rail. Endpoint versus transit is
// a property of the POSITION in the path, so filtering the node set once cannot express it (agni
// issue 374).
func (m *irModel) ReachToTerminus(start *ir.Net, hops int) Reach {
	return m.walk(start, hops, true)
}

// walk is the bounded BFS both reach variants share. admitTerminus decides whether a bus-like
// net is skipped entirely or recorded and never expanded.
func (m *irModel) walk(start *ir.Net, hops int, admitTerminus bool) Reach {
	r := Reach{Crossed: map[string]bool{}, Parent: map[string]ReachStep{}, Depth: map[string]int{}}
	if start == nil {
		return r
	}
	visited := map[string]bool{start.Name: true}
	frontier := []*ir.Net{start}
	r.Nets = append(r.Nets, start)
	r.Depth[start.Name] = 0
	for depth := 0; depth < hops && len(frontier) > 0; depth++ {
		var next []*ir.Net
		for _, n := range frontier {
			for _, c := range n.Connections {
				if !passClass(m.ComponentClass(c.ComponentRef)) {
					continue
				}
				others := m.passNets[c.ComponentRef]
				if len(others) != 2 {
					continue // only a TWO-net element is a series crossing
				}
				for _, o := range others {
					if visited[o.Name] || o.Name == n.Name {
						continue
					}
					busLike := IsBusLike(m, o)
					if busLike && !admitTerminus {
						continue
					}
					visited[o.Name] = true
					r.Crossed[c.ComponentRef] = true
					r.Parent[o.Name] = ReachStep{
						From: n.Name, Through: c.ComponentRef,
						FromPin: c.PinRef, ToPin: pinOn(o, c.ComponentRef),
					}
					r.Depth[o.Name] = depth + 1 // BFS, so the first visit is the shortest
					r.Nets = append(r.Nets, o)
					if busLike {
						continue // a legal destination, never a transit node
					}
					next = append(next, o)
				}
			}
		}
		frontier = next
	}
	return r
}

// pinOn returns ref's pin designator on net n, or "" when the connection carries none. The first
// match wins: a part with both pins on one net is a short rather than a crossing, and the walk
// refuses that case (o.Name == n.Name) before it gets here.
func pinOn(n *ir.Net, ref string) string {
	for _, c := range n.Connections {
		if c.ComponentRef == ref {
			return c.PinRef
		}
	}
	return ""
}

// Between reports whether a component of the given class sits ON the series path from
// one net to another, within the hop bound. It is also false when `to` is not reachable at
// all, so a caller that needs to tell unreachable from unprotected tests reachability first
// via Reach.
func (m *irModel) Between(from, to *ir.Net, class ComponentClass, hops int) bool {
	if from == nil || to == nil {
		return false
	}
	r := m.Reach(from, hops)
	if _, ok := r.Parent[to.Name]; !ok {
		return false
	}
	for _, ref := range r.ThroughOnPath(to) {
		if m.ComponentClass(ref) == class {
			return true
		}
	}
	return false
}
