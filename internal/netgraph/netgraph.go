// Package netgraph assembles a pin-level netlist from schematic geometry. Both the xschem and
// gEDA readers reduce their format to the same neutral inputs -- wire segments (optionally
// carrying a net name), named net anchors (power taps, port/label markers), and component pin
// placements -- expressed as integer grid points. netgraph unions everything that shares a
// grid point, names each resulting net, and reports which component pins land on it.
//
// It is format-agnostic and coordinate-agnostic: the caller quantizes its native units to an
// integer grid so that endpoints meant to touch compare equal. Two connection points at the
// same grid point are the same node; a wire additionally unions its two endpoints.
package netgraph

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
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
}

// Anchor names the net passing through a grid point: a power/ground tap or a port/label
// marker. It contributes a name but no pin. Driver marks an anchor that asserts the net is driven
// without being a pin — a KiCad PWR_FLAG — so a power rail fed only by a flag is not mis-read as
// undriven; it sets Net.Driven on whichever net the anchor lands in. External marks an anchor that
// continues the net onto another sheet (a global/hierarchical label or a global power rail), so a
// consumer can tell that the net's full membership is not visible in this single-sheet read; it
// sets Net.External.
type Anchor struct {
	At       Point
	Label    string
	Driver   bool
	External bool
	// Rank orders naming when a net carries several labels: the lowest-ranked label names
	// the net (ties keep input order). Every single-sheet caller leaves it zero; the KiCad
	// hierarchy walk uses it so a bare global/power name (0) beats an instance-qualified
	// local name (1), which beats a synthetic hier-join label (2) that exists only to union
	// a parent sheet pin with its child hierarchical label. An explicit wire label still
	// beats every anchor, as before.
	Rank int
}

// Pin is one component pin placed at a grid point. Comp is the component ref_des and Pin is
// the pin designator on that component. NoConnect marks a pin whose connect point carries
// the format's no-connect flag (a KiCad no_connect marker placed ON the pin, WS1-019): the
// caller detects the coincidence, and Build names that pin's LONE stub net in the
// tool-marker vocabulary ("unconnected-(REF-Pad)") instead of "N$<n>", so downstream
// no-connect awareness (intentionallyUnconnected, the NC channel) lights up with no new IR
// surface. A marked pin that ends up on a multi-pin net keeps normal naming — wiring an
// NC-flagged pin into a real net is the nc-pin-connected class of problem, not an
// intentional stub.
type Pin struct {
	At        Point
	Comp, Pin string
	NoConnect bool
	// Dir is the pin's electrical direction in the check vocabulary ("power_in",
	// "power_out", ...), set by callers whose pin belongs to a VIRTUAL component (a KiCad
	// power symbol, WS1-014) that never enters Components — the direction must travel on
	// the connection because no part-type pin exists to resolve it from. Empty for
	// ordinary pins (every existing caller), whose direction resolves via the part type.
	Dir string
}

// Conn is a pin's membership in a net.
type Conn struct{ Comp, Pin, Dir string }

// Net is one assembled net: a name and the pins on it. Conns are sorted for determinism. Driven is
// true when a driver anchor (a PWR_FLAG) landed on the net, so a consumer can tell a rail asserted
// as fed from one that is genuinely unconnected. External is true when the net continues onto
// another sheet (a global/hierarchical label or a global power rail), so an absence-based rule
// (nothing drives this, only inputs here) knows its view is incomplete and can decline to fire.
// Aliases lists EVERY distinct label that landed on the net (the chosen Name included), with each
// label's rank, in first-appearance order: the naming pass collapses them to one Name, and
// naming-conflict rules need to see what was collapsed (two rail taps spelled "+3V3" and "+3.3V"
// are one net with one name and a real capture hazard). Empty for unnamed nets.
type Net struct {
	Name string
	// ID is a deterministic identity for the net, independent of its Name: a hash of the sorted
	// connection set (see netID). Two electrically-distinct nets that happen to share a Name get
	// distinct IDs, so a consumer can tell them apart where the name alone collides (the
	// duplicate-net-name case). It is EPHEMERAL — recomputed every solve, never persisted — and
	// stable only while connectivity is unchanged, so it is a within-load join key, not a durable
	// handle for pinning saved work to a net. Empty for a pinless net (nothing to hash, and nothing
	// to highlight either), so consumers fall back to Name.
	ID       string
	Conns    []Conn
	Driven   bool
	External bool
	Aliases  []Alias
}

// NetRef identifies the net a wire or point resolved to: the display Name and the deterministic
// ID (see Net.ID). The wire/point maps carry both so the geometry sidecar can draw a wire by name
// AND join a highlight to the specific net instance by id, even when two nets share a name.
type NetRef struct {
	Name string
	ID   string
}

// netID hashes a net's connection set into a short stable id. Direction is excluded: it is a hint,
// not identity. A net with no connections hashes to "" — it cannot be highlighted (no wires/pins),
// so it falls back to Name.
func netID(conns []Conn) string {
	pairs := make([][2]string, len(conns))
	for i, c := range conns {
		pairs[i] = [2]string{c.Comp, c.Pin}
	}
	return hashPairs(pairs)
}

// hashPairs is the canonical net-id hash over (component, pin) pairs. It sorts internally, so the id
// is independent of the caller's ordering — the solver hands sorted conns, a direct-IR reader (EDIF)
// hands declaration order, and both must land on the same id for the same connectivity. Empty in,
// empty out.
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

// Alias is one label a net carries: the name and the Anchor.Rank it arrived with (wire
// labels use the wire rank, below every anchor).
type Alias struct {
	Name string
	Rank int
}

// Dangle is a wire endpoint that terminates on nothing — no pin, anchor/label, junction, or other
// wire endpoint shares its grid point — so a connection the author drew is incomplete. At is the
// endpoint; WireId is the source id of the wire it belongs to (from Wire.Id), "" when unknown. It is
// the endpoint-on-nothing case only; a wire end that lands mid-span on another wire's body without a
// junction (a T-tap missing its dot) is a separate diagnostic (WS1-012) that needs point-on-segment
// geometry this endpoint-coincidence pass does not do.
type Dangle struct {
	At     Point
	WireId string
}

// Build assembles nets from schematic geometry in two stages. First it unions connection
// points by grid coincidence (a wire also unions its two endpoints), giving geometric nodes.
// Then it unions nodes by EVERY label they carry: schematic net labels and power taps connect
// by name across the whole sheet, not only where wires physically touch, so every "0" ground
// tap or every wire labelled "Vout" is one net regardless of geometry — and a node carrying
// two labels aliases them into one net (which is also how the KiCad hierarchy walk stitches
// sheets: shared join labels union a parent sheet pin with its child hierarchical label).
// The merged net's name is its lowest-ranked label (wire labels first, then Anchor.Rank,
// ties by input order). Unnamed geometric nodes stay separate and get a synthetic "N$<n>"
// name numbered by first appearance; an unnamed, pinless node is dropped as drawing noise.
// Conns within a net are deduplicated and sorted.
//
// It also returns the dangling endpoints (see Dangle): a wire endpoint whose grid point holds
// nothing else — no pin, anchor, terminal, or second wire endpoint. terminals are the points a
// format marks as a legitimate wire end: a junction dot (connected) or a no-connect flag
// (intentionally open). A wire ending on one is not dangling. Formats with neither pass nil.
// Dangles are sorted for determinism.
//
// The third return maps each identified wire (by Wire.Id) to the net its endpoints
// resolved to — the join the geometry sidecar needs to draw a KiCad wire by net (WS1-022).
// Only wires with a native id populate it; a wire whose node is drawing noise is omitted.
func Build(wires []Wire, anchors []Anchor, pins []Pin, terminals []Point) ([]Net, []Dangle, map[string]NetRef) {
	nets, dangles, wireNets, _ := BuildWithPoints(wires, anchors, pins, terminals)
	return nets, dangles, wireNets
}

// BuildWithPoints is Build plus a point->net-name map covering every input point (both wire
// endpoints, each anchor, each pin). A caller whose points carry extra coordinate meaning the
// neutral solver does not model — the KiCad hierarchy walk offsets each sheet instance into
// its own grid band (WS9-028) — decodes that meaning per point and reads off which net the
// point landed on, so it can attribute a net to the sheets it touches. A point that resolved
// to drawing noise (unnamed, pinless) is omitted, exactly as wireNets omits a noise wire.
func BuildWithPoints(wires []Wire, anchors []Anchor, pins []Pin, terminals []Point) ([]Net, []Dangle, map[string]NetRef, map[Point]NetRef) {
	uf := newUnionFind()
	for _, w := range wires {
		uf.union(w.A, w.B)
	}
	// Anchors and pins that share a point with a wire endpoint already collapse to the same
	// node via the point key; ensure their points exist as nodes too.
	for _, a := range anchors {
		uf.add(a.At)
	}
	for _, p := range pins {
		uf.add(p.At)
	}

	// Union by label BEFORE resolving roots for naming: every point carrying label L joins
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

	// Assign each root its lowest-ranked label: wire labels rank below every anchor
	// (preserving the old wire-label-first behavior), anchors compare by Rank, ties keep
	// input order.
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
		if c := name[r]; c.name == "" || rank < c.rank {
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
	ncName := map[int]string{} // root -> no-connect stub name, when a marked pin sits there
	for _, p := range pins {
		r := uf.find(uf.id[p.At])
		conns[r] = append(conns[r], Conn{Comp: p.Comp, Pin: p.Pin, Dir: p.Dir})
		if p.NoConnect {
			ncName[r] = "unconnected-(" + p.Comp + "-Pad" + p.Pin + ")"
		}
	}

	var nets []Net
	rootName := map[int]string{} // root -> final net name (incl. N$), for the wire->net map
	rootID := map[int]string{}   // root -> deterministic net id, for the wire/point->net-instance join
	stubN := 0
	for _, r := range rootOrder {
		nm := name[r].name
		cs := conns[r]
		if nm == "" && len(cs) == 1 {
			// A lone no-connect-marked pin gets the tool-marker name, not N$: the stub is
			// intentional and every no-connect-aware consumer keys on this vocabulary.
			nm = ncName[r]
		}
		if nm == "" {
			if len(cs) == 0 {
				continue // unnamed and pinless: drawing noise, not a net
			}
			stubN++
			nm = "N$" + strconv.Itoa(stubN)
		}
		sorted := dedupSort(cs)
		id := netID(sorted)
		rootName[r] = nm
		rootID[r] = id
		nets = append(nets, Net{Name: nm, ID: id, Conns: sorted, Driven: driven[r], External: external[r], Aliases: aliases[r]})
	}

	// wireNets maps each identified wire to the net its endpoints resolved to — the join
	// the geometry sidecar needs to draw/highlight a wire by net (WS1-022). Keyed by
	// Wire.Id, so only formats that give wires a native id (KiCad uuids) populate it;
	// formats whose wires carry their net name inline (xschem lab=, gEDA) fill
	// WireGeometry.net directly and pass empty ids here. A wire on drawing-noise (unnamed,
	// pinless) has no net and is omitted.
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

	// pointNets records the net each input point resolved to, so a caller that gave points
	// extra coordinate meaning (sheet-instance bands, WS9-028) can attribute nets to sheets.
	// Points on drawing noise carry no net and are omitted.
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

// dangling finds the wire endpoints that terminate on nothing. A grid point is "occupied" if a
// pin, an anchor/label, or a terminal (junction dot or no-connect flag) sits there; an endpoint is
// dangling when it is the sole wire endpoint at its point (degree 1) and that point is unoccupied.
// Two wires meeting end-to-end (degree 2) or a polyline's interior vertex are therefore connected,
// not dangling. Results are sorted by point for determinism.
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

// dedupSort removes duplicate (Comp,Pin) conns and sorts the rest for determinism.
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
