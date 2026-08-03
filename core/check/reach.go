package check

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// passClass reports a component class the walk may cross terminal-to-terminal: the
// series pass elements. Capacitors are deliberately absent (a series cap is a DC block,
// not a pass), as are diodes (polarity, not a wire) and everything active.
func passClass(c ComponentClass) bool {
	switch c {
	case ClassResistor, ClassInductor, ClassFerrite, ClassFuse:
		return true
	}
	return false
}

// maxWalkFan bounds the fan-out of a net the walk may cross INTO: a series path node
// carries a handful of members (fuse, bead, clamp, a cap or two, the load); a rail or
// bus carries dozens. The degree guard is what keeps a name-only rail (an EDIF "+5V"
// with no attributes) from turning a pull-up into a doorway to the whole design.
const maxWalkFan = 16

// isBusLike reports a shared-DISTRIBUTION net — one the series-reach walk must not cross INTO,
// because it is not a point-to-point series path but a plane/rail/wide fan-out that would turn a
// pull-up into a doorway to the whole design. It is BUS evidence, three ways: a ground name (a
// plane, never a series path), the global fact (a design-wide by-name rail), and rail-scale fan-out
// (> maxWalkFan). Deliberately NOT bus-like: rail-looking NAMES (power-path nodes are legitimately
// named VBUS/5V_PROT and are exactly what protection rules walk through) and the power_driven fact
// (a PWR_FLAG marks the power ENTRY nets themselves — treating them as stops would blind the walk on
// its primary use case). WS3-080 named this (was the inline walkStop) so the reach walk and the
// net.bus_like query relation share ONE definition; the walk's start net is never treated as a stop.
func isBusLike(n *ir.Net) bool {
	a := n.GetAttributes()
	return a[netgraph.AttrGlobal] == "true" ||
		IsGroundName(n.Name) || len(n.Connections) > maxWalkFan
}

// reach runs the bounded BFS over the model's pass-element adjacency.
func (m *irModel) Reach(start *ir.Net, hops int) Reach {
	r := Reach{Crossed: map[string]bool{}, Parent: map[string]ReachStep{}}
	if start == nil {
		return r
	}
	visited := map[string]bool{start.Name: true}
	frontier := []*ir.Net{start}
	r.Nets = append(r.Nets, start)
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
					if isBusLike(o) {
						continue
					}
					visited[o.Name] = true
					r.Crossed[c.ComponentRef] = true
					r.Parent[o.Name] = ReachStep{From: n.Name, Through: c.ComponentRef}
					r.Nets = append(r.Nets, o)
					next = append(next, o)
				}
			}
		}
		frontier = next
	}
	return r
}

// Between reports whether a component of the given class sits ON the series path from
// one net to another, within the hop bound. False when `to` is not reachable at all —
// unreachable is indistinguishable from unprotected for every current consumer, and
// callers that care test reachability first via Reach.
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
