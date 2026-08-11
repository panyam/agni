package query

import (
	"errors"
	"fmt"
	"regexp"
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

// topologyReachHops is the radius the reaches built-in searches at: the whole series neighborhood.
// Generous — reaches is bounded by fan-out and finiteness anyway (check.Model.Reach walks only
// two-terminal pass elements and refuses to enter bus-like nets), so this only caps pathological
// depth and costs little more than a small bound would.
//
// It is deliberately much wider than check.ProtectionReachHops, and the difference is SEMANTIC, not
// a performance tradeoff. A topology search asks "what is connected to what through passives", where
// a distant answer is a good answer. A protection guard asks "is a clamp electrically adjacent to
// this pin", where a distant answer is a wrong answer. Narrowing this would silently shrink every
// topology query; widening the protection radius would silently create false passes. A rule that
// needs a specific radius states it with the third argument (WS3-112) rather than relying on either
// constant.
const topologyReachHops = 100

// relReaches is the built-in transitive relation reaches(from, to) and reaches(from, to, hops): nets
// reachable from `from` through series pass elements, bridged to check.Model.Reach (WS3-004's
// recursion, made real). It is a GENERATOR (it binds `to` by enumerating from the Model), the one
// built-in that is not a pure filter — which is why a public generator seam is deliberately withheld
// (a value-producing generator could break the finiteness guarantee), while reaches stays an
// internal builtin.
const relReaches = "reaches"

// builtin is a computed relation: an accepted arity range plus extend, the positive primitive for
// that name. Filters (contains/prefix/suffix and overlay predicates) are built from a boolean via
// filterBuiltin; reaches is a generator that consults the Base's Model. Overlay predicates register
// into this same map through RegisterPredicate, so a registered filter is dispatched exactly like a
// built-in one.
//
// maxArity is 0 for the fixed-arity majority, meaning "exactly arity". Only reaches varies, and the
// range exists so its optional distance argument is admitted by the POSITIVE path and the NEGATION
// path through one predicate (accepts), not two independent length checks that could drift — a
// divergence would accept reaches(?a,?b,?h) but reject not reaches(?a,?b,?h).
type builtin struct {
	arity    int
	maxArity int // 0 = fixed at arity; otherwise the inclusive upper bound
	// generator marks a builtin that PRODUCES values by enumerating from the Model rather than
	// filtering an already-bound tuple. It is the property that makes clause order load-bearing: a
	// generator whose own input argument is unbound enumerates from every candidate in the design,
	// so appearing first in a body is a whole-design scan no later literal can undo. Filters cannot
	// do this — they require every argument bound and so can only ever narrow. See GeneratorFirstRules.
	generator bool
	extend    func(atom *Atom, bnd *binding, b *Base, yield func(*binding) error) error
}

// accepts reports whether n is a valid argument count for this builtin.
func (bi builtin) accepts(n int) bool {
	if bi.maxArity == 0 {
		return n == bi.arity
	}
	return n >= bi.arity && n <= bi.maxArity
}

// arityLabel renders the accepted arity for an error message ("2" or "2 or 3").
func (bi builtin) arityLabel() string {
	if bi.maxArity == 0 {
		return fmt.Sprintf("%d", bi.arity)
	}
	return fmt.Sprintf("%d or %d", bi.arity, bi.maxArity)
}

var builtins = map[string]builtin{
	relReaches: {arity: 2, maxArity: 3, generator: true, extend: extendReaches},
	"contains": strFilter(strings.Contains),
	"prefix":   strFilter(strings.HasPrefix),
	"suffix":   strFilter(strings.HasSuffix),
	"glob":     patFilter(CompileGlob),
	"match":    patFilter(CompilePattern),
	// absent(?x) is the only way to ASK about a field the source did not state. Before Value.Absent
	// existed such a field bound to the empty string, so it was not merely hard to select, it was
	// indistinguishable from one that was stated as "". Its negation is the useful half as often as
	// not: `not absent(?min)` reads "this row states a lower bound".
	"absent": filterBuiltin(1, func(args []Value) (bool, error) { return args[0].Absent, nil }),
}

// strFilter wraps a string(value, pattern) bool as a 2-arity filter builtin (the shape of
// contains/prefix/suffix).
func strFilter(fn func(s, pat string) bool) builtin {
	return filterBuiltin(2, func(args []Value) (bool, error) { return fn(args[0].S, args[1].S), nil })
}

// patFilter is strFilter for the two PATTERN predicates (glob, match): the pattern must be compiled
// before it can be tested, so a malformed one is an EVAL ERROR rather than a non-match. That
// direction matters — a bad pattern that quietly matched nothing would read as "the design is clean"
// on a completeness check, the same silent-pass shape WS3-090 fixed at the profile presence gate.
func patFilter(compile func(string) (*regexp.Regexp, error)) builtin {
	return filterBuiltin(2, func(args []Value) (bool, error) {
		re, err := compile(args[1].S)
		if err != nil {
			return false, err
		}
		return re.MatchString(args[0].S), nil
	})
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

// extendReaches binds reaches(from, to) and reaches(from, to, hops). `from` is a bound/const net when
// possible, else every net is a candidate start; `to` binds to each net reachable from it (reflexive,
// so from==to holds at distance 0).
//
// The optional third argument binds the ACTUAL number of series crossings, so it is an exact value,
// not a budget. A radius question is therefore written with a comparison — reaches(?n,?rn,?h), ?h<=2
// — and NOT as reaches(?n,?rn,2), which binds by equality and so means "exactly two hops away",
// missing anything closer. The catalog entry and the reference doc say this outright, because the
// constant form is the spelling a reader reaches for first and it silently means something else.
//
// One walk serves both arities: the third argument is an extra bindArg over a distance the BFS
// already recorded, never a second traversal with different semantics.
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
		r := b.model.Reach(start, topologyReachHops)
		for _, dst := range r.Nets {
			next := bnd.clone()
			if !bindArg(next, from, Value{S: name}) || !bindArg(next, to, Value{S: dst.Name}) {
				continue
			}
			if len(atom.Args) > 2 {
				hops := float64(r.Depth[dst.Name])
				if !bindArg(next, atom.Args[2], Value{S: ftoa(hops), Num: &hops}) {
					continue
				}
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
		if !bi.accepts(len(atom.Args)) {
			return fmt.Errorf("query: %s takes %s args, got %d", rel, bi.arityLabel(), len(atom.Args))
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
//
// When the binding already fixes some of the atom's arguments, the candidates come from an index on
// exactly those positions instead of from the whole relation (WS3-125). unify still decides every
// candidate, so the index only ever has to avoid MISSING a match; see index.go.
func (b *Base) extendEDB(atom *Atom, fields []edbField, bnd *binding, yield func(*binding) error) error {
	facts := b.edb[atom.Relation]
	pos, all := b.edbCandidates(atom, fields, bnd)
	for i := 0; ; i++ {
		var f FactRow
		if all {
			if i >= len(facts) {
				break
			}
			f = facts[i]
		} else {
			if i >= len(pos) {
				break
			}
			f = facts[pos[i]]
		}
		b.countWork()
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
	for _, t := range b.idbCandidates(atom, bnd) {
		b.countWork()
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

// arityAccepts reports whether n is a valid argument count for any callable relation — built-in, EDB,
// or IDB — and whether the relation exists at all. It is the SAME admission test extendAtom applies
// on the positive path, so a variadic built-in (reaches, with its optional distance argument) cannot
// be accepted in a positive atom while its negation is rejected.
func (b *Base) arityAccepts(rel string, n int) (ok bool, known bool) {
	if bi, found := builtins[rel]; found {
		return bi.accepts(n), true
	}
	if fields, found := edbSchemaOf(rel); found {
		return n == len(fields), true
	}
	if ar, found := b.idbArity[rel]; found {
		return n == ar, true
	}
	return false, false
}

// arityLabelOf renders a relation's accepted argument count for an error message.
func (b *Base) arityLabelOf(rel string) string {
	if bi, ok := builtins[rel]; ok {
		return bi.arityLabel()
	}
	if fields, ok := edbSchemaOf(rel); ok {
		return fmt.Sprintf("%d", len(fields))
	}
	return fmt.Sprintf("%d", b.idbArity[rel])
}
