package query

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/datasheet/param"
)

// Base is the queryable fact base: the WS3-004 EDB relations indexed by name for lookup, plus the
// Model for intensional relations (reaches, computed via check.Model.Reach). Built once per design
// from a check.Model; a query re-uses it.
type Base struct {
	edb       map[string][]check.FactRow
	netByName map[string]*ir.Net
	model     check.Model
	// idb holds the derived (IDB) relations materialized from a query's user-defined rules, and
	// idbArity their positional arity (from the rule heads). Both are nil on the shared Base and
	// populated on a per-query shallow copy, so rules never leak between queries that reuse a Base.
	idb      map[string][]idbTuple
	idbArity map[string]int
}

// NewBase projects a Model into its fact base (check.Facts plus any overlay-registered relations)
// and indexes it for querying.
func NewBase(m check.Model) *Base {
	b := &Base{edb: map[string][]check.FactRow{}, netByName: map[string]*ir.Net{}, model: m}
	for _, f := range check.Facts(m) {
		b.edb[f.Relation] = append(b.edb[f.Relation], f)
	}
	for _, name := range registryOrder { // overlay relations, keyed by their registered name
		for _, f := range registry[name].project(m) {
			b.edb[name] = append(b.edb[name], f)
		}
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
	b := &Base{edb: map[string][]check.FactRow{}, netByName: map[string]*ir.Net{}}
	for _, f := range check.SpecLibFacts(fs.AllSpecs()) {
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
		if err := nb.materialize(q.Rules); err != nil {
			return nil, err
		}
		b = &nb
	}
	pos, negs := splitNegations(q.Goal.Literals)
	if err := b.validateNegations(negs); err != nil {
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
func (b *Base) validateNegations(negs []Literal) error {
	for _, lit := range negs {
		rel := lit.Neg.Relation
		arity, ok := b.arityOf(rel)
		if !ok {
			return fmt.Errorf("query: negation over unknown relation %q%s", rel, didYouMean(rel))
		}
		if len(lit.Neg.Args) != arity {
			return fmt.Errorf("query: negated relation %q takes %d args, got %d", rel, arity, len(lit.Neg.Args))
		}
	}
	return nil
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
func unify(args []Term, fields []edbField, f check.FactRow, bnd *binding) (*binding, bool) {
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
func valueEq(a, b Value) bool {
	if a.Num != nil && b.Num != nil {
		return *a.Num == *b.Num
	}
	return a.S == b.S
}

// evalCompare evaluates a comparison once both operands are bound: numeric when both carry a number.
func evalCompare(c Compare, bnd *binding) (bool, error) {
	l, okl := resolve(c.Left, bnd)
	r, okr := resolve(c.Right, bnd)
	if !okl || !okr {
		return false, fmt.Errorf("query: comparison operand is unbound (a variable must appear in a relation before it is compared)")
	}
	if l.Num != nil && r.Num != nil {
		return cmpNum(*l.Num, c.Op, *r.Num), nil
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
