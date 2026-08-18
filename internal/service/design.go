package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/panyam/agni/internal/artifact"

	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/expect"
	"github.com/panyam/agni/readers/formats"
)

// faithfulLayout is the layout name for an ingested faithful geometry (vs an auto-layout),
// shared with the CLI via the formats registry.
const faithfulLayout = formats.LayoutFaithful

// Error sentinels the implementations classify every failure with, so a transport adapter
// (internal/server) maps them to its codes without knowing which port an error came from:
//
//	ErrNotFound        -> not found (unknown mount, absent resource, unknown sheet)
//	ErrInvalidPath     -> invalid argument (containment violation; declared in workspace.go)
//	ErrInvalidArgument -> invalid argument (unloadable design, unsupported operation)
//	ErrNative*         -> the native-render gate codes (declared below)
//	ErrInternal        -> internal (an unexpected failure in an otherwise-gated path)
//
// An unclassified error reaching a transport is treated as an invalid argument, the common
// case for load/parse failures.
var (
	ErrNotFound        = errors.New("not found")
	ErrInvalidArgument = errors.New("invalid argument")
	ErrInternal        = errors.New("internal error")
)

// Loader materializes a design's read model from a (mount, path). The server adapter (fsLoader)
// is os-backed; a future WASM adapter is a seededLoader singleton over one already-materialized
// Design. All file I/O lives in the adapter, never here (CONSTRAINTS C13).
type Loader interface {
	// Design returns the netlist IR (for counts and checks). A geometry-only file has none and
	// returns an error the caller treats as "no netlist".
	Design(ctx context.Context, uri artifact.URI, opts ...ReadOption) (*ir.Design, error)
	// Geometry resolves drawable geometry for the layout and symbol source (the design's own
	// symbols when faithfulSymbols, else synthetic glyphs).
	//
	// It takes ReadOptions for the same reason Design does, and the omission was a real bug (agni
	// issue 347). A symbol library is config that changes what the read CONTAINS: an unresolved
	// symbol contributes no shapes, so the placement is dropped from the document along with the
	// entity keys that make it pickable, while its ref-des annotation still draws. The sheet then
	// looks complete and every component and pin on it is silently unclickable. A signature with no
	// options channel made that unfixable from the call site and the assertion unwritable.
	Geometry(ctx context.Context, uri artifact.URI, layout string, faithfulSymbols bool, opts ...ReadOption) (*geom.SchematicGeometry, error)
	// Report classifies how an auto-layout draws each component (the conversion report).
	// Report explains how each component was drawn. It takes ReadOptions for the same reason
	// Geometry does, and more sharply: this is the surface that would REPORT an unresolved symbol,
	// so a read that could not see the project's library would diagnose a problem it caused itself.
	Report(ctx context.Context, uri artifact.URI, faithfulSymbols bool, opts ...ReadOption) (*graph.ConversionReport, error)
	// Expectations loads a design's `<path>.expect.yaml` sidecar (WS6-006). A design with no sidecar
	// returns a nil map and a nil error (absence is normal), so the caller renders an empty panel
	// rather than an error.
	Expectations(ctx context.Context, uri artifact.URI) (*expect.Expectations, error)
	// Board returns the physical board sidecar (WS1-006) for formats that carry one
	// (.kicad_pcb today). nil with a nil error means the format has none — absence is
	// normal, mirroring Expectations — and the design then simply lists no board sheet.
	Board(ctx context.Context, uri artifact.URI) (*geom.BoardGeometry, error)
}

// boardSheetID is the synthetic sheet id the board renders under (WS7-034). It is a sheet
// so navigation, deep links, the sheet overview, and the highlight RPCs apply unchanged;
// the id is non-numeric on purpose (a numeric selector reads as a positional index).
const boardSheetID = "board"

// boardFor loads the board sidecar for a sheet request, classifying "this file has no
// board" as not-found (the caller asked for a sheet that does not exist).
func (s *DesignService) boardFor(ctx context.Context, uri artifact.URI) (*geom.BoardGeometry, error) {
	b, err := s.loader.Board(ctx, uri)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	if b == nil {
		return nil, fmt.Errorf("%w: no board geometry for this file", ErrNotFound)
	}
	return b, nil
}

// styleOrDefault is the service's theme, falling back so a service constructed without one
// still renders.
func (s *DesignService) styleOrDefault() render.Style {
	if s.style == (render.Style{}) {
		return render.DefaultStyle
	}
	return s.style
}

// Native-render gate failures an adapter reports so a transport maps them without importing
// the cmd tool cache: NoTool -> unimplemented; NotEnabled / NotFound -> failed precondition.
var (
	ErrNativeNoTool     = errors.New("no native renderer for this format")
	ErrNativeNotEnabled = errors.New("native renderer not enabled")
	ErrNativeNotFound   = errors.New("native renderer tool not found")
)

// NativeRenderer renders a page with the format's own golden tool (a shell-out platform effect).
// Available reports whether NATIVE can be offered for a file; Render returns the page's SVG or one
// of the ErrNative* gate errors.
type NativeRenderer interface {
	Available(uri artifact.URI) bool
	Render(ctx context.Context, uri artifact.URI, page int) (string, error)
}

// DesignService loads and renders one design over injected ports (CONSTRAINTS C13): it
// resolves a design's IR/geometry via a Loader, renders golden pages via a NativeRenderer,
// and does the pure select/pack/label itself. It performs no file I/O and knows no transport;
// rule checks are the separate CheckService (checks.go).
type DesignService struct {
	loader Loader
	native NativeRenderer
	// style is the render palette/font applied to both the SVG and packed (WebGL) output. The zero
	// value renders with render.DefaultStyle.
	style render.Style
	// projects resolves a design to its project and loads that project's config; nil when this
	// deployment resolves no projects, and then every design reads under the engine defaults.
	//
	// A RENDER tier needs this for the same reason the rule-running tiers do. A project's declared
	// symbol library decides whether a placement resolves to a body, and an unresolved symbol keeps
	// its reference designator while losing its pins, so the read is missing connections rather than
	// just artwork (agni issue 347).
	projects *ProjectResolver
}

// NewDesignService returns a DesignService backed by the given ports and render style. A nil
// projects resolver means this deployment resolves no projects.
func NewDesignService(loader Loader, native NativeRenderer, style render.Style, projects *ProjectResolver) *DesignService {
	return &DesignService{loader: loader, native: native, style: style, projects: projects}
}

// readOptions composes the per-read config this design's project supplies: its naming vocabulary and
// the symbol libraries it declares. A design belonging to no project yields no options, which is the
// ordinary loose-file case; a descriptor that exists and does not parse is returned, because reading
// under the defaults instead would answer a different question without saying so.
//
// It passes no request overlay: the four design surfaces carry no OverlayConfig on the wire, so the
// project's own config is the whole of what applies.
func (s *DesignService) readOptions(ctx context.Context, uri artifact.URI) ([]ReadOption, error) {
	ov, err := s.projects.Overlay(ctx, uri, nil, Overlay{}, "")
	if err != nil {
		return nil, err
	}
	return ov.ReadOptions(), nil
}

// classifyLoadErr keeps an already-classified loader error (unknown mount, containment) and
// wraps anything else — a resolve/parse failure — as an invalid argument, preserving the
// pre-split transport mapping.
func classifyLoadErr(err error) error {
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidPath) {
		return err
	}
	return fmt.Errorf("%w: %s", ErrInvalidArgument, err)
}

// availableLayouts lists the layouts a file can render: "faithful" when it carries ingested
// geometry, then each auto-layout strategy when it has a netlist IR. Derived from the extension.
func availableLayouts(path string) []string {
	var out []string
	if formats.HasFaithful(path) {
		out = append(out, faithfulLayout)
	}
	if formats.HasNetlist(path) {
		for _, s := range graph.Strategies() {
			out = append(out, s.Name)
		}
	}
	return out
}

// layoutForFile picks the effective layout: the requested one if available for the file, else the
// default (faithful when the file carries geometry, otherwise the default auto-layout).
func layoutForFile(path, requested string) string {
	if requested != "" {
		for _, l := range availableLayouts(path) {
			if l == requested {
				return requested
			}
		}
	}
	if formats.HasFaithful(path) {
		return faithfulLayout
	}
	return graph.DefaultStrategy
}

// GetDesign loads a file, resolves its geometry, and returns the design summary plus the drawable
// sheets. Netlist formats also carry IR counts; a geometry-only .eds does not, so its name comes
// from the geometry's design ref and its counts stay zero.
func (s *DesignService) GetDesign(ctx context.Context, req *webapi.GetDesignRequest) (*webapi.GetDesignResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	layout := layoutForFile(u.Path, req.GetLayout())
	opts, err := s.readOptions(ctx, u)
	if err != nil {
		return nil, err
	}
	g, err := s.loader.Geometry(ctx, u, layout, false, opts...)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	resp := &webapi.GetDesignResponse{
		Layout:           layout,
		NativeAvailable:  s.native.Available(u),
		AvailableLayouts: availableLayouts(u.Path),
	}
	for _, sh := range g.GetSheets() {
		resp.Sheets = append(resp.Sheets, &webapi.SheetRef{Id: sh.GetId(), Name: sh.GetName(), ParentId: sh.GetParentId()})
	}
	// A file with a board sidecar lists the physical board as one more sheet, after the
	// drawable ones, regardless of the layout axis (the board is faithful by nature).
	if b, err := s.loader.Board(ctx, u); err == nil && b != nil {
		resp.Sheets = append(resp.Sheets, &webapi.SheetRef{Id: boardSheetID, Name: "Board"})
	}
	if layout == faithfulLayout {
		resp.Name = g.GetDesignRef()
	} else if d, err := s.loader.Design(ctx, u, opts...); err == nil {
		resp.Name = d.GetName()
		resp.SourceFormat = d.GetSourceFormat()
		resp.ComponentCount = int32(len(d.GetComponents()))
		resp.NetCount = int32(len(d.GetNets()))
	}
	return resp, nil
}

// GetLayoutReport explains how an auto-layout would draw each component under the requested symbol
// source. A resolve error (unknown mount / escaping path) is returned; a file with no netlist has
// nothing to classify, so that returns an empty report rather than an error.
func (s *DesignService) GetLayoutReport(ctx context.Context, req *webapi.GetLayoutReportRequest) (*webapi.GetLayoutReportResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	faithful := req.GetSymbols() == webapi.SymbolSource_SYMBOL_SOURCE_FAITHFUL
	opts, err := s.readOptions(ctx, u)
	if err != nil {
		return nil, err
	}
	rep, err := s.loader.Report(ctx, u, faithful, opts...)
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidPath) {
			return nil, err
		}
		// A build failure (e.g. no netlist to classify) yields an empty report, not an error.
		return &webapi.GetLayoutReportResponse{Report: &webapi.ConversionReport{}}, nil
	}
	return &webapi.GetLayoutReportResponse{Report: reportToProto(rep)}, nil
}

// reportToProto converts the engine's conversion report to the wire message.
func reportToProto(r *graph.ConversionReport) *webapi.ConversionReport {
	out := &webapi.ConversionReport{}
	for _, c := range r.Components {
		out.Components = append(out.Components, &webapi.ComponentReport{
			RefDes:      c.RefDes,
			Symbol:      c.Symbol,
			DeviceClass: c.Class,
			Cell:        c.Cell,
			Kind:        c.Kind,
		})
	}
	return out
}

// GetSheet resolves the file's geometry, selects one sheet (by id, name, or 0-based index; empty
// selects the first), and renders it in the requested format: PACKED (tier-2 geometry for the
// WebGL viewer, the default), SVG (the render.SheetSVG reference), or NATIVE (the format's own
// tool). A bad selector classifies as ErrNotFound; an unexpected native-render failure (past the
// ErrNative* gates) as ErrInternal.
func (s *DesignService) GetSheet(ctx context.Context, req *webapi.GetSheetRequest) (*webapi.GetSheetResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	// The synthetic board sheet renders from the board sidecar, not the schematic geometry:
	// SVG from BoardSVG, PACKED (and UNSPECIFIED) from PackBoard — the WS7-035 tier, same
	// envelope as the schematic pack so the canvas draws it with one extra draw mode.
	// NATIVE has no board pages; it answers with the SVG document.
	if req.GetSheet() == boardSheetID {
		b, err := s.boardFor(ctx, u)
		if err != nil {
			return nil, err
		}
		if f := req.GetFormat(); f == webapi.SheetFormat_SHEET_FORMAT_SVG || f == webapi.SheetFormat_SHEET_FORMAT_NATIVE {
			return &webapi.GetSheetResponse{
				Content: &webapi.GetSheetResponse_Svg{Svg: render.BoardSVG(b, render.WithStyle(s.styleOrDefault()))},
			}, nil
		}
		return &webapi.GetSheetResponse{
			Content: &webapi.GetSheetResponse_Packed{Packed: render.PackBoard(b, render.WithStyle(s.styleOrDefault()))},
		}, nil
	}
	// Native renders the tool's own faithful pages, so its sheet selection comes from the faithful
	// geometry regardless of the requested layout; other formats honor the request.
	requested := req.GetLayout()
	if req.GetFormat() == webapi.SheetFormat_SHEET_FORMAT_NATIVE {
		requested = faithfulLayout
	}
	faithful := req.GetSymbols() == webapi.SymbolSource_SYMBOL_SOURCE_FAITHFUL
	opts, err := s.readOptions(ctx, u)
	if err != nil {
		return nil, err
	}
	g, err := s.loader.Geometry(ctx, u, layoutForFile(u.Path, requested), faithful, opts...)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	sel := req.GetSheet()
	if sel == "" {
		sel = "0"
	}
	sheet, err := render.PickSheet(g, sel)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, err)
	}

	style := s.style
	if style == (render.Style{}) {
		style = render.DefaultStyle // a service constructed without a theme still renders
	}
	resp := &webapi.GetSheetResponse{}
	switch req.GetFormat() {
	case webapi.SheetFormat_SHEET_FORMAT_SVG:
		// The served viewer is the one consumer that PICKS, so it is the one that asks for the
		// per-pin pick targets. `agni render` writing a file does not, and neither does a report
		// embedding a sheet: they would carry an invisible element per pin for an interaction that
		// never happens there (render.Style.PickTargets).
		resp.Content = &webapi.GetSheetResponse_Svg{Svg: render.SheetSVG(g, sheet, render.WithStyle(style), render.WithPickTargets())}
	case webapi.SheetFormat_SHEET_FORMAT_NATIVE:
		svg, err := s.native.Render(ctx, u, render.SheetIndex(g, sheet)+1)
		if err != nil {
			if errors.Is(err, ErrNativeNoTool) || errors.Is(err, ErrNativeNotEnabled) || errors.Is(err, ErrNativeNotFound) {
				return nil, err
			}
			return nil, fmt.Errorf("%w: %s", ErrInternal, err)
		}
		resp.Content = &webapi.GetSheetResponse_Svg{Svg: svg}
	default: // UNSPECIFIED or PACKED
		resp.Content = &webapi.GetSheetResponse_Packed{Packed: render.PackSheet(g, sheet, render.WithStyle(style))}
	}
	return resp, nil
}

// HighlightSheet resolves highlight specs against one rendered sheet and returns the drawable
// overlay layer, decoupled from the base render: PACKED yields primitive-index groups joining
// the GetSheet PackedSheet by index, SVG yields a transparent overlay document framed exactly
// like the GetSheet SVG. The sheet/layout/symbols selection mirrors GetSheet so the overlay
// describes the same geometry as the base render it stacks on. NATIVE (a golden shell-out with
// no overlay concept) classifies as ErrInvalidArgument.
func (s *DesignService) HighlightSheet(ctx context.Context, req *webapi.HighlightSheetRequest) (*webapi.HighlightSheetResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	if req.GetFormat() == webapi.SheetFormat_SHEET_FORMAT_NATIVE {
		return nil, fmt.Errorf("%w: no highlight overlay for the NATIVE format", ErrInvalidArgument)
	}
	// The board sheet's overlay comes from the board join (net -> copper, ref_des -> pads):
	// a transparent SVG framed exactly like the BoardSVG base, or primitive-index groups
	// over the SAME PackBoard primitive table the PACKED GetSheet returned.
	if req.GetSheet() == boardSheetID {
		b, err := s.boardFor(ctx, u)
		if err != nil {
			return nil, err
		}
		if req.GetFormat() == webapi.SheetFormat_SHEET_FORMAT_SVG {
			return &webapi.HighlightSheetResponse{
				Content: &webapi.HighlightSheetResponse_Svg{Svg: render.HighlightBoardSVG(b, req.GetSpecs())},
			}, nil
		}
		return &webapi.HighlightSheetResponse{
			Content: &webapi.HighlightSheetResponse_Packed{Packed: render.HighlightPacked(render.PackBoard(b), req.GetSpecs())},
		}, nil
	}
	faithful := req.GetSymbols() == webapi.SymbolSource_SYMBOL_SOURCE_FAITHFUL
	opts, err := s.readOptions(ctx, u)
	if err != nil {
		return nil, err
	}
	g, err := s.loader.Geometry(ctx, u, layoutForFile(u.Path, req.GetLayout()), faithful, opts...)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	sel := req.GetSheet()
	if sel == "" {
		sel = "0"
	}
	sheet, err := render.PickSheet(g, sel)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, err)
	}

	// On a NAME-ONLY canvas (a faithful .eds / WS1-047 companion schematic: wires named by net but
	// carrying no per-instance net_id), a net spec that targets a net by id alone cannot match, so
	// resolve those ids to their net NAMES via the netlist and add them for a name-join.
	specs := s.nameJoinSpecs(ctx, u, g, req.GetSpecs(), opts...)

	resp := &webapi.HighlightSheetResponse{}
	if req.GetFormat() == webapi.SheetFormat_SHEET_FORMAT_SVG {
		resp.Content = &webapi.HighlightSheetResponse_Svg{Svg: render.HighlightSVG(g, sheet, specs)}
	} else { // UNSPECIFIED or PACKED
		// Style does not affect packing indices/keys, so the default-style pack joins the
		// styled GetSheet pack exactly (same primitive table).
		packed := render.PackSheet(g, sheet)
		resp.Content = &webapi.HighlightSheetResponse_Packed{Packed: render.HighlightPacked(packed, specs)}
	}
	return resp, nil
}

// nameJoinSpecs adapts highlight specs for a NAME-ONLY geometry canvas (a faithful .eds / WS1-047
// companion schematic). Such a canvas names its wires by net but carries no per-instance net_id —
// the net_id hashes ref-des, which the schematic under-annotates, so it can never match the
// netlist's id. A spec that targets a net by id alone therefore matches nothing there. This resolves
// each spec's net_id to its net NAME (from the netlist at path) and adds it, so the match lands by
// name. It is a NO-OP on an id-capable canvas (nameOnlyCanvas is false) and when no spec carries a
// bare net_id, so the primary-canvas per-instance precision and the goldens are untouched.
func (s *DesignService) nameJoinSpecs(ctx context.Context, uri artifact.URI, g *geom.SchematicGeometry, specs []*geom.HighlightSpec, opts ...ReadOption) []*geom.HighlightSpec {
	if !nameOnlyCanvas(g) || !anyNetIDSpec(specs) {
		return specs
	}
	// The caller's already-resolved options rather than a second resolution: this read must see the
	// same config as the geometry it is joining against, or an id would resolve to a name the drawing
	// does not carry.
	d, err := s.loader.Design(ctx, uri, opts...)
	if err != nil {
		return specs // cannot resolve the netlist: leave specs as-is, no worse than before
	}
	idToName := map[string]string{}
	for _, n := range d.GetNets() {
		if id := n.GetId(); id != "" {
			idToName[id] = n.GetName()
		}
	}
	out := make([]*geom.HighlightSpec, len(specs))
	for i, sp := range specs {
		if len(sp.GetNetIds()) == 0 {
			out[i] = sp
			continue
		}
		seen := map[string]bool{}
		for _, nm := range sp.GetNets() {
			seen[nm] = true
		}
		cp := proto.Clone(sp).(*geom.HighlightSpec) // don't mutate the request's specs
		for _, id := range sp.GetNetIds() {
			if nm := idToName[id]; nm != "" && !seen[nm] {
				cp.Nets = append(cp.Nets, nm)
				seen[nm] = true
			}
		}
		out[i] = cp
	}
	return out
}

// nameOnlyCanvas reports whether a geometry names its wires by net but carries no per-instance
// net_id — the faithful .eds / companion case where a net highlight must join by name, not id.
func nameOnlyCanvas(g *geom.SchematicGeometry) bool {
	named := false
	for _, sh := range g.GetSheets() {
		for _, w := range sh.GetWires() {
			if w.GetNetId() != "" {
				return false // an id-capable canvas: id-join works, leave it alone
			}
			if w.GetNet() != "" {
				named = true
			}
		}
	}
	return named
}

// anyNetIDSpec reports whether any spec targets a net by id (so a name resolution is worth doing).
func anyNetIDSpec(specs []*geom.HighlightSpec) bool {
	for _, sp := range specs {
		if len(sp.GetNetIds()) > 0 {
			return true
		}
	}
	return false
}

// artifactURI parses a request's artifact URI, classifying a malformed one for the transport.
//
// Every rpc that names an artifact funnels through here, which is what makes containment a property
// of the type rather than a step somebody remembers: a parsed URI cannot name a location outside the
// mount it claims, so the adapters below the ports stopped re-checking it. Before this there were 26
// separate containment checks and any new adapter had to know to add a 27th.
func artifactURI(s string) (artifact.URI, error) {
	u, err := artifact.Parse(s)
	if err != nil {
		return artifact.URI{}, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	return u, nil
}

// optionalArtifactURI is artifactURI for a field whose absence is legal (a board export a design may
// not have). An empty string yields the zero URI and no error; anything else must still parse, so a
// typo is not silently read as "not supplied".
func optionalArtifactURI(s string) (artifact.URI, error) {
	if s == "" {
		return artifact.URI{}, nil
	}
	return artifactURI(s)
}
