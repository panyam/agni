package service

import (
	"context"
	"fmt"

	"github.com/panyam/agni/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/datasheet/param"
)

// ModelLoader is the file surface BuildModel needs: the netlist design and its board sidecar, both
// mount-scoped by the impl. The fat service.Loader and the review ReviewLoader both satisfy it, so
// every service reaches BuildModel through the loader it already holds.
type ModelLoader interface {
	Design(ctx context.Context, mount, path string) (*ir.Design, error)
	Board(ctx context.Context, mount, path string) (*geom.BoardGeometry, error)
}

// GeometryLoader is the file surface BuildGeometry needs. The fat service.Loader and DiffService's
// DesignLoader both satisfy it.
type GeometryLoader interface {
	Geometry(ctx context.Context, mount, path, layout string, faithfulSymbols bool) (*geom.SchematicGeometry, error)
}

// BuildGeometry loads a design's default-layout, glyph-symbol schematic geometry for LOCATING results
// on sheets — finding badges (AnnotateSheets), query-cell navigation (indexSheets). It is the
// presentation/location layer, distinct from BuildModel's rule fact-base: it is loaded only where
// results are placed on sheets, never to evaluate a rule, so surfaces that run rules but don't locate
// (GetComponentParams) and scalar-only queries skip it. Best-effort by contract — a netlist-only file
// or an unresolvable layout yields nil (NOT an error), so the caller degrades to "no sheet badges"
// rather than failing. That nil-not-error semantics is exactly why geometry is NOT a BuildModel tier:
// the Model's board/params tiers are load-bearing (a bad one fails the run), geometry is optional.
func BuildGeometry(ctx context.Context, loader GeometryLoader, mount, path string) *geom.SchematicGeometry {
	g, err := loader.Geometry(ctx, mount, path, layoutForFile(path, ""), false)
	if err != nil {
		return nil
	}
	return g
}

// BuildModel constructs the FULL check Model for a design — netlist + board tier + params tier — so
// every rule-running service surface builds an identical Model and none silently drops a tier
// (WS9-048). The drift it closes: served CheckDesign / GetCheckReport / GetInterfaceCoverage / RunQuery
// were building plain netlist models (check.NewModel), so check.Available gated every board-DRC and
// datasheet rule to not-applicable even for a board-bearing file and even with serve --params — while
// the CLI on the same file ran them. The board tier reads the design's own sidecar, or boardPath when
// set (a separate export, WS3-089's --board-path); specs is the injected datasheet provider, nil when
// serve ran without --params (NewModelWithParams guards a nil provider — no joined specs, no false
// pass). It is the service-side twin of the CLI's readModelWithParamsBoard: one tier policy, two edges.
//
// A boardPath override that resolves to no board reader (a non-board file type) is a loud error, never
// a silent nil — an explicit board request that read nothing would report the board items clean without
// checking them. A design's OWN path that carries no board (a netlist) is not an override, so it is the
// normal nil-board case, not an error.
func BuildModel(ctx context.Context, loader ModelLoader, mount, path, boardPath string, specs param.ParamProvider) (check.Model, error) {
	d, err := loader.Design(ctx, mount, path)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	boardFrom := path
	if boardPath != "" {
		boardFrom = boardPath
	}
	bg, err := loader.Board(ctx, mount, boardFrom)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	if boardPath != "" && bg == nil {
		return nil, fmt.Errorf("%w: board_path %q carries no board geometry", ErrInvalidArgument, boardPath)
	}
	return check.NewModelWithParams(d, bg, specs), nil
}
