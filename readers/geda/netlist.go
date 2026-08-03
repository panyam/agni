package geda

import (
	"bytes"
	"math"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
	"github.com/panyam/agni/internal/symread"
)

// gEDA coordinates are integers on a fine grid, so no scaling is needed; quant just rounds to
// int64. (A shared grid keeps this parallel to the xschem reader, which scales half-integers.)
func quant(x, y float64) netgraph.Point {
	return netgraph.Point{X: round(x), Y: round(y)}
}

func point(sx, sy string) (netgraph.Point, bool) {
	x, ok1 := atof(sx)
	y, ok2 := atof(sy)
	if !ok1 || !ok2 {
		return netgraph.Point{}, false
	}
	return quant(x, y), true
}

func round(f float64) int64 { return int64(math.Round(f)) }

// atof parses a gEDA coordinate. gEDA coordinates are integers, so this is an int parse
// widened to float64 (the shared netgraph/transform code works in float64).
func atof(s string) (float64, bool) {
	n, ok := parseInt(s)
	return float64(n), ok
}

func atoiInt(s string) int {
	n, _ := parseInt(s)
	return n
}

// loadPins adapts the gEDA symbol pipeline (open, line split, pin extraction) to the
// shared resolver; memoization lives in symread.ResolvePins. The bool reports whether the
// symbol RESOLVED (opened) — the signal the dangling-endpoint gate needs (WS1-013): a
// failed open drops the symbol's pins and would fabricate dangles.
func loadPins(open SymbolOpener) func(string) ([]symread.Pin, bool) {
	return func(symref string) ([]symread.Pin, bool) {
		data, err := open(symref)
		if err != nil {
			return nil, false
		}
		var out []symread.Pin
		for _, sp := range symbolPins(splitLines(data)) {
			out = append(out, symread.Pin{X: sp.x, Y: sp.y, Number: sp.number, Name: sp.name, Dir: pinDir(sp.pintype), Seq: sp.seq})
		}
		return out, true
	}
}

// pinDir maps a gEDA pintype onto the neutral pin-direction vocabulary (WS1-021). gEDA's
// `pwr` marks a power/ground pin, which on a placed part is a supply INPUT (the chip draws
// power); gEDA does not distinguish power_in from power_out, so a rare power-output pin
// typed `pwr` is miscategorized, tolerated because the rail's power symbol marks the net
// External (so power-input-not-driven does not false-fire on it). Unmapped types stay
// UNSPECIFIED.
func pinDir(pintype string) ir.PinDirection {
	switch pintype {
	case "in":
		return ir.PinDirection_PIN_DIRECTION_INPUT
	case "out":
		return ir.PinDirection_PIN_DIRECTION_OUTPUT
	case "io":
		return ir.PinDirection_PIN_DIRECTION_INOUT
	case "pas":
		return ir.PinDirection_PIN_DIRECTION_PASSIVE
	case "pwr":
		return ir.PinDirection_PIN_DIRECTION_POWER_IN
	}
	return ir.PinDirection_PIN_DIRECTION_UNSPECIFIED
}

// resolveSlots fills each placement's SlotPins from its symbol's slotdef table, keyed by the
// instance slot= (default "1" when absent). Only placements of a slotted symbol (one that
// carries a slotdef) get a mapping; for an unslotted symbol SlotPins stays nil and the drawn
// pin numbers stand. The symbol is opened once per distinct reference (memoized here), separate
// from the pin-geometry open in ResolvePins. slots aligns index-for-index with placements.
func resolveSlots(placements []symread.Placement, slots []string, open SymbolOpener) {
	cache := map[string]map[string][]string{}
	table := func(symref string) map[string][]string {
		if t, ok := cache[symref]; ok {
			return t
		}
		var t map[string][]string
		if data, err := open(symref); err == nil {
			t = symbolSlots(splitLines(data))
		}
		cache[symref] = t
		return t
	}
	for i := range placements {
		t := table(placements[i].Symref)
		if len(t) == 0 {
			continue
		}
		slot := slots[i]
		if slot == "" {
			slot = "1" // gEDA's default slot when a placement names none
		}
		if pins, ok := t[slot]; ok {
			placements[i].SlotPins = pins
		}
	}
}

// resolveAnchors places each power/ground tap by resolving its symbol pin, transforming it to
// the schematic grid, and naming the net: the instance net= wins, else the symbol's own net=
// attribute, else the conventional name for the symbol basename. A tap whose symbol fails to
// open is skipped (it cannot be located without its pin geometry).
func resolveAnchors(powers []powerTap, open SymbolOpener) []netgraph.Anchor {
	cache := map[string][]symbolPin{}
	netCache := map[string]string{}
	var out []netgraph.Anchor
	for _, pt := range powers {
		pins, ok := cache[pt.symref]
		if !ok {
			if data, err := open(pt.symref); err == nil {
				lines := splitLines(data)
				pins = symbolPins(lines)
				netCache[pt.symref] = symbolNet(lines)
			}
			cache[pt.symref] = pins
		}
		if len(pins) == 0 {
			continue
		}
		name := pt.instanceNet
		if name == "" {
			name = netCache[pt.symref]
		}
		if name == "" {
			name = conventionalSupply(symread.SymbolBase(pt.symref))
		}
		// A power symbol has a single connection pin.
		ax, ay := transform(pins[0].x, pins[0].y, pt.x, pt.y, pt.angle, pt.mirror)
		// A power/ground supply symbol asserts the net is a global rail fed from a supply
		// (WS1-021): mark it External, matching KiCad's power-symbol semantics (a rail
		// name whose full membership/source may lie off the read). External keeps
		// power-input-not-driven quiet on a tapped rail without the bulk-cap noise a
		// power_driven mark would add on sim-oriented designs. gEDA has no separate
		// PWR_FLAG-style driver directive, so the supply symbol carries this alone.
		out = append(out, netgraph.Anchor{At: quant(ax, ay), Label: name, External: true})
	}
	return out
}

// conventionalSupply maps a power symbol basename to its usual net name.
func conventionalSupply(sym string) string {
	switch {
	case strings.HasPrefix(sym, "gnd"):
		return "GND"
	case strings.HasPrefix(sym, "vcc"):
		return "VCC"
	case strings.HasPrefix(sym, "vdd"):
		return "VDD"
	case strings.HasPrefix(sym, "vss"):
		return "VSS"
	default:
		return ""
	}
}

// transform places a symbol-local point onto the schematic grid for a gEDA instance with
// origin (xoff,yoff), rotation angle in degrees (0/90/180/270, counter-clockwise) and mirror
// (0/1, reflection about the y-axis). gEDA mirrors first, then rotates, then translates.
func transform(px, py, xoff, yoff float64, angle, mirror int) (float64, float64) {
	if mirror == 1 {
		px = -px
	}
	var rx, ry float64
	switch ((angle % 360) + 360) % 360 {
	case 90:
		rx, ry = -py, px
	case 180:
		rx, ry = -px, -py
	case 270:
		rx, ry = py, -px
	default: // 0
		rx, ry = px, py
	}
	return rx + xoff, ry + yoff
}

// snapLabels attaches each standalone netname= text to the net whose wire endpoint it sits
// nearest to, emitting an anchor at that endpoint. A label with no wires to snap to is dropped.
func snapLabels(labels []textLabel, wires []netgraph.Wire) []netgraph.Anchor {
	if len(wires) == 0 {
		return nil
	}
	// Collect the distinct wire endpoints once.
	var pts []netgraph.Point
	seen := map[netgraph.Point]bool{}
	addPt := func(p netgraph.Point) {
		if !seen[p] {
			seen[p] = true
			pts = append(pts, p)
		}
	}
	for _, w := range wires {
		addPt(w.A)
		addPt(w.B)
	}

	var out []netgraph.Anchor
	for _, lb := range labels {
		best := pts[0]
		bestD := dist2(lb.at, pts[0])
		for _, p := range pts[1:] {
			if d := dist2(lb.at, p); d < bestD {
				best, bestD = p, d
			}
		}
		out = append(out, netgraph.Anchor{At: best, Label: lb.name})
	}
	return out
}

func dist2(a, b netgraph.Point) int64 {
	dx, dy := a.X-b.X, a.Y-b.Y
	return dx*dx + dy*dy
}

func splitLines(data []byte) []string {
	var lines []string
	for _, ln := range bytes.Split(data, []byte("\n")) {
		lines = append(lines, string(bytes.TrimRight(ln, "\r")))
	}
	return lines
}
