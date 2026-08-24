package check

import (
	"fmt"
	"sort"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// This file is the Spec half of the rule-definition contract (WS3-103): a Spec is already a value, so
// giving it a wire form is a mapping rather than a redesign. It lives beside the AST it converts so
// that adding a node to the closed vocabulary and forgetting its wire form is a compile error here,
// not a silent drop at the far end of a package boundary.
//
// The conversion is deliberately total in one direction and validating in the other. Encoding a Spec
// this package built cannot fail: the AST is closed and every node has a case. Decoding CAN fail,
// because the bytes came from somewhere else — so SpecFromProto builds the value and then runs the
// same Validate that binding a hand-written Spec runs, which is what catches an unknown fact, an
// unbound var, an unregistered FFI name, or a pattern that will not compile.

// SpecProto encodes a Spec as its wire form. It panics on a Term or Expr outside the closed
// vocabulary, which can only happen if a node type was added without a case here — a programming
// error, caught the first time anything encodes it, rather than a silently truncated definition.
func SpecProto(s Spec) *checkspb.SpecBody {
	out := &checkspb.SpecBody{Over: s.Over, Message: s.Message}
	if len(s.Let) > 0 {
		out.Let = make(map[string]*checkspb.SpecTerm, len(s.Let))
		for k, t := range s.Let {
			out.Let[k] = termProto(t)
		}
	}
	if s.Where != nil {
		out.Where = exprProto(s.Where)
	}
	if s.Scope != nil {
		out.Scope = exprProto(s.Scope)
	}
	return out
}

// SpecFromProto decodes a Spec and validates it against this build's vocabulary. The error names what
// it did not recognize, because a definition that cannot compile has to say so when it is read: a
// rule that silently fails to exist is indistinguishable from one that ran and found nothing.
func SpecFromProto(p *checkspb.SpecBody) (Spec, error) {
	s := Spec{Over: p.GetOver(), Message: p.GetMessage()}
	if len(p.GetLet()) > 0 {
		s.Let = make(map[string]Term, len(p.GetLet()))
		for k, t := range p.GetLet() {
			term, err := termFromProto(t)
			if err != nil {
				return Spec{}, fmt.Errorf("let %q: %w", k, err)
			}
			s.Let[k] = term
		}
	}
	if p.GetWhere() != nil {
		w, err := exprFromProto(p.GetWhere())
		if err != nil {
			return Spec{}, err
		}
		s.Where = w
	}
	if p.GetScope() != nil {
		sc, err := exprFromProto(p.GetScope())
		if err != nil {
			return Spec{}, err
		}
		s.Scope = sc
	}
	if err := s.Validate(); err != nil {
		return Spec{}, err
	}
	return s, nil
}

func termProto(t Term) *checkspb.SpecTerm {
	switch x := t.(type) {
	case Lit:
		return &checkspb.SpecTerm{Term: &checkspb.SpecTerm_Lit{Lit: litProto(x)}}
	case Fact:
		return &checkspb.SpecTerm{Term: &checkspb.SpecTerm_Fact{Fact: x.Name}}
	case Var:
		return &checkspb.SpecTerm{Term: &checkspb.SpecTerm_Var{Var: x.Name}}
	case Call:
		c := &checkspb.SpecCall{Fn: x.Fn}
		for _, a := range x.Args {
			c.Args = append(c.Args, termProto(a))
		}
		return &checkspb.SpecTerm{Term: &checkspb.SpecTerm_Call{Call: c}}
	case CountOf:
		co := &checkspb.SpecCountOf{Over: x.Over}
		if x.Where != nil {
			co.Where = exprProto(x.Where)
		}
		return &checkspb.SpecTerm{Term: &checkspb.SpecTerm_CountOf{CountOf: co}}
	}
	panic(fmt.Sprintf("check: no wire form for spec term %T", t))
}

func litProto(l Lit) *checkspb.SpecLit {
	switch v := l.V.(type) {
	case string:
		return &checkspb.SpecLit{Value: &checkspb.SpecLit_S{S: v}}
	case int:
		return &checkspb.SpecLit{Value: &checkspb.SpecLit_I{I: int64(v)}}
	case int64:
		return &checkspb.SpecLit{Value: &checkspb.SpecLit_I{I: v}}
	case bool:
		return &checkspb.SpecLit{Value: &checkspb.SpecLit_B{B: v}}
	}
	panic(fmt.Sprintf("check: no wire form for spec literal %T", l.V))
}

func termFromProto(p *checkspb.SpecTerm) (Term, error) {
	switch t := p.GetTerm().(type) {
	case *checkspb.SpecTerm_Lit:
		return litFromProto(t.Lit)
	case *checkspb.SpecTerm_Fact:
		return Fact{Name: t.Fact}, nil
	case *checkspb.SpecTerm_Var:
		return Var{Name: t.Var}, nil
	case *checkspb.SpecTerm_Call:
		c := Call{Fn: t.Call.GetFn()}
		for _, a := range t.Call.GetArgs() {
			arg, err := termFromProto(a)
			if err != nil {
				return nil, err
			}
			c.Args = append(c.Args, arg)
		}
		return c, nil
	case *checkspb.SpecTerm_CountOf:
		co := CountOf{Over: t.CountOf.GetOver()}
		if t.CountOf.GetWhere() != nil {
			w, err := exprFromProto(t.CountOf.GetWhere())
			if err != nil {
				return nil, err
			}
			co.Where = w
		}
		return co, nil
	}
	return nil, fmt.Errorf("spec term is empty (no variant set)")
}

// litFromProto decodes an integer literal as a Go int, not int64: the facts a Cmp orders against
// (net.pin_count, segment.width) are ints, and the comparison is type-exact, so an int64 here would
// make every ordering silently false rather than wrong-looking.
func litFromProto(p *checkspb.SpecLit) (Term, error) {
	switch v := p.GetValue().(type) {
	case *checkspb.SpecLit_S:
		return Lit{V: v.S}, nil
	case *checkspb.SpecLit_I:
		return Lit{V: int(v.I)}, nil
	case *checkspb.SpecLit_B:
		return Lit{V: v.B}, nil
	}
	return nil, fmt.Errorf("spec literal is empty (no variant set)")
}

func exprProto(e Expr) *checkspb.SpecExpr {
	switch x := e.(type) {
	case And:
		return &checkspb.SpecExpr{Expr: &checkspb.SpecExpr_And{And: exprListProto(x.Xs)}}
	case Or:
		return &checkspb.SpecExpr{Expr: &checkspb.SpecExpr_Or{Or: exprListProto(x.Xs)}}
	case Not:
		return &checkspb.SpecExpr{Expr: &checkspb.SpecExpr_Not{Not: exprProto(x.X)}}
	case Cmp:
		return &checkspb.SpecExpr{Expr: &checkspb.SpecExpr_Cmp{Cmp: &checkspb.SpecCmp{
			L: termProto(x.L), Op: x.Op, R: termProto(x.R),
		}}}
	case In:
		return &checkspb.SpecExpr{Expr: &checkspb.SpecExpr_In{In: &checkspb.SpecIn{
			T: termProto(x.T), Set: x.Set,
		}}}
	case Match:
		return &checkspb.SpecExpr{Expr: &checkspb.SpecExpr_Match{Match: &checkspb.SpecMatch{
			T: termProto(x.T), Pattern: x.Pattern,
		}}}
	case ExistsIn:
		ex := &checkspb.SpecExistsIn{Over: x.Over}
		if x.Where != nil {
			ex.Where = exprProto(x.Where)
		}
		return &checkspb.SpecExpr{Expr: &checkspb.SpecExpr_ExistsIn{ExistsIn: ex}}
	case IsTrue:
		return &checkspb.SpecExpr{Expr: &checkspb.SpecExpr_IsTrue{IsTrue: termProto(x.T)}}
	}
	panic(fmt.Sprintf("check: no wire form for spec expr %T", e))
}

func exprListProto(xs []Expr) *checkspb.SpecExprList {
	out := &checkspb.SpecExprList{}
	for _, x := range xs {
		out.Xs = append(out.Xs, exprProto(x))
	}
	return out
}

func exprFromProto(p *checkspb.SpecExpr) (Expr, error) {
	switch x := p.GetExpr().(type) {
	case *checkspb.SpecExpr_And:
		xs, err := exprListFromProto(x.And)
		return And{Xs: xs}, err
	case *checkspb.SpecExpr_Or:
		xs, err := exprListFromProto(x.Or)
		return Or{Xs: xs}, err
	case *checkspb.SpecExpr_Not:
		inner, err := exprFromProto(x.Not)
		if err != nil {
			return nil, err
		}
		return Not{X: inner}, nil
	case *checkspb.SpecExpr_Cmp:
		l, err := termFromProto(x.Cmp.GetL())
		if err != nil {
			return nil, err
		}
		r, err := termFromProto(x.Cmp.GetR())
		if err != nil {
			return nil, err
		}
		return Cmp{L: l, Op: x.Cmp.GetOp(), R: r}, nil
	case *checkspb.SpecExpr_In:
		t, err := termFromProto(x.In.GetT())
		if err != nil {
			return nil, err
		}
		return In{T: t, Set: x.In.GetSet()}, nil
	case *checkspb.SpecExpr_Match:
		t, err := termFromProto(x.Match.GetT())
		if err != nil {
			return nil, err
		}
		return Match{T: t, Pattern: x.Match.GetPattern()}, nil
	case *checkspb.SpecExpr_ExistsIn:
		ex := ExistsIn{Over: x.ExistsIn.GetOver()}
		if x.ExistsIn.GetWhere() != nil {
			w, err := exprFromProto(x.ExistsIn.GetWhere())
			if err != nil {
				return nil, err
			}
			ex.Where = w
		}
		return ex, nil
	case *checkspb.SpecExpr_IsTrue:
		t, err := termFromProto(x.IsTrue)
		if err != nil {
			return nil, err
		}
		return IsTrue{T: t}, nil
	}
	return nil, fmt.Errorf("spec expr is empty (no variant set)")
}

func exprListFromProto(p *checkspb.SpecExprList) ([]Expr, error) {
	out := make([]Expr, 0, len(p.GetXs()))
	for _, x := range p.GetXs() {
		e, err := exprFromProto(x)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// RuleMetaProto encodes a rule's identity, prose, and declared gates. Reads and Primitives are
// deliberately not carried: both are derived from the rule's body by its compiler, so serializing them
// would let a definition assert reads that disagree with what it actually does.
func RuleMetaProto(r Rule) *checkspb.RuleMeta {
	m := &checkspb.RuleMeta{
		Name:          r.Name,
		Severity:      r.Severity,
		Summary:       r.Summary,
		Impact:        r.Impact,
		Remedy:        r.Remedy,
		Detail:        r.Detail,
		Tags:          r.Tags,
		OptionalReads: r.OptionalReads,
	}
	for _, c := range r.RequiresCapability {
		m.RequiresCapability = append(m.RequiresCapability, string(c))
	}
	return m
}

// RuleMetaFromProto decodes rule metadata into the Rule fields a compiler then completes with Eval
// and the derived Reads/Primitives.
func RuleMetaFromProto(p *checkspb.RuleMeta) Rule {
	r := Rule{
		Name:          p.GetName(),
		Severity:      p.GetSeverity(),
		Summary:       p.GetSummary(),
		Impact:        p.GetImpact(),
		Remedy:        p.GetRemedy(),
		Detail:        p.GetDetail(),
		Tags:          p.GetTags(),
		OptionalReads: p.GetOptionalReads(),
	}
	for _, c := range p.GetRequiresCapability() {
		r.RequiresCapability = append(r.RequiresCapability, Capability(c))
	}
	return r
}

// SpecNames returns the entity sets, collections, facts, and registered functions a definition may
// reference in this build, sorted. It is the spec language's lexicon made readable, for an error
// message, a documentation surface, or an author checking what a definition can name.
func SpecNames() (overs, collections, facts, funcs []string) {
	for k := range specOvers {
		overs = append(overs, k)
	}
	for k := range specColls {
		collections = append(collections, k)
	}
	for k := range specFacts {
		facts = append(facts, k)
	}
	for k := range specFuncs {
		funcs = append(funcs, k)
	}
	sort.Strings(overs)
	sort.Strings(collections)
	sort.Strings(facts)
	sort.Strings(funcs)
	return
}
