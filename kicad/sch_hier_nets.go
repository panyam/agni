package kicad

import (
	"bytes"
	"fmt"
	"path"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// instStep is the grid offset separating sheet instances in the design-wide net solve:
// each instance's geometry is translated onto its own X band so wires only join within
// their sheet, while labels union across bands. ~2.2e12 nm (2.2 km) per band dwarfs any
// sheet (an A0 page is ~1.2e9 nm) and thousands of instances stay far from int64 range.
const instStep = int64(1) << 41

// ReadSchematicHierarchyNets reads a schematic and its sub-sheet tree into ONE netlist
// Design (WS1-018): components and nets from every sheet instance, rails unified by
// global labels and power symbols, hierarchical labels joined to their parent sheet pins,
// and per-instance reference designators for reused sheet files. It is the netlist twin
// of ReadSchematicHierarchy (geometry): the same traversal, opener contract, cycle guard,
// and hierarchical sheet ids ("/", "/<Sheetname>", ...), so netlist and geometry agree on
// sheet identity.
//
// open fetches a child by its (relative) Sheetfile path; the caller resolves it against
// the root's location, so this package does no file I/O (CONSTRAINTS C1). Unlike the
// geometry walk, netlist correctness is judged by the returned complete flag: true only
// when every referenced sub-sheet was opened and walked. A missing child skips that
// subtree (the rest still reads) but leaves the design partial, so cross-sheet (external)
// net markings must then stay conservative — WS1-017's external->global downgrade is the
// caller's decision, gated on complete (see ReadProject). A nil open reads the root sheet
// alone (complete only if it references no sub-sheets).
//
// Net names follow KiCad's own convention so schematic and board reads agree: global
// labels and power rails keep bare names everywhere, root-sheet local labels are bare,
// and sub-sheet local/hierarchical names are qualified by the instance's sheet path
// ("/ampli_ht_vertical/PIEZO_IN"). Naming priority (bare beats qualified, shallower wins
// ties) was pinned against kicad-cli sch export netlist.
func ReadSchematicHierarchyNets(rootName string, rootContent []byte, open func(relPath string) ([]byte, error)) (*ir.Design, bool, error) {
	return ReadSchematicHierarchyNetsWithSymbols(rootName, rootContent, open, nil)
}

// ReadSchematicHierarchyNetsWithSymbols is the walk plus external symbol-library
// resolution (WS1-016): openSym fetches .kicad_sym bytes by library nickname for lib_id
// references no sheet embeds. One cache serves the whole walk. nil openSym resolves
// nothing.
func ReadSchematicHierarchyNetsWithSymbols(rootName string, rootContent []byte, open, openSym func(string) ([]byte, error)) (*ir.Design, bool, error) {
	d := &ir.Design{
		IrVersion:    "0",
		SourceFormat: "kicad-sch",
		Attributes:   map[string]string{},
		Prov:         &ir.Provenance{SourceFile: rootName},
	}
	if open == nil {
		open = func(string) ([]byte, error) { return nil, fmt.Errorf("no sub-sheet opener") }
	}
	w := &hierNetWalker{
		d:        d,
		libs:     newLibAccum(),
		comps:    newCompAccum(),
		syms:     newSymLibCache(openSym),
		complete: true,
		open:     open,
	}
	if err := w.walk(rootContent, rootName, "/", "", map[string]bool{}); err != nil {
		return nil, false, err
	}

	var collisions []*ir.RefDesCollision
	d.Components, collisions = w.comps.components()
	w.libs.resolveExternal(d.Components, w.syms, rootName)
	d.Libraries = w.libs.libraries()

	built, dangles, _, pointNets := netgraph.BuildWithPoints(w.in.wires, w.in.anchors, w.in.pins, w.in.terminals)
	kept := built[:0]
	for _, n := range built {
		if len(n.Conns) > 0 {
			kept = append(kept, n) // KiCad itself omits a named-but-pinless net
		}
	}
	d.Nets = netgraph.IRNets(kept, rootName)
	stampNetSheets(d.Nets, pointNets, d.Sheets)
	d.InputDiagnostics = &ir.InputDiagnostics{
		DanglingEndpoints:   hierDangles(dangles, w.srcs),
		RefDesCollisions:    collisions,
		NoJunctionEndpoints: w.in.noJunction,
		UnmodeledBuses:      w.buses,
	}
	return d, w.complete, nil
}

// hierNetWalker carries the accumulator state for the hierarchical netlist walk — the fields a
// recursive closure would otherwise capture: the Design under construction, the library and
// component accumulators, the external symbol-library cache, the collected net inputs, the
// per-instance source list, the completeness flag, and the sub-sheet opener. One walk call runs
// per sheet instance.
type hierNetWalker struct {
	d        *ir.Design
	libs     *libAccum
	comps    *compAccum
	syms     *symLibCache
	in       netInputs
	srcs     []string              // per-instance source file, indexed by the instance's offset band
	buses    []*ir.BusNotModeled   // bus constructs detected across sheets (WS1-034)
	complete bool
	open     func(relPath string) ([]byte, error)
}

// walk reads one sheet instance (content) at hierarchical id, appends its libraries,
// components, nets, and sheet record, then recurses into referenced sub-sheets. ancestors is
// the source-file set on the path from the root, for cycle breaking; instPath is the KiCad
// instance path used to resolve per-instance reference designators.
func (w *hierNetWalker) walk(content []byte, src, id, instPath string, ancestors map[string]bool) error {
	if len(ancestors) > 64 {
		w.complete = false // depth backstop, in addition to the ancestor-cycle guard
		return nil
	}
	root, err := parse(bytes.NewReader(content))
	if err != nil {
		return err
	}
	if root.Head() != "kicad_sch" {
		return fmt.Errorf("kicad: %q is not a .kicad_sch file (root is %q)", src, root.Head())
	}

	name := ""
	if id == "/" {
		if tb := root.Child("title_block"); tb != nil {
			w.d.Name = atomOf(tb.Child("title").Arg(1))
		}
		name = w.d.Name
		if u := uuidOf(root); u != "" {
			instPath = "/" + u
		}
	}

	k := int64(len(w.srcs))
	w.srcs = append(w.srcs, src)
	sc := sheetScope{offset: netgraph.Point{X: k * instStep}, instPath: instPath, src: src, syms: w.syms}
	if id != "/" {
		sc.prefix = id
		name = path.Base(id)
	}
	w.libs.collect(root, src)
	w.comps.collect(root, src, instPath)
	collectSheetNets(root, sc, &w.in)
	w.buses = append(w.buses, collectBuses(root, src, sc.local)...)
	w.d.Sheets = append(w.d.Sheets, &ir.Sheet{
		Id:   id,
		Name: name,
		Prov: &ir.Provenance{SourceFile: src},
	})

	for _, sub := range root.Children("sheet") {
		file := propValue(sub, "Sheetfile")
		childID := path.Join(id, propValue(sub, "Sheetname"))
		// The parent half of each hierarchical port: an anchor at the sheet pin's
		// position carrying the CHILD-qualified name, the same label the child's
		// hierarchical_label emits — label-union joins the two sheets. Emitted
		// whether or not the child opens, so the parent net still gets the port's
		// KiCad-style name on a partial read.
		for _, p := range sub.Children("pin") {
			if at := sheetPt(p.Child("at")); at != nil {
				w.in.anchors = append(w.in.anchors, netgraph.Anchor{At: sc.at(gp(at)), Label: childID + "/" + unescapeName(atomOf(p.Arg(1))), Rank: rankLocal})
			}
		}
		if file == "" || ancestors[file] {
			w.complete = false // unreferenced or a cycle back to an ancestor
			continue
		}
		childBytes, err := w.open(file)
		if err != nil {
			w.complete = false // a missing/unreadable sub-sheet is skipped, not fatal
			continue
		}
		childAnc := map[string]bool{src: true}
		for a := range ancestors {
			childAnc[a] = true
		}
		if err := w.walk(childBytes, file, childID, instPath+"/"+uuidOf(sub), childAnc); err != nil {
			return err
		}
	}
	return nil
}

// hierWireNets runs the COMBINED hierarchy net solve — all sheet instances into one
// netgraph.Build, exactly as ReadSchematicHierarchyNets does — and returns each wire's
// uuid -> solved net name (WS1-022). The geometry hierarchy reader uses this so its wire
// net names are byte-identical to the netlist read's net names (same N$ numbering, same
// per-instance qualification), which is what makes the finding-subject -> wire -> sheet
// join land. It mirrors the netlist walk's traversal but accumulates only the solver
// inputs (no libraries/components/sheets), since only the wire->net map is needed. open
// nil reads the root sheet alone.
func hierWireNets(rootName string, rootContent []byte, open, openSym func(string) ([]byte, error)) map[string]netgraph.NetRef {
	syms := newSymLibCache(openSym)
	var in netInputs
	var k int64

	var walk func(content []byte, src, id, instPath string, ancestors map[string]bool)
	walk = func(content []byte, src, id, instPath string, ancestors map[string]bool) {
		if len(ancestors) > 64 {
			return
		}
		root, err := parse(bytes.NewReader(content))
		if err != nil || root.Head() != "kicad_sch" {
			return
		}
		if id == "/" {
			if u := uuidOf(root); u != "" {
				instPath = "/" + u
			}
		}
		sc := sheetScope{offset: netgraph.Point{X: k * instStep}, instPath: instPath, src: src, syms: syms, wirePfx: id}
		if id != "/" {
			sc.prefix = id
		}
		k++
		collectSheetNets(root, sc, &in)
		for _, sub := range root.Children("sheet") {
			file := propValue(sub, "Sheetfile")
			childID := path.Join(id, propValue(sub, "Sheetname"))
			for _, p := range sub.Children("pin") {
				if at := sheetPt(p.Child("at")); at != nil {
					in.anchors = append(in.anchors, netgraph.Anchor{At: sc.at(gp(at)), Label: childID + "/" + unescapeName(atomOf(p.Arg(1))), Rank: rankLocal})
				}
			}
			if file == "" || ancestors[file] {
				continue
			}
			childBytes, err := open(file)
			if err != nil {
				continue
			}
			childAnc := map[string]bool{src: true}
			for a := range ancestors {
				childAnc[a] = true
			}
			walk(childBytes, file, childID, instPath+"/"+uuidOf(sub), childAnc)
		}
	}

	if open == nil {
		open = func(string) ([]byte, error) { return nil, fmt.Errorf("no sub-sheet opener") }
	}
	walk(rootContent, rootName, "/", "", map[string]bool{})
	_, _, wireNets := netgraph.Build(in.wires, in.anchors, in.pins, in.terminals)
	return wireNets
}

// stampNetSheets attributes each net the set of sheet instances it touches (WS9-028), so a
// finding whose subject is a net gets a sheet badge like a component subject does. Membership
// is authoritative: pointNets says which net every connection point resolved to, and a point's
// X band (the same instStep offset hierDangles decodes) names its sheet instance — so even a
// wireless single-pin net on a sub-sheet, which carries no wire geometry to join, is placed.
// Ids come out in sheet order; a net touching several sheets lists each. Stamped only on a
// genuine hierarchy (more than one sheet): a single-sheet read has nothing to disambiguate and
// the badge layer hides badges below two sheets, so no attribute is written.
func stampNetSheets(nets []*ir.Net, pointNets map[netgraph.Point]netgraph.NetRef, sheets []*ir.Sheet) {
	if len(sheets) <= 1 {
		return
	}
	// Key by the per-instance net id, NOT the name (WS9): two electrically-distinct nets that share
	// a name resolve to distinct ids, so each attributes to only the sheets IT touches instead of
	// both getting the union — which is what inflated a design-global finding onto every sheet. A
	// pinless net has no id, so it keys by name (its old, still-correct behavior).
	sheetKey := func(r netgraph.NetRef) string {
		if r.ID != "" {
			return r.ID
		}
		return r.Name
	}
	// net key -> band -> present, then flattened in band (sheet) order.
	bands := map[string]map[int64]bool{}
	for pt, ref := range pointNets {
		k := floorDiv(pt.X+instStep/2, instStep)
		if k < 0 || k >= int64(len(sheets)) {
			continue
		}
		key := sheetKey(ref)
		if bands[key] == nil {
			bands[key] = map[int64]bool{}
		}
		bands[key][k] = true
	}
	for _, n := range nets {
		key := n.GetId()
		if key == "" {
			key = n.GetName()
		}
		present := bands[key]
		if len(present) == 0 {
			continue
		}
		var ids []string
		for k := int64(0); k < int64(len(sheets)); k++ {
			if present[k] {
				ids = append(ids, sheets[k].GetId())
			}
		}
		if len(ids) == 0 {
			continue
		}
		if n.Attributes == nil {
			n.Attributes = map[string]string{}
		}
		n.Attributes[netgraph.AttrSheets] = netgraph.EncodeSheets(ids)
	}
}

// hierDangles translates dangling endpoints back out of their instance offset bands into
// sheet-frame coordinates (the viewer draws these on the sheet), attributing each to its
// instance's source file.
func hierDangles(dangles []netgraph.Dangle, srcs []string) []*ir.DanglingEndpoint {
	var out []*ir.DanglingEndpoint
	for _, dg := range dangles {
		k := floorDiv(dg.At.X+instStep/2, instStep)
		src := ""
		if k >= 0 && k < int64(len(srcs)) {
			src = srcs[k]
		}
		prov := &ir.Provenance{SourceFile: src}
		if dg.WireId != "" {
			prov.NativeId, prov.NativeIdKind = dg.WireId, kicadNativeIDKind
		}
		out = append(out, &ir.DanglingEndpoint{X: dg.At.X - k*instStep, Y: dg.At.Y, Prov: prov})
	}
	return out
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b < 0 {
		q--
	}
	return q
}
