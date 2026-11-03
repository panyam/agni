package relations

import (
	"context"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/jaala/ns"
)

// The two circuit walks a query can call, registered as generators over the Model (agni issue 751).
// They read nothing but a check.Model, so they live with the relations that project it rather than in
// the query engine's adapter.

// topologyReachHops is the search radius of net.reaches, the whole series neighborhood.
// check.Model.Reach is bounded by fan-out anyway (two-terminal pass elements only, no bus-like nets),
// so this only caps pathological depth.
//
// It is much wider than check.ProtectionReachHops because a topology question wants distant answers
// and a protection guard does not. Narrowing this silently shrinks every topology query. A rule that
// needs a specific radius states it with the third argument (WS3-112). See
// docsite/content/reference/relations/net.reaches.md and
// docsite/content/architecture/net-solving.md#how-far-a-walk-crosses-series-parts.
const topologyReachHops = 100

// RelReaches is the transitive relation net.reaches(from, to) and net.reaches(from, to, hops). It
// holds the nets reachable from `from` through series pass elements, bridged to check.Model.Reach.
// It is a GENERATOR, and it binds `to` by enumerating from the Model. Every value it emits is a net
// in the design or a distance the walk measured. Any generator owes that. A generator that invented
// values would break the guarantee that evaluation terminates.
const RelReaches = "net.reaches"

// RelRoute is net.route(from, to, path). It makes the SAME walk as reaches, and it binds the route
// it found as a value instead of discarding it. `path` renders as
//
//	VBUS -> [R5] -> VBUS_F -> [L1] -> VDD_3V3
//
// That value is a string, so every query column is still scalar. Arity 3 on reaches already means
// hops, so route is a separate predicate rather than a fourth argument on reaches.
//
// It yields exactly the same pairs as reaches. The walk excludes bus-like nets, so a route never
// ENDS on a rail; `agni trace` is the pin-to-pin form that does. It gives ONE route per pair, the
// BFS tree path, so of two resistors bridging the same nets it names one. See
// docsite/content/reference/relations/net.route.md.
const RelRoute = "net.route"

// walkModes are the bindings both walks accept. With `from` bound a walk starts from one net. With
// nothing bound it starts from every net on the board, and an all-pairs question legitimately asks
// for that. Declaring the second mode makes the full walk a stated choice. Wherever the body can
// bind `from` first, the engine's planner runs the walk after it is bound. The planner uses the
// full walk only when nothing can bind it. The WS3-114 rule that made `agni check` non-terminating
// opened with exactly that full walk.
var walkModes = [][]bool{{true, false, false}, {false, false, false}}

func init() {
	netArg := ns.ArgType{Kind: check.KindNet}
	facts.RegisterPredicate(RelReaches, ns.Builtin{
		Arity: 2, MaxArity: 3, Gen: genReaches, Modes: walkModes,
		Labels: []string{"from", "net", "hops?"},
		Types:  []ns.ArgType{netArg, netArg, {}},
		Doc:    "transitive reachability through series pass elements (R/L/ferrite/fuse); the optional third argument binds the EXACT number of crossings, so a radius is written `net.reaches(?a,?b,?h), ?h <= 2` and not `net.reaches(?a,?b,2)`, which means exactly two",
	})
	facts.RegisterPredicate(RelRoute, ns.Builtin{
		Arity: 3, Gen: genRoute, Modes: walkModes,
		Labels: []string{"from", "net", "path"},
		Types:  []ns.ArgType{netArg, netArg, {}},
		Doc:    "the same walk as `net.reaches`, with the route it found bound as a readable value (`VBUS -> [R5] -> VBUS_F -> [L1] -> VDD_3V3`), so a connectivity answer carries the evidence for itself; one route per pair, and a route never ends on a rail because the walk refuses one",
	})
}

// genReaches binds net.reaches(from, to) and net.reaches(from, to, hops). `from` is a bound/const net
// when possible, else every net is a candidate start; `to` binds to each net reachable from it
// (reflexive, so from==to holds at distance 0).
//
// The optional third argument binds the ACTUAL number of series crossings, not a budget. A radius is
// written net.reaches(?n,?rn,?h), ?h<=2, and net.reaches(?n,?rn,2) means exactly two hops, missing
// anything closer (docsite/content/reference/relations/net.reaches.md).
func genReaches(ctx context.Context, src ns.Source, args []ns.Arg, emit func([]ns.Value, []string) error) error {
	return walk(ctx, RelReaches, src, args, func(r check.Reach, dst *ir.Net) (ns.Value, bool) {
		if len(args) <= 2 {
			return ns.Value{}, false
		}
		return ns.N(float64(r.Depth[dst.Name])), true
	}, emit)
}

// genRoute binds net.route(from, to, path). It makes the same walk as genReaches and renders the
// route into the third argument instead of the distance.
//
// A path is bound and never TESTED against, so it is safe as a generator output. Its value comes
// from the walk, so the design bounds the strings it can produce, as it bounds the net names in the
// first two arguments. Evaluation therefore remains finite.
func genRoute(ctx context.Context, src ns.Source, args []ns.Arg, emit func([]ns.Value, []string) error) error {
	return walk(ctx, RelRoute, src, args, func(r check.Reach, dst *ir.Net) (ns.Value, bool) {
		return ns.Value{S: r.RouteLine(dst)}, true
	}, emit)
}

// walk is the shared body of reaches and route. It resolves the start argument to one net or to
// every net, walks from each, and emits every net the walk reached. When the atom has a third
// argument, third supplies it.
//
// The START RESOLUTION is shared so it cannot drift between the two. A bound or constant `from` is
// one walk; an unbound one walks from every net on the board, the second of walkModes.
//
// The name index is built once per fact base (Env.Memo) rather than per call. The walk runs once
// per binding of the atom that calls it, so a per-call index would make it quadratic.
func walk(ctx context.Context, rel string, src ns.Source, args []ns.Arg, third func(r check.Reach, dst *ir.Net) (ns.Value, bool), emit func([]ns.Value, []string) error) error {
	env := facts.EnvOf(src)
	if env == nil || env.Model == nil {
		return nil // spec library mode: no design topology, so the walk yields nothing
	}
	m := env.Model
	byName := env.Memo("relations.netByName", func() any {
		idx := map[string]*ir.Net{}
		for _, n := range m.Nets() {
			idx[n.Name] = n
		}
		return idx
	}).(map[string]*ir.Net)
	starts := byName
	if args[0].Bound { // from is bound/const: one start
		starts = map[string]*ir.Net{args[0].Value.S: byName[args[0].Value.S]}
	}
	// One buffer for every solution, because the engine copies out of vals and cites before emit returns.
	vals := make([]ns.Value, 0, 3)
	cites := make([]string, 1)
	for name, start := range starts {
		if start == nil {
			continue
		}
		// A query's context reaches the walk, so a cancelled or timed-out query stops between start
		// nets rather than finishing an all-pairs walk nobody is waiting for.
		if err := ctx.Err(); err != nil {
			return err
		}
		cites[0] = rel + " from " + name
		r := m.Reach(start, topologyReachHops)
		if err := env.Spend(ctx, reachCost(r)); err != nil {
			return err
		}
		for _, dst := range r.Nets {
			vals = append(vals[:0], ns.Value{S: name}, ns.Value{S: dst.Name})
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

// reachCost counts what one walk examined, the connections on every net it reached. The engine
// charges a generator per row it emits. A walk also reads each reached net's connections to find
// the next crossing. Without this cost, a walk over wide nets would charge a budget little more
// than its answer does.
func reachCost(r check.Reach) int64 {
	var n int64
	for _, net := range r.Nets {
		n += int64(len(net.Connections))
	}
	return n
}
