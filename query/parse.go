package query

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse reads a text query into a Query. Grammar (EBNF; whitespace is insignificant):
//
//	query       = { rule ";" } goal ;                 (* zero or more rules, then one goal *)
//	rule        = atom ":-" literals ;                (* head :- body; defines a derived relation *)
//	goal        = literals [ "=>" projection ] ;
//	literals    = literal { "," literal } ;
//	literal     = atom | "not" atom | comparison ;    (* "not atom" = stratified negation *)
//	atom        = relation "(" [ term { "," term } ] ")" ;
//	comparison  = term op term ;
//	op          = "<" | "<=" | "=" | "!=" | ">" | ">=" ;
//	projection  = column { "," column } ;
//	column      = variable | aggregate ;
//	aggregate   = aggfunc "(" variable ")" ;          (* grouped by the variable columns *)
//	aggfunc     = "count" | "min" | "max" | "sum" ;
//	term        = variable | string | number ;
//	variable    = "?" ident | "_" ;
//	string      = '"' { char } '"' ;
//	number      = [ "+" | "-" ] digit { digit } [ "." digit { digit } ] ;
//	relation    = ident { "." | "-" | ident } ;       (* net.max_voltage, component-on-net *)
//
// Clauses are separated by ";"; a clause with ":-" is a rule, and the one clause without one is the
// goal. A rule head defines a derived (IDB) relation the goal (or another rule) can then read, and a
// rule whose body reads its own head is recursion (evaluated to a stratified fixpoint). Some relation
// names are built-in predicates rather than fact-base relations: reaches(from, net) (transitive
// connectivity) and the string filters contains/prefix/suffix(?value, "pattern"). They parse as
// ordinary atoms; the evaluator dispatches them. A rule head may not redefine a built-in or an EDB
// relation.
//
// This covers the whole bounded fragment the evaluator serves: user-defined (recursive, stratified)
// rules, conjunction, comparison, the built-in reaches and string predicates, stratified negation,
// and aggregation.
// It parses to the query.Query IR; for
//
//	component.mpn(?r,"REG-24"), net.max_voltage(?n,?v), ?v < 30 => ?r, ?n
//
// the result is Query{Goal.Literals: [Atom component.mpn(?r,"REG-24"), Atom net.max_voltage(?n,?v),
// Compare(?v < 30)], Select: [?r, ?n]}. And `component-on-net(?r,?n) => ?n, count(?r)` parses to a
// Goal with one Atom and Select [Term{Var:"n"}, Term{Agg: &Aggregate{Func:"count", Var:"r"}}].
func Parse(s string) (Query, error) {
	rules, goalText, err := splitClauses(s)
	if err != nil {
		return Query{}, err
	}
	body, proj, err := splitProjection(goalText)
	if err != nil {
		return Query{}, err
	}
	lits, err := parseLiterals(body)
	if err != nil {
		return Query{}, err
	}
	sel, err := parseSelect(proj)
	if err != nil {
		return Query{}, err
	}
	return Query{Rules: rules, Goal: Body{Literals: lits}, Select: sel}, nil
}

// splitClauses separates a query into its rule definitions and its single goal clause. Clauses are
// separated by ";"; a clause containing ":-" is a rule (head :- body), and the one remaining clause
// is the goal. A query with no ";" and no ":-" is just a goal, so the common case is unchanged.
func splitClauses(s string) (rules []Rule, goal string, err error) {
	var goals []string
	for _, clause := range splitTop(s, ";") {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		parts := splitTop(clause, ":-")
		switch len(parts) {
		case 1:
			goals = append(goals, clause)
		case 2:
			rule, rerr := parseRule(parts[0], parts[1])
			if rerr != nil {
				return nil, "", rerr
			}
			rules = append(rules, rule)
		default:
			return nil, "", fmt.Errorf("query: clause %q has more than one %q", clause, ":-")
		}
	}
	switch len(goals) {
	case 1:
		return rules, goals[0], nil
	case 0:
		return nil, "", fmt.Errorf("query: no goal clause (a query needs one clause without %q to ask)", ":-")
	default:
		return nil, "", fmt.Errorf("query: %d goal clauses; a query asks one goal (rules use %q, the goal does not)", len(goals), ":-")
	}
}

// parseRule parses one "head :- body" clause into a Rule. The head is a single atom; the body is a
// conjunction of literals, the same grammar a goal body uses.
func parseRule(headText, bodyText string) (Rule, error) {
	head, err := parseAtom(headText)
	if err != nil {
		return Rule{}, err
	}
	lits, err := parseLiterals(bodyText)
	if err != nil {
		return Rule{}, err
	}
	return Rule{Head: head, Body: Body{Literals: lits}}, nil
}

// splitProjection splits a query on the top-level "=>" into body and projection (the projection
// empty when absent). "=>" is unambiguous — no comparison operator is "=>".
func splitProjection(s string) (body, proj string, err error) {
	parts := splitTop(s, "=>")
	switch len(parts) {
	case 1:
		return parts[0], "", nil
	case 2:
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("query: more than one %q projection separator", "=>")
	}
}

func parseLiterals(body string) ([]Literal, error) {
	var lits []Literal
	for _, piece := range splitTop(body, ",") {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
		lit, err := parseLiteral(piece)
		if err != nil {
			return nil, err
		}
		lits = append(lits, lit)
	}
	if len(lits) == 0 {
		return nil, fmt.Errorf("query: empty query")
	}
	return lits, nil
}

// parseLiteral parses one literal: a "not R(...)" is a negated atom, another parenthesised piece is
// a positive atom, anything else a comparison (only atoms carry parentheses).
func parseLiteral(s string) (Literal, error) {
	if rest, isNeg := strings.CutPrefix(strings.TrimSpace(s), "not "); isNeg {
		atom, err := parseAtom(strings.TrimSpace(rest))
		if err != nil {
			return Literal{}, err
		}
		return Literal{Neg: &atom}, nil
	}
	if strings.Contains(s, "(") {
		atom, err := parseAtom(s)
		if err != nil {
			return Literal{}, err
		}
		return Literal{Pos: &atom}, nil
	}
	cmp, err := parseComparison(s)
	if err != nil {
		return Literal{}, err
	}
	return Literal{Compare: &cmp}, nil
}

func parseAtom(s string) (Atom, error) {
	open := strings.IndexByte(s, '(')
	if open < 0 || !strings.HasSuffix(strings.TrimSpace(s), ")") {
		return Atom{}, fmt.Errorf("query: malformed atom %q (want reln(args))", s)
	}
	rel := strings.TrimSpace(s[:open])
	if !isRelation(rel) {
		return Atom{}, fmt.Errorf("query: bad relation name %q", rel)
	}
	inner := strings.TrimSpace(s)
	inner = inner[open+1 : len(inner)-1]
	var args []Term
	for _, a := range splitTop(inner, ",") {
		if strings.TrimSpace(a) == "" {
			continue
		}
		t, err := parseTerm(a)
		if err != nil {
			return Atom{}, err
		}
		args = append(args, t)
	}
	return Atom{Relation: rel, Args: args}, nil
}

var compareOps = []string{"<=", ">=", "!=", "<", ">", "="} // longest first

func parseComparison(s string) (Compare, error) {
	for _, op := range compareOps {
		if parts := splitTop(s, op); len(parts) == 2 {
			l, err := parseTerm(parts[0])
			if err != nil {
				return Compare{}, err
			}
			r, err := parseTerm(parts[1])
			if err != nil {
				return Compare{}, err
			}
			return Compare{Left: l, Op: op, Right: r}, nil
		}
	}
	return Compare{}, fmt.Errorf("query: %q is neither an atom nor a comparison", strings.TrimSpace(s))
}

func parseTerm(s string) (Term, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return Term{}, fmt.Errorf("query: empty term")
	case s == "_":
		return Term{Var: "_"}, nil
	case s[0] == '?':
		name := s[1:]
		if name == "" {
			return Term{}, fmt.Errorf("query: empty variable name")
		}
		return Term{Var: Var(name)}, nil
	case s[0] == '"':
		if len(s) < 2 || s[len(s)-1] != '"' {
			return Term{}, fmt.Errorf("query: unterminated string %q", s)
		}
		return Term{Const: &Value{S: s[1 : len(s)-1]}}, nil
	default:
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return Term{Const: &Value{S: s, Num: &f}}, nil
		}
		return Term{}, fmt.Errorf("query: bare identifier %q — a term must be a ?variable, a \"string\", or a number", s)
	}
}

// parseSelect parses the projection columns: each is a ?variable (a group key) or an aggregate
// func(?variable), func in {count,min,max,sum}.
func parseSelect(proj string) ([]Term, error) {
	proj = strings.TrimSpace(proj)
	if proj == "" {
		return nil, nil
	}
	var sel []Term
	for _, p := range splitTop(proj, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		t, err := parseSelItem(p)
		if err != nil {
			return nil, err
		}
		sel = append(sel, t)
	}
	return sel, nil
}

func parseSelItem(p string) (Term, error) {
	if i := strings.IndexByte(p, '('); i >= 0 { // aggregate: func(?x)
		fn := strings.TrimSpace(p[:i])
		if !strings.HasSuffix(p, ")") {
			return Term{}, fmt.Errorf("query: malformed aggregate %q", p)
		}
		inner := strings.TrimSpace(p[i+1 : len(p)-1])
		if len(inner) < 2 || inner[0] != '?' {
			return Term{}, fmt.Errorf("query: aggregate %s(...) expects a ?variable, got %q", fn, inner)
		}
		return Term{Agg: &Aggregate{Func: fn, Var: Var(inner[1:])}}, nil
	}
	if p[0] != '?' || len(p) == 1 {
		return Term{}, fmt.Errorf("query: projection column %q must be a ?variable or an aggregate", p)
	}
	return Term{Var: Var(p[1:])}, nil
}

// isRelation reports whether name is a valid relation identifier (letters, digits, '.', '-', '_').
func isRelation(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// splitTop splits s on sep at the top level: not inside parentheses and not inside a double-quoted
// string. Returns the whole string as one element when sep does not occur at the top level.
func splitTop(s, sep string) []string {
	var parts []string
	depth, inQuote, start := 0, false, 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case inQuote:
			// skip
		case c == '(':
			depth++
		case c == ')':
			depth--
		case depth == 0 && strings.HasPrefix(s[i:], sep):
			parts = append(parts, s[start:i])
			i += len(sep) - 1
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
