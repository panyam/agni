package render

import (
	"encoding/binary"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// TestPackSheet checks the tier-2 projection: vertices are rebased to the sheet min corner
// as int32 pairs, primitives are fixed-width records over vertex ranges, and keys map each
// primitive to its ref_des/net.
func TestPackSheet(t *testing.T) {
	g := &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{{
			CellRef: "S", LibraryRef: "L", ViewRef: "v",
			Shapes: []*geom.Shape{{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{{X: 0, Y: 0}, {X: 40, Y: 20}}}},
			Pins:   []*geom.PinPoint{{PortRef: "1", Loc: &geom.Point{X: 0, Y: 10}}},
		}},
		Sheets: []*geom.SheetGeometry{{
			Id: "P1",
			Wires: []*geom.WireGeometry{{Net: "NET1", Polylines: []*geom.Polyline{
				{Points: []*geom.Point{{X: 100, Y: 100}, {X: 200, Y: 100}}},
			}}},
			Placements: []*geom.SymbolPlacement{{
				RefDes: "R1", CellRef: "S", LibraryRef: "L", ViewRef: "v",
				Transform: &geom.Transform{Origin: &geom.Point{X: 300, Y: 400}},
			}},
		}},
	}

	ps := PackSheet(g, g.Sheets[0])

	// Rebase origin is the min corner over all primitives (the wire's 100,100).
	if ps.OriginX != 100 || ps.OriginY != 100 {
		t.Fatalf("origin = (%d,%d), want (100,100)", ps.OriginX, ps.OriginY)
	}
	verts := decodeVerts(ps.Vertices)
	// wire (2) + rect loop (4) + pin (1) = 7 vertices.
	if len(verts) != 7 {
		t.Fatalf("vertices = %d, want 7", len(verts))
	}
	// Wire rebased: (0,0),(100,0). Rect first corner (world 300,400) rebased (200,300).
	if verts[0] != [2]int32{0, 0} || verts[1] != [2]int32{100, 0} {
		t.Errorf("wire verts = %v, %v, want (0,0),(100,0)", verts[0], verts[1])
	}
	if verts[2] != [2]int32{200, 300} {
		t.Errorf("rect corner0 = %v, want (200,300)", verts[2])
	}
	if verts[6] != [2]int32{200, 310} {
		t.Errorf("pin vertex = %v, want (200,310)", verts[6])
	}

	recs := decodeRecords(ps.Primitives)
	if len(recs) != 3 {
		t.Fatalf("primitive records = %d, want 3", len(recs))
	}
	want := []primRecord{
		{primLineStrip, groupWire, 0, 2},
		{primLineLoop, groupSymbol, 2, 4},
		{primPoints, groupPin, 6, 1},
	}
	for i, w := range want {
		if recs[i] != w {
			t.Errorf("record %d = %+v, want %+v", i, recs[i], w)
		}
	}

	if len(ps.Keys) != 3 {
		t.Fatalf("keys = %d, want 3", len(ps.Keys))
	}
	if k := ps.Keys[0]; k.Primitive != 0 || k.Net != "NET1" || k.RefDes != "" {
		t.Errorf("wire key = %+v, want primitive 0 net NET1", k)
	}
	if k := ps.Keys[1]; k.Primitive != 1 || k.RefDes != "R1" {
		t.Errorf("symbol key = %+v, want primitive 1 ref_des R1", k)
	}
	if k := ps.Keys[2]; k.Primitive != 2 || k.RefDes != "R1" {
		t.Errorf("pin key = %+v, want primitive 2 ref_des R1", k)
	}
}

// TestPackSheetFreeGraphics checks that sheet-level free graphics (junction dots,
// no-connect markers, notes) — the ones the SVG backend draws from sheet.Shapes — are
// packed into the WebGL path under groupFree, so the WebGL render reaches parity with the
// SVG oracle. These carry no ref_des/net, so they get no PrimitiveKey.
// TestPackSheetImages packs sheet-level images into PackedImage, rebased to the same origin as
// the vertices, carrying mime + raw bytes; images with no bbox or no data are skipped.
func TestPackSheetImages(t *testing.T) {
	data := []byte{0x89, 'P', 'N', 'G'}
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Id: "P1",
			// A wire fixes the rebase origin at its min corner (100,100).
			Wires: []*geom.WireGeometry{{Net: "N", Polylines: []*geom.Polyline{
				{Points: []*geom.Point{{X: 100, Y: 100}, {X: 200, Y: 100}}},
			}}},
			Images: []*geom.Image{
				{Bbox: &geom.BBox{Min: &geom.Point{X: 120, Y: 130}, Max: &geom.Point{X: 160, Y: 180}},
					Mime: "image/png", Data: data, RotationDeg: 90, Mirror: true},
				{Bbox: &geom.BBox{Min: &geom.Point{X: 0, Y: 0}, Max: &geom.Point{X: 10, Y: 10}}}, // no data -> skipped
				{Data: data}, // no bbox -> skipped
			},
		}},
	}
	ps := PackSheet(g, g.Sheets[0])
	if len(ps.Images) != 1 {
		t.Fatalf("packed images = %d, want 1 (the two malformed ones skipped)", len(ps.Images))
	}
	im := ps.Images[0]
	if im.X != 20 || im.Y != 30 || im.W != 40 || im.H != 50 {
		t.Errorf("rebased image box = {x:%d y:%d w:%d h:%d}, want {20 30 40 50}", im.X, im.Y, im.W, im.H)
	}
	if im.Mime != "image/png" || string(im.Data) != string(data) {
		t.Errorf("image mime/data = %q/%v, want image/png + raw bytes", im.Mime, im.Data)
	}
	if im.RotationDeg != 90 || !im.Mirror {
		t.Errorf("image rotation/mirror = %d/%v, want 90/true", im.RotationDeg, im.Mirror)
	}
}

func TestPackSheetFreeGraphics(t *testing.T) {
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Id: "P1",
			Wires: []*geom.WireGeometry{{Net: "NET1", Polylines: []*geom.Polyline{
				{Points: []*geom.Point{{X: 100, Y: 100}, {X: 200, Y: 100}}},
			}}},
			// A junction dot on the wire and a free note polyline; neither is owned by a
			// placed symbol.
			Shapes: []*geom.Shape{
				{Kind: geom.Shape_KIND_DOT, Points: []*geom.Point{{X: 200, Y: 100}}},
				{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{{X: 100, Y: 100}, {X: 100, Y: 200}}},
			},
		}},
	}

	ps := PackSheet(g, g.Sheets[0])
	recs := decodeRecords(ps.Primitives)

	// Coverage guard: every wire polyline + every free shape yields at least one record.
	if len(recs) != 3 {
		t.Fatalf("primitive records = %d, want 3 (1 wire + 2 free shapes)", len(recs))
	}
	if recs[1] != (primRecord{primPoints, groupFree, 2, 1}) {
		t.Errorf("junction record = %+v, want points/groupFree at vertex 2", recs[1])
	}
	if recs[2] != (primRecord{primLineStrip, groupFree, 3, 2}) {
		t.Errorf("free polyline record = %+v, want line-strip/groupFree at vertex 3", recs[2])
	}
	// Free graphics carry no ref_des/net, so only the wire produces a key.
	if len(ps.Keys) != 1 {
		t.Fatalf("keys = %d, want 1 (wire only)", len(ps.Keys))
	}
}

// TestPackSheetWorksheetFurniture checks that a sheet with a page (sheet.Size) gets the
// synthesized worksheet furniture (frame + zone-ruler ticks + title-block box/dividers)
// packed under groupFrame, and a sheet with no page gets none.
func TestPackSheetWorksheetFurniture(t *testing.T) {
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Id: "P1", Name: "Sheet1",
			Size:       &geom.BBox{Min: &geom.Point{X: 0, Y: 0}, Max: &geom.Point{X: 1000, Y: 800}},
			TitleBlock: &geom.TitleBlock{Title: "T", Company: "C"},
			Wires: []*geom.WireGeometry{{Net: "N", Polylines: []*geom.Polyline{
				{Points: []*geom.Point{{X: 400, Y: 400}, {X: 500, Y: 400}}},
			}}},
		}},
	}

	countFrame := func(ps *geom.PackedSheet) int {
		n := 0
		for _, r := range decodeRecords(ps.Primitives) {
			if r.group == groupFrame {
				n++
			}
		}
		return n
	}

	// KiCad grid rows = {Company, Sheet, Title, Size|Date|Rev, Id} -> 5, so 4 row dividers, plus
	// the two split rows add 3 interior column dividers (Size|Date|Rev: 2, Id: 1).
	// furniture = 1 frame + (6-1)*2 col ticks + (4-1)*2 row ticks + 1 title box + 4 row + 3 col = 25.
	if got := countFrame(PackSheet(g, g.Sheets[0])); got != 25 {
		t.Fatalf("groupFrame primitives = %d, want 25", got)
	}

	g.Sheets[0].Size = nil
	if got := countFrame(PackSheet(g, g.Sheets[0])); got != 0 {
		t.Fatalf("furniture emitted for a sheet with no page: %d groupFrame primitives", got)
	}
}

// TestPackBusQuads checks that a bus trunk/entry (WS7-042) packs as true-width triangle quads in
// groupBus (GL can't stroke a line wider than 1px), while a plain wire stays a groupWire line
// strip, and that groupBus resolves the bus color.
func TestPackBusQuads(t *testing.T) {
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Id: "P1",
			Wires: []*geom.WireGeometry{
				{Net: "SIG", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 100_000, Y: 0}}}}},
				{Kind: geom.WireGeometry_KIND_BUS, Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 200_000}, {X: 100_000, Y: 200_000}}}}},
				{Kind: geom.WireGeometry_KIND_BUS_ENTRY, Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 100_000, Y: 200_000}, {X: 102_540, Y: 197_460}}}}},
			},
		}},
	}

	ps := PackSheet(g, g.Sheets[0])
	recs := decodeRecords(ps.Primitives)
	if len(recs) != 3 {
		t.Fatalf("records = %d, want 3 (1 wire strip + bus quad + bus_entry quad)", len(recs))
	}
	if recs[0].kind != primLineStrip || recs[0].group != groupWire {
		t.Errorf("wire record = %+v, want line-strip/groupWire", recs[0])
	}
	// Each single-segment bus polyline tessellates to one quad: two triangles = 6 vertices.
	for i, r := range recs[1:] {
		if r.kind != primTriangles || r.group != groupBus || r.count != 6 {
			t.Errorf("bus record %d = %+v, want triangles/groupBus/6 verts", i+1, r)
		}
	}
	if got := ps.GroupColors[groupBus]; got != DefaultStyle.Bus {
		t.Errorf("GroupColors[groupBus] = %q, want %q", got, DefaultStyle.Bus)
	}
}

// TestPackBusKey checks that a bus primitive carries a PrimitiveKey keyed by the bus NAME (WS7-042b),
// so a bus-not-modeled finding can highlight it — a bus has no net, so without this key the build()
// guard would drop it and the bus would be unpickable. The name rides PrimitiveKey.bus_id, disjoint
// from a net wire's net key.
func TestPackBusKey(t *testing.T) {
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Id: "P1",
			Wires: []*geom.WireGeometry{
				{Net: "SIG", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 100_000, Y: 0}}}}},
				{Kind: geom.WireGeometry_KIND_BUS, Net: "DATA[7:0]",
					Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 200_000}, {X: 100_000, Y: 200_000}}}}},
			},
		}},
	}
	ps := PackSheet(g, g.Sheets[0])
	var busKeys, netKeys int
	for _, k := range ps.Keys {
		if k.GetBusId() == "DATA[7:0]" {
			busKeys++
			if k.GetNet() != "" || k.GetNetId() != "" {
				t.Errorf("bus key carries a net (%q/%q), want none", k.GetNet(), k.GetNetId())
			}
		}
		if k.GetNet() == "SIG" {
			netKeys++
			if k.GetBusId() != "" {
				t.Errorf("wire key carries a bus id %q, want none", k.GetBusId())
			}
		}
	}
	if busKeys != 1 {
		t.Errorf("bus keys with name DATA[7:0] = %d, want 1", busKeys)
	}
	if netKeys != 1 {
		t.Errorf("wire keys for SIG = %d, want 1", netKeys)
	}
}

type primRecord struct {
	kind, group        uint8
	firstVertex, count uint32
}

func decodeVerts(b []byte) [][2]int32 {
	out := make([][2]int32, 0, len(b)/8)
	for i := 0; i+8 <= len(b); i += 8 {
		out = append(out, [2]int32{
			int32(binary.LittleEndian.Uint32(b[i:])),
			int32(binary.LittleEndian.Uint32(b[i+4:])),
		})
	}
	return out
}

func decodeRecords(b []byte) []primRecord {
	out := make([]primRecord, 0, len(b)/primRecordBytes)
	for i := 0; i+primRecordBytes <= len(b); i += primRecordBytes {
		out = append(out, primRecord{
			kind:        b[i],
			group:       b[i+1],
			firstVertex: binary.LittleEndian.Uint32(b[i+4:]),
			count:       binary.LittleEndian.Uint32(b[i+8:]),
		})
	}
	return out
}
