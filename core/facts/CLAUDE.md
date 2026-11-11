# core/facts

This file covers the fact and relation layer, its registry, and the edits a new relation needs.

## The layer

- **`core/facts` depends on NO query engine** (C29). A relation projects a `check.Model` into tuples and registers with `facts.RegisterRelation`.
- An engine that answers over those tuples imports `core/facts`, never the reverse. Datalog (`core/query`, over jaala) is one query shape among several.
- `check.Spec` answers per-entity questions with no fact base at all. Datalog cannot express a path question (agni 518).
- **C29 lets this layer import `jaala/ns` and `jaala/stdlib` and nothing more of jaala.** `jaala/ns` holds names and signatures, and `jaala/stdlib` holds the standard predicates. Neither holds an evaluator.
- **Every queryable name registers at a path of jaala's `ns.Vocabulary`** (agni 751), through `facts.RegisterRelation`, `facts.RegisterPredicate` and `facts.RegisterModules`.
- **A predicate reaches the design through `facts.EnvOf`.**
- **No core package outside `core/query` knows a query language.** `core/review` compiles a manifest's inline query through a registered `review.QueryCompiler`. `stdlib/reviewquery` is the datalog one.

## The registry

- **A relation vocabulary is a composed VALUE.** `facts.DefaultRegistry` builds it, the twin of `check.DefaultCatalog`.
- The package globals are an append-only registration buffer. Every READ goes through a `*Registry` that a caller holds.
- **Installing no relation catalog still builds and runs.** The fact base is then empty, and every datalog rule reports clean. `Registry.Installed` tells that case apart from a query that matched nothing.
- **At init time the registry is EMPTY** (agni 540). `stdlib/profiles` compiles its built-in profiles in an `init()`, and that runs before any relation catalog registers. Code that validates a query against `facts.DefaultRegistry()` at construction must stand its vocabulary checks down. Otherwise it calls every relation unknown.
- **A numeric slot declares its position a NUMBER.** `numericTypes` derives this from the relation's `FieldNum`/`FieldMin` layout.

## Relations

- **`pin.name(ref_des, pin, name)` is the pin's FUNCTIONAL name.** Every other pin relation keys on the package designator. Both come from `ir.Pin`.
- `pin.name` is projected VERBATIM. Canonicalizing belongs on the comparison, never in the projector.

## Adding a relation

- The wiring table in `CONTRIBUTING.md` ("Wiring, per subsystem") lists seven edits plus `make catalog-docs`. Two of them are easy to miss.
- **The sixth edit is `service/testdata/columnkinds.golden`** (agni 687). It snapshots how every query column types. Regenerate it with `UPDATE_GOLDEN=1 go test ./service/ -run TestColumnKindsMatchGolden`.
- READ the golden diff. A new relation only adds lines. Any other change means an existing column was retyped.
- **The seventh edit is `dimensionlessNumericRelations` in `stdlib/relations/facts_test.go`**, for a relation whose number is a bare count (agni 727). `TestNoRelationPublishesAnUnlabelledNumber` fails until the count is listed there.
