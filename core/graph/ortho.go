package graph

import (
	"sort"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// orthoPlace is the orthogonal layout (WS7-010): node positions come from the stress
// strategy (deterministic, pitch-snapped, shortest edge lengths of the current strategies),
// and the RouteOrthogonal style tells assemble to draw each net as axis-aligned L-runs to a
// Manhattan-median hub instead of a straight star. The first-cut scope (no node avoidance,
// no shared-trunk bus channels) is the ticket's blessed grid-routing heuristic; full
// topology-shape-metrics stays future work.
func orthoPlace(d *ir.Design) Placement {
	p := stressPlace(d)
	p.Route = RouteOrthogonal
	return p
}

// routeOrthogonal draws one net as L-runs: every pin routes horizontally then vertically to
// the hub, where the hub is the component-wise median of the pins (the rectilinear 1-median,
// which minimizes total Manhattan wire length). The hub is nudged sideways by a small
// per-net track offset so different nets whose hubs share a column do not perfectly overlap
// their vertical trunks. Aligned pins degenerate to a straight segment (no bend). Returns
// the polylines and the hub (label and junction anchor).
func routeOrthogonal(pins []*geom.Point, netIndex int) ([]*geom.Polyline, *geom.Point) {
	hub := &geom.Point{X: medianOf(pins, func(p *geom.Point) int64 { return p.X }),
		Y: medianOf(pins, func(p *geom.Point) int64 { return p.Y })}
	hub.X += int64(netIndex%5-2) * (gutter / 2) // deterministic track offset between nets

	polys := make([]*geom.Polyline, 0, len(pins))
	for _, p := range pins {
		pts := []*geom.Point{p}
		if p.X != hub.X && p.Y != hub.Y {
			pts = append(pts, &geom.Point{X: hub.X, Y: p.Y}) // horizontal leg, then the elbow
		}
		pts = append(pts, hub)
		polys = append(polys, &geom.Polyline{Points: pts})
	}
	return polys, hub
}

// medianOf is the lower median of one coordinate across the pins, the deterministic
// tie-break for even counts.
func medianOf(pins []*geom.Point, coord func(*geom.Point) int64) int64 {
	vs := make([]int64, len(pins))
	for i, p := range pins {
		vs[i] = coord(p)
	}
	sort.Slice(vs, func(i, j int) bool { return vs[i] < vs[j] })
	return vs[(len(vs)-1)/2]
}
