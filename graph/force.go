package graph

import (
	"maps"
	"math"
	"sort"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Force-directed layout parameters. Fixed budgets keep the output deterministic (no
// convergence threshold, no RNG); the temperature schedule cools linearly so early sweeps
// make large corrections and late sweeps only settle.
const (
	forceIterations = 200
	forceIdeal      = pitch // ideal spring length k, in layout units
)

// forcePlace is the deterministic force-directed layout (WS7-008, Fruchterman-Reingold):
// all node pairs repel (k^2/d), connected nodes attract (d^2/k), iterate under a cooling
// displacement cap. Each net becomes a star through a virtual hub node (the WS7-003
// hyperedge model), so the force model stays node-node without turning a wide net into a
// clique; hubs join the simulation and are dropped before output. Init is the layered
// placement (deterministic, roughly right), components solve independently and pack left to
// right, and final positions snap to free pitch cells, all exactly as the stress strategy
// does, so compactBySize's grid-alignment contract keeps holding.
func forcePlace(d *ir.Design) Placement {
	adj := adjacency(d)
	init := layeredPlace(d).Positions

	// Net membership by component ref, to attach each component's nets' hubs.
	memberNets := map[string][]*ir.Net{}
	for _, n := range d.Nets {
		seen := map[string]bool{}
		for _, c := range n.Connections {
			if !seen[c.ComponentRef] {
				seen[c.ComponentRef] = true
				memberNets[c.ComponentRef] = append(memberNets[c.ComponentRef], n)
			}
		}
	}

	pos := map[string]*geom.Point{}
	var offsetX int64
	for _, comp := range connectedComponents(adj) {
		placed := forceComponent(comp, memberNets, init)
		snapped := snapToGrid(comp, placed)
		maxX := shiftInto(snapped, offsetX)
		maps.Copy(pos, snapped)
		offsetX = maxX + 2*pitch
	}
	return Placement{Positions: pos}
}

// forceComponent runs the FR simulation over one connected component's members plus one
// virtual hub per net touching them, returning float positions for the members only.
func forceComponent(refs []string, memberNets map[string][]*ir.Net, init map[string]*geom.Point) map[string][2]float64 {
	if len(refs) == 1 {
		return map[string][2]float64{refs[0]: {0, 0}}
	}
	inComp := map[string]bool{}
	for _, r := range refs {
		inComp[r] = true
	}

	// Simulation nodes: members (sorted), then this component's net hubs (net-name sorted).
	nodes := append([]string{}, refs...)
	hubMembers := map[string][]int{} // hub node index -> member node indexes
	netSeen := map[string]bool{}
	var netNames []string
	netByName := map[string]*ir.Net{}
	for _, r := range refs {
		for _, n := range memberNets[r] {
			if !netSeen[n.Name] {
				netSeen[n.Name] = true
				netNames = append(netNames, n.Name)
				netByName[n.Name] = n
			}
		}
	}
	sort.Strings(netNames)
	idx := map[string]int{}
	for i, r := range nodes {
		idx[r] = i
	}
	for _, name := range netNames {
		hub := len(nodes)
		nodes = append(nodes, "net:"+name)
		seen := map[int]bool{}
		for _, c := range netByName[name].Connections {
			if i, ok := idx[c.ComponentRef]; ok && inComp[c.ComponentRef] && !seen[i] {
				seen[i] = true
				hubMembers[nodes[hub]] = append(hubMembers[nodes[hub]], i)
			}
		}
	}

	n := len(nodes)
	x := make([]float64, n)
	y := make([]float64, n)
	for i, r := range refs {
		if p := init[r]; p != nil {
			x[i], y[i] = float64(p.X), float64(p.Y)
		}
		x[i] += float64(i) * 1e-3 // break exact coincidence deterministically
	}
	// Hubs start at their members' centroid.
	for i := len(refs); i < n; i++ {
		ms := hubMembers[nodes[i]]
		for _, m := range ms {
			x[i] += x[m]
			y[i] += y[m]
		}
		if len(ms) > 0 {
			x[i] /= float64(len(ms))
			y[i] /= float64(len(ms))
		}
		x[i] += float64(i) * 1e-3
	}

	// Edges: hub to each member.
	type edge struct{ a, b int }
	var edges []edge
	for i := len(refs); i < n; i++ {
		for _, m := range hubMembers[nodes[i]] {
			edges = append(edges, edge{i, m})
		}
	}

	k := float64(forceIdeal)
	for iter := range forceIterations {
		// Linear cooling: from 3*pitch of allowed movement down to (almost) none.
		temp := 3 * k * float64(forceIterations-iter) / float64(forceIterations)
		dx := make([]float64, n)
		dy := make([]float64, n)
		// Repulsion between every node pair.
		for i := range n {
			for j := i + 1; j < n; j++ {
				ddx, ddy := x[i]-x[j], y[i]-y[j]
				dd := math.Hypot(ddx, ddy)
				if dd < 1e-9 {
					ddx, ddy, dd = 1, 0, 1
				}
				f := k * k / dd / dd // force magnitude over distance, applied to the delta vector
				dx[i] += ddx * f
				dy[i] += ddy * f
				dx[j] -= ddx * f
				dy[j] -= ddy * f
			}
		}
		// Attraction along hyperedge-star edges.
		for _, e := range edges {
			ddx, ddy := x[e.a]-x[e.b], y[e.a]-y[e.b]
			dd := math.Hypot(ddx, ddy)
			if dd < 1e-9 {
				continue
			}
			f := dd / k // d^2/k force over distance d
			dx[e.a] -= ddx * f
			dy[e.a] -= ddy * f
			dx[e.b] += ddx * f
			dy[e.b] += ddy * f
		}
		// Move, capped by temperature.
		for i := range n {
			disp := math.Hypot(dx[i], dy[i])
			if disp < 1e-9 {
				continue
			}
			step := math.Min(disp, temp)
			x[i] += dx[i] / disp * step
			y[i] += dy[i] / disp * step
		}
	}

	out := make(map[string][2]float64, len(refs))
	for i, r := range refs {
		out[r] = [2]float64{x[i], y[i]}
	}
	return out
}
