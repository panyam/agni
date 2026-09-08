package kicad

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
	"github.com/panyam/agni/internal/refdes"
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
	if err := w.walk(rootContent, rootName, "/", "", map[string]bool{}, nil); err != nil {
		return nil, false, err
	}

	var collisions []*ir.RefDesCollision
	d.Components, collisions = w.comps.components()
	unresolvedSyms := w.libs.resolveExternal(d.Components, w.syms, rootName)
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
		DanglingEndpoints: hierDangles(dangles, w.srcs),
		RefDesCollisions:  collisions,
		// Declared even when the slice is empty: that is the point of `supplied`. This reader
		// looked, so an empty list means "no collisions" here, where on a reader that cannot look
		// it would mean "nobody asked" (agni issue 309).
		Supplied:              []string{"ref_des_collisions", "resolved_symbols", "junction_taps"},
		NoJunctionEndpoints:   w.in.noJunction,
		JoinedTaps:            w.in.joinedTaps,
		UnmodeledBuses:        w.buses,
		UnresolvedSymbols:     unresolvedSyms,
		ResolvedSymbols:       w.libs.resolvedSymbols(),
		UnannotatedComponents: refdes.Unannotated(d.Components),
	}
	return d, w.complete, nil
}

// busPrefix is a vector bus name without its index range: "PP_OUT[8..15]" -> "PP_OUT".
func busPrefix(name string) string {
	if i := strings.IndexByte(name, '['); i >= 0 {
		return name[:i]
	}
	return name
}

// slicedBusPrefixes names the bus prefixes this sheet cuts into more than one range, which is the
// case where a member name stops identifying a signal. `vme-wren` draws PP_OUT[0..31] alongside
// PP_OUT[0..7], PP_OUT[8..15], PP_OUT[16..23] and PP_OUT[24..31], and hands each slice to one
// instance of the same 8-wide driver sheet. Every instance's pin is PP_OUT[0..7], so its member
// PP_OUT1 is the parent's PP_OUT1 in one instance and PP_OUT9 in the next: KiCad maps the two buses
// by BIT POSITION, and only a positional map gets that right. Promoting by name would merge nets the
// design keeps apart, so a sliced prefix is left alone.
func slicedBusPrefixes(root *node) map[string]bool {
	ranges := map[string]map[string]bool{}
	for _, tag := range []string{"label", "global_label", "hierarchical_label"} {
		for _, l := range root.Children(tag) {
			name := unescapeName(atomOf(l.Arg(1)))
			if !isKiCadBusVector(name) {
				continue
			}
			pfx := busPrefix(name)
			if ranges[pfx] == nil {
				ranges[pfx] = map[string]bool{}
			}
			ranges[pfx][name] = true
		}
	}
	out := map[string]bool{}
	for pfx, rs := range ranges {
		if len(rs) > 1 {
			out[pfx] = true
		}
	}
	return out
}

// sheetBusNames solves ONE sheet's bus geometry and returns the bus name at each bus point.
//
// It is a second, tiny net solve over the `(bus ...)` segments alone, with the sheet's labels as
// anchors, reusing the same union-find the wire solve uses. Buses are otherwise not modelled (a bus
// is a drawing convention, and its members connect through the tap labels), but a bus SHEET PIN
// needs to know which bus it sits on, and that is a connectivity question about the bus itself.
func sheetBusNames(root *node) map[netgraph.Point]string {
	var segs []netgraph.Wire
	for _, b := range root.Children("bus") {
		pts := xyPoints(b.Child("pts"), sheetPt)
		for i := 0; i+1 < len(pts); i++ {
			segs = append(segs, netgraph.Wire{A: gp(pts[i]), B: gp(pts[i+1])})
		}
	}
	if len(segs) == 0 {
		return nil
	}
	var anchors []netgraph.Anchor
	var onBus []netgraph.Point
	for _, tag := range []string{"label", "global_label", "hierarchical_label"} {
		for _, l := range root.Children(tag) {
			if at := sheetPt(l.Child("at")); at != nil {
				anchors = append(anchors, netgraph.Anchor{At: gp(at), Label: unescapeName(atomOf(l.Arg(1)))})
				onBus = append(onBus, gp(at))
			}
		}
	}
	// A sheet pin is registered as a label-less anchor purely so its point appears in the map; an
	// empty label neither names nor unions anything.
	for _, sh := range root.Children("sheet") {
		for _, p := range sh.Children("pin") {
			if at := sheetPt(p.Child("at")); at != nil {
				anchors = append(anchors, netgraph.Anchor{At: gp(at)})
				onBus = append(onBus, gp(at))
			}
		}
	}
	// A bus label is placed ALONG the bus, not at a segment end, so the segments have to be split
	// at it exactly as the wire solve splits wires — otherwise the label is an isolated point that
	// names nothing and every bus comes back anonymous.
	_, _, _, pointNets := netgraph.BuildWithPoints(splitWiresAt(segs, onBus), anchors, nil, nil)
	out := map[netgraph.Point]string{}
	for pt, ref := range pointNets {
		out[pt] = ref.Name
	}
	return out
}

// busPinPromotions returns the name overrides a child sheet inherits through sub's sheet pins.
//
// A scalar sheet pin joins its two halves positionally: the parent drops an anchor at the pin
// carrying the CHILD-qualified name, which is the same string the child's hierarchical_label emits,
// so label-union does the rest. A BUS sheet pin cannot work that way, because the members are not
// at the pin — each one is tapped off the bus somewhere else on each sheet, under its own label. So
// the join is by NAME instead: every member of a crossing bus vector resolves, inside the child, to
// the name it has in the PARENT (agni issue 561). `AN[0..7]` entering `inout_user` makes the child's
// `AN0` label mean the root's `AN0` rather than `/inout_user/AN0`, which is what KiCad does and what
// its own netlist export shows.
//
// It promotes ONLY when the parent's bus at that pin carries the same vector name as the pin, which
// is the case where joining by member name and joining by bit position are the same answer. KiCad
// really joins the two buses positionally, so a parent bus under another name maps its bit 0 to the
// child's bit 0 whatever the two are called, and promoting by name there is wrong in the expensive
// direction: `vme-wren` instantiates one driver sheet four times, and name-promotion merged four
// distinct bank buses into one net. Skipping leaves those split, which is the status quo and is
// visible in the oracle baseline, rather than inventing a connection the design does not have.
//
// Only the members are promoted, never the bus name itself: the parent's anchor for the bus pin is
// the child-qualified `/<sheet>/AN[0..7]`, and promoting that would break the very join it makes.
// Promotion composes through nesting, because sc.local already carries whatever this sheet inherited.
func busPinPromotions(sub *node, sc sheetScope, busAt map[netgraph.Point]string, sliced map[string]bool) map[string]string {
	var out map[string]string
	for _, p := range sub.Children("pin") {
		name := unescapeName(atomOf(p.Arg(1)))
		if !isKiCadBusVector(name) {
			continue
		}
		at := sheetPt(p.Child("at"))
		if at == nil || busAt[gp(at)] != name || sliced[busPrefix(name)] {
			continue
		}
		for _, m := range netgraph.ExpandBusName(name) {
			if out == nil {
				out = map[string]string{}
			}
			out[m] = sc.local(m)
		}
	}
	return out
}

// kicadBusVectorRe is KiCad's vector-bus spelling and ONLY it: `PREFIX[first..last]`, two dots.
// netgraph.IsBusName also accepts the `[hi:lo]` form that xschem and gEDA use, which is right for
// the shared bus-not-modeled diagnostic but wrong to act on here. KiCad reads `DATA[1:0]` as an
// ordinary scalar label, and its own netlist export keeps `/DATA0` and `/sub/DATA0` apart for such
// a label — so promoting members off one would invent a connection the design does not have.
// TestHierBusMembersDoNotCross is that control.
var kicadBusVectorRe = regexp.MustCompile(`^.*\[\d+\.\.\d+\]$`)

func isKiCadBusVector(name string) bool { return kicadBusVectorRe.MatchString(name) }

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
	srcs     []string            // per-instance source file, indexed by the instance's offset band
	buses    []*ir.BusNotModeled // bus constructs detected across sheets (WS1-034)
	complete bool
	open     func(relPath string) ([]byte, error)
}

// walk reads one sheet instance (content) at hierarchical id, appends its libraries,
// components, nets, and sheet record, then recurses into referenced sub-sheets. ancestors is
// the source-file set on the path from the root, for cycle breaking; instPath is the KiCad
// instance path used to resolve per-instance reference designators.
func (w *hierNetWalker) walk(content []byte, src, id, instPath string, ancestors map[string]bool, promoted map[string]string) error {
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
	sc := sheetScope{offset: netgraph.Point{X: k * instStep}, instPath: instPath, src: src, syms: w.syms, promoted: promoted}
	if id != "/" {
		sc.prefix = id
		name = path.Base(id)
	}
	busAt, slicedBus := sheetBusNames(root), slicedBusPrefixes(root)
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
		if err := w.walk(childBytes, file, childID, instPath+"/"+uuidOf(sub), childAnc, busPinPromotions(sub, sc, busAt, slicedBus)); err != nil {
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

	var walk func(content []byte, src, id, instPath string, ancestors map[string]bool, promoted map[string]string)
	walk = func(content []byte, src, id, instPath string, ancestors map[string]bool, promoted map[string]string) {
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
		busAt, slicedBus := sheetBusNames(root), slicedBusPrefixes(root)
		sc := sheetScope{offset: netgraph.Point{X: k * instStep}, instPath: instPath, src: src, syms: syms, wirePfx: id, promoted: promoted}
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
			walk(childBytes, file, childID, instPath+"/"+uuidOf(sub), childAnc, busPinPromotions(sub, sc, busAt, slicedBus))
		}
	}

	if open == nil {
		open = func(string) ([]byte, error) { return nil, fmt.Errorf("no sub-sheet opener") }
	}
	walk(rootContent, rootName, "/", "", map[string]bool{}, nil)
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
