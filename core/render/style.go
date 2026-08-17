package render

// Style is the render layer's color palette and default font: view policy expressed as data.
// Both the SVG backend and the WebGL text-label colors resolve from one Style, so the two
// renderers agree by construction rather than by copied literals, and a caller can override the
// defaults (theming, dark mode, accessibility) with WithStyle. Colors are "#rrggbb"; Font is a
// CSS/SVG font-family.
type Style struct {
	// Geometry line/stroke colors.
	Wire   string // wire polylines
	Bus    string // bus trunks / entries (WS7-042), drawn thicker than a wire
	Symbol string // symbol graphics stroke (and dots)
	Pin    string // pin connect dots
	Frame  string // worksheet frame + zone-ruler tick lines
	Free   string // sheet-level free graphics (junction dots, no-connects, notes)

	// Text colors.
	Field      string // ref-des / value / custom placement fields
	Label      string // sheet labels (net labels, page text)
	Annotation string // symbol annotations and title-block field text
	Ruler      string // zone-ruler numbers/letters and pin numbers
	PinName    string // pin names

	// Fill colors.
	ShapeFill      string // default fill for a filled symbol shape
	Page           string // page background
	TitleBlockFill string // title-block box fill

	// Board colors (WS7-034): the BoardSVG strata. Copper follows the KiCad reading habit
	// (front red, back blue); inner layers share one muted tone until per-layer theming is
	// needed. Via colors the barrel ring and through-hole pad lands; the drill hole is Page.
	BoardOutline string // Edge.Cuts outline
	CopperFront  string // F.Cu segments, SMD pads on the front
	CopperBack   string // B.Cu
	CopperInner  string // any other signal layer
	Via          string // via rings + through-hole pad lands
	Silk         string // silkscreen / fab body-outline graphics (WS7)

	// Font is the default font-family for all text: a CSS family list, not one face
	// (see SchematicFontStack). No per-element font yet.
	Font string

	// PinDots draws a dot at every pin connect-point (the Pin color). It is a verification aid
	// for the pin-on-wire eyeball check, off by default because Eeschema draws no such dots and
	// a hardware engineer reads them as noise (WS7-017). Real junction dots are sheet Shapes and
	// draw regardless. Enable with WithPinDots for the eyeball check.
	PinDots bool
	// PickTargets emits an invisible, keyed circle at every pin so a viewer can CLICK a pin.
	//
	// Off by default, and the default is the point. Wire and symbol keys ride on elements the render
	// already draws, so they cost nothing and any consumer of the SVG gains them. A pin has no drawn
	// element of its own in a faithful render (the dot above is a verification aid Eeschema does not
	// draw), so making pins pickable means ADDING elements — one per pin, which on a large sheet is
	// the biggest single contributor to document size. A report embedding a sheet, or a diff artifact,
	// should not pay for the viewer's interaction model; the viewer asks for it (WithPickTargets).
	PickTargets bool
}

// GroupColor returns the geometry color for a packed primitive group (the groupSymbol..
// groupFrame constants). Both renderers use it: the SVG backend picks per shape, and the
// packer sends the whole set to the WebGL renderer (PackedSheet.group_colors), so geometry is
// colored from one Style on both surfaces. Unknown groups fall back to Symbol.
func (s Style) GroupColor(group uint8) string {
	switch group {
	case groupWire:
		return s.Wire
	case groupBus:
		return s.Bus
	case groupPin:
		return s.Pin
	case groupFree:
		return s.Free
	case groupFrame:
		return s.Frame
	default:
		return s.Symbol
	}
}

// SchematicFontStack is the default font-family for schematic text: a CSS fallback chain rather
// than one face. Arial leads because the authoring tools this engine reads print their schematics
// in it, and it ships with Windows and macOS, so a viewer gets the real face with no setup.
// Liberation Sans follows because it is metric-compatible with Arial and OFL-licensed, so a Linux
// box or CI runner with no Arial installed lays text out at identical advance widths instead of
// drifting per machine. The engine never loads or distributes a font file; this is only a name the
// viewer resolves locally.
//
// Family names are SINGLE-quoted deliberately: svg.Attr writes its value verbatim inside double
// quotes, so a double-quoted family name would emit invalid XML on the SVG root element. CSS
// accepts either quote, so single quotes cost nothing and survive both surfaces.
const SchematicFontStack = "Arial, 'Liberation Sans', Helvetica, sans-serif"

// DefaultStyle is the built-in palette and font, matching the schematic scheme the SVG backend
// has always drawn. Override per render call with WithStyle.
var DefaultStyle = Style{
	Wire:           "#0a7d2c",
	Bus:            "#1a4de0",
	Symbol:         "#222222",
	Pin:            "#dd1111",
	Frame:          "#8a6d3b",
	Field:          "#1560bd",
	Label:          "#555555",
	Annotation:     "#333333",
	Ruler:          "#8a6d3b",
	PinName:        "#2a7d5a",
	Free:           "#333333",
	ShapeFill:      "#fdf6d0",
	Page:           "#fdfdfb",
	BoardOutline:   "#8a6d3b",
	CopperFront:    "#c83434",
	CopperBack:     "#3050c8",
	CopperInner:    "#8a8a8a",
	Via:            "#9aa0a6",
	Silk:           "#7a7f88",
	TitleBlockFill: "#fffef8",
	Font:           SchematicFontStack,
}

// DarkStyle is a dark-background preset, useful for demonstrating that a WithStyle override
// recolors every surface (SVG, WebGL geometry, and the text overlay) from one palette.
var DarkStyle = Style{
	Wire:           "#4ade80",
	Bus:            "#60a5fa",
	Symbol:         "#e5e5e5",
	Pin:            "#f87171",
	Frame:          "#b0895a",
	Free:           "#bbbbbb",
	Field:          "#7aa2f7",
	Label:          "#aaaaaa",
	Annotation:     "#dddddd",
	Ruler:          "#b0895a",
	PinName:        "#5eead4",
	ShapeFill:      "#1a1a1a",
	Page:           "#111111",
	BoardOutline:   "#b0895a",
	CopperFront:    "#f87171",
	CopperBack:     "#7aa2f7",
	CopperInner:    "#9aa0a6",
	Via:            "#c0c6cc",
	Silk:           "#aab0bb",
	TitleBlockFill: "#1a1a1a",
	Font:           SchematicFontStack,
}

// Themes maps a theme name to its Style, for the `agni serve --theme` knob.
var Themes = map[string]Style{"default": DefaultStyle, "dark": DarkStyle}

// Option configures one render call. With no options a render uses DefaultStyle.
type Option func(*Style)

// WithStyle overrides the palette and font for one render call.
func WithStyle(s Style) Option { return func(dst *Style) { *dst = s } }

// WithPinDots turns the per-pin verification dots on for one render call (they are off by
// default; see Style.PinDots). It applies after any WithStyle, so it re-enables the dots
// even when a preset Style left them off.
func WithPinDots() Option { return func(dst *Style) { dst.PinDots = true } }

// WithPickTargets emits the invisible per-pin pick targets (see Style.PickTargets). The served
// viewer asks for them; a render destined for a file or a report does not.
func WithPickTargets() Option { return func(dst *Style) { dst.PickTargets = true } }

// resolveStyle applies the options over a copy of DefaultStyle.
func resolveStyle(opts []Option) Style {
	s := DefaultStyle
	for _, o := range opts {
		o(&s)
	}
	return s
}

// DefaultHighlightColor is the color a highlight spec with no color gets: a saturated
// magenta that stands out against the schematic palette (wires/symbols are dark or primary
// colors). It matches the tint the WebGL viewer has always used for finding highlights, and
// lives here so every drawable color stays in one file (CONSTRAINTS C12); the web mirror is
// DEFAULT_HIGHLIGHT_COLOR in web/src/highlights.ts.
const DefaultHighlightColor = "#ed1cb8"
