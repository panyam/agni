package kicad

import (
	"bytes"
	"math"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

func readBoardFixture(t *testing.T) *geom.BoardGeometry {
	t.Helper()
	g, err := ReadBoardGeometry(bytes.NewReader(readFixture(t, "board.kicad_pcb")), "board.kicad_pcb")
	if err != nil {
		t.Fatalf("ReadBoardGeometry: %v", err)
	}
	return g
}

func TestBoardGeometryPlacements(t *testing.T) {
	g := readBoardFixture(t)
	if g.DesignRef != "Board Geom Fixture" || g.UnitNm != 1 {
		t.Errorf("design_ref=%q unit_nm=%d", g.DesignRef, g.UnitNm)
	}
	if len(g.Placements) != 2 {
		t.Fatalf("placements = %d, want 2 (the Reference-less logo footprint is skipped)", len(g.Placements))
	}
	byRef := map[string]*geom.ComponentPlacement{}
	for _, p := range g.Placements {
		byRef[p.RefDes] = p
	}

	r1 := byRef["R1"]
	// (at 10 10 90): mm -> nm, Y flipped. KiCad's angle is counter-clockwise as drawn, and the Y
	// flip keeps the picture, so it carries over unchanged (agni issue 1261: kicad-cli's position
	// export says 90 for R1, and its ODB++ export puts pad 1 at (10,-11)mm). F.Cu so not mirrored.
	if r1.At.X != 10_000_000 || r1.At.Y != -10_000_000 || r1.RotationDeg != 90 || r1.Layer != "F.Cu" || r1.Mirror {
		t.Errorf("R1 placement = %+v", r1)
	}
	if len(r1.Pads) != 2 {
		t.Fatalf("R1 pads = %d, want 2", len(r1.Pads))
	}
	p1 := r1.Pads[0]
	// Pad positions stay footprint-local: (at -1 0) -> (-1mm, 0), not board coordinates.
	if p1.Number != "1" || p1.At.X != -1_000_000 || p1.At.Y != 0 || p1.Net != "SIG" || p1.Shape != "roundrect" {
		t.Errorf("R1 pad 1 = %+v", p1)
	}
	if p1.Size.X != 1_200_000 || p1.Size.Y != 1_400_000 || p1.Drill != 0 {
		t.Errorf("R1 pad 1 size/drill = %+v drill=%d", p1.Size, p1.Drill)
	}
	if r1.Pads[1].RotationDeg != 90 || r1.Pads[1].Net != "GND" {
		t.Errorf("R1 pad 2 = %+v", r1.Pads[1])
	}

	j1 := byRef["J1"]
	// B.Cu: rotation 0 negates to 0, and the back side sets mirror (WS1-030).
	if j1.Layer != "B.Cu" || j1.RotationDeg != 0 || !j1.Mirror {
		t.Errorf("J1 placement = %+v", j1)
	}
	if j1.Pads[0].Drill != 1_000_000 || len(j1.Pads[0].Layers) != 2 || j1.Pads[0].Layers[0] != "*.Cu" {
		t.Errorf("J1 pad 1 = %+v", j1.Pads[0])
	}
	if j1.Pads[1].Net != "" {
		t.Errorf("J1 pad 2 net = %q, want unconnected", j1.Pads[1].Net)
	}
}

func TestBoardGeometryText(t *testing.T) {
	g := readBoardFixture(t)
	byKind := map[string]*geom.BoardText{}
	for _, tx := range g.Texts {
		byKind[tx.Kind+"|"+tx.Text] = tx
	}
	// R1 (footprint at 10,10 rot 90): its ref-des and value are composed to board
	// coordinates. Reference local (0,-2) sits above the part in KiCad's Y-down frame, and KiCad
	// turns it 90 degrees counter-clockwise to the left (x' = x cos + y sin = -2): x = 10mm - 2mm,
	// y = -10mm (agni issue 1261).
	ref := byKind["reference|R1"]
	if ref == nil || ref.RefDes != "R1" || ref.Mirror {
		t.Fatalf("R1 reference text = %+v", ref)
	}
	// The footprint is at 90 deg; -(90) folds to +90 under keep_upright so the ref-des reads
	// bottom-to-top rather than upside down.
	if ref.At.X != 8_000_000 || ref.At.Y != -10_000_000 || ref.RotationDeg != 90 {
		t.Errorf("R1 reference placement = %+v (want 8mm,-10mm, rot 90 keep-upright)", ref)
	}
	if v := byKind["value|10k"]; v == nil || v.RefDes != "R1" {
		t.Errorf("R1 value text = %+v, want kind=value text=10k ref=R1", v)
	}
	// The REF** placeholder footprint and the Reference-less logo contribute no text.
	for k := range byKind {
		if k == "reference|REF**" {
			t.Errorf("placeholder ref-des must not emit silk text")
		}
	}
	// J1 is on B.Cu: its silk text is marked mirrored (back side).
	if j := byKind["reference|J1"]; j == nil || !j.Mirror {
		t.Errorf("J1 reference text = %+v, want mirror=true (B.Cu)", j)
	}
	// gr_text: absolute board coordinates (no placement compose), justify mirror -> mirror,
	// authored text kept verbatim (the reader stores the raw string; flattening the newline
	// is the single-line renderers' job).
	var title *geom.BoardText
	for _, tx := range g.Texts {
		if tx.Kind == "gr" {
			title = tx
		}
	}
	if title == nil || title.RefDes != "" || !title.Mirror || title.Height != 2_000_000 {
		t.Fatalf("gr_text title = %+v, want absolute mirror text h=2mm", title)
	}
	if !strings.Contains(title.Text, "Demo") || !strings.Contains(title.Text, "Board") {
		t.Errorf("gr_text = %q, want the two-line Demo/Board title", title.Text)
	}
	if title.At.X != 15_000_000 || title.At.Y != -3_000_000 {
		t.Errorf("gr_text position = %+v, want (15mm,-3mm) absolute (no placement compose)", title.At)
	}
}

func TestBoardGeometryGraphics(t *testing.T) {
	g := readBoardFixture(t)
	// Three graphics: R1's fp_line and fp_circle (composed to board frame) and the free
	// gr_line. The Edge.Cuts outline (gr_rect + gr_arc) is BoardOutline, NOT a graphic.
	if len(g.Graphics) != 3 {
		t.Fatalf("graphics = %d, want 3 (R1 fp_line + fp_circle + free gr_line; Edge.Cuts excluded)", len(g.Graphics))
	}
	var line, circle, free *geom.BoardGraphic
	for _, gr := range g.Graphics {
		switch {
		case gr.RefDes == "R1" && gr.Shape.Kind == geom.Shape_KIND_CIRCLE:
			circle = gr
		case gr.RefDes == "R1":
			line = gr
		case gr.RefDes == "":
			free = gr
		}
	}
	// R1 fp_line: local (-1.5,-0.7)-(1.5,-0.7) rotated by the footprint's 90 deg -> a vertical
	// segment at x = 10mm - 0.7mm (KiCad's x' = y = -0.7), spanning the rotated endpoints. Composed the same way pads
	// and text are, so it lands on the part; layer and width are carried verbatim.
	if line == nil || line.Layer != "F.SilkS" || line.Width != 120_000 || len(line.Shape.Points) != 2 {
		t.Fatalf("R1 fp_line = %+v", line)
	}
	// Composed with the 90 deg footprint the segment is vertical at x = 9.3mm, from KiCad's y 11.5
	// to 8.5; allow a 2nm tolerance for the shared composer's truncation (same int64 truncation
	// padWorld uses).
	near := func(got, want int64) bool { d := got - want; return d >= -2 && d <= 2 }
	if !near(line.Shape.Points[0].X, 9_300_000) || line.Shape.Points[0].Y != -11_500_000 ||
		!near(line.Shape.Points[1].X, 9_300_000) || line.Shape.Points[1].Y != -8_500_000 {
		t.Errorf("R1 fp_line points = %+v, want vertical at x=9.3mm (composed with the 90 deg footprint)", line.Shape.Points)
	}
	// R1 fp_circle: center composed to the footprint origin; radius is rotation-invariant.
	if circle == nil || circle.Shape.Radius != 500_000 || circle.Shape.Points[0].X != 10_000_000 || circle.Shape.Points[0].Y != -10_000_000 {
		t.Errorf("R1 fp_circle = %+v, want center (10mm,-10mm) r=0.5mm", circle)
	}
	// Free gr_line: absolute board coordinates, no owning ref-des.
	if free == nil || free.Layer != "F.SilkS" || free.RefDes != "" ||
		free.Shape.Points[0].X != 2_000_000 || free.Shape.Points[0].Y != -6_000_000 {
		t.Errorf("free gr_line = %+v, want absolute (2mm,-6mm) with empty ref_des", free)
	}
}

func TestBoardGeometryCopper(t *testing.T) {
	g := readBoardFixture(t)
	if len(g.Nets) != 2 || g.Nets[0].Net != "GND" || g.Nets[1].Net != "SIG" {
		t.Fatalf("nets = %+v, want [GND SIG] (net-0 copper dropped, sorted by name)", g.Nets)
	}
	gnd, sig := g.Nets[0], g.Nets[1]
	if len(gnd.Segments) != 1 || len(gnd.Vias) != 0 {
		t.Errorf("GND copper = %d segments %d vias", len(gnd.Segments), len(gnd.Vias))
	}
	if len(sig.Segments) != 2 || len(sig.Vias) != 1 {
		t.Fatalf("SIG copper = %d segments %d vias", len(sig.Segments), len(sig.Vias))
	}
	s := sig.Segments[0]
	if s.A.X != 9_000_000 || s.A.Y != -10_000_000 || s.Width != 250_000 || s.Layer != "F.Cu" {
		t.Errorf("SIG segment = %+v", s)
	}
	v := sig.Vias[0]
	if v.At.X != 15_000_000 || v.At.Y != -12_000_000 || v.Size != 800_000 || v.Drill != 400_000 ||
		v.LayerFrom != "F.Cu" || v.LayerTo != "B.Cu" {
		t.Errorf("SIG via = %+v", v)
	}
}

func TestBoardGeometryLayersOutlineZones(t *testing.T) {
	g := readBoardFixture(t)
	if len(g.Layers) != 3 || g.Layers[0].Name != "F.Cu" || g.Layers[0].Kind != "signal" ||
		g.Layers[2].Number != 44 || g.Layers[2].Name != "Edge.Cuts" {
		t.Errorf("layers = %+v", g.Layers)
	}
	if g.Outline == nil || len(g.Outline.Paths) != 2 {
		t.Fatalf("outline paths = %+v, want rect + arc", g.Outline)
	}
	rect := g.Outline.Paths[0]
	if len(rect.Points) != 5 || rect.Points[0].X != 0 || rect.Points[2].X != 30_000_000 || rect.Points[2].Y != -20_000_000 {
		t.Errorf("outline rect = %+v", rect.Points)
	}
	arc := g.Outline.Paths[1]
	if len(arc.Points) != 17 {
		t.Fatalf("arc approximated with %d points, want 17 (16 segments)", len(arc.Points))
	}
	// Every arc point sits on the circle through start/mid/end (center (30,-12)mm, r=2mm).
	for _, p := range arc.Points {
		r := math.Hypot(float64(p.X-30_000_000), float64(p.Y-(-12_000_000)))
		if math.Abs(r-2_000_000) > 1_000 {
			t.Fatalf("arc point %+v is %.0fnm from center, want 2mm radius", p, r)
		}
	}
	if len(g.Zones) != 1 || g.Zones[0].Net != "GND" || g.Zones[0].Layer != "F.Cu" || len(g.Zones[0].Outline.Points) != 4 {
		t.Errorf("zones = %+v", g.Zones)
	}
}

// TestBoardGeometryKiCad10NetNames covers a KiCad 10 board, which drops the numbered net table and
// references copper nets by name only; the copper must still group and join (found on
// the corpus pic_programmer board, where the numbered-only resolution dropped all 370
// segments).
func TestBoardGeometryKiCad10NetNames(t *testing.T) {
	g, err := ReadBoardGeometry(bytes.NewReader(readFixture(t, "board_v10.kicad_pcb")), "board_v10.kicad_pcb")
	if err != nil {
		t.Fatalf("ReadBoardGeometry: %v", err)
	}
	if len(g.Nets) != 1 || g.Nets[0].Net != "SIG" || len(g.Nets[0].Segments) != 1 || len(g.Nets[0].Vias) != 1 {
		t.Errorf("nets = %+v, want SIG with 1 segment + 1 via", g.Nets)
	}
	if len(g.Zones) != 1 || g.Zones[0].Net != "GND" {
		t.Errorf("zones = %+v, want GND zone via the name-only form", g.Zones)
	}
}

// TestBoardGeometryJoinsNetlistIR is the WS1-006 done-when. The same file read through
// both readers joins by the stable keys, so every placement resolves to an IR component,
// every copper net to an IR net, and every connected pad to that component's connection
// on that net, yielding per-component placement and per-net routed geometry.
func TestBoardGeometryJoinsNetlistIR(t *testing.T) {
	raw := readFixture(t, "board.kicad_pcb")
	d, err := Read(bytes.NewReader(raw), "board.kicad_pcb")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	g, err := ReadBoardGeometry(bytes.NewReader(raw), "board.kicad_pcb")
	if err != nil {
		t.Fatalf("ReadBoardGeometry: %v", err)
	}

	comps := map[string]bool{}
	for _, c := range d.Components {
		comps[c.RefDes] = true
	}
	conns := map[string]string{} // "refdes.pad" -> net name
	nets := map[string]bool{}
	for _, n := range d.Nets {
		nets[n.Name] = true
		for _, c := range n.Connections {
			conns[c.ComponentRef+"."+c.PinRef] = n.Name
		}
	}

	for _, p := range g.Placements {
		if !comps[p.RefDes] {
			t.Errorf("placement %q has no IR component", p.RefDes)
		}
		for _, pad := range p.Pads {
			if pad.Net == "" {
				continue
			}
			if got := conns[p.RefDes+"."+pad.Number]; got != pad.Net {
				t.Errorf("pad %s.%s: sidecar net %q, IR connection net %q", p.RefDes, pad.Number, pad.Net, got)
			}
		}
	}
	for _, nc := range g.Nets {
		if !nets[nc.Net] {
			t.Errorf("copper net %q has no IR net", nc.Net)
		}
	}
	if len(g.Placements) != len(d.Components) {
		t.Errorf("placements=%d IR components=%d; the two artifacts should agree on the component set",
			len(g.Placements), len(d.Components))
	}
}

// TestPlaceholderFootprintsSkipped (WS1-024) checks that an unannotated footprint is skipped by BOTH
// readers. A placeholder is annotation state, not an identity, and keying it merges
// distinct parts' pads onto one component (the corpus cimos board's 26 REF** footprints
// collapsed to one, tripping pin-net-conflict).
//
// The fixture carries both forms this reader has to recognize: the fully unassigned "REF**"
// and the partly-assigned "C?1845" a tool leaves when only some digits are filled in. The
// second is the one a suffix-only predicate misses, which is what agni issue 311 unified.
func TestPlaceholderFootprintsSkipped(t *testing.T) {
	raw := readFixture(t, "board.kicad_pcb")
	d, err := Read(bytes.NewReader(raw), "board.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	g := readBoardFixture(t)
	for _, ref := range []string{"REF**", "C?1845"} {
		for _, c := range d.Components {
			if c.RefDes == ref {
				t.Errorf("netlist reader kept the placeholder component %q", ref)
			}
		}
		for _, p := range g.Placements {
			if p.RefDes == ref {
				t.Errorf("board-geometry reader kept the placeholder placement %q", ref)
			}
		}
		for _, txt := range g.Texts {
			if txt.GetRefDes() == ref {
				t.Errorf("board-geometry reader kept text for the placeholder %q", ref)
			}
		}
	}
	if len(g.Placements) != len(d.Components) {
		t.Errorf("artifacts disagree: %d placements vs %d components", len(g.Placements), len(d.Components))
	}
}

// A footprint's graphics on the silkscreen, fab and courtyard layers are drawn, and a library's own
// artwork on the user layers is not (agni issue 946).
func TestFootprintUserLayerArtIsNotDrawn(t *testing.T) {
	line := func(layer string) string {
		return `(fp_line (start -1 0) (end 1 0) (stroke (width 0.1) (type solid)) (layer "` + layer + `"))`
	}
	var b strings.Builder
	b.WriteString("(kicad_pcb (version 20240108) (generator test)\n(footprint \"Lib:C_0402\" (layer \"F.Cu\") (at 10 10)\n(property \"Reference\" \"C1\" (at 0 -1 0))\n")
	for _, l := range []string{"F.SilkS", "B.SilkS", "F.Fab", "B.Fab", "F.CrtYd", "B.CrtYd", "User.9", "User.5", "Dwgs.User", "Cmts.User", "Eco1.User"} {
		b.WriteString(line(l) + "\n")
	}
	b.WriteString("))\n")
	g, err := ReadBoardGeometry(strings.NewReader(b.String()), "art.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, gr := range g.Graphics {
		got = append(got, gr.GetLayer())
	}
	if strings.Join(got, " ") != "F.SilkS B.SilkS F.Fab B.Fab F.CrtYd B.CrtYd" {
		t.Fatalf("drawn footprint graphics on %q, want the six silkscreen, fab and courtyard layers alone", got)
	}
}

// TestBoardGeometryArcTrackAndSlot reads a curved trace as copper and a slotted hole as its narrow
// width. Both read as nothing until agni issues 1260 and 1268's cross-check: an (arc ...) track was
// never read, and (drill oval w h) parsed its "oval" keyword as the size.
func TestBoardGeometryArcTrackAndSlot(t *testing.T) {
	g, err := ReadBoardGeometry(bytes.NewReader(readFixture(t, "board_arc_slot.kicad_pcb")), "board_arc_slot.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.GetNets()) != 1 || g.GetNets()[0].GetNet() != "SHLD" {
		t.Fatalf("nets = %v, want the arc's copper on SHLD", g.GetNets())
	}
	segs := g.GetNets()[0].GetSegments()
	if len(segs) != 16 || segs[0].GetWidth() != 200_000 || segs[0].GetLayer() != "F.Cu" {
		t.Fatalf("arc segments = %d (first %v), want 16 pieces 0.2mm wide on F.Cu", len(segs), segs[0])
	}
	first, last := segs[0].GetA(), segs[len(segs)-1].GetB()
	if first.GetX() != 10_000_000 || first.GetY() != -10_000_000 || last.GetX() != 14_000_000 || last.GetY() != -10_000_000 {
		t.Errorf("arc runs %v to %v, want (10,-10)mm to (14,-10)mm", first, last)
	}
	pads := g.GetPlacements()[0].GetPads()
	if len(pads) != 1 || pads[0].GetDrill() != 600_000 {
		t.Errorf("J1.S1 drill = %v, want the slot's 0.6mm width", pads)
	}
}
