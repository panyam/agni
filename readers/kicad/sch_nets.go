package kicad

import (
	"sort"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/num"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/geomath"
	"github.com/panyam/agni/internal/netgraph"
)

// schNets extracts the nets from a schematic's geometry. A .kicad_sch stores connectivity
// implicitly (wires are segments, pins sit at symbol-relative coordinates, labels and power symbols
// name a point), so nets are computed by feeding wires, net-name anchors and absolute pin positions
// into the shared solver in internal/netgraph, as the xschem and gEDA readers do.
//
// Each lib pin is mapped to sheet coordinates with geomath.ApplyTransform, the transform the renderer
// draws with (C17), so a pin lands on the wire endpoint it connects to. Wires, labels and pins all go
// through the same coordinate conversion so coincident points compare equal.
func schNets(root *node, src string, syms *symLibCache) ([]*ir.Net, []*ir.DanglingEndpoint, []*ir.DanglingEndpoint, []*ir.JoinedTap) {
	var in netInputs
	collectSheetNets(root, sheetScope{src: src, syms: syms}, &in)
	built, dangles, _ := netgraph.Build(in.wires, in.anchors, in.pins, in.terminals, kicadNaming)
	// KiCad omits a named-but-pinless net (a label on a dangling wire), so drop it before emission.
	kept := built[:0]
	for _, n := range built {
		if len(n.Conns) > 0 {
			kept = append(kept, n)
		}
	}
	return netgraph.IRNets(kept, src), netgraph.IRDangles(dangles, src, "kicad-uuid"), in.noJunction, in.joinedTaps
}

// Anchor ranks for KiCad net naming, which netgraph resolves lowest first and, between equal
// ranks, alphabetically (netgraph.Options.AlphaTies). They follow KiCad's driver priority (agni
// issue 1019): a design-wide name (a global label or a power symbol) beats any sheet-scoped name, and
// among those a local label beats a hierarchical label, which beats a sheet pin. Global labels and
// power symbols share rank 0, because the alias rules read 0 as "design-wide" (netgraph.AttrAliases);
// KiCad also prefers a global label to a power symbol, which only matters when both name one net. A sheet-scoped rank adds
// the sheet's depth, because KiCad prefers the name nearer the root at equal priority. rankJoin
// names nothing and exists only to union a promoted bus member with its parent's. See
// docsite/content/architecture/net-solving.md#name-scoping.
const (
	rankGlobal   = 0   // global labels
	rankPower    = 0   // power-symbol rails, design-wide like a global label
	rankLocal    = 100 // local labels, plus sheet depth
	rankHier     = 200 // hierarchical labels, plus sheet depth
	rankSheetPin = 300 // a parent's sheet pin, plus sheet depth
	rankJoin     = 900
)

// kicadNaming is the netgraph naming every KiCad build uses, so a net's name is KiCad's own.
var kicadNaming = netgraph.Options{AlphaTies: true}

// netName spells a label as KiCad spells the net it names: the label's escapes resolved, then a
// slash escaped again and line breaks dropped, so "FTDI_CBUS0{slash}SDA" stays as written while
// the "/" separating a sheet path from a name does not collide with one inside it.
func netName(raw string) string {
	s := unescapeName(raw)
	s = strings.NewReplacer("/", "{slash}", "\n", "", "\r", "").Replace(s)
	return s
}

// sheetScope places one sheet INSTANCE's geometry inside a design-wide net solve (WS1-018). The
// zero value is the plain single-sheet read, with no offset, bare local names and first-entry ref
// resolution. The hierarchy walk gives each instance a disjoint grid offset so wires only join
// within their own sheet, its sheet path as a prefix for sheet-scoped labels, and its KiCad
// instances path for per-instance ref-des resolution of reused sheet files.
type sheetScope struct {
	offset   netgraph.Point
	prefix   string // "" for the root sheet, whose locals stay bare to match KiCad net names
	instPath string
	src      string // this instance's source file, for collector-emitted diagnostics
	syms     *symLibCache
	wirePfx  string // per-instance wire-id namespace for the WS1-022 wire->net map; "" means bare uuid
	// promoted maps a sheet-scoped name to the name it takes INSTEAD of this instance's
	// qualification, for members of a bus vector that entered through a bus sheet pin. So `AN0`
	// inside a sub-sheet reached by `AN[0..7]` is the PARENT's `AN0` (agni issue 561). Nil on the
	// root and on any instance no bus enters, which is the common case.
	promoted map[string]promotion
}

// promotion is the parent's name a promoted bus member unions with, and the rank that name has as a
// candidate: a local label's when the parent draws a label for the bus, else rankJoin, a join that
// names nothing.
type promotion struct {
	name string
	rank int
}

func (sc sheetScope) at(p netgraph.Point) netgraph.Point {
	return netgraph.Point{X: p.X + sc.offset.X, Y: p.Y + sc.offset.Y}
}

// wireID namespaces a wire uuid to this sheet instance for the wire->net map, because a reused
// sub-sheet's wires share uuids across instances and the bare uuid would collapse them (WS1-022).
// An empty uuid stays empty. Callers that do not build the map (the netlist walk) leave wirePfx
// empty and get the bare uuid.
func (sc sheetScope) wireID(uuid string) string {
	if uuid == "" || sc.wirePfx == "" {
		return uuid
	}
	return sc.wirePfx + "\x00" + uuid
}

// local qualifies a sheet-scoped name as KiCad does: "/X" on the root and "/<sheet path>/X" on a
// sub-sheet instance, the spelling the board reader produces too, so schematic-vs-board joins
// agree. A bus member that entered through a bus sheet pin keeps this qualified name; join gives
// the parent's name it must union with.
func (sc sheetScope) local(name string) string {
	return sc.prefix + "/" + name
}

// join returns the parent's name a promoted bus member unions with (agni issue 561), if it is one.
func (sc sheetScope) join(name string) (promotion, bool) {
	p, ok := sc.promoted[name]
	return p, ok
}

// depth is the instance's distance from the root sheet, which orders equal-priority names.
func (sc sheetScope) depth() int {
	return strings.Count(sc.prefix, "/")
}

// localAnchors returns a sheet-scoped label's anchor at the given rank, plus the join anchor a
// promoted bus member needs so it still unions with its parent's member.
func (sc sheetScope) localAnchors(at netgraph.Point, name string, rank int, external bool) []netgraph.Anchor {
	out := []netgraph.Anchor{{At: at, Label: sc.local(name), External: external, Rank: rank + sc.depth()}}
	if p, ok := sc.join(name); ok {
		out = append(out, netgraph.Anchor{At: at, Label: p.name, Rank: p.rank})
	}
	return out
}

// netInputs accumulates the connectivity solver's inputs across one or more sheet
// instances; one netgraph.Build call solves whatever was collected.
type netInputs struct {
	wires     []netgraph.Wire
	anchors   []netgraph.Anchor
	pins      []netgraph.Pin
	terminals []netgraph.Point
	// noJunction holds the WS1-012 endpoint-on-body diagnostics in SHEET-frame coordinates,
	// collected before the walk's offset.
	noJunction []*ir.DanglingEndpoint
	// joinedTaps are the WS1-012 taps that something DOES join, the other half of noJunction, so the
	// rule over them partitions rather than filters (agni issue 420).
	joinedTaps []*ir.JoinedTap
}

// collectSheetNets gathers one sheet instance's wires, anchors, pins and terminals into in, placed
// by sc. Both the single-sheet read (schNets, zero scope) and the hierarchy walk go through it, so
// the two cannot drift. Everything is collected in SHEET-frame coordinates and offset on append,
// because wire splitting would overflow int64 in the walk's offset bands (see splitWiresAt).
func collectSheetNets(root *node, sc sheetScope, in *netInputs) {
	var wires []netgraph.Wire
	var anchors []netgraph.Anchor
	var pins []netgraph.Pin
	var terminals []netgraph.Point
	var onWire []netgraph.Point // points that bind anywhere ALONG a wire, not only at its ends
	// joins records WHAT binds at each on-wire point, for the joined-tap half of the T-tap diagnostic
	// (agni issue 420). Junction dots are collected before labels and the label branches do not
	// overwrite, so a dot wins over a label at the same point, since the dot is what a reviewer is
	// checking for.
	joins := map[netgraph.Point]tapJoin{}

	// Each (wire (pts (xy ..) (xy ..) ..)) becomes a segment per consecutive point pair, carrying the
	// wire's uuid (qualified per instance by sc.wireID) so a dangling endpoint can point back at it.
	for _, w := range root.Children("wire") {
		pts := xyPoints(w.Child("pts"), sheetPt)
		id := sc.wireID(uuidOf(w))
		for i := 0; i+1 < len(pts); i++ {
			wires = append(wires, netgraph.Wire{A: gp(pts[i]), B: gp(pts[i+1]), Id: id})
		}
	}

	// A wire ending on a junction dot (connected) or a no-connect flag (intentionally open) is not
	// dangling, so both are terminals. The no-connect points are also kept in ncPts, because a marker
	// ON a pin's connect point declares that pin intentionally unconnected (WS1-019), which the pin
	// collection below stamps onto netgraph.Pin.NoConnect.
	ncPts := map[netgraph.Point]bool{}
	for _, tag := range []string{"junction", "no_connect"} {
		for _, t := range root.Children(tag) {
			if at := sheetPt(t.Child("at")); at != nil {
				p := gp(at)
				terminals = append(terminals, p)
				if tag == "no_connect" {
					ncPts[p] = true
				} else {
					onWire = append(onWire, p) // a junction dot joins the wires it sits on
					joins[p] = tapJoin{kind: "junction"}
				}
			}
		}
	}
	// A bus entry joins a wire to a bus, so a wire ending on either of its two ends is connected rather
	// than dangling (agni issue 808). It contributes terminals only: which bus member the wire carries
	// comes from the wire's own label, not from the entry, so the entry joins no net here. Without this
	// every wire drawn to a bus entry was a dangling endpoint, 428 of them on one 17-sheet board.
	for _, e := range root.Children("bus_entry") {
		for _, p := range busEntryPoints(e) {
			terminals = append(terminals, gp(p))
		}
	}
	// A hierarchical sheet's pins are ports to its sub-sheet, so a wire ending on one is not
	// dangling. The single-sheet read stops there, having no child file. The hierarchy walk ALSO
	// emits a port anchor at this point, joining the parent net to the child's hierarchical label.
	for _, sh := range root.Children("sheet") {
		for _, p := range sh.Children("pin") {
			if at := sheetPt(p.Child("at")); at != nil {
				terminals = append(terminals, gp(at))
			}
		}
	}

	// Anchors from labels. A local label is sheet-scoped and qualified by the instance prefix. A
	// global label is design-wide, bare, and External, since it may continue into sheets this read
	// did not cover (a complete project walk downgrades that, WS1-017). A hierarchical label is the
	// child half of a parent sheet-pin port, so on the walk it takes the SAME qualified name the
	// parent's port anchor emits and is not External. On a single-sheet read (empty prefix) it keeps
	// its bare name and is marked External, because the binding parent, if any, was not read.
	for _, l := range root.Children("label") {
		if at := sheetPt(l.Child("at")); at != nil {
			anchors = append(anchors, sc.localAnchors(gp(at), netName(atomOf(l.Arg(1))), rankLocal, false)...)
			onWire = append(onWire, gp(at))
			if _, seen := joins[gp(at)]; !seen {
				joins[gp(at)] = tapJoin{kind: "label", label: sc.local(netName(atomOf(l.Arg(1))))}
			}
		}
	}
	for _, l := range root.Children("global_label") {
		if at := sheetPt(l.Child("at")); at != nil {
			anchors = append(anchors, netgraph.Anchor{At: gp(at), Label: netName(atomOf(l.Arg(1))), External: true, Rank: rankGlobal})
			onWire = append(onWire, gp(at))
			if _, seen := joins[gp(at)]; !seen {
				joins[gp(at)] = tapJoin{kind: "label", label: netName(atomOf(l.Arg(1)))}
			}
		}
	}
	for _, l := range root.Children("hierarchical_label") {
		if at := sheetPt(l.Child("at")); at != nil {
			anchors = append(anchors, sc.localAnchors(gp(at), netName(atomOf(l.Arg(1))), rankHier, sc.prefix == "")...)
			onWire = append(onWire, gp(at))
			if _, seen := joins[gp(at)]; !seen {
				joins[gp(at)] = tapJoin{kind: "label", label: sc.local(netName(atomOf(l.Arg(1))))}
			}
		}
	}

	libPins := libPinIndex(root, sc.syms)

	// A placed real component contributes its pins. A power symbol (#PWR) is not a physical
	// component, and its pin is a net-name anchor named by the symbol's Value (GND, +5V). A power
	// FLAG (#FLG, value "PWR_FLAG") only asserts a net is driven and does NOT name it, so it
	// contributes an anchor with an empty label (a pinless junction).
	for _, ps := range root.Children("symbol") {
		ref := symbolRefAt(ps, sc.instPath)
		if ref == "" {
			continue
		}
		t := pinTransform(ps)
		local := libPins.forUnit(symbolKey(ps), unitOf(ps))
		if ref[0] == '#' {
			name := netName(propValue(ps, "Value"))
			isFlag := name == "PWR_FLAG" // a directive asserting the net is driven, not a net name
			if isFlag {
				name = ""
			}
			for _, pp := range local {
				at := gp(geomath.ApplyTransform(t, pp.loc))
				anchors = append(anchors, netgraph.Anchor{At: at, Label: name, Driver: isFlag, External: !isFlag, Rank: rankPower})
				// The symbol's pin ALSO becomes a typed virtual connection (WS1-014) carrying
				// the power evidence rules reason over. A PWR_FLAG's power_out pin is the
				// driver, and a rail symbol's power_in pin is KiCad's "this rail wants a
				// driver" declaration. The virtual component (#PWR05) never enters
				// Components, so the direction rides ir.Connection.attributes["direction"].
				if d := powerDir(pp.etype); d != "" {
					pins = append(pins, netgraph.Pin{At: at, Comp: ref, Pin: pp.designator, Dir: d})
				}
			}
			continue
		}
		multi := libPins.multiUnit(symbolKey(ps))
		onBoard := ps.Child("on_board") == nil || atomOf(ps.Child("on_board").Arg(1)) != "no"
		for _, pp := range local {
			at := gp(geomath.ApplyTransform(t, pp.loc))
			pin := netgraph.Pin{At: at, Comp: ref, Pin: pp.designator, NoConnect: ncPts[at]}
			if onBoard {
				pin.AutoName, pin.NCName = pinNetNames(ref, unitOf(ps), multi, pp, local)
			}
			pins = append(pins, pin)
		}
	}

	// KiCad attaches LABELS and JUNCTION DOTS anywhere ALONG a wire while the solver unions by point
	// identity, so each wire is split at those on-body points. PINS and other wires' endpoints are
	// NOT split candidates. See docsite/content/architecture/net-solving.md#kicad-connection-point-semantics.
	// The joined taps must be found BEFORE the split, because afterwards a dotted or labeled tap is an
	// endpoint of both wires and looks like a point no wire crossed (agni issue 420). segments is
	// counted after, since a tap joins the wire halves the split creates.
	preSplit := wires
	wires = splitWiresAt(wires, onWire)
	segments := map[netgraph.Point]int{}
	for _, w := range wires {
		segments[w.A]++
		segments[w.B]++
	}
	in.joinedTaps = append(in.joinedTaps, joinedTaps(preSplit, joins, segments, sc.src)...)

	// WS1-012: after the split a dotted or labeled touch point is an endpoint of both wires, so an
	// endpoint still INTERIOR to another segment's body is the silent T-tap, drawn as connected and
	// electrically two nets. Crossings with no endpoint at the meet never flag, since KiCad does not
	// connect crossings and the drawing does not suggest it.
	noJunction := noJunctionEndpoints(wires, sc.src)
	in.noJunction = append(in.noJunction, noJunction...)
	// The on-body endpoints also become solver terminals, so the dangling check leaves them to
	// wire-no-junction's more specific diagnosis instead of both rules reporting the same point.
	for _, e := range noJunction {
		terminals = append(terminals, netgraph.Point{X: e.X, Y: e.Y})
	}

	for _, w := range wires {
		in.wires = append(in.wires, netgraph.Wire{A: sc.at(w.A), B: sc.at(w.B), Label: w.Label, Id: w.Id})
	}
	for _, a := range anchors {
		a.At = sc.at(a.At)
		in.anchors = append(in.anchors, a)
	}
	for _, p := range pins {
		p.At = sc.at(p.At)
		in.pins = append(in.pins, p)
	}
	for _, t := range terminals {
		in.terminals = append(in.terminals, sc.at(t))
	}
}

// endpointsOnBody reports every wire endpoint lying strictly inside another segment's body, with the
// id of the wire the endpoint belongs to. O(endpoints x segments) with the bbox early-out in
// onSegment, the same cost class as the split pass.
//
// Both halves of the T-tap diagnostic run this one definition at two points in the pipeline, which
// keeps them a partition. Run POST-split it yields the silent taps, since a junction dot or a
// mid-span label has already made a joined tap an endpoint of both wires. Run PRE-split it yields
// every tap, joined or not.
func endpointsOnBody(wires []netgraph.Wire) []bodyTap {
	var out []bodyTap
	seen := map[netgraph.Point]bool{}
	for _, w := range wires {
		for _, p := range []netgraph.Point{w.A, w.B} {
			if seen[p] {
				continue
			}
			for _, o := range wires {
				if o.Id == w.Id {
					continue // a polyline's own corners are endpoints of both halves
				}
				if onSegment(o.A, o.B, p) {
					seen[p] = true
					out = append(out, bodyTap{At: p, WireId: w.Id})
					break
				}
			}
		}
	}
	return out
}

// bodyTap is one wire endpoint sitting inside another wire's body, before anything decides whether
// the two are joined.
type bodyTap struct {
	At     netgraph.Point
	WireId string
}

// noJunctionEndpoints reports the taps with NOTHING joining them (post-split, sheet frame), which
// are the silent T-taps the rule reports.
func noJunctionEndpoints(wires []netgraph.Wire, src string) []*ir.DanglingEndpoint {
	var out []*ir.DanglingEndpoint
	for _, t := range endpointsOnBody(wires) {
		out = append(out, &ir.DanglingEndpoint{X: t.At.X, Y: t.At.Y, Prov: tapProv(t.WireId, src)})
	}
	return out
}

// joinedTaps reports the taps that ARE joined (pre-split, sheet frame), naming the construct that
// joined them (agni issue 420). joins maps a point to whatever binds along a wire there; segments
// counts the wire ends meeting at the point once the split has run. Callers run it pre-split,
// because after the split a joined tap is indistinguishable from a point no wire crossed.
func joinedTaps(wires []netgraph.Wire, joins map[netgraph.Point]tapJoin, segments map[netgraph.Point]int, src string) []*ir.JoinedTap {
	var out []*ir.JoinedTap
	for _, t := range endpointsOnBody(wires) {
		j, ok := joins[t.At]
		if !ok {
			continue // nothing joins this tap; noJunctionEndpoints reports it after the split
		}
		out = append(out, &ir.JoinedTap{
			X: t.At.X, Y: t.At.Y,
			JoinKind: j.kind, Label: j.label,
			Segments: num.Int32(segments[t.At]),
			Prov:     tapProv(t.WireId, src),
		})
	}
	return out
}

// tapJoin is what binds a point along a wire: an explicit junction dot, or a label whose text names
// the net there.
type tapJoin struct {
	kind  string // "junction" | "label"
	label string
}

func tapProv(wireID, src string) *ir.Provenance {
	prov := &ir.Provenance{SourceFile: src}
	if wireID != "" {
		prov.NativeId, prov.NativeIdKind = wireID, kicadNativeIDKind
	}
	return prov
}

// splitWiresAt splits wire segments at the given points where a point lies strictly
// inside a segment (collinear, between the endpoints). Sheet-frame coordinates only, because
// the collinearity cross product needs |coord| well under sqrt(MaxInt64), which sheet
// coordinates satisfy and the hierarchy walk's offset bands do not.
func splitWiresAt(wires []netgraph.Wire, pts []netgraph.Point) []netgraph.Wire {
	uniq := map[netgraph.Point]bool{}
	for _, p := range pts {
		uniq[p] = true
	}
	var out []netgraph.Wire
	for _, w := range wires {
		var hits []netgraph.Point
		for p := range uniq {
			if onSegment(w.A, w.B, p) {
				hits = append(hits, p)
			}
		}
		if len(hits) == 0 {
			out = append(out, w)
			continue
		}
		// Order the cut points along the segment, then emit the chain.
		sort.Slice(hits, func(i, j int) bool {
			di := absI64(hits[i].X-w.A.X) + absI64(hits[i].Y-w.A.Y)
			dj := absI64(hits[j].X-w.A.X) + absI64(hits[j].Y-w.A.Y)
			return di < dj
		})
		prev := w.A
		for _, h := range hits {
			out = append(out, netgraph.Wire{A: prev, B: h, Label: w.Label, Id: w.Id})
			prev = h
		}
		out = append(out, netgraph.Wire{A: prev, B: w.B, Label: w.Label, Id: w.Id})
	}
	return out
}

// onSegment reports whether p lies strictly inside segment ab (collinear and between,
// excluding the endpoints, which already union by point identity).
func onSegment(a, b, p netgraph.Point) bool {
	if p == a || p == b {
		return false
	}
	if p.X < min64(a.X, b.X) || p.X > max64(a.X, b.X) || p.Y < min64(a.Y, b.Y) || p.Y > max64(a.Y, b.Y) {
		return false
	}
	if (b.X-a.X)*(p.Y-a.Y) != (b.Y-a.Y)*(p.X-a.X) {
		return false
	}
	dot := (p.X-a.X)*(b.X-a.X) + (p.Y-a.Y)*(b.Y-a.Y)
	if dot <= 0 {
		return false
	}
	length2 := (b.X-a.X)*(b.X-a.X) + (b.Y-a.Y)*(b.Y-a.Y)
	return dot < length2
}

func absI64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// unescapeName undoes KiCad's brace escapes in label and net names, so "VPP{slash}MCLR" and
// "VPP/MCLR" are ONE net, as kicad-cli reads them (pic_programmer has both). Unknown {tokens}
// stay literal. See docsite/content/architecture/net-solving.md#kicad-connection-point-semantics.
func unescapeName(s string) string {
	if !strings.Contains(s, "{") {
		return s
	}
	repl := strings.NewReplacer(
		"{slash}", "/",
		"{colon}", ":",
		"{space}", " ",
		"{dblquote}", `"`,
		"{quote}", "'",
		"{lt}", "<",
		"{gt}", ">",
		"{bar}", "|",
		"{comma}", ",",
		"{tab}", "\t",
		"{return}", "\n",
		"{brace}", "{",
	)
	return repl.Replace(s)
}

// gp converts a geom point to a netgraph grid point; a nil point is the origin.
func gp(p *geom.Point) netgraph.Point {
	if p == nil {
		return netgraph.Point{}
	}
	return netgraph.Point{X: p.X, Y: p.Y}
}

// pinTransform is the placement transform for computing pin CONNECT positions. It differs from
// kicadTransform only in rotation. Lib pin coordinates are in KiCad's un-flipped frame, so an
// unmirrored placement takes KiCad's raw angle, since geomRotation's 360-deg flip is right at 0/180
// and swaps 90<->270.
//
// A MIRRORED placement keeps geomRotation's flipped angle. KiCad rotates and THEN mirrors while
// geomath.ApplyTransform mirrors first, and the inverse angle reconciles the two; the derivation is in
// docsite/content/architecture/net-solving.md#kicad-connection-point-semantics. Otherwise a symbol
// mirrored at 90 or 270 swaps its two pins onto each other's nets (agni issue 577). Measured against
// kicad-cli across all twelve placements of a two-pin symbol; the matrix is TestMirroredPinPlacement.
func pinTransform(ps *node) *geom.Transform {
	t := kicadTransform(ps)
	if t.MirrorX || t.MirrorY {
		return t
	}
	if a := ps.Child("at").Arg(3); a != nil {
		if deg, err := strconv.ParseFloat(atomOf(a), 64); err == nil {
			t.RotationDeg = ((int32(deg) % 360) + 360) % 360
		}
	}
	return t
}

// unitOf reads a placement's (unit N); placements without one are unit 1.
func unitOf(ps *node) int {
	if u := ps.Child("unit"); u != nil {
		if n, err := strconv.Atoi(atomOf(u.Arg(1))); err == nil {
			return n
		}
	}
	return 1
}

// pinPos is one lib-symbol pin: its designator and its symbol-local connect point.
type pinPos struct {
	designator string
	name       string // the pin's name, empty for KiCad's "~"
	loc        *geom.Point
	etype      string // the pin's electrical type atom as printed ("power_in", "passive", ...)
}

// libPinsByUnit indexes a lib symbol's pins by unit (0 = graphics/pins common to every unit).
type libPinsByUnit map[int][]pinPos

// schPinIndex maps each lib_symbols entry (by lib_id) to its pins keyed by unit, falling
// back to the external symbol cache (WS1-016) for lib_ids the schematic does not embed.
type schPinIndex struct {
	byID map[string]libPinsByUnit
	syms *symLibCache
}

// libPinIndex reads each lib symbol's pins (local connect point + designator, via the same kicadPin
// the geometry reader uses) into an index keyed by lib_id then unit.
func libPinIndex(root *node, syms *symLibCache) schPinIndex {
	idx := schPinIndex{syms: syms, byID: map[string]libPinsByUnit{}}
	for _, libSym := range root.Child("lib_symbols").Children("symbol") {
		idx.byID[atomOf(libSym.Arg(1))] = symPinsByUnit(libSym)
	}
	return idx
}

// symPinsByUnit indexes one lib-symbol node's pins by unit.
func symPinsByUnit(libSym *node) libPinsByUnit {
	byUnit := libPinsByUnit{}
	for _, sub := range libSym.Children("symbol") {
		u := unitOfSubSymbol(atomOf(sub.Arg(1)))
		for _, pn := range sub.Children("pin") {
			if p := kicadPin(pn, false); p != nil && p.PortRef != "" {
				name := ""
				if nm := pn.Child("name"); nm != nil && atomOf(nm.Arg(1)) != "~" {
					name = atomOf(nm.Arg(1))
				}
				byUnit[u] = append(byUnit[u], pinPos{designator: p.PortRef, name: name, loc: p.Loc, etype: atomOf(pn.Arg(1))})
			}
		}
	}
	return byUnit
}

// multiUnit reports whether a lib symbol draws more than one unit, which is when KiCad adds the
// unit letter to a reference in a net name.
func (idx schPinIndex) multiUnit(libID string) bool {
	units := 0
	for u := range idx.byID[libID] {
		if u > 0 {
			units++
		}
	}
	return units > 1
}

// pinNetNames are the two names KiCad gives a net after one of its pins (agni issue 1019): auto,
// "Net-(...)" for an unlabelled net, and nc, "unconnected-(...)" for a no-connect-flagged pin's
// lone stub. A pin with a real name is named by reference (with the unit letter on a multi-unit
// part) and pin name, adding the pad number when the pin is unconnected or the part has another
// pin of that name. A pin with no name, or one named by its own number, is named by pad. A
// no-connect pin type is always "unconnected". See KiCad's SCH_PIN::GetDefaultNetName.
func pinNetNames(ref string, unit int, multi bool, pp pinPos, unitPins []pinPos) (auto, nc string) {
	if ref == "" || strings.HasSuffix(ref, "?") {
		return "", ""
	}
	name := func(unconnected bool) string {
		head := "Net-("
		if unconnected {
			head = "unconnected-("
		}
		if pp.name == "" || pp.name == pp.designator {
			return head + ref + "-Pad" + netName(pp.designator) + ")"
		}
		multiple := false
		for _, o := range unitPins {
			if o.name == pp.name && o.designator != pp.designator && unconnected == (o.etype == "no_connect") {
				multiple = true
				break
			}
		}
		r := ref
		if multi {
			r += unitLetter(unit)
		}
		out := head + r + "-" + netName(pp.name)
		if unconnected || multiple {
			out += "-Pad" + netName(pp.designator)
		}
		return out + ")"
	}
	isNC := pp.etype == "no_connect"
	return name(isNC), name(true)
}

// unitLetter is KiCad's letter for a unit number: A for 1, Z for 26, then AA, AB and so on.
func unitLetter(unit int) string {
	if unit < 1 {
		return ""
	}
	var out []byte
	for unit > 0 {
		unit--
		out = append([]byte{byte('A' + unit%26)}, out...)
		unit /= 26
	}
	return string(out)
}

// forUnit returns the pins a placement of the given unit exposes: the common (unit 0) pins plus
// that unit's own pins.
func (idx schPinIndex) forUnit(libID string, unit int) []pinPos {
	byUnit, ok := idx.byID[libID]
	if !ok {
		if sym := idx.syms.symbol(libID); sym != nil {
			byUnit = symPinsByUnit(sym)
		}
		idx.byID[libID] = byUnit // cache the miss too
	}
	out := append([]pinPos(nil), byUnit[0]...)
	if unit != 0 {
		out = append(out, byUnit[unit]...)
	}
	return out
}

// powerDir maps a lib pin's electrical-type atom to the connection-direction vocabulary, for
// VIRTUAL (power-symbol) pins only. Power directions pass through and everything else is empty,
// so a virtual pin never fabricates signal-direction facts.
func powerDir(etype string) string {
	switch etype {
	case "power_in", "power_out":
		return etype
	}
	return ""
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
