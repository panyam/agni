package kicad

import (
	"sort"
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/geomath"
	"github.com/panyam/agni/internal/netgraph"
)

// schNets extracts the nets from a schematic's geometry. A .kicad_sch stores connectivity
// implicitly — wires are line segments, pins sit at symbol-relative coordinates, and labels/power
// symbols name a point — so nets are computed, not read: it feeds the wires, net-name anchors, and
// absolute pin positions into the shared connectivity solver (internal/netgraph), the same path the
// xschem and gEDA readers take.
//
// Pin positions are the one non-trivial part: a lib symbol's pins are in symbol-local coordinates,
// so each is mapped to sheet coordinates with geomath.ApplyTransform(placement, pin) — the same
// shared transform the renderer draws with (internal/geomath, C15), which is what guarantees a pin
// lands on the wire endpoint it connects to. Wires, labels, and pins all pass through the same coordinate conversion
// so coincident points compare equal.
func schNets(root *node, src string, syms *symLibCache) ([]*ir.Net, []*ir.DanglingEndpoint, []*ir.DanglingEndpoint, []*ir.JoinedTap) {
	var in netInputs
	collectSheetNets(root, sheetScope{src: src, syms: syms}, &in)
	built, dangles, _ := netgraph.Build(in.wires, in.anchors, in.pins, in.terminals)
	// KiCad itself omits a named-but-pinless net (a label on a dangling wire), so filter
	// before the shared emission.
	kept := built[:0]
	for _, n := range built {
		if len(n.Conns) > 0 {
			kept = append(kept, n)
		}
	}
	return netgraph.IRNets(kept, src), netgraph.IRDangles(dangles, src, "kicad-uuid"), in.noJunction, in.joinedTaps
}

// Anchor ranks for KiCad net naming (netgraph picks the lowest-ranked label on a net).
// Bare design-wide names (global labels, power rails) beat instance-qualified names
// (local labels, hierarchical ports), so a rail touched by both a VCC tap and a local
// alias is still "VCC" — and on the hierarchy walk a net named on the root wins over a
// deeper sheet's name of equal kind (input order breaks the tie, and the walk is
// pre-order).
const (
	rankGlobal = 0 // global labels, power-symbol rails
	rankLocal  = 1 // local labels, hierarchical-label/sheet-pin port names
)

// sheetScope situates one sheet INSTANCE's geometry inside a design-wide net solve
// (WS1-018). The zero value is the plain single-sheet read: no offset, bare local names,
// first-entry ref resolution. The hierarchy walk gives each instance a disjoint grid
// offset (so wires only join within their own sheet), a name prefix (the instance's sheet
// path, qualifying sheet-scoped labels), and the KiCad instances path (per-instance
// ref-des resolution for reused sheet files).
type sheetScope struct {
	offset   netgraph.Point
	prefix   string // "" for the root sheet: root locals stay bare, matching KiCad net names
	instPath string
	src      string // this instance's source file, for collector-emitted diagnostics
	syms     *symLibCache
	wirePfx  string // per-instance wire-id namespace for the WS1-022 wire->net map ("" = bare uuid)
}

func (sc sheetScope) at(p netgraph.Point) netgraph.Point {
	return netgraph.Point{X: p.X + sc.offset.X, Y: p.Y + sc.offset.Y}
}

// wireID namespaces a wire uuid to this sheet instance for the wire->net map: a reused
// sub-sheet's wires share uuids across instances, so the bare uuid would collapse them
// (WS1-022). Empty uuid stays empty (no id). Callers that do not build the map (the
// netlist walk) leave wirePfx empty, so the id is the bare uuid, unchanged.
func (sc sheetScope) wireID(uuid string) string {
	if uuid == "" || sc.wirePfx == "" {
		return uuid
	}
	return sc.wirePfx + "\x00" + uuid
}

// local qualifies a sheet-scoped name: "X" on the root stays "X", on a sub-sheet instance
// it becomes "/<sheet path>/X" — KiCad's own net-name convention, which is what the board
// reader already produces, so schematic-vs-board joins agree.
func (sc sheetScope) local(name string) string {
	if sc.prefix == "" {
		return name
	}
	return sc.prefix + "/" + name
}

// netInputs accumulates the connectivity solver's inputs across one or more sheet
// instances; one netgraph.Build call solves whatever was collected.
type netInputs struct {
	wires     []netgraph.Wire
	anchors   []netgraph.Anchor
	pins      []netgraph.Pin
	terminals []netgraph.Point
	// noJunction are the WS1-012 endpoint-on-body diagnostics, in SHEET-frame
	// coordinates (collected before the walk's offset, so no translation back).
	noJunction []*ir.DanglingEndpoint
	// joinedTaps are the WS1-012 taps that something DOES join, the other half of noJunction, so the
	// rule over them partitions rather than filters (agni issue 420).
	joinedTaps []*ir.JoinedTap
}

// collectSheetNets gathers one sheet instance's wires, anchors, pins, and terminals into
// in, situated by sc. It is the single emission path for both the single-sheet read
// (schNets, zero scope) and the hierarchy walk, so the two cannot drift. Everything is
// collected in SHEET-frame coordinates first — wire splitting needs collinearity products
// that would overflow int64 in the walk's offset bands — and offset on append.
func collectSheetNets(root *node, sc sheetScope, in *netInputs) {
	var wires []netgraph.Wire
	var anchors []netgraph.Anchor
	var pins []netgraph.Pin
	var terminals []netgraph.Point
	var onWire []netgraph.Point // points that bind anywhere ALONG a wire, not only at its ends
	// joins records WHAT binds at each on-wire point, for the joined-tap half of the T-tap diagnostic
	// (agni issue 420). Junction dots are collected before labels below and the label branches do not
	// overwrite, so an explicit dot wins over a label sharing its point: the dot is the construct
	// someone placed on purpose and is the one a reviewer is checking for.
	joins := map[netgraph.Point]tapJoin{}

	// Wires: each (wire (pts (xy ..) (xy ..) ..)) becomes a segment between consecutive points.
	// The wire's uuid rides on every segment so a dangling endpoint can point back at it.
	// sc.wireID qualifies it per instance for the wire->net map: a reused sub-sheet's wires
	// share uuids across instances, so a bare uuid would collapse them (WS1-022).
	for _, w := range root.Children("wire") {
		pts := xyPoints(w.Child("pts"), sheetPt)
		id := sc.wireID(uuidOf(w))
		for i := 0; i+1 < len(pts); i++ {
			wires = append(wires, netgraph.Wire{A: gp(pts[i]), B: gp(pts[i+1]), Id: id})
		}
	}

	// Terminals are points where a bare wire end is not a defect: a junction dot (connected) or a
	// no-connect flag (intentionally open). A wire ending on either is not dangling. The
	// no-connect points are also kept separately: a marker placed ON a pin's connect point
	// declares that pin intentionally unconnected (WS1-019), which the pin collection below
	// stamps onto netgraph.Pin.NoConnect.
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
	// A hierarchical sheet's pins are connection ports to its sub-sheet; a wire ending on one is
	// connected to the child, not dangling. The single-sheet read stops there (the port net
	// resolves no further without the child file); the hierarchy walk ALSO emits a port
	// anchor at this point, unioning the parent net with the child's hierarchical label.
	for _, sh := range root.Children("sheet") {
		for _, p := range sh.Children("pin") {
			if at := sheetPt(p.Child("at")); at != nil {
				terminals = append(terminals, gp(at))
			}
		}
	}

	// Anchors from labels. A local label is sheet-scoped: qualified by the instance prefix
	// ("/<sheet path>/NAME", bare on the root). A global label is design-wide: bare, and
	// External (it may continue into sheets this read did not cover; a complete project
	// walk downgrades that, WS1-017). A hierarchical label is the child half of a parent
	// sheet-pin port: on the walk it gets the SAME qualified name the parent's port anchor
	// emits — label-union stitches the two sheets — and it stays sheet-scoped, not
	// External. On a single-sheet read (empty prefix: this file is the root or is being
	// read alone) it keeps its bare name and the conservative External marking, because
	// the binding parent, if any, was not read.
	for _, l := range root.Children("label") {
		if at := sheetPt(l.Child("at")); at != nil {
			anchors = append(anchors, netgraph.Anchor{At: gp(at), Label: sc.local(unescapeName(atomOf(l.Arg(1)))), Rank: rankLocal})
			onWire = append(onWire, gp(at))
			if _, seen := joins[gp(at)]; !seen {
				joins[gp(at)] = tapJoin{kind: "label", label: sc.local(unescapeName(atomOf(l.Arg(1))))}
			}
		}
	}
	for _, l := range root.Children("global_label") {
		if at := sheetPt(l.Child("at")); at != nil {
			anchors = append(anchors, netgraph.Anchor{At: gp(at), Label: unescapeName(atomOf(l.Arg(1))), External: true, Rank: rankGlobal})
			onWire = append(onWire, gp(at))
			if _, seen := joins[gp(at)]; !seen {
				joins[gp(at)] = tapJoin{kind: "label", label: unescapeName(atomOf(l.Arg(1)))}
			}
		}
	}
	for _, l := range root.Children("hierarchical_label") {
		if at := sheetPt(l.Child("at")); at != nil {
			anchors = append(anchors, netgraph.Anchor{At: gp(at), Label: sc.local(unescapeName(atomOf(l.Arg(1)))), External: sc.prefix == "", Rank: rankLocal})
			onWire = append(onWire, gp(at))
			if _, seen := joins[gp(at)]; !seen {
				joins[gp(at)] = tapJoin{kind: "label", label: sc.local(unescapeName(atomOf(l.Arg(1))))}
			}
		}
	}

	libPins := libPinIndex(root, sc.syms)

	// Placed symbols: a real component contributes its pins. A power symbol (#PWR) is not a physical
	// component — its pin is a net-name anchor whose name is the symbol's Value (GND, +5V). A power
	// FLAG (#FLG, value "PWR_FLAG") only asserts a net is driven; it is still a connection point but
	// does NOT name the net, so it contributes an anchor with an empty label (a pinless junction).
	for _, ps := range root.Children("symbol") {
		ref := symbolRefAt(ps, sc.instPath)
		if ref == "" {
			continue
		}
		t := pinTransform(ps)
		local := libPins.forUnit(atomOf(ps.Child("lib_id").Arg(1)), unitOf(ps))
		if ref[0] == '#' {
			name := propValue(ps, "Value")
			isFlag := name == "PWR_FLAG" // a directive, not a net name: it asserts the net is driven
			if isFlag {
				name = ""
			}
			for _, pp := range local {
				at := gp(geomath.ApplyTransform(t, pp.loc))
				anchors = append(anchors, netgraph.Anchor{At: at, Label: name, Driver: isFlag, External: !isFlag, Rank: rankGlobal})
				// The symbol's pin ALSO becomes a typed virtual connection (WS1-014): the
				// net keeps its name anchor semantics (WS1-017 untouched) and gains the
				// power evidence rules reason over — a PWR_FLAG's power_out pin is the
				// driver, a rail symbol's power_in pin is KiCad's "this rail wants a
				// driver" declaration. Only power directions are emitted; the virtual
				// component (#PWR05) never enters Components, so the direction rides the
				// connection attribute (ir.Connection.attributes["direction"]).
				if d := powerDir(pp.etype); d != "" {
					pins = append(pins, netgraph.Pin{At: at, Comp: ref, Pin: pp.designator, Dir: d})
				}
			}
			continue
		}
		for _, pp := range local {
			at := gp(geomath.ApplyTransform(t, pp.loc))
			pins = append(pins, netgraph.Pin{At: at, Comp: ref, Pin: pp.designator, NoConnect: ncPts[at]})
		}
	}

	// KiCad attaches LABELS and JUNCTION DOTS anywhere ALONG a wire — eeschema never
	// rewrites the wire when a label lands mid-span (the corpus: an S_OUT+ label in the
	// middle of a 70mm segment) — while the solver unions by point identity at endpoints.
	// Splitting each wire at those on-body points reconciles the two. PINS are NOT split
	// candidates: KiCad joins a pin only where a wire ENDS on its connect point (the
	// showcase board has a GND symbol whose pin sits mid-span on the USB_D- wire, and
	// kicad-cli keeps them separate), and wire-wire crossings without a junction dot
	// likewise stay unconnected, so other wires' endpoints are not candidates either.
	// The joined taps must be found BEFORE the split, because the split is exactly what hides them:
	// afterwards a dotted or labeled tap is an endpoint of both wires and reads like a point where no
	// wire ever crossed (agni issue 420). segments is counted after, since a tap joins the wire halves
	// the split creates.
	preSplit := wires
	wires = splitWiresAt(wires, onWire)
	segments := map[netgraph.Point]int{}
	for _, w := range wires {
		segments[w.A]++
		segments[w.B]++
	}
	in.joinedTaps = append(in.joinedTaps, joinedTaps(preSplit, joins, segments, sc.src)...)

	// WS1-012: after the split, a junction-dotted or labeled touch point is already an
	// endpoint of both wires. An endpoint still INTERIOR to another segment's body is
	// therefore exactly the silent T-tap: drawn as connected, electrically two nets
	// (KiCad connects only at a dot). Crossings with no endpoint at the meet never flag —
	// KiCad does not connect crossings regardless, so the drawing tells no lie.
	noJunction := noJunctionEndpoints(wires, sc.src)
	in.noJunction = append(in.noJunction, noJunction...)
	// The on-body endpoints also become solver terminals: the endpoint is not "dangling"
	// (it touches something — the wrong way), and wire-no-junction owns the more specific
	// diagnosis. Without this, both rules would report the same point.
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
// Both halves of the T-tap diagnostic come from this one definition, run at two different points in
// the pipeline, which is what keeps them a partition. Run POST-split it yields the silent taps, since
// a junction dot or a mid-span label has already made a joined tap an endpoint of both wires rather
// than an interior point. Run PRE-split it yields every tap, joined or not.
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

// noJunctionEndpoints reports the taps with NOTHING joining them (post-split, sheet frame): the
// silent T-tap the rule reports.
func noJunctionEndpoints(wires []netgraph.Wire, src string) []*ir.DanglingEndpoint {
	var out []*ir.DanglingEndpoint
	for _, t := range endpointsOnBody(wires) {
		out = append(out, &ir.DanglingEndpoint{X: t.At.X, Y: t.At.Y, Prov: tapProv(t.WireId, src)})
	}
	return out
}

// joinedTaps reports the taps that ARE joined (pre-split, sheet frame), naming the construct that
// joined them (agni issue 420). joins maps a point to whatever binds along a wire there; segments
// counts the wire ends meeting at the point once the split has run.
//
// Computed pre-split BECAUSE the split is what erases these: after it a joined tap is an endpoint of
// both wires, so nothing downstream can tell it from a point where no wire ever crossed.
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
			Segments: int32(segments[t.At]),
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
// inside a segment (collinear, between the endpoints). Sheet-frame coordinates only: the
// collinearity cross product needs |coord| well under sqrt(MaxInt64), which sheet
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
// excluding the endpoints — an endpoint already unions by point identity).
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

// unescapeName undoes KiCad's brace escapes in label/net names. KiCad escapes characters
// that collide with its name syntax when WRITING ("/" is the hierarchy separator, so a
// global label "VPP/MCLR" is stored as "VPP{slash}MCLR") and unescapes on load — two
// labels stored with different spellings are ONE net (pic_programmer has exactly this;
// kicad-cli joins them, and so must we). Unknown {tokens} stay literal.
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
// kicadTransform in the rotation only: it uses KiCad's raw angle rather than the render-frame angle
// geomRotation produces. Lib pin coordinates are in KiCad's own (un-flipped) frame, so the rotation
// must be applied in that frame; geomRotation's 360-deg flip is right for 0/180 but reverses
// 90<->270, which swaps a rotated symbol's two pins onto the wrong nets. Origin and mirror are the
// same as kicadTransform.
func pinTransform(ps *node) *geom.Transform {
	t := kicadTransform(ps)
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
				byUnit[u] = append(byUnit[u], pinPos{designator: p.PortRef, loc: p.Loc, etype: atomOf(pn.Arg(1))})
			}
		}
	}
	return byUnit
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

// powerDir maps a lib pin's electrical-type atom to the connection-direction vocabulary,
// for VIRTUAL (power-symbol) pins only: power evidence travels, everything else stays
// empty — a virtual pin must not fabricate signal-direction facts.
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
