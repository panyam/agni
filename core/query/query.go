// Package query is the design query surface (WS3-029): a small declarative datalog over the
// WS3-004 fact base (check.Facts), so an engineer runs ad-hoc queries — "search your whole design
// as relations, including datasheets" — and every answer carries the provenance of the facts that
// produced it. Rules and search share one vocabulary: a rule asserts a property over these
// relations; a search is an arbitrary query over the same ones.
//
// The IR is datalog, not relational algebra: circuits are graph-structured (the core queries are
// transitive closures), and a declarative logical query says WHAT not HOW, so the Evaluator behind
// this IR is swappable (our naïve interpreter here; a SQL or external-engine backend later, if
// scale ever demands). The IR admits the full bounded primitive set — join, union, comparison,
// stratified negation, bounded recursion, aggregation — even where the evaluator fills them in
// incrementally, because the IR is the expensive-to-change layer.
//
// The package stays WASM-clean (imports only check + the generated IR, never os/net/cgo), so the
// whole chain read → Model → Facts → query runs client-side in the browser.
package query

// A Query is a datalog program answered over a fact base: Rules define derived (IDB) relations and
// Goal is the conjunction to solve. Goal's free variables — narrowed by Select — are the answer
// columns. EDB relations come from the fact base (relations.Facts) via the edb schema; IDB relations
// come from Rules; a handful of built-in relations (reaches) are computed from the Model.
//
// The whole IR is what Parse produces. For the query text
//
//	component.mpn(?r,?m), net.max_voltage(?n,?v), ?v < 30 => ?r, ?n
//
// the Query is: no Rules; Goal.Literals = [ Atom component.mpn(?r,?m), Atom net.max_voltage(?n,?v),
// Compare(?v < 30) ]; Select = [?r, ?n].
type Query struct {
	Rules []Rule
	Goal  Body
	// Select is the answer columns: each item is a variable (a group key) or an aggregate over the
	// group formed by the variable columns (count/min/max/sum). Empty selects Goal's variables in
	// first-seen order. When any item is an aggregate, the result is grouped-and-reduced.
	Select []Term
}

// A Rule derives its Head for every binding satisfying Body. A rule is recursive when its Head
// relation is reachable from a Body atom in the rule dependency graph; recursion terminates by
// finiteness of the fact base (no function symbols). Rules materialize by stratified fixpoint
// (query.materialize): a program that reads a relation under negation from inside that relation's own
// recursive cycle is rejected, which is what keeps `not` well-defined.
//
// Example (transitive closure over a derived edge): connected(?a,?c) :- connected(?a,?b), link(?b,?c)
// is Rule{Head: Atom connected(?a,?c), Body: [Atom connected(?a,?b), Atom link(?b,?c)]}.
type Rule struct {
	Head Atom
	Body Body
	Hops int // 0 = run to fixpoint; >0 = bound recursion depth (reserved; the fixpoint is finite regardless)
}

// A Body is an implicit conjunction (AND) of Literals. Disjunction (OR) is several Rules sharing
// one Head relation — never an OR node — so the program stays stratifiable. Example: the body of
// `a(?x), b(?x,?y), ?y < 5` is Body{Literals: [Atom a, Atom b, Compare]}.
type Body struct{ Literals []Literal }

// A Literal is exactly one of: a positive atom, a negated atom (stratified negation), or a
// comparison built-in. Exactly one field is non-nil. Examples: `param(?m,"VIN",?v)` is
// Literal{Pos: &Atom{...}}; `not param(?m,"VIN",?v)` is Literal{Neg: ...}; `?v < 30` is
// Literal{Compare: ...}.
type Literal struct {
	Pos     *Atom
	Neg     *Atom
	Compare *Compare
}

// An Atom applies a relation to argument terms: Relation(Args...). Relation is an EDB name
// (check.Rel*), a built-in (reaches), or an IDB name a Rule defines. Example: `component.mpn(?r,?m)`
// is Atom{Relation: "component.mpn", Args: [Var("r"), Var("m")]}.
type Atom struct {
	Relation string
	Args     []Term
}

// A Compare is a built-in predicate over two terms, evaluated once both are bound: Left Op Right,
// Op in {<, <=, =, !=, >, >=}. Numeric when both bound values carry a number, string otherwise.
type Compare struct {
	Left  Term
	Op    string
	Right Term
}

// A Term is a variable, a constant, or (in a Rule Head only) an aggregate over the group formed by
// the Head's other variables. Exactly one of Var/Const/Agg is set (Var == "" means not a variable).
type Term struct {
	Var   Var
	Const *Value
	Agg   *Aggregate
}

// Var is a logic variable name (the leading "?" is stripped at parse time).
type Var string

// A Value is a scalar fact value. A fact carries a string and, when numeric, a number, so a bound
// term keeps both: string equality and numeric comparison both work with no re-parse.
type Value struct {
	S   string
	Num *float64
}

// An Aggregate reduces Var over each group of the projection's plain-variable columns. Example:
// `component-on-net(?ref,?net) => ?net, count(?ref)` groups by ?net and counts the ?ref bindings
// per group (parts per net); Aggregate{Func: "count", Var: "ref"}. min/max/sum reduce Var's numeric
// value over the group.
type Aggregate struct {
	Func string // count | min | max | sum
	Var  Var
}

// Row is one answer: the projected variables bound to values, plus the provenance of the base
// facts that produced it — so an answer stays verifiable.
type Row struct {
	Bind  map[Var]Value
	Cites []string
}

// v builds a variable term; k builds a constant string term. Kept unexported helpers for tests and
// the parser to construct queries without the struct noise.
func v(name string) Term { return Term{Var: Var(name)} }
func k(s string) Term    { return Term{Const: &Value{S: s}} }
func num(f float64) Term { return Term{Const: &Value{S: ftoa(f), Num: &f}} }
