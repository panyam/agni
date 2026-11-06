package orcad

import (
	"fmt"
	"path"
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// sets is a union-find over node keys.
type sets struct{ parent map[string]string }

func (s *sets) find(x string) string {
	if _, ok := s.parent[x]; !ok {
		s.parent[x] = x
	}
	for s.parent[x] != x {
		s.parent[x] = s.parent[s.parent[x]]
		x = s.parent[x]
	}
	return x
}

func (s *sets) join(a, b string) { s.parent[s.find(a)] = s.find(b) }

// netName is a name a net carries, at the block depth it was found.
type netName struct {
	name   string
	depth  int
	global bool
	path   string // the block path a local name belongs to
	isGen  bool   // Capture generated the name, since the net carries none of its own
	gen    uint32 // a generated name's number
}

// generated is the name Capture gives a net that carries none, N and the number padded to five
// digits. Capture numbers it from the lowest wire id on the net, so Clay's charger block names the
// net on wires 299, 467 and 490 N00299. A net no wire carries is numbered from the page net id
// its pins name; Capture names those some other way not yet found (agni issue 1406).
func generated(id uint32, depth int, bpath string) netName {
	return netName{name: fmt.Sprintf("N%05d", id), depth: depth, path: bpath, isGen: true, gen: id}
}

// placement is one placed part, resolved to its designator and its cached definitions.
type placement struct {
	ref   string
	part  placedPart
	d     *dsn
	src   string
	pkg   *pkg
	lp    *libPart
	pinOf map[int16]string // placed pin index -> pin number
}

type flattener struct {
	open    Opener
	designs map[string]*dsn
	globals map[string]bool
	sets    sets
	names   map[string][]netName // node -> names recorded on it
	genName map[string]netName   // node -> Capture's generated name, for a net that carries none
	pins    map[string][2]string // pin node -> ref, number
	placed  []placement
	missing []*ir.UnexpandedHierarchy

	// visits is every page the walk reached, once per page however often its block is placed,
	// with the sheet that places it. netOf names each net's root once nets has run.
	visits  []visit
	sheetOf map[string]string // page key -> the sheet id it draws under
	parent  string
	netOf   map[string]string
}

// visit is one page as the walk first reached it.
type visit struct {
	d      *dsn
	pg     *page
	pk     string // the prefix of its wire nodes
	bpath  string
	sheet  string
	parent string
	occ    *scope
}

func newFlattener(top *dsn, open Opener) *flattener {
	return &flattener{
		open:    open,
		designs: map[string]*dsn{top.name: top},
		globals: map[string]bool{},
		sets:    sets{parent: map[string]string{}},
		names:   map[string][]netName{},
		genName: map[string]netName{},
		pins:    map[string][2]string{},
		sheetOf: map[string]string{},
		netOf:   map[string]string{},
	}
}

// run walks the root design and every block it places, depth first.
func (f *flattener) run(top *dsn) error {
	f.collectGlobals(top, map[string]bool{})
	return f.walk(top, top.root, "", top.occ, 0, map[string]bool{top.name: true})
}

// flatten walks the root design and every block it places, and builds the design.
func flatten(top *dsn, open Opener) (*ir.Design, error) {
	f := newFlattener(top, open)
	if err := f.run(top); err != nil {
		return nil, err
	}
	return f.design(top), nil
}

// pageKey names a page by its design, folder and page, so a block page placed twice is drawn once.
func pageKey(d *dsn, folder string, pg *page) string {
	return path.Base(d.name) + ":" + folder + ":" + pg.name
}

// sheetPath is the sheet id a page draws under, its block path as KiCad's are: /IMU for the one
// page of the block IMU, /IMU/PAGE2 for a page of a block with several, and /PAGE1 for a page of
// the top folder.
func sheetPath(bpath string, pages int, pg *page) string {
	if bpath == "" || pages > 1 {
		return bpath + "/" + pg.name
	}
	return bpath
}

// collectGlobals records every power symbol's name across the design and the blocks it reaches,
// because a power name joins nets in every block whether or not that block draws the symbol.
func (f *flattener) collectGlobals(d *dsn, seen map[string]bool) {
	if seen[d.name] {
		return
	}
	seen[d.name] = true
	for _, pages := range d.folders {
		for _, pg := range pages {
			for _, s := range pg.symbols {
				if s.kind == symGlobal && s.name != "" {
					f.globals[s.name] = true
				}
			}
			for _, bl := range pg.blocks {
				if child := f.child(d, bl); child != nil {
					f.collectGlobals(child, seen)
				}
			}
		}
	}
}

// child opens a block's design once, or returns nil when it cannot be opened.
func (f *flattener) child(parent *dsn, bl block) *dsn {
	if bl.file == "" {
		return parent // the block's pages are a folder of its own design
	}
	if f.open == nil {
		return nil
	}
	name, data, err := f.open(parent.name, bl.file)
	if err != nil {
		return nil
	}
	if d, ok := f.designs[name]; ok {
		return d
	}
	d, err := load(data, name)
	if err != nil {
		return nil
	}
	f.designs[name] = d
	return d
}

func (f *flattener) addName(node string, n netName) {
	if n.name != "" {
		f.names[node] = append(f.names[node], n)
	}
}

// walk instantiates one schematic folder at a block path.
func (f *flattener) walk(d *dsn, folder, bpath string, sc *scope, depth int, open map[string]bool) error {
	pages := d.folders[folder]
	ports := folderPorts(pages)
	for pi, pg := range pages {
		pk := fmt.Sprintf("%s|%s|%d", bpath, folder, pi)
		folderKey := "name|" + bpath + "|" + folder + "|"
		key := pageKey(d, folder, pg)
		if f.sheetOf[key] == "" {
			f.sheetOf[key] = sheetPath(bpath, len(pages), pg)
			f.visits = append(f.visits, visit{d: d, pg: pg, pk: pk, bpath: bpath, sheet: f.sheetOf[key], parent: f.parent, occ: sc})
		}
		sid := f.sheetOf[key]
		type at struct {
			p    point
			node string
		}
		var points []at
		var symbols []pageSymbol
		for _, w := range pg.wires {
			if w.bus {
				continue // a bus joins its members by name, which this reader does not read yet
			}
			node := fmt.Sprintf("w|%s|%d", pk, w.net)
			f.sets.find(node)
			if g, ok := f.genName[node]; !ok || w.id < g.gen {
				f.genName[node] = generated(w.id, depth, bpath)
			}
			for _, nm := range pg.netName[w.net] {
				f.sets.join(node, folderKey+strings.ToUpper(nm))
				f.addName(node, netName{name: nm, depth: depth, path: bpath})
			}
		}
		touching := func(p point) []string {
			var out []string
			for _, w := range pg.wires {
				if !w.bus && onSegment(p, w.a, w.b) {
					out = append(out, fmt.Sprintf("w|%s|%d", pk, w.net))
				}
			}
			for _, q := range points {
				if q.p == p {
					out = append(out, q.node)
				}
			}
			return out
		}
		// A symbol joins the wires whose ends lie inside its box. A power symbol makes its net global,
		// a port joins it to its block's pin, and an off-page connector joins it within the folder.
		for _, s := range pg.symbols {
			if s.name == "" {
				continue
			}
			key := symbolKey(s, bpath, folderKey, ports)
			for _, w := range pg.wires {
				if !w.bus && (s.covers(w.a) || s.covers(w.b)) {
					node := fmt.Sprintf("w|%s|%d", pk, w.net)
					f.sets.join(node, key)
					if s.kind == symGlobal && !ports[strings.ToUpper(s.name)] {
						f.addName(node, netName{name: s.name, global: true})
					}
				}
			}
			symbols = append(symbols, s)
		}
		for _, part := range pg.parts {
			pl := f.place(d, part, sc, pk)
			byName := map[string][]string{}
			for _, pin := range part.pins {
				if pin.index < 0 && pin.net == 0 {
					continue // explicitly not connected, and naming no net
				}
				num := pl.pinOf[pin.index]
				if num == "" {
					continue
				}
				node := "pin|" + pl.ref + "|" + num
				f.pins[node] = [2]string{pl.ref, num}
				f.sets.find(node)
				for _, n := range touching(pin.at) {
					f.sets.join(node, n)
				}
				// A pin can name its page net itself, even one no wire carries: two pins that touch
				// share one, and a pin flagged not-connected can still be given one.
				if pin.net != 0 {
					wnode := fmt.Sprintf("w|%s|%d", pk, pin.net)
					f.sets.join(node, wnode)
					if _, ok := f.genName[wnode]; !ok {
						f.genName[wnode] = generated(pin.net, depth, bpath)
					}
					for _, nm := range pg.netName[pin.net] {
						f.sets.join(node, folderKey+strings.ToUpper(nm))
						f.addName(node, netName{name: nm, depth: depth, path: bpath})
					}
				}
				for _, s := range symbols {
					if s.covers(pin.at) {
						f.sets.join(node, symbolKey(s, bpath, folderKey, ports))
						if s.kind == symGlobal && !ports[strings.ToUpper(s.name)] {
							f.addName(node, netName{name: s.name, global: true})
						}
					}
				}
				points = append(points, at{pin.at, node})
				if pl.lp != nil {
					if slot := int(pin.index) - 1; slot >= 0 {
						for _, sp := range pl.lp.pins {
							if sp.slot == slot {
								byName[sp.name] = append(byName[sp.name], node)
							}
						}
					}
				}
			}
			shorts := part.props["PACK_SHORT"]
			if shorts == "" && pl.lp != nil {
				shorts = pl.lp.props["PACK_SHORT"]
			}
			for _, group := range packShorts(shorts) {
				var first string
				for _, name := range group {
					for _, node := range byName[name] {
						if first == "" {
							first = node
						} else {
							f.sets.join(node, first)
						}
					}
				}
			}
			f.placed = append(f.placed, pl)
		}
		for _, bl := range pg.blocks {
			occ := sc.find(bl.db)
			ref := bl.ref
			if occ != nil && occ.ref != "" {
				ref = occ.ref
			}
			childPath := bpath + "/" + ref
			child := f.child(d, bl)
			if child == nil {
				f.missing = append(f.missing, &ir.UnexpandedHierarchy{
					Name:   ref,
					Kind:   "orcad_block",
					Reason: ir.UnexpandedReason_UNEXPANDED_REASON_FILE_MISSING,
					Prov:   &ir.Provenance{SourceFile: d.name, NativeId: bl.file},
				})
				continue
			}
			for i, pin := range bl.pins {
				if pin.index < 0 || i >= len(bl.pinNames) {
					continue
				}
				name := strings.ToUpper(bl.pinNames[i])
				node := "bpin|" + childPath + "|" + name
				for _, n := range touching(pin.at) {
					f.sets.join(node, n)
				}
				f.sets.join(node, "port|"+childPath+"|"+name)
			}
			childFolder := strings.ToLower(bl.folder)
			if _, ok := child.folders[childFolder]; !ok {
				childFolder = child.root
			}
			var childScope *scope
			if occ != nil {
				childScope = occ.nested
			}
			if open[child.name+"|"+childFolder] {
				f.missing = append(f.missing, &ir.UnexpandedHierarchy{
					Name:   ref,
					Kind:   "orcad_block",
					Reason: ir.UnexpandedReason_UNEXPANDED_REASON_CYCLE,
					Prov:   &ir.Provenance{SourceFile: d.name, NativeId: bl.file},
				})
				continue
			}
			open[child.name+"|"+childFolder] = true
			saved := f.parent
			f.parent = sid
			if err := f.walk(child, childFolder, childPath, childScope, depth+1, open); err != nil {
				return err
			}
			f.parent = saved
			delete(open, child.name+"|"+childFolder)
		}
	}
	return nil
}

// symbolKey is the node a page symbol joins: a power name across the design, a port with its
// block's pin, and an off-page connector within its schematic folder.
//
// A power symbol in a folder that also has a port of its name joins that port instead. The port
// carries the net to the parent, and Capture names the net for the parent's side: Clay's charger
// block draws GND on the net its GND port takes to the board's GND_EARTH. Names compare without
// case, as Capture's do.
func symbolKey(s pageSymbol, bpath, folderKey string, ports map[string]bool) string {
	name := strings.ToUpper(s.name)
	switch {
	case s.kind == symGlobal && ports[name]:
		return "port|" + bpath + "|" + name
	case s.kind == symGlobal:
		return "global|" + name
	case s.kind == symPort:
		return "port|" + bpath + "|" + name
	}
	return folderKey + name
}

// folderPorts is the names of a folder's ports, upper-cased.
func folderPorts(pages []*page) map[string]bool {
	out := map[string]bool{}
	for _, pg := range pages {
		for _, s := range pg.symbols {
			if s.kind == symPort && s.name != "" {
				out[strings.ToUpper(s.name)] = true
			}
		}
	}
	return out
}

// place resolves a placed part: its designator from the occurrence tree where the design has one,
// and its pin numbers from its package's device.
func (f *flattener) place(d *dsn, part placedPart, sc *scope, pk string) placement {
	pl := placement{ref: part.ref, part: part, d: d, src: d.name, pinOf: map[int16]string{}}
	if o := sc.find(part.db); o != nil && o.ref != "" {
		pl.ref = o.ref
	}
	pl.lp = d.cache.parts[part.cacheKey]
	pl.pkg = d.cache.packages[part.pkg]
	var numbers []string
	if pl.pkg != nil && part.unit < len(pl.pkg.devices) {
		numbers = pl.pkg.devices[part.unit].pins
	}
	for _, pin := range part.pins {
		i := pin.index
		if i < 0 {
			i = -i
		}
		if slot := int(i) - 1; slot >= 0 && slot < len(numbers) && numbers[slot] != "" { // a flagged pin keeps its number
			pl.pinOf[pin.index] = numbers[slot]
		}
	}
	return pl
}

// onSegment reports whether p lies on the segment from a to b.
func onSegment(p, a, b point) bool {
	cross := int64(p.x-a.x)*int64(b.y-a.y) - int64(p.y-a.y)*int64(b.x-a.x)
	if cross != 0 {
		return false
	}
	return min(a.x, b.x) <= p.x && p.x <= max(a.x, b.x) && min(a.y, b.y) <= p.y && p.y <= max(a.y, b.y)
}

// design builds the IR from the flattened walk.
func (f *flattener) design(top *dsn) *ir.Design {
	prov := func(src string) *ir.Provenance { return &ir.Provenance{SourceFile: src} }
	d := &ir.Design{
		IrVersion:    "0",
		SourceFormat: "orcad-capture",
		Name:         strings.TrimSuffix(path.Base(top.name), path.Ext(top.name)),
		Attributes:   map[string]string{"orcad_format": fmt.Sprintf("%d.%d", top.lib.major, top.lib.minor)},
		Prov:         prov(top.name),
	}

	// Components, one per designator, with a section per placed unit.
	byRef := map[string]*ir.Component{}
	counts := map[string]int{}
	var refs []string
	types := map[string]*ir.PartType{}
	fps := map[string]bool{}
	for _, pl := range f.placed {
		attrs := map[string]string{}
		for k, v := range pl.lp.propsOrNil() {
			attrs[k] = v
		}
		for k, v := range pl.part.props {
			attrs[k] = v
		}
		if pl.part.value != "" {
			attrs["Value"] = pl.part.value
		}
		delete(attrs, "Part Reference")
		comp, ok := byRef[pl.ref]
		if !ok {
			comp = &ir.Component{RefDes: pl.ref, Attributes: map[string]string{}, Prov: prov(pl.src)}
			byRef[pl.ref] = comp
			refs = append(refs, pl.ref)
		}
		counts[pl.ref+"|"+pl.part.pkg+"|"+fmt.Sprint(pl.part.unit)]++
		fp := attrs["PCB Footprint"]
		if fp == "" && pl.pkg != nil {
			fp = pl.pkg.footprint
		}
		if comp.FootprintRef == "" && fp != "" {
			comp.FootprintRef = fp
			if !fps[fp] {
				fps[fp] = true
				d.Footprints = append(d.Footprints, &ir.Footprint{Name: fp, Prov: prov(pl.src)})
			}
		}
		for k, v := range attrs {
			if _, set := comp.Attributes[k]; !set && v != "" {
				comp.Attributes[k] = v
			}
		}
		comp.Sections = append(comp.Sections, &ir.ComponentSection{
			Index: int32(pl.part.unit), PartRef: pl.part.pkg, LibraryRef: "orcad", Attributes: attrs, // #nosec G115 -- a u16 unit index
			Prov: &ir.Provenance{SourceFile: pl.src, NativeId: fmt.Sprint(pl.part.db)},
		})
		if _, ok := types[pl.part.pkg]; !ok && pl.part.pkg != "" {
			types[pl.part.pkg] = partType(pl)
		}
	}
	lib := &ir.PartLibrary{Name: "orcad"}
	var tnames []string
	for n := range types {
		tnames = append(tnames, n)
	}
	sort.Strings(tnames)
	for _, n := range tnames {
		lib.Parts = append(lib.Parts, types[n])
	}
	d.Libraries = []*ir.PartLibrary{lib}
	for _, r := range refs {
		d.Components = append(d.Components, byRef[r])
	}

	d.Nets = f.nets(top.name)

	// A designator placed twice as the same unit of the same package is two parts claiming one
	// name. Units of one part sharing a designator are sections, not a collision. The list is
	// declared even when empty, so duplicate-ref-des knows this reader looked (agni issue 309).
	diag := &ir.InputDiagnostics{Supplied: []string{"ref_des_collisions"}}
	collided := map[string]int{}
	for k, c := range counts {
		if c > 1 {
			ref := k[:strings.IndexByte(k, '|')]
			collided[ref] += c
		}
	}
	for _, r := range refs {
		if n := collided[r]; n > 1 {
			col := &ir.RefDesCollision{RefDes: r}
			for i := 0; i < n; i++ {
				col.Instances = append(col.Instances, prov(top.name))
			}
			diag.RefDesCollisions = append(diag.RefDesCollisions, col)
		}
	}
	diag.UnexpandedHierarchy = f.missing
	d.InputDiagnostics = diag
	return d
}

func (lp *libPart) propsOrNil() map[string]string {
	if lp == nil {
		return nil
	}
	return lp.props
}

// partType is a package's pins, numbered by its first device and named by its symbol.
func partType(pl placement) *ir.PartType {
	pt := &ir.PartType{Name: pl.part.pkg, Prov: &ir.Provenance{SourceFile: pl.src}}
	if pl.pkg == nil || len(pl.pkg.devices) == 0 {
		return pt
	}
	names := map[int]symPin{}
	if pl.lp != nil {
		for _, sp := range pl.lp.pins {
			names[sp.slot] = sp
		}
	}
	seen := map[string]bool{}
	for _, dev := range pl.pkg.devices {
		for slot, num := range dev.pins {
			if num == "" || seen[num] {
				continue
			}
			seen[num] = true
			sp := names[slot]
			pt.Pins = append(pt.Pins, &ir.Pin{Name: sp.name, Designator: num, Direction: direction(sp.portType)})
		}
	}
	return pt
}

func direction(t uint32) ir.PinDirection {
	switch t {
	case 0:
		return ir.PinDirection_PIN_DIRECTION_INPUT
	case 1:
		return ir.PinDirection_PIN_DIRECTION_INOUT
	case 2:
		return ir.PinDirection_PIN_DIRECTION_OUTPUT
	case 3:
		return ir.PinDirection_PIN_DIRECTION_OPEN_COLLECTOR
	case 4:
		return ir.PinDirection_PIN_DIRECTION_PASSIVE
	case 5:
		return ir.PinDirection_PIN_DIRECTION_TRISTATE
	case 6:
		return ir.PinDirection_PIN_DIRECTION_OPEN_EMITTER
	case 7:
		return ir.PinDirection_PIN_DIRECTION_POWER_IN
	}
	return ir.PinDirection_PIN_DIRECTION_UNSPECIFIED
}

// nets groups the pins by their set and names each net.
func (f *flattener) nets(src string) []*ir.Net {
	members := map[string][]string{}
	for node := range f.pins {
		root := f.sets.find(node)
		members[root] = append(members[root], node)
	}
	names := map[string][]netName{}
	gen := map[string]netName{}
	for node, ns := range f.names {
		root := f.sets.find(node)
		names[root] = append(names[root], ns...)
	}
	for node, g := range f.genName {
		root := f.sets.find(node)
		if cur, ok := gen[root]; !ok || better(g, cur) {
			gen[root] = g
		}
	}
	// A power name wins, then the shallowest name, so a net a block labels CHG_IN takes the name the
	// root gives it, generated or not, as Capture's netlist does (Clay's N08855). At one depth a
	// label beats a generated name, and the lowest number or the first by spelling breaks a tie, so a
	// read names each net the same way every time.
	chosen := map[string]netName{}
	for root := range members {
		best, ok := gen[root]
		for _, n := range names[root] {
			if !ok || better(n, best) {
				best, ok = n, true
			}
		}
		if best.name == "" {
			continue // a pin that touches nothing is unconnected, not a net of its own
		}
		chosen[root] = best
	}
	uses := map[string]int{}
	for _, n := range chosen {
		uses[n.name]++
	}
	var out []*ir.Net
	for root, ps := range members {
		n, ok := chosen[root]
		if !ok {
			continue
		}
		name := n.name
		if !n.global && uses[name] > 1 && n.path != "" {
			name = n.path + "/" + name
		}
		f.netOf[root] = name
		sort.Strings(ps)
		net := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: src}}
		for _, p := range ps {
			rp := f.pins[p]
			net.Connections = append(net.Connections, &ir.Connection{ComponentRef: rp[0], PinRef: rp[1], Prov: &ir.Provenance{SourceFile: src}})
		}
		out = append(out, net)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetName() < out[j].GetName() })
	return out
}

func better(a, b netName) bool {
	if a.global != b.global {
		return a.global
	}
	if a.depth != b.depth {
		return a.depth < b.depth
	}
	if a.isGen != b.isGen {
		return !a.isGen
	}
	if a.gen != b.gen {
		return a.gen < b.gen
	}
	return a.name < b.name
}
