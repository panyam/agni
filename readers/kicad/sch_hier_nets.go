package kicad

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
	"github.com/panyam/agni/internal/refdes"
)

// instStep is the X offset between sheet instances in the design-wide net solve, so wires join only
// within their own sheet while labels union across bands. About 2.2e12 nm (2.2 km) per band dwarfs any
// sheet (an A0 page is ~1.2e9 nm) and keeps thousands of instances far from int64 range.
const instStep = int64(1) << 41

// ReadSchematicHierarchyNets reads a schematic and its sub-sheet tree into ONE netlist Design
// (WS1-018), with per-instance reference designators for reused sheet files. It shares
// ReadSchematicHierarchy's traversal, cycle guard and sheet ids ("/", "/<Sheetname>", ...), so
// netlist and geometry agree on sheet identity.
//
// open fetches a child by its relative Sheetfile path, resolved by the caller, so this package does
// no file I/O (CONSTRAINTS C1). The returned complete flag is true only when every referenced
// sub-sheet was opened and walked. A missing child skips its subtree and leaves complete false, and
// the caller gates WS1-017's external->global downgrade on it (see ReadProject). A nil open reads the
// root sheet alone, complete only if it references no sub-sheets.
//
// Net names match kicad-cli's netlist export: globals, power rails and root-sheet locals are bare,
// sub-sheet local and hierarchical names carry the instance's sheet path
// ("/ampli_ht_vertical/PIEZO_IN"), and bare beats qualified with shallower winning ties. See
// docsite/content/architecture/net-solving.md#the-hierarchy-walk.
func ReadSchematicHierarchyNets(rootName string, rootContent []byte, open func(relPath string) ([]byte, error)) (*ir.Design, bool, error) {
	return ReadSchematicHierarchyNetsWithSymbols(rootName, rootContent, open, nil)
}

// ReadSchematicHierarchyNetsWithSymbols is the walk plus external symbol-library resolution
// (WS1-016). openSym fetches .kicad_sym bytes by library nickname for lib_id references no sheet
// embeds. One cache serves the whole walk, and a nil openSym resolves nothing.
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
		aliases:  projectBusAliases(rootContent, open),
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
		// Declared even when empty, so an empty list reads as "looked, found none" (agni issue 309).
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

// kicadBusVectorRe matches KiCad's vector-bus spelling `PREFIX[first..last]` (two dots) and ONLY it.
// netgraph.IsBusName also accepts xschem and gEDA's `[hi:lo]`, which suits the shared bus-not-modeled
// diagnostic, but KiCad reads `DATA[1:0]` as a scalar label and its netlist export keeps `/DATA0` and
// `/sub/DATA0` apart, so promoting members off one would invent a connection.
// TestHierBusMembersDoNotCross is the control.
var kicadBusVectorRe = regexp.MustCompile(`^(.*)\[(\d+)\.\.(\d+)\]$`)

// groupBus splits a group bus label, KiCad's `PREFIX{...}` spelling, into the prefix its members are
// named under (`PREFIX.MEMBER`) and its members, reporting false for anything that is not one.
//
// The braces either name a `bus_alias`, as the jetson baseboard writes `I2C7{I2C}`, or list the members
// inline, separated by spaces or commas, as the RoyalBlue54L Feather writes `ANALOG{A[0..5]}`,
// `DIG{D5 D6 D[9..13]}` and `SWD_TRG{~{RESET}, SWDIO, SWDCLK}`. A listed member may be a vector,
// which expands, and may carry formatting braces of its own (agni issue 936). The group is the LAST
// top-level brace pair, so a prefix may carry a subscript, as in `I2C_{SYS}{I2C}`, 33 of the jetson
// baseboard's 224 group-bus occurrences.
//
// A brace pair right after `_`, `^` or `~` is KiCad's subscript, superscript or overbar, so `V_{OUT}`
// and `SWD_TRG.~{RESET}` are scalar labels. Reading one as a group would promote a member across a
// sheet boundary and join nets the design keeps apart.
func groupBus(name string, aliases map[string][]string) (prefix string, members []string, ok bool) {
	open := lastBraceGroup(name)
	if open <= 0 {
		return "", nil, false
	}
	prefix, inner := name[:open], name[open+1:len(name)-1]
	if strings.ContainsAny(prefix[len(prefix)-1:], "_^~") {
		return "", nil, false
	}
	if members, ok := aliases[inner]; ok && len(members) > 0 {
		return prefix, members, true
	}
	for _, m := range splitGroupMembers(inner) {
		if v := busMembersAscending(m); v != nil {
			members = append(members, v...)
		} else {
			members = append(members, m)
		}
	}
	if len(members) == 0 {
		return "", nil, false
	}
	return prefix, members, true
}

// lastBraceGroup is the index of the `{` that opens a trailing top-level brace pair in name, or -1 when
// name does not end in one.
func lastBraceGroup(name string) int {
	if !strings.HasSuffix(name, "}") {
		return -1
	}
	depth := 0
	for i := len(name) - 1; i >= 0; i-- {
		switch name[i] {
		case '}':
			depth++
		case '{':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// splitGroupMembers splits an inline member list at spaces and commas outside any formatting braces.
func splitGroupMembers(inner string) []string {
	var out []string
	depth, start := 0, 0
	flush := func(end int) {
		if m := strings.TrimSpace(inner[start:end]); m != "" {
			out = append(out, m)
		}
	}
	for i, r := range inner {
		switch {
		case r == '{':
			depth++
		case r == '}':
			depth--
		case depth == 0 && (r == ' ' || r == ','):
			flush(i)
			start = i + 1
		}
	}
	flush(len(inner))
	return out
}

// busMembersAscending expands a KiCad vector bus into its members by ASCENDING index, the order two
// buses pair up in, and returns nil for anything that is not one. kicad-cli joins bit 0 of a child's
// `B[0..1]` to `A0` of a parent's `A[3..0]`, not to `A3`. netgraph.ExpandBusName keeps the WRITTEN
// direction so a diagram reads its bits as drawn, so this cannot reuse it.
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

// sheetBusNames maps each bus sheet pin on a sheet to the name of the bus BRANCH it sits on, the
// nearest bus label along the `(bus ...)` segments.
//
// Nearest rather than one name per connected bus, because a trunk's labelled branches carry different
// members. `vme-wren` cuts PP_OUT[0..31] into four eight-wide slices for four instances of one driver
// sheet, and naming the cluster would map every instance's bit 0 onto the same parent member. Buses
// wired together share members by NAME, while a bus crossing a sheet pin maps BY BIT POSITION, so the
// branch's own label is the one that decides. See
// docsite/content/architecture/net-solving.md#buses-cross-by-a-different-rule.
func sheetBusNames(root *node, aliases map[string][]string) map[netgraph.Point]string {
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
			if name := unescapeName(atomOf(l.Arg(1))); isBusLabel(name, aliases) {
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
	// A label sits ALONG a bus rather than at a segment end, so split the segments at every label and
	// pin as the wire solve splits wires. Without the split every bus comes back anonymous.
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
			// Nothing labels the branch, so the pins on it name it. A GROUP bus joining sibling sheets
			// takes one name for the whole bus, so members of the same name meet even when the two
			// sheets spell the group differently: kicad-cli joins the Feather's `I2C{SCL, SDA}` and
			// `I2C_54{SCL, SDA}` across one bare bus wire (agni issue 936). Anything else keeps its own
			// name and its members cross as spelled.
			out[pin] = pinName
			if _, _, ok := groupBus(pinName, aliases); ok {
				out[pin] = groupBusNameOf(pin, adj, pinAt, aliases)
			}
		}
	}
	return out
}

// groupBusNameOf is the name an unlabelled bus carries: the first, in sorted order, of the group-bus
// pins it connects, which is every pin reachable from start along the bus.
func groupBusNameOf(start netgraph.Point, adj map[netgraph.Point][]netgraph.Point, pinAt map[netgraph.Point]string, aliases map[string][]string) string {
	best := pinAt[start]
	seen := map[netgraph.Point]bool{start: true}
	frontier := []netgraph.Point{start}
	for len(frontier) > 0 {
		var next []netgraph.Point
		for _, p := range frontier {
			if name, ok := pinAt[p]; ok && name < best {
				if _, _, group := groupBus(name, aliases); group {
					best = name
				}
			}
			for _, q := range adj[p] {
				if !seen[q] {
					seen[q] = true
					next = append(next, q)
				}
			}
		}
		frontier = next
	}
	return best
}

// isBusLabel reports whether a label names a bus this walk acts on, in either of KiCad's two
// spellings. Everything else stays a scalar label, since a label wrongly read as a bus promotes member
// names across a sheet boundary and joins nets the design keeps apart.
func isBusLabel(name string, aliases map[string][]string) bool {
	if busMembersAscending(name) != nil {
		return true
	}
	_, _, ok := groupBus(name, aliases)
	return ok
}

// nearestBusLabel breadth-first searches the bus graph from start and returns the closest bus
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

// reusedSheetFiles names the sub-sheet files this sheet instantiates more than once. Every instance's
// bus pin is spelled identically, so promoting by the pin's own members would merge the instances,
// and nearestBusLabel cannot resolve the branch on a bus that forks four ways (`vme-wren` places one
// driver sheet four times off slices of PP_OUT[0..31]). A reused sheet promotes nothing and its nets
// stay split.
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

// busPinPromotions returns the name overrides a child sheet inherits through sub's BUS sheet pins, or
// nil for a reused sheet. A bus's members are tapped off elsewhere on each sheet rather than at the
// pin, so each member of a crossing bus resolves, inside the child, to the name it lands on in the
// PARENT (agni issue 561).
//
// Vectors pair BY BIT POSITION, not member name, so a rename carries across. kicad-cli joins a child's
// `PP_OUT0` to the parent's `PP_OUT2` when the parent's bus at that pin is `PP_OUT[2..3]`, and pairs
// off only as far as the shorter bus. Only members are promoted, never the bus name, because the
// parent's anchor for the pin is the child-qualified `/<sheet>/AN[0..7]` and promoting it would break
// that join. Promotion composes through nesting since sc.local already carries what this sheet
// inherited. See docsite/content/architecture/net-solving.md#buses-cross-by-a-different-rule.
func busPinPromotions(sub *node, sc sheetScope, busAt map[netgraph.Point]string, aliases map[string][]string, reused bool) map[string]string {
	if reused {
		return nil
	}
	out := map[string]string{}
	// A child member must promote to exactly one parent name. The rules below never produce two, so a
	// collision means an assumption here is wrong, and the member stays unpromoted rather than picking.
	conflict := map[string]bool{}
	set := func(from, to string) {
		if prev, ok := out[from]; ok && prev != to {
			conflict[from] = true
			return
		}
		out[from] = to
	}
	for _, p := range sub.Children("pin") {
		at := sheetPt(p.Child("at"))
		if at == nil {
			continue
		}
		pinName, busName := unescapeName(atomOf(p.Arg(1))), busAt[gp(at)]
		// A VECTOR pairs by bit position, so a rename across the boundary carries.
		if mine, theirs := busMembersAscending(pinName), busMembersAscending(busName); mine != nil && theirs != nil {
			for j := 0; j < len(mine) && j < len(theirs); j++ {
				set(mine[j], sc.local(theirs[j]))
			}
			continue
		}
		// A GROUP BUS pairs by member NAME, measured against kicad-cli. A parent `A0{ALPHA}` of
		// members XX/YY against a child pin `B0{BETA}` of members PP/QQ joins NOTHING, so pairing by
		// position would short two unrelated signals while moving the net count the right way.
		// Members come from the parent's bus, re-prefixed with the child's, so a child alias
		// declaring other members yields names no net carries and the entries never apply.
		childPrefix, _, okChild := groupBus(pinName, aliases)
		parentPrefix, members, okParent := groupBus(busName, aliases)
		if !okChild || !okParent {
			continue
		}
		for _, m := range members {
			set(childPrefix+"."+m, sc.local(parentPrefix+"."+m))
		}
	}
	for k := range conflict {
		delete(out, k)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// projectBusAliases unions the bus_alias declarations of every sheet in the tree, because an alias
// resolves PROJECT-WIDE though it is written into one sheet file. Nine of the fourteen jetson sheets
// that USE a group bus declare no alias, kicad-cli crosses the boundary unchanged with the alias only
// in the CHILD, and resolving per file recognized 12 of that board's 251 split members.
//
// The first declaration of a name wins, in traversal order. A missing or unreadable sub-sheet is
// skipped here and reported by the walk.
func projectBusAliases(rootContent []byte, open func(string) ([]byte, error)) map[string][]string {
	out := map[string][]string{}
	seen := map[string]bool{}
	var visit func(content []byte, depth int)
	visit = func(content []byte, depth int) {
		if depth > 64 {
			return
		}
		root, err := parse(bytes.NewReader(content))
		if err != nil {
			return
		}
		for name, members := range busAliases(root) {
			if _, ok := out[name]; !ok {
				out[name] = members
			}
		}
		for _, sub := range root.Children("sheet") {
			file := propValue(sub, "Sheetfile")
			if file == "" || seen[file] {
				continue
			}
			seen[file] = true
			child, err := open(file)
			if err != nil {
				continue
			}
			visit(child, depth+1)
		}
	}
	visit(rootContent, 0)
	if len(out) == 0 {
		return nil
	}
	return out
}

// hierNetWalker carries the hierarchical netlist walk's accumulator state. One walk call runs per
// sheet instance.
type hierNetWalker struct {
	// aliases is the PROJECT-wide bus_alias table, collected before the walk because a sheet may use
	// an alias another sheet declares.
	aliases  map[string][]string
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
	busAt, reusedSheet := sheetBusNames(root, w.aliases), reusedSheetFiles(root)
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
		// Each sheet pin gets the parent half of its port, an anchor carrying the CHILD-qualified
		// name the child's hierarchical_label emits, so label-union joins the two sheets. Emitted
		// even when the child fails to open, so a partial read still names the parent net.
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
		if err := w.walk(childBytes, file, childID, instPath+"/"+uuidOf(sub), childAnc, busPinPromotions(sub, sc, busAt, w.aliases, reusedSheet[propValue(sub, "Sheetfile")])); err != nil {
			return err
		}
	}
	return nil
}

// hierWireNets runs the same combined solve as ReadSchematicHierarchyNets and returns each wire's
// uuid -> solved net name (WS1-022). The geometry reader uses it so its wire net names are
// byte-identical to the netlist read's (same N$ numbering, same per-instance qualification), which a
// finding's net -> wire -> sheet join needs. It repeats the walk's traversal but collects only solver
// inputs. A nil open reads the root sheet alone.
func hierWireNets(rootName string, rootContent []byte, open, openSym func(string) ([]byte, error)) map[string]netgraph.NetRef {
	syms := newSymLibCache(openSym)
	aliases := projectBusAliases(rootContent, open)
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
		busAt, reusedSheet := sheetBusNames(root, aliases), reusedSheetFiles(root)
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
			walk(childBytes, file, childID, instPath+"/"+uuidOf(sub), childAnc, busPinPromotions(sub, sc, busAt, aliases, reusedSheet[propValue(sub, "Sheetfile")]))
		}
	}

	if open == nil {
		open = func(string) ([]byte, error) { return nil, fmt.Errorf("no sub-sheet opener") }
	}
	walk(rootContent, rootName, "/", "", map[string]bool{}, nil)
	_, _, wireNets := netgraph.Build(in.wires, in.anchors, in.pins, in.terminals)
	return wireNets
}

// stampNetSheets attributes each net the sheet instances it touches (WS9-028), so a net finding gets
// a sheet badge like a component one. Membership comes from pointNets and each point's X band (the
// instStep offset hierDangles decodes), so a wireless single-pin net on a sub-sheet is placed too. Ids
// come out in sheet order. A single-sheet read gets no attribute, since the badge layer hides badges
// below two sheets.
func stampNetSheets(nets []*ir.Net, pointNets map[netgraph.Point]netgraph.NetRef, sheets []*ir.Sheet) {
	if len(sheets) <= 1 {
		return
	}
	// Key by the per-instance net id, NOT the name (WS9), or two distinct nets sharing a name both get
	// the union of their sheets and a design-global finding lands on every sheet. A pinless net has
	// no id and keys by name.
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
