package query

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/jaala/datalog"
)

// topologyReachHops is the radius the reaches built-in searches at, the whole series neighborhood.
// check.Model.Reach is bounded by fan-out anyway (two-terminal pass elements only, no bus-like nets),
// so this only caps pathological depth.
//
// It is much wider than check.ProtectionReachHops because a topology question wants distant answers
// and a protection guard does not. Narrowing this silently shrinks every topology query. A rule that
// needs a specific radius states it with the third argument (WS3-112). See
// docsite/content/reference/relations/reaches.md and
// docsite/content/architecture/net-solving.md#how-far-a-walk-crosses-series-parts.
const topologyReachHops = 100

// relReaches is the built-in transitive relation reaches(from, to) and reaches(from, to, hops), the
// nets reachable from `from` through series pass elements, bridged to check.Model.Reach. It is a
// GENERATOR (it binds `to` by enumerating from the Model), which is why overlays get no public
// generator hook (see RegisterPredicate).
const relReaches = "reaches"

// relRoute is the built-in route(from, to, path), the SAME walk reaches makes with the route it
// found bound as a value instead of discarded. `path` renders as
//
//	VBUS -> [R5] -> VBUS_F -> [L1] -> VDD_3V3
//
// which is a string, so every query column stays scalar. It is a separate predicate rather than a
// fourth argument on reaches because arity 3 there already means hops.
//
// It yields exactly the pairs reaches yields, so a route never ENDS on a rail (the walk excludes
// bus-like nets); `agni trace` is the pin-to-pin form that does. ONE route per pair, the BFS tree
// path, so of two resistors bridging the same nets it names one. See
// docsite/content/reference/relations/route.md.
const relRoute = "route"

// predicates is every computed predicate a query may call: the engine's standard filters, the two
// circuit generators, and whatever an overlay registers through RegisterPredicate. It is the one
// value every Base in the process is built with.
var predicates = func() *datalog.Predicates {
	p := datalog.StandardPredicates()
	p.Add(relReaches, datalog.Builtin{Arity: 2, MaxArity: 3, Gen: genReaches})
	p.Add(relRoute, datalog.Builtin{Arity: 3, Gen: genRoute})
	return p
}()

// init claims this engine's computed predicate names with the fact layer, so a relation cannot
// shadow reaches or one of the string filters. The two packages' inits may run in either order, so
// facts.Reserve checks both directions.
func init() {
	facts.Reserve("core/query", predicates.Names()...)
}

// RegisterPredicate adds an extension-supplied filter predicate to the query surface. name is how
// queries call it, arity its argument count, and holds the boolean it computes over the (all-bound)
// argument values. It is a pure filter like the built-in contains/prefix/suffix, and `not name(...)`
// is derived from the same function.
//
// holds must depend only on its arguments, with no Model or fact-base access. A predicate that could
// ENUMERATE bindings (a generator, like reaches) could produce values and break the finiteness that
// makes evaluation terminate, so there is no generator hook until a real need designs one safely. It
// panics on a duplicate or shadowing name, at load rather than at query time, as net/http.Handle does.
func RegisterPredicate(name string, arity int, holds func(args []Value) (bool, error)) {
	if name == "" {
		panic("query: RegisterPredicate with empty name")
	}
	if arity < 1 {
		panic(fmt.Sprintf("query: RegisterPredicate(%q) needs arity >= 1", name))
	}
	if holds == nil {
		panic(fmt.Sprintf("query: RegisterPredicate(%q) with nil predicate", name))
	}
	if predicates.Has(name) {
		panic(fmt.Sprintf("query: RegisterPredicate(%q) collides with a built-in predicate", name))
	}
	// Reserve first, since it panics when the name is already a fact relation, so a rejected
	// predicate leaves nothing behind.
	facts.Reserve("core/query", name)
	predicates.Add(name, datalog.Filter(arity, holds))
}

// GeneratorFirstRules reports the rules that OPEN their body with a value-producing generator whose
// own input argument is unbound, naming each offender by its head relation. A shipped profile rule
// opened with `reaches(?n, ?rn, ?h)` and took `agni check` from 13s to not finishing at all on a real
// design (WS3-114). See the engine's documentation for what it does and does not catch.
func GeneratorFirstRules(q Query) []string { return datalog.GeneratorFirstRules(q, predicates) }

// genReaches binds reaches(from, to) and reaches(from, to, hops). `from` is a bound/const net when
// possible, else every net is a candidate start; `to` binds to each net reachable from it (reflexive,
// so from==to holds at distance 0).
//
// The optional third argument binds the ACTUAL number of series crossings, not a budget. A radius is
// written reaches(?n,?rn,?h), ?h<=2, and reaches(?n,?rn,2) means exactly two hops, missing anything
// closer (docsite/content/reference/relations/reaches.md).
func genReaches(src datalog.Source, args []datalog.Arg, emit func([]Value, []string) error) error {
	return walk(relReaches, src, args, func(r check.Reach, dst *ir.Net) (Value, bool) {
		if len(args) <= 2 {
			return Value{}, false
		}
		return datalog.N(float64(r.Depth[dst.Name])), true
	}, emit)
}

// genRoute binds route(from, to, path), the walk genReaches makes with the route rendered into the
// third argument rather than the distance.
//
// A path is bound and never TESTED against, which keeps it safe as a generator output. The value is
// drawn from the walk, so the strings it can produce are bounded by the design, as the net names in
// the first two arguments are, and evaluation stays finite.
func genRoute(src datalog.Source, args []datalog.Arg, emit func([]Value, []string) error) error {
	return walk(relRoute, src, args, func(r check.Reach, dst *ir.Net) (Value, bool) {
		return Value{S: r.RouteLine(dst)}, true
	}, emit)
}

// walk is the body reaches and route share. It resolves the start argument to one net or to every
// net, walks from each, and emits every net the walk reached, with third supplying the third argument
// when the atom has one.
//
// The START RESOLUTION is shared so it cannot drift between the two. A bound or constant `from` is
// one walk; an unbound one walks from every net on the board, which is what GeneratorFirstRules
// reports.
func walk(rel string, src datalog.Source, args []datalog.Arg, third func(r check.Reach, dst *ir.Net) (Value, bool), emit func([]Value, []string) error) error {
	ms, ok := src.(*modelSource)
	if !ok || ms.model == nil {
		return nil // spec library mode (NewSpecLibBase): no design topology, so the walk yields nothing
	}
	starts := ms.netByName
	if args[0].Bound { // from is bound/const: one start
		starts = map[string]*ir.Net{args[0].Value.S: ms.netByName[args[0].Value.S]}
	}
	// One buffer for every solution, because the engine copies out of vals and cites before emit returns.
	vals := make([]Value, 0, 3)
	cites := make([]string, 1)
	for name, start := range starts {
		if start == nil {
			continue
		}
		cites[0] = rel + " from " + name
		r := ms.model.Reach(start, topologyReachHops)
		for _, dst := range r.Nets {
			vals = append(vals[:0], Value{S: name}, Value{S: dst.Name})
			if v, ok := third(r, dst); ok {
				vals = append(vals, v)
			}
			if err := emit(vals, cites); err != nil {
				return err
			}
		}
	}
	return nil
}
