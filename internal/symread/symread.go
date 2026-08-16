// Package symread is the shared rim of the symbol-file schematic readers (xschem, gEDA
// gschem, and any future dialect such as Lepton EDA): the netlist-tier logic that was
// byte-identical or constant-parameterized between them. What stays in each reader is the
// format itself: parsing, the placement transform semantics (rotation/flip encodings), and
// the coordinate grid; those come in as closures and a Dialect. See the cross-format notes
// in the private research corpus for why the tokenizers deliberately stay forked.
package symread

import (
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
	"github.com/panyam/agni/internal/refdes"
)

// Pin is a symbol-local terminal as read from a .sym file: the connection point plus the
// designator/name/direction the IR carries. Dir is the pin's electrical direction mapped
// to the neutral vocabulary (gEDA pintype / xschem dir); UNSPECIFIED when the format's pin
// carries no type or one with no clean mapping, so direction-based rules stay conservative.
//
// Seq is the pin's 1-based sequence position within the symbol (gEDA pinseq), used only to
// select a slot's physical pin number via Placement.SlotPins; it is 0 for formats/symbols
// that carry no sequence (xschem, non-slotted gEDA), where the drawn Number stands.
type Pin struct {
	X, Y   float64
	Number string
	Name   string
	Dir    ir.PinDirection
	Seq    int
}

// Placement is one symbol instance to resolve into absolute pins. Place maps a
// symbol-local point onto the schematic plane; it captures the instance's origin and the
// format's own rotation/flip semantics, which is exactly the part that differs per dialect.
//
// SlotPins remaps the symbol's drawn pin numbers onto this instance's physical package pins
// for a multi-gate package (gEDA slot=/slotdef=): the pin with Seq i takes SlotPins[i-1] as
// its number, so slot 2 of a hex inverter resolves to physical pins 3,4 instead of the
// drawn 1,2. It is nil for single-gate placements, where the drawn Number stands. A symbol's
// slotdef is the same for every instance, but the selected row differs per instance, so the
// remap rides here and not on the memoized symbol load.
type Placement struct {
	Symref   string
	Ref      string
	Part     *ir.PartType // the library entry to enrich with resolved pins
	Place    func(px, py float64) (float64, float64)
	SlotPins []string
}

// ResolvePins turns each placement into its absolute-grid pins: load reads and parses the
// referenced .sym (memoized here, so a symbol used N times is opened once), Place maps each
// pin onto the schematic, and quant maps schematic coordinates onto the dialect's netgraph
// grid. Resolved pins are also recorded on the PartType, so the library reflects the
// symbol's terminals.
//
// load reports whether the symbol RESOLVED (its file opened and parsed) alongside its
// pins: a symbol that resolves with zero pins is fine (a graphic-only part), but one that
// fails to resolve drops every pin it should have contributed, which turns each wire end
// meant to land on those pins into a phantom dangling endpoint (WS1-013). unresolved lists
// each reference that did not resolve with the placements it cost pins, so the caller can
// gate dangling emission (a design with any unresolved placement cannot trust its dangle
// set) AND report the gap (WS1-052) rather than only going quieter because of it.
func ResolvePins(pls []Placement, load func(symref string) ([]Pin, bool), quant func(x, y float64) netgraph.Point) (out []netgraph.Pin, unresolved []Unresolved) {
	type entry struct {
		pins []Pin
		ok   bool
	}
	cache := map[string]entry{}
	resolve := func(symref string) entry {
		if e, ok := cache[symref]; ok {
			return e
		}
		pins, ok := load(symref)
		e := entry{pins: pins, ok: ok}
		cache[symref] = e
		return e
	}
	// Order of first appearance, so the report is stable across runs without a sort that would
	// scramble the source's own ordering.
	var missing []string
	byRef := map[string][]string{}

	for _, pl := range pls {
		e := resolve(pl.Symref)
		if !e.ok {
			if _, seen := byRef[pl.Symref]; !seen {
				missing = append(missing, pl.Symref)
			}
			byRef[pl.Symref] = append(byRef[pl.Symref], pl.Ref)
		}
		if len(pl.Part.Pins) == 0 {
			for _, sp := range e.pins {
				pl.Part.Pins = append(pl.Part.Pins, &ir.Pin{Designator: pl.pinNumber(sp), Name: sp.Name, Direction: sp.Dir})
			}
		}
		for _, sp := range e.pins {
			ax, ay := pl.Place(sp.X, sp.Y)
			out = append(out, netgraph.Pin{At: quant(ax, ay), Comp: pl.Ref, Pin: pl.pinNumber(sp)})
		}
	}
	for _, symref := range missing {
		unresolved = append(unresolved, Unresolved{Symref: symref, RefDes: byRef[symref]})
	}
	return out, unresolved
}

// Unresolved is one symbol reference that failed to load, with every placement that lost its pins.
// Grouped per reference rather than per placement because one missing file is one cause, however
// many parts are drawn with it. The reader turns this into ir.UnresolvedSymbol, stamping the kind
// and provenance it alone knows.
type Unresolved struct {
	Symref string
	RefDes []string
}

// pinNumber returns the physical pin number for a drawn symbol pin under this placement:
// the slot's remapped number when SlotPins applies (a multi-gate package), else the pin's
// own drawn number. A slot table indexes by the pin's 1-based sequence; a pin whose Seq
// falls outside the table keeps its drawn number, so a partially-annotated symbol degrades
// to the faithful reading rather than dropping the pin.
func (pl Placement) pinNumber(sp Pin) string {
	if len(pl.SlotPins) > 0 && sp.Seq >= 1 && sp.Seq <= len(pl.SlotPins) {
		return pl.SlotPins[sp.Seq-1]
	}
	return sp.Number
}

// NameOnlyNets returns the distinct net names from wire labels and anchors, in first-seen
// order, for the no-symbol path (components + net names, no pin-level connectivity).
func NameOnlyNets(wires []netgraph.Wire, anchors []netgraph.Anchor) []string {
	seen := map[string]bool{}
	var order []string
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		order = append(order, s)
	}
	for _, w := range wires {
		add(w.Label)
	}
	for _, a := range anchors {
		add(a.Label)
	}
	return order
}

// SymbolBase strips the directory and .sym suffix from a symbol reference
// ("devices/res.sym" -> "res").
func SymbolBase(ref string) string {
	ref = strings.TrimSpace(ref)
	if i := strings.LastIndexAny(ref, "/\\"); i >= 0 {
		ref = ref[i+1:]
	}
	return strings.TrimSuffix(ref, ".sym")
}

// Dialect carries the constants that distinguish one symbol-file format's IR emission from
// another's: the library name components file under, the provenance id kind, and which
// instance attributes copy onto a component section ("value" also promotes to the
// component-level Value).
type Dialect struct {
	Lib          string
	NativeIDKind string
	SectionAttrs []string
}

// NewComponent builds a single-section Component from a placement's instance attributes.
// A multi-gate package (gEDA slotting) folds several instances into one Component by adding
// further sections with AddSection.
func (d Dialect) NewComponent(ref, sym string, attrs map[string]string, src string) *ir.Component {
	comp := &ir.Component{
		RefDes:     ref,
		Attributes: map[string]string{},
		Prov:       &ir.Provenance{SourceFile: src, NativeId: ref, NativeIdKind: d.NativeIDKind},
	}
	d.AddSection(comp, sym, attrs, src)
	return comp
}

// AddSection appends one more instance section to an existing Component, for a multi-gate
// package whose gates share a ref-des (gEDA slot=): each C line is a section of the same
// physical part, not a separate component. The section index follows the current count, and
// the promoted component-level Value is filled from the first section that carries one.
func (d Dialect) AddSection(comp *ir.Component, sym string, attrs map[string]string, src string) {
	sec := &ir.ComponentSection{
		Index:      int32(len(comp.Sections)),
		PartRef:    sym,
		LibraryRef: d.Lib,
		Attributes: map[string]string{},
		Prov:       &ir.Provenance{SourceFile: src, NativeId: sectionNativeID(comp, attrs), NativeIdKind: d.NativeIDKind},
	}
	for _, k := range d.SectionAttrs {
		if v := attrs[k]; v != "" {
			sec.Attributes[k] = v
			if k == "value" && comp.Attributes["Value"] == "" {
				comp.Attributes["Value"] = v
			}
		}
	}
	comp.Sections = append(comp.Sections, sec)
}

// sectionNativeID names a section's provenance native id: the component ref-des, suffixed with
// the gEDA slot when present so a multi-gate package's sections stay distinguishable.
func sectionNativeID(comp *ir.Component, attrs map[string]string) string {
	if slot := attrs["slot"]; slot != "" {
		return comp.RefDes + ":" + slot
	}
	return comp.RefDes
}

// RefDesCollisions reports designators claimed by more than one distinct physical placement, for
// the readers that build components through this package (gEDA and xschem).
//
// Both can answer the question their format's own rules already settle, which is what makes the
// answer trustworthy rather than a guess. gEDA STATES THE GATE: a package's gates share a refdes and
// carry distinct slot=, folded by the caller into one Component with a section per gate. xschem
// DECLARES NAMES UNIQUE within a schematic, and this module relies on that (an instance name is the
// provenance native id), so a repeat is a duplicate and a break of the reader's own assumption at
// once. Neither is EDIF, which represents a multi-gate part as instances sharing a designator with
// no unit to distinguish them, and therefore supplies nothing (agni issue 309).
//
// A designator is duplicated when the same gate is claimed twice, in either of the two shapes that
// produces:
//
//   - two placements with the SAME slot=, folded into one Component whose sections then share a
//     native id (refdes:slot);
//   - two placements that were never folded, which stay separate Components wearing one refdes.
//     That is every xschem repeat, and gEDA's unslotted one.
//
// A placeholder designator is not a claimed name (two unnamed resistors are not fighting over "R?"),
// so it is skipped here exactly as the KiCad reader skips it; unannotated-components reports those.
//
// Callers declare "ref_des_collisions" in InputDiagnostics.supplied alongside the result, INCLUDING
// when it is empty: that declaration is what separates "no duplicates" from "never looked".
func RefDesCollisions(comps []*ir.Component) []*ir.RefDesCollision {
	order := []string{}
	byRef := map[string][]*ir.Component{}
	for _, c := range comps {
		ref := c.GetRefDes()
		if ref == "" || refdes.IsPlaceholder(ref) {
			continue
		}
		if _, seen := byRef[ref]; !seen {
			order = append(order, ref)
		}
		byRef[ref] = append(byRef[ref], c)
	}

	var out []*ir.RefDesCollision
	for _, ref := range order {
		group := byRef[ref]
		var instances []*ir.Provenance
		if len(group) > 1 {
			// Separate Components under one refdes: unslotted placements, each its own part.
			for _, c := range group {
				if len(c.Sections) > 0 {
					instances = append(instances, c.Sections[0].Prov)
				}
			}
		} else {
			// One Component: a gate claimed twice shows up as two sections with one native id.
			count := map[string]int{}
			for _, s := range group[0].Sections {
				count[s.GetProv().GetNativeId()]++
			}
			for _, s := range group[0].Sections {
				if count[s.GetProv().GetNativeId()] > 1 {
					instances = append(instances, s.Prov)
				}
			}
		}
		if len(instances) > 1 {
			out = append(out, &ir.RefDesCollision{RefDes: ref, Instances: instances})
		}
	}
	return out
}
