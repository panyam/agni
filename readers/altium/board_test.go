package altium

import (
	"math"
	"strconv"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// Altium stores pads at board positions and angles, and the contract keeps them footprint-local.
// Whatever angle and side a part has, composing a pad back through the contract's transform must land
// it where Altium put it, at the angle Altium gave it. The foxBMS boards only have bottom parts at 180
// degrees, so this covers the other angles (agni issue 1392).
func TestEveryPadComposesBackToItsBoardPosition(t *testing.T) {
	type part struct {
		side string
		rot  float64
	}
	parts := []part{{"TOP", 0}, {"TOP", 37}, {"TOP", 270}, {"BOTTOM", 90}, {"BOTTOM", 180}, {"BOTTOM", 270}, {"BOTTOM", 333}}
	p := &pcb{board: props{"ORIGINX": "10mil", "ORIGINY": "20mil"}, stack: []int{layerTop, layerBottom}}
	// Each part sits at (1000 + 100i, 500) mil with two pads off its centre.
	offsets := [][2]int64{{300000, 0}, {-120000, 450000}}
	for i, pt := range parts {
		cx, cy := int64(10000000+1000000*i), int64(5000000)
		p.comps = append(p.comps, comp{ref: "U" + strconv.Itoa(i), p: props{
			"LAYER": pt.side, "X": strconv.Itoa(int(cx/10000)) + "mil", "Y": strconv.Itoa(int(cy/10000)) + "mil",
			"ROTATION": strconv.FormatFloat(pt.rot, 'f', -1, 64),
		}})
		for j, o := range offsets {
			layer := byte(layerTop)
			if pt.side == "BOTTOM" {
				layer = layerBottom
			}
			p.pads = append(p.pads, pad{
				name: strconv.Itoa(j + 1), comp: i, net: -1, layer: layer,
				x: cx + o[0], y: cy + o[1], w: 40000, h: 20000, shape: 2, rot: 15,
			})
		}
	}
	g, err := p.toBoard("parts.PcbDoc")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.GetPlacements()) != len(parts) {
		t.Fatalf("placed %d parts, want %d", len(g.GetPlacements()), len(parts))
	}
	checked := 0
	for i, pl := range g.GetPlacements() {
		for j, pd := range pl.GetPads() {
			x, y := compose(pl, pd.GetAt())
			src := p.pads[2*i+j]
			wx, wy := float64(nm(src.x-100000)), float64(nm(src.y-200000)) // less the 10, 20 mil origin
			if math.Abs(x-wx) > 2 || math.Abs(y-wy) > 2 {
				t.Errorf("%s pad %s (%s at %g): composes to (%.0f, %.0f) nm, Altium has (%.0f, %.0f)",
					pl.GetRefDes(), pd.GetNumber(), parts[i].side, parts[i].rot, x, y, wx, wy)
			}
			// The contract draws a pad at the placement's angle plus its own, negated on a mirrored
			// placement. That has to be the angle Altium stored.
			drawn := pl.GetRotationDeg() + pd.GetRotationDeg()
			if pl.GetMirror() {
				drawn = -drawn
			}
			if d := math.Mod(drawn-src.rot+720, 360); d > 1e-6 && d < 360-1e-6 {
				t.Errorf("%s pad %s draws at %g degrees, Altium has %g", pl.GetRefDes(), pd.GetNumber(), drawn, src.rot)
			}
			checked++
		}
	}
	if checked != 2*len(parts) {
		t.Fatalf("checked %d pads", checked)
	}
}

// compose is the contract's placement transform: world = at + M(R(rotation) * local), with M
// mirroring X on a mirrored placement.
func compose(pl *geom.ComponentPlacement, local *geom.Point) (float64, float64) {
	s, c := math.Sincos(pl.GetRotationDeg() * math.Pi / 180)
	x := float64(local.GetX())*c - float64(local.GetY())*s
	y := float64(local.GetX())*s + float64(local.GetY())*c
	if pl.GetMirror() {
		x = -x
	}
	return float64(pl.GetAt().GetX()) + x, float64(pl.GetAt().GetY()) + y
}
