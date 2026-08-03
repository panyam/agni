package render

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/kicad"
)

// boardFixtureGeometry loads the hand-authored board fixture through the real reader, so
// the golden covers reader output shapes (nm units, Y-flip, verbatim rotations), not just
// hand-built geometry.
func boardFixtureGeometry(t testing.TB) *geom.BoardGeometry {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "readers", "kicad", "testdata", "board.kicad_pcb"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, err := kicad.ReadBoardGeometry(f, "board.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBoardSVGGolden(t *testing.T) {
	goldenCompare(t, "board-fixture.svg", BoardSVG(boardFixtureGeometry(t)))
}

// TestBoardSVGStrata: the document is stratified into the classed groups the layer
// visibility CSS keys on, copper draws back before front (B.Cu group precedes F.Cu), and
// the strata carry the expected content: named copper layers, a via ring with a page-color
// drill, an outline polyline, and every ref-des label.
func TestBoardSVGStrata(t *testing.T) {
	b := boardFixtureGeometry(t)
	got := BoardSVG(b)
	for _, cls := range []string{`class="copper-front"`, `class="copper-back"`, `class="through"`, `class="edge"`, `class="labels"`, `class="zones-front"`} {
		if !strings.Contains(got, cls) {
			t.Errorf("missing stratum %s", cls)
		}
	}
	if strings.Index(got, `data-layer="B.Cu"`) > strings.Index(got, `data-layer="F.Cu"`) {
		t.Error("back copper must draw before front copper")
	}
	for _, pl := range b.GetPlacements() {
		if !strings.Contains(got, ">"+pl.GetRefDes()+"</text>") {
			t.Errorf("missing ref-des label %q", pl.GetRefDes())
		}
	}
	if !strings.Contains(got, DefaultStyle.CopperFront) || !strings.Contains(got, DefaultStyle.CopperBack) {
		t.Error("copper colors must come from the Style (C12)")
	}
	if strings.Count(got, "<polyline") < 2 {
		t.Error("outline paths missing (rect + arc polyline)")
	}
}

// TestBoardSVGEmpty: a board with no geometry still frames a sane document.
func TestBoardSVGEmpty(t *testing.T) {
	got := BoardSVG(&geom.BoardGeometry{})
	if !strings.HasPrefix(got, "<svg") || !strings.Contains(got, "</svg>") {
		t.Fatalf("empty board must still render a document, got %q", got[:min(len(got), 80)])
	}
}

// TestBoardCopperTrueWidth: copper renders at true width proportional to the real trace, not
// clamped to a fixed output-pixel floor. On a board large enough that both traces are
// sub-pixel, a 2x-wider trace must render a ~2x-wider stroke; the old fixed 0.8px floor
// clamped every sub-pixel trace to the same width (ratio 1), merging dense copper into a blob.
func TestBoardCopperTrueWidth(t *testing.T) {
	const span = 400_000_000 // 400mm: scales fine copper well below the old 0.8px floor
	b := &geom.BoardGeometry{
		Outline: &geom.BoardOutline{Paths: []*geom.Polyline{{Points: []*geom.Point{
			{X: 0, Y: 0}, {X: span, Y: 0}, {X: span, Y: span / 4}, {X: 0, Y: span / 4}, {X: 0, Y: 0},
		}}}},
		Nets: []*geom.NetCopper{{Net: "N", Segments: []*geom.TrackSegment{
			{A: &geom.Point{X: 10_000_000, Y: 50_000_000}, B: &geom.Point{X: 20_000_000, Y: 50_000_000}, Width: 100_000, Layer: "F.Cu"}, // 100um
			{A: &geom.Point{X: 10_000_000, Y: 60_000_000}, B: &geom.Point{X: 20_000_000, Y: 60_000_000}, Width: 50_000, Layer: "F.Cu"},  // 50um
		}}},
	}
	widths := copperStrokeWidths(BoardSVG(b))
	if len(widths) != 2 {
		t.Fatalf("expected 2 copper strokes, got %d (%v)", len(widths), widths)
	}
	wide, thin := widths[0], widths[1]
	if wide >= 0.8 || thin >= 0.8 {
		t.Fatalf("test traces must be sub-0.8px to exercise the floor, got %v", widths)
	}
	if r := wide / thin; r < 1.8 || r > 2.2 {
		t.Errorf("100um/50um strokes should keep the ~2:1 width ratio, got %.3f/%.3f = %.2f (a ratio near 1 means the fixed floor clamped both)", wide, thin, r)
	}
}

// copperStrokeWidths pulls the stroke-width of every <line> in the copper strata, in document
// order, so a test can assert copper is stroked at true width.
func copperStrokeWidths(svg string) []float64 {
	var out []float64
	for _, seg := range strings.Split(svg, `class="copper-`)[1:] {
		grp := seg
		if i := strings.Index(grp, `</g>`); i >= 0 {
			grp = grp[:i]
		}
		for _, ln := range strings.Split(grp, "<line")[1:] {
			const key = `stroke-width="`
			i := strings.Index(ln, key)
			if i < 0 {
				continue
			}
			rest := ln[i+len(key):]
			j := strings.IndexByte(rest, '"')
			if j < 0 {
				continue
			}
			if v, err := strconv.ParseFloat(rest[:j], 64); err == nil {
				out = append(out, v)
			}
		}
	}
	return out
}

// TestHighlightBoardSVG: the board face of the highlight contract — a net spec re-strokes
// its copper and rings its connected pads and vias; a component spec rings all of its pads;
// a pin spec rings exactly the (ref_des, pad number) land. The overlay is transparent (no
// background rect) and framed like BoardSVG.
func TestHighlightBoardSVG(t *testing.T) {
	b := boardFixtureGeometry(t)
	sig := HighlightBoardSVG(b, []*geom.HighlightSpec{{Nets: []string{"SIG"}, Color: "#00ff00"}})
	if strings.Contains(sig, "<rect") {
		t.Error("overlay must be transparent (no background rect)")
	}
	if n := strings.Count(sig, "<line"); n != 2 {
		t.Errorf("SIG has 2 routed segments, overlay drew %d lines", n)
	}
	if !strings.Contains(sig, "#00ff00") {
		t.Error("spec color must be used")
	}
	// SIG: one via + its connected pads ringed as circles with stroke (not filled).
	if n := strings.Count(sig, "<circle"); n < 2 {
		t.Errorf("SIG via + pads should ring at least 2 circles, got %d", n)
	}

	comp := HighlightBoardSVG(b, []*geom.HighlightSpec{{Components: []string{"R1"}}})
	// R1 has 2 pads (rings) + the placement origin marker (filled dot).
	if n := strings.Count(comp, "<circle"); n != 3 {
		t.Errorf("R1 highlight = %d circles, want 2 pad rings + 1 origin dot", n)
	}
	if !strings.Contains(comp, DefaultHighlightColor) {
		t.Error("colorless spec must fall back to DefaultHighlightColor")
	}

	pin := HighlightBoardSVG(b, []*geom.HighlightSpec{{Pins: []*geom.PinRef{{RefDes: "R1", Pin: "2"}}}})
	if n := strings.Count(pin, "<circle"); n != 1 {
		t.Errorf("pin highlight = %d circles, want exactly the one pad ring", n)
	}

	// Frames match: same width/height as the base document.
	base := BoardSVG(b)
	baseHead := base[:strings.Index(base, ">")]
	overlayHead := sig[:strings.Index(sig, ">")]
	for _, attr := range []string{`width="`, `height="`} {
		bi, oi := strings.Index(baseHead, attr), strings.Index(overlayHead, attr)
		bv := baseHead[bi : bi+20]
		ov := overlayHead[oi : oi+20]
		if bv != ov {
			t.Errorf("frame mismatch: base %s vs overlay %s", bv, ov)
		}
	}
}

// TestPackBoard: the packed board reuses the PackedSheet envelope — triangle-kind records
// for areas (copper quads, via disks, pads), line strips for the outline, keys joining
// copper to nets and pads to (ref_des, number, net), and group colors indexed by the board
// group constants from Style.
func TestPackBoard(t *testing.T) {
	b := boardFixtureGeometry(t)
	ps := PackBoard(b)
	if ps.GetSheetId() != "board" || len(ps.GetVertices())%8 != 0 || len(ps.GetPrimitives())%12 != 0 {
		t.Fatalf("envelope malformed: id=%q verts=%d prims=%d", ps.GetSheetId(), len(ps.GetVertices()), len(ps.GetPrimitives()))
	}
	recs := ps.GetPrimitives()
	kinds := map[uint8]int{}
	groups := map[uint8]int{}
	for i := 0; i+primRecordBytes <= len(recs); i += primRecordBytes {
		kinds[recs[i]]++
		groups[recs[i+1]]++
	}
	if kinds[primTriangles] == 0 {
		t.Error("no triangle records: copper/pads/vias must pack as filled areas")
	}
	if groups[groupBoardEdge] == 0 || groups[groupBoardCopperFront] == 0 || groups[groupBoardCopperBack] == 0 {
		t.Errorf("missing board strata in groups: %v", groups)
	}
	if groups[groupBoardHole] == 0 {
		t.Error("drilled fixture must emit page-colored hole fills")
	}
	// The fixture's silk graphics (R1 fp_line + fp_circle, the free gr_line) pack as outline
	// primitives in the silk group.
	if groups[groupBoardSilk] == 0 {
		t.Errorf("no silk graphics packed: %v", groups)
	}
	// Keys: SIG copper joins by net; R1's pads join by (ref_des, number).
	var sigCopper, r1Pads int
	for _, k := range ps.GetKeys() {
		if k.GetNet() == "SIG" && k.GetRefDes() == "" {
			sigCopper++
		}
		if k.GetRefDes() == "R1" && k.GetPin() != "" {
			r1Pads++
		}
	}
	if sigCopper < 2 || r1Pads != 2 {
		t.Errorf("keys: SIG copper=%d (want >=2), R1 pads=%d (want 2)", sigCopper, r1Pads)
	}
	if got := ps.GetGroupColors(); len(got) != boardGroupCount || got[groupBoardCopperFront] != DefaultStyle.CopperFront ||
		got[groupBoardHole] != DefaultStyle.Page || got[groupBoardSilk] != DefaultStyle.Silk {
		t.Errorf("group colors not Style-indexed: %v", got)
	}
	// The highlight join works unchanged over the board keys.
	hl := HighlightPacked(ps, []*geom.HighlightSpec{{Nets: []string{"SIG"}}})
	if len(hl.GetGroups()) != 1 || len(hl.GetGroups()[0].GetPrimitives()) < 3 {
		t.Errorf("HighlightPacked over board keys: %+v", hl.GetGroups())
	}
	// Silk labels are the authored ref-des AND value of each real placement (the REF**
	// placeholder and the Reference-less logo are skipped upstream): R1/10k and J1/CONN.
	labelText := map[string]bool{}
	for _, l := range ps.GetLabels() {
		labelText[l.GetText()] = true
	}
	if len(ps.GetLabels()) != 5 {
		t.Errorf("labels = %d, want 5 (ref-des + value for R1 and J1, plus the gr_text title); got %v", len(ps.GetLabels()), labelText)
	}
	for _, want := range []string{"R1", "10k", "J1", "CONN", "Demo Board"} {
		if !labelText[want] {
			t.Errorf("packed board labels missing %q; got %v", want, labelText)
		}
	}
}
