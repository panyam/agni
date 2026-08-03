package query

import (
	"fmt"
	"sort"
)

// This file is the programmatic constructor for a Query — the typed alternative to Parse (WS3-043).
// A hand-authored rule is clearest as a string (MustParse); a GENERATED rule (the interface-profile
// compiler) is safer as an AST: no separator or quoting to get wrong, and a malformed program is a
// compile error, not a parse panic at init. Both produce the same Query the evaluator runs.

// V builds a variable term (?name in the text syntax).
func V(name string) Term { return Term{Var: Var(name)} }

// Str builds a string-constant term.
func Str(s string) Term { return Term{Const: &Value{S: s}} }

// Num builds a numeric-constant term.
func Num(f float64) Term { return Term{Const: &Value{S: ftoa(f), Num: &f}} }

// Rel builds an atom Relation(args...) — an EDB relation, a built-in, or an IDB relation a Def
// introduces.
func Rel(name string, args ...Term) Atom { return Atom{Relation: name, Args: args} }

// Pos wraps an atom as a positive body literal.
func Pos(a Atom) Literal { return Literal{Pos: &a} }

// Neg wraps an atom as a negated body literal (stratified negation).
func Neg(a Atom) Literal { return Literal{Neg: &a} }

// Cmp builds a comparison literal, Op in {<, <=, =, !=, >, >=}.
func Cmp(l Term, op string, r Term) Literal { return Literal{Compare: &Compare{Left: l, Op: op, Right: r}} }

// Def builds an IDB rule Head :- body. Disjunction is several Defs sharing a Head relation.
func Def(head Atom, body ...Literal) Rule { return Rule{Head: head, Body: Body{Literals: body}} }

// Build assembles a Query from its IDB rules, the goal literals to solve, and the projected answer
// terms (the => columns). Passing no rules is the common single-goal case.
func Build(rules []Rule, goal []Literal, sel ...Term) Query {
	return Query{Rules: rules, Goal: Body{Literals: goal}, Select: sel}
}

// MustParse parses a query string and panics on error — for hand-authored rules constructed at
// package init, where a malformed built-in query is a programming error surfaced at startup (the
// same contract RuleFromQuery had when it parsed internally).
func MustParse(s string) Query {
	q, err := Parse(s)
	if err != nil {
		panic(fmt.Sprintf("query.MustParse(%q): %v", s, err))
	}
	return q
}

// Reads returns the EDB (fact-base) relations a query references, sorted and deduped — the facts a
// query-backed rule declares it reads (drives check.Available's param/board gating). IDB relations a
// Def introduces and built-in generators (reaches) are excluded; only relations in the fact schema
// count as reads.
func Reads(q Query) []string {
	seen := map[string]bool{}
	visit := func(b Body) {
		for _, l := range b.Literals {
			for _, a := range []*Atom{l.Pos, l.Neg} {
				if a != nil {
					if _, ok := edbSchema[a.Relation]; ok {
						seen[a.Relation] = true
					}
				}
			}
		}
	}
	for _, r := range q.Rules {
		visit(r.Body)
	}
	visit(q.Goal)
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}
