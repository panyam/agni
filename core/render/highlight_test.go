package render

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// highlightFixture is a two-component, two-net sheet: R1 (rect + pins 1,2) at (300,400),
// U1 (rect + pin 1) at (600,400), NET1 and NET2 wires. Primitive order (PackSheet packs
// wires, then free shapes, then placements): 0 NET1 wire, 1 NET2 wire, 2 R1 rect,
// 3 R1 pin1, 4 R1 pin2, 5 U1 rect, 6 U1 pin1.
func highlightFixture() *geom.SchematicGeometry {
	return &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{
			{
				CellRef: "R", LibraryRef: "L", ViewRef: "v",
				Shapes: []*geom.Shape{{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{{X: 0, Y: 0}, {X: 40, Y: 20}}}},
				Pins: []*geom.PinPoint{
					{PortRef: "1", Loc: &geom.Point{X: 0, Y: 10}},
					{PortRef: "2", Loc: &geom.Point{X: 40, Y: 10}},
				},
			},
			{
				CellRef: "U", LibraryRef: "L", ViewRef: "v",
				Shapes: []*geom.Shape{{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{{X: 0, Y: 0}, {X: 60, Y: 60}}}},
				Pins:   []*geom.PinPoint{{PortRef: "1", Loc: &geom.Point{X: 0, Y: 30}}},
			},
		},
		Sheets: []*geom.SheetGeometry{{
			Id: "P1",
			Wires: []*geom.WireGeometry{
				{Net: "NET1", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 100, Y: 100}, {X: 200, Y: 100}}}}},
				{Net: "NET2", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 100, Y: 200}, {X: 200, Y: 200}}}}},
			},
			Placements: []*geom.SymbolPlacement{
				{RefDes: "R1", CellRef: "R", LibraryRef: "L", ViewRef: "v",
					Transform: &geom.Transform{Origin: &geom.Point{X: 300, Y: 400}}},
				{RefDes: "U1", CellRef: "U", LibraryRef: "L", ViewRef: "v",
					Transform: &geom.Transform{Origin: &geom.Point{X: 600, Y: 400}}},
			},
		}},
	}
}

// TestHighlightPacked resolves component/net/pin specs to primitive index groups against
// the packed keys: a component matches its symbol graphics and all its pins, a net its
// wire polylines, and a pin just that pin's primitive. Groups come back in spec order with
// the spec's style.
func TestHighlightPacked(t *testing.T) {
	g := highlightFixture()
	ps := PackSheet(g, g.Sheets[0])

	specs := []*geom.HighlightSpec{
		{Components: []string{"R1"}, Nets: []string{"NET2"}, Color: "#ff0000", Alpha: 0.5},
		{Pins: []*geom.PinRef{{RefDes: "U1", Pin: "1"}}},
	}
	hl := HighlightPacked(ps, specs)

	if len(hl.Groups) != 2 {
		t.Fatalf("groups = %d, want 2 (one per spec)", len(hl.Groups))
	}
	g0 := hl.Groups[0]
	if g0.Color != "#ff0000" || g0.Alpha != 0.5 {
		t.Errorf("group 0 style = %q/%v, want #ff0000/0.5", g0.Color, g0.Alpha)
	}
	// R1 = rect(2) + pin1(3) + pin2(4); NET2 = wire(1).
	if want := []uint32{1, 2, 3, 4}; !equalU32(g0.Primitives, want) {
		t.Errorf("group 0 primitives = %v, want %v", g0.Primitives, want)
	}
	// U1 pin "1" alone, not U1's rect.
	if want := []uint32{6}; !equalU32(hl.Groups[1].Primitives, want) {
		t.Errorf("group 1 primitives = %v, want %v", hl.Groups[1].Primitives, want)
	}
}

// TestHighlightPackedNoMatch: a spec matching nothing still yields its (empty) group, so
// groups stay aligned with the input specs.
func TestHighlightPackedNoMatch(t *testing.T) {
	g := highlightFixture()
	ps := PackSheet(g, g.Sheets[0])
	hl := HighlightPacked(ps, []*geom.HighlightSpec{{Components: []string{"NOPE"}, Nets: []string{"X"}}})
	if len(hl.Groups) != 1 || len(hl.Groups[0].Primitives) != 0 {
		t.Fatalf("groups = %+v, want one empty group", hl.Groups)
	}
}

// TestHighlightSVG checks the SVG overlay projection: the document has the exact frame
// (width/height/viewBox) of SheetSVG for the same sheet so the layers composite, it is
// transparent (no background rect), and it draws the matched net wire, the matched
// component's outline and pin dots, and the individually matched pin — in the spec's
// color/alpha — while unmatched elements stay out.
func TestHighlightSVG(t *testing.T) {
	g := highlightFixture()
	specs := []*geom.HighlightSpec{
		{Components: []string{"R1"}, Nets: []string{"NET1"}, Color: "#00ff00", Alpha: 0.4},
		{Pins: []*geom.PinRef{{RefDes: "U1", Pin: "1"}}},
	}
	overlay := HighlightSVG(g, g.Sheets[0], specs)
	base := SheetSVG(g, g.Sheets[0])

	frame := func(doc string) string {
		open := doc[:strings.Index(doc, ">")+1]
		var out []string
		for _, attr := range []string{"width", "height", "viewBox"} {
			i := strings.Index(open, attr+`="`)
			if i < 0 {
				t.Fatalf("no %s in root element %q", attr, open)
			}
			rest := open[i+len(attr)+2:]
			out = append(out, attr+"="+rest[:strings.Index(rest, `"`)])
		}
		return strings.Join(out, " ")
	}
	if frame(overlay) != frame(base) {
		t.Errorf("overlay frame %q != base frame %q", frame(overlay), frame(base))
	}
	if strings.Contains(overlay, "<rect x=\"0.0\" y=\"0.0\"") {
		t.Error("overlay has a background rect; it must be transparent")
	}

	// One wire (NET1), not two: count highlight polylines.
	if n := strings.Count(overlay, "<polyline"); n != 1 {
		t.Errorf("overlay polylines = %d, want 1 (NET1 only)", n)
	}
	// R1's rect outline, in the spec color/alpha, never filled.
	if !strings.Contains(overlay, `stroke="#00ff00"`) || !strings.Contains(overlay, `stroke-opacity="0.4"`) {
		t.Error("overlay missing spec color/alpha on strokes")
	}
	if strings.Contains(overlay, `fill="#00ff00"  stroke`) {
		t.Error("component outline must not be filled")
	}
	// Pin dots: R1 has 2 pins (whole-component match) + U1 pin 1 = 3 circles... but the R1
	// rect is a <rect>, so circles are exactly the pin dots.
	if n := strings.Count(overlay, "<circle"); n != 3 {
		t.Errorf("overlay pin dots = %d, want 3 (R1 pins 1,2 + U1 pin 1)", n)
	}
	// The unlisted spec-2 color defaults.
	if !strings.Contains(overlay, `fill="`+DefaultHighlightColor+`"`) {
		t.Errorf("defaulted pin highlight color missing (want %s)", DefaultHighlightColor)
	}
}

// TestSheetSVGHighlighted proves the baked single-document render composites exactly: the base
// sheet content (page rect + wires) AND the highlight overlay (the marked net's re-stroke, the
// matched pin dots) in one SVG, with nothing lost or doubled. For every element kind the two layers
// contribute, the baked count equals base + overlay — the one-code-path guarantee the CLI static
// render leans on (same projection the server serves as a separate overlay).
func TestSheetSVGHighlighted(t *testing.T) {
	g := highlightFixture()
	specs := []*geom.HighlightSpec{
		{Components: []string{"R1"}, Nets: []string{"NET1"}, Color: "#00ff00", Alpha: 0.4},
		{Pins: []*geom.PinRef{{RefDes: "U1", Pin: "1"}}},
	}
	base := SheetSVG(g, g.Sheets[0])
	overlay := HighlightSVG(g, g.Sheets[0], specs)
	baked := SheetSVGHighlighted(g, g.Sheets[0], specs)

	// The baked document carries the base page fill (it is NOT a transparent overlay).
	if !strings.Contains(baked, `<rect x="0.0" y="0.0"`) {
		t.Error("baked render is missing the base page rect; a static picture must not be transparent")
	}
	// The highlight is baked in, in its spec color/alpha.
	if !strings.Contains(baked, `stroke="#00ff00"`) || !strings.Contains(baked, `stroke-opacity="0.4"`) {
		t.Error("baked render is missing the highlight color/alpha")
	}
	// Compositing is lossless and duplication-free: baked = base + overlay per element kind.
	for _, el := range []string{"<polyline", "<circle"} {
		if got, want := strings.Count(baked, el), strings.Count(base, el)+strings.Count(overlay, el); got != want {
			t.Errorf("baked %s = %d, want base(%d)+overlay(%d)=%d",
				el, got, strings.Count(base, el), strings.Count(overlay, el), want)
		}
	}
	// The overlay draws LAST, above the schematic, so the highlight stroke sits after the base
	// symbol graphics in document order (SVG painter's model: later = on top).
	if strings.Index(baked, `stroke="#00ff00"`) < strings.LastIndex(base, "<polyline") {
		t.Error("highlight must be painted after the base content so the marked entity reads through it")
	}
}

// TestHighlightSVGBus checks that a bus spec re-strokes ONLY its own bus trunk, keyed by the bus
// NAME and gated on the bus kind so it never matches a net (WS7-042b): a non-matching name paints
// nothing, and a net wire that shares the bus name is not caught.
func TestHighlightSVGBus(t *testing.T) {
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{
		Id: "P1",
		Wires: []*geom.WireGeometry{
			{Net: "SIG", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 100, Y: 0}}}}},
			{Kind: geom.WireGeometry_KIND_BUS, Net: "DATA[7:0]",
				Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 50}, {X: 100, Y: 50}}}}},
		},
	}}}
	overlay := HighlightSVG(g, g.Sheets[0], []*geom.HighlightSpec{{BusIds: []string{"DATA[7:0]"}, Color: "#00ffff"}})
	// The bus trunk re-stroked (one polyline) in the spec color; the SIG wire is untouched.
	if n := strings.Count(overlay, "<polyline"); n != 1 {
		t.Errorf("overlay polylines = %d, want 1 (the bus trunk only)", n)
	}
	if !strings.Contains(overlay, `stroke="#00ffff"`) {
		t.Error("bus highlight missing its spec color")
	}
	// A non-matching bus name paints nothing (no bleed onto the wire).
	none := HighlightSVG(g, g.Sheets[0], []*geom.HighlightSpec{{BusIds: []string{"nope"}}})
	if n := strings.Count(none, "<polyline"); n != 0 {
		t.Errorf("non-matching bus overlay polylines = %d, want 0", n)
	}
	// A net-kind wire sharing the bus name is NOT a bus, so a bus spec does not catch it.
	netOnly := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "P1", Wires: []*geom.WireGeometry{
		{Net: "DATA[7:0]", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 100, Y: 0}}}}},
	}}}}
	if n := strings.Count(HighlightSVG(netOnly, netOnly.Sheets[0], []*geom.HighlightSpec{{BusIds: []string{"DATA[7:0]"}}}), "<polyline"); n != 0 {
		t.Errorf("bus spec matched a net wire of the same name: %d polylines, want 0", n)
	}
}

var polyWidthRe = regexp.MustCompile(`<polyline[^>]*stroke-width="([0-9.]+)"`)

// firstPolylineWidth extracts the stroke-width of the first highlight polyline in an overlay.
func firstPolylineWidth(t *testing.T, doc string) float64 {
	t.Helper()
	m := polyWidthRe.FindStringSubmatch(doc)
	if m == nil {
		t.Fatalf("no highlight polyline in overlay:\n%s", doc)
	}
	w, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("bad stroke-width %q: %v", m[1], err)
	}
	return w
}

// TestHighlightSVGPathStyle: a PATH-shape net (WS9-040) re-strokes its wire like OUTLINE but
// wider and translucent by default, so a focused net reads as a highlighter marker along its
// path rather than an opaque bar.
func TestHighlightSVGPathStyle(t *testing.T) {
	g := highlightFixture()
	outline := HighlightSVG(g, g.Sheets[0], []*geom.HighlightSpec{{Nets: []string{"NET1"}}})
	path := HighlightSVG(g, g.Sheets[0], []*geom.HighlightSpec{
		{Nets: []string{"NET1"}, Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_PATH},
	})
	// PATH defaults translucent (0.4); OUTLINE defaults opaque.
	if !strings.Contains(path, `stroke-opacity="0.4"`) {
		t.Errorf("PATH net should default to translucent 0.4:\n%s", path)
	}
	// PATH strokes wider than OUTLINE (pathStrokePx = highlightStrokePx * 2).
	ow, pw := firstPolylineWidth(t, outline), firstPolylineWidth(t, path)
	if pw != ow*2 {
		t.Errorf("PATH stroke-width = %v, want 2x OUTLINE %v", pw, ow)
	}
	// An explicit in-range alpha is honored, not forced to the default.
	explicit := HighlightSVG(g, g.Sheets[0], []*geom.HighlightSpec{
		{Nets: []string{"NET1"}, Alpha: 0.7, Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_PATH},
	})
	if !strings.Contains(explicit, `stroke-opacity="0.7"`) {
		t.Errorf("explicit PATH alpha should be honored:\n%s", explicit)
	}
}

// TestHighlightSVGStrokeScale: stroke_scale multiplies the PATH marker width (WS9-044); 0/unset
// behaves as 1.
func TestHighlightSVGStrokeScale(t *testing.T) {
	g := highlightFixture()
	pathSpec := func(scale float32) []*geom.HighlightSpec {
		return []*geom.HighlightSpec{{Nets: []string{"NET1"}, Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_PATH, StrokeScale: scale}}
	}
	base := firstPolylineWidth(t, HighlightSVG(g, g.Sheets[0], pathSpec(0))) // unset -> 1
	scaled := firstPolylineWidth(t, HighlightSVG(g, g.Sheets[0], pathSpec(2)))
	if scaled != base*2 {
		t.Errorf("stroke_scale=2 width = %v, want 2x the unscaled %v", scaled, base)
	}
}

// TestPackSheetPinKeys: pin primitives carry the pin designator so pin-granularity
// highlight targeting can join them; symbol and wire primitives don't.
func TestPackSheetPinKeys(t *testing.T) {
	g := highlightFixture()
	ps := PackSheet(g, g.Sheets[0])
	var pins []string
	for _, k := range ps.Keys {
		if k.Pin != "" {
			pins = append(pins, k.RefDes+"."+k.Pin)
		}
	}
	want := "R1.1,R1.2,U1.1"
	if got := strings.Join(pins, ","); got != want {
		t.Errorf("pin keys = %q, want %q", got, want)
	}
}

// TestHighlightPackedEchoesShape: the packed group carries the spec's shape so an index-only
// consumer knows what to draw.
func TestHighlightPackedEchoesShape(t *testing.T) {
	g := highlightFixture()
	ps := PackSheet(g, g.Sheets[0])
	hl := HighlightPacked(ps, []*geom.HighlightSpec{
		{Components: []string{"R1"}, Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT},
		{Nets: []string{"NET1"}},
	})
	if got := hl.Groups[0].Shape; got != geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT {
		t.Errorf("group 0 shape = %v, want BOUNDING_RECT", got)
	}
	if got := hl.Groups[1].Shape; got != geom.HighlightShape_HIGHLIGHT_SHAPE_UNSPECIFIED {
		t.Errorf("group 1 shape = %v, want UNSPECIFIED (outline default)", got)
	}
}

// TestHighlightEntityFraming pins the twinned world-space framing numbers (the same
// constants are asserted in web/src/highlights.test.ts, so the Go SVG projection and the
// TS WebGL overlay frame entities identically): R1 = its rect + both pin points, padded by
// max(8, 10% of the larger side); NET1 = its wire, zero height so the floor pad applies in
// Y; a lone pin = a zero-area point framed by the floor pad.
func TestHighlightEntityFraming(t *testing.T) {
	g := highlightFixture()
	syms := indexSymbols(g)
	sheet := g.Sheets[0]

	cases := []struct {
		name                   string
		m                      specMatcher
		minX, minY, maxX, maxY float64 // padded world bounds
		cx, cy, r              float64 // bounding circle (center + circumscribed radius)
	}{
		// R1: rect (300,400)-(340,420) + pins (300,410),(340,410) -> 40x20, pad 8.
		{"component", matcherFor(&geom.HighlightSpec{Components: []string{"R1"}}),
			292, 392, 348, 428, 320, 410, 30.360679774997898},
		// NET1: wire (100,100)-(200,100) -> 100x0, pad 10.
		{"net", matcherFor(&geom.HighlightSpec{Nets: []string{"NET1"}}),
			90, 90, 210, 110, 150, 100, 60},
		// U1 pin 1: point (600,430) -> 0x0, floor pad 8.
		{"pin", matcherFor(&geom.HighlightSpec{Pins: []*geom.PinRef{{RefDes: "U1", Pin: "1"}}}),
			592, 422, 608, 438, 600, 430, 8},
	}
	for _, tc := range cases {
		ents := collectEntities(syms, sheet, tc.m)
		if len(ents) != 1 {
			t.Fatalf("%s: entities = %d, want 1", tc.name, len(ents))
		}
		fb, ok := framedBounds(ents[0])
		if !ok {
			t.Fatalf("%s: no bounds", tc.name)
		}
		if fb.minX != tc.minX || fb.minY != tc.minY || fb.maxX != tc.maxX || fb.maxY != tc.maxY {
			t.Errorf("%s: padded bounds = (%v,%v)-(%v,%v), want (%v,%v)-(%v,%v)",
				tc.name, fb.minX, fb.minY, fb.maxX, fb.maxY, tc.minX, tc.minY, tc.maxX, tc.maxY)
		}
		cx, cy, r := fb.circle()
		if cx != tc.cx || cy != tc.cy || r != tc.r {
			t.Errorf("%s: circle = (%v,%v) r=%v, want (%v,%v) r=%v", tc.name, cx, cy, r, tc.cx, tc.cy, tc.r)
		}
	}
}

// TestHighlightSVGBoundingShapes: a BOUNDING_RECT spec emits one translucent filled <rect>
// per matched entity (component and net — never one box over the union) and none of the
// outline re-strokes; BOUNDING_CIRCLE likewise emits per-entity filled circles. Unset alpha
// on a bounding shape defaults to 0.3, not opaque.
func TestHighlightSVGBoundingShapes(t *testing.T) {
	g := highlightFixture()
	rect := HighlightSVG(g, g.Sheets[0], []*geom.HighlightSpec{{
		Components: []string{"R1"}, Nets: []string{"NET1"},
		Color: "#00aaff", Alpha: 0.5, Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT,
	}})
	if n := strings.Count(rect, "<rect"); n != 2 {
		t.Errorf("bounding rects = %d, want 2 (one per entity)", n)
	}
	if strings.Contains(rect, "<polyline") {
		t.Error("bounding-rect overlay must not re-stroke wires")
	}
	if !strings.Contains(rect, `fill="#00aaff"`) || !strings.Contains(rect, `fill-opacity="0.5"`) {
		t.Error("bounding rect missing translucent fill in spec color/alpha")
	}

	circle := HighlightSVG(g, g.Sheets[0], []*geom.HighlightSpec{{
		Components: []string{"R1"}, Nets: []string{"NET1"},
		Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_CIRCLE,
	}})
	if n := strings.Count(circle, "<circle"); n != 2 {
		t.Errorf("bounding circles = %d, want 2 (one per entity)", n)
	}
	if strings.Contains(circle, "<rect") || strings.Contains(circle, "<polyline") {
		t.Error("bounding-circle overlay must not emit rects or re-strokes")
	}
	if !strings.Contains(circle, `fill-opacity="0.3"`) {
		t.Error("unset alpha on a bounding shape must default to 0.3")
	}
}

func equalU32(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
