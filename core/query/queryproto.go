package query

import (
	"fmt"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// This file is the datalog half of the rule-definition contract (WS3-103). A Query is already an AST
// (Parse produces it, the typed builder produces it, RuleFromQuery consumes it), so the wire form is a
// mapping, not a second grammar. Crucially it is the AST that travels, not the query TEXT: text would
// make every consumer carry a parser, and it would let a document round-trip through a spelling this
// build cannot parse while looking intact.
//
// Encoding cannot fail. Decoding validates what is statically checkable — that every relation an atom
// names is either a catalog relation, a built-in, or defined by the program's own rules — because a
// program naming a relation that does not exist yields nothing, and yielding nothing is exactly what a
// clean design looks like.

// QueryProto encodes a datalog program as its wire form.
func QueryProto(q Query) *checkspb.DatalogQuery {
	out := &checkspb.DatalogQuery{Goal: bodyProto(q.Goal)}
	for _, r := range q.Rules {
		out.Rules = append(out.Rules, &checkspb.DatalogRule{
			Head: atomProto(r.Head),
			Body: bodyProto(r.Body),
			Hops: int32(r.Hops),
		})
	}
	for _, t := range q.Select {
		out.Select = append(out.Select, qtermProto(t))
	}
	return out
}

// QueryFromProto decodes a datalog program and rejects one that names a relation this build does not
// know, suggesting the closest catalog name when the miss looks like a typo. The check is static (no
// design is loaded), so it catches an author's mistake at read time rather than presenting an empty
// answer as a clean result.
func QueryFromProto(p *checkspb.DatalogQuery) (Query, error) {
	q := Query{}
	for _, pr := range p.GetRules() {
		head, err := atomFromProto(pr.GetHead())
		if err != nil {
			return Query{}, err
		}
		body, err := bodyFromProto(pr.GetBody())
		if err != nil {
			return Query{}, err
		}
		q.Rules = append(q.Rules, Rule{Head: head, Body: body, Hops: int(pr.GetHops())})
	}
	goal, err := bodyFromProto(p.GetGoal())
	if err != nil {
		return Query{}, err
	}
	q.Goal = goal
	for _, t := range p.GetSelect() {
		term, err := qtermFromProto(t)
		if err != nil {
			return Query{}, err
		}
		q.Select = append(q.Select, term)
	}
	if err := ValidateRelations(q); err != nil {
		return Query{}, err
	}
	return q, nil
}

// ValidateRelations reports the first relation the program names that resolves to nothing: not a
// catalog relation (a fact relation or a computed built-in) and not defined by one of the program's
// own rules. It is the static half of the checks the evaluator makes, hoisted so a definition read
// from a document fails when it is READ rather than running to a silent empty answer.
//
// It cannot see relations a not-yet-imported source registers, so it is applied where a program
// arrives from outside (decoding a definition), not to programs this build constructs.
func ValidateRelations(q Query) error {
	known := map[string]bool{}
	for _, r := range Catalog() {
		known[r.Name] = true
	}
	for _, r := range q.Rules {
		known[r.Head.Relation] = true
	}
	check := func(b Body, where string) error {
		for _, lit := range b.Literals {
			var rel string
			switch {
			case lit.Pos != nil:
				rel = lit.Pos.Relation
			case lit.Neg != nil:
				rel = lit.Neg.Relation
			default:
				continue
			}
			if !known[rel] {
				return fmt.Errorf("query: %s names unknown relation %q%s", where, rel, didYouMean(rel))
			}
		}
		return nil
	}
	for _, r := range q.Rules {
		if err := check(r.Body, fmt.Sprintf("rule %q", r.Head.Relation)); err != nil {
			return err
		}
	}
	return check(q.Goal, "goal")
}

func bodyProto(b Body) *checkspb.DatalogBody {
	out := &checkspb.DatalogBody{}
	for _, lit := range b.Literals {
		switch {
		case lit.Pos != nil:
			out.Literals = append(out.Literals, &checkspb.DatalogLiteral{
				Literal: &checkspb.DatalogLiteral_Pos{Pos: atomProto(*lit.Pos)},
			})
		case lit.Neg != nil:
			out.Literals = append(out.Literals, &checkspb.DatalogLiteral{
				Literal: &checkspb.DatalogLiteral_Neg{Neg: atomProto(*lit.Neg)},
			})
		case lit.Compare != nil:
			out.Literals = append(out.Literals, &checkspb.DatalogLiteral{
				Literal: &checkspb.DatalogLiteral_Compare{Compare: &checkspb.DatalogCompare{
					Left:  qtermProto(lit.Compare.Left),
					Op:    lit.Compare.Op,
					Right: qtermProto(lit.Compare.Right),
				}},
			})
		}
	}
	return out
}

func bodyFromProto(p *checkspb.DatalogBody) (Body, error) {
	b := Body{}
	for _, pl := range p.GetLiterals() {
		switch l := pl.GetLiteral().(type) {
		case *checkspb.DatalogLiteral_Pos:
			a, err := atomFromProto(l.Pos)
			if err != nil {
				return Body{}, err
			}
			b.Literals = append(b.Literals, Literal{Pos: &a})
		case *checkspb.DatalogLiteral_Neg:
			a, err := atomFromProto(l.Neg)
			if err != nil {
				return Body{}, err
			}
			b.Literals = append(b.Literals, Literal{Neg: &a})
		case *checkspb.DatalogLiteral_Compare:
			left, err := qtermFromProto(l.Compare.GetLeft())
			if err != nil {
				return Body{}, err
			}
			right, err := qtermFromProto(l.Compare.GetRight())
			if err != nil {
				return Body{}, err
			}
			b.Literals = append(b.Literals, Literal{Compare: &Compare{Left: left, Op: l.Compare.GetOp(), Right: right}})
		default:
			return Body{}, fmt.Errorf("query: literal is empty (no variant set)")
		}
	}
	return b, nil
}

func atomProto(a Atom) *checkspb.DatalogAtom {
	out := &checkspb.DatalogAtom{Relation: a.Relation}
	for _, t := range a.Args {
		out.Args = append(out.Args, qtermProto(t))
	}
	return out
}

func atomFromProto(p *checkspb.DatalogAtom) (Atom, error) {
	a := Atom{Relation: p.GetRelation()}
	for _, pt := range p.GetArgs() {
		t, err := qtermFromProto(pt)
		if err != nil {
			return Atom{}, err
		}
		a.Args = append(a.Args, t)
	}
	return a, nil
}

func qtermProto(t Term) *checkspb.DatalogTerm {
	switch {
	case t.Agg != nil:
		return &checkspb.DatalogTerm{Term: &checkspb.DatalogTerm_Agg{Agg: &checkspb.DatalogAggregate{
			Func: t.Agg.Func, Var: string(t.Agg.Var),
		}}}
	case t.Const != nil:
		v := &checkspb.DatalogValue{S: t.Const.S, Absent: t.Const.Absent, BaseUnit: t.Const.BaseUnit}
		if t.Const.Num != nil {
			v.Num = t.Const.Num
		}
		return &checkspb.DatalogTerm{Term: &checkspb.DatalogTerm_Constant{Constant: v}}
	default:
		return &checkspb.DatalogTerm{Term: &checkspb.DatalogTerm_Var{Var: string(t.Var)}}
	}
}

func qtermFromProto(p *checkspb.DatalogTerm) (Term, error) {
	switch t := p.GetTerm().(type) {
	case *checkspb.DatalogTerm_Var:
		return Term{Var: Var(t.Var)}, nil
	case *checkspb.DatalogTerm_Constant:
		v := &Value{S: t.Constant.GetS(), Absent: t.Constant.GetAbsent(), BaseUnit: t.Constant.GetBaseUnit()}
		if t.Constant.Num != nil {
			n := t.Constant.GetNum()
			v.Num = &n
		}
		return Term{Const: v}, nil
	case *checkspb.DatalogTerm_Agg:
		return Term{Agg: &Aggregate{Func: t.Agg.GetFunc(), Var: Var(t.Agg.GetVar())}}, nil
	}
	return Term{}, fmt.Errorf("query: term is empty (no variant set)")
}

// FindingQueryProto encodes the row-to-finding mapping that turns a datalog program into a rule.
func FindingQueryProto(fq FindingQuery) *checkspb.QueryRule {
	return &checkspb.QueryRule{
		Query:       QueryProto(fq.Query),
		Kind:        fq.Kind,
		SubjectVar:  fq.SubjectVar,
		PinVar:      fq.PinVar,
		Message:     fq.Message,
		ParamSymbol: fq.ParamSymbol,
	}
}
