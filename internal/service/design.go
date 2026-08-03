package service

import (
	"context"
	"errors"
	"fmt"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/internal/expect"
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/core/render"
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
	Design(ctx context.Context, mount, path string) (*ir.Design, error)
	// Geometry resolves drawable geometry for the layout and symbol source (the design's own
	// symbols when faithfulSymbols, else synthetic glyphs).
	Geometry(ctx context.Context, mount, path, layout string, faithfulSymbols bool) (*geom.SchematicGeometry, error)
	// Report classifies how an auto-layout draws each component (the conversion report).
	Report(ctx context.Context, mount, path string, faithfulSymbols bool) (*graph.ConversionReport, error)
	// Expectations loads a design's `<path>.expect.yaml` sidecar (WS6-006). A design with no sidecar
	// returns a nil map and a nil error (absence is normal), so the caller renders an empty panel
	// rather than an error.
	Expectations(ctx context.Context, mount, path string) (*expect.Expectations, error)
	// Board returns the physical board sidecar (WS1-006) for formats that carry one
	// (.kicad_pcb today). nil with a nil error means the format has none — absence is
	// normal, mirroring Expectations — and the design then simply lists no board sheet.
	Board(ctx context.Context, mount, path string) (*geom.BoardGeometry, error)
}

// boardSheetID is the synthetic sheet id the board renders under (WS7-034). It is a sheet
// so navigation, deep links, the sheet overview, and the highlight RPCs apply unchanged;
// the id is non-numeric on purpose (a numeric selector reads as a positional index).
const boardSheetID = "board"

// boardFor loads the board sidecar for a sheet request, classifying "this file has no
// board" as not-found (the caller asked for a sheet that does not exist).
func (s *DesignService) boardFor(ctx context.Context, mount, path string) (*geom.BoardGeometry, error) {
	b, err := s.loader.Board(ctx, mount, path)
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
	Available(mount, path string) bool
	Render(ctx context.Context, mount, path string, page int) (string, error)
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
}

// NewDesignService returns a DesignService backed by the given ports and render style.
func NewDesignService(loader Loader, native NativeRenderer, style render.Style) *DesignService {
	return &DesignService{loader: loader, native: native, style: style}
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
	mount, path := req.GetMount(), req.GetPath()
	layout := layoutForFile(path, req.GetLayout())
	g, err := s.loader.Geometry(ctx, mount, path, layout, false)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	resp := &webapi.GetDesignResponse{
		Layout:           layout,
		NativeAvailable:  s.native.Available(mount, path),
		AvailableLayouts: availableLayouts(path),
	}
	for _, sh := range g.GetSheets() {
		resp.Sheets = append(resp.Sheets, &webapi.SheetRef{Id: sh.GetId(), Name: sh.GetName(), ParentId: sh.GetParentId()})
	}
	// A file with a board sidecar lists the physical board as one more sheet, after the
	// drawable ones, regardless of the layout axis (the board is faithful by nature).
	if b, err := s.loader.Board(ctx, mount, path); err == nil && b != nil {
		resp.Sheets = append(resp.Sheets, &webapi.SheetRef{Id: boardSheetID, Name: "Board"})
	}
	if layout == faithfulLayout {
		resp.Name = g.GetDesignRef()
	} else if d, err := s.loader.Design(ctx, mount, path); err == nil {
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
	faithful := req.GetSymbols() == webapi.SymbolSource_SYMBOL_SOURCE_FAITHFUL
	rep, err := s.loader.Report(ctx, req.GetMount(), req.GetPath(), faithful)
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
	mount, path := req.GetMount(), req.GetPath()
	// The synthetic board sheet renders from the board sidecar, not the schematic geometry:
	// SVG from BoardSVG, PACKED (and UNSPECIFIED) from PackBoard — the WS7-035 tier, same
	// envelope as the schematic pack so the canvas draws it with one extra draw mode.
	// NATIVE has no board pages; it answers with the SVG document.
	if req.GetSheet() == boardSheetID {
		b, err := s.boardFor(ctx, mount, path)
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
	g, err := s.loader.Geometry(ctx, mount, path, layoutForFile(path, requested), faithful)
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
		resp.Content = &webapi.GetSheetResponse_Svg{Svg: render.SheetSVG(g, sheet, render.WithStyle(style))}
	case webapi.SheetFormat_SHEET_FORMAT_NATIVE:
		svg, err := s.native.Render(ctx, mount, path, render.SheetIndex(g, sheet)+1)
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
	if req.GetFormat() == webapi.SheetFormat_SHEET_FORMAT_NATIVE {
		return nil, fmt.Errorf("%w: no highlight overlay for the NATIVE format", ErrInvalidArgument)
	}
	mount, path := req.GetMount(), req.GetPath()
	// The board sheet's overlay comes from the board join (net -> copper, ref_des -> pads):
	// a transparent SVG framed exactly like the BoardSVG base, or primitive-index groups
	// over the SAME PackBoard primitive table the PACKED GetSheet returned.
	if req.GetSheet() == boardSheetID {
		b, err := s.boardFor(ctx, mount, path)
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
	g, err := s.loader.Geometry(ctx, mount, path, layoutForFile(path, req.GetLayout()), faithful)
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

	resp := &webapi.HighlightSheetResponse{}
	if req.GetFormat() == webapi.SheetFormat_SHEET_FORMAT_SVG {
		resp.Content = &webapi.HighlightSheetResponse_Svg{Svg: render.HighlightSVG(g, sheet, req.GetSpecs())}
	} else { // UNSPECIFIED or PACKED
		// Style does not affect packing indices/keys, so the default-style pack joins the
		// styled GetSheet pack exactly (same primitive table).
		packed := render.PackSheet(g, sheet)
		resp.Content = &webapi.HighlightSheetResponse_Packed{Packed: render.HighlightPacked(packed, req.GetSpecs())}
	}
	return resp, nil
}
