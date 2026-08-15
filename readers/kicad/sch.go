package kicad

import (
	"fmt"
	"io"
	"sort"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/refdes"
)

// ReadSchematic parses a KiCad .kicad_sch schematic into an ir.Design.
//
// Fidelity: lossy-bounded (structural subset). We extract the part-type library
// (lib_symbols), the placed components -- grouped by reference designator, with each
// multi-unit placement becoming a ComponentSection -- the hierarchical sub-sheet references, and
// the nets. KiCad stores schematic connectivity implicitly (wires + pins + labels are geometry),
// so nets are computed from that geometry by schNets (see sch_nets.go), not read from the file.
// sourceFile is recorded in provenance only; the caller owns file I/O (CONSTRAINTS C1).
func ReadSchematic(r io.Reader, sourceFile string) (*ir.Design, error) {
	return ReadSchematicWithSymbols(r, sourceFile, nil)
}

// ReadSchematicWithSymbols is ReadSchematic plus external symbol-library resolution
// (WS1-016): openSym fetches a library's .kicad_sym bytes by nickname for lib_id
// references the schematic does not embed. Embedded lib_symbols always win; a library
// that resolves nowhere degrades to today's behavior (placeholder artwork, no typed
// pins). nil openSym resolves nothing. The caller owns file I/O (C1); formats
// builds the opener from the project's sym-lib-table and the --symbol-path dirs.
func ReadSchematicWithSymbols(r io.Reader, sourceFile string, openSym func(lib string) ([]byte, error)) (*ir.Design, error) {
	root, err := parse(r)
	if err != nil {
		return nil, err
	}
	if root.Head() != "kicad_sch" {
		return nil, fmt.Errorf("kicad: not a .kicad_sch file (root is %q)", root.Head())
	}
	return extractSch(root, sourceFile, newSymLibCache(openSym)), nil
}

// extractSch walks the schematic tree: the part-type library, the placed symbols (grouped
// into components + sections), and the sub-sheet references.
func extractSch(root *node, src string, syms *symLibCache) *ir.Design {
	d := &ir.Design{
		IrVersion:    "0",
		SourceFormat: "kicad-sch",
		Attributes:   map[string]string{},
		Prov:         &ir.Provenance{SourceFile: src},
	}
	if tb := root.Child("title_block"); tb != nil {
		d.Name = atomOf(tb.Child("title").Arg(1))
	}

	libs := newLibAccum()
	libs.collect(root, src)

	comps := newCompAccum()
	comps.collect(root, src, "")
	var collisions []*ir.RefDesCollision
	d.Components, collisions = comps.components()
	unresolvedSyms := libs.resolveExternal(d.Components, syms, src)
	d.Libraries = libs.libraries()

	// Nets are computed from the schematic geometry (wires + pins + labels); see sch_nets.go.
	// The same geometry pass surfaces dangling wire endpoints (connections drawn but not completed).
	nets, dangles, noJunction := schNets(root, src, syms)
	d.Nets = nets
	d.InputDiagnostics = &ir.InputDiagnostics{DanglingEndpoints: dangles, RefDesCollisions: collisions, NoJunctionEndpoints: noJunction, UnmodeledBuses: collectBuses(root, src, nil), UnresolvedSymbols: unresolvedSyms}

	// Hierarchical sub-sheet references. Recorded so the hierarchy is visible; the sub-sheet
	// files themselves are not recursively loaded here (needs multi-file I/O).
	for _, sh := range root.Children("sheet") {
		d.Sheets = append(d.Sheets, &ir.Sheet{
			Id:   uuidOf(sh),
			Name: propValue(sh, "Sheetname"),
			Prov: &ir.Provenance{SourceFile: src, NativeId: uuidOf(sh), NativeIdKind: kicadNativeIDKind},
		})
	}
	return d
}

// libAccum accumulates part-type libraries across one or more sheet files, deduping parts
// by their lib_id (a hierarchy's files each embed their own lib_symbols copy of shared
// parts; the first definition wins).
type libAccum struct {
	byName   map[string]*ir.PartLibrary
	order    []string
	partSeen map[string]bool
}

func newLibAccum() *libAccum {
	return &libAccum{byName: map[string]*ir.PartLibrary{}, partSeen: map[string]bool{}}
}

// collect adds root's lib_symbols entries, grouped into libraries by lib_id prefix
// (e.g. "Device:R" -> library "Device").
func (a *libAccum) collect(root *node, src string) {
	for _, sym := range root.Child("lib_symbols").Children("symbol") {
		id := atomOf(sym.Arg(1))
		if a.partSeen[id] {
			continue
		}
		a.partSeen[id] = true
		pt := &ir.PartType{
			Name:             id,
			DesignatorPrefix: propValue(sym, "Reference"),
			Pins:             partPins(sym),
			Prov:             &ir.Provenance{SourceFile: src},
		}
		prefix := libPrefix(id)
		lib := a.byName[prefix]
		if lib == nil {
			lib = &ir.PartLibrary{Name: prefix, Prov: &ir.Provenance{SourceFile: src}}
			a.byName[prefix] = lib
			a.order = append(a.order, prefix)
		}
		lib.Parts = append(lib.Parts, pt)
	}
}

// resolveExternal adds part types for component sections whose PartRef is missing from
// the embedded libraries, resolved through the external symbol cache (WS1-016). Embedded
// parts always win.
//
// A ref that resolves nowhere is REPORTED (WS1-052) rather than left absent: the part keeps its
// section but gains no pins, and a component with no pins has no connections, so the design reads
// clean and emptier than it is. Returned per lib_id with the placements that lost pins, in
// first-appearance order. A nil cache (no opener supplied) reports nothing — that is the caller
// deliberately reading without symbols, not a resolution failure.
func (a *libAccum) resolveExternal(comps []*ir.Component, syms *symLibCache, src string) []*ir.UnresolvedSymbol {
	if syms == nil {
		return nil
	}
	var missing []string
	byRef := map[string][]string{}
	for _, c := range comps {
		for _, sec := range c.Sections {
			if a.partSeen[sec.PartRef] {
				continue
			}
			sym := syms.symbol(sec.PartRef)
			if sym == nil {
				if _, seen := byRef[sec.PartRef]; !seen {
					missing = append(missing, sec.PartRef)
				}
				byRef[sec.PartRef] = append(byRef[sec.PartRef], c.RefDes)
				continue
			}
			a.partSeen[sec.PartRef] = true
			pt := &ir.PartType{
				Name:             sec.PartRef,
				DesignatorPrefix: propValue(sym, "Reference"),
				Pins:             partPins(sym),
				Prov:             &ir.Provenance{SourceFile: src},
			}
			prefix := libPrefix(sec.PartRef)
			lib := a.byName[prefix]
			if lib == nil {
				lib = &ir.PartLibrary{Name: prefix, Prov: &ir.Provenance{SourceFile: src}}
				a.byName[prefix] = lib
				a.order = append(a.order, prefix)
			}
			lib.Parts = append(lib.Parts, pt)
		}
	}
	var out []*ir.UnresolvedSymbol
	for _, libID := range missing {
		out = append(out, &ir.UnresolvedSymbol{
			Symref: libID,
			Kind:   "kicad_sym_lib",
			RefDes: byRef[libID],
			Prov:   &ir.Provenance{SourceFile: src},
		})
	}
	return out
}

func (a *libAccum) libraries() []*ir.PartLibrary {
	var libs []*ir.PartLibrary
	for _, name := range a.order {
		libs = append(libs, a.byName[name])
	}
	return libs
}

// compAccum accumulates placed components across one or more sheet instances, grouped by
// resolved reference designator; each placement is a section (one per KiCad unit).
// Virtual power/flag symbols (#PWR, #FLG) are connectivity anchors, not physical
// components, and are skipped.
type compAccum struct {
	byRef    map[string]*ir.Component
	refOrder []string
}

func newCompAccum() *compAccum { return &compAccum{byRef: map[string]*ir.Component{}} }

// collect adds root's placed symbols, resolving each reference for the given instance
// path (symbolRefAt; empty for a single-file read).
func (a *compAccum) collect(root *node, src, instPath string) {
	for _, ps := range root.Children("symbol") {
		ref := symbolRefAt(ps, instPath)
		if ref == "" || ref[0] == '#' {
			continue
		}
		comp := a.byRef[ref]
		if comp == nil {
			comp = &ir.Component{RefDes: ref, Attributes: map[string]string{}, Prov: &ir.Provenance{SourceFile: src}}
			a.byRef[ref] = comp
			a.refOrder = append(a.refOrder, ref)
		}
		sec := &ir.ComponentSection{
			Index:      int32(unitOf(ps) - 1),
			PartRef:    atomOf(ps.Child("lib_id").Arg(1)),
			LibraryRef: libPrefix(atomOf(ps.Child("lib_id").Arg(1))),
			Attributes: map[string]string{},
			Prov:       &ir.Provenance{SourceFile: src, NativeId: uuidOf(ps), NativeIdKind: kicadNativeIDKind},
		}
		// Value plus the part-identity properties (MPN, Manufacturer — the WS10-003
		// join key when no BomLine exists) and Footprint/Datasheet (footprint-consistency
		// rules + the WS10 datasheet join, WS1-037) are carried into attributes; other user
		// properties are deliberately not swept until a consumer earns them (C9).
		for _, key := range []string{"Value", "MPN", "Manufacturer", "Footprint", "Datasheet"} {
			if v := propValue(ps, key); v != "" {
				sec.Attributes[key] = v
				if _, ok := comp.Attributes[key]; !ok {
					comp.Attributes[key] = v
				}
			}
		}
		// dnp/in_bom/on_board are symbol-instance TOKENS ((dnp yes)), not properties: the
		// populated-vs-not fabrication flags a check or a diff needs (WS1-037). A DNP part
		// counts as unpopulated; in_bom/on_board scope it out of BOM/assembly. Faithful to
		// the source spelling ("yes"/"no"); the concept generalizes across formats so it
		// rides the open attributes map, not a new semantic field (C9).
		for _, tok := range []string{"dnp", "in_bom", "on_board"} {
			if v := atomOf(ps.Child(tok).Arg(1)); v != "" {
				sec.Attributes[tok] = v
				if _, ok := comp.Attributes[tok]; !ok {
					comp.Attributes[tok] = v
				}
			}
		}
		comp.Sections = append(comp.Sections, sec)
	}
}

// components returns the accumulated components (sections sorted by unit) and the ref-des
// collision diagnostics (see refDesCollision).
func (a *compAccum) components() ([]*ir.Component, []*ir.RefDesCollision) {
	var comps []*ir.Component
	var collisions []*ir.RefDesCollision
	for _, ref := range a.refOrder {
		comp := a.byRef[ref]
		sort.SliceStable(comp.Sections, func(i, j int) bool { return comp.Sections[i].Index < comp.Sections[j].Index })
		comps = append(comps, comp)
		if c := refDesCollision(comp); c != nil {
			collisions = append(collisions, c)
		}
	}
	return comps, collisions
}

// refDesCollision reports a genuine duplicate ref-des: two schematic symbols claiming the same unit
// of one designator. A legitimate multi-unit part spreads across distinct unit indices, so it does
// not trip; only a repeated unit does. The colliding placements' provenance (uuids) is returned so
// a finding can point at each. Sections are already sorted by Index, so the instance order is
// deterministic. Returns nil when the component is clean.
// symbolRef resolves a placed symbol's reference designator for a single-file read; see
// symbolRefAt for the resolution rules.
func symbolRef(ps *node) string { return symbolRefAt(ps, "") }

// symbolRefAt resolves a placed symbol's reference designator for one sheet INSTANCE. The
// instances block's per-project reference is post-annotation truth (KiCad writes it on
// save and honors it on load), so it beats the Reference property — the symbol's
// authoring-time value, which files edited across projects may leave stale or as an
// unannotated placeholder (WS1-020). instPath is the instance's KiCad path (the
// "/<root uuid>/<sheet uuid>..." chain the hierarchy walk tracks): a reused sheet file
// carries one instances entry PER placement, and the matching path's reference is this
// instance's identity (RV201 in one amplifier, RV301 in the other). An empty instPath
// (single-file read) or an unmatched path falls back to the first non-placeholder entry,
// then to the Reference property.
func symbolRefAt(ps *node, instPath string) string {
	if inst := ps.Child("instances"); inst != nil {
		first := ""
		for _, proj := range inst.Children("project") {
			for _, path := range proj.Children("path") {
				rf := path.Child("reference")
				if rf == nil {
					continue
				}
				r := atomOf(rf.Arg(1))
				if r == "" || refdes.IsPlaceholder(r) {
					continue
				}
				if instPath != "" && atomOf(path.Arg(1)) == instPath {
					return r
				}
				if first == "" {
					first = r
				}
			}
		}
		if first != "" {
			return first
		}
	}
	return propValue(ps, "Reference")
}

func refDesCollision(c *ir.Component) *ir.RefDesCollision {
	count := map[int32]int{}
	for _, s := range c.Sections {
		count[s.Index]++
	}
	var instances []*ir.Provenance
	for _, s := range c.Sections {
		if count[s.Index] > 1 {
			instances = append(instances, s.Prov)
		}
	}
	if len(instances) == 0 {
		return nil
	}
	return &ir.RefDesCollision{RefDes: c.RefDes, Instances: instances}
}

// partPins pools the pins across a lib symbol's unit/body-style sub-symbols, deduped by
// pin number, mapping each pin's electrical type onto ir.PinDirection.
func partPins(sym *node) []*ir.Pin {
	seen := map[string]bool{}
	var pins []*ir.Pin
	for _, sub := range sym.Children("symbol") {
		for _, pn := range sub.Children("pin") {
			num := atomOf(pn.Child("number").Arg(1))
			if num == "" || seen[num] {
				continue
			}
			seen[num] = true
			p := &ir.Pin{
				Designator: num,
				Name:       atomOf(pn.Child("name").Arg(1)),
				Attributes: map[string]string{},
			}
			raw := atomOf(pn.Arg(1))
			p.Direction = mapSchDirection(raw)
			if p.Direction == ir.PinDirection_PIN_DIRECTION_UNSPECIFIED && raw != "" {
				p.Attributes["direction_raw"] = raw
			}
			pins = append(pins, p)
		}
	}
	return pins
}

// mapSchDirection normalizes a KiCad pin electrical type onto ir.PinDirection. Types with
// no clean mapping (tri_state, open_collector, ...) fall through to UNSPECIFIED and keep
// their raw spelling in attributes["direction_raw"].
func mapSchDirection(raw string) ir.PinDirection {
	switch raw {
	case "input":
		return ir.PinDirection_PIN_DIRECTION_INPUT
	case "output":
		return ir.PinDirection_PIN_DIRECTION_OUTPUT
	case "bidirectional":
		return ir.PinDirection_PIN_DIRECTION_INOUT
	case "passive", "free":
		return ir.PinDirection_PIN_DIRECTION_PASSIVE
	case "power_in":
		return ir.PinDirection_PIN_DIRECTION_POWER_IN
	case "power_out":
		return ir.PinDirection_PIN_DIRECTION_POWER_OUT
	case "no_connect":
		return ir.PinDirection_PIN_DIRECTION_NO_CONNECT
	default:
		return ir.PinDirection_PIN_DIRECTION_UNSPECIFIED
	}
}
