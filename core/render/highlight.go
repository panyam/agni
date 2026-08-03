package render

import (
	"math"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	geomath "github.com/panyam/agni/internal/geomath"
	"github.com/panyam/agni/core/svg"
)

// The highlight layer: a renderer-agnostic selection of elements ("these components, nets,
// and pins, in this color/alpha", geom.HighlightSpec) projected into a drawable overlay,
// decoupled from the base sheet render. Two projections mirror the two render backends:
// HighlightPacked joins a PackedSheet by primitive index (a GPU renderer redraws just those
// primitives), and HighlightSVG emits a transparent SVG document in the exact frame of
// SheetSVG (an SVG client stacks it above the sheet, or a server composites the layers).
// Both resolve the same spec semantics, so what lights up is backend-independent.

// highlightAlpha normalizes a spec alpha: values outside (0, 1) mean fully opaque (proto3
// cannot distinguish unset from 0, and an invisible highlight is never what a caller means).
func highlightAlpha(a float32) float64 {
	if a <= 0 || a >= 1 {
		return 1
	}
	return float64(a)
}

// highlightPathAlpha normalizes a PATH-style alpha (WS9-040): unset/out-of-range means
// translucent (0.4), not opaque, so the focus highlighter marks a net's path like a marker
// pen and the wire underneath stays visible. An explicit in-range alpha is honored.
func highlightPathAlpha(a float32) float64 {
	if a <= 0 || a >= 1 {
		return 0.4
	}
	return float64(a)
}

// highlightColor normalizes a spec color, defaulting an empty one.
func highlightColor(c string) string {
	if c == "" {
		return DefaultHighlightColor
	}
	return c
}

// specMatcher is one spec's target sets, indexed for O(1) membership tests.
type specMatcher struct {
	comps  map[string]bool
	nets   map[string]bool
	netIDs map[string]bool // per-instance net ids (WS9); a spec targeting one net instance lists it here
	busIDs map[string]bool // bus source ids (WS7-042b); a bus joins by uuid, having no net
	pins   map[[2]string]bool // {ref_des, pin}
}

func matcherFor(spec *geom.HighlightSpec) specMatcher {
	m := specMatcher{comps: map[string]bool{}, nets: map[string]bool{}, netIDs: map[string]bool{}, busIDs: map[string]bool{}, pins: map[[2]string]bool{}}
	for _, c := range spec.GetComponents() {
		m.comps[c] = true
	}
	for _, n := range spec.GetNets() {
		m.nets[n] = true
	}
	for _, id := range spec.GetNetIds() {
		m.netIDs[id] = true
	}
	for _, id := range spec.GetBusIds() {
		m.busIDs[id] = true
	}
	for _, p := range spec.GetPins() {
		m.pins[[2]string{p.GetRefDes(), p.GetPin()}] = true
	}
	return m
}

// matchWire reports whether a wire with the given net name and per-instance id falls in the spec:
// by id when the spec lists ids (so it targets ONE of two same-named nets), OR by name when it
// lists names (the whole-selection highlight). A spec that lists only ids never matches by name, so
// per-instance focus does not bleed onto a sibling net that shares the name (WS9).
func (m specMatcher) matchWire(net, netID string) bool {
	if netID != "" && m.netIDs[netID] {
		return true
	}
	return net != "" && m.nets[net]
}

// matchBus reports whether a bus (trunk or entry) with the given name falls in the spec. A bus
// carries no net, so its NAME (the range-label identity on WireGeometry.Net / PrimitiveKey.bus_id) is
// its only join key (WS7-042b).
func (m specMatcher) matchBus(busID string) bool {
	return busID != "" && m.busIDs[busID]
}

// isBusKind reports whether a wire geometry kind is a bus trunk or entry, so bus matching gates on
// the construct kind rather than the name field a net wire also fills.
func isBusKind(k geom.WireGeometry_Kind) bool {
	return k == geom.WireGeometry_KIND_BUS || k == geom.WireGeometry_KIND_BUS_ENTRY
}

// matchKey reports whether a packed primitive key falls in the spec's selection: a listed
// component matches all of its primitives (symbol graphics and pins), a listed net matches
// its wire polylines, and a listed pin matches just that pin's primitive.
func (m specMatcher) matchKey(k *geom.PrimitiveKey) bool {
	if k.GetRefDes() != "" && m.comps[k.GetRefDes()] {
		return true
	}
	if m.matchWire(k.GetNet(), k.GetNetId()) {
		return true
	}
	if m.matchBus(k.GetBusId()) {
		return true
	}
	return k.GetPin() != "" && m.pins[[2]string{k.GetRefDes(), k.GetPin()}]
}

// HighlightPacked resolves highlight specs against one packed sheet and returns, per spec
// (same order), the indices of the primitives it selects plus the spec's style. The caller
// redraws those primitives above the base geometry; no vertices are duplicated, the group
// joins PackedSheet.vertices/primitives by index. An element matched by several specs
// belongs to each matching group; paint order (later spec wins) is the renderer's job.
func HighlightPacked(sheet *geom.PackedSheet, specs []*geom.HighlightSpec) *geom.PackedHighlight {
	out := &geom.PackedHighlight{}
	for _, spec := range specs {
		m := matcherFor(spec)
		g := &geom.PackedHighlight_Group{Color: spec.GetColor(), Alpha: spec.GetAlpha(), Shape: spec.GetShape()}
		for _, k := range sheet.GetKeys() {
			if m.matchKey(k) {
				g.Primitives = append(g.Primitives, k.GetPrimitive())
			}
		}
		out.Groups = append(out.Groups, g)
	}
	return out
}

// highlightStrokePx is the overlay stroke width: wider than the base render's strokePx so
// the highlight reads as a halo around the element rather than repainting its line.
const highlightStrokePx = strokePx * 3

// pathStrokePx is the PATH highlighter's stroke width (WS9-040): about twice the outline
// halo, so a focused net's translucent path reads as a marker stroke, not a thin re-stroke.
const pathStrokePx = highlightStrokePx * 2

// highlightEntity is one matched entity — a component (its placed symbol shapes plus its
// pin connect points), a net (every polyline of its wires, sheet-wide), or a single listed
// pin — resolved to world coordinates. Strategies draw entities, never raw sheet elements,
// so per-entity framing works the same for every shape: a bounding rect frames one
// component or one net, not the union of everything the spec matched.
type highlightEntity struct {
	shapes    []*geom.Shape
	polylines []*geom.Polyline
	pins      []*geom.Point
}

// highlightStrategy is HighlightSVG's shape dispatch: how one matched entity draws into
// the overlay. outlineStrategy re-strokes the entity's own geometry (the WS9-016 behavior
// and the default); the bounding strategies frame its bounds with a translucent fill.
type highlightStrategy interface {
	// normAlpha normalizes the spec's raw alpha for this strategy: outline treats
	// unset/out-of-range as opaque, the bounding shapes as 0.3 (an opaque box would hide
	// the entity it frames).
	normAlpha(a float32) float64
	// entity draws one matched entity in the spec's style.
	entity(c *svg.Canvas, e *highlightEntity, fr sheetFrame, color string, alpha float64)
}

// collectEntities resolves one spec's selection against a sheet into entities, in paint
// order: matched nets first (all of a net's wires merge into one entity, first-seen wire
// order), then placements (a matched component is one entity holding its shapes and every
// pin point; an individually listed pin is its own single-point entity).
func collectEntities(syms map[string]*geom.SymbolDef, sheet *geom.SheetGeometry, m specMatcher) []*highlightEntity {
	var out []*highlightEntity
	byNet := map[string]*highlightEntity{}
	for _, w := range sheet.Wires {
		// A bus (WS7-042b) joins by its NAME (its range-label identity), not a net; group its
		// segments into one entity so an OUTLINE re-stroke recolors the whole bus. Gated on the bus
		// kind so a net wire that happens to share the name never enters this branch.
		if isBusKind(w.GetKind()) && m.matchBus(w.GetNet()) {
			key := "bus:" + w.GetNet()
			e := byNet[key]
			if e == nil {
				e = &highlightEntity{}
				byNet[key] = e
				out = append(out, e)
			}
			e.polylines = append(e.polylines, w.Polylines...)
			continue
		}
		if !m.matchWire(w.GetNet(), w.GetNetId()) {
			continue
		}
		// Group wires per net INSTANCE (by id when present, else name), so two same-named nets
		// frame as two entities, not one merged span (WS9).
		key := w.GetNet()
		if w.GetNetId() != "" {
			key = w.GetNetId()
		}
		e := byNet[key]
		if e == nil {
			e = &highlightEntity{}
			byNet[key] = e
			out = append(out, e)
		}
		e.polylines = append(e.polylines, w.Polylines...)
	}
	for _, pl := range sheet.Placements {
		wholeComp := m.comps[pl.GetRefDes()]
		var comp *highlightEntity
		if wholeComp {
			comp = &highlightEntity{shapes: placedShapes(syms, pl)}
			out = append(out, comp)
		}
		sym := symbolFor(syms, pl)
		if sym == nil {
			continue
		}
		for _, pin := range sym.Pins {
			if !wholeComp && !m.pins[[2]string{pl.GetRefDes(), pin.GetPortRef()}] {
				continue
			}
			wp := geomath.PlacePin(pl.Transform, pin)
			if wp == nil {
				continue
			}
			if wholeComp {
				comp.pins = append(comp.pins, wp)
			} else {
				out = append(out, &highlightEntity{pins: []*geom.Point{wp}})
			}
		}
	}
	return out
}

// outlineStrategy re-strokes the entity's own geometry: wires and symbol outlines get a
// wider stroke in the spec's color/alpha, pins a filled dot at the connect point.
type outlineStrategy struct{}

func (outlineStrategy) normAlpha(a float32) float64 { return highlightAlpha(a) }

func (outlineStrategy) entity(c *svg.Canvas, e *highlightEntity, fr sheetFrame, color string, alpha float64) {
	strokeEntity(c, e, fr, color, alpha, highlightStrokePx)
}

// pathStrategy is the focus highlighter (WS9-040): the same re-stroke as outlineStrategy but
// a wider, translucent-by-default marker, so focusing a net paints its path like a
// highlighter pen instead of an opaque bar. Only nets carry it (withFocusShape), but it draws
// any entity's geometry the same way outline does, just wider. width is pathStrokePx times the
// spec's stroke_scale (WS9-044), so a user can tune the marker thickness.
type pathStrategy struct{ width float64 }

func (pathStrategy) normAlpha(a float32) float64 { return highlightPathAlpha(a) }

func (p pathStrategy) entity(c *svg.Canvas, e *highlightEntity, fr sheetFrame, color string, alpha float64) {
	strokeEntity(c, e, fr, color, alpha, p.width)
}

// strokeScaleOr1 normalizes a spec's stroke_scale: 0/unset or a non-positive value means 1
// (the default width), matching the proto3 "cannot tell unset from 0" convention.
func strokeScaleOr1(s float32) float64 {
	if s <= 0 {
		return 1
	}
	return float64(s)
}

// strokeEntity re-strokes one matched entity at the given width: wires and symbol outlines as
// strokes, pins as a filled dot at the connect point. Shared by the outline and path
// strategies (they differ only in stroke width and the default alpha).
func strokeEntity(c *svg.Canvas, e *highlightEntity, fr sheetFrame, color string, alpha, width float64) {
	for _, pl := range e.polylines {
		c.El("polyline", svg.A("fill", "none"), svg.A("stroke", color),
			svg.F("stroke-opacity", alpha), svg.F("stroke-width", width),
			svg.A("points", points(pl.Points, fr.tx, fr.ty)))
	}
	for _, s := range e.shapes {
		writeHighlightShape(c, s, fr, color, alpha, width)
	}
	for _, p := range e.pins {
		c.El("circle", svg.F("cx", fr.tx(p.X)), svg.F("cy", fr.ty(p.Y)),
			svg.F("r", pinRPx*1.6), svg.A("fill", color), svg.F("fill-opacity", alpha))
	}
}

// HighlightSVG resolves highlight specs against one tier-1 sheet and returns a standalone
// transparent SVG overlay document with the exact size, viewBox, and world->pixel mapping
// of SheetSVG for the same sheet (both use frameSheet), so a client can stack it above the
// base document (or a compositor can merge the layers) and every highlight lands on its
// element. Specs paint in order, so a later spec wins where selections overlap. Each spec's
// shape picks the strategy its entities draw with (outline when unset).
func HighlightSVG(g *geom.SchematicGeometry, sheet *geom.SheetGeometry, specs []*geom.HighlightSpec) string {
	syms := indexSymbols(g)
	fr := frameSheet(sheet, syms)
	c := svg.Open(fr.pxW, fr.pxH) // no background rect: the overlay is transparent

	for _, spec := range specs {
		m := matcherFor(spec)
		strat := strategyFor(spec.GetShape(), spec.GetStrokeScale())
		color := highlightColor(spec.GetColor())
		alpha := strat.normAlpha(spec.GetAlpha())
		for _, e := range collectEntities(syms, sheet, m) {
			strat.entity(c, e, fr, color, alpha)
		}
	}
	return c.String()
}

// strategyFor maps a spec's shape to its strategy; unspecified draws as outline. strokeScale
// tunes the PATH width (WS9-044); the other strategies ignore it.
func strategyFor(shape geom.HighlightShape, strokeScale float32) highlightStrategy {
	switch shape {
	case geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT:
		return boundingRectStrategy{}
	case geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_CIRCLE:
		return boundingCircleStrategy{}
	case geom.HighlightShape_HIGHLIGHT_SHAPE_PATH:
		return pathStrategy{width: pathStrokePx * strokeScaleOr1(strokeScale)}
	default:
		return outlineStrategy{}
	}
}

// highlightFillAlpha normalizes a bounding-shape alpha: unset/out-of-range means 0.3, not
// opaque — an opaque fill would hide the entity the shape frames.
func highlightFillAlpha(a float32) float64 {
	if a <= 0 || a >= 1 {
		return 0.3
	}
	return float64(a)
}

// framedBox is an entity's framing geometry in world coordinates: the raw bbox padded by
// the twinned margin, plus the circumscribed framing circle.
type framedBox struct {
	minX, minY, maxX, maxY float64
	cx, cy, r              float64
}

// circle returns the framing circle: the raw bbox's center and its half-diagonal plus the
// pad, so the circle never clips the entity it frames (circumscribed, not inscribed).
func (b framedBox) circle() (cx, cy, r float64) {
	return b.cx, b.cy, b.r
}

// framedBounds resolves an entity's world geometry to its framing box and circle. The pad
// is 10% of the raw bbox's larger side with a floor of 8 world units, so a zero-area
// entity (a single pin) still gets a visible frame. The formula is TWINNED with
// entityFrame in web/src/highlights.ts — the fixture in highlight_test.go /
// highlights.test.ts asserts the same numbers on both sides; change them together.
// Shapes with a radius (circles, arcs) expand each of their points by it — exact for
// circles, conservative for arcs. ok is false for an entity with no geometry.
func framedBounds(e *highlightEntity) (framedBox, bool) {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	add := func(x, y, r float64) {
		minX = min(minX, x-r)
		minY = min(minY, y-r)
		maxX = max(maxX, x+r)
		maxY = max(maxY, y+r)
	}
	for _, pl := range e.polylines {
		for _, p := range pl.Points {
			add(float64(p.X), float64(p.Y), 0)
		}
	}
	for _, s := range e.shapes {
		for _, p := range s.Points {
			add(float64(p.X), float64(p.Y), float64(s.Radius))
		}
	}
	for _, p := range e.pins {
		add(float64(p.X), float64(p.Y), 0)
	}
	if minX > maxX {
		return framedBox{}, false
	}
	w, h := maxX-minX, maxY-minY
	pad := max(8, 0.1*max(w, h))
	// Explicit sqrt, not math.Hypot: mul/add/sqrt are correctly rounded IEEE ops in both
	// Go and JS, so the twinned fixtures can assert exact equality across backends
	// (JS engines do not guarantee a correctly rounded Math.hypot).
	return framedBox{
		minX: minX - pad, minY: minY - pad, maxX: maxX + pad, maxY: maxY + pad,
		cx: (minX + maxX) / 2, cy: (minY + maxY) / 2, r: math.Sqrt(w/2*(w/2)+h/2*(h/2)) + pad,
	}, true
}

// worldToPx maps fractional world coordinates through the frame's affine int64 maps
// (padded bounds are fractional; tx/ty take int64): tx is linear so tx(x) = tx(0) +
// x*scale, and ty flips Y so ty(y) = ty(0) - y*scale.
func worldToPx(fr sheetFrame, x, y float64) (float64, float64) {
	return fr.tx(0) + x*fr.scale, fr.ty(0) - y*fr.scale
}

// boundingRectStrategy frames each matched entity with one translucent filled rect over
// its padded bbox — area emphasis for dense entities where a re-stroke reads poorly.
type boundingRectStrategy struct{}

func (boundingRectStrategy) normAlpha(a float32) float64 { return highlightFillAlpha(a) }

func (boundingRectStrategy) entity(c *svg.Canvas, e *highlightEntity, fr sheetFrame, color string, alpha float64) {
	b, ok := framedBounds(e)
	if !ok {
		return
	}
	x, y := worldToPx(fr, b.minX, b.maxY) // world top-left is the px top-left (Y flips)
	c.El("rect", svg.F("x", x), svg.F("y", y),
		svg.F("width", (b.maxX-b.minX)*fr.scale), svg.F("height", (b.maxY-b.minY)*fr.scale),
		svg.A("fill", color), svg.F("fill-opacity", alpha))
}

// boundingCircleStrategy frames each matched entity with one translucent filled circle
// circumscribing its bbox.
type boundingCircleStrategy struct{}

func (boundingCircleStrategy) normAlpha(a float32) float64 { return highlightFillAlpha(a) }

func (boundingCircleStrategy) entity(c *svg.Canvas, e *highlightEntity, fr sheetFrame, color string, alpha float64) {
	b, ok := framedBounds(e)
	if !ok {
		return
	}
	cx, cy, r := b.circle()
	px, py := worldToPx(fr, cx, cy)
	c.El("circle", svg.F("cx", px), svg.F("cy", py), svg.F("r", r*fr.scale),
		svg.A("fill", color), svg.F("fill-opacity", alpha))
}

// writeHighlightShape re-strokes one placed shape in the highlight style: outline only
// (never filled, so the element underneath stays readable), wider stroke, spec color/alpha.
func writeHighlightShape(c *svg.Canvas, s *geom.Shape, fr sheetFrame, color string, alpha, width float64) {
	stroke := []svg.Attr{
		svg.A("fill", "none"), svg.A("stroke", color),
		svg.F("stroke-opacity", alpha), svg.F("stroke-width", width),
	}
	switch s.Kind {
	case geom.Shape_KIND_RECT:
		if len(s.Points) < 2 {
			return
		}
		x0, y0 := fr.tx(s.Points[0].X), fr.ty(s.Points[0].Y)
		x1, y1 := fr.tx(s.Points[1].X), fr.ty(s.Points[1].Y)
		c.El("rect", append(stroke,
			svg.F("x", min(x0, x1)), svg.F("y", min(y0, y1)),
			svg.F("width", abs(x1-x0)), svg.F("height", abs(y1-y0)))...)
	case geom.Shape_KIND_CIRCLE:
		if len(s.Points) < 1 {
			return
		}
		c.El("circle", append(stroke,
			svg.F("cx", fr.tx(s.Points[0].X)), svg.F("cy", fr.ty(s.Points[0].Y)),
			svg.F("r", float64(s.Radius)*fr.scale))...)
	case geom.Shape_KIND_DOT:
		if len(s.Points) < 1 {
			return
		}
		c.El("circle", svg.F("cx", fr.tx(s.Points[0].X)), svg.F("cy", fr.ty(s.Points[0].Y)),
			svg.F("r", width*2), svg.A("fill", color), svg.F("fill-opacity", alpha))
	case geom.Shape_KIND_POLYLINE, geom.Shape_KIND_ARC:
		if len(s.Points) == 0 {
			return
		}
		c.El("polyline", append(stroke, svg.A("points", points(s.Points, fr.tx, fr.ty)))...)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
