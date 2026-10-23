package service

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// witnessPresenter puts an evaluation's witnesses on the wire (agni issue 794). It shares the
// response's entity and source tables with the rows, so a net a witness names is the entity a cell
// naming it points to, and a citation is kept once whether a row or a witness cites it.
type witnessPresenter struct {
	reg        *facts.Registry
	rules      []query.Rule // the query's own rules, which type a witness of a relation the query defines
	entity     func(kind, ref, pin string) int32
	sources    *SourceTable
	designPath string
	typed      map[string]witnessTyping
}

// witnessTyping is what each argument position of one relation denotes, read as a cell's column is.
type witnessTyping struct {
	kinds    []string
	kindVars []int // the position whose value is this position's kind, -1 for none
	refs     []query.Term
	at       map[query.Var]int
}

func newWitnessPresenter(reg *facts.Registry, q query.Query, entity func(kind, ref, pin string) int32, sources *SourceTable, designPath string) *witnessPresenter {
	return &witnessPresenter{reg: reg, rules: q.Rules, entity: entity, sources: sources, designPath: designPath, typed: map[string]witnessTyping{}}
}

// nodes are one row's witnesses as the wire carries them.
func (p *witnessPresenter) nodes(ws []*query.Witness) []*webapi.QueryWitness {
	if len(ws) == 0 {
		return nil
	}
	out := make([]*webapi.QueryWitness, 0, len(ws))
	for _, w := range ws {
		out = append(out, p.node(w))
	}
	return out
}

func (p *witnessPresenter) node(w *query.Witness) *webapi.QueryWitness {
	n := &webapi.QueryWitness{Relation: w.Relation, Rule: w.Rule, Negated: w.Negated, Values: make([]string, len(w.Values))}
	for i, v := range w.Values {
		n.Values[i] = query.Display(v)
	}
	t := p.typing(w.Relation, len(w.Values))
	for i := range n.Values {
		kind := t.kinds[i]
		if j := t.kindVars[i]; j >= 0 {
			kind = entityKind(n.Values[j])
		}
		ref, pin := n.Values[i], ""
		if kind == check.KindPin {
			ref, pin = t.ref(i, n.Values), n.Values[i]
		}
		// A value the negation left unbound, or a pin whose component did not resolve, names nothing.
		if kind == "" || ref == "" || n.Values[i] == "" {
			continue
		}
		if n.ValueEntity == nil {
			n.ValueEntity = make([]int32, len(n.Values))
		}
		n.ValueEntity[i] = p.entity(kind, ref, pin)
	}
	n.CiteIndex = p.sources.Indexes(portableCites(w.Cites, p.designPath))
	n.Children = p.nodes(w.Children)
	return n
}

// typing types a relation's positions the way columnKindsIn types a query's columns: it asks the
// engine to type a goal that calls the relation with a fresh variable per position, under the query's
// own rules, so a library member, a shipped relation and a relation the query defines all type from
// their declarations. A relation the engine cannot type, such as a walk, answers with scalars.
func (p *witnessPresenter) typing(rel string, arity int) witnessTyping {
	key := fmt.Sprintf("%s/%d", rel, arity)
	if t, ok := p.typed[key]; ok {
		return t
	}
	t := witnessTyping{kinds: make([]string, arity), kindVars: make([]int, arity), refs: make([]query.Term, arity), at: map[query.Var]int{}}
	args := make([]query.Term, arity)
	for i := range args {
		v := query.Var(fmt.Sprintf("w%d", i))
		args[i] = query.Term{Var: v}
		t.at[v] = i
		t.kindVars[i] = -1
	}
	goal := query.Query{Rules: p.rules, Goal: query.Body{Literals: []query.Literal{{Pos: &query.Atom{Relation: rel, Args: args}}}}}
	if arity > 0 {
		kinds, kindVars, refs := columnKindsIn(p.reg, goal)
		for i := 0; i < arity && i < len(kinds); i++ {
			t.kinds[i], t.refs[i] = kinds[i], refs[i]
			if j, ok := t.at[kindVars[i]]; ok && kindVars[i] != "" {
				t.kindVars[i] = j
			}
		}
	}
	p.typed[key] = t
	return t
}

// ref is the component a pin at position i belongs to, read from the position or constant its
// relation names as the pin's owner.
func (t witnessTyping) ref(i int, values []string) string {
	o := t.refs[i]
	if o.Const != nil {
		return o.Const.S
	}
	if j, ok := t.at[o.Var]; ok && o.Var != "" {
		return values[j]
	}
	return ""
}
