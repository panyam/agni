package graph

import (
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Choice is what a symbol source drew for a component and why. It is the per-component detail
// behind the conversion report: which cell, which device class, and whether the drawn symbol was
// the design's own (Provided) or a fallback because a provided one was requested but missing.
type Choice struct {
	Cell     string // the drawn symbol's cell_ref
	Class    string // device class id when a classified glyph was chosen (empty for the box or a provided symbol)
	Provided bool   // a provided (the design's own) symbol was drawn
	Fallback bool   // a provided symbol was requested but unavailable for this ref, so it fell back
}

// Explainer is an optional SymbolSource capability: besides drawing a symbol, it explains the
// choice (class, provided/fallback) so the conversion report can attribute each node. Registry
// and FaithfulSource implement it; a source that does not is reported from its drawn symbol
// alone (provided-vs-synthetic inferred from the cell_ref).
type Explainer interface {
	Explain(ref string, c *ir.Component, parts map[string]*ir.PartType) Choice
}

// Explain implements Explainer for the Registry: the component's class and its glyph cell.
func (r *Registry) Explain(_ string, c *ir.Component, parts map[string]*ir.PartType) Choice {
	class := ClassOther
	if c != nil {
		class = r.Classify(c, parts)
	}
	return Choice{Cell: r.cellFor(class), Class: class}
}

// Explain implements Explainer for the FaithfulSource: a provided symbol when the sidecar covers
// the ref, else the base source's choice marked as a fallback (the "unresolved" signal).
func (f *FaithfulSource) Explain(ref string, c *ir.Component, parts map[string]*ir.PartType) Choice {
	if s := f.byRef[ref]; s != nil && !s.GetAsset().GetPlaceholder() {
		return Choice{Cell: s.CellRef, Provided: true}
	}
	// No provided symbol for this ref, or only a placeholder box (the .sym did not resolve): the
	// drawn node falls back to the base source, and the report flags it to pass --symbol-path.
	ch := explain(f.base, ref, c, parts)
	ch.Fallback = true
	return ch
}

// explain asks a source to explain a choice, using its Explainer when it has one, else inferring
// provided-vs-synthetic from the drawn symbol's cell_ref (synthetic glyph/box cells are __node*).
func explain(source SymbolSource, ref string, c *ir.Component, parts map[string]*ir.PartType) Choice {
	if ex, ok := source.(Explainer); ok {
		return ex.Explain(ref, c, parts)
	}
	sym := source.Symbol(ref, c, parts)
	return Choice{Cell: sym.GetCellRef(), Provided: !strings.HasPrefix(sym.GetCellRef(), nodeCellPrefix)}
}

// nodeCellPrefix is the shared prefix of every synthetic node cell (the box nodeCell and the
// per-class glyph cells), so a cell that lacks it is a provided (real) symbol.
const nodeCellPrefix = "__node"

// Report kinds: what a component's node was drawn as.
const (
	KindProvided   = "provided"   // the design's own symbol
	KindGlyph      = "glyph"      // a classified synthetic glyph
	KindBox        = "box"        // the generic box: no device glyph for this class
	KindUnresolved = "unresolved" // provided symbols requested, but none for this ref (fell back)
)

// ComponentReport is one component's entry in the conversion report.
type ComponentReport struct {
	RefDes string `json:"ref_des"`
	Symbol string `json:"symbol,omitempty"` // source part/symbol name (PartType.Name)
	Class  string `json:"class,omitempty"`  // assigned device class (empty for box/provided)
	Cell   string `json:"cell"`             // drawn symbol cell_ref
	Kind   string `json:"kind"`             // provided | glyph | box | unresolved
}

// Label is how the component groups in the report summary: its device class for a glyph, else its
// kind (provided / box / unresolved).
func (c ComponentReport) Label() string {
	if c.Kind == KindGlyph {
		return c.Class
	}
	return c.Kind
}

// ConversionReport is how an auto-layout mapped each component to a drawn node: its device class,
// the symbol it drew, and whether that was a glyph, the box (unmapped class), a provided symbol,
// or an unresolved fallback. It is the visibility behind "why is this a box" (WS7-029).
type ConversionReport struct {
	Components []ComponentReport `json:"components"`
}

// BuildReport classifies and resolves each component through the symbol source (the same
// decisions assemble makes when drawing) and records them, in ref-des order for stable output. A
// nil source reports against the default registry (synthetic glyphs).
func BuildReport(d *ir.Design, source SymbolSource) *ConversionReport {
	if source == nil {
		source = DefaultRegistry()
	}
	parts := partIndex(d)
	comps := make([]*ir.Component, len(d.GetComponents()))
	copy(comps, d.GetComponents())
	sort.Slice(comps, func(i, j int) bool { return comps[i].GetRefDes() < comps[j].GetRefDes() })

	rep := &ConversionReport{}
	for _, c := range comps {
		ch := explain(source, c.GetRefDes(), c, parts)
		rep.Components = append(rep.Components, ComponentReport{
			RefDes: c.GetRefDes(),
			Symbol: partName(c, parts),
			Class:  ch.Class,
			Cell:   ch.Cell,
			Kind:   kindOf(ch),
		})
	}
	return rep
}

// RefsByKind returns the ref-des of every component drawn as the given kind, in order.
func (r *ConversionReport) RefsByKind(kind string) []string {
	var out []string
	for _, c := range r.Components {
		if c.Kind == kind {
			out = append(out, c.RefDes)
		}
	}
	return out
}

// kindOf maps a Choice to a report kind: a provided symbol wins, then a fallback is unresolved
// (the actionable "no provided symbol" case), then the box (no device glyph), else a glyph.
func kindOf(ch Choice) string {
	switch {
	case ch.Provided:
		return KindProvided
	case ch.Fallback:
		return KindUnresolved
	case ch.Class == ClassOther:
		return KindBox
	default:
		return KindGlyph
	}
}

// partName is the component's source symbol/part name (the first section that resolves to a
// PartType), or "" when none resolves.
func partName(c *ir.Component, parts map[string]*ir.PartType) string {
	if pt := resolvePart(c, parts); pt != nil {
		return pt.GetName()
	}
	return ""
}
