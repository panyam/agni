package check

import (
	"sort"

	"github.com/panyam/agni/core/ident"
)

// ReachablePin is a pin a trace from the start pin routes to within the radius.
type ReachablePin struct {
	TraceEnd
	// Depth is the series crossings between the start pin's net and this pin's, 0 on the start net.
	Depth int `json:"depth"`
}

// ReachedRail is a bus-like net the walk reached, such as a ground or a supply. A trace may end on
// one but never crosses it, and its pins are counted rather than listed, since a ground on a real
// board carries hundreds of them.
type ReachedRail struct {
	Net   string `json:"net"`
	Depth int    `json:"depth"`
	Pins  int    `json:"pins"`
}

// Reachable answers what a trace from one pin can reach within a radius (agni issue 1541).
type Reachable struct {
	From TraceEnd `json:"from"`
	// Reason says why the start pin did not resolve, as a trace's unresolved outcome does. Pins and
	// Rails are empty then.
	Reason string         `json:"reason,omitempty"`
	Radius int            `json:"radius"`
	Pins   []ReachablePin `json:"pins,omitempty"`
	Rails  []ReachedRail  `json:"rails,omitempty"`
}

// ReachablePins lists the pins a trace from `from` routes to within hops series crossings, nearest
// first. It walks as TracePins does (ReachToTerminus), so every listed pin routes at that radius, and
// every pin that routes and sits on no rail is listed. A reached rail is named with its pin count
// instead. The start pin is not listed, and a virtual part (#PWR01) has no pin to trace to. hops of
// 0 or less searches DefaultTraceHops, as a trace does.
func ReachablePins(m Model, from Endpoint, hops int) Reachable {
	if hops <= 0 {
		hops = DefaultTraceHops
	}
	out := Reachable{Radius: hops}
	start, end, reason := resolveEndpoint(m, from)
	out.From = end
	if reason != "" {
		out.Reason = reason
		return out
	}
	r := m.ReachToTerminus(start, hops)
	for _, n := range r.Nets {
		depth := r.Depth[n.Name]
		if IsBusLike(m, n) {
			out.Rails = append(out.Rails, ReachedRail{Net: n.Name, Depth: depth, Pins: len(n.Connections)})
			continue
		}
		for _, c := range n.Connections {
			e := Endpoint{RefDes: c.ComponentRef, Pin: c.PinRef}
			if e == end.Endpoint || IsVirtualRef(e.RefDes) {
				continue
			}
			p := ReachablePin{TraceEnd: TraceEnd{Endpoint: e, Net: n.Name}, Depth: depth}
			if pn := m.PinName(e.RefDes, e.Pin); pn != "~" {
				p.PinName = pn
			}
			out.Pins = append(out.Pins, p)
		}
	}
	sort.SliceStable(out.Pins, func(i, j int) bool {
		a, b := out.Pins[i], out.Pins[j]
		if a.Depth != b.Depth {
			return a.Depth < b.Depth
		}
		if a.RefDes != b.RefDes {
			return ident.CompareNatural(a.RefDes, b.RefDes) < 0
		}
		return ident.CompareNatural(a.Pin, b.Pin) < 0
	})
	sort.SliceStable(out.Rails, func(i, j int) bool { return out.Rails[i].Depth < out.Rails[j].Depth })
	return out
}
