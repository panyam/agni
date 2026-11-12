package service

import (
	"context"
	"fmt"
	"github.com/panyam/agni/artifact"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/core/timing"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// ModelLoader is the file surface BuildModel needs: the netlist design and its board sidecar, both
// mount-scoped by the impl. The fat service.Loader and the review ReviewLoader both satisfy it, so
// every service reaches BuildModel through the loader it already holds.
type ModelLoader interface {
	Design(ctx context.Context, uri artifact.URI, opts ...ReadOption) (*ir.Design, error)
	Board(ctx context.Context, uri artifact.URI) (*geom.BoardGeometry, error)
}

// GeometryLoader is the file surface BuildGeometry needs. The fat service.Loader and DiffService's
// DesignLoader both satisfy it.
type GeometryLoader interface {
	Geometry(ctx context.Context, uri artifact.URI, layout string, faithfulSymbols bool, opts ...ReadOption) (*geom.SchematicGeometry, error)
}

// BuildGeometry loads a design's default-layout, glyph-symbol schematic geometry for LOCATING results
// on sheets (AnnotateSheets badges, indexSheets query-cell navigation). It never feeds a rule, so
// surfaces that run rules without locating them (GetComponentParams, scalar-only queries) skip it.
// It is best-effort. A netlist-only file or an unresolvable layout yields nil, NOT an error, and the
// caller shows no sheet badges. So geometry is not a BuildModel tier, since a bad board or params
// tier there fails the run.
//
// Pass the same ReadOptions the Model was built with (agni issue 347). A geometry read without them
// misses a project's declared symbol library, so findings are placed on sheets that lack the
// components they name.
func BuildGeometry(ctx context.Context, loader GeometryLoader, uri artifact.URI, opts ...ReadOption) *geom.SchematicGeometry {
	g, _ := locateGeometry(ctx, loader, uri, opts...)
	return g
}

// locateGeometry is BuildGeometry that also says whether the schematic failed to read. A netlist-only
// file draws an auto-layout rather than failing. So a read error means the file meant to draw the
// design failed to read. The locate answer says so rather than calling every entity drawn nowhere
// (agni issue 1230).
func locateGeometry(ctx context.Context, loader GeometryLoader, uri artifact.URI, opts ...ReadOption) (g *geom.SchematicGeometry, unreadable bool) {
	defer timing.Begin(ctx, "read.geometry")()
	g, err := loader.Geometry(ctx, uri, layoutForFile(uri.Path, ""), false, opts...)
	if err != nil {
		return nil, true
	}
	return g, false
}

// BuildModel constructs the FULL check Model for a design (netlist, board tier, params tier), so every
// rule-running service surface builds the same Model and none drops a tier (WS9-048). A plain
// check.NewModel gates every board-DRC and datasheet rule to not-applicable. The board tier reads the
// design's own sidecar, or boardURI when set (WS3-089's --board-path). specs is the datasheet provider
// and may be nil when serve ran without --params. The CLI's counterpart is readModelWithParams.
//
// A boardURI that yields no board geometry is an error. Without that error, an explicit board
// request that read nothing would report the board items clean without checking them. When a
// design's OWN path has no board (a netlist), that is the normal nil-board case.
func BuildModel(ctx context.Context, loader ModelLoader, uri, boardURI artifact.URI, specs param.ParamProvider, opts ...ReadOption) (check.Model, error) {
	return BuildModelTier(ctx, loader, uri, boardURI, specs, ReadBoard, opts...)
}

// A BoardTier says whether a model reads the design's board. On a large board, most of a model's
// memory is the board's geometry and the per-net copper built from it (385 of 547 MB on the Jetson
// seed, agni issue 987). So a request that asks nothing of the copper skips it.
type BoardTier bool

const (
	// ReadBoard reads the board tier, as every rule run does.
	ReadBoard BoardTier = true
	// SkipBoard builds the model without one, so it reports HasBoard false and every board.*
	// relation is empty. Use it only for a request that reads no board fact, such as a query naming
	// no board.* relation (see boardTierFor), interface coverage, or a part's parameters.
	SkipBoard BoardTier = false
)

// boardReadURI returns the artifact to read a model's board from. That is the separate board when
// the request names one. Otherwise it is the design itself, and the loader finds the design's
// sidecar or declared companion.
func boardReadURI(uri, boardURI artifact.URI) artifact.URI {
	if !boardURI.IsZero() {
		return boardURI
	}
	return uri
}

// BuildModelTier is BuildModel with the board tier chosen by the caller. With SkipBoard the loader is
// never asked for a board, boardURI included.
func BuildModelTier(ctx context.Context, loader ModelLoader, uri, boardURI artifact.URI, specs param.ParamProvider, board BoardTier, opts ...ReadOption) (check.Model, error) {
	endRead := timing.Begin(ctx, "read.netlist")
	d, err := loader.Design(ctx, uri, opts...)
	endRead()
	if err != nil {
		return nil, ClassifyLoadErr(err)
	}
	if d == nil {
		return nil, fmt.Errorf("%w: %s carries no netlist", ErrInvalidArgument, uri)
	}
	boardFrom := boardReadURI(uri, boardURI)
	var bg *geom.BoardGeometry
	if board == ReadBoard {
		endBoard := timing.Begin(ctx, "read.board")
		bg, err = loader.Board(ctx, boardFrom)
		endBoard()
		if err != nil {
			return nil, ClassifyLoadErr(err)
		}
	}
	if board == ReadBoard && !boardURI.IsZero() && bg == nil {
		return nil, fmt.Errorf("%w: board_uri %q carries no board geometry", ErrInvalidArgument, boardURI)
	}
	// A corpus reached over a network fetches the design's parts here, where a failure can be an
	// error. Its Lookup cannot report one, and a nil from it reads as "not seeded", which would make
	// every datasheet rule skip the part silently (agni issue 749).
	if p, ok := specs.(param.Prefetcher); ok {
		mpns := make([]string, 0, len(d.GetComponents()))
		for _, c := range d.GetComponents() {
			mpns = append(mpns, c.GetMpn())
		}
		endFetch := timing.Begin(ctx, "prefetch.params")
		err := p.Prefetch(ctx, mpns)
		endFetch()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
	}
	// The read's lexicon also reaches the MODEL. Some name matches hold no net (spec name FFIs,
	// pin-role derivation), and they use the same vocabulary the design was stamped with.
	mopts := []check.ModelOption{check.WithBoard(bg), check.WithParamProvider(specs)}
	ro := ReadOpts(opts...)
	if ro.Lexicon != nil {
		mopts = append(mopts, check.WithLexicon(ro.Lexicon))
	}
	if ro.Intent != nil {
		mopts = append(mopts, check.WithIntent(ro.Intent))
	}
	defer timing.Begin(ctx, "model")()
	return check.NewModel(d, mopts...), nil
}
