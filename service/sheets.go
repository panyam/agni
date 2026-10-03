package service

import (
	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/netgraph"
)

// NetSource is the slice of the design Model these annotate helpers read, the netlist's nets for
// the AttrSheets membership (WS9-028). Declared at the point of use rather than taking the whole
// check.Model, which satisfies it structurally (WS1-044). It also keeps these helpers off the raw
// *ir.Design (C19).
type NetSource interface {
	Nets() []*ir.Net
}

// LocateSource is NetSource plus the three questions needed to EXPLAIN a subject that will not
// highlight: is this ref a real component, is this name a real net, and is that net a power rail.
//
// Required rather than probed for with a type assertion, so a caller that cannot answer them fails
// to compile. LOCATE_REASON_UNSPECIFIED means "the entity IS drawn and should highlight", so
// leaving it on an undrawn subject tells the viewer to say nothing.
//
// check.Model satisfies it, and both production callers pass one.
type LocateSource interface {
	NetSource
	check.LocateModel
}

// sheetIndex maps a finding subject to the sheets it appears on (WS9-024): a component or pin
// subject through the ref_des of the sheet's placements, a net subject through the net names of
// its wires. Built once per check call from the geometry the viewer renders (the file's default
// layout), so the ids join the SheetRefs GetDesign returned.
type sheetIndex struct {
	comps map[string][]string // ref_des -> sheet ids, in sheet order
	nets  map[string][]string // net KEY -> sheet ids, in sheet order (a spanning net lists each)
	buses map[string][]string // bus name -> sheet ids where a KIND_BUS wire of that name is drawn (WS7-042c)
}

// netKey is the per-instance join key for a net: its deterministic id (ir.Net.id) when present,
// else its name. Keying on the id lets two electrically-distinct nets that share a name resolve to
// their OWN sheets instead of both getting the union (WS9). A pinless net has no id and keys by
// name. The finding's Subject, the wire geometry, and the AttrSheets channel all key through this,
// so they join.
func netKey(id, name string) string {
	if id != "" {
		return id
	}
	return name
}

// indexSheets walks the geometry once. Sheets are visited in order, so each subject's sheet list
// comes out in the design's sheet order; multiple wires of one net on one sheet dedupe to a
// single entry (sheets are visited one at a time, so "last appended id" is the dedupe check).
//
// The design supplies the AUTHORITATIVE net membership when it has it (the hierarchy walk's
// AttrSheets, WS9-028), because a sub-sheet's wireless single-pin net has no wire geometry to join
// and the geometry pass alone leaves it badge-less. The netlist attribute overrides the
// geometry-wire tally per net, and a net the design does not mark falls back to its wires (faithful
// single-sheet exports, formats without the attribute). Components and pins stay geometry-only,
// since placements always exist.
func indexSheets(g *geom.SchematicGeometry, m NetSource) sheetIndex {
	ix := sheetIndex{comps: map[string][]string{}, nets: map[string][]string{}, buses: map[string][]string{}}
	appendOnce := func(m map[string][]string, key, sheetID string) {
		if key == "" {
			return
		}
		if got := m[key]; len(got) > 0 && got[len(got)-1] == sheetID {
			return
		}
		m[key] = append(m[key], sheetID)
	}
	for _, sh := range g.GetSheets() {
		for _, pl := range sh.GetPlacements() {
			appendOnce(ix.comps, pl.GetRefDes(), sh.GetId())
		}
		for _, w := range sh.GetWires() {
			// A bus (WS7-042c) indexes under its own name-keyed map, gated on the bus kind, so a bus
			// finding gets ITS drawn sheets (and, if none, the not-drawn reason) without colliding
			// with a net that shares the name.
			if w.GetKind() == geom.WireGeometry_KIND_BUS || w.GetKind() == geom.WireGeometry_KIND_BUS_ENTRY {
				appendOnce(ix.buses, w.GetNet(), sh.GetId())
				continue
			}
			// Index a wire under BOTH its name (the diff panel keys by net name) and its per-instance
			// id (the findings path, which supplies the id to get ITS sheets rather than the union of
			// every same-named net). A format without ids indexes by name alone.
			appendOnce(ix.nets, w.GetNet(), sh.GetId())
			if w.GetNetId() != "" {
				appendOnce(ix.nets, w.GetNetId(), sh.GetId())
			}
		}
	}
	// A nil source is the geometry-only path, with no netlist AttrSheets channel. Guarding the
	// interface avoids a nil-method call.
	if m != nil {
		for _, n := range m.Nets() {
			if ids := netgraph.ParseSheets(n.GetAttributes()[netgraph.AttrSheets]); len(ids) > 0 {
				ix.nets[n.GetName()] = ids // name key, for the diff panel's name-based lookup
				if id := n.GetId(); id != "" {
					ix.nets[id] = ids // id key, for the finding's per-instance lookup
				}
			}
		}
	}
	return ix
}

// sheetsFor resolves one subject: nets by name, components and pins by ref_des (a pin subject's
// ref is its component's ref_des, so it locates through the placement). An unknown subject gets
// nil, which a consumer treats the same as "no geometry".
func (ix sheetIndex) sheetsFor(s *checkspb.Subject) []string {
	switch s.GetKind() {
	case check.KindNet:
		return ix.nets[netKey(s.GetNetId(), s.GetRef())]
	case check.KindBus:
		return ix.buses[s.GetRef()]
	}
	return ix.comps[s.GetRef()]
}

// locateReasonProto maps a check locate code to its wire enum. The two vocabularies are declared
// separately (core states netlist facts, the proto states what a viewer shows), and this is the one
// place they meet, so the query path and the findings path translate a code the same way.
func locateReasonProto(code string) checkspb.LocateReason {
	switch code {
	case check.LocateVirtual:
		return checkspb.LocateReason_LOCATE_REASON_VIRTUAL_SYMBOL
	case check.LocatePowerRail:
		return checkspb.LocateReason_LOCATE_REASON_POWER_RAIL_NO_WIRE
	case check.LocateNotInDesign:
		return checkspb.LocateReason_LOCATE_REASON_NOT_IN_DESIGN
	default:
		return checkspb.LocateReason_LOCATE_REASON_NO_GEOMETRY
	}
}

// AnnotateSheets fills each finding's sheets in place. It is a post-pass over FindingProto's
// output rather than a FindingProto parameter, so the canonical conversion keeps its shape and a
// caller without either source skips the pass. Geometry supplies component and pin badges (and net
// badges for formats whose wires carry names), and the design supplies the authoritative net
// membership (AttrSheets, WS9-028) that covers the wireless sub-sheet nets geometry misses. Both nil
// is a no-op and findings keep empty sheets, which the viewer handles. A net-only channel (design
// set, geometry nil) still annotates net subjects.
func AnnotateSheets(findings []*checkspb.Finding, g *geom.SchematicGeometry, m LocateSource) {
	if g == nil && (m == nil || len(m.Nets()) == 0) {
		return
	}
	ix := indexSheets(g, m)
	for _, f := range findings {
		f.Sheets = ix.sheetsFor(f.GetSubject())
		if len(f.Sheets) > 0 {
			continue // drawn somewhere, so UNSPECIFIED is correct and the viewer highlights
		}
		// Nothing to locate, so say WHY. A bus that maps to no drawn bus (a bus_alias, an EDIF array,
		// a hierarchical bus port with no wire on the shown sheet) gets its own reason (WS7-042c),
		// because a bus is never in the netlist and the general classifier would call every one
		// NOT_IN_DESIGN.
		if f.GetSubject().GetKind() == check.KindBus {
			f.LocateReason = checkspb.LocateReason_LOCATE_REASON_BUS_NOT_DRAWN
			continue
		}
		// With no source there is nothing to ask, so the subject stays UNSPECIFIED rather than being
		// guessed at.
		if m == nil {
			continue
		}
		f.LocateReason = locateReasonProto(check.LocateReason(m, f.GetSubject().GetKind(), f.GetSubject().GetRef()))
	}
}

// AnnotateTraceSheets fills a trace's per-net and per-endpoint sheet ids, the same way
// AnnotateSheets fills a finding's.
//
// PER NET rather than one sheet for the answer, because a route crossing three sheets is when a
// reader wants to choose which to open, and the panel already renders a list of badges for a
// finding and for a query cell (agni issue 657).
//
// Both outcomes are annotated. On a no-route the two endpoints' nets are still drawn somewhere, and
// that is the picture a reader wants once they learn the pins do not join. Empty sheet ids mean the
// net is drawn nowhere, rather than that nobody looked.
func AnnotateTraceSheets(t *webapi.Trace, g *geom.SchematicGeometry, m LocateSource) {
	if t == nil || (g == nil && (m == nil || len(m.Nets()) == 0)) {
		return
	}
	ix := indexSheets(g, m)
	netSheets := func(name string) []string {
		if name == "" {
			return nil
		}
		return ix.sheetsFor(&checkspb.Subject{Kind: check.KindNet, Ref: name})
	}
	for _, n := range t.GetNets() {
		n.SheetIds = netSheets(n.GetName())
	}
	// An endpoint resolves by its PLACEMENT, not by its net. A trace endpoint is a pin on a part, and
	// where that part is drawn is what a reader wants to open. The net it sits on may be drawn on
	// sheets the part does not appear on (agni issue 657). This is the same lookup a pin finding uses.
	//
	// Falling back to the net keeps an endpoint whose part is not placed (an unresolved symbol) from
	// going blank when its net is drawn somewhere.
	for _, e := range []*webapi.TraceEnd{t.GetFrom(), t.GetTo()} {
		if e == nil {
			continue
		}
		ep := e.GetEndpoint()
		e.SheetIds = ix.sheetsFor(&checkspb.Subject{Kind: check.KindPin, Ref: ep.GetRefDes(), Pin: ep.GetPin()})
		if len(e.SheetIds) == 0 {
			e.SheetIds = netSheets(e.GetNet())
		}
	}
}
