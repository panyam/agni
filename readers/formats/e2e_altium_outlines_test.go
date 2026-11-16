package formats

import (
	"math"
	"testing"

	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// outlineTol is how far a block's box may sit from the export's package outline, in nanometres.
const outlineTol = 50_000

// padSlack is how far past its block a pad centre may sit, in nanometres.
const padSlack = 250_000

// bodyReach is how far past its pads a part's block may reach, in nanometres. A wrong reading of a
// rectangle's corners reaches across the board.
const bodyReach = 15_000_000

type box struct{ x0, y0, x1, y1 float64 }

func (b box) wider(x, y float64) box {
	return box{math.Min(b.x0, x), math.Min(b.y0, y), math.Max(b.x1, x), math.Max(b.y1, y)}
}

func emptyBox() box { return box{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)} }

func (b box) near(o box, tol float64) bool {
	return math.Abs(b.x0-o.x0) <= tol && math.Abs(b.y0-o.y0) <= tol && math.Abs(b.x1-o.x1) <= tol && math.Abs(b.y1-o.y1) <= tol
}

// fabBoxes is the box round each part's assembly outlines.
func fabBoxes(g *geom.BoardGeometry) map[string]box {
	out := map[string]box{}
	for _, gr := range g.GetGraphics() {
		if gr.GetRefDes() == "" || (gr.GetLayer() != "F.Fab" && gr.GetLayer() != "B.Fab") {
			continue
		}
		b, ok := out[gr.GetRefDes()]
		if !ok {
			b = emptyBox()
		}
		for _, p := range gr.GetShape().GetPoints() {
			b = b.wider(float64(p.GetX()), float64(p.GetY()))
		}
		out[gr.GetRefDes()] = b
	}
	return out
}

// blockBoxes is the box round each part's block in the stack view.
func blockBoxes(g *geom.BoardGeometry) map[string]box {
	out := map[string]box{}
	for _, body := range render.BoardStack(g).GetBodies() {
		b := emptyBox()
		ring := body.GetOutline()
		for i := 0; i+1 < len(ring); i += 2 {
			b = b.wider(ring[i], ring[i+1])
		}
		out[body.GetRefDes()] = b
	}
	return out
}

// TestAltiumBlocksStandOnTheirBodies holds each foxBMS part's block, stood on its 3D bodies' outlines,
// to the package outline Altium wrote for it in the ODB++ job it exported from the same board (agni
// 1668). Altium writes that outline from the same bodies.
func TestAltiumBlocksStandOnTheirBodies(t *testing.T) {
	held, missed := 0, 0
	for _, pr := range altiumPairs(t) {
		t.Run(pr.name, func(t *testing.T) {
			_, pg := readBoth(t, pr.pcb)
			_, og := readBoth(t, pr.odb)
			ours, theirs := blockBoxes(pg), fabBoxes(og)
			for ref, want := range theirs {
				got, ok := ours[ref]
				if !ok {
					continue
				}
				if got.near(want, outlineTol) {
					held++
				} else {
					missed++
					if missed <= 10 {
						t.Errorf("%s: block %.2f..%.2f x %.2f..%.2f mm, the job's outline %.2f..%.2f x %.2f..%.2f mm", ref,
							got.x0/1e6, got.x1/1e6, got.y0/1e6, got.y1/1e6, want.x0/1e6, want.x1/1e6, want.y0/1e6, want.y1/1e6)
					}
				}
			}
		})
	}
	t.Logf("%d blocks match the job's package outline, %d do not", held, missed)
	if held < 1000 {
		t.Errorf("%d blocks held to their outlines, want at least 1000 across foxBMS", held)
	}
}

// TestAllegroBlocksStandOnTheirPlaceBounds holds each parasdr part that states a height, whose block
// stands on the place-bound outline carrying that height, to the package outline the ODX Allegro
// exported from the board draws for it (agni 1668).
func TestAllegroBlocksStandOnTheirPlaceBounds(t *testing.T) {
	l := &Loader{}
	bg, err := l.BoardGeometry(parasdr + "parasdr_layout.brd")
	if err != nil {
		t.Fatal(err)
	}
	og, err := l.BoardGeometry(parasdr + "parasdr_odb++.xml")
	if err != nil {
		t.Fatal(err)
	}
	ours, theirs := blockBoxes(bg), fabBoxes(og)
	held, missed := 0, 0
	for _, pl := range bg.GetPlacements() {
		if pl.Height == nil {
			continue
		}
		got, ok1 := ours[pl.GetRefDes()]
		want, ok2 := theirs[pl.GetRefDes()]
		if !ok1 || !ok2 {
			continue
		}
		if got.near(want, outlineTol) {
			held++
		} else {
			missed++
			if missed <= 10 {
				t.Errorf("%s: block %.2f..%.2f x %.2f..%.2f mm, the ODX's outline %.2f..%.2f x %.2f..%.2f mm", pl.GetRefDes(),
					got.x0/1e6, got.x1/1e6, got.y0/1e6, got.y1/1e6, want.x0/1e6, want.x1/1e6, want.y0/1e6, want.y1/1e6)
			}
		}
	}
	t.Logf("%d blocks match the ODX's package outline, %d do not", held, missed)
	if held < 250 {
		t.Errorf("%d blocks held to their outlines, want most of parasdr's 284 with a height", held)
	}
}

// TestAllegro172PlaceBoundsCoverTheirPads checks the BeagleBone AI's 17.2 place-bound blocks against
// the parts they stand for, since nothing else states those outlines (agni 1668). Every pad centre of
// a part that states a height lies inside its block's box, or within padSlack of it, since an edge
// connector's pads can reach past its body. A wrong offset or turn misses by millimetres, as 43 of
// parasdr's parts did before the turn was read. And no block reaches more than bodyReach past its
// pads.
func TestAllegro172PlaceBoundsCoverTheirPads(t *testing.T) {
	g, err := (&Loader{}).BoardGeometry(allegroBoards + "/beaglebone-ai/BeagleBone-AI.brd")
	if err != nil {
		t.Fatal(err)
	}
	blocks := blockBoxes(g)
	checked := 0
	for _, pl := range g.GetPlacements() {
		b, ok := blocks[pl.GetRefDes()]
		if pl.Height == nil || !ok {
			continue
		}
		checked++
		pads := emptyBox()
		for _, pad := range pl.GetPads() {
			w := geomath.ComposePlacement(pl.GetAt(), pl.GetRotationDeg(), pl.GetMirror(), pad.GetAt())
			pads = pads.wider(float64(w.GetX()), float64(w.GetY()))
		}
		if b.x0 < pads.x0-bodyReach || b.y0 < pads.y0-bodyReach || b.x1 > pads.x1+bodyReach || b.y1 > pads.y1+bodyReach {
			t.Errorf("%s: block %.2f..%.2f x %.2f..%.2f mm reaches more than 15 mm past its pads", pl.GetRefDes(), b.x0/1e6, b.x1/1e6, b.y0/1e6, b.y1/1e6)
			continue
		}
		for _, pad := range pl.GetPads() {
			w := geomath.ComposePlacement(pl.GetAt(), pl.GetRotationDeg(), pl.GetMirror(), pad.GetAt())
			x, y := float64(w.GetX()), float64(w.GetY())
			if x < b.x0-padSlack || x > b.x1+padSlack || y < b.y0-padSlack || y > b.y1+padSlack {
				t.Errorf("%s pad %s at (%.2f, %.2f) mm lies outside its block %.2f..%.2f x %.2f..%.2f", pl.GetRefDes(), pad.GetNumber(), x/1e6, y/1e6, b.x0/1e6, b.x1/1e6, b.y0/1e6, b.y1/1e6)
				break
			}
		}
	}
	if checked < 50 {
		t.Errorf("checked %d parts, want the BeagleBone's 71 with a height", checked)
	}
}
