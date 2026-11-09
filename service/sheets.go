package service

import (
	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/geomath"
	"github.com/panyam/agni/internal/netgraph"
)

// NetSource is the slice of the design Model that these annotate helpers read. It gives them the
// netlist's nets for the AttrSheets membership (WS9-028). It is declared at the point of use rather
// than taking the whole check.Model, and check.Model satisfies it structurally (WS1-044). It also
// keeps these helpers off the raw *ir.Design (C19).
type NetSource interface {
	Nets() []*ir.Net
}

// LocateSource is NetSource plus the three questions needed to EXPLAIN a subject that will not
// highlight. Is this ref a real component? Is this name a real net? Is that net a power rail?
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

// sheetIndex maps a finding subject to the sheets it appears on (WS9-024). A component or pin
// subject maps through the ref_des of the sheet's placements. A net subject maps through the net
// names of its wires. The index is built once per check call from the geometry the viewer renders,
// which is the file's default layout. So the ids join the SheetRefs GetDesign returned.
type sheetIndex struct {
	comps map[string][]string // ref_des -> sheet ids, in sheet order
	// pins maps pinKey(ref, pin) to the sheets whose placement of ref draws that pin. A multi-unit
	// part draws each unit on its own sheet, so U1's pin 5 answers U1B's sheet (agni issue 1325).
	pins  map[string][]string
	nets  map[string][]string // net KEY -> sheet ids, in sheet order (a spanning net lists each)
	buses map[string][]string // bus name -> sheet ids where a KIND_BUS wire of that name is drawn (WS7-042c)
	// wireIDs says the geometry stamps a net id on its wires. Only then does a net subject's id have
	// anything to match, so without them a net resolves by its name (agni issue 1230).
	wireIDs bool
	// unreadable says the schematic failed to read, so an entity it would have drawn reads as
	// SCHEMATIC_UNREADABLE rather than NO_GEOMETRY (agni issue 1230).
	unreadable bool
	// board, when set, adds the board sheet after the schematic's for a subject the board draws, so
	// a finding badges the board and a viewer can open it (agni issue 1325, #142's WS7-034).
	board *boardIndex
}

// undrawn returns r, unless the schematic could not be read. Then the run cannot know that a subject
// is drawn nowhere, so NO_GEOMETRY becomes SCHEMATIC_UNREADABLE.
func (ix sheetIndex) undrawn(r checkspb.LocateReason) checkspb.LocateReason {
	if ix.unreadable && r == checkspb.LocateReason_LOCATE_REASON_NO_GEOMETRY {
		return checkspb.LocateReason_LOCATE_REASON_SCHEMATIC_UNREADABLE
	}
	return r
}

// netKey is the per-instance join key for a net. It is the net's deterministic id (ir.Net.id) when
// present, else its name. Keying on the id lets two electrically-distinct nets that share a name
// resolve to their OWN sheets instead of both getting the union (WS9). A pinless net has no id and
// keys by name. The finding's Subject, the wire geometry, and the AttrSheets channel all key
// through this, so they join.
func netKey(id, name string) string {
	if id != "" {
		return id
	}
	return name
}

// indexSheets walks the geometry once. Sheets are visited in order, so each subject's sheet list
// follows the design's sheet order. Several wires of one net on one sheet dedupe to a single entry.
// Sheets are visited one at a time, so comparing against the last appended id is the dedupe check.
//
// The design supplies the AUTHORITATIVE net membership when it has it, through the hierarchy walk's
// AttrSheets (WS9-028). A sub-sheet's wireless single-pin net has no wire geometry to join, so the
// geometry pass alone leaves it badge-less. The netlist attribute overrides the geometry-wire tally
// per net. A net the design does not mark falls back to its wires, as in faithful single-sheet
// exports and formats without the attribute. Components and pins use the geometry alone, since
// placements always exist.
func indexSheets(g *geom.SchematicGeometry, m NetSource) sheetIndex {
	ix := sheetIndex{comps: map[string][]string{}, pins: map[string][]string{}, nets: map[string][]string{}, buses: map[string][]string{}}
	syms := geomath.IndexSymbols(g)
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
			if pl.GetRefDes() == "" || pl.GetNetAnchor() != "" {
				continue
			}
			if sym := syms.SymbolFor(pl); sym != nil {
				for _, pin := range sym.GetPins() {
					appendOnce(ix.pins, pinKey(pl.GetRefDes(), pin.GetPortRef()), sh.GetId())
				}
			}
		}
		for _, w := range sh.GetWires() {
			// A bus (WS7-042c) indexes under its own name-keyed map, gated on the bus kind. So a bus
			// finding gets ITS drawn sheets, or the not-drawn reason if there are none, and never
			// collides with a net that shares the name.
			if w.GetKind() == geom.WireGeometry_KIND_BUS || w.GetKind() == geom.WireGeometry_KIND_BUS_ENTRY {
				appendOnce(ix.buses, w.GetNet(), sh.GetId())
				continue
			}
			// Index a wire under BOTH its name and its per-instance id. The diff panel keys by net
			// name. The findings path supplies the id, so it gets ITS sheets rather than the union
			// of every same-named net. A format without ids indexes by name alone.
			appendOnce(ix.nets, w.GetNet(), sh.GetId())
			if w.GetNetId() != "" {
				appendOnce(ix.nets, w.GetNetId(), sh.GetId())
				ix.wireIDs = true
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

// sheetsFor resolves one subject. Nets resolve by name, and components and pins by ref_des. A pin
// subject's ref is its component's ref_des, so it locates through the placement. An unknown subject
// gets nil, which a consumer treats the same as "no geometry".
func (ix sheetIndex) sheetsFor(s *checkspb.Subject) []string {
	got := ix.schematicSheetsFor(s)
	if ix.board.draws(s) {
		got = append(append([]string(nil), got...), boardSheetID)
	}
	return got
}

// schematicSheetsFor is sheetsFor over the schematic alone. A pin answers the sheets that draw its
// unit, and falls back to its part's when no placed symbol names the pin, such as an unresolved one.
func (ix sheetIndex) schematicSheetsFor(s *checkspb.Subject) []string {
	switch s.GetKind() {
	case check.KindPin:
		if got, ok := ix.pins[pinKey(s.GetRef(), s.GetPin())]; ok {
			return got
		}
		return ix.comps[s.GetRef()]
	case check.KindNet:
		if got, ok := ix.nets[netKey(s.GetNetId(), s.GetRef())]; ok || ix.wireIDs {
			return got
		}
		// The geometry names its wires' nets and gives them no id, as in every format but KiCad. So
		// the name is the only key the two sides share. Two nets sharing a name then share their
		// sheets, since the geometry cannot tell them apart.
		return ix.nets[s.GetRef()]
	case check.KindBus:
		return ix.buses[s.GetRef()]
	}
	return ix.comps[s.GetRef()]
}

// locateReasonProto maps a check locate code to its wire enum. The two vocabularies are declared
// separately. Core states netlist facts, and the proto states what a viewer shows. They meet only
// here, so the query path and the findings path translate a code the same way.
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
// caller without either source skips the pass. Geometry supplies component and pin badges, plus net
// badges for formats whose wires carry names. The design supplies the authoritative net membership
// (AttrSheets, WS9-028), which covers the wireless sub-sheet nets that geometry misses. With both nil
// the pass is a no-op, and findings keep empty sheets, which the viewer handles. A net-only channel,
// with the design set and the geometry nil, still annotates net subjects.
func AnnotateSheets(findings []*checkspb.Finding, g *geom.SchematicGeometry, m LocateSource) {
	annotateSheets(findings, g, m, false, nil)
}

// annotateSheets is AnnotateSheets for a run that knows whether its schematic failed to read, and
// that adds the board sheet from board when the run read one.
func annotateSheets(findings []*checkspb.Finding, g *geom.SchematicGeometry, m LocateSource, unreadable bool, board *boardIndex) {
	if !unreadable && g == nil && board == nil && (m == nil || len(m.Nets()) == 0) {
		return
	}
	ix := indexSheets(g, m)
	ix.unreadable = unreadable
	ix.board = board
	ix.annotateFindings(findings, m)
}

// AnnotateRunSheets fills a check run's findings and its verdicts from one walk of the geometry. A
// verdict gets the sheets that draw its subjects, so a viewer opens one before drawing the proof. A
// passing verdict has no finding to borrow them from, and it gets them too (agni issue 1151).
func AnnotateRunSheets(resp *webapi.CheckDesignResponse, g *geom.SchematicGeometry, m LocateSource) {
	annotateRunSheets(resp, g, m, false, nil)
}

func annotateRunSheets(resp *webapi.CheckDesignResponse, g *geom.SchematicGeometry, m LocateSource, unreadable bool, board *boardIndex) {
	if !unreadable && g == nil && board == nil && (m == nil || len(m.Nets()) == 0) {
		return
	}
	ix := indexSheets(g, m)
	ix.unreadable = unreadable
	ix.board = board
	ix.annotateFindings(resp.GetFindings(), m)
	for _, v := range resp.GetVerdicts() {
		v.Sheets = ix.sheetsForAll(v.GetSubjects())
	}
}

// sheetsForAll returns the sheets that draw a tuple of subjects. It lists each subject's sheets in
// tuple order, without repeats, so a clearance verdict between two nets lists the first net's sheets
// first.
func (ix sheetIndex) sheetsForAll(subjects []*checkspb.Subject) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range subjects {
		for _, id := range ix.sheetsFor(s) {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

func (ix sheetIndex) annotateFindings(findings []*checkspb.Finding, m LocateSource) {
	for _, f := range findings {
		f.Sheets = ix.sheetsFor(f.GetSubject())
		if len(f.Sheets) > 0 {
			continue // drawn somewhere, so UNSPECIFIED is correct and the viewer highlights
		}
		f.LocateReason = ix.reasonFor(f.GetSubject(), m)
	}
}

// reasonFor says why a subject that sheetsFor placed nowhere is undrawn.
func (ix sheetIndex) reasonFor(s *checkspb.Subject, m LocateSource) checkspb.LocateReason {
	// Some buses map to no drawn bus, such as a bus_alias, an EDIF array, or a hierarchical bus port
	// with no wire on the shown sheet. Such a bus gets its own reason (WS7-042c). A bus is never in
	// the netlist, so the general classifier would call every one NOT_IN_DESIGN.
	if s.GetKind() == check.KindBus {
		return checkspb.LocateReason_LOCATE_REASON_BUS_NOT_DRAWN
	}
	// With no source there is nothing to ask, so the subject keeps UNSPECIFIED rather than being
	// guessed at.
	if m == nil {
		return checkspb.LocateReason_LOCATE_REASON_UNSPECIFIED
	}
	return ix.undrawn(locateReasonProto(check.LocateReason(m, s.GetKind(), s.GetRef())))
}

// AnnotateTraceSheets fills a trace's per-net and per-endpoint sheet ids, the same way
// AnnotateSheets fills a finding's.
//
// The ids are PER NET rather than one sheet for the answer. When a route crosses three sheets, a
// reader wants to choose which to open. The panel already renders a list of badges for a finding and
// for a query cell (agni issue 657).
//
// Both outcomes are annotated. On a no-route the two endpoints' nets are still drawn somewhere. A
// reader wants that picture once they learn the pins do not join. Empty sheet ids mean the
// net is drawn nowhere, rather than that nobody looked.
func AnnotateTraceSheets(t *webapi.Trace, g *geom.SchematicGeometry, m LocateSource) {
	annotateTraceSheets(t, g, m, nil)
}

// annotateTraceSheets is AnnotateTraceSheets that also locates on board, when set.
func annotateTraceSheets(t *webapi.Trace, g *geom.SchematicGeometry, m LocateSource, board *boardIndex) {
	if t == nil || (g == nil && board == nil && (m == nil || len(m.Nets()) == 0)) {
		return
	}
	ix := indexSheets(g, m)
	ix.board = board
	netSheets := func(name string) []string {
		if name == "" {
			return nil
		}
		return ix.sheetsFor(&checkspb.Subject{Kind: check.KindNet, Ref: name})
	}
	for _, n := range t.GetNets() {
		n.SheetIds = netSheets(n.GetName())
	}
	annotateEnds(ix, netSheets, t.GetFrom(), t.GetTo())
}

// AnnotateWalkSheets fills a walk's per-net sheet ids and its start's, as AnnotateTraceSheets does
// for a trace, and its goal's route nets and pin. A walk started from a part locates by the part.
func AnnotateWalkSheets(w *webapi.Walk, g *geom.SchematicGeometry, m LocateSource) {
	annotateWalkSheets(w, g, m, nil)
}

// annotateWalkSheets is AnnotateWalkSheets that also locates on board, when set.
func annotateWalkSheets(w *webapi.Walk, g *geom.SchematicGeometry, m LocateSource, board *boardIndex) {
	if w == nil || (g == nil && board == nil && (m == nil || len(m.Nets()) == 0)) {
		return
	}
	ix := indexSheets(g, m)
	ix.board = board
	netSheets := func(name string) []string {
		if name == "" {
			return nil
		}
		return ix.sheetsFor(&checkspb.Subject{Kind: check.KindNet, Ref: name})
	}
	for _, n := range w.GetNets() {
		n.GetNet().SheetIds = netSheets(n.GetNet().GetName())
	}
	if goal := w.GetGoal(); goal != nil {
		for _, r := range goal.GetRoutes() {
			for _, n := range r.GetNets() {
				n.SheetIds = netSheets(n.GetName())
			}
		}
		annotateEnds(ix, netSheets, goal.GetTo())
	}
	if f := w.GetFrom(); f != nil && f.GetEndpoint().GetPin() == "" && f.GetEndpoint().GetRefDes() != "" {
		f.SheetIds = ix.sheetsFor(&checkspb.Subject{Kind: check.KindComponent, Ref: f.GetEndpoint().GetRefDes()})
		return
	}
	annotateEnds(ix, netSheets, w.GetFrom())
}

// annotateEnds fills each endpoint's sheet ids.
func annotateEnds(ix sheetIndex, netSheets func(string) []string, ends ...*webapi.TraceEnd) {
	// An endpoint resolves by its PLACEMENT, not by its net. A trace endpoint is a pin on a part, and
	// a reader wants to open the sheet where that part is drawn. The endpoint's net may be drawn on
	// sheets the part does not appear on (agni issue 657). A pin finding uses the same lookup.
	//
	// Some endpoints have a part that is not placed, such as an unresolved symbol. Falling back to the
	// net keeps such an endpoint from going blank when its net is drawn somewhere.
	for _, e := range ends {
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
