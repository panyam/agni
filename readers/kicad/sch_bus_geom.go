package kicad

import (
	"slices"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/netgraph"
)

// drawnBuses draws one sheet's bus trunks and entries, each named for the bus it carries (agni issue
// 1435). The name is the bus's join key, WireGeometry.net, so a click on a bus selects it and a bus
// finding lights it, in both renderers.
//
// A trunk takes its name the way the net solver names the branch under a sheet pin (sheetBusNames).
// The name is the nearest bus label along the bus, a vector `DATA[0..7]` or a group `I2C0{I2C}`, on
// a local, global or hierarchical label. So a label anywhere on a connected bus names the segments
// around it, and a trunk carrying two labelled branches names each branch for its own label. A piece
// that two different labels reach at the same distance gets no name, as the solver gives it none.
// A bus no label reaches takes the name of the nearest sheet pin's branch, and a bus with neither
// stays unnamed, as KiCad leaves it.
//
// A polyline whose pieces all take one name keeps its shape. A polyline that crosses from one
// branch to another splits where the name changes. An entry takes the name of the bus its end
// touches, so a click on the stub selects that bus.
func drawnBuses(root *node, aliases map[string][]string, src string) []*geom.WireGeometry {
	type trunk struct {
		pts  []*geom.Point
		uuid string
	}
	var trunks []trunk
	var segs []netgraph.Wire
	for _, b := range root.Children("bus") {
		pts := xyPoints(b.Child("pts"), sheetPt)
		if len(pts) == 0 {
			continue
		}
		trunks = append(trunks, trunk{pts: pts, uuid: uuidOf(b)})
		for i := 0; i+1 < len(pts); i++ {
			segs = append(segs, netgraph.Wire{A: gp(pts[i]), B: gp(pts[i+1])})
		}
	}
	type entry struct {
		pts  []*geom.Point
		uuid string
	}
	var entries []entry
	for _, e := range root.Children("bus_entry") {
		if pts := busEntryPoints(e); len(pts) > 0 {
			entries = append(entries, entry{pts: pts, uuid: uuidOf(e)})
		}
	}

	// The bus graph, split at every label, sheet pin and entry end, as sheetBusNames splits it.
	labelAt := map[netgraph.Point]string{}
	var cuts []netgraph.Point
	for _, tag := range []string{"label", "global_label", "hierarchical_label"} {
		for _, l := range root.Children(tag) {
			at := sheetPt(l.Child("at"))
			if at == nil {
				continue
			}
			cuts = append(cuts, gp(at))
			if name := unescapeName(atomOf(l.Arg(1))); isBusLabel(name, aliases) {
				labelAt[gp(at)] = name
			}
		}
	}
	pinName, _ := sheetBusNames(root, aliases)
	for p := range pinName {
		cuts = append(cuts, p)
	}
	for _, e := range entries {
		for _, p := range e.pts {
			cuts = append(cuts, gp(p))
		}
	}
	adj := map[netgraph.Point][]netgraph.Point{}
	for _, w := range splitWiresAt(segs, cuts) {
		adj[w.A] = append(adj[w.A], w.B)
		adj[w.B] = append(adj[w.B], w.A)
	}
	nameAt := func(starts ...netgraph.Point) string {
		if name, ok := nearestOf(starts, adj, labelAt); ok {
			return name
		}
		name, _ := nearestOf(starts, adj, pinName)
		return name
	}

	var out []*geom.WireGeometry
	for _, t := range trunks {
		// Each segment of the polyline, split at the cuts on it, and the name of each piece.
		var pieces [][2]netgraph.Point
		for i := 0; i+1 < len(t.pts); i++ {
			for _, w := range splitWiresAt([]netgraph.Wire{{A: gp(t.pts[i]), B: gp(t.pts[i+1])}}, cuts) {
				pieces = append(pieces, [2]netgraph.Point{w.A, w.B})
			}
		}
		names := make([]string, len(pieces))
		for i, pc := range pieces {
			names[i] = nameAt(pc[0], pc[1])
		}
		if len(slices.Compact(slices.Clone(names))) <= 1 {
			out = append(out, busWire(geom.WireGeometry_KIND_BUS, t.pts, t.uuid, firstOr(names), src))
			continue
		}
		for start := 0; start < len(pieces); {
			end := start
			for end+1 < len(pieces) && names[end+1] == names[start] {
				end++
			}
			pts := []*geom.Point{gpt(pieces[start][0])}
			for _, pc := range pieces[start : end+1] {
				pts = append(pts, gpt(pc[1]))
			}
			out = append(out, busWire(geom.WireGeometry_KIND_BUS, pts, t.uuid, names[start], src))
			start = end + 1
		}
	}
	for _, e := range entries {
		name := ""
		for _, p := range e.pts {
			if q := gp(p); onBus(q, segs) {
				name = nameAt(q)
				break
			}
		}
		out = append(out, busWire(geom.WireGeometry_KIND_BUS_ENTRY, e.pts, e.uuid, name, src))
	}
	return out
}

// nearestOf is nearestBusLabel from several starts at once, so a piece of bus is named from both its
// ends. A name found at one distance from either end wins, and two different names at that distance
// name nothing.
func nearestOf(starts []netgraph.Point, adj map[netgraph.Point][]netgraph.Point, nameAt map[netgraph.Point]string) (string, bool) {
	seen := map[netgraph.Point]bool{}
	frontier := slices.Clone(starts)
	for _, s := range starts {
		seen[s] = true
	}
	for len(frontier) > 0 {
		var found []string
		var next []netgraph.Point
		for _, p := range frontier {
			if name, ok := nameAt[p]; ok && !slices.Contains(found, name) {
				found = append(found, name)
			}
			for _, q := range adj[p] {
				if !seen[q] {
					seen[q] = true
					next = append(next, q)
				}
			}
		}
		if len(found) == 1 {
			return found[0], true
		}
		if len(found) > 1 {
			return "", false
		}
		frontier = next
	}
	return "", false
}

// onBus reports whether p lies on any bus segment, its ends included.
func onBus(p netgraph.Point, segs []netgraph.Wire) bool {
	for _, s := range segs {
		if p == s.A || p == s.B || onSegment(s.A, s.B, p) {
			return true
		}
	}
	return false
}

func gpt(p netgraph.Point) *geom.Point { return &geom.Point{X: p.X, Y: p.Y} }

func firstOr(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return names[0]
}
