package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/core/query"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/jaala/ns"
)

// QueryService evaluates ad-hoc datalog queries over a design's fact base (WS3-029) for the web
// query panel, over the same injected Loader the design and check services use (CONSTRAINTS C13).
// It answers what `agni query` answers and knows no transport. The evaluator sits behind
// query.Evaluator and defaults to the naive interpreter.
type QueryService struct {
	// projects and fallback are documented on ProjectResolver and Overlay.
	projects *ProjectResolver
	fallback Overlay
	loader   Loader
	eval     query.Evaluator
	// specs is the datasheet provider the param.* and component.device_class relations read
	// (WS9-048), nil when serve ran without --params. A project's own params win (Overlay.SpecsOr).
	specs param.ParamProvider
}

// NewQueryService returns a QueryService backed by the given loader and (optional) datasheet
// provider, using the default naive datalog evaluator. Pass a nil provider when no datasheet corpus
// is wired; the model's param.* relations then yield no rows (never an error).
func NewQueryService(loader Loader, specs param.ParamProvider, projects *ProjectResolver) *QueryService {
	return &QueryService{loader: loader, eval: query.Default, specs: specs, projects: projects}
}

// RunQuery loads the design, parses and evaluates the datalog query over its fact base, and returns
// the projected columns and answer rows with provenance. A geometry-only file with no netlist and a
// malformed query are both invalid arguments, so the panel shows the parse error inline. A query
// that matches nothing returns an empty row set, not an error.
func (s *QueryService) RunQuery(ctx context.Context, req *webapi.RunQueryRequest) (*webapi.RunQueryResponse, error) {
	u, err := ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	boardURI, err := optionalArtifactURI(req.GetBoardUri())
	if err != nil {
		return nil, err
	}
	q, err := query.Parse(req.GetQuery())
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	d, err := s.read(ctx, u, boardURI, req.GetUri(), req.GetOverlay(), req.GetAsNamed())
	if err != nil {
		return nil, err
	}
	resp, err := s.answer(query.NarrowBudget(ctx, req.GetWorkBudget()), d, q, req.GetQuery(), BindingsFromProto(req.GetBindings()))
	if err != nil {
		if stopped(err) {
			return nil, err
		}
		if overBudget(err) {
			return nil, fmt.Errorf("%w: %s", ErrResourceExhausted, err)
		}
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	return resp, nil
}

// overBudget reports whether an evaluation stopped at its work budget.
func overBudget(err error) bool {
	var be *query.BudgetExceeded
	return errors.As(err, &be)
}

// stopped reports whether an evaluation ended because its caller's context was cancelled or timed
// out. The engine checks the context it is handed, and so do the walks, so a query nobody is waiting
// for stops; reporting that as the caller's mistake (ErrInvalidArgument) would misdescribe it, and in
// a set it would be repeated for every query left.
func stopped(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// RunQueries answers every query of a set over one read of the design (agni issue 729). Each
// result is what RunQuery would return for that query alone; a query that does not parse or names
// something the design's relations lack is reported against its name, and the others still answer.
// A set that is unusable as a whole is an invalid argument, and a design that cannot be read fails
// the call as it would fail RunQuery.
func (s *QueryService) RunQueries(ctx context.Context, req *webapi.RunQueriesRequest) (*webapi.RunQueriesResponse, error) {
	u, err := ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	boardURI, err := optionalArtifactURI(req.GetBoardUri())
	if err != nil {
		return nil, err
	}
	set := QuerySetFromProto(req.GetSet())
	if err := set.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	d, err := s.read(ctx, u, boardURI, req.GetUri(), req.GetOverlay(), req.GetAsNamed())
	if err != nil {
		return nil, err
	}
	ctx = query.NarrowBudget(ctx, req.GetWorkBudget())
	out := &webapi.RunQueriesResponse{Title: set.Title, Preamble: set.Preamble, Source: req.GetUri()}
	for i, nq := range set.Queries {
		res := &webapi.NamedQueryResult{Name: nq.Name, Description: nq.Description}
		out.Results = append(out.Results, res)
		q, err := set.Compile(i)
		if err != nil {
			res.Error = err.Error()
			continue
		}
		resp, err := s.answer(ctx, d, q, nq.Query, nq.Bind)
		if err != nil {
			if stopped(err) {
				return nil, err
			}
			res.Error = err.Error()
			continue
		}
		res.Result = resp
	}
	return out, nil
}

// QuerySetFromProto converts the wire form of a query set to the engine's.
func QuerySetFromProto(p *webapi.QuerySet) query.QuerySet {
	s := query.QuerySet{Title: p.GetTitle(), Preamble: p.GetPreamble()}
	for _, q := range p.GetQueries() {
		s.Queries = append(s.Queries, query.NamedQuery{Name: q.GetName(), Query: q.GetQuery(), Description: q.GetDescription(), Bind: BindingsFromProto(q.GetBindings())})
	}
	return s
}

// BindingsFromProto converts a request's bindings to the values the engine binds (agni issue 793). A
// value that sets neither field binds the empty text, as `""` written in the query would.
func BindingsFromProto(m map[string]*webapi.QueryValue) map[string]query.Value {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]query.Value, len(m))
	for k, v := range m {
		if n, ok := v.GetKind().(*webapi.QueryValue_Number); ok {
			out[k] = query.Number(n.Number)
		} else {
			out[k] = query.Text(v.GetText())
		}
	}
	return out
}

// BindingsProto converts bound values to their wire form: a value carrying a number is a number,
// any other is text.
func BindingsProto(m map[string]query.Value) map[string]*webapi.QueryValue {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]*webapi.QueryValue, len(m))
	for k, v := range m {
		if v.Num != nil {
			out[k] = &webapi.QueryValue{Kind: &webapi.QueryValue_Number{Number: *v.Num}}
		} else {
			out[k] = &webapi.QueryValue{Kind: &webapi.QueryValue_Text{Text: v.S}}
		}
	}
	return out
}

// QuerySetProto converts a query set to its wire form, for a caller that read one from a file.
func QuerySetProto(s query.QuerySet) *webapi.QuerySet {
	p := &webapi.QuerySet{Title: s.Title, Preamble: s.Preamble}
	for _, q := range s.Queries {
		p.Queries = append(p.Queries, &webapi.NamedQuery{Name: q.Name, Query: q.Query, Description: q.Description, Bindings: BindingsProto(q.Bind)})
	}
	return p
}

// designRead is one read of a design, shared by every query asked of it. It holds the model, one
// fact base over it, and the schematic geometry, which loads once and only when some answer has a
// cell to place on a sheet.
type designRead struct {
	u      artifact.URI
	source string
	model  check.Model
	base   *query.Base
	reg    *facts.Registry // the vocabulary base answers over, the project's library included
	ov     Overlay
	gu     artifact.URI

	geomLoaded bool
	ix         sheetIndex
	drawnComps map[string]bool
	drawnNets  map[string]bool
}

// read resolves the design's tiers, composes its overlay, and builds the model and fact base once.
func (s *QueryService) read(ctx context.Context, u, boardURI artifact.URI, source string, overlay *webapi.OverlayConfig, asNamed bool) (*designRead, error) {
	// The overlay is composed BEFORE the read because its lexicon has to reach the READ (see
	// readopt.go), which decides what `net.rail`, `net.feedback` and their derivatives answer (WS3-113).
	// Its RULES half is ignored and no base convention is passed, since a query composes no catalog
	// and a conventions file carrying rules is still valid for a query.
	ov, err := s.projects.Overlay(ctx, u, overlay, s.fallback, "")
	if err != nil {
		return nil, err
	}
	// One FULL Model (netlist, board, params; WS9-048) backs the board.* and param.* relations and
	// the per-cell locate classifier (WS9-039). Tiers come from the design's declaration, so a query
	// addressed at a schematic companion counts the netlist's 1617 nets rather than the drawing's
	// 4572 per-sheet segments (agni issue 656).
	nu, bu, gu, err := s.projects.TierURIs(ctx, u, boardURI, asNamed)
	if err != nil {
		return nil, err
	}
	model, err := BuildModel(ctx, s.loader, nu, bu, ov.SpecsOr(s.specs), ov.ReadOptions()...)
	if err != nil {
		return nil, err
	}
	// The project's own library joins the vocabulary here, so every query surface that reads through
	// this function sees it (agni issue 773). A library that does not compose fails the read rather
	// than answering without it.
	reg, err := ov.Registry()
	if err != nil {
		return nil, err
	}
	return &designRead{u: u, source: source, model: model, base: query.NewBaseFrom(reg, model), reg: reg, ov: ov, gu: gu}, nil
}

// geometry loads the schematic geometry on first use and returns the sheet index and the entities it
// draws. Empty maps mean no geometry, in which case no locate reasons are emitted (the design renders
// on an auto-layout that draws every entity).
func (d *designRead) geometry(ctx context.Context, loader Loader) (sheetIndex, map[string]bool, map[string]bool) {
	if !d.geomLoaded {
		d.geomLoaded = true
		g := BuildGeometry(ctx, loader, d.gu, d.ov.ReadOptions()...)
		d.ix = indexSheets(g, d.model)
		if g != nil {
			d.drawnComps, d.drawnNets = drawnEntities(g)
		}
	}
	return d.ix, d.drawnComps, d.drawnNets
}

// answer evaluates one query over a read and assembles its response, echoing queryText as the
// response's query. An error is the evaluator's (a malformed or unanswerable query), and the caller
// decides what it means for the call.
func (s *QueryService) answer(ctx context.Context, d *designRead, q query.Query, queryText string, bind map[string]query.Value) (*webapi.RunQueryResponse, error) {
	// Work is measured on this read's own fact base, so it is this query's alone, and reported so a
	// deployment can see what its queries cost before it sets a budget (agni issue 792).
	before := d.base.Work()
	opts := query.EvalOptions(ctx)
	if len(bind) > 0 {
		opts = append(opts, query.Bind(bind))
	}
	rows, err := s.eval.Eval(ctx, q, d.base, opts...)
	if err != nil {
		return nil, err
	}
	work := d.base.Work() - before
	cols := q.Columns()
	kinds, kindVars, refTerms := columnKindsIn(d.reg, q)
	// Query and Source come from the request rather than being re-derived, so a saved response
	// cannot describe a different run than the one that produced these rows.
	resp := &webapi.RunQueryResponse{
		Columns: make([]string, len(cols)), ColumnKinds: kinds,
		Query: queryText, Source: d.source, Work: work, Bindings: BindingsProto(bind),
	}
	for i, c := range cols {
		resp.Columns[i] = string(c)
	}
	// Entity cells get their sheets so the panel can badge them and navigate on click (WS9-038).
	// Nets resolve from the netlist alone; components need schematic geometry, loaded best-effort, so
	// a netlist-only file yields no component badges rather than an error. A scalar-only query loads
	// no geometry.
	var ix sheetIndex
	navigable := false
	for i := range kinds {
		if kinds[i] != "" || kindVars[i] != "" {
			navigable = true
			break
		}
	}
	// A navigable cell missing from drawnComps/drawnNets gets a locate reason (WS9-039).
	var drawnComps, drawnNets map[string]bool
	if navigable {
		ix, drawnComps, drawnNets = d.geometry(ctx, s.loader)
	}
	for _, r := range rows {
		cells := make([]string, len(cols))
		for i, c := range cols {
			cells[i] = r.Bind[c].S
		}
		row := &webapi.QueryRow{Cells: cells, Cites: portableCites(r.Cites, d.u.Path)}
		if navigable {
			row.CellSheets = make([]*webapi.CellSheets, len(cols))
			row.CellReasons = make([]checkspb.LocateReason, len(cols))
			for i := range cols {
				// A polymorphic column takes its kind from THIS row's binding of the kind variable
				// and puts it on the wire so the client types the cell the same way (agni issue 338).
				kind := kinds[i]
				if v := kindVars[i]; v != "" {
					kind = entityKind(r.Bind[v].S)
					if row.CellKinds == nil {
						row.CellKinds = make([]string, len(cols))
					}
					row.CellKinds[i] = kind
				}
				// A pin cell carries the ref this row bound, and the ref answers which sheets the pin
				// is on and whether it is drawn.
				ref := cells[i]
				if kind == check.KindPin {
					ref = termValue(refTerms[i], r.Bind)
					if row.CellRefs == nil {
						row.CellRefs = make([]string, len(cols))
					}
					row.CellRefs[i] = ref
					// A pin whose component did not resolve stays a scalar.
					if ref == "" {
						kind = ""
					}
				}
				cs := &webapi.CellSheets{}
				if kind != "" {
					cs.SheetIds = ix.sheetsFor(&checkspb.Subject{Kind: kind, Ref: ref, Pin: cells[i]})
					row.CellReasons[i] = cellReason(d.model, kind, ref, drawnComps, drawnNets, len(cs.SheetIds) > 0)
				}
				row.CellSheets[i] = cs
			}
		}
		resp.Rows = append(resp.Rows, row)
	}
	return resp, nil
}

// drawnEntities collects the ref_des with a symbol placement and the nets with a wire in the
// faithful geometry. Membership answers whether highlighting an entity paints anything, in either
// render mode, since SVG and WebGL draw the same geometry.
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
//
// onSheet reports whether the cell resolved to any sheet, the only drawn-test a BUS has, since a
// bus is neither a placement nor a named wire. Keep it identical to AnnotateSheets' rule for a bus
// finding.
func cellReason(m check.Model, kind, subject string, drawnComps, drawnNets map[string]bool, onSheet bool) checkspb.LocateReason {
	if kind == check.KindBus {
		if onSheet {
			return checkspb.LocateReason_LOCATE_REASON_UNSPECIFIED
		}
		return checkspb.LocateReason_LOCATE_REASON_BUS_NOT_DRAWN
	}
	drawn := drawnComps[subject]
	if kind == check.KindNet {
		drawn = drawnNets[subject]
	}
	// A pin is drawn if its COMPONENT is, and is explained by its component's facts. Asking about the
	// pin designator instead would answer about a thing called "5".
	if kind == check.KindPin {
		kind = check.KindComponent
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
// "component" (a ref_des), "net", "bus", or "" (a scalar or unresolved column). The engine reads it
// from what each relation and predicate DECLARES about its arguments, never their labels (agni issue
// 548), and follows derived relations into their bodies (agni issue 654). An aggregate column stays
// scalar even when it reduces an entity variable, since count(?ref) is a number.
//
// kindVars[i] names the variable whose per-row binding types column i, for a relation like
// `entity(?name, ?kind)` whose answer set mixes components, nets and buses (agni issue 338). It is
// "" for an ordinary column, and where it is set, kinds[i] is "". refs[i] is the term locating a
// column whose entity is found only through another, such as a pin through its component.
//
// A query the engine cannot type answers with plain columns rather than failing, since typing only
// decides which cells are clickable.
func columnKinds(q query.Query) (kinds []string, kindVars []query.Var, refs []query.Term) {
	return columnKindsIn(facts.DefaultRegistry(), q)
}

// columnKindsIn is columnKinds over an explicit vocabulary, the one a design's read composed with its
// project's own library, so a project member's columns type as clickable as a shipped one's.
func columnKindsIn(reg *facts.Registry, q query.Query) (kinds []string, kindVars []query.Var, refs []query.Term) {
	n := len(q.Select)
	if n == 0 {
		n = len(q.Columns())
	}
	kinds = make([]string, n)
	kindVars = make([]query.Var, n)
	refs = make([]query.Term, n)
	cks, err := query.ColumnKindsFrom(reg, q)
	if err != nil {
		return kinds, kindVars, refs
	}
	for i, ck := range cks {
		if i >= n {
			break
		}
		kinds[i], kindVars[i], refs[i] = ck.Kind, ck.KindFrom, ck.Owner
	}
	return kinds, kindVars, refs
}

// termValue resolves a term against one answer row. A constant yields itself and a variable yields
// its binding, which the row keeps even when the projection dropped the column.
func termValue(t query.Term, bind map[query.Var]query.Value) string {
	if t.Const != nil {
		return t.Const.S
	}
	if t.Var != "" {
		return bind[t.Var].S
	}
	return ""
}

// entityKind passes through the kinds a POLYMORPHIC column can take and rejects everything else.
// The vocabulary is check's. Pins stay out because entity() does not enumerate them; a pin cell is
// clickable through a pin column and its cell_refs instead (see columnKinds).
func entityKind(s string) string {
	switch s {
	case check.KindComponent, check.KindNet, check.KindBus:
		return s
	}
	return ""
}

// ListRelations returns the queryable relation catalog (WS9-037) for the panel's relation picker:
// the built-in relations, the shipped library's derived relations and the predicates, plus any
// overlay-registered relations, pre-sorted by kind then name (query.Catalog). It loads no design, so
// it never fails on a bad path and the client can fetch it once at startup.
//
// A request naming a path answers that one place in the namespace tree instead (describeEntry), and
// an unknown path is ErrInvalidArgument carrying the engine's suggestion.
func (s *QueryService) ListRelations(ctx context.Context, req *webapi.ListRelationsRequest) (*webapi.ListRelationsResponse, error) {
	reg, err := s.catalogRegistry(ctx, req.GetUri(), req.GetOverlay())
	if err != nil {
		return nil, err
	}
	if p := req.GetPath(); p != "" {
		e, err := describeEntry(reg, p)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
		}
		return &webapi.ListRelationsResponse{Entry: e}, nil
	}
	resp := &webapi.ListRelationsResponse{}
	for _, r := range query.CatalogFrom(reg) {
		info := &webapi.RelationInfo{
			Name:       r.Name,
			Args:       r.Args,
			Summary:    r.Summary,
			Kind:       r.Kind,
			Detail:     r.Detail,
			Definition: r.Definition,
		}
		if e, err := query.DescribeFrom(reg, r.Name); err == nil {
			info.Signature = e.Signature()
		}
		resp.Relations = append(resp.Relations, info)
	}
	for _, e := range query.EntityQueries() {
		resp.EntityQueries = append(resp.EntityQueries, &webapi.EntityQuery{Kind: e.Kind, Query: e.Query, Teaches: e.Teaches, Binds: e.Binds})
	}
	for _, e := range query.Examples() {
		resp.Examples = append(resp.Examples, &webapi.ExampleQuery{Label: e.Label, Query: e.Query, Teaches: e.Teaches})
	}
	sq := query.Search()
	resp.SearchQuery = &webapi.SearchQuery{Query: sq.Query, Teaches: sq.Teaches, Bind: sq.Bind, Pattern: sq.Pattern}
	return resp, nil
}

// portableCites rewrites a fact's provenance so it names the design the way the CALLER did, rather
// than the ABSOLUTE host path the loader handed the reader (agni issue 242). It keys on the design's
// own URI path, so it holds however the design was addressed. A cite not containing that path, such
// as a datasheet citation, is left alone.
func portableCites(cites []string, designPath string) []string {
	if designPath == "" || len(cites) == 0 {
		return cites
	}
	out := make([]string, len(cites))
	for i, c := range cites {
		if j := strings.Index(c, designPath); j > 0 {
			out[i] = c[j:]
		} else {
			out[i] = c
		}
	}
	return out
}

// catalogRegistry is the vocabulary a catalog request describes: the shipped one, or with a design
// named, that design's project's library joined to it, resolved exactly as a query on that design
// would resolve it.
func (s *QueryService) catalogRegistry(ctx context.Context, uri string, overlay *webapi.OverlayConfig) (*facts.Registry, error) {
	u, err := optionalArtifactURI(uri)
	if err != nil {
		return nil, err
	}
	if u.IsZero() {
		ov, err := ComposeOverlay(overlay, "")
		if err != nil {
			return nil, err
		}
		return ov.Registry()
	}
	ov, err := s.projects.Overlay(ctx, u, overlay, s.fallback, "")
	if err != nil {
		return nil, err
	}
	return ov.Registry()
}

// describeEntry is one place in the namespace tree as the wire carries it, with a module's direct
// children described one level deep. The CLI's `--relations <path>` prints this same message, so the
// two surfaces cannot describe a member differently.
func describeEntry(reg *facts.Registry, path string) (*webapi.RelationEntry, error) {
	if path == "." {
		path = "" // the wire's spelling of the root, since an empty path asks for the flat catalog
	}
	e, err := query.DescribeFrom(reg, path)
	if err != nil {
		return nil, err
	}
	out := entryProto(reg, e)
	for _, m := range e.Members {
		c, err := query.DescribeFrom(reg, m)
		if err != nil {
			return nil, err
		}
		out.Members = append(out.Members, entryProto(reg, c))
	}
	return out, nil
}

// entryProto converts one entry without its members. A member's reference markdown is attached
// where one exists, as the flat catalog attaches it.
func entryProto(reg *facts.Registry, e query.Entry) *webapi.RelationEntry {
	out := &webapi.RelationEntry{
		Path:       e.Path,
		EntryKind:  string(e.Kind),
		Doc:        e.Doc,
		Module:     e.Module,
		Definition: e.Definition,
	}
	if e.Kind == ns.EntryModule {
		return out
	}
	out.Signature = e.Signature()
	out.Detail = reg.Doc(e.Path)
	for _, a := range e.Args {
		if a.Inferred {
			out.Inferred = append(out.Inferred, a.Name)
		}
	}
	return out
}
