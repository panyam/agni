package query

import (
	"errors"
	"fmt"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// This file is the single positive primitive of the evaluator: extendAtom. Every callable relation —
// a fact relation (EDB), a rule-derived relation (IDB), or a computed built-in (reaches, the string
// filters, and any overlay-registered predicate) — is "given a partial binding, yield zero-or-more
// extended bindings." solve() drives the positive body through extendAtom; negation reuses the SAME
// primitive (atomHolds runs extendAtom and asks only whether it yields anything — negation as
// failure). That unification is why there is one dispatch, not one per kind, and why `not R(...)`
// works uniformly for EDB, IDB, filters, and reaches.

// reachHops bounds the reaches built-in's series walk. Generous — reaches is bounded by fan-out and
// finiteness anyway (check.Model.Reach); this only caps pathological depth.
const reachHops = 100

// relReaches is the built-in transitive relation reaches(from, to): nets reachable from `from`
// through series pass elements, bridged to check.Model.Reach (WS3-004's recursion, made real). It is
// a GENERATOR (it binds `to` by enumerating from the Model), the one built-in that is not a pure
// filter — which is why a public generator seam is deliberately withheld (a value-producing generator
// could break the finiteness guarantee), while reaches stays an internal builtin.
const relReaches = "reaches"

// builtin is a computed relation: a fixed arity plus extend, the positive primitive for that name.
// Filters (contains/prefix/suffix and overlay predicates) are built from a boolean via filterBuiltin;
// reaches is a generator that consults the Base's Model. Overlay predicates register into this same
// map through RegisterPredicate, so a registered filter is dispatched exactly like a built-in one.
type builtin struct {
	arity  int
	extend func(atom *Atom, bnd *binding, b *Base, yield func(*binding) error) error
}

var builtins = map[string]builtin{
	relReaches: {arity: 2, extend: extendReaches},
	"contains": strFilter(strings.Contains),
	"prefix":   strFilter(strings.HasPrefix),
	"suffix":   strFilter(strings.HasSuffix),
}

// strFilter wraps a string(value, pattern) bool as a 2-arity filter builtin (the shape of
// contains/prefix/suffix). Plain strings only (no regex), so the package stays WASM-clean.
func strFilter(fn func(s, pat string) bool) builtin {
	return filterBuiltin(2, func(args []Value) (bool, error) { return fn(args[0].S, args[1].S), nil })
}

// filterBuiltin builds a pure filter from a boolean over its (all-bound) argument values: it keeps
// the binding when holds is true and yields nothing otherwise. Because negation runs the same extend
// and asks whether it yielded, `not name(...)` keeps a binding exactly when holds is false — the two
// directions derive from the one function, so they cannot disagree. A filter cannot enumerate, so
// every argument must already be bound (a variable must appear in a relation before a filter tests
// it); an unbound argument is an error, the same shape as an unbound comparison operand.
func filterBuiltin(arity int, holds func(args []Value) (bool, error)) builtin {
	return builtin{arity: arity, extend: func(atom *Atom, bnd *binding, b *Base, yield func(*binding) error) error {
		args := make([]Value, len(atom.Args))
		for i, a := range atom.Args {
			v, ok := resolve(a, bnd)
			if !ok {
				return fmt.Errorf("query: %s needs all arguments bound (a variable must appear in a relation before %s tests it)", atom.Relation, atom.Relation)
			}
			args[i] = v
		}
		ok, err := holds(args)
		if err != nil {
			return err
		}
		if ok {
			return yield(bnd)
		}
		return nil
	}}
}

// extendReaches binds reaches(from, to). `from` is a bound/const net when possible, else every net is
// a candidate start; `to` binds to each net reachable from it (reflexive, so from==to holds).
func extendReaches(atom *Atom, bnd *binding, b *Base, yield func(*binding) error) error {
	if b.model == nil {
		return nil // spec library mode (NewSpecLibBase): no design topology, so reaches yields nothing
	}
	from, to := atom.Args[0], atom.Args[1]
	starts := b.netByName
	if fv, ok := resolve(from, bnd); ok { // from is bound/const: one start
		starts = map[string]*ir.Net{fv.S: b.netByName[fv.S]}
	}
	for name, start := range starts {
		if start == nil {
			continue
		}
		cite := "reaches from " + name
		for _, dst := range b.model.Reach(start, reachHops).Nets {
			next := bnd.clone()
			if !bindArg(next, from, Value{S: name}) || !bindArg(next, to, Value{S: dst.Name}) {
				continue
			}
			next.cites = append(next.cites, cite)
			if err := yield(next); err != nil {
				return err
			}
		}
	}
	return nil
}

// extendAtom is the single positive primitive: it yields every binding that satisfies atom as an
// extension of bnd. Dispatch is by relation kind — a computed built-in, an EDB fact relation, or an
// IDB rule relation — each checked for arity first so a wrong-arity atom fails clearly. yield is
// called per solution and its error (from a deeper solve, or the negation early-stop) propagates.
func (b *Base) extendAtom(atom *Atom, bnd *binding, yield func(*binding) error) error {
	rel := atom.Relation
	if bi, ok := builtins[rel]; ok {
		if len(atom.Args) != bi.arity {
			return fmt.Errorf("query: %s takes %d args, got %d", rel, bi.arity, len(atom.Args))
		}
		return bi.extend(atom, bnd, b, yield)
	}
	if fields, ok := edbSchemaOf(rel); ok {
		if len(atom.Args) != len(fields) {
			return fmt.Errorf("query: relation %q takes %d args, got %d", rel, len(fields), len(atom.Args))
		}
		return b.extendEDB(atom, fields, bnd, yield)
	}
	if b.isIDB(rel) {
		if len(atom.Args) != b.idbArity[rel] {
			return fmt.Errorf("query: relation %q takes %d args, got %d", rel, b.idbArity[rel], len(atom.Args))
		}
		return b.extendIDB(atom, bnd, yield)
	}
	return fmt.Errorf("query: unknown relation %q%s", rel, didYouMean(rel))
}

// extendEDB fans an EDB atom over the facts of its relation, unifying each into the binding.
func (b *Base) extendEDB(atom *Atom, fields []edbField, bnd *binding, yield func(*binding) error) error {
	for _, f := range b.edb[atom.Relation] {
		if next, ok := unify(atom.Args, fields, f, bnd); ok {
			if err := yield(next); err != nil {
				return err
			}
		}
	}
	return nil
}

// extendIDB fans a rule-defined atom over the derived tuples of its relation, carrying each tuple's
// provenance forward — the same shape as extendEDB, over the materialized IDB store.
func (b *Base) extendIDB(atom *Atom, bnd *binding, yield func(*binding) error) error {
	for _, t := range b.idb[atom.Relation] {
		out := bnd.clone()
		ok := true
		for j, arg := range atom.Args {
			if !bindArg(out, arg, t.vals[j]) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		out.cites = append(out.cites, t.cites...)
		if err := yield(out); err != nil {
			return err
		}
	}
	return nil
}

// errStop unwinds extendAtom after the first yield — negation only needs existence, not enumeration.
var errStop = errors.New("query: stop")

// atomHolds reports whether atom has any solution under bnd (negation as failure): `not atom` holds
// exactly when this is false. It runs the same extendAtom the positive solve uses, stopping at the
// first match. A malformed atom (an unbound filter argument, wrong arity) surfaces as an error rather
// than silently reading as "no match".
func (b *Base) atomHolds(atom *Atom, bnd *binding) (bool, error) {
	found := false
	err := b.extendAtom(atom, bnd, func(*binding) error { found = true; return errStop })
	if err != nil && err != errStop {
		return false, err
	}
	return found, nil
}

// arityOf reports the declared arity of any callable relation — built-in, EDB, or IDB — and whether
// it is one. Used to validate negated atoms up front.
func (b *Base) arityOf(rel string) (int, bool) {
	if bi, ok := builtins[rel]; ok {
		return bi.arity, true
	}
	if fields, ok := edbSchemaOf(rel); ok {
		return len(fields), true
	}
	if ar, ok := b.idbArity[rel]; ok {
		return ar, true
	}
	return 0, false
}
