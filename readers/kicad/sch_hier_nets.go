package kicad

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"

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

// kicadBusVectorRe is KiCad's vector-bus spelling and ONLY it: `PREFIX[first..last]`, two dots.
// netgraph.IsBusName also accepts the `[hi:lo]` form that xschem and gEDA use, which is right for
// the shared bus-not-modeled diagnostic but wrong to act on here. KiCad reads `DATA[1:0]` as an
// ordinary scalar label, and its own netlist export keeps `/DATA0` and `/sub/DATA0` apart for such
// a label, so promoting members off one would invent a connection the design does not have.
// TestHierBusMembersDoNotCross is that control.
var kicadBusVectorRe = regexp.MustCompile(`^(.*)\[(\d+)\.\.(\d+)\]$`)

// busMembersAscending expands a KiCad vector bus into its members ordered by ASCENDING INDEX, which
// is the order two buses are matched up in, and returns nil for anything that is not one.
//
// Ascending rather than written, and the distinction is load-bearing rather than tidiness. Asked
// directly, kicad-cli joins the child's bit 0 of `B[0..1]` to `A0` of a parent bus spelled `A[3..0]`,
// not to `A3`. netgraph.ExpandBusName deliberately preserves the WRITTEN direction so a diagram reads
// its bits as drawn, so this cannot reuse it.
func busMembersAscending(name string) []string {
	m := kicadBusVectorRe.FindStringSubmatch(name)
	if m == nil {
		return nil
	}
	first, _ := strconv.Atoi(m[2])
	last, _ := strconv.Atoi(m[3])
	if first > last {
		first, last = last, first
	}
	out := make([]string, 0, last-first+1)
	for i := first; i <= last; i++ {
		out = append(out, m[1]+strconv.Itoa(i))
	}
	return out
}

// sheetBusNames resolves, for each bus sheet pin on a sheet, the name of the bus BRANCH it sits on.
//
// It walks the `(bus ...)` segments outward from each pin and takes the first vector label it
// reaches. Nearest-label rather than one name for the whole bus, because a bus is routinely drawn as
// a trunk with labelled branches and the branches carry different members: `vme-wren` wires
// PP_OUT[0..31] to PP_OUT[0..7], PP_OUT[8..15], PP_OUT[16..23] and PP_OUT[24..31], and hands one
// slice to each of four instances of the same driver sheet. Every one of those pins is on the same
// connected bus, so a solve that names the cluster gives all four the same answer and maps every
// instance's bit 0 onto the same parent member.
//
// That is consistent with how KiCad joins buses. Two buses wired together share the members whose
// NAMES match, which is what makes a trunk and its slices one drawing; a bus crossing a sheet pin
// maps BY BIT POSITION, which is what makes the branch's own label the one that matters.
//
// Buses are otherwise not modelled, and should not be: a bus is a drawing convention whose members
// connect through their tap labels. Which bus a pin sits on is a question about the bus itself.
func sheetBusNames(root *node) map[netgraph.Point]string {
	type seg struct{ a, b netgraph.Point }
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
	labelAt := map[netgraph.Point]string{}
	var onBus []netgraph.Point
	for _, tag := range []string{"label", "global_label", "hierarchical_label"} {
		for _, l := range root.Children(tag) {
			at := sheetPt(l.Child("at"))
			if at == nil {
				continue
			}
			onBus = append(onBus, gp(at))
			if name := unescapeName(atomOf(l.Arg(1))); busMembersAscending(name) != nil {
				labelAt[gp(at)] = name
			}
		}
	}
	pinAt := map[netgraph.Point]string{}
	for _, sh := range root.Children("sheet") {
		for _, p := range sh.Children("pin") {
			if at := sheetPt(p.Child("at")); at != nil {
				onBus = append(onBus, gp(at))
				pinAt[gp(at)] = unescapeName(atomOf(p.Arg(1)))
			}
		}
	}
	// A label sits ALONG a bus rather than at a segment end, so the segments split at every label and
	// pin exactly as the wire solve splits wires. Without it a label is an isolated point naming
	// nothing, and every bus comes back anonymous.
	adj := map[netgraph.Point][]netgraph.Point{}
	for _, w := range splitWiresAt(segs, onBus) {
		adj[w.A] = append(adj[w.A], w.B)
		adj[w.B] = append(adj[w.B], w.A)
	}

	out := map[netgraph.Point]string{}
	for pin, pinName := range pinAt {
		if name, ok := nearestBusLabel(pin, adj, labelAt); ok {
			out[pin] = name
		} else if _, onIt := adj[pin]; onIt {
			// Nothing labels the branch, so the pin names it and the members still cross as spelled.
			out[pin] = pinName
		}
	}
	return out
}

// nearestBusLabel breadth-first searches the bus graph from start and returns the closest vector
// label. It reports false when nothing is reachable, and when two DIFFERENT labels tie at the same
// distance, since the drawing then does not say which branch the pin is on and guessing would invent
// a connection.
func nearestBusLabel(start netgraph.Point, adj map[netgraph.Point][]netgraph.Point, labelAt map[netgraph.Point]string) (string, bool) {
	seen := map[netgraph.Point]bool{start: true}
	frontier := []netgraph.Point{start}
	for len(frontier) > 0 {
		var found []string
		var next []netgraph.Point
		for _, p := range frontier {
			if name, ok := labelAt[p]; ok && !slices.Contains(found, name) {
				found = append(found, name)
			}
			for _, q := range adj[p] {
				if !seen[q] {
					seen[q] = true
					next = append(next, q)
				}
			}
		}
		if len(found) == 1 {
			return found[0], true
		}
		if len(found) > 1 {
			return "", false // a tie names no single branch
		}
		frontier = next
	}
	return "", false
}

// reusedSheetFiles names the sub-sheet files this sheet instantiates more than once.
//
// One file placed several times is where member names stop identifying a signal hardest: every
// instance's bus pin is spelled identically, so promoting by the pin's own members maps them all
// onto whichever branch resolved, and the instances merge. `vme-wren` places one eight-wide driver
// sheet four times off slices of PP_OUT[0..31]. Getting those right needs the branch a pin sits on
// resolved exactly, and nearestBusLabel does not manage it on a bus that forks four ways, so a
// reused sheet promotes nothing and its nets stay split.
func reusedSheetFiles(root *node) map[string]bool {
	n := map[string]int{}
	for _, sh := range root.Children("sheet") {
		n[propValue(sh, "Sheetfile")]++
	}
	out := map[string]bool{}
	for f, c := range n {
		if c > 1 {
			out[f] = true
		}
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
// the name of the member it lands on in the PARENT (agni issue 561).
//
// The two buses match up BY BIT POSITION and not by member name, which is the whole reason this is a
// map rather than a rename. Asked directly, kicad-cli joins a child's `PP_OUT0` to the parent's
// `PP_OUT2` when the parent's bus at that pin is `PP_OUT[2..3]`, joins `B0` to `A0` across a rename,
// and pairs off as far as the shorter of the two when the widths differ. Where the two names happen
// to be equal the map is the identity, which is why `AN[0..7]` on both sides needs no special case.
//
// Only the members are promoted, never the bus name itself: the parent's anchor for the bus pin is
// the child-qualified `/<sheet>/AN[0..7]`, and promoting that would break the very join it makes.
// Promotion composes through nesting, because sc.local already carries whatever this sheet inherited.
func busPinPromotions(sub *node, sc sheetScope, busAt map[netgraph.Point]string, reused bool) map[string]string {
	if reused {
		return nil
	}
	var out map[string]string
	for _, p := range sub.Children("pin") {
		at := sheetPt(p.Child("at"))
		if at == nil {
			continue
		}
		mine := busMembersAscending(unescapeName(atomOf(p.Arg(1))))
		theirs := busMembersAscending(busAt[gp(at)])
		for j := 0; j < len(mine) && j < len(theirs); j++ {
			if out == nil {
				out = map[string]string{}
			}
			out[mine[j]] = sc.local(theirs[j])
		}
	}
	return out
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
	busAt, reusedSheet := sheetBusNames(root), reusedSheetFiles(root)
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
		if err := w.walk(childBytes, file, childID, instPath+"/"+uuidOf(sub), childAnc, busPinPromotions(sub, sc, busAt, reusedSheet[propValue(sub, "Sheetfile")])); err != nil {
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
		busAt, reusedSheet := sheetBusNames(root), reusedSheetFiles(root)
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
			walk(childBytes, file, childID, instPath+"/"+uuidOf(sub), childAnc, busPinPromotions(sub, sc, busAt, reusedSheet[propValue(sub, "Sheetfile")]))
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
