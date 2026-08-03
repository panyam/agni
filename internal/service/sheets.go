package service

import (
	"github.com/panyam/agni/core/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/netgraph"
)

// NetSource is the slice of the design Model these annotate helpers actually read: the netlist's
// nets, for the AttrSheets membership (WS9-028). Declared here, at the point of use, rather than
// taking the whole check.Model — a consumer depends only on what it needs, and any Model satisfies
// it structurally (WS1-044). It also keeps these helpers off the raw *ir.Design (C19).
type NetSource interface {
	Nets() []*ir.Net
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
// else its name. Keying on the id, not the name, is what lets two electrically-distinct nets that
// share a name resolve to their OWN sheets instead of both getting the union — the WS9 de-inflation.
// A pinless net has no id and falls back to name (its old behavior). The finding's Subject, the wire
// geometry, and the AttrSheets channel all key through this, so they join.
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
// AttrSheets, WS9-028): a sub-sheet's wireless single-pin net has no wire geometry to join, so
// the geometry pass alone leaves it badge-less. The netlist attribute overrides the geometry-wire
// tally per net; a net the design does not mark falls back to its wires (faithful single-sheet
// exports, formats without the attribute). Components and pins stay geometry-only — placements
// always exist, so that join never had the wireless gap.
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
			// Index a wire under BOTH its name (the name-based callers: the diff panel keys by net
			// name) and its per-instance id (the findings path, which supplies the id to get ITS
			// sheets, not the union of every same-named net). netKey collapses to the name when
			// there is no id, so a format without ids indexes exactly as before.
			appendOnce(ix.nets, w.GetNet(), sh.GetId())
			if w.GetNetId() != "" {
				appendOnce(ix.nets, w.GetNetId(), sh.GetId())
			}
		}
	}
	// A nil source is the geometry-only path (annotateDiffSheets passes it): the netlist
	// AttrSheets channel is simply absent, same as the old nil-*ir.Design whose getter returned
	// no nets. Guarding the interface avoids a nil-method call.
	if m != nil {
		for _, n := range m.Nets() {
			if ids := netgraph.ParseSheets(n.GetAttributes()[netgraph.AttrSheets]); len(ids) > 0 {
				ix.nets[n.GetName()] = ids // name key: the diff panel's name-based lookup (old behavior)
				if id := n.GetId(); id != "" {
					ix.nets[id] = ids // id key: the finding's per-instance lookup
				}
			}
		}
	}
	return ix
}

// sheetsFor resolves one subject: nets by name, components and pins by ref_des (a pin subject's
// ref is its component's ref_des, so it locates through the placement). An unknown subject gets
// nil, which a consumer treats the same as "no geometry".
func (ix sheetIndex) sheetsFor(s *webapi.Subject) []string {
	switch s.GetKind() {
	case check.KindNet:
		return ix.nets[netKey(s.GetNetId(), s.GetRef())]
	case check.KindBus:
		return ix.buses[s.GetRef()]
	}
	return ix.comps[s.GetRef()]
}

// AnnotateSheets fills each finding's sheets in place. It is a post-pass over FindingProto's
// output rather than a FindingProto parameter, so the one canonical conversion (shared with the
// CLI's `check --format json`, which calls this with nil geometry for the net channel alone)
// keeps its shape and a caller without either source skips the pass. Geometry supplies
// component/pin badges (and net badges for formats whose wires carry names); the design supplies
// the authoritative net membership (AttrSheets, WS9-028) that covers the wireless sub-sheet nets
// geometry misses. Both nil is a no-op (findings keep empty sheets — the pre-WS9-024 behavior the
// viewer already handles); a net-only channel (design set, geometry nil) still annotates net
// subjects.
func AnnotateSheets(findings []*webapi.Finding, g *geom.SchematicGeometry, m NetSource) {
	if g == nil && len(m.Nets()) == 0 {
		return
	}
	ix := indexSheets(g, m)
	for _, f := range findings {
		f.Sheets = ix.sheetsFor(f.GetSubject())
		// A bus finding that maps to no drawn bus (a bus_alias, an EDIF array, a hierarchical bus
		// port with no wire on the shown sheet) has nothing to highlight; flag WHY so the viewer can
		// say so instead of silently doing nothing (WS7-042c). A drawn bus keeps its sheets and the
		// default UNSPECIFIED reason, so it highlights as before.
		if f.GetSubject().GetKind() == check.KindBus && len(f.Sheets) == 0 {
			f.LocateReason = webapi.LocateReason_LOCATE_REASON_BUS_NOT_DRAWN
		}
	}
}
