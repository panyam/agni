package ipc2581

import (
	"bytes"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

func readBoardGeom(t *testing.T, name string) *geom.BoardGeometry {
	t.Helper()
	g, err := ReadBoardGeometry(bytes.NewReader(readFixture(t, name)), "test.xml")
	if err != nil {
		t.Fatalf("ReadBoardGeometry %s: %v", name, err)
	}
	return g
}

// TestBoardGeometryZonesViaSpansUserPads (WS1-031 D/E/F): a Set-direct Contour becomes a copper
// Zone (fill), a drill layer's Span sets the via's layer span, and a pad using UserPrimitiveRef
// resolves to a real size (its user-dictionary bounding box) instead of 0x0.
func TestBoardGeometryZonesViaSpansUserPads(t *testing.T) {
	g := readBoardGeom(t, "board_geom.xml")

	// D: the GNDPLANE Set's Contour -> a Zone with the authored outline.
	var zone *geom.Zone
	for _, z := range g.Zones {
		if z.Net == "GNDPLANE" {
			zone = z
		}
	}
	if zone == nil || zone.Layer != "F.Cu" || zone.Outline == nil || len(zone.Outline.Points) != 5 {
		t.Fatalf("GNDPLANE zone = %+v, want F.Cu fill with a 5-point outline", zone)
	}
	if p := zone.Outline.Points[0]; p.X != 0 || p.Y != 0 {
		t.Errorf("zone outline[0] = (%d,%d), want (0,0)", p.X, p.Y)
	}

	// E: the via (H1) records the TOP..BOTTOM span from the TOP layer's <Span>.
	var via *geom.Via
	for _, nc := range g.Nets {
		for _, v := range nc.GetVias() {
			via = v
		}
	}
	if via == nil || via.LayerFrom != "F.Cu" || via.LayerTo != "B.Cu" {
		t.Fatalf("via = %+v, want layer span F.Cu..B.Cu", via)
	}

	// F: U1 pin 8 uses UserPrimitiveRef UPRIM_1; its pad resolves to the 0.8x0.5mm user-primitive
	// bounding box, not 0x0.
	u1 := placementByRef(g, "U1")
	pad8 := padByNumber(u1, "8")
	if pad8 == nil || pad8.Size.X != 800_000 || pad8.Size.Y != 500_000 {
		t.Errorf("U1 pad 8 = %+v, want 800e3x500e3 (UserPrimitiveRef bbox), not zero", pad8)
	}
}

// TestBoardGeometrySilkGraphics (WS1-031): a package's Marking silkscreen artwork is composed to
// board coordinates per placement, so it lands on its part. R1 (R0603, at 3mm,4mm, rotation 0,
// front) carries the R0603 body Marking; its first point (local -0.5,-0.3 mm) composes to
// (2.5mm, 3.7mm) on F.SilkS. R2 (same package, rotated 90) also emits one, so two graphics total.
func TestBoardGeometrySilkGraphics(t *testing.T) {
	g := readBoardGeom(t, "board_geom.xml")
	var r1 *geom.BoardGraphic
	n := 0
	for _, gr := range g.Graphics {
		if gr.RefDes == "R1" {
			r1 = gr
		}
		n++
	}
	if n != 2 {
		t.Fatalf("graphics = %d, want 2 (R1 + R2 from the R0603 Marking)", n)
	}
	if r1 == nil || r1.Layer != "F.SilkS" || r1.Shape.GetKind() != geom.Shape_KIND_POLYLINE {
		t.Fatalf("R1 graphic = %+v, want F.SilkS polyline", r1)
	}
	if p := r1.Shape.Points[0]; p.X != 2_500_000 || p.Y != 3_700_000 {
		t.Errorf("R1 first point = (%d,%d), want (2.5mm,3.7mm) composed", p.X, p.Y)
	}
}

func placementByRef(g *geom.BoardGeometry, ref string) *geom.ComponentPlacement {
	for _, p := range g.Placements {
		if p.RefDes == ref {
			return p
		}
	}
	return nil
}

func padByNumber(pl *geom.ComponentPlacement, num string) *geom.Pad {
	if pl == nil {
		return nil
	}
	for _, p := range pl.Pads {
		if p.Number == num {
			return p
		}
	}
	return nil
}

// TestBoardGeometryPlacementRotationFrame pins the canonical emit contract (WS1-030): IPC-2581
// is Y-up, so the source Xform rotation is carried verbatim and the back side sets Mirror; pads
// stay footprint-local and unmodified. The renderer composes rotation and mirror (see the render
// package's behavioral test that this lands each pin on its copper).
func TestBoardGeometryPlacementRotationFrame(t *testing.T) {
	g := readBoardGeom(t, "board_geom_rotation.xml")

	// Top: rotation verbatim, not mirrored, pads local.
	at := placementByRef(g, "AT")
	if at == nil || at.Layer != "F.Cu" || at.RotationDeg != 90 || at.Mirror {
		t.Fatalf("AT = %v, want F.Cu rot=90 (verbatim) mirror=false", at)
	}
	if p := padByNumber(at, "1"); p == nil || p.At.X != 1_000_000 || p.At.Y != 0 {
		t.Errorf("AT pad 1 at %v, want local (1e6,0)", p.GetAt())
	}

	// Bottom: rotation verbatim, Mirror set, pads unchanged (the renderer mirrors).
	ab := placementByRef(g, "AB")
	if ab == nil || ab.Layer != "B.Cu" || ab.RotationDeg != 90 || !ab.Mirror {
		t.Fatalf("AB = %v, want B.Cu rot=90 (verbatim) mirror=true", ab)
	}
	if p := padByNumber(ab, "1"); p == nil || p.At.X != 1_000_000 || p.At.Y != 0 {
		t.Errorf("AB pad 1 at %v, want local (1e6,0) unmodified (renderer applies the mirror)", p.GetAt())
	}
}

// TestBoardGeometryPlacements: placements are emitted sorted by ref_des, in integer nm
// (unit_nm=1), with side normalized to the contract's KiCad vocabulary and pads resolved
// through the padstack def/instance indirection.
func TestBoardGeometryPlacements(t *testing.T) {
	g := readBoardGeom(t, "board_geom.xml")
	if g.UnitNm != 1 {
		t.Errorf("unit_nm = %d, want 1 (integer nm frame)", g.UnitNm)
	}
	if len(g.Placements) != 3 {
		t.Fatalf("placements = %d, want 3 (R1, R2, U1)", len(g.Placements))
	}
	if got := []string{g.Placements[0].RefDes, g.Placements[1].RefDes, g.Placements[2].RefDes}; got[0] != "R1" || got[1] != "R2" || got[2] != "U1" {
		t.Errorf("placement order = %v, want [R1 R2 U1] (sorted by ref_des)", got)
	}

	r1 := placementByRef(g, "R1")
	if r1.At.X != 3_000_000 || r1.At.Y != 4_000_000 || r1.RotationDeg != 0 || r1.Layer != "F.Cu" {
		t.Errorf("R1 = at(%d,%d) rot=%v layer=%q, want at(3e6,4e6) rot=0 F.Cu", r1.At.X, r1.At.Y, r1.RotationDeg, r1.Layer)
	}
	pad2 := padByNumber(r1, "2")
	if pad2 == nil || pad2.Shape != "rect" || pad2.Size.X != 800_000 || pad2.Size.Y != 600_000 {
		t.Errorf("R1 pad 2 = %v, want rect 800e3x600e3 (from RECT_1)", pad2)
	}
	if pad2.At.X != 750_000 || pad2.At.Y != 0 {
		t.Errorf("R1 pad 2 at (%d,%d), want footprint-local (750e3,0)", pad2.At.X, pad2.At.Y)
	}
	if len(pad2.Layers) != 1 || pad2.Layers[0] != "F.Cu" {
		t.Errorf("R1 pad 2 layers = %v, want [F.Cu]", pad2.Layers)
	}

	u1 := placementByRef(g, "U1")
	if u1.Layer != "B.Cu" || u1.RotationDeg != 270 {
		t.Errorf("U1 = layer=%q rot=%v, want B.Cu (BOTTOM) rot=270", u1.Layer, u1.RotationDeg)
	}
	pad4 := padByNumber(u1, "4")
	if pad4 == nil || pad4.Shape != "circle" || pad4.Size.X != 500_000 || pad4.Size.Y != 500_000 {
		t.Errorf("U1 pad 4 = %v, want circle 500e3 (from CIRCLE_1)", pad4)
	}
}

// TestBoardGeometryLayersOutline: the stackup table keeps IPC-2581's layerFunction verbatim in
// kind, and the Profile becomes an outline polyline (Y-up, no negation).
func TestBoardGeometryLayersOutline(t *testing.T) {
	g := readBoardGeom(t, "board_geom.xml")
	if len(g.Layers) != 2 || g.Layers[0].Name != "TOP" || g.Layers[0].Kind != "CONDUCTOR" || g.Layers[1].Name != "BOTTOM" {
		t.Errorf("layers = %v, want TOP/BOTTOM kind=CONDUCTOR", g.Layers)
	}
	if g.Outline == nil || len(g.Outline.Paths) != 1 {
		t.Fatalf("outline = %v, want 1 path", g.Outline)
	}
	pts := g.Outline.Paths[0].Points
	if len(pts) != 5 {
		t.Fatalf("outline points = %d, want 5 (closed rect)", len(pts))
	}
	if pts[0].X != 0 || pts[0].Y != 0 || pts[2].X != 10_000_000 || pts[2].Y != 8_000_000 {
		t.Errorf("outline corners = %v..%v, want (0,0)..(10e6,8e6)", pts[0], pts[2])
	}
}

// TestBoardGeometryOutlineArc: a Profile arc step (PolyStepCurve) is expanded into chord points
// in document order (WS1-028), so the outline follows the arc bulge instead of cutting across it.
// The fixture's top edge is a semicircle whose apex at (5,9) exists only if the curve is emitted;
// before the fix the arc was dropped and the outline stopped at y=4 with 4 straight points.
func TestBoardGeometryOutlineArc(t *testing.T) {
	g := readBoardGeom(t, "board_geom_outline_arc.xml")
	if g.Outline == nil || len(g.Outline.Paths) != 1 {
		t.Fatalf("outline = %v, want 1 path", g.Outline)
	}
	pts := g.Outline.Paths[0].Points
	if len(pts) <= 5 {
		t.Fatalf("outline points = %d, want the arc expanded to many chords (>5); a small count means the curve was dropped", len(pts))
	}
	var maxY int64
	var apex bool
	for _, p := range pts {
		if p.Y > maxY {
			maxY = p.Y
		}
		if abs64(p.X-5_000_000) <= 2 && abs64(p.Y-9_000_000) <= 2 {
			apex = true
		}
	}
	if maxY < 8_900_000 {
		t.Errorf("outline max Y = %d, want ~9e6 (the arc apex); a max of ~4e6 means the arc cut straight across", maxY)
	}
	if !apex {
		t.Errorf("outline missing the arc apex point near (5e6,9e6); the semicircle was not expanded")
	}
}

// TestBoardGeometryOutlineArcClockwiseCase guards the Allegro uppercase-casing fix: the arc step
// carries clockwise="TRUE", so it must sweep clockwise (the apex dips DOWN to y=-1). A
// case-sensitive =="true" parse would read it as counter-clockwise and bulge UP to y=9 instead.
func TestBoardGeometryOutlineArcClockwiseCase(t *testing.T) {
	g := readBoardGeom(t, "board_geom_arc_cw.xml")
	if g.Outline == nil || len(g.Outline.Paths) != 1 {
		t.Fatalf("outline = %v, want 1 path", g.Outline)
	}
	pts := g.Outline.Paths[0].Points
	var minY, maxY int64
	var apex bool
	for _, p := range pts {
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
		if abs64(p.X-5_000_000) <= 2 && abs64(p.Y+1_000_000) <= 2 {
			apex = true
		}
	}
	if minY > -900_000 {
		t.Errorf("outline min Y = %d, want ~-1e6 (clockwise arc dips down); >=0 means it was read as counter-clockwise", minY)
	}
	if maxY > 4_000_100 {
		t.Errorf("outline max Y = %d, want ~4e6 (arc does not bulge up); ~9e6 means the clockwise flag was inverted", maxY)
	}
	if !apex {
		t.Errorf("outline missing the clockwise-arc apex near (5e6,-1e6)")
	}
}

// TestBoardGeometryDocumentLayerNotZone guards the zone copper-gate: a Zone is a copper pour, so a
// Contour on a DOCUMENT (fab) layer must not become one — otherwise off-board fabrication-drawing
// bars leak in as copper and balloon the render bounds.
func TestBoardGeometryDocumentLayerNotZone(t *testing.T) {
	g := readBoardGeom(t, "board_geom_doc_zone.xml")
	if len(g.Zones) != 1 {
		t.Fatalf("zones = %d, want 1 (GNDPLANE copper only; the fab DOCUMENT contour excluded)", len(g.Zones))
	}
	if z := g.Zones[0]; z.Net != "GNDPLANE" || z.Layer != "F.Cu" {
		t.Errorf("zone = %+v, want GNDPLANE on F.Cu", z)
	}
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func netCopperByName(g *geom.BoardGeometry, net string) *geom.NetCopper {
	for _, nc := range g.Nets {
		if nc.Net == net {
			return nc
		}
	}
	return nil
}

// TestBoardGeometryCopper: a routed track becomes width-carrying TrackSegments (its arc step
// approximated as arcSegments chords), and a drilled VIA hole becomes a Via whose annular is
// the co-located copper pad minus the drill.
func TestBoardGeometryCopper(t *testing.T) {
	g := readBoardGeom(t, "board_geom.xml")
	gnd := netCopperByName(g, "GND")
	if gnd == nil {
		t.Fatalf("no GND copper; nets = %v", g.Nets)
	}
	// begin + one straight step (1 seg) + one arc step (arcSegments chords) = 1 + arcSegments segments.
	if want := 1 + arcSegments; len(gnd.Segments) != want {
		t.Errorf("GND segments = %d, want %d (1 straight + %d-chord arc)", len(gnd.Segments), want, arcSegments)
	}
	s0 := gnd.Segments[0]
	if s0.Width != 200_000 || s0.Layer != "F.Cu" {
		t.Errorf("GND seg0 width=%d layer=%q, want 200000 F.Cu", s0.Width, s0.Layer)
	}
	if s0.A.X != 2_000_000 || s0.A.Y != 1_000_000 || s0.B.X != 4_000_000 {
		t.Errorf("GND seg0 = %v..%v, want (2e6,1e6)..(4e6,*)", s0.A, s0.B)
	}
	if len(gnd.Vias) != 1 {
		t.Fatalf("GND vias = %d, want 1", len(gnd.Vias))
	}
	v := gnd.Vias[0]
	if v.Drill != 300_000 || v.Size != 500_000 {
		t.Errorf("GND via drill=%d size=%d, want drill=300000 size=500000 (co-located CIRCLE_1)", v.Drill, v.Size)
	}
	if v.At.X != 4_000_000 || v.At.Y != 1_000_000 {
		t.Errorf("GND via at (%d,%d), want (4e6,1e6)", v.At.X, v.At.Y)
	}
}

// TestBoardGeometryJoinsNetlistIR is the WS1-006/023 cross-artifact acceptance: the same file
// read as netlist (Read) and as board geometry (ReadBoardGeometry) must agree on ref_des and
// on pin<->pad identity, so a board rule can key one against the other.
func TestBoardGeometryJoinsNetlistIR(t *testing.T) {
	fixture := readFixture(t, "board_geom.xml")
	d, err := Read(bytes.NewReader(fixture), "test.xml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	g := readBoardGeom(t, "board_geom.xml")

	if len(g.Placements) != len(d.Components) {
		t.Errorf("placements = %d, components = %d, want equal", len(g.Placements), len(d.Components))
	}
	for _, pl := range g.Placements {
		if findComponent(d, pl.RefDes) == nil {
			t.Errorf("placement %s has no netlist component", pl.RefDes)
		}
	}
	// Every netlist connection lands on a pad of that component's placement.
	for _, n := range d.Nets {
		for _, c := range n.Connections {
			pl := placementByRef(g, c.ComponentRef)
			if padByNumber(pl, c.PinRef) == nil {
				t.Errorf("connection %s.%s (net %s) has no matching pad", c.ComponentRef, c.PinRef, n.Name)
			}
		}
	}
}
