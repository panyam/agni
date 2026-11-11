# core/query

This file covers agni's adapter over the jaala Datalog engine, and how queries bind, aggregate, type and render.

## The engine and the adapter

- **The Datalog engine is a separate module, `github.com/panyam/jaala/datalog`** (agni 731). `core/query` is agni's adapter over it.
- The adapter holds the `Source` that projects a Model through the fact registry. It also holds a fact base per design over one vocabulary, `RuleFromQuery`, the wire form and the catalog.
- **A language change goes to jaala first.** A parser, evaluator, index or aggregation change is a jaala PR and a tag, then a `go get` here. Its issues live on panyam/jaala.
- **agni answers with `query.Default`**, jaala's `SemiNaive`. It plans each rule body. A body that calls a relation with a bound argument derives that relation for that value alone, so the written clause order does not decide cost.
- Tests compare against `Naive` as the reference. A new call site that evaluates for a user uses `query.Default`.
- **A generator declares the bindings it accepts** in `ns.Builtin.Modes`. `net.reaches` and `net.route` declare `from` bound, or a deliberate full walk.
- **A jaala bump can change a rule's plan**, so time Jetson's Run checks across one (agni 1037). Bisect by building agni against each release and timing one rule with `check --rule <name> --timing`. `stdlib/profiles/esdplan_test.go` holds the ESD profile's shape to a work budget.

## Names and the library

- **Every queryable name registers in `core/facts`** at a path of jaala's `ns.Vocabulary` (agni 751). The calls are `facts.RegisterRelation`, `facts.RegisterPredicate` and `facts.RegisterModules`. `core/facts/CLAUDE.md` covers that layer.
- **A predicate is an `ns.Builtin`.** One example is the `net.reaches` generator in `stdlib/relations`. It reaches the design through `facts.EnvOf`.
- **The shipped library of derived relations is `stdlib/lib`**, one `.dl` file per module (`component.probed_both`, `net.has_test_point`). `agni query --relations <path>` prints any member's signature and definition.
- **The library's modules register as ONE batch.** The vocabulary is checked at every registration, so a module that reads another module's member is refused if it arrives first.
- **A library reaches a query three ways** (agni 788). They are the shipped one, a project's `lib/`, and modules sent with a request (`AnalysisConfig.library_modules`, the CLI's `--lib`). All three compose in `Overlay.Registry`.

## Routes

- **`net.reaches` answers whether, and `net.route` answers how.** `net.route(from, net, path)` runs the same walk and binds the route as a rendered string (`VBUS -> [R5] -> VBUS_F`). A connectivity answer then carries its own evidence.
- `net.route` holds for exactly the pairs `net.reaches` holds for, so a route never ends on a rail. `agni trace` is the pin-to-pin form, and it does end on one.
- `net.route` gives ONE route per pair, the BFS tree path. It cannot speak about parallel paths.
- **The rendered route string is a CONTRACT.** `model.RenderRoute` owns it, and nothing else writes it. DECISIONS.md records why a path became a column.

## Output formats and sets

- **`query` emits five formats**, `--format text|csv|json|markdown|html`, plus `--title`.
- **markdown and html are DOCUMENTS.** They carry the title, the design and the query above the answer, so a saved view states its question. Both render `core/report.Table`.
- **csv carries no preamble.** Its first row has to be the header that something binds to. The csv encodes the engine's projection, as every csv does.
- An empty result never produces an empty artifact.
- **`--set <file>` asks a named SET of queries over one read** (agni 729). It writes one document with a section per query and refuses csv. It exits non-zero AFTER writing when any query failed.
- `RunQueries` answers each query exactly as `RunQuery` does. Both call the `answer` helper in `service/query.go`.

## Bindings

- **A value enters a query as a BINDING, never as spliced text** (agni 793). The forms are `--bind name=value`, `RunQueryRequest.bindings` and a set's `bind:`.
- **A variable can be bound to a LIST** (agni 1121). The forms are `QueryValue.list`, `--bind` naming the variable twice, `bind: {n: [A, B]}`, and repeated `bind` params in a link. The query then answers for each value.
- `query.Bindings` holds a variable's values everywhere in Go.
- The viewer's search and click presets bind too (`EntityQuery.binds`, `SearchQuery.bind`/`pattern`). They show the values as chips, so a served preset's text never changes.
- The engine refuses a binding that the goal does not use.
- **A list holding ONE value is substituted as a constant**, so it cannot be aggregated (agni 904). A question that counts a bound variable counts a copy of it instead (`?part = ?ref`). The area questions in `core/query/entity.go` do this.

## Numbers and units

- **A position filled from a numeric slot is declared a NUMBER automatically.** `numericTypes` in `core/facts` derives it from the relation's `FieldNum`/`FieldMin` layout.
- The engine reads a constant or bound value in a numeric position as a number. So `"3"` against a count is 3, and `"abc"` is refused.
- **An UNDECLARED position keeps a value as given** (panyam/jaala#65). A text constant compared with an untyped number answers nothing rather than failing.
- **A length is published in metres and shown in millimetres** (agni 1035). The engine folds a query literal to its SI base. A threshold over a length writes its unit (`?w < 0.2mm`), and a bare `0.2` means metres.
- `query.Display` turns a metre value back into mm wherever a value becomes text.

## Aggregation

- **Aggregation reduces BINDINGS, not values, unless you say `distinct`.** `count/min/max/sum/avg/list` group by the projection's plain columns. `count(distinct ?x)` reduces the SET of values.
- **A goal that joins two things yields one binding per combination.** On a net with 7 test points and 20 capacitors, `count(?tp)` is 140 and `count(distinct ?tp)` is 7.
- `distinct` works the same across every function, `list` included. An implicitly distinct `list` would make `count(?r)` and `list(?r)` in one projection disagree about the group.
- **`list` answers a list VALUE in JSON text** (`["R3","R4"]`, `[]` over nothing). Never split its cell on spaces.
- **`having` filters the GROUPS after the reduce**, as in `... => ?p having count(distinct ?n) = 1`. A goal comparison runs before grouping and has nothing to count.
- An aggregate can be filtered on without being projected (agni 613). The answer then lists the subjects rather than the tally.
- A derived relation also gives a distinct reduce. It projects the extra variable away before the group forms.
- **An aggregate-only projection always answers ONE row**, as SQL's `COUNT(*)` does (agni 726). A count over nothing is `0`. A grouped projection over nothing is still empty.
- **Every row of a finding query becomes a finding.** So `RuleFromQuery` refuses a finding query whose subject is not a plain projected column.

## Column kinds

- **jaala's `ColumnKinds` types a projected variable** from the catalog relations in the goal, then follows user rules into their bodies. `service/query.go` maps its answer (agni 751).
- **A variable's entity kind survives a derived relation** (agni 654). A bucket built with negation is clickable without repeating a catalog atom. The walk follows more than one hop.
- Rules that DISAGREE about a head position yield a scalar. A column typed from the first rule written would be wrong for the other rule's rows.
- A rule wrapping `entity(?name, ?kind)` yields a scalar. That kind varies per row, and a head argument has no per-row identity to carry it.
- **A recursive clause ABSTAINS rather than vetoing**, so the base case types a transitive closure.
