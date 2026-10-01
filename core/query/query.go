// Package query is the design query surface (WS3-029): Datalog over the design's fact base, so an
// engineer runs ad-hoc queries ("search your whole design as relations, including datasheets") and
// every answer carries the provenance of the facts that produced it. Rules and search share one
// vocabulary, since a rule asserts a property over these relations and a search is an arbitrary query
// over the same ones.
//
// The engine itself lives in github.com/panyam/jaala/datalog (agni issue 731), which knows nothing
// about circuits. This package is agni's adapter over it:
//
//   - a Source projecting a check.Model through a facts.Registry, so every relation the fact layer
//     registers is queryable (source.go);
//   - agni's own uses of queries: RuleFromQuery, the wire form, the catalog, and the teaching
//     examples.
//
// Every name a query calls (relations, predicates, modules) is registered in the fact layer's
// vocabulary (facts.Registry.Vocabulary), so this package adds no names of its own. The circuit walks
// net.reaches and net.route live with the relations, in stdlib/relations.
//
// The types are aliases for the engine's, so a Query built here is the engine's Query and nothing
// converts at the boundary.
//
// The package stays WASM-clean (the engine imports only the standard library, and this adapter only
// check, facts and the generated IR), so the whole chain read → Model → facts → query runs
// client-side in the browser.
package query

import (
	"github.com/panyam/jaala/datalog"
	"github.com/panyam/jaala/ns"
)

// The query IR. See the engine's documentation for each; they are aliases, not copies.
type (
	Query     = datalog.Query
	Rule      = datalog.Rule
	Body      = datalog.Body
	Literal   = datalog.Literal
	Atom      = datalog.Atom
	Compare   = datalog.Compare
	Term      = datalog.Term
	Var       = datalog.Var
	Value     = ns.Value
	Aggregate = datalog.Aggregate
	Row       = datalog.Row
	// Base is a design's queryable fact base. Build one with NewBase.
	Base = datalog.Base
	// Evaluator answers a Query over a Base.
	Evaluator = datalog.Evaluator
	// Naive is the default backtracking-join evaluator.
	Naive = datalog.Naive
)

// V builds a variable term (?name in the text syntax).
func V(name string) Term { return datalog.V(name) }

// Str builds a string-constant term.
func Str(s string) Term { return datalog.Str(s) }

// Num builds a numeric-constant term.
func Num(f float64) Term { return datalog.Num(f) }

// Rel builds an atom Relation(args...).
func Rel(name string, args ...Term) Atom { return datalog.Rel(name, args...) }

// Pos wraps an atom as a positive body literal.
func Pos(a Atom) Literal { return datalog.Pos(a) }

// Neg wraps an atom as a negated body literal.
func Neg(a Atom) Literal { return datalog.Neg(a) }

// Cmp builds a comparison literal, Op in {<, <=, =, !=, >, >=}.
func Cmp(l Term, op string, r Term) Literal { return datalog.Cmp(l, op, r) }

// Def builds a rule Head :- body. Disjunction is several Defs sharing a Head relation.
func Def(head Atom, body ...Literal) Rule { return datalog.Def(head, body...) }

// Build assembles a Query from its rules, the goal literals to solve, and the projected answer terms.
func Build(rules []Rule, goal []Literal, sel ...Term) Query { return datalog.Build(rules, goal, sel...) }

// Parse parses the query text syntax.
func Parse(s string) (Query, error) { return datalog.Parse(s) }

// MustParse parses a query string and panics on error, for hand-authored rules constructed at package
// init, where a malformed built-in query is a programming error surfaced at startup.
func MustParse(s string) Query { return datalog.MustParse(s) }

// NonInjectiveRules reports rules whose body lets two variables in the same position of the same
// relation bind the same value with nothing separating them. See the engine's documentation.
func NonInjectiveRules(q Query) []string { return datalog.NonInjectiveRules(q) }

// CompileGlob compiles a shell-style glob the way the str.glob predicate does, for a Go caller that
// must match the same pattern the same way.
var CompileGlob = ns.CompileGlob

// CompilePattern compiles a regular expression the way the str.match predicate does.
var CompilePattern = ns.CompilePattern
