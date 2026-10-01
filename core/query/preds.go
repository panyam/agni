package query

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/jaala/datalog"
)

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

// relReaches is the built-in transitive relation net.reaches(from, to) and net.reaches(from, to, hops): nets
// reachable from `from` through series pass elements, bridged to check.Model.Reach. It is a GENERATOR
// (it binds `to` by enumerating from the Model), which is why a public generator seam for overlays is
// deliberately withheld (a value-producing generator could break the finiteness guarantee), while
// reaches stays a built-in.
const relReaches = "net.reaches"

// relRoute is the built-in net.route(from, to, path): the SAME walk reaches makes, with the route it
// found bound as a value instead of discarded. `path` renders as
//
//	VBUS -> [R5] -> VBUS_F -> [L1] -> VDD_3V3
//
// which is a string, so every query column stays scalar and a route survives into a csv cell, a
// markdown table and a rule's finding unchanged. That is the whole reason this is a separate
// predicate rather than a fourth argument on reaches: arity 3 there already means hops, so a path
// could only be reached by binding a distance the caller did not ask for.
//
// It yields exactly the pairs reaches yields, deliberately, so the two never disagree about what is
// connected to what. One consequence follows and is worth stating: a route never ENDS on a rail
// here, because the walk excludes a bus-like net entirely rather than admitting it as a terminus.
// `agni trace` is the pin-to-pin form that does admit one.
//
// ONE route per pair, not every route. The walk is a BFS and the path is its tree path, so where two
// resistors bridge the same two nets the answer names one of them and is silent about the other.
const relRoute = "net.route"

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
// shadow reaches or one of the string filters. The two registrations are independent imports and may
// run in either order, which is why facts.Reserve checks both directions rather than assuming it
// went first.
func init() {
	facts.Reserve("core/query", predicates.Names()...)
}

// RegisterPredicate adds an extension-supplied filter predicate to the query surface. name is how
// queries call it, arity its argument count, and holds the boolean it computes over the (all-bound)
// argument values. It is a pure filter, the same kind as the built-in contains/prefix/suffix: it
// keeps a binding when holds is true, and `not name(...)` keeps it when holds is false — both derived
// from the one function, so they can never disagree. holds must depend only on its arguments: no
// Model or fact-base access. That is deliberate. A predicate that could ENUMERATE bindings from the
// Model (a generator, like reaches) can produce values, and a value-producing generator would break
// the finiteness guarantee that makes evaluation terminate — so a generator seam is withheld until a
// real need justifies designing it safely. It panics on a misregistration, because a duplicate or
// shadowing predicate is a programming error that must fail loudly at load, not silently at query
// time (the same contract as net/http.Handle).
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
	// Reserve first: it panics when the name is already a fact relation, and doing it before the
	// registration means a rejected predicate leaves nothing behind.
	facts.Reserve("core/query", name)
	predicates.Add(name, datalog.Filter(arity, holds))
}

// GeneratorFirstRules reports the rules that OPEN their body with a value-producing generator whose
// own input argument is unbound, naming each offender by its head relation. A shipped profile rule
// opened with `net.reaches(?n, ?rn, ?h)` and took `agni check` from 13s to not finishing at all on a real
// design (WS3-114). See the engine's documentation for what it does and does not catch.
func GeneratorFirstRules(q Query) []string { return datalog.GeneratorFirstRules(q, predicates) }

// genReaches binds net.reaches(from, to) and net.reaches(from, to, hops). `from` is a bound/const net when
// possible, else every net is a candidate start; `to` binds to each net reachable from it (reflexive,
// so from==to holds at distance 0).
//
// The optional third argument binds the ACTUAL number of series crossings, so it is an exact value,
// not a budget. A radius question is therefore written with a comparison — net.reaches(?n,?rn,?h), ?h<=2
// — and NOT as net.reaches(?n,?rn,2), which binds by equality and so means "exactly two hops away",
// missing anything closer. The catalog entry and the reference doc say this outright, because the
// constant form is the spelling a reader reaches for first and it silently means something else.
func genReaches(src datalog.Source, args []datalog.Arg, emit func([]Value, []string) error) error {
	return walk(relReaches, src, args, func(r check.Reach, dst *ir.Net) (Value, bool) {
		if len(args) <= 2 {
			return Value{}, false
		}
		return datalog.N(float64(r.Depth[dst.Name])), true
	}, emit)
}

// genRoute binds net.route(from, to, path): the walk genReaches makes, with the route rendered into the
// third argument rather than the distance.
//
// A path is bound and never TESTED against, which is what keeps it safe as a generator output. The
// value is drawn from the walk, so the set of strings it can produce is bounded by the design the
// same way the net names in the first two arguments are, and the finiteness the evaluator rests on
// is untouched.
func genRoute(src datalog.Source, args []datalog.Arg, emit func([]Value, []string) error) error {
	return walk(relRoute, src, args, func(r check.Reach, dst *ir.Net) (Value, bool) {
		return Value{S: r.RouteLine(dst)}, true
	}, emit)
}

// walk is the body reaches and route share: resolve the start argument to one net or to every net,
// walk from each, and emit every net the walk reached, with third supplying the third argument when
// the atom has one.
//
// One body rather than two because the START RESOLUTION is the part with the trap in it, and a
// second copy is a second place for it to drift. A bound or constant `from` is one walk; an unbound
// one walks from every net on the board, which is what GeneratorFirstRules exists to report.
func walk(rel string, src datalog.Source, args []datalog.Arg, third func(r check.Reach, dst *ir.Net) (Value, bool), emit func([]Value, []string) error) error {
	ms, ok := src.(*modelSource)
	if !ok || ms.model == nil {
		return nil // spec library mode (NewSpecLibBase): no design topology, so the walk yields nothing
	}
	starts := ms.netByName
	if args[0].Bound { // from is bound/const: one start
		starts = map[string]*ir.Net{args[0].Value.S: ms.netByName[args[0].Value.S]}
	}
	// One buffer for every solution: the engine copies out of vals and cites before emit returns.
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
