package xschem

import (
	"bytes"
	"math"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
	"github.com/panyam/agni/internal/symread"
)

// gridScale quantizes xschem coordinates onto an integer grid for netgraph. xschem symbol pin
// boxes use half-integer coordinates (their centres are integers), and wire endpoints are
// integers, so scaling by 2 makes every connection point an exact integer.
const gridScale = 2

// quant maps a native (x,y) to a netgraph grid point.
func quant(x, y float64) netgraph.Point {
	return netgraph.Point{X: round(x * gridScale), Y: round(y * gridScale)}
}

// pinDir maps an xschem pin `dir` attribute onto the neutral pin-direction vocabulary
// (WS1-021). xschem's dir is in/out/inout only — it has no power-pin concept — so
// power-input-not-driven stays N/A on xschem, but the enrichment makes floating-input and
// output-output-conflict reachable. Unmapped values stay UNSPECIFIED.
func pinDir(dir string) ir.PinDirection {
	switch dir {
	case "in":
		return ir.PinDirection_PIN_DIRECTION_INPUT
	case "out":
		return ir.PinDirection_PIN_DIRECTION_OUTPUT
	case "inout":
		return ir.PinDirection_PIN_DIRECTION_INOUT
	}
	return ir.PinDirection_PIN_DIRECTION_UNSPECIFIED
}

// geomDangles maps solver dangling endpoints back from the netgraph grid to the geometry
// frame the viewer draws (WS1-013): xschem scales native coordinates by gridScale for the
// grid, but geometry is native (UnitNm 1), so a dangle at grid (X,Y) is at native
// (X/gridScale, Y/gridScale) — exact, since wire endpoints are integers pre-scale.
func geomDangles(ds []netgraph.Dangle, src string) []*ir.DanglingEndpoint {
	out := make([]*ir.DanglingEndpoint, 0, len(ds))
	for _, d := range ds {
		out = append(out, &ir.DanglingEndpoint{
			X:    d.At.X / gridScale,
			Y:    d.At.Y / gridScale,
			Prov: &ir.Provenance{SourceFile: src},
		})
	}
	return out
}

// point parses two coordinate words into a grid point.
func point(sx, sy string) (netgraph.Point, bool) {
	x, ok1 := atoi(sx)
	y, ok2 := atoi(sy)
	if !ok1 || !ok2 {
		return netgraph.Point{}, false
	}
	return quant(x, y), true
}

func round(f float64) int64 { return int64(math.Round(f)) }

// atoiInt parses a small integer field (rotation 0-3, flip 0/1); a bad value is 0.
func atoiInt(s string) int {
	f, ok := atoi(s)
	if !ok {
		return 0
	}
	return int(f)
}

// loadPins adapts the xschem symbol pipeline (open, s-expr-ish parse, pin boxes) to the
// shared resolver; memoization lives in symread.ResolvePins. The bool reports whether the
// symbol RESOLVED (opened and parsed) — the signal the dangling-endpoint gate needs
// (WS1-013): a failed open drops the symbol's pins and would fabricate dangles.
func loadPins(open SymbolOpener) func(string) ([]symread.Pin, bool) {
	return func(symref string) ([]symread.Pin, bool) {
		data, err := open(symref)
		if err != nil {
			return nil, false
		}
		objs, err := parse(bytes.NewReader(data))
		if err != nil {
			return nil, false
		}
		var out []symread.Pin
		for _, sp := range symbolPins(objs) {
			out = append(out, symread.Pin{X: sp.x, Y: sp.y, Number: sp.number, Name: sp.name, Dir: pinDir(sp.dir)})
		}
		return out, true
	}
}

// transform places a symbol-local point onto the schematic grid for an xschem instance with
// origin (xoff,yoff), rotation rot (0-3, in 90-degree CCW steps) and flip (0/1, mirror about
// the y-axis). xschem applies the flip first, then the rotation, then the translation. Verified
// against a real schematic: res.sym pin (0,-30) under "150 -460 rot=3 flip=1" lands at
// (120,-460), exactly on the wire it is drawn to.
func transform(px, py, xoff, yoff float64, rot, flip int) (float64, float64) {
	if flip == 1 {
		px = -px
	}
	var rx, ry float64
	switch rot & 3 {
	case 0:
		rx, ry = px, py
	case 1:
		rx, ry = -py, px
	case 2:
		rx, ry = -px, -py
	case 3:
		rx, ry = py, -px
	}
	return rx + xoff, ry + yoff
}
