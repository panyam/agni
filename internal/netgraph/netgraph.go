// Package netgraph assembles a pin-level netlist from schematic geometry. Both the xschem and
// gEDA readers reduce their format to the same neutral inputs on an integer grid. The inputs are
// wire segments, each optionally with a net name, named net anchors such as power taps and port or
// label markers, and component pin placements. netgraph unions everything that shares a grid
// point, names each resulting net, and reports which component pins sit on it.
//
// It is format-agnostic and coordinate-agnostic. The caller quantizes its native units to an
// integer grid so that endpoints meant to touch compare equal. Two connection points at the
// same grid point are the same node; a wire additionally unions its two endpoints.
package netgraph

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
)

// Point is a connection location on the caller's integer grid. The caller is responsible for
// quantizing native coordinates (e.g. xschem half-integers scaled by 2) so that points meant
// to coincide are exactly equal.
type Point struct{ X, Y int64 }

// Wire is a net segment between two grid points. Label is the net name the segment carries
// inline (xschem "lab="), or "" when the format leaves wires unnamed (gEDA). Id is the source's
// native id for the wire (e.g. a KiCad uuid), carried through so a dangling-endpoint diagnostic can
// point back at the offending wire; "" when the format has no per-wire id.
type Wire struct {
	A, B  Point
	Label string
	Id    string
	// Sheet is the sheet instance the wire is drawn on, set by a reader that solves several sheets
	// at once (the KiCad hierarchy walk) and empty otherwise. See Net.SheetGroups.
	Sheet string
}

// Anchor names the net passing through a grid point, such as a power/ground tap or a port/label
// marker. It contributes a name but no pin. Driver marks an anchor that asserts the net is driven
// without being a pin, such as a KiCad PWR_FLAG. It sets Net.Driven, so a rail fed only by a flag
// is not read as undriven. External marks an anchor that continues the net onto another sheet,
// such as a global or hierarchical label or a global power rail. It sets Net.External.
type Anchor struct {
	At       Point
	Label    string
	Driver   bool
	External bool
	// Rank orders naming when a net carries several labels. The lowest-ranked label names the
	// net, ties keep input order, and a wire label beats every anchor. Single-sheet callers
	// leave it zero. The KiCad hierarchy walk ranks a bare global/power name (0) over an
	// instance-qualified local name (1) over a synthetic hier-join label (2), which exists only
	// to union a parent sheet pin with its child hierarchical label.
	Rank int
	// Sheet is the sheet instance the anchor sits on, as for Wire.Sheet.
	Sheet string
}

// Pin is one component pin placed at a grid point. Comp is the component ref_des and Pin is
// the pin designator on that component. NoConnect marks a pin whose connect point carries
// the format's no-connect flag (a KiCad no_connect marker placed ON the pin, WS1-019). The
// caller detects the coincidence. Build then names that pin's LONE stub net in the tool-marker
// vocabulary ("unconnected-(REF-Pad)") instead of "N$<n>". The existing no-connect consumers
// recognise that name with no new IR field. They are intentionallyUnconnected and the NC channel.
// A marked pin on a multi-pin net is named normally, because that net is an nc-pin-connected
// problem rather than an intentional stub.
type Pin struct {
	At        Point
	Comp, Pin string
	NoConnect bool
	// Dir is the pin's electrical direction in the check vocabulary ("power_in",
	// "power_out", ...). Only a pin on a VIRTUAL component sets it, such as a KiCad power
	// symbol (WS1-014). A virtual component never enters Components, so no part-type pin
	// exists to resolve the direction from. Dir is empty for ordinary pins, whose direction
	// resolves via the part type.
	Dir string
	// AutoName is the format tool's own name for an unlabelled net named after this pin, such
	// as KiCad's "Net-(R1-Pad2)". NCName is the tool's name for the lone stub of a
	// no-connect-flagged pin, such as "unconnected-(J1A-MID0-PadK40)". A format that sets
	// neither gets the synthetic "N$<n>" and "unconnected-(REF-PadN)" names.
	AutoName, NCName string
}

// Options adjusts how Build names nets. The zero value is the shared behaviour every format
// without its own naming rules uses.
type Options struct {
	// AlphaTies breaks a tie between labels of equal rank by name rather than input order, and
	// picks a pin-derived AutoName the same way, preferring a name that does not end in a pad
	// number. KiCad names nets this way, so a read that follows it agrees with KiCad's netlist.
	AlphaTies bool
}

// Conn is a pin's membership in a net.
type Conn struct{ Comp, Pin, Dir string }

// Net is one assembled net, a name and the pins on it, with Conns sorted. Driven is true when a
// driver anchor such as a PWR_FLAG sits on the net. External is true when the net continues onto
// another sheet. An absence-based rule, such as "nothing drives this" or "only inputs here", then
// knows its view is incomplete and can decline to fire. Aliases lists EVERY distinct label on the
// net, Name included, with its rank, in first-appearance order. The naming pass collapses them to
// one name, and naming-conflict rules read Aliases to see what it collapsed, such as taps spelled
// "+3V3" and "+3.3V" on one net. Aliases is empty for unnamed nets.
type Net struct {
	Name string
	// ID is a hash of the sorted connection set (see netID), independent of Name, so two
	// distinct nets sharing a Name get distinct IDs. It is EPHEMERAL, recomputed every solve
	// and stable only while connectivity is unchanged, so it is a within-load join key and never
	// a durable handle for saved work. Empty for a pinless net; consumers fall back to Name.
	ID       string
	Conns    []Conn
	Driven   bool
	External bool
	// NoConnect is true when a pin on the net carries the format's no-connect flag. The flag is
	// the designer saying the open pin is deliberate. A net's name never says so, because KiCad
	// names every unlabelled one-pin net "unconnected-(...)", flagged or not (agni issue 1019).
	NoConnect bool
	Aliases   []Alias
	// SheetGroups splits Aliases by where the names met. Each group holds the names that share one
	// sheet's wiring, with that sheet. A wire joins them there, or one name drawn twice on that
	// sheet does, before any other sheet joins them. SheetGroups is set only when the net's names
	// fall into two groups or more, and only a caller that sets Sheet can produce two. Two labels
	// on one wire form one group. A parent sheet can strap two hierarchical labels together, and
	// those form two groups (agni issue 1249). Nil means every alias met in one place. A caller
	// that sets no Sheet always gets nil.
	SheetGroups []SheetGroup
}

// NetRef identifies a net by its display Name and its deterministic ID (see Net.ID). The wire and
// point maps give each wire or point the NetRef of the net it resolved to. The geometry sidecar
// can then draw a wire by name AND join a highlight to the specific net instance by id, even when
// two nets share a name.
type NetRef struct {
	Name string
	ID   string
}

// netID hashes a net's connection set into a short stable id. Direction is excluded because it is
// a hint rather than identity. A net with no connections hashes to "", and consumers fall back to
// Name.
func netID(conns []Conn) string {
	pairs := make([][2]string, len(conns))
	for i, c := range conns {
		pairs[i] = [2]string{c.Comp, c.Pin}
	}
	return hashPairs(pairs)
}

// hashPairs is the canonical net-id hash over (component, pin) pairs. It sorts internally because
// the solver hands sorted conns and StampNetIDs hands a direct-IR reader's declaration order, and
// both must give the same id for the same connectivity. Empty in, empty out.
func hashPairs(pairs [][2]string) string {
	if len(pairs) == 0 {
		return ""
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i][0] != pairs[j][0] {
			return pairs[i][0] < pairs[j][0]
		}
		return pairs[i][1] < pairs[j][1]
	})
	h := sha256.New()
	for _, p := range pairs {
		h.Write([]byte(p[0]))
		h.Write([]byte{0})
		h.Write([]byte(p[1]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// Alias is one label a net carries, with the Anchor.Rank it arrived with (wire
// labels use the wire rank, below every anchor).
type Alias struct {
	Name string
	Rank int
}

// SheetGroup is the names of one net that met on one sheet's wiring (Net.SheetGroups). A name can
// sit in a group on a sheet other than its own. A parent sheet's pin carries its child's
// hierarchical name, so when the parent wires two child ports together, their names meet on the
// parent.
type SheetGroup struct {
	Sheet string
	Names []string
}

// Dangle is a wire endpoint that terminates on nothing (no pin, anchor/label, junction, or other
// wire endpoint shares its grid point). At is the endpoint and WireId the Wire.Id it belongs to, ""
// when unknown. A wire end can sit mid-span on another wire without a junction, as a T-tap missing
// its dot does. Dangle does not report that case, which is WS1-012 and needs point-on-segment
// geometry.
type Dangle struct {
	At     Point
	WireId string
}

// Build assembles nets from schematic geometry in two stages. First it unions connection
// points by grid coincidence, and a wire also unions its two endpoints. That gives geometric
// nodes. Then it unions nodes by EVERY label they carry. Schematic net labels and power taps
// connect by name across the whole sheet, not only where wires physically touch, so every "0"
// ground tap is one net regardless of geometry. A node carrying two labels aliases them into one
// net. The KiCad hierarchy walk uses this to stitch a parent sheet pin to its child hierarchical
// label.
// The merged net's name is its lowest-ranked label (wire labels first, then Anchor.Rank,
// ties by input order). Unnamed geometric nodes are kept separate and get a synthetic "N$<n>"
// name numbered by first appearance; an unnamed, pinless node is dropped as drawing noise.
// Conns within a net are deduplicated and sorted.
//
// It also returns the dangling endpoints (see Dangle), each a wire endpoint whose grid point holds
// nothing else (no pin, anchor, terminal, or second wire endpoint). A format marks a junction dot
// or a no-connect flag as a legitimate wire end and passes those points as terminals. Formats
// with neither pass nil. Dangles are sorted.
//
// The third return maps each identified wire, keyed by Wire.Id, to the net of its endpoints. The
// geometry sidecar uses the map to draw a KiCad wire by net (WS1-022). Only wires with a native
// id populate it, and a wire whose node is drawing noise is omitted.
func Build(wires []Wire, anchors []Anchor, pins []Pin, terminals []Point, opts ...Options) ([]Net, []Dangle, map[string]NetRef) {
	nets, dangles, wireNets, _ := BuildWithPoints(wires, anchors, pins, terminals, opts...)
	return nets, dangles, wireNets
}

// BuildWithPoints is Build plus a point->net-name map covering every input point (both wire
// endpoints, each anchor, each pin). The KiCad hierarchy walk offsets each sheet instance into
// its own grid band (WS9-028) and uses this map to attribute each net to the sheets it touches.
// A point on drawing noise (unnamed, pinless) is omitted, as in the wire map.
func BuildWithPoints(wires []Wire, anchors []Anchor, pins []Pin, terminals []Point, opts ...Options) ([]Net, []Dangle, map[string]NetRef, map[Point]NetRef) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}
	uf := newUnionFind()
	for _, w := range wires {
		uf.union(w.A, w.B)
	}
	// Anchors and pins not on any wire still need a node.
	for _, a := range anchors {
		uf.add(a.At)
	}
	for _, p := range pins {
		uf.add(p.At)
	}

	// Union by label BEFORE resolving roots for naming. Every point carrying label L joins
	// L's first point, so aliased labels (one node, two names) fold their clusters together.
	labelAt := map[string]Point{}
	labelUnion := func(label string, at Point) {
		if label == "" {
			return
		}
		if first, ok := labelAt[label]; ok {
			uf.union(first, at)
		} else {
			labelAt[label] = at
		}
	}
	for _, w := range wires {
		labelUnion(w.Label, w.A)
	}
	for _, a := range anchors {
		labelUnion(a.Label, a.At)
	}

	// Assign each root its lowest-ranked label. Wire labels rank below every anchor, anchors
	// compare by Rank, ties keep input order.
	type candidate struct {
		rank int
		name string
	}
	name := map[int]candidate{}
	var rootOrder []int
	seenRoot := func(r int) {
		if _, ok := name[r]; !ok {
			name[r] = candidate{}
			rootOrder = append(rootOrder, r)
		}
	}
	aliases := map[int][]Alias{}
	consider := func(r, rank int, label string) {
		if label == "" {
			return
		}
		if c := name[r]; c.name == "" || rank < c.rank || (opt.AlphaTies && rank == c.rank && label < c.name) {
			name[r] = candidate{rank: rank, name: label}
		}
		for _, a := range aliases[r] {
			if a.Name == label {
				return
			}
		}
		aliases[r] = append(aliases[r], Alias{Name: label, Rank: rank})
	}
	const wireRank = -1
	for _, w := range wires {
		r := uf.find(uf.id[w.A])
		seenRoot(r)
		consider(r, wireRank, w.Label)
	}
	driven := map[int]bool{}
	external := map[int]bool{}
	for _, a := range anchors {
		r := uf.find(uf.id[a.At])
		seenRoot(r)
		consider(r, a.Rank, a.Label)
		if a.Driver {
			driven[r] = true
		}
		if a.External {
			external[r] = true
		}
	}
	for _, p := range pins {
		r := uf.find(uf.id[p.At])
		seenRoot(r)
	}

	conns := map[int][]Conn{}
	ncName := map[int]string{}      // root -> no-connect stub name, when a marked pin sits there
	autoName := map[int]string{}    // root -> the format's own name for an unlabelled net, if it has one
	autoPins := map[int]int{}       // root -> how many pins carry an AutoName
	loneNC := map[int]string{}      // root -> the NCName of its last AutoName pin, used when it is the only one
	flaggedName := map[int]string{} // root -> the best NCName among its pins, used when a pin is flagged no-connect
	flagged := map[int]bool{}
	for _, p := range pins {
		r := uf.find(uf.id[p.At])
		conns[r] = append(conns[r], Conn{Comp: p.Comp, Pin: p.Pin, Dir: p.Dir})
		if p.NoConnect {
			if p.NCName != "" {
				ncName[r] = p.NCName
			} else {
				ncName[r] = "unconnected-(" + p.Comp + "-Pad" + p.Pin + ")"
			}
		}
		if p.NoConnect {
			flagged[r] = true
		}
		if p.NCName != "" {
			if cur, ok := flaggedName[r]; !ok || betterAutoName(p.NCName, cur) {
				flaggedName[r] = p.NCName
			}
		}
		if p.AutoName != "" {
			autoPins[r]++
			loneNC[r] = p.NCName
			if cur, ok := autoName[r]; !ok || betterAutoName(p.AutoName, cur) {
				autoName[r] = p.AutoName
			}
		}
	}

	groups := sheetGroups(uf, wires, anchors)

	var nets []Net
	rootName := map[int]string{} // root -> final net name (incl. N$), for the wire->net map
	rootID := map[int]string{}   // root -> deterministic net id, for the wire/point->net-instance join
	stubN := 0
	for _, r := range rootOrder {
		nm := name[r].name
		cs := conns[r]
		if nm == "" && len(cs) == 1 {
			// A lone no-connect-marked pin gets the tool-marker name, not N$. The stub is
			// intentional, and every no-connect-aware consumer keys on this vocabulary.
			nm = ncName[r]
		}
		if nm == "" && autoName[r] != "" {
			// When only one pin can name an unlabelled net, the net is a stub whatever its flags
			// say. It takes that pin's unconnected name, as KiCad does for a weakly driven
			// single-pin net.
			// A no-connect flag on the net does the same for every pin, as KiCad's forced no-connect
			// naming does.
			switch {
			case flagged[r] && flaggedName[r] != "":
				nm = flaggedName[r]
			case autoPins[r] == 1 && loneNC[r] != "":
				nm = loneNC[r]
			default:
				nm = autoName[r]
			}
		}
		if nm == "" {
			if len(cs) == 0 {
				continue // unnamed and pinless, so drawing noise rather than a net
			}
			stubN++
			nm = "N$" + strconv.Itoa(stubN)
		}
		sorted := dedupSort(cs)
		id := netID(sorted)
		rootName[r] = nm
		rootID[r] = id
		nets = append(nets, Net{Name: nm, ID: id, Conns: sorted, Driven: driven[r], External: external[r], NoConnect: flagged[r], Aliases: aliases[r], SheetGroups: groups[r]})
	}

	// Only formats with native wire ids (KiCad uuids) populate wireNets. Formats whose wires
	// carry their net name inline (xschem lab=, gEDA) fill WireGeometry.net directly and pass
	// empty ids.
	wireNets := map[string]NetRef{}
	for _, w := range wires {
		if w.Id == "" {
			continue
		}
		r := uf.find(uf.id[w.A])
		if nm, ok := rootName[r]; ok {
			wireNets[w.Id] = NetRef{Name: nm, ID: rootID[r]}
		}
	}

	pointNets := map[Point]NetRef{}
	addPoint := func(p Point) {
		r := uf.find(uf.id[p])
		if nm, ok := rootName[r]; ok {
			pointNets[p] = NetRef{Name: nm, ID: rootID[r]}
		}
	}
	for _, w := range wires {
		addPoint(w.A)
		addPoint(w.B)
	}
	for _, a := range anchors {
		addPoint(a.At)
	}
	for _, p := range pins {
		addPoint(p.At)
	}
	return nets, dangling(wires, anchors, pins, terminals), wireNets, pointNets
}

// betterAutoName reports whether a pin-derived net name should replace the current pick. A name
// with no pad number beats one with, and otherwise the alphabetically lower wins. KiCad uses this
// order for a net named after one of its pins.
func betterAutoName(a, cur string) bool {
	aPad, curPad := strings.Contains(a, "-Pad"), strings.Contains(cur, "-Pad")
	if aPad != curPad {
		return !aPad
	}
	return a < cur
}

// dangling finds the wire endpoints that terminate on nothing. A grid point is "occupied" if a
// pin, an anchor/label, or a terminal (junction dot or no-connect flag) sits there; an endpoint is
// dangling when it is the sole wire endpoint at its point (degree 1) and that point is unoccupied.
// Two wires meeting end-to-end (degree 2) or a polyline's interior vertex are therefore connected.
// Results are sorted by point.
func dangling(wires []Wire, anchors []Anchor, pins []Pin, terminals []Point) []Dangle {
	degree := map[Point]int{}
	for _, w := range wires {
		degree[w.A]++
		degree[w.B]++
	}
	occupied := map[Point]bool{}
	for _, a := range anchors {
		occupied[a.At] = true
	}
	for _, p := range pins {
		occupied[p.At] = true
	}
	for _, tm := range terminals {
		occupied[tm] = true
	}
	var out []Dangle
	seen := map[Point]bool{}
	for _, w := range wires {
		for _, e := range [2]Point{w.A, w.B} {
			if degree[e] == 1 && !occupied[e] && !seen[e] {
				seen[e] = true
				out = append(out, Dangle{At: e, WireId: w.Id})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].At.X != out[j].At.X {
			return out[i].At.X < out[j].At.X
		}
		return out[i].At.Y < out[j].At.Y
	})
	return out
}

// dedupSort removes duplicate conns (Dir included in the key) and sorts the rest by (Comp, Pin).
func dedupSort(cs []Conn) []Conn {
	seen := map[Conn]bool{}
	out := cs[:0:0]
	for _, c := range cs {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Comp != out[j].Comp {
			return out[i].Comp < out[j].Comp
		}
		return out[i].Pin < out[j].Pin
	})
	return out
}

// unionFind maps grid points to dense node ids and unions them.
type unionFind struct {
	id     map[Point]int
	parent []int
}

// sheetGroups groups each net's names by where they met, for Net.SheetGroups. It unions points as
// the solve does, by wire and by one name drawn twice, except that a name joins only within one
// sheet. So the groups of one net are the pieces it was in before other sheets joined them. For
// each root of uf whose net's names fall into two groups or more, it returns the sorted groups.
func sheetGroups(uf *unionFind, wires []Wire, anchors []Anchor) map[int][]SheetGroup {
	local := newUnionFind()
	for _, w := range wires {
		local.union(w.A, w.B)
	}
	type sheetLabel struct{ sheet, label string }
	first := map[sheetLabel]Point{}
	join := func(sheet, label string, at Point) {
		local.add(at)
		if label == "" {
			return
		}
		k := sheetLabel{sheet, label}
		if p, ok := first[k]; ok {
			local.union(p, at)
		} else {
			first[k] = at
		}
	}
	for _, w := range wires {
		join(w.Sheet, w.Label, w.A)
	}
	for _, a := range anchors {
		join(a.Sheet, a.Label, a.At)
	}
	names := map[int]map[int]map[string]bool{} // net root -> sheet-local root -> names
	sheetOf := map[int]string{}                // sheet-local root -> its sheet
	for k, at := range first {
		r := uf.find(uf.id[at])
		lr := local.find(local.id[at])
		if names[r] == nil {
			names[r] = map[int]map[string]bool{}
		}
		if names[r][lr] == nil {
			names[r][lr] = map[string]bool{}
		}
		names[r][lr][k.label] = true
		sheetOf[lr] = k.sheet
	}
	out := map[int][]SheetGroup{}
	for r, byLocal := range names {
		if len(byLocal) < 2 {
			continue
		}
		var gs []SheetGroup
		for lr, set := range byLocal {
			g := SheetGroup{Sheet: sheetOf[lr]}
			for n := range set {
				g.Names = append(g.Names, n)
			}
			sort.Strings(g.Names)
			gs = append(gs, g)
		}
		key := func(g SheetGroup) string { return g.Sheet + "\x00" + strings.Join(g.Names, "\x00") }
		sort.Slice(gs, func(i, j int) bool { return key(gs[i]) < key(gs[j]) })
		out[r] = gs
	}
	return out
}

func newUnionFind() *unionFind { return &unionFind{id: map[Point]int{}} }

func (u *unionFind) add(p Point) int {
	if i, ok := u.id[p]; ok {
		return i
	}
	i := len(u.parent)
	u.id[p] = i
	u.parent = append(u.parent, i)
	return i
}

func (u *unionFind) find(i int) int {
	for u.parent[i] != i {
		u.parent[i] = u.parent[u.parent[i]]
		i = u.parent[i]
	}
	return i
}

func (u *unionFind) union(a, b Point) {
	ra, rb := u.find(u.add(a)), u.find(u.add(b))
	if ra != rb {
		u.parent[rb] = ra
	}
}
