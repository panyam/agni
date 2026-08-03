package graph

import (
	"path/filepath"
	"sort"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Built-in device class ids. Class ids are open strings, not a closed enum: a new class is just
// a rule plus a glyph (WS7-030), so users can extend classification without a code change. The
// empty id is the "no class" fallback that draws the generic node box.
const (
	ClassResistor   = "resistor"
	ClassCapacitor  = "capacitor"
	ClassInductor   = "inductor"
	ClassFerrite    = "ferrite"
	ClassDiode      = "diode"
	ClassLED        = "led"
	ClassTVS        = "tvs"
	ClassFuse       = "fuse"
	ClassConnector  = "connector"
	ClassTestPoint  = "test_point"
	ClassCrystal    = "crystal"
	ClassIC         = "ic"
	ClassTransistor = "transistor"
	ClassGround     = "ground"
	ClassOther      = "" // generic box
)

// ClassRule maps a component to a device class when it matches. A rule matches when every one
// of its non-empty conditions matches (AND); an all-empty rule never matches. Rules are tried in
// order and the first match wins, so order is precedence: put the strongest signal (the source
// symbol/part-type) and the more specific patterns first.
type ClassRule struct {
	// Class is the class id assigned on a match (one of the Class* ids or a user-defined id).
	Class string
	// Symbol is a shell glob (path/filepath.Match) matched, case-insensitively, against the
	// resolved PartType name / section part_ref, i.e. the source symbol (xschem "res", KiCad
	// "Device:R"). This is the authoritative signal and should be tried before Prefix.
	Symbol string
	// Prefix matches when the component's ref-des letter run STARTS WITH it (uppercased), so
	// "R" catches "R1", "RE1", and "RC2" alike. A multi-letter prefix ("LED") is more specific
	// and must be ordered before its single-letter counterpart ("L").
	Prefix string
}

// matches reports whether the rule applies to a component's (lowercased) source symbol name and
// (uppercased) ref-des letter prefix.
func (r ClassRule) matches(symbol, prefix string) bool {
	if r.Symbol == "" && r.Prefix == "" {
		return false
	}
	if r.Symbol != "" {
		if symbol == "" {
			return false
		}
		if ok, _ := filepath.Match(r.Symbol, symbol); !ok {
			return false
		}
	}
	if r.Prefix != "" && !strings.HasPrefix(prefix, strings.ToUpper(r.Prefix)) {
		return false
	}
	return true
}

// Registry is the data behind auto-layout node drawing: an ordered rule list that classifies a
// component, and a class-id -> glyph map that supplies the artwork. It is injected into the
// layout (WithRegistry) rather than hardcoded, so callers can extend classification and glyphs.
// The zero value is not usable; build one with DefaultRegistry (optionally .With user rules).
type Registry struct {
	Rules  []ClassRule
	Glyphs map[string]*geom.SymbolDef // class id -> glyph; a class with no entry draws the box
}

// DefaultRegistry is the built-in classifier and glyph set. Rules are ordered strongest-signal
// first: the source symbol/part-type name, then ref-des prefixes (multi-letter exceptions like
// LED before the single letters). Callers extend it with With; the CLI/serve edge layers user
// rules on top.
func DefaultRegistry() *Registry {
	return &Registry{
		Rules: []ClassRule{
			// Source symbol / part-type name (authoritative). Globs, matched lowercased.
			{Class: ClassResistor, Symbol: "res*"},
			{Class: ClassCapacitor, Symbol: "cap*"},
			{Class: ClassFerrite, Symbol: "*ferrite*"},
			{Class: ClassFerrite, Symbol: "*bead*"},
			{Class: ClassInductor, Symbol: "ind*"},
			{Class: ClassInductor, Symbol: "induc*"},
			{Class: ClassLED, Symbol: "led*"},
			{Class: ClassTVS, Symbol: "tvs*"},
			{Class: ClassTVS, Symbol: "esd*"},
			{Class: ClassDiode, Symbol: "diode*"},
			{Class: ClassTransistor, Symbol: "npn*"},
			{Class: ClassTransistor, Symbol: "pnp*"},
			{Class: ClassTransistor, Symbol: "*transistor*"},
			{Class: ClassFuse, Symbol: "fuse*"},
			{Class: ClassConnector, Symbol: "conn*"},
			{Class: ClassTestPoint, Symbol: "*testpoint*"},
			{Class: ClassTestPoint, Symbol: "*test_point*"},
			{Class: ClassCrystal, Symbol: "crystal*"},
			{Class: ClassCrystal, Symbol: "xtal*"},
			{Class: ClassCrystal, Symbol: "osc*"},
			{Class: ClassGround, Symbol: "gnd*"},
			{Class: ClassGround, Symbol: "ground*"},
			// Ref-des letter prefix (startswith). Ground/power first, then the multi-letter
			// prefixes whose first letter is claimed by another class (LED/L, TVS/T, TP/T,
			// FB/F, CR/CN vs C, IC/I), then the single letters.
			{Class: ClassGround, Prefix: "PWR"},
			{Class: ClassLED, Prefix: "LED"},
			{Class: ClassTVS, Prefix: "TVS"},
			{Class: ClassTestPoint, Prefix: "TP"},
			{Class: ClassFerrite, Prefix: "FB"},
			{Class: ClassDiode, Prefix: "CR"},
			{Class: ClassConnector, Prefix: "CN"},
			{Class: ClassIC, Prefix: "IC"},
			{Class: ClassResistor, Prefix: "R"},
			{Class: ClassCapacitor, Prefix: "C"},
			{Class: ClassInductor, Prefix: "L"},
			{Class: ClassDiode, Prefix: "D"},
			{Class: ClassTransistor, Prefix: "Q"},
			{Class: ClassFuse, Prefix: "F"},
			{Class: ClassConnector, Prefix: "J"},
			{Class: ClassConnector, Prefix: "P"},
			{Class: ClassIC, Prefix: "U"},
			{Class: ClassCrystal, Prefix: "Y"},
		},
		Glyphs: map[string]*geom.SymbolDef{
			ClassResistor:   resistorSymbol(),
			ClassCapacitor:  capacitorSymbol(),
			ClassInductor:   inductorSymbol(),
			ClassFerrite:    ferriteSymbol(),
			ClassDiode:      diodeSymbol(),
			ClassLED:        ledSymbol(),
			ClassTVS:        tvsSymbol(),
			ClassFuse:       fuseSymbol(),
			ClassConnector:  connectorSymbol(),
			ClassTestPoint:  testPointSymbol(),
			ClassCrystal:    crystalSymbol(),
			ClassIC:         icSymbol(),
			ClassTransistor: transistorSymbol(),
			ClassGround:     groundSymbol(),
		},
	}
}

// With returns a copy of the registry with the given rules prepended, so they take precedence
// over the built-in defaults (first match wins). The glyph map is shared: user rules map to the
// existing glyphs unless the caller also adds a glyph for a new class id.
func (r *Registry) With(rules ...ClassRule) *Registry {
	if len(rules) == 0 {
		return r
	}
	merged := make([]ClassRule, 0, len(rules)+len(r.Rules))
	merged = append(merged, rules...)
	merged = append(merged, r.Rules...)
	return &Registry{Rules: merged, Glyphs: r.Glyphs}
}

// GlyphClasses returns the class ids that have a glyph, sorted, for validating user input and
// for error messages.
func (r *Registry) GlyphClasses() []string {
	out := make([]string, 0, len(r.Glyphs))
	for c := range r.Glyphs {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// Classify returns the device class id for a component: the first rule that matches its source
// symbol/part-type name (preferred) or ref-des prefix, else ClassOther (the box). A resolved
// PartType's designator_prefix, when present, overrides the ref-des guess.
func (r *Registry) Classify(c *ir.Component, parts map[string]*ir.PartType) string {
	prefix := refDesPrefix(c.GetRefDes())
	symbol := ""
	if pt := resolvePart(c, parts); pt != nil {
		symbol = strings.ToLower(pt.GetName())
		if p := strings.ToUpper(pt.GetDesignatorPrefix()); p != "" {
			prefix = p
		}
	}
	for _, rule := range r.Rules {
		if rule.matches(symbol, prefix) {
			return rule.Class
		}
	}
	return ClassOther
}

// Symbol implements SymbolSource: it classifies the component and returns the class's glyph (the
// box for an unknown class), so the Registry is the synthetic-glyph symbol source for a layout.
func (r *Registry) Symbol(_ string, c *ir.Component, parts map[string]*ir.PartType) *geom.SymbolDef {
	class := ClassOther
	if c != nil {
		class = r.Classify(c, parts)
	}
	return r.glyphFor(class)
}

// glyphFor returns the glyph for a class, or the generic box when the class has no glyph.
func (r *Registry) glyphFor(class string) *geom.SymbolDef {
	if g := r.Glyphs[class]; g != nil {
		return g
	}
	return nodeSymbol()
}

// cellFor returns the placement cell_ref for a class: its glyph's cell, or the box cell.
func (r *Registry) cellFor(class string) string {
	return r.glyphFor(class).CellRef
}

// refDesPrefix is the leading run of letters of a ref-des (a leading '#', as in KiCad's
// "#PWR", is skipped), uppercased: "R12" -> "R", "RE1" -> "RE", "#PWR01" -> "PWR".
func refDesPrefix(ref string) string {
	ref = strings.TrimPrefix(ref, "#")
	i := 0
	for i < len(ref) {
		c := ref[i]
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			break
		}
		i++
	}
	return strings.ToUpper(ref[:i])
}

// partIndex maps a PartType name to its definition across all libraries, for resolving a
// component's referenced part.
func partIndex(d *ir.Design) map[string]*ir.PartType {
	m := make(map[string]*ir.PartType)
	for _, lib := range d.GetLibraries() {
		for _, pt := range lib.GetParts() {
			m[pt.GetName()] = pt
		}
	}
	return m
}

// resolvePart returns the first of a component's sections whose part_ref joins to a PartType,
// or nil when none resolves (the component then classifies by ref-des alone).
func resolvePart(c *ir.Component, parts map[string]*ir.PartType) *ir.PartType {
	for _, s := range c.GetSections() {
		if pt, ok := parts[s.GetPartRef()]; ok {
			return pt
		}
	}
	return nil
}

// glyphCell is the reserved cell_ref for a class's built-in glyph. Like nodeCell, no reader
// emits these, so they never collide with a real part.
func glyphCell(class string) string {
	if class == ClassOther {
		return nodeCell
	}
	return "__node:" + class + "__"
}

// pin is a PinPoint at (x,y) with a numeric designator. Auto-layout glyphs carry no pin-number
// label (label_origin nil), so the terminal stub is the only visible marker.
func pin(portRef string, x, y int64) *geom.PinPoint {
	return &geom.PinPoint{PortRef: portRef, Loc: &geom.Point{X: x, Y: y}}
}

// poly builds an open polyline shape through the given x,y pairs.
func poly(xy ...int64) *geom.Shape {
	pts := make([]*geom.Point, 0, len(xy)/2)
	for i := 0; i+1 < len(xy); i += 2 {
		pts = append(pts, &geom.Point{X: xy[i], Y: xy[i+1]})
	}
	return &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: pts}
}

// Node glyphs live in the same coordinate scale as the box node (halfNode), centered on the
// origin so a placement's transform origin is the node center. Terminals reach out to terminalX
// so net edges (WS7-026) will attach at the pin, not the body.
const (
	terminalX = 40 // half-width to the pin terminals
	bodyHalf  = 18 // half-height of a glyph body
)

// glyph assembles a SymbolDef with the standard node bounding box, the given shapes, and pins.
func glyph(class string, shapes []*geom.Shape, pins []*geom.PinPoint) *geom.SymbolDef {
	return &geom.SymbolDef{
		CellRef: glyphCell(class),
		Bbox:    &geom.BBox{Min: &geom.Point{X: -terminalX, Y: -bodyHalf}, Max: &geom.Point{X: terminalX, Y: bodyHalf}},
		Shapes:  shapes,
		Pins:    pins,
	}
}

// resistorSymbol is the US zigzag body with two horizontal terminal stubs.
func resistorSymbol() *geom.SymbolDef {
	return glyph(ClassResistor, []*geom.Shape{
		poly(-terminalX, 0, -20, 0, -15, 10, -5, -10, 5, 10, 15, -10, 20, 0, terminalX, 0),
	}, []*geom.PinPoint{pin("1", -terminalX, 0), pin("2", terminalX, 0)})
}

// capacitorSymbol is two parallel plates with terminal stubs (non-polarized).
func capacitorSymbol() *geom.SymbolDef {
	return glyph(ClassCapacitor, []*geom.Shape{
		poly(-terminalX, 0, -6, 0), // left stub
		poly(-6, -15, -6, 15),      // left plate
		poly(6, -15, 6, 15),        // right plate
		poly(6, 0, terminalX, 0),   // right stub
	}, []*geom.PinPoint{pin("1", -terminalX, 0), pin("2", terminalX, 0)})
}

// inductorSymbol is three rounded humps between two terminal stubs. The humps are
// semicircle-shaped (not angular) so the coil reads as an inductor, not a resistor zigzag.
func inductorSymbol() *geom.SymbolDef {
	return glyph(ClassInductor, []*geom.Shape{
		poly(-terminalX, 0,
			-21, 0, -20, 4, -17, 6, -14, 7, -11, 6, -8, 4, -7, 0, // hump 1
			-6, 4, -3, 6, 0, 7, 3, 6, 6, 4, 7, 0, // hump 2
			8, 4, 11, 6, 14, 7, 17, 6, 20, 4, 21, 0, // hump 3
			terminalX, 0),
	}, []*geom.PinPoint{pin("1", -terminalX, 0), pin("2", terminalX, 0)})
}

// diodeSymbol is an anode-to-cathode triangle with a cathode bar and terminal stubs. Pin 1 is
// the anode (left), pin 2 the cathode (right).
func diodeSymbol() *geom.SymbolDef {
	return glyph(ClassDiode, []*geom.Shape{
		poly(-terminalX, 0, -10, 0),              // anode stub
		poly(-10, -12, -10, 12, 10, 0, -10, -12), // triangle (closed)
		poly(10, -12, 10, 12),                    // cathode bar
		poly(10, 0, terminalX, 0),                // cathode stub
	}, []*geom.PinPoint{pin("1", -terminalX, 0), pin("2", terminalX, 0)})
}

// transistorSymbol is a simple NPN: base bar on the left with collector/emitter leads. Pin 1 is
// the base (left), pin 2 the collector (top-right), pin 3 the emitter (bottom-right).
func transistorSymbol() *geom.SymbolDef {
	return glyph(ClassTransistor, []*geom.Shape{
		poly(-terminalX, 0, -12, 0),     // base stub
		poly(-12, -16, -12, 16),         // base bar
		poly(-12, -8, 18, -18, 18, -30), // collector lead
		poly(-12, 8, 18, 18, 18, 30),    // emitter lead
	}, []*geom.PinPoint{pin("1", -terminalX, 0), pin("2", 18, -30), pin("3", 18, 30)})
}

// ferriteSymbol is the slanted solid bead (a tilted rectangle) between two terminal stubs,
// distinct from the inductor's humps.
func ferriteSymbol() *geom.SymbolDef {
	return glyph(ClassFerrite, []*geom.Shape{
		poly(-terminalX, 0, -13, 0),
		poly(-15, 8, -7, -12, 15, -8, 7, 12, -15, 8), // tilted bead body (closed)
		poly(13, 0, terminalX, 0),
	}, []*geom.PinPoint{pin("1", -terminalX, 0), pin("2", terminalX, 0)})
}

// ledSymbol is the diode body plus two emission arrows leaving the triangle.
func ledSymbol() *geom.SymbolDef {
	return glyph(ClassLED, []*geom.Shape{
		poly(-terminalX, 0, -10, 0),
		poly(-10, -12, -10, 12, 10, 0, -10, -12),
		poly(10, -12, 10, 12),
		poly(10, 0, terminalX, 0),
		poly(-2, -13, 5, -22), poly(5, -22, 0, -21), poly(5, -22, 4, -16), // arrow 1
		poly(6, -9, 13, -18), poly(13, -18, 8, -17), poly(13, -18, 12, -12), // arrow 2
	}, []*geom.PinPoint{pin("1", -terminalX, 0), pin("2", terminalX, 0)})
}

// tvsSymbol is the bidirectional suppressor: two triangles meeting a shared center bar.
func tvsSymbol() *geom.SymbolDef {
	return glyph(ClassTVS, []*geom.Shape{
		poly(-terminalX, 0, -24, 0),
		poly(-24, -12, -24, 12, 0, 0, -24, -12), // left triangle (closed)
		poly(0, -14, 0, 14),                     // shared bar
		poly(24, -12, 24, 12, 0, 0, 24, -12),    // right triangle (closed)
		poly(24, 0, terminalX, 0),
	}, []*geom.PinPoint{pin("1", -terminalX, 0), pin("2", terminalX, 0)})
}

// fuseSymbol is the IEC box with the element line running straight through it.
func fuseSymbol() *geom.SymbolDef {
	return glyph(ClassFuse, []*geom.Shape{
		poly(-terminalX, 0, terminalX, 0),             // element + stubs, one run
		poly(-20, -8, 20, -8, 20, 8, -20, 8, -20, -8), // body (closed)
	}, []*geom.PinPoint{pin("1", -terminalX, 0), pin("2", terminalX, 0)})
}

// connectorSymbol is an open socket bracket facing the board (right); multi-pin connectors
// carry no per-pin terminals, so edges attach at the node center like the box.
func connectorSymbol() *geom.SymbolDef {
	return glyph(ClassConnector, []*geom.Shape{
		poly(-terminalX, 0, -10, 0),
		poly(10, -16, -10, -16, -10, 16, 10, 16),                  // open bracket
		poly(2, -8, -2, -8), poly(2, 0, -2, 0), poly(2, 8, -2, 8), // socket ticks
	}, nil)
}

// testPointSymbol is the probe loop: a small circle on a stub, pin at the stub end.
func testPointSymbol() *geom.SymbolDef {
	return glyph(ClassTestPoint, []*geom.Shape{
		poly(0, 20, 0, 6),
		{Kind: geom.Shape_KIND_CIRCLE, Points: []*geom.Point{{X: 0, Y: 0}}, Radius: 6},
	}, []*geom.PinPoint{pin("1", 0, 20)})
}

// crystalSymbol is the quartz plate between its two electrode bars.
func crystalSymbol() *geom.SymbolDef {
	return glyph(ClassCrystal, []*geom.Shape{
		poly(-terminalX, 0, -12, 0),
		poly(-12, -14, -12, 14),                       // left electrode
		poly(-6, -12, 6, -12, 6, 12, -6, 12, -6, -12), // plate (closed)
		poly(12, -14, 12, 14),                         // right electrode
		poly(12, 0, terminalX, 0),
	}, []*geom.PinPoint{pin("1", -terminalX, 0), pin("2", terminalX, 0)})
}

// icSymbol is a chip body with a pin-1 index notch; like the box (and the connector), it
// carries no per-pin terminals, so edges attach at the node center.
func icSymbol() *geom.SymbolDef {
	return glyph(ClassIC, []*geom.Shape{
		poly(-16, -14, 16, -14, 16, 14, -16, 14, -16, -14), // body (closed)
		poly(-4, -14, -4, -10, 4, -10, 4, -14),             // index notch
	}, nil)
}

// groundSymbol is the three descending bars with a single terminal stub above.
func groundSymbol() *geom.SymbolDef {
	return glyph(ClassGround, []*geom.Shape{
		poly(0, 20, 0, 0),   // stub down to the top bar
		poly(-12, 0, 12, 0), // widest bar
		poly(-8, -6, 8, -6),
		poly(-4, -12, 4, -12),
	}, []*geom.PinPoint{pin("1", 0, 20)})
}
