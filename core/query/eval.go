package query

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Base is the queryable fact base: the WS3-004 EDB relations indexed by name for lookup, plus the
// Model for intensional relations (reaches, computed via check.Model.Reach). Built once per design
// from a check.Model; a query re-uses it.
type Base struct {
	edb       map[string][]facts.Row
	netByName map[string]*ir.Net
	model     check.Model
	// reg is the relation vocabulary this base was built from. It is held rather than looked up
	// globally so a base and the schema it resolves against cannot disagree: a registration that
	// happens after NewBase must not change how an in-flight query reads.
	reg *facts.Registry
	// idb holds the derived (IDB) relations materialized from a query's user-defined rules, and
	// idbArity their positional arity (from the rule heads). Both are nil on the shared Base and
	// populated on a per-query shallow copy, so rules never leak between queries that reuse a Base.
	idb      map[string][]idbTuple
	idbArity map[string]int
	// edbIdx caches fact lookups by binding pattern (WS3-125), built lazily on first probe. It
	// belongs to the SHARED Base because the EDB is immutable for the Base's life, so a second
	// query over the same design reuses it — which is the case that matters for the profile
	// coverage pass, where many queries run against one Base.
	//
	// It is the only mutable state a query writes through a shared *Base, so it carries a lock.
	// Before this, concurrent Eval on one Base was safe by construction (everything shared was
	// read-only) and nothing should have to discover that it stopped being so. The lock is taken
	// once per atom extension, not per fact, which is precisely the cost this index removes.
	//
	// A POINTER, so Eval's shallow copy shares the one cache rather than copying a mutex (which vet
	// rejects, correctly: two copies of a lock guard nothing).
	edbIdx *edbIndexCache
	// idbIdx is the derived-relation analogue and is per-QUERY, living on the shallow copy beside
	// idb: a derived relation only exists for one query, so a cached index of it must not outlive
	// that query.
	idbIdx map[idxKey]*idbIndex
	// work counts candidate COMPARISONS: every fact or tuple the solver actually examines. It is
	// what went quadratic, and counting it rather than timing it gives the scaling guard a
	// deterministic signal — a duration assertion would be a flake generator across machines and CI
	// runners, and would say nothing about complexity. Behind a pointer so Eval's shallow copy
	// accumulates into the same counter.
	work *int64
}

// Work reports how many candidate comparisons the solver has performed against this Base. It exists
// for the scaling guard (WS3-125): assert the RATIO of work at n and 2n rather than any absolute
// number, so the test measures complexity instead of the machine it runs on.
func (b *Base) Work() int64 {
	if b.work == nil {
		return 0
	}
	return *b.work
}

// NewBase projects a Model into its fact base over the process-default relation vocabulary. Use
// NewBaseFrom to supply one explicitly.
//
// A binary that installs no relation catalog gets an EMPTY base, which answers nothing while looking
// exactly like a query that matched nothing — see Registry.Installed for the check that separates
// those.
func NewBase(m check.Model) *Base { return NewBaseFrom(facts.DefaultRegistry(), m) }

// NewBaseFrom projects a Model into its fact base over the given relation vocabulary and indexes it
// for querying. The registry is captured, so what this base can answer is fixed at construction.
func NewBaseFrom(reg *facts.Registry, m check.Model) *Base {
	b := &Base{edb: map[string][]facts.Row{}, netByName: map[string]*ir.Net{}, model: m, reg: reg, edbIdx: newEDBIndexCache(), work: new(int64)}
	for _, f := range reg.Rows(m) {
		b.edb[f.Relation] = append(b.edb[f.Relation], f)
	}
	for _, n := range m.Nets() {
		b.netByName[n.Name] = n
	}
	return b
}

// NewSpecLibBase builds a fact base over a whole seeded datasheet corpus with NO design (WS10-010): the
// datasheet relations (`param`, `part.audience`) project over every PartSpec the FactSource yields,
// so `agni query --speclib` searches the spec library instead of the parts joined to one design. There is no
// model (a spec library is not a design), so model-dependent relations and predicates (net.*, component.*,
// reaches) have no facts and yield nothing — a spec library query is over the datasheet relations only.
func NewSpecLibBase(fs param.FactSource) *Base {
	return NewSpecLibBaseFrom(facts.DefaultRegistry(), fs)
}

// NewSpecLibBaseFrom is NewSpecLibBase over an explicit relation vocabulary.
func NewSpecLibBaseFrom(reg *facts.Registry, fs param.FactSource) *Base {
	b := &Base{edb: map[string][]facts.Row{}, netByName: map[string]*ir.Net{}, reg: reg, edbIdx: newEDBIndexCache(), work: new(int64)}
	for _, f := range reg.SpecLibRows(fs.AllSpecs()) {
		b.edb[f.Relation] = append(b.edb[f.Relation], f)
	}
	return b
}

// Evaluator answers a Query over a Base, each answer Row carrying the provenance of the facts that
// derived it. The IR is declarative, so backends swap behind this interface (the naïve interpreter
// here; a SQL or external-engine backend later if scale ever demands).
type Evaluator interface {
	Eval(q Query, b *Base) ([]Row, error)
}

// Naive is the default backtracking-join interpreter over the in-memory fact base. Correct and
// dependency-free; naïve join is sufficient because one design's fact base is small (no optimizer,
// per WS3-004). It serves the full bounded fragment: conjunction, comparison, the built-in reaches
// and string filters, stratified negation, aggregation, user-defined (recursive, stratified) rules,
// and overlay-registered relations and predicates. Every callable — EDB, IDB, or a computed built-in
// — is one positive primitive (extendAtom), and negation reuses it (atomHolds), so a single dispatch
// covers all of them.
type Naive struct{}

// Eval answers the query. It solves the positive body (atoms + comparisons) by backtracking,
// filters each binding through the negated literals (stratified negation), then projects — a plain
// select-project, or a group-and-reduce when the projection contains an aggregate. Results are
// deduplicated and sorted, so a query is a deterministic, regenerable view; each row carries the
// provenance of the facts that produced it.
func (Naive) Eval(q Query, b *Base) ([]Row, error) {
	if len(q.Rules) > 0 {
		nb := *b // shallow copy: edb/netByName/model are read-only and shared; idb is fresh per query
		nb.idb = map[string][]idbTuple{}
		nb.idbArity = map[string]int{}
		// Fresh alongside idb, and for the same reason: an index of derived tuples describes THIS
		// query's derivations and must not be reachable from the next one. Copying the struct
		// carried the map header across, so leaving this out would have one query probing an index
		// whose positions point into another query's idb slice.
		nb.idbIdx = map[idxKey]*idbIndex{}
		if err := nb.materialize(q.Rules); err != nil {
			return nil, err
		}
		b = &nb
	}
	pos, negs := splitNegations(q.Goal.Literals)
	if err := b.validateNegations(q.Goal, negs); err != nil {
		return nil, err
	}
	sel := q.Select
	if len(sel) == 0 {
		sel = defaultSelect(q.Goal)
	}
	if err := validateSelect(sel, q.Goal); err != nil {
		return nil, err
	}

	var raw []*binding
	err := solve(pos, 0, newBinding(), b, func(bnd *binding) error {
		ok, err := passesNegations(bnd, negs, b)
		if err != nil {
			return err
		}
		if ok {
			raw = append(raw, bnd.clone())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if hasAggregate(sel) {
		return aggregate(sel, raw), nil
	}
	return projectRows(sel, raw), nil
}

// splitNegations separates the body into the positive part (atoms + comparisons, solved by
// backtracking) and the negated literals (applied as a post-solve filter). Splitting makes
// negation safe regardless of clause order: a negated literal's variables are bound by the time it
// is checked.
func splitNegations(lits []Literal) (pos, negs []Literal) {
	for _, lit := range lits {
		if lit.Neg != nil {
			negs = append(negs, lit)
		} else {
			pos = append(pos, lit)
		}
	}
	return pos, negs
}

// validateNegations rejects a negated literal over an unknown relation or with the wrong arity —
// caught here so a bad `not` fails clearly instead of silently never matching. Negation ranges over
// every callable relation uniformly: EDB, IDB, string filters, overlay predicates, AND reaches
// (negation as failure — atomHolds runs the same extendAtom the positive solve uses). Stratification
// (materialize) already guarantees a negated IDB relation is fully derived before the rule or goal
// that negates it runs, so the filter is safe.
func (b *Base) validateNegations(goal Body, negs []Literal) error {
	bound := map[Var]bool{}
	for _, v := range positiveVars(goal) {
		bound[v] = true
	}
	for _, lit := range negs {
		rel := lit.Neg.Relation
		ok, known := b.arityAccepts(rel, len(lit.Neg.Args))
		if !known {
			return fmt.Errorf("query: negation over unknown relation %q%s", rel, didYouMean(b.reg, rel))
		}
		if !ok {
			return fmt.Errorf("query: negated relation %q takes %s args, got %d", rel, b.arityLabelOf(rel), len(lit.Neg.Args))
		}
		if err := checkNegationAnchored(lit.Neg, bound); err != nil {
			return err
		}
	}
	return nil
}

// checkNegationAnchored rejects a negated atom that shares NO variable with the positive body, which
// is the unsafe-negation case (agni issue 522).
//
// Such a literal has nothing to range over per row, so it collapses to a design-wide constant:
// `entity(?n,"net"), not component.class(?tp,"test_point")` asks whether the design contains no test
// point at all, and on any board that has one it silently filters every row away. The author meant
// "nets with no test point", which needs a negated CONJUNCTION and is a separate gap. Returning zero
// rows for a question that was never asked is the worst available answer, because an empty result
// from a negation reads as a reassuring fact about the design.
//
// The rule is ANCHORING, not full safety, and the difference matters. Classic datalog safety demands
// every variable in a negated literal occur positively, which would reject the shape this language
// documents and people correctly rely on: `component-on-net(?r,?n), not component.mpn(?r,?m) => ?r`
// leaves ?m free ON PURPOSE, and means "no m exists for this r". That is well defined precisely
// because ?r anchors it. What is never meaningful is a negation anchored to nothing.
//
// A negated atom with no variables at all is ground and needs no anchor: `not component.class("U1",
// "test_point")` is a constant filter, and a legitimate one.
func checkNegationAnchored(a *Atom, bound map[Var]bool) error {
	var vars []Var
	for _, t := range a.Args {
		// "_" is the wildcard: it never binds and never anchors, so it does not count either way.
		if t.Var != "" && t.Var != "_" {
			vars = append(vars, t.Var)
		}
	}
	if len(vars) == 0 {
		return nil
	}
	for _, v := range vars {
		if bound[v] {
			return nil
		}
	}
	return fmt.Errorf("query: negated relation %q shares no variable with the rest of the query (?%s appears only inside the `not`, so the negation has nothing to range over and matches either every row or none)",
		a.Relation, vars[0])
}

// isIDB reports whether a relation is a rule-defined (IDB) relation in this query's materialized set.
func (b *Base) isIDB(rel string) bool {
	_, ok := b.idbArity[rel]
	return ok
}

// passesNegations keeps a binding only if every negated literal holds: `not R(args)` holds when R has
// NO solution under the current binding (negation as failure — atomHolds runs the same extendAtom the
// positive solve uses). A negated arg that is a constant or an already-bound variable must match; a
// variable appearing only under negation is a wildcard (existential — "no solution for any value").
//
// Example — `component.mpn(?r,?m), not param(?m,"VIN",?v)` (parts with no VIN datasheet param): the
// positive solve binds ?m; here, for that ?m, ?v is bound by nothing, so it is a wildcard. The
// binding is kept iff NO param fact has subject == the bound ?m and symbol == "VIN" — i.e. "?m has
// no VIN param at any value". A part whose ?m does have such a fact is dropped.
func passesNegations(bnd *binding, negs []Literal, b *Base) (bool, error) {
	for _, lit := range negs {
		matched, err := b.atomHolds(lit.Neg, bnd)
		if err != nil {
			return false, err
		}
		if matched {
			return false, nil // some solution exists, so the `not` is violated
		}
	}
	return true, nil
}

// binding is a partial solution: variable bindings plus the cites of the facts consumed so far.
type binding struct {
	vals  map[Var]Value
	cites []string
}

func newBinding() *binding { return &binding{vals: map[Var]Value{}} }

func (b *binding) clone() *binding {
	nv := make(map[Var]Value, len(b.vals))
	for k, v := range b.vals {
		nv[k] = v
	}
	return &binding{vals: nv, cites: append([]string(nil), b.cites...)}
}

// solve recurses over the goal literals: a positive atom fans out through extendAtom (the one
// primitive for every relation kind), a comparison prunes. emit is called for every complete
// binding; the first error it (or an atom) returns stops the search.
//
// Worked example — `component.mpn(?r,?m), param(?m,"VIN",?v), ?v < 30`:
//
//	i=0 component.mpn(?r,?m): for each component.mpn fact, bind ?r,?m -> recurse i=1
//	i=1 param(?m,"VIN",?v):   for each param fact whose subject == the bound ?m and symbol == "VIN",
//	                          bind ?v -> recurse i=2
//	i=2 ?v < 30:              keep the binding iff ?v < 30, else prune
//	i=3 (past the end):       emit the binding (its accumulated cites travel with it)
func solve(lits []Literal, i int, bnd *binding, b *Base, emit func(*binding) error) error {
	if i == len(lits) {
		return emit(bnd)
	}
	lit := lits[i]
	switch {
	case lit.Compare != nil:
		ok, err := evalCompare(*lit.Compare, bnd)
		if err != nil {
			return err
		}
		if ok {
			return solve(lits, i+1, bnd, b, emit)
		}
		return nil
	case lit.Neg != nil:
		// Unreachable: Eval splits negated literals out before solving (they are a post-solve
		// filter). A Neg here would be an internal error, not a user one.
		return fmt.Errorf("query: internal: negated literal reached the positive solver")
	case lit.Pos != nil:
		return b.extendAtom(lit.Pos, bnd, func(ext *binding) error {
			return solve(lits, i+1, ext, b, emit)
		})
	default:
		return fmt.Errorf("query: empty literal")
	}
}

// unify matches an atom's args against one fact, extending bnd: a constant must equal the field, a
// variable binds it (or must equal its existing binding). Returns the extended binding, or false.
//
// This is logic-programming pattern matching (as in Prolog/datalog), not function application: it
// is symmetric — an argument matches whether the value comes from the fact or from an existing
// binding — and it commits variables into the binding as a side result, closer to destructuring a
// value against a pattern than to calling a function with arguments.
func unify(args []Term, fields []facts.Field, f facts.Row, bnd *binding) (*binding, bool) {
	out := bnd.clone()
	for j, arg := range args {
		if !bindArg(out, arg, fieldValue(f, fields[j])) {
			return nil, false
		}
	}
	out.cites = append(out.cites, f.Cite)
	return out, true
}

// bindArg unifies one argument term with a value: a constant must equal it, a variable binds it (or
// must match its existing binding). "_" and the empty variable are wildcards.
func bindArg(bnd *binding, arg Term, val Value) bool {
	switch {
	case arg.Const != nil:
		return valueEq(val, *arg.Const)
	case arg.Var == "" || arg.Var == "_":
		return true
	default:
		if bound, ok := bnd.vals[arg.Var]; ok {
			return valueEq(val, bound)
		}
		bnd.vals[arg.Var] = val
		return true
	}
}

// resolve reads a term's value under the binding: a constant is itself, a bound variable its
// binding. ok is false for an unbound variable.
func resolve(t Term, bnd *binding) (Value, bool) {
	if t.Const != nil {
		return *t.Const, true
	}
	val, ok := bnd.vals[t.Var]
	return val, ok
}

// valueEq compares two values: numeric when both carry a number, string otherwise.
//
// ABSENT UNIFIES ONLY WITH ABSENT, which is what stops it colliding with the empty string. Two
// unstated bounds ARE the same answer to "what does this row state", so this is true rather than
// SQL's UNKNOWN. That is a deliberate deviation: full three-valued logic would have to thread UNKNOWN
// through negation, aggregation and the index, and "both unstated" is the reading an engineer running
// a search actually wants.
func valueEq(a, b Value) bool {
	if a.Absent || b.Absent {
		return a.Absent && b.Absent
	}
	if a.Num != nil && b.Num != nil {
		return *a.Num == *b.Num
	}
	return a.S == b.S
}

// orderingOps are the comparisons that ask which of two values is LARGER. Equality and inequality are
// deliberately not here: asking whether two values are the same is meaningful across kinds (a number
// and a word are simply not equal), while asking which is larger is not.
var orderingOps = map[string]bool{"<": true, "<=": true, ">": true, ">=": true}

// evalCompare evaluates a comparison once both operands are bound: numeric when both carry a number,
// string otherwise.
//
// THREE REFUSALS, and they are the point of this function rather than details of it. Each answers a
// question that HAS no answer, and each previously answered it anyway.
//
// 1. AN ABSENT OPERAND IS NOT COMPARABLE. A datasheet row stating only a maximum leaves its minimum
// absent. Before Value.Absent existed such a field bound to the EMPTY STRING, and ordering fell
// through to cmpStr and answered by LEXICOGRAPHY:
//
//	"" <= "5.0"  -> true      an absent lower bound passes a lower-bound test
//	"" >= "3.0"  -> false     the same absent bound, opposite phrasing, opposite answer
//	"5.0" <= ""  -> false     an absent upper bound fails an upper-bound test
//	"" <= "-2"   -> true      an absent bound "passes" against MINUS TWO, since "" precedes everything
//
// The answer depended on how the author phrased the inequality and on the sign of the constant. This
// now refuses on the FLAG rather than on a nil Num, which matters because a non-numeric string also
// has a nil Num: inferring absence from that coincidence conflated two different things.
//
// 2. A NUMBER AND A NON-NUMBER HAVE NO ORDER. `?name < 5` is a question about nothing, and cmpStr
// used to answer it by comparing "ALPHA" against "5".
//
// 3. UNLIKE DIMENSIONS HAVE NO ORDER. Volts are not smaller or larger than amps. Both sides must
// carry a base unit for this to fire, because a bare literal cannot state one: `?vmax < 5.0` has to
// keep working, so an empty BaseUnit is polymorphic rather than a dimension of its own. SCALE is not
// this layer's problem and never reaches it (param.InBaseUnit, C24), so this compares "V" against "A"
// and never "mV" against "V".
//
// ALL THREE EVALUATE TO NO MATCH RATHER THAN AN ERROR. An error aborts the whole query, so one
// max-only row among many would make a legitimate range query unusable. No-match leaves that row
// unjudged, which is this engine's posture everywhere else: silence means "I could not tell", never
// "this is fine". An author who wants an absent lower bound to count as unbounded-below writes that
// clause explicitly instead of inheriting it from string ordering.
//
// EQUALITY IS DELIBERATELY NOT DIMENSION-CHECKED, and ordering two non-numbers is untouched
// (`?name < "M"` still splits alphabetically). Equality here is the author's explicit operator, but
// the same values also unify implicitly when a variable repeats across atoms, and unification is
// identity rather than physics. Making one unit-aware and not the other would be incoherent, and
// making both would break joins and the fact index, which bucket by string value.
func evalCompare(c Compare, bnd *binding) (bool, error) {
	l, okl := resolve(c.Left, bnd)
	r, okr := resolve(c.Right, bnd)
	if !okl || !okr {
		return false, fmt.Errorf("query: comparison operand is unbound (a variable must appear in a relation before it is compared)")
	}
	if l.Absent || r.Absent {
		// An unstated value has no ORDER, but it does have an IDENTITY: two unstated bounds are the
		// same answer to "what does this row state". Routing equality through valueEq keeps the
		// explicit operator and implicit unification agreeing, which is the property that stops
		// `?a = ?b` and a repeated `?a` meaning different things.
		if orderingOps[c.Op] {
			return false, nil
		}
		eq := valueEq(l, r)
		if c.Op == "!=" {
			return !eq, nil
		}
		return eq, nil
	}
	if l.Num != nil && r.Num != nil {
		if orderingOps[c.Op] && l.BaseUnit != "" && r.BaseUnit != "" && l.BaseUnit != r.BaseUnit {
			return false, nil
		}
		return cmpNum(*l.Num, c.Op, *r.Num), nil
	}
	if orderingOps[c.Op] && (l.Num != nil) != (r.Num != nil) {
		return false, nil
	}
	return cmpStr(l.S, c.Op, r.S), nil
}

func cmpNum(a float64, op string, b float64) bool {
	switch op {
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	case "!=":
		return a != b
	default: // "="
		return a == b
	}
}

func cmpStr(a, op, b string) bool {
	switch op {
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	case "!=":
		return a != b
	default: // "="
		return a == b
	}
}

// Columns returns the answer column order: the explicit Select, or the goal's variables in
// first-seen order when Select is empty. A caller printing a result table uses this for the header
// and the per-row order (Row.Bind is a map).
func (q Query) Columns() []Var {
	sel := q.Select
	if len(sel) == 0 {
		sel = defaultSelect(q.Goal)
	}
	return selKeys(sel)
}

// colLabel is a select item's column identifier: the variable, or "func(var)" for an aggregate.
// It is also the key an aggregate result is stored under in Row.Bind, so printing keys both alike.
func colLabel(t Term) Var {
	if t.Agg != nil {
		return Var(t.Agg.Func + "(" + string(t.Agg.Var) + ")")
	}
	return t.Var
}

func selKeys(sel []Term) []Var {
	out := make([]Var, 0, len(sel))
	for _, t := range sel {
		out = append(out, colLabel(t))
	}
	return out
}

// defaultSelect is the projection when none is given: the positive goal variables, first-seen. Only
// positive variables — a variable used only under negation is existential and not selectable.
func defaultSelect(goal Body) []Term {
	out := make([]Term, 0)
	for _, vv := range positiveVars(goal) {
		out = append(out, Term{Var: vv})
	}
	return out
}

// positiveVars returns the variables bound by positive atoms (including reaches), first-seen.
func positiveVars(goal Body) []Var {
	seen := map[Var]bool{}
	var out []Var
	for _, lit := range goal.Literals {
		if lit.Pos == nil {
			continue
		}
		for _, a := range lit.Pos.Args {
			if a.Var != "" && a.Var != "_" && !seen[a.Var] {
				seen[a.Var] = true
				out = append(out, a.Var)
			}
		}
	}
	return out
}

// validateSelect rejects a projection over a variable no positive relation binds (an unknown or
// negation-only existential) and an unknown aggregate function — so a bad query errors clearly.
func validateSelect(sel []Term, goal Body) error {
	pv := map[Var]bool{}
	for _, vv := range positiveVars(goal) {
		pv[vv] = true
	}
	for _, t := range sel {
		switch {
		case t.Agg != nil:
			if !validAggFunc(t.Agg.Func) {
				return fmt.Errorf("query: unknown aggregate %q (want count/min/max/sum)", t.Agg.Func)
			}
			if t.Agg.Var != "" && !pv[t.Agg.Var] {
				return fmt.Errorf("query: %s aggregates ?%s, which no relation binds", t.Agg.Func, t.Agg.Var)
			}
		case t.Var != "" && !pv[t.Var]:
			return fmt.Errorf("query: projected ?%s is not bound by a positive relation (a variable used only under negation is existential and cannot be selected)", t.Var)
		}
	}
	return nil
}

func validAggFunc(f string) bool {
	switch f {
	case "count", "min", "max", "sum":
		return true
	}
	return false
}

func hasAggregate(sel []Term) bool {
	for _, t := range sel {
		if t.Agg != nil {
			return true
		}
	}
	return false
}

// projectRows is the plain select-project: one output row per solved binding, columns = the select
// variables, cites deduped.
func projectRows(sel []Term, raw []*binding) []Row {
	rows := make([]Row, 0, len(raw))
	for _, bnd := range raw {
		row := Row{Bind: make(map[Var]Value, len(sel)), Cites: dedupStrings(bnd.cites)}
		for _, t := range sel {
			if t.Var != "" {
				row.Bind[t.Var] = bnd.vals[t.Var]
			}
		}
		rows = append(rows, row)
	}
	return dedupSort(rows, selKeys(sel))
}

// aggregate groups the solved bindings by the select's variable columns and reduces each aggregate
// column over the group. A group's provenance is the union of its rows' cites.
//
// Example — `component-on-net(?ref,?net) => ?net, count(?ref)`: the solve yields one binding per
// (ref,net) fact; grouping by ?net collapses them per net, and count(?ref) is the group size — parts
// per net. min/max/sum reduce the numeric value of their variable over the group instead.
func aggregate(sel []Term, raw []*binding) []Row {
	var keyVars []Var
	var aggs []Term
	for _, t := range sel {
		if t.Agg != nil {
			aggs = append(aggs, t)
		} else if t.Var != "" {
			keyVars = append(keyVars, t.Var)
		}
	}
	type group struct {
		keyVals map[Var]Value
		rows    []*binding
	}
	groups := map[string]*group{}
	for _, bnd := range raw {
		key := groupKeyOf(keyVars, bnd)
		g := groups[key]
		if g == nil {
			g = &group{keyVals: map[Var]Value{}}
			for _, kv := range keyVars {
				g.keyVals[kv] = bnd.vals[kv]
			}
			groups[key] = g
		}
		g.rows = append(g.rows, bnd)
	}
	var out []Row
	for _, g := range groups {
		row := Row{Bind: map[Var]Value{}}
		var cites []string
		for _, kv := range keyVars {
			row.Bind[kv] = g.keyVals[kv]
		}
		for _, bnd := range g.rows {
			cites = append(cites, bnd.cites...)
		}
		row.Cites = dedupStrings(cites)
		for _, a := range aggs {
			row.Bind[colLabel(a)] = reduce(*a.Agg, g.rows)
		}
		out = append(out, row)
	}
	return dedupSort(out, selKeys(sel))
}

func groupKeyOf(keyVars []Var, bnd *binding) string {
	var b strings.Builder
	for _, kv := range keyVars {
		b.WriteString(bnd.vals[kv].S)
		b.WriteByte('\x1f')
	}
	return b.String()
}

// reduce computes one aggregate over a group's bindings: count is the row count; min/max/sum are
// over the numeric value of the aggregated variable (rows whose value is non-numeric are skipped).
func reduce(a Aggregate, rows []*binding) Value {
	if a.Func == "count" {
		n := float64(len(rows))
		return Value{S: ftoa(n), Num: &n}
	}
	var nums []float64
	for _, bnd := range rows {
		if val, ok := bnd.vals[a.Var]; ok && val.Num != nil {
			nums = append(nums, *val.Num)
		}
	}
	if len(nums) == 0 {
		return Value{}
	}
	r := nums[0]
	switch a.Func {
	case "min":
		for _, x := range nums[1:] {
			if x < r {
				r = x
			}
		}
	case "max":
		for _, x := range nums[1:] {
			if x > r {
				r = x
			}
		}
	case "sum":
		r = 0
		for _, x := range nums {
			r += x
		}
	}
	return Value{S: ftoa(r), Num: &r}
}

// dedupSort removes duplicate answer rows (same projected values) and sorts them, so a query is a
// deterministic view.
func dedupSort(rows []Row, sel []Var) []Row {
	sort.SliceStable(rows, func(i, j int) bool { return rowKey(rows[i], sel) < rowKey(rows[j], sel) })
	out := rows[:0]
	var last string
	for i, r := range rows {
		if k := rowKey(r, sel); i == 0 || k != last {
			out = append(out, r)
			last = k
		}
	}
	return out
}

func rowKey(r Row, sel []Var) string {
	var b strings.Builder
	for _, v := range sel {
		b.WriteString(string(v))
		b.WriteByte('=')
		b.WriteString(r.Bind[v].S)
		b.WriteByte('\x1f')
	}
	return b.String()
}

func dedupStrings(ss []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }
