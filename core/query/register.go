package query

import (
	"fmt"

	"github.com/panyam/agni/core/facts"
)

// This file is the evaluator's half of the extension story. A RELATION is data derived from the
// Model and registers with core/facts, which knows nothing about datalog (C29); a PREDICATE is a
// filter this evaluator computes, so it registers here.

// init claims this engine's computed predicate names with the fact layer, so a relation cannot
// shadow reaches or one of the string filters. The two registrations are independent imports and may
// run in either order, which is why facts.Reserve checks both directions rather than assuming it
// went first.
func init() {
	names := make([]string, 0, len(builtins))
	for name := range builtins {
		names = append(names, name)
	}
	facts.Reserve("core/query", names...)
}

// RegisterPredicate adds an overlay-supplied filter predicate to the query surface. name is how
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
	if facts.IsRelation(name) {
		panic(fmt.Sprintf("query: RegisterPredicate(%q) collides with a fact-base relation", name))
	}
	if _, ok := builtins[name]; ok {
		panic(fmt.Sprintf("query: RegisterPredicate(%q) collides with a built-in predicate", name))
	}
	facts.Reserve("core/query", name)
	builtins[name] = filterBuiltin(arity, holds)
}

// edbSchemaOf resolves an EDB relation's positional layout from the fact layer, so the evaluator,
// negation, and arity checks treat an overlay-registered relation exactly like a built-in one.
func edbSchemaOf(rel string) ([]facts.Field, bool) { return facts.SchemaOf(rel) }

// isEDBRelation reports whether a relation is a fact-base relation (built-in or overlay-registered),
// so a rule may not redefine it and a rule body may read it.
func isEDBRelation(rel string) bool { return facts.IsRelation(rel) }
