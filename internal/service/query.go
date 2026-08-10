package service

import (
	"context"
	"fmt"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/datasheet/param"
	"github.com/panyam/agni/core/query"
)

// QueryService evaluates ad-hoc datalog queries over a design's fact base (WS3-029) for the web
// query panel, over the same injected Loader the design and check services use (CONSTRAINTS C13).
// It is the web front-end to the engine `agni query` runs: a rule is one named query, a search is
// an arbitrary one, and both share the `query` evaluator. It knows no transport.
//
// The evaluator is pluggable behind query.Evaluator; this service holds the default naive
// interpreter. v1 evaluates the netlist fact base only — the datasheet `param` relation is empty
// because serve wires no params dir and datasheet data stays deployment-bound (C16), so a query
// over `param` yields no rows rather than an error.
type QueryService struct {
	loader Loader
	eval   query.Evaluator
	// specs is the datasheet provider the model's param.* relations read (WS9-048), nil when serve
	// ran without --params. Without it a query over param.* / component.device_class returned no rows
	// while the CLI's `agni query --params` did — the same board/params drift BuildModel closes.
	specs param.ParamProvider
}

// NewQueryService returns a QueryService backed by the given loader and (optional) datasheet
// provider, using the default naive datalog evaluator. Pass a nil provider when no datasheet corpus
// is wired; the model's param.* relations then yield no rows (never an error).
func NewQueryService(loader Loader, specs param.ParamProvider) *QueryService {
	return &QueryService{loader: loader, eval: query.Naive{}, specs: specs}
}

// RunQuery loads the design, parses and evaluates the datalog query over its fact base, and returns
// the projected columns and answer rows with provenance. A geometry-only file with no netlist maps
// to invalid argument (via classifyLoadErr); a malformed query is an invalid argument too, so the
// panel shows the parse error inline rather than treating it as a server fault. A well-formed query
// that matches nothing returns an empty row set (not an error).
func (s *QueryService) RunQuery(ctx context.Context, req *webapi.RunQueryRequest) (*webapi.RunQueryResponse, error) {
	q, err := query.Parse(req.GetQuery())
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	// The request's overlay is composed BEFORE the read, because only its lexicon half matters here and
	// that half has to reach the READ: net roles are resolved once at ingestion, so the vocabulary
	// decides what `rail`, `feedback`, and everything derived from them answer (WS3-113).
	//
	// The convention's RULES half is ignored, deliberately. A query composes no catalog, and a project
	// keeps one conventions file carrying both halves, so refusing it over rules this call will never
	// run would reject a config that is perfectly valid for the question being asked. There is no base
	// convention to replace for the same reason: nothing here holds a catalog.
	ov, err := ComposeOverlay(req.GetOverlay(), "")
	if err != nil {
		return nil, err
	}
	// One FULL Model over the design (netlist + board + params, WS9-048): the query evaluator reads
	// it, and the per-cell locate classifier (WS9-039) shares its indexes rather than re-scanning the
	// raw IR. The board/params tiers back the board.* / param.* query relations, matching `agni query`.
	model, err := BuildModel(ctx, s.loader, req.GetMount(), req.GetPath(), req.GetBoardRef(), s.specs, ov.ReadOptions()...)
	if err != nil {
		return nil, err
	}
	rows, err := s.eval.Eval(q, query.NewBase(model))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	cols := q.Columns()
	kinds := columnKinds(q)
	resp := &webapi.RunQueryResponse{Columns: make([]string, len(cols)), ColumnKinds: kinds}
	for i, c := range cols {
		resp.Columns[i] = string(c)
	}
	// A query naming an entity column (a component or net) resolves each such cell's sheet
	// membership so the panel can badge it and navigate on click (WS9-038). Nets resolve from the
	// netlist alone (AttrSheets); components need schematic geometry, loaded best-effort — a
	// netlist-only file then simply yields no component badges rather than an error. A scalar-only
	// query names no entity and loads no geometry.
	var ix sheetIndex
	navigable := false
	for _, k := range kinds {
		if k != "" {
			navigable = true
			break
		}
	}
	// drawnComps/drawnNets are the entities the faithful geometry actually draws (a placement /
	// a wire). A navigable cell absent from them will not highlight in the faithful view, so it
	// gets a locate reason (WS9-039). Empty when no geometry loaded, in which case no reasons are
	// emitted (the design renders via an auto-layout that draws every entity).
	var drawnComps, drawnNets map[string]bool
	if navigable {
		g := BuildGeometry(ctx, s.loader, req.GetMount(), req.GetPath())
		ix = indexSheets(g, model)
		if g != nil {
			drawnComps, drawnNets = drawnEntities(g)
		}
	}
	for _, r := range rows {
		cells := make([]string, len(cols))
		for i, c := range cols {
			cells[i] = r.Bind[c].S
		}
		row := &webapi.QueryRow{Cells: cells, Cites: r.Cites}
		if navigable {
			row.CellSheets = make([]*webapi.CellSheets, len(cols))
			row.CellReasons = make([]checkspb.LocateReason, len(cols))
			for i := range cols {
				cs := &webapi.CellSheets{}
				if kinds[i] != "" {
					cs.SheetIds = ix.sheetsFor(&checkspb.Subject{Kind: kinds[i], Ref: cells[i]})
					row.CellReasons[i] = cellReason(model, kinds[i], cells[i], drawnComps, drawnNets)
				}
				row.CellSheets[i] = cs
			}
		}
		resp.Rows = append(resp.Rows, row)
	}
	return resp, nil
}

// drawnEntities collects the ref_des and net names the faithful geometry actually draws — a
// component with a symbol placement, a net with a wire. Membership answers "will highlighting this
// paint anything on the faithful view", the authoritative locate check regardless of render mode
// (SVG or WebGL draw the same geometry).
func drawnEntities(g *geom.SchematicGeometry) (comps, nets map[string]bool) {
	comps, nets = map[string]bool{}, map[string]bool{}
	for _, sh := range g.GetSheets() {
		for _, pl := range sh.GetPlacements() {
			comps[pl.GetRefDes()] = true
		}
		for _, w := range sh.GetWires() {
			if n := w.GetNet(); n != "" {
				nets[n] = true
			}
		}
	}
	return comps, nets
}

// cellReason returns why a navigable cell will not highlight in the faithful view (WS9-039), or
// UNSPECIFIED when the entity IS drawn. An undrawn entity is explained by its netlist facts
// (virtual `#` symbol, power rail, unknown ref/net); an undrawn entity with no such fact is
// NO_GEOMETRY (drawn nowhere for no more specific reason). A drawn entity never gets a reason, so a
// rail that happens to carry a wire (e.g. VBUS) reports UNSPECIFIED.
func cellReason(m check.Model, kind, subject string, drawnComps, drawnNets map[string]bool) checkspb.LocateReason {
	drawn := drawnComps[subject]
	if kind == check.KindNet {
		drawn = drawnNets[subject]
	}
	if drawn {
		return checkspb.LocateReason_LOCATE_REASON_UNSPECIFIED
	}
	switch check.LocateReason(m, kind, subject) {
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

// columnKinds derives each answer column's entity kind for the panel's click-to-locate (WS9-038):
// "component" (a ref_des), "net", or "" (a scalar or unresolved column). Kind is a column property,
// not a per-cell one, because a variable binds at the same relation position in every row. It reads
// the arg-labels the relation catalog already declares (component-on-net(ref_des, net), ...), so it
// adds no new vocabulary. An explicit Select is walked term-by-term so an aggregate or constant
// column stays scalar even when it reduces an entity variable (count(?ref) is a number, not a part);
// the default select (goal variables) has no aggregates, so its columns map straight through.
func columnKinds(q query.Query) []string {
	labels := catalogArgLabels()
	if len(q.Select) == 0 {
		cols := q.Columns()
		kinds := make([]string, len(cols))
		for i, col := range cols {
			kinds[i] = varKind(col, q.Goal, labels)
		}
		return kinds
	}
	kinds := make([]string, len(q.Select))
	for i, t := range q.Select {
		if t.Agg == nil && t.Var != "" {
			kinds[i] = varKind(t.Var, q.Goal, labels)
		}
	}
	return kinds
}

// varKind returns the entity kind a variable resolves to: the label-kind of the first positive body
// atom that binds it to an entity position. "First entity-yielding binding wins" so a variable used
// as a net in one atom and a scalar in another is a net; a variable bound only in scalar positions,
// or only by a user rule / IDB relation absent from the catalog, stays "".
func varKind(col query.Var, body query.Body, labels map[string][]string) string {
	for _, lit := range body.Literals {
		a := lit.Pos
		if a == nil {
			continue
		}
		ls := labels[a.Relation]
		for j, term := range a.Args {
			if j >= len(ls) || term.Var != col {
				continue
			}
			if k := labelKind(ls[j]); k != "" {
				return k
			}
		}
	}
	return ""
}

// labelKind maps a catalog arg-label to the highlightable entity kind it names, or "" for a scalar
// (a number, or an mpn/symbol/layer string the canvas cannot locate). Only ref_des and net-valued
// endpoints resolve — "from" is reaches' source net, so it counts as a net.
func labelKind(label string) string {
	switch label {
	case "ref_des":
		return check.KindComponent
	case "net", "from":
		return check.KindNet
	default:
		return ""
	}
}

// catalogArgLabels indexes the relation catalog by name to its ordered arg-labels, the lookup
// columnKinds uses to type each bound variable.
func catalogArgLabels() map[string][]string {
	m := map[string][]string{}
	for _, ri := range query.Catalog() {
		m[ri.Name] = ri.Args
	}
	return m
}

// ListRelations returns the queryable relation catalog (WS9-037) for the panel's relation picker:
// the built-in relations and predicates plus any overlay-registered relations, pre-sorted by kind
// then name (query.Catalog). It loads no design — the catalog is static per build — so it never
// fails on a bad path and the client can fetch it once at startup.
func (s *QueryService) ListRelations(_ context.Context, _ *webapi.ListRelationsRequest) (*webapi.ListRelationsResponse, error) {
	resp := &webapi.ListRelationsResponse{}
	for _, r := range query.Catalog() {
		resp.Relations = append(resp.Relations, &webapi.RelationInfo{
			Name:    r.Name,
			Args:    r.Args,
			Summary: r.Summary,
			Kind:    r.Kind,
			Detail:  r.Detail,
		})
	}
	for _, e := range query.Examples() {
		resp.Examples = append(resp.Examples, &webapi.ExampleQuery{Label: e.Label, Query: e.Query, Teaches: e.Teaches})
	}
	return resp, nil
}
