package formats

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// allegroBoards holds the Allegro boards fetched as the allegro samples artifact (CC-BY-4.0,
// agni-samples PROVENANCE.md). Each sits beside an extract Allegro wrote from the same board, which
// the reader is held to.
const allegroBoards = "../../tools/samples/allegro"

// milNm is nanometres per mil, the extract's unit.
const milNm = 25400

// readAllegroSample reads the BeagleBone AI board both ways and parses its extract.
func readAllegroSample(t *testing.T) (*ir.Design, *geom.BoardGeometry, *allegroExtract) {
	t.Helper()
	dir := filepath.Join(allegroBoards, "beaglebone-ai")
	brd := filepath.Join(dir, "BeagleBone-AI.brd")
	if _, err := os.Stat(brd); err != nil {
		// Fatal rather than skipped: an absent corpus must not read as a pass.
		t.Fatalf("no Allegro board at %s; run `make samples-oracle`", brd)
	}
	l := &Loader{}
	d, err := l.ReadDesign(brd)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	g, err := l.BoardGeometry(brd)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	return d, g, readAllegroExtract(t, filepath.Join(dir, "cds2f_BeagleBone-AI.txt"))
}

// TestAllegroNetlistMatchesItsExtract holds the netlist to the extract: the same pins on the same
// nets, compared as a partition, and the same components with the same values.
func TestAllegroNetlistMatchesItsExtract(t *testing.T) {
	d, _, x := readAllegroSample(t)

	want := map[string][]string{}
	for _, p := range x.pins {
		if p.net != "" {
			want[p.net] = append(want[p.net], p.ref+"."+p.number)
		}
	}
	got := map[string][]string{}
	for _, n := range d.GetNets() {
		for _, c := range n.GetConnections() {
			got[n.GetName()] = append(got[n.GetName()], c.GetComponentRef()+"."+c.GetPinRef())
		}
	}
	if len(want) < 500 {
		t.Fatalf("extract lists %d nets, want the BeagleBone AI's 595", len(want))
	}
	if len(got) != len(want) {
		t.Errorf("read %d nets, extract has %d", len(got), len(want))
	}
	pins := 0
	for net, ps := range want {
		sort.Strings(ps)
		g := append([]string(nil), got[net]...)
		sort.Strings(g)
		if strings.Join(g, " ") != strings.Join(ps, " ") {
			t.Errorf("net %s: read %v, extract %v", net, g, ps)
		}
		pins += len(ps)
	}
	if pins < 2500 {
		t.Fatalf("compared %d pins, want the extract's 2,788 on named nets", pins)
	}

	byRef := map[string]*ir.Component{}
	for _, c := range d.GetComponents() {
		byRef[c.GetRefDes()] = c
	}
	if len(byRef) != len(x.comps) {
		t.Errorf("read %d components, extract has %d", len(byRef), len(x.comps))
	}
	for ref, xc := range x.comps {
		c := byRef[ref]
		if c == nil {
			t.Errorf("%s: in the extract, not read", ref)
			continue
		}
		if got := c.GetAttributes()["Value"]; got != xc.value {
			t.Errorf("%s: value %q, extract %q", ref, got, xc.value)
		}
		if c.GetFootprintRef() != xc.sym {
			t.Errorf("%s: footprint %q, extract %q", ref, c.GetFootprintRef(), xc.sym)
		}
	}
}

// TestAllegroBoardMatchesItsExtract holds the board geometry to the extract: each placement's origin,
// angle and side, every pin's position on the board once the contract's transform is applied, and
// every via's position and net.
func TestAllegroBoardMatchesItsExtract(t *testing.T) {
	_, g, x := readAllegroSample(t)

	near := func(nm int64, mils float64) bool { return math.Abs(float64(nm)-mils*milNm) <= 30 }
	placed := 0
	pins := 0
	for _, pl := range g.GetPlacements() {
		xc, ok := x.comps[pl.GetRefDes()]
		if !ok {
			t.Errorf("%s: placed, not in the extract", pl.GetRefDes())
			continue
		}
		placed++
		if !near(pl.GetAt().GetX(), xc.x) || !near(pl.GetAt().GetY(), xc.y) {
			t.Errorf("%s: at (%d, %d) nm, extract (%.3f, %.3f) mil", pl.GetRefDes(), pl.GetAt().GetX(), pl.GetAt().GetY(), xc.x, xc.y)
		}
		if pl.GetMirror() != xc.mirror {
			t.Errorf("%s: mirror %v, extract %v", pl.GetRefDes(), pl.GetMirror(), xc.mirror)
		}
		// The contract rotates and then mirrors; Allegro mirrors and then rotates, so a bottom
		// placement's angle is the extract's negated.
		wantRot := xc.rotDeg
		if xc.mirror {
			wantRot = math.Mod(360-wantRot, 360)
		}
		if math.Abs(pl.GetRotationDeg()-wantRot) > 1e-6 {
			t.Errorf("%s: rotation %g, want %g (extract %g, mirror %v)", pl.GetRefDes(), pl.GetRotationDeg(), wantRot, xc.rotDeg, xc.mirror)
		}
		for _, pad := range pl.GetPads() {
			want, ok := x.pinAt[pl.GetRefDes()+"."+pad.GetNumber()]
			if !ok {
				continue
			}
			wx, wy := boardPoint(pl, pad.GetAt())
			if math.Abs(wx-want[0]*milNm) > 60 || math.Abs(wy-want[1]*milNm) > 60 {
				t.Errorf("%s.%s: on the board at (%.0f, %.0f) nm, extract (%.3f, %.3f) mil", pl.GetRefDes(), pad.GetNumber(), wx, wy, want[0], want[1])
			}
			pins++
		}
	}
	if placed < 700 || pins < 3000 {
		t.Fatalf("checked %d placements and %d pins, want the BeagleBone AI's 716 and about 3,135", placed, pins)
	}

	// Every track width read must be one the extract lists, which holds the unit conversion.
	widths := 0
	for _, nc := range g.GetNets() {
		for _, s := range nc.GetSegments() {
			if !x.widths[int64(math.Round(float64(s.GetWidth())*1000/milNm))] {
				t.Errorf("net %s: a track %d nm wide, a width the extract never lists", nc.GetNet(), s.GetWidth())
			}
			widths++
		}
	}
	if widths < 10000 || len(x.widths) < 5 {
		t.Fatalf("checked %d track segments against %d extract widths", widths, len(x.widths))
	}

	type via struct {
		x, y int64
		net  string
	}
	key := func(xnm, ynm int64, net string) via { return via{xnm / 1000, ynm / 1000, net} } // micrometres
	want := map[via]int{}
	for _, v := range x.vias {
		want[key(int64(math.Round(v.x*milNm)), int64(math.Round(v.y*milNm)), v.net)]++
	}
	got := map[via]int{}
	n := 0
	for _, nc := range g.GetNets() {
		for _, v := range nc.GetVias() {
			got[key(v.GetAt().GetX(), v.GetAt().GetY(), nc.GetNet())]++
			n++
		}
	}
	if len(x.vias) < 1500 {
		t.Fatalf("extract lists %d vias, want the BeagleBone AI's 1,789", len(x.vias))
	}
	if n != len(x.vias) {
		t.Errorf("read %d vias, extract has %d", n, len(x.vias))
	}
	for k, c := range want {
		if got[k] != c {
			t.Errorf("via at (%d, %d) um on %s: read %d, extract %d", k.x, k.y, k.net, got[k], c)
		}
	}
}

// boardPoint composes a footprint-local point onto the board the way the contract defines it:
// world = at + M(R(rotation) * local), with M mirroring X when the placement is mirrored.
func boardPoint(pl *geom.ComponentPlacement, local *geom.Point) (float64, float64) {
	a := pl.GetRotationDeg() * math.Pi / 180
	x := float64(local.GetX())*math.Cos(a) - float64(local.GetY())*math.Sin(a)
	y := float64(local.GetX())*math.Sin(a) + float64(local.GetY())*math.Cos(a)
	if pl.GetMirror() {
		x = -x
	}
	return float64(pl.GetAt().GetX()) + x, float64(pl.GetAt().GetY()) + y
}

// TestAllegroPadstacksMatchTheExtract holds every padstack the board read to the rows Allegro's
// extract lists for it, layer by layer: the regular pad, the thermal relief and the antipad, by
// shape and by size (agni 1356). The extract names the copper layers by their Allegro names, in
// stackup order, between the technical rows, so the n-th copper row is the board's n-th layer.
func TestAllegroPadstacksMatchTheExtract(t *testing.T) {
	_, g, x := readAllegroSample(t)
	byID := map[string]*geom.Padstack{}
	for _, ps := range g.GetPadstacks() {
		byID[ps.GetId()] = ps
	}
	// The extract's shape words, as the reader spells them.
	word := map[string]string{"CIRCLE": "circle", "SQUARE": "rect", "RECTANGLE": "rect", "OBLONG_X": "oval", "OBLONG_Y": "oval", "SHAPE": "custom", "OCTAGON": "octagon"}
	sizeOK := func(got *geom.Point, w, h float64) bool {
		return got != nil && math.Abs(float64(got.GetX())-w*25400) <= 1000 && math.Abs(float64(got.GetY())-h*25400) <= 1000
	}
	check := func(what string, got *geom.PadShape, want extractPadShape) string {
		if want.shape == "" {
			if got != nil {
				return fmt.Sprintf("%s %s %v, the extract has none", what, got.GetShape(), got.GetSize())
			}
			return ""
		}
		if got == nil {
			return fmt.Sprintf("%s missing, the extract has %s %gx%g", what, want.shape, want.w, want.h)
		}
		if got.GetShape() != word[want.shape] || !sizeOK(got.GetSize(), want.w, want.h) {
			return fmt.Sprintf("%s %s %d×%d nm, the extract has %s %gx%g mil", what, got.GetShape(), got.GetSize().GetX(), got.GetSize().GetY(), want.shape, want.w, want.h)
		}
		return ""
	}
	layer := map[string]int{} // the copper row's position within its padstack
	var compared, stacks int
	seen := map[string]bool{}
	for _, r := range x.padRows {
		if r.layer == "internal_pad_def" || strings.HasPrefix(r.layer, "~") {
			continue
		}
		i := layer[r.padstack]
		layer[r.padstack]++
		ps := byID[r.padstack]
		if ps == nil {
			continue // a padstack no pad or via on the board uses
		}
		if !seen[r.padstack] {
			seen[r.padstack] = true
			stacks++
		}
		if i >= len(g.GetLayers()) {
			t.Fatalf("%s lists more copper rows than the board's %d layers", r.padstack, len(g.GetLayers()))
		}
		name := g.GetLayers()[i].GetName()
		var got *geom.PadstackLayer
		for _, l := range ps.GetLayers() {
			if l.GetLayer() == name {
				got = l
			}
		}
		if got == nil {
			got = &geom.PadstackLayer{}
		}
		for _, msg := range []string{check("pad", got.GetPad(), r.pad), check("thermal", got.GetThermal(), r.thermal), check("antipad", got.GetAntipad(), r.antipad)} {
			if msg != "" {
				t.Errorf("%s on %s (%s): %s", r.padstack, name, r.layer, msg)
			}
		}
		compared++
	}
	t.Logf("compared %d copper rows of %d padstacks", compared, stacks)
	if stacks < 50 || compared < 500 {
		t.Fatalf("compared %d rows of %d padstacks, too few to mean anything", compared, stacks)
	}
}
