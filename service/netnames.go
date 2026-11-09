package service

import (
	"context"

	"google.golang.org/protobuf/proto"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// netNames joins a drawing that names its nets to the design's net ids. A board file and an EDIF
// schematic export both name a net and carry no id, while a finding names its net by id. So a
// drawn net takes the id of the design's one net with its name. A name that two nets share joins
// neither, because the drawing draws one net under that name and cannot say which net it is.
type netNames struct {
	idByName map[string]string
	nameByID map[string]string
	shared   map[string]bool
}

// readNetNames reads the snapshot's netlist through its loader with its options, so the ids match
// the ones the design's findings carry. A design that fails to read joins nothing, since the
// drawing still draws and only the id join is lost.
func (s *DesignService) readNetNames(ctx context.Context, snap Snap) netNames {
	d, err := LoaderOr(snap, s.loader).Design(ctx, snap.Netlist, snap.Overlay.ReadOptions()...)
	if err != nil {
		return netNames{}
	}
	n := netNames{idByName: map[string]string{}, nameByID: map[string]string{}, shared: map[string]bool{}}
	for _, net := range d.GetNets() {
		name, id := net.GetName(), net.GetId()
		if name == "" || id == "" {
			continue
		}
		if prev, ok := n.idByName[name]; ok && prev != id {
			n.shared[name] = true
		}
		n.idByName[name] = id
		n.nameByID[id] = name
	}
	return n
}

// id answers the design's net id for a drawn net name, or "" for a name two nets share or no net
// has.
func (n netNames) id(name string) string {
	if n.shared[name] {
		return ""
	}
	return n.idByName[name]
}

// withWireNetIDs returns the sheet with each named wire keyed by its net's id (agni issue 1443).
// Only a name-only canvas is stamped, such as a companion .eds, whose wires name their nets and
// carry no id. A KiCad sheet's wires carry ids already, and the sheet comes back unchanged. The
// stamped sheet is a copy, because the geometry is a cached read shared by every request.
//
// So a packed wire key carries the id, and the WebGL canvas, which matches a highlight against
// the packed keys in the page, lights a finding's net as the SVG overlay does.
func (s *DesignService) withWireNetIDs(ctx context.Context, snap Snap, g *geom.SchematicGeometry, sheet *geom.SheetGeometry) *geom.SheetGeometry {
	if !nameOnlyCanvas(g) {
		return sheet
	}
	return stampWireNetIDs(sheet, s.readNetNames(ctx, snap))
}

func stampWireNetIDs(sheet *geom.SheetGeometry, names netNames) *geom.SheetGeometry {
	if len(names.idByName) == 0 {
		return sheet
	}
	out := proto.Clone(sheet).(*geom.SheetGeometry)
	for _, w := range out.GetWires() {
		if isBusWire(w) || w.GetNetId() != "" {
			continue
		}
		w.NetId = names.id(w.GetNet())
	}
	return out
}

func isBusWire(w *geom.WireGeometry) bool {
	k := w.GetKind()
	return k == geom.WireGeometry_KIND_BUS || k == geom.WireGeometry_KIND_BUS_ENTRY
}

// sharedNameSpecs keeps a highlight lighting a net whose name another net shares. Such a net's
// wires carry no id after withWireNetIDs, so a spec naming the net by id alone would match nothing.
// It adds the net's name to the spec, as the page's resolveHighlights joins an id-less wire through
// the finding's own name. Both nets sharing the name then light, since the drawing cannot tell them
// apart. A spec with no such id, and every spec on a canvas whose wires carry ids, pass unchanged.
func sharedNameSpecs(names netNames, specs []*geom.HighlightSpec) []*geom.HighlightSpec {
	if len(names.shared) == 0 {
		return specs
	}
	out := make([]*geom.HighlightSpec, len(specs))
	for i, sp := range specs {
		out[i] = sp
		var add []string
		seen := map[string]bool{}
		for _, nm := range sp.GetNets() {
			seen[nm] = true
		}
		for _, id := range sp.GetNetIds() {
			if nm := names.nameByID[id]; nm != "" && names.shared[nm] && !seen[nm] {
				add = append(add, nm)
				seen[nm] = true
			}
		}
		if len(add) > 0 {
			cp := proto.Clone(sp).(*geom.HighlightSpec) // don't mutate the request's specs
			cp.Nets = append(cp.Nets, add...)
			out[i] = cp
		}
	}
	return out
}
