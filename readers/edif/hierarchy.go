package edif

import (
	"strings"

	"github.com/panyam/agni/core/num"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// A hierarchical EDIF netlist places BLOCKS, cells whose own contents hold instances, beside its
// parts. Read with FlattenHierarchy, the reader walks from the design's top cell into every block it
// places, so the design is the whole schematic rather than its top sheet (agni issue 718). Without it
// only the top cell is read and the blocks are listed in InputDiagnostics.unexpanded_hierarchy.
//
// Three rules decide identity, each written in docsite/content/architecture/ingestion-and-ir.md:
//
//   - A net crossing a block's port is ONE net with its parent's, named by its shallowest piece. A net
//     inside a block that reaches no port keeps its name when no other net in the design carries it
//     and its block is placed once. Otherwise it is qualified by the block's instance path,
//     "/A1/MID", because sheets reuse local and generated names and joining them by name would merge
//     two nets.
//   - A part keeps the designator the file states, and parts sharing one are sections of one
//     component wherever they sit, since a multi-gate part split across sheets is one part.
//   - A part reached through a block the design places more than once is qualified by its instance
//     path, "B2/R1", so two placements are two sets of parts rather than one merged set.
//   - A file with no designators names each part by its instance name, which is unique within its
//     cell only. A name used in more than one cell instance is qualified by its block's path in every
//     block it appears in ("A1/R1"), the top cell keeping its own, so two cells' R1 are two parts. A
//     multi-gate part split across sheets then reads as two, which shows, where a wrong merge would
//     hide a part.

// leaf is one placed part, reached through path, the block instance names above it joined by "/",
// empty in the top cell.
type leaf struct {
	node *node
	path string
	// repeated is set when a block on the path is placed more than once in the design, so the part's
	// ref-des must carry its path to stay distinct.
	repeated bool
}

// netPiece is one (net) list in the block instance at path.
type netPiece struct {
	node *node
	path string
	// repeated is set when a block on the path is placed more than once, as for leaf.
	repeated bool
}

// hierarchy is what a read extracts from: the parts, the nets as groups of pieces that are one
// electrical net with the naming piece first, and the block instances a portRef can name.
type hierarchy struct {
	leaves []leaf
	nets   [][]netPiece
	// blockPaths maps a block instance, keyed as instKey keys it, to the path of its contents. A
	// portRef naming one is a port of that block rather than a pin.
	blockPaths map[string]string
	// blocks are the cells walked into, which are sheets rather than part types.
	blocks map[*node]bool
	// cycles are the blocks a walk refused to enter because they contain themselves.
	cycles []*node
	// flattened is set by the walk, so a read that descended reports only its cycles as unexpanded.
	flattened bool
}

// instKey keys an instance by the path of the cell instance it sits in, since instance ids repeat
// across cells and across two placements of one cell.
func instKey(path, id string) string { return path + "\x00" + id }

// occurrence is an instance's id as a locator: the id itself in the top cell, and its path within a
// block ("B1/R1"), since the bare id names a different part in each placement.
func occurrence(path, id string) string {
	if path == "" || id == "" {
		return id
	}
	return path + "/" + id
}

// topOnly is the top cell read alone, as every read was before agni issue 718.
func topOnly(scope *node) *hierarchy {
	h := &hierarchy{blockPaths: map[string]string{}, blocks: map[*node]bool{}}
	var insts, nets []*node
	collect(scope, "instance", &insts)
	collect(scope, "net", &nets)
	for _, in := range insts {
		h.leaves = append(h.leaves, leaf{node: in})
	}
	for _, n := range nets {
		h.nets = append(h.nets, []netPiece{{node: n}})
	}
	return h
}

// cellIndex resolves an instance's (cellRef C (libraryRef L)) to the cell it names.
type cellIndex struct {
	byLib map[string]map[string]*node
	byID  map[string][]*node
	libOf map[*node]string
	block map[*node]bool
}

// cellKeys are the spellings a reference may name an entity by, the identifier and the display,
// each without the & escape (see instanceOf).
func cellKeys(n *node) []string {
	nm := parseName(n.Arg(1))
	keys := []string{strings.TrimPrefix(nm.ID, "&")}
	if d := strings.TrimPrefix(nm.Display, "&"); d != "" && d != keys[0] {
		keys = append(keys, d)
	}
	return keys
}

func indexCells(root *node) *cellIndex {
	ix := &cellIndex{byLib: map[string]map[string]*node{}, byID: map[string][]*node{}, libOf: map[*node]string{}, block: map[*node]bool{}}
	var libs []*node
	collect(root, "library", &libs)
	for _, l := range libs {
		lkeys := cellKeys(l)
		for _, c := range l.Children("cell") {
			ix.libOf[c] = lkeys[0]
			var in []*node
			collect(c, "instance", &in)
			ix.block[c] = len(in) > 0
			for _, k := range cellKeys(c) {
				for _, lk := range lkeys {
					if ix.byLib[lk] == nil {
						ix.byLib[lk] = map[string]*node{}
					}
					if _, ok := ix.byLib[lk][k]; !ok {
						ix.byLib[lk][k] = c
					}
				}
				ix.byID[k] = append(ix.byID[k], c)
			}
		}
	}
	return ix
}

// resolve returns the cell an instance places, looking in the library its cellRef names or, with
// none named, the library of the cell holding the instance, as EDIF scopes it. A name no library
// declares under that scope resolves only when exactly one cell anywhere carries it.
func (ix *cellIndex) resolve(in, holder *node) *node {
	v := in.Child("viewRef")
	if v == nil {
		return nil
	}
	cr := v.Child("cellRef")
	if cr == nil {
		return nil
	}
	name := strings.TrimPrefix(atom(cr.Arg(1)), "&")
	lib := ix.libOf[holder]
	if lr := cr.Child("libraryRef"); lr != nil {
		lib = strings.TrimPrefix(atom(lr.Arg(1)), "&")
	}
	if c := ix.byLib[lib][name]; c != nil {
		return c
	}
	if cs := ix.byID[name]; len(cs) == 1 {
		return cs[0]
	}
	return nil
}

// flattenFrom walks the hierarchy under top. It counts each block's placements first, because a part
// is qualified by its path only when a block above it is placed more than once.
func flattenFrom(root, top *node) *hierarchy {
	ix := indexCells(root)
	placed := map[*node]int{}
	var count func(c *node, stack map[*node]bool)
	count = func(c *node, stack map[*node]bool) {
		stack[c] = true
		defer delete(stack, c)
		var insts []*node
		collect(c, "instance", &insts)
		for _, in := range insts {
			if b := ix.resolve(in, c); b != nil && ix.block[b] && !stack[b] {
				placed[b]++
				count(b, stack)
			}
		}
	}
	count(top, map[*node]bool{})

	h := &hierarchy{blockPaths: map[string]string{}, blocks: map[*node]bool{}, flattened: true}
	cycled := map[*node]bool{}
	var walk func(c *node, path string, repeated bool, stack map[*node]bool)
	walk = func(c *node, path string, repeated bool, stack map[*node]bool) {
		stack[c] = true
		defer delete(stack, c)
		type child struct {
			cell     *node
			path     string
			repeated bool
		}
		var children []child
		var insts []*node
		collect(c, "instance", &insts)
		for _, in := range insts {
			b := ix.resolve(in, c)
			if b == nil || !ix.block[b] {
				h.leaves = append(h.leaves, leaf{node: in, path: path, repeated: repeated})
				continue
			}
			if stack[b] {
				if !cycled[b] {
					cycled[b] = true
					h.cycles = append(h.cycles, b)
				}
				continue
			}
			nm := parseName(in.Arg(1))
			p := nm.best()
			if path != "" {
				p = path + "/" + p
			}
			h.blockPaths[instKey(path, nm.ID)] = p
			h.blocks[b] = true
			children = append(children, child{cell: b, path: p, repeated: repeated || placed[b] > 1})
		}
		var nets []*node
		collect(c, "net", &nets)
		for _, n := range nets {
			h.nets = append(h.nets, []netPiece{{node: n, path: path, repeated: repeated}})
		}
		for _, ch := range children {
			walk(ch.cell, ch.path, ch.repeated, stack)
		}
	}
	walk(top, "", false, map[*node]bool{})
	h.joinAcrossPorts()
	return h
}

// joinAcrossPorts merges the walk's single-piece nets into electrical nets. A block's net names its
// own port as a portRef with no instanceRef, and the parent's net names the same port with an
// instanceRef to the block, so both tie to one key, the block's path and the port.
func (h *hierarchy) joinAcrossPorts() {
	n := len(h.nets)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	owner := map[string]int{}
	tie := func(key string, i int) {
		if j, ok := owner[key]; ok {
			if a, b := find(i), find(j); a != b {
				// The lower index stays the root, so a group is ordered by its first piece.
				if a > b {
					a, b = b, a
				}
				parent[b] = a
			}
			return
		}
		owner[key] = i
	}
	for i, g := range h.nets {
		piece := g[0]
		var prs []*node
		collect(piece.node, "portRef", &prs)
		for _, pr := range prs {
			port := portName(pr.Arg(1))
			inst := ""
			if ins := pr.Child("instanceRef"); ins != nil {
				inst = atom(ins.Arg(1))
			}
			switch {
			case inst == "" && piece.path != "":
				tie(piece.path+"\x00"+port, i)
			case inst != "":
				if p, ok := h.blockPaths[instKey(piece.path, inst)]; ok {
					tie(p+"\x00"+port, i)
				}
			}
		}
	}
	groups := map[int]int{}
	var out [][]netPiece
	for i, g := range h.nets {
		r := find(i)
		gi, ok := groups[r]
		if !ok {
			gi = len(out)
			groups[r] = gi
			out = append(out, nil)
		}
		out[gi] = append(out[gi], g[0])
	}
	for _, g := range out {
		best := 0
		for k, p := range g {
			if depth(p.path) < depth(g[best].path) {
				best = k
			}
		}
		named := g[best]
		copy(g[1:best+1], g[:best])
		g[0] = named
	}
	h.nets = out
}

// netNames names each net group: its naming piece's own name, qualified by the piece's block path
// unless the piece is in the top cell, or its name is carried by no other group and its block is
// placed once.
func (h *hierarchy) netNames() []string {
	seen := map[string]int{}
	for _, g := range h.nets {
		seen[parseName(g[0].node.Arg(1)).best()]++
	}
	out := make([]string, len(h.nets))
	for i, g := range h.nets {
		name := parseName(g[0].node.Arg(1)).best()
		if p := g[0]; p.path != "" && (p.repeated || seen[name] > 1) {
			name = "/" + p.path + "/" + name
		}
		out[i] = name
	}
	return out
}

// namesInSeveralBlocks is every instance name the walk met in more than one cell instance, which a
// file naming its parts by instance name (no designators) cannot tell apart by name alone.
func (h *hierarchy) namesInSeveralBlocks() map[string]bool {
	at := map[string]string{}
	out := map[string]bool{}
	for _, lf := range h.leaves {
		n := parseName(lf.node.Arg(1)).best()
		if p, seen := at[n]; !seen {
			at[n] = lf.path
		} else if p != lf.path {
			out[n] = true
		}
	}
	return out
}

// depth is how many blocks deep a path is, 0 for the top cell.
func depth(path string) int {
	if path == "" {
		return 0
	}
	return strings.Count(path, "/") + 1
}

// unexpandedEntry describes a cell whose contents a read left out, and why.
func unexpandedEntry(c *node, src string, reason ir.UnexpandedReason) *ir.UnexpandedHierarchy {
	var ci []*node
	collect(c, "instance", &ci)
	nm := parseName(c.Arg(1))
	return &ir.UnexpandedHierarchy{
		Name:          nm.best(),
		Kind:          "edif_cell",
		InstanceCount: num.Int32(len(ci)),
		Prov:          &ir.Provenance{SourceFile: src, NativeId: nm.ID, NativeIdKind: edifNativeIDKind},
		Reason:        reason,
	}
}
