package query

import (
	"fmt"

	"github.com/panyam/agni/core/facts"
	"github.com/panyam/jaala/ns"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// This file is the datalog half of the rule-definition contract (WS3-103). The wire form maps the
// Query AST rather than carrying query TEXT, because text would make every consumer carry a parser and
// would let a document round-trip through a spelling this build cannot parse while looking intact.
//
// Encoding cannot fail. Decoding checks that every relation an atom names is a catalog relation, a
// built-in, or defined by the program's own rules, because a program naming a relation that does not
// exist yields nothing, which looks the same as a clean design.

// QueryProto encodes a datalog program as its wire form.
func QueryProto(q Query) *checkspb.DatalogQuery {
	out := &checkspb.DatalogQuery{Goal: bodyProto(q.Goal)}
	for _, r := range q.Rules {
		out.Rules = append(out.Rules, &checkspb.DatalogRule{
			Head:      atomProto(r.Head),
			Body:      bodyProto(r.Body),
			Hops:      int32(r.Hops),
			HeadTypes: argTypeProtos(r.HeadTypes),
		})
	}
	for _, t := range q.Select {
		out.Select = append(out.Select, qtermProto(t))
	}
	for _, h := range q.Having {
		out.Having = append(out.Having, compareProto(h))
	}
	for _, o := range q.OrderBy {
		out.OrderBy = append(out.OrderBy, &checkspb.DatalogOrder{Term: qtermProto(o.Term), Desc: o.Desc})
	}
	for _, v := range q.After {
		out.After = append(out.After, valueProto(v))
	}
	out.Limit, out.Offset = int32(q.Limit), int32(q.Offset)
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
		q.Rules = append(q.Rules, Rule{Head: head, Body: body, Hops: int(pr.GetHops()), HeadTypes: argTypesFromProto(pr.GetHeadTypes())})
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
	for _, h := range p.GetHaving() {
		c, err := compareFromProto(h)
		if err != nil {
			return Query{}, err
		}
		q.Having = append(q.Having, c)
	}
	for _, o := range p.GetOrderBy() {
		t, err := qtermFromProto(o.GetTerm())
		if err != nil {
			return Query{}, err
		}
		q.OrderBy = append(q.OrderBy, Order{Term: t, Desc: o.GetDesc()})
	}
	for _, v := range p.GetAfter() {
		q.After = append(q.After, valueFromProto(v))
	}
	q.Limit, q.Offset = int(p.GetLimit()), int(p.GetOffset())
	if err := ValidateRelations(q); err != nil {
		return Query{}, err
	}
	return q, nil
}

// ValidateRelations reports the first relation the program names that is neither a catalog relation
// (a fact relation or a computed built-in) nor defined by one of the program's own rules. It is the static half of the checks the evaluator makes, hoisted so a definition read
// from a document fails when it is READ rather than running to a silent empty answer.
//
// It cannot see relations a not-yet-imported source registers, so it is applied where a program
// arrives from outside (decoding a definition), not to programs this build constructs.
func ValidateRelations(q Query) error { return ValidateRelationsFrom(facts.DefaultRegistry(), q) }

// ValidateRelationsFrom is ValidateRelations over an explicit relation vocabulary.
func ValidateRelationsFrom(reg *facts.Registry, q Query) error {
	known := map[string]bool{}
	for _, r := range CatalogFrom(reg) {
		known[r.Name] = true
	}
	for _, r := range q.Rules {
		known[r.Head.Relation] = true
	}
	var check func(b Body, where string) error
	check = func(b Body, where string) error {
		for _, lit := range b.Literals {
			var rel string
			switch {
			case lit.Pos != nil:
				rel = lit.Pos.Relation
			case lit.Neg != nil:
				rel = lit.Neg.Relation
			case lit.Agg != nil:
				if err := check(lit.Agg.Body, where); err != nil {
					return err
				}
				continue
			default:
				continue
			}
			if !known[rel] {
				return fmt.Errorf("query: %s names unknown relation %q%s", where, rel, didYouMean(reg, rel))
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
				Literal: &checkspb.DatalogLiteral_Compare{Compare: compareProto(*lit.Compare)},
			})
		case lit.Agg != nil:
			out.Literals = append(out.Literals, &checkspb.DatalogLiteral{
				Literal: &checkspb.DatalogLiteral_Agg{Agg: &checkspb.DatalogBodyAggregate{
					Result: string(lit.Agg.Result), Agg: aggregateProto(lit.Agg.Agg), Body: bodyProto(lit.Agg.Body),
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
			c, err := compareFromProto(l.Compare)
			if err != nil {
				return Body{}, err
			}
			b.Literals = append(b.Literals, Literal{Compare: &c})
		case *checkspb.DatalogLiteral_Agg:
			body, err := bodyFromProto(l.Agg.GetBody())
			if err != nil {
				return Body{}, err
			}
			b.Literals = append(b.Literals, Literal{Agg: &BodyAggregate{
				Result: Var(l.Agg.GetResult()), Agg: aggregateFromProto(l.Agg.GetAgg()), Body: body,
			}})
		default:
			return Body{}, fmt.Errorf("query: literal is empty (no variant set)")
		}
	}
	return b, nil
}

// compareProto and compareFromProto carry a two-term comparison, shared by a body literal and a
// having filter. The two differ only in WHEN they apply, so a having needs no message of its own; an
// aggregate on the left is what tells them apart on the wire.
func compareProto(c Compare) *checkspb.DatalogCompare {
	return &checkspb.DatalogCompare{Left: qtermProto(c.Left), Op: c.Op, Right: qtermProto(c.Right)}
}

func compareFromProto(p *checkspb.DatalogCompare) (Compare, error) {
	left, err := qtermFromProto(p.GetLeft())
	if err != nil {
		return Compare{}, err
	}
	right, err := qtermFromProto(p.GetRight())
	if err != nil {
		return Compare{}, err
	}
	return Compare{Left: left, Op: p.GetOp(), Right: right}, nil
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
		return &checkspb.DatalogTerm{Term: &checkspb.DatalogTerm_Agg{Agg: aggregateProto(*t.Agg)}}
	case t.Const != nil:
		return &checkspb.DatalogTerm{Term: &checkspb.DatalogTerm_Constant{Constant: valueProto(*t.Const)}}
	default:
		return &checkspb.DatalogTerm{Term: &checkspb.DatalogTerm_Var{Var: string(t.Var)}}
	}
}

func qtermFromProto(p *checkspb.DatalogTerm) (Term, error) {
	switch t := p.GetTerm().(type) {
	case *checkspb.DatalogTerm_Var:
		return Term{Var: Var(t.Var)}, nil
	case *checkspb.DatalogTerm_Constant:
		v := valueFromProto(t.Constant)
		return Term{Const: &v}, nil
	case *checkspb.DatalogTerm_Agg:
		a := aggregateFromProto(t.Agg)
		return Term{Agg: &a}, nil
	}
	return Term{}, fmt.Errorf("query: term is empty (no variant set)")
}

func aggregateProto(a Aggregate) *checkspb.DatalogAggregate {
	return &checkspb.DatalogAggregate{Func: a.Func, Var: string(a.Var), Distinct: a.Distinct}
}

func aggregateFromProto(p *checkspb.DatalogAggregate) Aggregate {
	return Aggregate{Func: p.GetFunc(), Var: Var(p.GetVar()), Distinct: p.GetDistinct()}
}

func valueProto(v Value) *checkspb.DatalogValue {
	return &checkspb.DatalogValue{S: v.S, Num: v.Num, Absent: v.Absent, BaseUnit: v.BaseUnit}
}

func valueFromProto(p *checkspb.DatalogValue) Value {
	v := Value{S: p.GetS(), Absent: p.GetAbsent(), BaseUnit: p.GetBaseUnit()}
	if p.Num != nil {
		n := p.GetNum()
		v.Num = &n
	}
	return v
}

func argTypeProtos(ts []ns.ArgType) []*checkspb.DatalogArgType {
	var out []*checkspb.DatalogArgType
	for _, t := range ts {
		out = append(out, &checkspb.DatalogArgType{
			Kind: t.Kind, KindFrom: t.KindFrom, Owner: t.Owner, Type: t.Type, Unit: t.Unit, Domain: t.Domain,
		})
	}
	return out
}

func argTypesFromProto(ps []*checkspb.DatalogArgType) []ns.ArgType {
	var out []ns.ArgType
	for _, at := range ps {
		out = append(out, ns.ArgType{
			Kind: at.GetKind(), KindFrom: at.GetKindFrom(), Owner: at.GetOwner(), Type: at.GetType(), Unit: at.GetUnit(), Domain: at.GetDomain(),
		})
	}
	return out
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
		ContextVars: contextVarProtos(fq.ContextVars),
	}
}

// contextVarProtos encodes a rule's context-variable bindings, preserving the author's order.
func contextVarProtos(cvs []ContextVar) []*checkspb.ContextVar {
	if len(cvs) == 0 {
		return nil
	}
	out := make([]*checkspb.ContextVar, 0, len(cvs))
	for _, cv := range cvs {
		out = append(out, &checkspb.ContextVar{Var: cv.Var, Kind: cv.Kind, Role: cv.Role})
	}
	return out
}

// ContextVarsFromProto decodes a rule's context-variable bindings, preserving order.
func ContextVarsFromProto(ps []*checkspb.ContextVar) []ContextVar {
	if len(ps) == 0 {
		return nil
	}
	out := make([]ContextVar, 0, len(ps))
	for _, p := range ps {
		out = append(out, ContextVar{Var: p.GetVar(), Kind: p.GetKind(), Role: p.GetRole()})
	}
	return out
}
