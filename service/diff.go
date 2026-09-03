package service

import (
	"context"
	"github.com/panyam/agni/artifact"
	"sort"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/diff"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// sharedPlacementCap bounds the alignment sample (WS9-007): enough shared components to
// judge whether two revisions share a coordinate frame, without shipping a whole placement
// table for a large board. The sample is deterministic (sorted ref_des, first placement per
// side), so the same pair always yields the same evidence.
const sharedPlacementCap = 50

// DesignLoader is the slice of Loader the diff tier needs: the netlist IR for both sides,
// plus each side's default-layout geometry for the sheet-membership annotation (WS9-006 —
// geometry is best-effort there, see annotateDiffSheets). Declared separately so DiffService
// states its dependencies; the server's osLoader (and any Loader) satisfies it.
type DesignLoader interface {
	Design(ctx context.Context, uri artifact.URI, opts ...ReadOption) (*ir.Design, error)
	Geometry(ctx context.Context, uri artifact.URI, layout string, faithfulSymbols bool, opts ...ReadOption) (*geom.SchematicGeometry, error)
}

// DiffService computes the semantic diff between two designs over an injected loader (C13): it
// reads the two sides' netlist IR through the port, runs the pure diff.Designs, and converts
// to the wire form. The diff core stays presentation-free (docs/18); this service owns the
// wire shape, shared with the CLI's `diff --format json` via DiffResponseProto. It knows no
// transport.
type DiffService struct {
	loader DesignLoader
	// projects resolves each side to its project and loads that project's config; nil when this
	// deployment resolves no projects.
	//
	// A diff needs it on the NETLIST reads, not just the geometry ones. Both sides were read with no
	// options at all, so a project whose symbol library did not resolve was compared from two reads
	// that each lose every connection through the affected parts, and a revision that changed such a
	// connection showed no change. `agni diff` reads through readDesign and IS configured, so the two
	// surfaces answered the same question differently (agni issue 347, the service-side survivor of
	// agni issue 228).
	projects *ProjectResolver
}

// NewDiffService returns a DiffService backed by the given loader. A nil projects resolver means
// this deployment resolves no projects.
func NewDiffService(loader DesignLoader, projects *ProjectResolver) *DiffService {
	return &DiffService{loader: loader, projects: projects}
}

// readOptions composes one side's per-read config from its project. Each side resolves its own,
// because a diff may span two projects.
func (s *DiffService) readOptions(ctx context.Context, uri artifact.URI) ([]ReadOption, error) {
	ov, err := s.projects.Overlay(ctx, uri, nil, Overlay{}, "")
	if err != nil {
		return nil, err
	}
	return ov.ReadOptions(), nil
}

// DiffDesigns loads both designs' netlist IR and returns the classified report plus the
// highlight maps. Either side failing to load fails the whole call with the classified error
// (unknown mount -> ErrNotFound, escaping path or parse failure -> invalid argument); there
// is no partial diff.
func (s *DiffService) DiffDesigns(ctx context.Context, req *webapi.DiffDesignsRequest) (*webapi.DiffDesignsResponse, error) {
	aURI, err := artifactURI(req.GetAUri())
	if err != nil {
		return nil, err
	}
	bURI, err := artifactURI(req.GetBUri())
	if err != nil {
		return nil, err
	}
	// Each side resolves its OWN config, because a diff may span two projects and the comparison is
	// only meaningful when each revision is read the way its own project declares.
	aOpts, err := s.readOptions(ctx, aURI)
	if err != nil {
		return nil, err
	}
	bOpts, err := s.readOptions(ctx, bURI)
	if err != nil {
		return nil, err
	}
	a, err := s.loader.Design(ctx, aURI, aOpts...)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	b, err := s.loader.Design(ctx, bURI, bOpts...)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	resp := DiffResponseProto(diff.Designs(a, b))
	gA := BuildGeometry(ctx, s.loader, aURI, aOpts...)
	gB := BuildGeometry(ctx, s.loader, bURI, bOpts...)
	// Plain netlist models on purpose (NOT BuildModel): diff runs no rules — the model is used only
	// for per-sheet net annotation (annotateDiffSheets reads Nets()), so the board/params tiers would
	// be dead weight. This is the intentional exception to the WS9-048 full-model rule, not drift.
	annotateDiffSheets(resp, check.NewModel(a), gA, check.NewModel(b), gB)
	annotateSharedPlacements(resp, gA, gB)
	return resp, nil
}

// annotateDiffSheets fills the response's per-side sheet maps (WS9-006) for exactly the keys
// of the status maps, from each side's geometry via the same sheetIndex findings use
// (WS9-024). It is a post-pass over DiffResponseProto's output — the one canonical
// report-to-wire conversion (shared with `agni diff --format json`) stays geometry-free,
// and a caller without geometry (the CLI, a nil side) just gets empty maps. A renamed net is
// keyed under both names in the status map, but each side's geometry only knows its own
// name, so the old name lands in a's map and the new name in b's with no special casing.
//
// Each side also supplies its design as the NetSource so the net channel uses the
// AUTHORITATIVE hierarchy membership (AttrSheets, WS9-028), matching the findings path
// (AnnotateSheets): a sub-sheet's wireless single-pin net has no wire geometry to join, so
// without it that net gets no diff badge or navigation (WS9-027). Components stay
// geometry-only — placements always exist, so that join never had the gap.
func annotateDiffSheets(resp *webapi.DiffDesignsResponse, mA NetSource, gA *geom.SchematicGeometry, mB NetSource, gB *geom.SchematicGeometry) {
	side := func(m NetSource, g *geom.SchematicGeometry) (comps, nets map[string]*webapi.DiffDesignsResponse_SheetIds) {
		if g == nil {
			return nil, nil
		}
		ix := indexSheets(g, m)
		comps = map[string]*webapi.DiffDesignsResponse_SheetIds{}
		for ref := range resp.GetComponentStatus() {
			if ids := ix.comps[ref]; len(ids) > 0 {
				comps[ref] = &webapi.DiffDesignsResponse_SheetIds{Ids: ids}
			}
		}
		nets = map[string]*webapi.DiffDesignsResponse_SheetIds{}
		for name := range resp.GetNetStatus() {
			if ids := ix.nets[name]; len(ids) > 0 {
				nets[name] = &webapi.DiffDesignsResponse_SheetIds{Ids: ids}
			}
		}
		return comps, nets
	}
	resp.ComponentSheetsA, resp.NetSheetsA = side(mA, gA)
	resp.ComponentSheetsB, resp.NetSheetsB = side(mB, gB)
}

// annotateSharedPlacements fills the overlay-alignment sample (WS9-007): components placed
// in BOTH sides' geometry — unchanged ones included, they are the evidence — with each
// side's own sheet id and placement origin. Alignment needs both frames, so a missing
// geometry on either side leaves the sample empty rather than half-filled; the viewer then
// falls back to frame-size evidence alone. The verdict itself is computed client-side: the
// viewer owns sheet pairing (including its positional fallback), so it must own which
// placements are comparable.
func annotateSharedPlacements(resp *webapi.DiffDesignsResponse, gA, gB *geom.SchematicGeometry) {
	if gA == nil || gB == nil {
		return
	}
	first := func(g *geom.SchematicGeometry) map[string]*webapi.DiffDesignsResponse_Placement {
		m := map[string]*webapi.DiffDesignsResponse_Placement{}
		for _, sh := range g.GetSheets() {
			for _, pl := range sh.GetPlacements() {
				ref := pl.GetRefDes()
				if ref == "" {
					continue
				}
				if _, ok := m[ref]; ok {
					continue // first placement wins (multi-section parts place several times)
				}
				o := pl.GetTransform().GetOrigin()
				m[ref] = &webapi.DiffDesignsResponse_Placement{Sheet: sh.GetId(), X: float64(o.GetX()), Y: float64(o.GetY())}
			}
		}
		return m
	}
	pa, pb := first(gA), first(gB)
	shared := make([]string, 0, len(pa))
	for ref := range pa {
		if _, ok := pb[ref]; ok {
			shared = append(shared, ref)
		}
	}
	if len(shared) == 0 {
		return
	}
	sort.Strings(shared)
	if len(shared) > sharedPlacementCap {
		shared = shared[:sharedPlacementCap]
	}
	resp.SharedPlacementsA = map[string]*webapi.DiffDesignsResponse_Placement{}
	resp.SharedPlacementsB = map[string]*webapi.DiffDesignsResponse_Placement{}
	for _, ref := range shared {
		resp.SharedPlacementsA[ref] = pa[ref]
		resp.SharedPlacementsB[ref] = pb[ref]
	}
}

// DiffResponseProto is the one place a diff.Report becomes its webapi wire form, so the RPC
// and the CLI's `diff --format json` share a single shape instead of two that can drift (the
// FindingProto pattern). Alongside the report it derives the highlight maps: ref_des ->
// added|removed|changed and net name -> new|deleted|renamed|hard|soft, with a renamed net
// keyed under BOTH its old and new name so each side of a visual diff joins by the name its
// own geometry carries.
func DiffResponseProto(r *diff.Report) *webapi.DiffDesignsResponse {
	rep := &webapi.DiffReport{
		ComponentsAdded:   r.ComponentsAdded,
		ComponentsRemoved: r.ComponentsRemoved,
	}
	comp := map[string]string{}
	for _, ref := range r.ComponentsAdded {
		comp[ref] = "added"
	}
	for _, ref := range r.ComponentsRemoved {
		comp[ref] = "removed"
	}
	for _, c := range r.ComponentsChanged {
		rep.ComponentsChanged = append(rep.ComponentsChanged, &webapi.DiffReport_ComponentChange{
			RefDes: c.RefDes, Field: c.Field, Old: c.Old, New: c.New,
		})
		comp[c.RefDes] = "changed"
	}
	nets := map[string]string{}
	for _, nc := range r.Nets {
		rep.Nets = append(rep.Nets, &webapi.DiffReport_NetChange{
			Kind:    string(nc.Kind),
			Name:    nc.Name,
			OldName: nc.OldName,
			Added:   nc.Added,
			Removed: nc.Removed,
			OldProv: nc.OldProv,
			NewProv: nc.NewProv,
			Approx:  renameEvidenceProto(nc.Approx),
		})
		nets[nc.Name] = string(nc.Kind)
		// BOTH rename kinds carry an old name, and the status map is what a viewer joins to the OLD
		// design's geometry. Registering only the exact kind here left an approximate rename invisible
		// on the old side, which is the side a reader checks when deciding whether to believe it.
		if nc.Kind == diff.NetRenamed || nc.Kind == diff.NetRenamedApprox {
			nets[nc.OldName] = string(nc.Kind)
		}
	}
	return &webapi.DiffDesignsResponse{Report: rep, ComponentStatus: comp, NetStatus: nets}
}

// renameEvidenceProto carries a near-match pairing's arithmetic onto the wire, nil-safe because
// every other net-change kind has none.
func renameEvidenceProto(e *diff.RenameEvidence) *webapi.DiffReport_RenameEvidence {
	if e == nil {
		return nil
	}
	return &webapi.DiffReport_RenameEvidence{
		OldCoverage:            e.OldCoverage,
		OldCoverageSignificant: e.OldCoverageSignificant,
		NewCoverageSignificant: e.NewCoverageSignificant,
		Overlap:                int32(e.Overlap),
		OverlapSignificant:     int32(e.OverlapSignificant),
		OldEndpoints:           int32(e.OldEndpoints),
		NewEndpoints:           int32(e.NewEndpoints),
		OldSignificant:         int32(e.OldSignificant),
		NewSignificant:         int32(e.NewSignificant),
	}
}
