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

// GeneratorFirstRules reports the rules that OPEN their body with a value-producing generator whose
// own input argument is unbound, naming each offender by its head relation. Empty means no rule starts
// by enumerating the whole design.
//
// This is the shape that makes a generated rule non-terminating rather than merely slow. The evaluator
// is a naive backtracking join running literals left to right, and `reaches` is the one built-in that
// PRODUCES values instead of filtering: given a bound start net it walks that net's series
// neighborhood, but given an unbound one it walks from EVERY net on the board. In first position
// nothing can have bound it, so the walk is unconditional. A shipped profile rule opened with
// `reaches(?n, ?rn, ?h)` and took `agni check` from 13s to not finishing at all on a real design
// (WS3-114). The fix was to lead with the guard the consuming rule already conjoined, which binds the
// start net before the walk begins.
//
// A generator whose input is a CONSTANT is fine (`reaches("VBUS", ?n, ?h)` is one walk), so only an
// unbound first argument is reported.
//
// SCOPE, stated because it is narrower than it looks. This catches the catastrophic case, not every
// bad join order. `pulled` opened with `component-on-net(?pu, ?n)`, both variables unbound, which is a
// full EDB scan re-entered per survivor: 21s rather than forever, and NOT reported here. A general
// answer is a cost-based join planner (WS3-031), not a lint. Note also what does NOT work as a guard,
// since it is the obvious first idea: requiring the first literal to share a variable with the head
// catches neither offender, because both `reaches(?n, ...)` and `component-on-net(?pu, ?n)` do mention
// the head variable. Mentioning it is not binding it.
func GeneratorFirstRules(q Query) []string {
	var out []string
	for _, r := range q.Rules {
		for _, l := range r.Body.Literals {
			if l.Pos == nil {
				continue // a comparison or negation cannot enumerate, so it is not the opening scan
			}
			bi, ok := builtins[l.Pos.Relation]
			if ok && bi.generator && len(l.Pos.Args) > 0 && l.Pos.Args[0].Const == nil {
				out = append(out, r.Head.Relation)
			}
			break // only the FIRST positive literal opens the scan
		}
	}
	sort.Strings(out)
	return out
}
