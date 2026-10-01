package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/datasheet/param"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
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
	return &QueryService{loader: loader, eval: query.Naive{}, specs: specs, projects: projects}
}

// RunQuery loads the design, parses and evaluates the datalog query over its fact base, and returns
// the projected columns and answer rows with provenance. A geometry-only file with no netlist and a
// malformed query are both invalid arguments, so the panel shows the parse error inline. A query
// that matches nothing returns an empty row set, not an error.
func (s *QueryService) RunQuery(ctx context.Context, req *webapi.RunQueryRequest) (*webapi.RunQueryResponse, error) {
	u, err := artifactURI(req.GetUri())
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
	resp, err := s.answer(ctx, d, q, req.GetQuery())
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	return resp, nil
}

// RunQueries answers every query of a set over one read of the design (agni issue 729). Each
// result is what RunQuery would return for that query alone; a query that does not parse or names
// something the design's relations lack is reported against its name, and the others still answer.
// A set that is unusable as a whole is an invalid argument, and a design that cannot be read fails
// the call as it would fail RunQuery.
func (s *QueryService) RunQueries(ctx context.Context, req *webapi.RunQueriesRequest) (*webapi.RunQueriesResponse, error) {
	u, err := artifactURI(req.GetUri())
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
	out := &webapi.RunQueriesResponse{Title: set.Title, Preamble: set.Preamble, Source: req.GetUri()}
	for i, nq := range set.Queries {
		res := &webapi.NamedQueryResult{Name: nq.Name, Description: nq.Description}
		out.Results = append(out.Results, res)
		q, err := set.Compile(i)
		if err != nil {
			res.Error = err.Error()
			continue
		}
		resp, err := s.answer(ctx, d, q, nq.Query)
		if err != nil {
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
		s.Queries = append(s.Queries, query.NamedQuery{Name: q.GetName(), Query: q.GetQuery(), Description: q.GetDescription()})
	}
	return s
}

// QuerySetProto converts a query set to its wire form, for a caller that read one from a file.
func QuerySetProto(s query.QuerySet) *webapi.QuerySet {
	p := &webapi.QuerySet{Title: s.Title, Preamble: s.Preamble}
	for _, q := range s.Queries {
		p.Queries = append(p.Queries, &webapi.NamedQuery{Name: q.Name, Query: q.Query, Description: q.Description})
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
	return &designRead{u: u, source: source, model: model, base: query.NewBase(model), ov: ov, gu: gu}, nil
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
func (s *QueryService) answer(ctx context.Context, d *designRead, q query.Query, queryText string) (*webapi.RunQueryResponse, error) {
	rows, err := s.eval.Eval(q, d.base)
	if err != nil {
		return nil, err
	}
	cols := q.Columns()
	kinds, kindVars, refTerms := columnKinds(q)
	// Query and Source come from the request rather than being re-derived, so a saved response
	// cannot describe a different run than the one that produced these rows.
	resp := &webapi.RunQueryResponse{
		Columns: make([]string, len(cols)), ColumnKinds: kinds,
		Query: queryText, Source: d.source,
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
// "component" (a ref_des), "net", "bus", or "" (a scalar or unresolved column). It reads what each
// relation DECLARES about its arguments (facts.RelationInfo.ArgKinds), never its arg labels (agni
// issue 548). An aggregate or constant column stays scalar even when it reduces an entity variable,
// since count(?ref) is a number.
//
// kindVars[i] names the variable whose per-row binding types column i, for a relation like
// `entity(?name, ?kind)` whose answer set mixes components, nets and buses (agni issue 338). It is
// "" for an ordinary column, and where it is set, kinds[i] is "".
func columnKinds(q query.Query) (kinds []string, kindVars []query.Var, refs []query.Term) {
	decls := catalogArgDecls()
	terms := q.Select
	if len(terms) == 0 {
		for _, col := range q.Columns() {
			terms = append(terms, query.Term{Var: col})
		}
	}
	kinds = make([]string, len(terms))
	kindVars = make([]query.Var, len(terms))
	refs = make([]query.Term, len(terms))
	for i, t := range terms {
		if t.Agg == nil && t.Var != "" {
			kinds[i], kindVars[i], refs[i] = varKind(t.Var, q.Goal, decls, q.Rules, nil)
		}
	}
	return kinds, kindVars, refs
}

// varKind returns the entity kind a variable resolves to, or the variable whose per-row binding
// carries it. It walks the positive body atoms and takes the first entity-yielding binding, so a
// variable used as a net in one atom and a scalar in another is a net; a variable bound only in
// scalar positions is a scalar.
//
// A variable the catalog cannot type is followed into the USER RULES that bind it (agni issue 654),
// so a derived relation keeps the kind its body established. That walk has three properties, each
// wrong in the obvious implementation:
//
//   - It follows MORE THAN ONE HOP, since a rule defined through another rule is ordinary. `seen`
//     bounds it, because a rule set may be recursive.
//   - Rules that DISAGREE about a head position yield a scalar rather than the first one written,
//     which would be wrong for half the rows.
//   - A rule wrapping `entity(?name, ?kind)` stays scalar, because a head argument has no per-row
//     identity to carry kindVars through.
//
// Every branch reads a DECLARATION, never the catalog's arg labels (agni issue 548).
func varKind(col query.Var, body query.Body, decls map[string]relArgDecl, rules []query.Rule, seen map[string]bool) (string, query.Var, query.Term) {
	for _, lit := range body.Literals {
		a := lit.Pos
		if a == nil {
			continue
		}
		d, ok := decls[a.Relation]
		if !ok {
			continue
		}
		for j, term := range a.Args {
			if j >= len(d.labels) || term.Var != col {
				continue
			}
			k, ok := d.kinds[d.labels[j]]
			if !ok {
				continue // the relation declares this argument as a scalar
			}
			// A polymorphic column takes its kind from the VALUE another column binds, per row.
			if k.KindArg != "" {
				if arg, ok := argAt(a, d.labels, k.KindArg); ok {
					switch {
					case arg.Var != "":
						return "", arg.Var, query.Term{}
					case arg.Const != nil:
						// A constant types the column statically, so "find a net by name" gives a
						// plain net column. A value outside entityKind's vocabulary stays a scalar.
						if ek := entityKind(arg.Const.S); ek != "" {
							return ek, "", query.Term{}
						}
					}
				}
				continue
			}
			// A pin is locatable only through its owning component. A declared owner argument the
			// relation lacks means the declaration is wrong, and the column stays untyped.
			if k.OwnerArg != "" {
				if ref, ok := argAt(a, d.labels, k.OwnerArg); ok {
					return k.Entity, "", ref
				}
				continue
			}
			return k.Entity, "", query.Term{}
		}
	}
	return kindThroughRules(col, body, decls, rules, seen)
}

// kindThroughRules resolves a variable the catalog could not type by following the user rules that
// define the relations binding it. It runs only after the catalog walk fails, so a variable the
// catalog can type is unaffected.
func kindThroughRules(col query.Var, body query.Body, decls map[string]relArgDecl, rules []query.Rule, seen map[string]bool) (string, query.Var, query.Term) {
	if len(rules) == 0 {
		return "", "", query.Term{}
	}
	for _, lit := range body.Literals {
		a := lit.Pos
		if a == nil || seen[a.Relation] {
			continue
		}
		for j, term := range a.Args {
			if term.Var != col {
				continue
			}
			kind, ref, ok := headKind(a.Relation, j, decls, rules, seen)
			if ok && kind != "" {
				return kind, "", ref
			}
		}
	}
	return "", "", query.Term{}
}

// headKind types argument j of a derived relation by asking every rule that defines it, and returns a
// kind only when they agree. A rule binding the position to a per-row kind (kindVars) counts as
// DISAGREEMENT, so the column is a scalar.
func headKind(rel string, j int, decls map[string]relArgDecl, rules []query.Rule, seen map[string]bool) (string, query.Term, bool) {
	next := make(map[string]bool, len(seen)+1)
	for k := range seen {
		next[k] = true
	}
	next[rel] = true

	var kind string
	var ref query.Term
	found := false
	for _, r := range rules {
		if r.Head.Relation != rel || j >= len(r.Head.Args) {
			continue
		}
		hv := r.Head.Args[j].Var
		if hv == "" {
			return "", query.Term{}, false // a constant head argument types nothing
		}
		// A rule reaching back into one already being resolved ABSTAINS rather than vetoing, so a
		// transitive closure is typed by its base case instead of collapsing to a scalar.
		if bodyTouches(r.Body, next) {
			continue
		}
		k, kv, rf := varKind(hv, r.Body, decls, rules, next)
		if k == "" || kv != "" {
			return "", query.Term{}, false // untypeable or per-row, so the head is scalar
		}
		if found && k != kind {
			return "", query.Term{}, false // two rules, two kinds: scalar rather than first-wins
		}
		kind, ref, found = k, rf, true
	}
	return kind, ref, found
}

// bodyTouches reports whether any positive literal names a relation currently being resolved, which
// is how a cycle is recognised without walking into it.
func bodyTouches(body query.Body, seen map[string]bool) bool {
	for _, lit := range body.Literals {
		if lit.Pos != nil && seen[lit.Pos.Relation] {
			return true
		}
	}
	return false
}

// relArgDecl is one relation's argument labels and what they denote.
type relArgDecl struct {
	labels []string
	kinds  map[string]facts.ArgKind
}

// catalogArgDecls indexes the relation catalog by name. A relation with no declared argument kinds
// still appears, so a column of a known relation types as a scalar rather than as unknown.
func catalogArgDecls() map[string]relArgDecl {
	m := make(map[string]relArgDecl, len(query.Catalog()))
	for _, ri := range query.Catalog() {
		m[ri.Name] = relArgDecl{labels: ri.Args, kinds: ri.ArgKinds}
	}
	return m
}

// argAt returns the atom's argument at the position the catalog labels `label`, and whether the
// relation declares one at all.
func argAt(a *query.Atom, labels []string, label string) (query.Term, bool) {
	for j, l := range labels {
		if l == label && j < len(a.Args) {
			return a.Args[j], true
		}
	}
	return query.Term{}, false
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
// clickable through a pin column and its cell_refs instead (see varKind).
func entityKind(s string) string {
	switch s {
	case check.KindComponent, check.KindNet, check.KindBus:
		return s
	}
	return ""
}

// ListRelations returns the queryable relation catalog (WS9-037) for the panel's relation picker:
// the built-in relations and predicates plus any overlay-registered relations, pre-sorted by kind
// then name (query.Catalog). It loads no design, so it never fails on a bad path and the client can
// fetch it once at startup.
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
	for _, e := range query.EntityQueries() {
		resp.EntityQueries = append(resp.EntityQueries, &webapi.EntityQuery{Kind: e.Kind, Query: e.Query, Teaches: e.Teaches})
	}
	for _, e := range query.Examples() {
		resp.Examples = append(resp.Examples, &webapi.ExampleQuery{Label: e.Label, Query: e.Query, Teaches: e.Teaches})
	}
	sq := query.Search()
	resp.SearchQuery = &webapi.SearchQuery{Query: sq.Query, Teaches: sq.Teaches}
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
