package graph

import (
	"sort"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// orthoPlace is the orthogonal layout (WS7-010). Node positions come from the stress strategy
// (deterministic, pitch-snapped, shortest edge lengths of the current strategies), and
// RouteOrthogonal tells assemble to draw each net as axis-aligned L-runs to a Manhattan-median hub.
// It does no node avoidance and no shared-trunk bus channels.
func orthoPlace(d *ir.Design) Placement {
	p := stressPlace(d)
	p.Route = RouteOrthogonal
	return p
}

// routeOrthogonal draws one net as L-runs, each pin routing horizontally then vertically to the hub.
// The hub is the component-wise median of the pins (the rectilinear 1-median, which minimizes total
// Manhattan wire length), nudged by a per-net track offset so nets whose hubs share a column do not
// overlap their vertical trunks. Aligned pins get a straight segment. Returns the polylines and the
// hub (label and junction anchor).
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
