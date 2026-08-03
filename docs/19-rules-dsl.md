# 19 — Rules & checks: requirements, prior art, evaluation model

The design/requirements survey for the rules layer. It answers three
questions: what kinds of rules must we express, what expressive power do they need, and
how do we evaluate them. The competitive/positioning analysis lives in the private research
notes, not here.

## Scope boundary: rules assert, analysis computes

A **rule** asserts something must hold over a design and reports where it does not:
"every I2C net has a pull-up," "no output pin drives another output." A rule reads the IR
and produces findings; it does not simulate or solve. Worst-case tolerance, timing, and SI
are **analysis**, a different engine. Drawing this line keeps the rules layer a
query-and-assert system, not a general compute environment.

**Rules and analysis cooperate without blurring the line.** Some rules assert over a
quantity that analysis computes: an inductor's saturation-current margin needs the peak
current, a capacitor's derating needs a rail's worst-case maximum. The rule references that
quantity by name through an interface the analysis engine fills; the rule still only asserts and reports, it
never simulates. The boundary stays crisp even where a rule and an analysis compose.

**A third category sits beside rules and analysis: queries that report.** Some asks are not
pass/fail. Group the BOM by sub-circuit and roll cost up against an external supply feed, for
instance. A query reuses the same select / traverse / aggregate / join primitives but emits a
**table, not findings**. Keeping queries a sibling surface preserves the rule layer as a clean
pass/fail contract; a report is not a rule.

## Where a rule runs: input diagnostics vs analysis checks

A rule is what a user cares about, a named thing that should fire on a design. But rules do not
all run in the same place, and conflating that leaks one format's structure into the shared engine.
The line is a compiler's: parsing catches malformed input; name resolution catches duplicate
declarations while building the symbol table; type-checking and dataflow run over the built
program. Our layers map onto that.

**The litmus test: can the rule be computed from the final netlist IR alone?**

- **No** (it needs detail the reader *normalized away*: the pre-merge placements, the raw label
  set, the wire geometry): it is an **input diagnostic**. The reader detects it while building the
  IR, applying its own format's semantics, and records a **neutral** result. Example:
  duplicate-ref-des. The IR merges components by ref_des on purpose (a multi-unit part is one
  component with sections), so by netlist time the collision is gone; only the reader,
  mid-merge, knows a genuine duplicate from a legitimate multi-unit part (KiCad: same unit claimed
  twice; a flat netlist: a repeated designator). Likewise dangling-endpoint: the wire geometry is
  gone by netlist time.
- **Yes** (nets, connections, pin electrical types are enough): it is an **analysis check** in
  `check/`: output-output-conflict, floating-input, decoupling-presence. These run over the IR the
  same way regardless of source format.

**The front-end emits two kinds of derived output, and they are not the same thing:**

- **Input diagnostics** are *problems* ("this is wrong"): duplicate-ref-des, dangling-endpoint,
  conflicting-net-name. They are reportable as findings and live in a neutral `InputDiagnostics`
  message on the design.
- **Input facts** are *annotations* ("this is so"): a net is PWR_FLAG-driven, a net is cross-sheet,
  a net's class. They are not findings; they are data a later check reads, and they live as
  attributes/typed fields on the entity they describe. This is already load-bearing:
  `power-input-not-driven` consumes the reader's `power_driven`/`external` net facts to avoid
  false positives, the front-end handing an attributed netlist to the analyzer.

**Vocabulary.** *Rule* is the umbrella (what the catalog, the expectation sidecar, and the viewer
track). A *check* is a rule computed by the analysis engine over the IR. A *diagnostic* is a rule
detected by the reader from source structure the IR normalizes away. A *fact* is reader-derived data
a check reads, not itself a rule. A rule's implementation site (reader-diagnostic / IR-check /
parametric / geometric) is a tag on it, not a separate catalog.

**Consequence for design.** A check that cannot be computed from the netlist IR does not belong in
`check/`; pushing its format-specific judgment up into a rule (a KiCad `unit`-index heuristic, say)
is the smell this rule prevents. Detection goes to the reader; the neutral result goes into
`InputDiagnostics`; the `check/` rule that reports it is thin and format-agnostic. Input diagnostics
therefore run at read time; they exist before any rule is selected, so a viewer or `stats` can
surface them without invoking the analysis engine.

**A reader may legitimately contribute nothing.** A diagnostic is only producible by a reader whose
format carries the needed structure: dangling-endpoint needs wire geometry (schematic readers only);
ref-des-collision needs capture-unit semantics (KiCad-schematic has them, a flat EDIF netlist does
not, its multi-gate grouping is indistinguishable from a duplicate without a unit). An empty
contribution there is correct, not a gap, the same way a board/netlist source yields no dangling
endpoints.

**Watch for silent false-negatives.** Because "no diagnostics" is indistinguishable from "diagnostics
this reader can't observe", input-diagnostic coverage cannot be inferred from a clean run. It is
pinned two ways: a **labeled corpus fixture** (a known-bad design, staged `pending` in the expectation
sidecar until the reader can catch it, so the gap is a visible row in the harness rather than tribal
memory), and a **source-tool-oracle cross-check** (diff our findings against the originating tool's
own ERC). Absent both, a missed diagnostic is invisible.

## What rules look like (expressiveness tiers)

The *set* of hardware rules is effectively unbounded (the design-intent tail below is
open-ended), but the *machinery* they need is bounded. Classifying by required expressive
power is the useful axis, because it decides the evaluation model:

- **Tier P, parametric.** A fixed, standardized catalog with per-process parameters:
  geometric DRC (clearance, track width, via/annular-ring, courtyard) and electrical ERC
  (pin-type conflicts, unconnected pins, single-pin nets). The rule *types* are finite; only
  values vary. Config-shaped, not language-shaped.
- **Tier R, relational / graph query.** Select and traverse the netlist, then quantify:
  "for every I2C net, there exists a pull-up to VCC," "is this net reachable from GND
  through only passives" (transitive closure). This is the bulk of the design-intent tail.
- **Tier A, aggregate.** Counts and ratios over the selections: "test-point coverage ≥ 95%,"
  "≥ 1 decoupling cap per power pin."
- **Tier X, external join.** Bring in data outside the design: approved-MPN list, part
  parametrics from a spec DB ("every passive has an MPN from the approved-vendor list").

**Expressive ceiling:** Tiers R+A+X are a *Datalog / relational-algebra + aggregation +
external relations* class: pattern-match, traverse, quantify, aggregate, join. Not
Turing-complete, not a general programming language. That bounded ceiling is what makes a
declarative rules layer feasible; anything needing real computation is analysis, by
the boundary above.

## What runs now vs later

- **Now (on the netlist IR):** ERC (Tier P electrical), and the connectivity/attribute/
  quantified/aggregate rules of Tiers R and A. This is where the value is and what we have
  the data for.
- **Now (on the board tier):** the first geometric DRC class (track width, hole
  size, annular width, copper clearance), over the board sidecar, gated by
  `Available`'s `board.` read prefix (a netlist-only design reports "unavailable", the same
  honest split the datasheet gate makes). Thresholds are fab-capability floors; per-design
  values are rule parameterization. Two structural notes the batch established:
  per-net threshold rules are ordinary Specs over a `board.nets` entity set, while
  **clearance is a pairwise cross-entity join the AST deliberately does not express**: a
  geometry-query/join vocabulary must be evidenced by more rules before it earns AST nodes
  (the earn-it guard), so clearance is the catalog's one purpose-built Go rule and
  the standing evidence. Its O(S²) walk is the tripwire: ~0.7ms at corpus scale
  (400 segments), ~16ms at 2k, ~380ms at 10k (`BenchmarkCopperClearance`).
- **Later:** the remaining DRC classes (pad/zone clearance, edge/silk, hole-to-hole,
  courtyard) need pad-shape and zone-fill facts; external joins (Tier X) need the parts/spec
  data source. Both are additive; the evaluation model below does not change.

The corpus-derived sequencing (the concrete corpus is maintained internally) splits the not-yet-built rules by what each waits on:

- **Buildable now, pure netlist:** signal-net naming convention, TX/RX connection-role
  compatibility, test-point coverage; diode orientation once pin polarity roles land (a
  small additive IR enrichment); the ordering variants of the ESD/protection rules
  once the reachability primitive lands.
- **With the parameter layer:** cap voltage derating, VIH/VIL vs VOH/VOL
  margin, passive-value-vs-recommendation, IC pin-mapping against a reference map. These
  are the differentiated Tier-X category: a rule that proves a margin from datasheet data.
- **Touch analysis for an input only:** inductor Isat vs peak current, cap voltage vs
  a computed rail max. The assert stays a rule; the analysis engine supplies the number through a named
  fact, so the boundary above holds.
- **Not rules:** BOM-cost-by-application and similar partition/aggregate/join reports are
  queries (see the scope boundary above): same primitives, tabular output, no pass/fail.

## Technical prior art (by mechanism)

| Approach | Mechanism | Fits tier | Note for us |
|---|---|---|---|
| Classic ERC/DRC engines | fixed parametric catalog | P | the built-in baseline; not extensible by users |
| KiCad `.kicad_dru` | text custom rules: `condition` + `constraint` expressions | P, some R | closest open, text-based precedent; geometry-focused |
| Datalog | facts + rules, transitive closure | R (+agg variants) | natural fit for netlist reachability/quantification |
| OPA / Rego | policy query language over JSON | R, A | proven policy-as-code, but a heavy external dep + JSON impedance with our proto IR |
| CUE | constraint + schema unification | P, some R | great for parametric/schema constraints, awkward for graph traversal |

Commercial constraint managers are surveyed in the private competitive notes; technically
they are Tier-P/R constraint systems bound to their host tool and design database.

## Evaluation model (the decision)

Rules evaluate **over the neutral IR** (the same posture as today's `check/`), producing
findings tied to **provenance** so each violation points back to source in every affected
revision (like the diff, doc 18). This makes rules format-agnostic and review-integrable.

**Phased, library-first (recommended):**

1. **Phase 1: an embedded rules *library* in Go.** Rules are Go predicates over the IR that
   emit provenance-tied findings, built on a small set of query primitives (`select`,
   `traverse`, `forEach`/`exists`, `count`). This generalizes `check/`'s two hardcoded rules
   (`single-pin-net`, `unconnected-component`) into a reusable kit. It validates *the
   primitives and the starter rule set against real designs* before we commit to syntax.
2. **Phase 2: grow a declarative DSL surface** (via the in-house **Galore** parser-generator,
   already in the stack) *once the library is well-tested and robust*. The DSL compiles down
   to the same primitives; the library is the runtime. We do **not** adopt Rego/CUE (external
   dep + impedance with the proto IR), and we do **not** build the language before its
   primitives are proven, the same earn-it discipline as CONSTRAINTS C9.

The `check.Finding` shape is the seed of the rule-result contract; **diff-gates** are rules
that run over a diff (doc 18) rather than a single design ("fail the review if a `Hard`
change touches a net tagged critical").

### Phase-1 rule model (as shipped)

The Phase-1 library landed with a deliberate split in the `check.Rule` shape, now a constraint
(CONSTRAINTS C14):

- **Typed core = what the engine acts on.** Only `Name`, `Severity`, `Reads` (the facts the rule
  reads), and `Eval` are typed fields, plus the prose (`Summary`/`Impact`/`Detail`). Everything
  classificatory (category, tier, distribution, and any provider-defined axis) is an open
  `Tags map[string]string`. Classification is data, not columns, so a rule from an operator, a
  Phase-2 DSL, or an integrator embedding Agni (rules outside `check/`) adds its own axes with no
  core change, and a browsable catalog groups and filters by whatever tag keys are present.
- **Availability derives from `Reads`, not a stored track.** `check.Available(rule, design)` reports
  a rule unavailable when it reads a fact whose provider layer is absent (a `param(...)` datasheet
  fact before the parameter layer lands), so a green "no findings" is distinguishable from "never
  ran." When that layer arrives, the same signature becomes design-aware with no wire change.
- **The catalog is injected, not a global, and composed from sources.** A
  `check.RuleSource` yields rules (the built-ins, an embedder's Go suite, later the DSL
  compiler's Spec output, one seam for all); `check.NewCatalog(sources...)` composes them
  under the collision policy: the anonymous built-in source keeps bare names, every other
  source is namespaced (`tesla/ctrs-naming`, copied so the source's own rules are never
  mutated) with a `source` tag stamped so one suite selects as an ordinary facet, and any
  post-composition name collision rejects at wiring time instead of shadowing silently.
  Consumers (`CheckService`, the CLI) hold a `*check.Catalog`. **Out-of-module registration:**
  `check.RegisterSource(src)` adds a suite from another module (house-style or
  proprietary rules in the open-core overlay) to a process-global registry, and
  `check.DefaultCatalog()` / `check.CatalogWith(extra...)` compose the built-ins + every
  registered source, so the engine's own CLI and serve pick the suite up with no re-wiring, 
  the rule-side twin of `formats.Register`. An embedder wanting explicit control still composes
  `check.NewCatalog(check.Builtins, src)` directly; a registered rule is a Go rule and does not
  join the built-in Spec-twin regression suite. Riders that build on the seam: per-design
  thresholds and `.kicad_dru`
  files arrive as sources yielding re-parameterized Spec rules; severity overrides and
  source-owned vocabulary are the next accretions (see OUT_OF_SCOPE.md triggers).
- **Findings carry their subject kind.** `check.Finding` records whether its subject is a net,
  component, or pin, so a consumer groups and highlights by entity exactly instead of string-guessing.

The web tier consumes this over the serve API (`ListRules` + a rule-subset selector on `CheckDesign`,
with a structured `Finding.Subject`) and renders it as a faceted rules catalog, a group-able results
panel with multi-subject highlight and a per-rule finding cache, and named rule bundles. None of that
touched the evaluation model; it all rides the typed-core + Tags contract above.

### A rule is a value (the spec layer)

The Phase-1 library gained a second authoring form (stakeholder direction): a rule
body can be a **`check.Spec`**: a small AST of the nine primitives over named facts,
evaluated by a tiny interpreter, instead of a Go closure. The typed core of `check.Rule`
(C14) is unchanged; a Spec binds into it by supplying the `Eval`. What the value form buys:

- **Rules are data.** A Spec is inspectable, diffable, and serializable. Phase 2 stops being
  a rewrite: the DSL parser's job is to produce a Spec value, and the interpreter is already
  the runtime ("the DSL compiles to the same primitives", now literally).
- **Derived metadata.** A spec-built rule's `Reads` and `Primitives` are computed from the
  body (`DerivedReads`/`DerivedPrimitives`), so the C14 metadata cannot drift from what the
  rule does. The FFI boundary keeps this honest too: a registered function declares the
  reads and primitives it consumes.
- **Go is a primitive, not the rule.** A `Call` node invokes a registered `SpecFunc` by
  name (`intentionally_unconnected`, `ground_name`, ...), so multi-clause heuristics stay in
  Go without making the whole rule opaque. This is the escape hatch a datasheet-joined rule
  or an integrator uses for the awkward ten percent.
- **One optimization seam.** The interpreter resolves every fact through `Model`, never the
  raw IR. Storage/indexing questions therefore have one answer: `Model` is the interface,
  today's `irModel` (precomputed maps + linear scans) is the naive implementation, and an
  indexed fact base is a drop-in replacement no rule or spec would notice. No
  external graph/datalog engine, the earn-it rule that rejected Rego/CUE applies to storage
  dependencies too. The same split governs the interpreter's vocabulary tables (entity
  sets, collections, facts): they are the language's closed lexicon and stay private, 
  a faster implementation of a name swaps in behind `Model` (one name, one meaning),
  while genuinely external vocabulary arrives via registration with the provider story,
  the way `RegisterSpecFunc` already works for functions.

The original thirteen rules carry both forms: the Go `Eval` stays canonical, and the
declarative twin lives in `check.Specs`, held to it by a parity gate (identical findings
over the unit fixtures and every conformance fixture) and a metadata gate (hand-written
`Reads`/`Primitives` equal the derived ones). Writing those twins was the acceptance test
that locked the nine primitives: every rule fit the AST plus five FFI helpers, and none
needed a tenth primitive.

**The twin discipline** (which rules get both forms) follows what a twin actually checks.
For the soaked originals, the Go side was an oracle, so their twins are the interpreter's
standing regression suite. They stay. For a *new* rule, a Go twin is a second guess by the
same author, weaker evidence than the conformance fixture pair every rule must ship anyway;
so a new rule on proven vocabulary is **spec-only**, authored as a `Spec` bound through
`Spec.Rule` (the connection-matrix rows in `rule_pin_matrix.go` are the first). A new rule
that introduces interpreter vocabulary (a new entity set, fact, or node) ships **with** a Go
twin as the bring-up reference until that vocabulary has more users (`unconnected-pin`,
which introduced the pins entity set, is the pattern). Flipping a twinned rule to
spec-canonical is a one-line change gated by the parity test; `output-output-conflict` was
the first flip when the matrix subsumed it.

Cost, measured (full 13-rule catalog over a synthetic 2000-net design, M4 Pro): the Go
closures run in ~1.6ms, the interpreter in ~8.7ms (~5.5x, allocation-dominated: boxing
entities and per-entity scope maps). Both are far below interactive thresholds, so the twin
form is affordable today; the benchmark pair (`BenchmarkRulesGo`/`BenchmarkRulesSpec`) is
the standing evidence for when a fact base earns its complexity.

### Fact vocabulary (what the Model exposes today)

A rule declares the facts it reads in `Reads`; the `check.Model` interface is where each fact
is computed. The current vocabulary: the entity selects (`Nets`, `Components`, and the
part-type pin set `Pins`), the reader input diagnostics (dangling endpoints, ref-des
collisions), `pin.direction` (`PinDir`), `on_net` (`IsConnected`) and its per-pin analogue
(`PinConnected`), per-pin net identity (`PinNetName`), the net-name pair primitive
(`HasNetName`), `component.class` (`ComponentClass`), `pin.role` (`PinRole`: see below),
and the design-level no-connect channel (`HasNoConnectChannel`: whether the source can
express "intentionally unconnected" at all, the gate that keeps per-pin absence rules quiet
on bare netlist exports).

Three connection-scope facts arrived with power symbols as typed virtual
connections. `pin.electrical_type` in connection scope resolves the connection's
`attributes["direction"]` FIRST and falls back to `PinDir` (`connDir` in Go): a virtual
power-symbol pin has no part type, so its direction travels on the connection itself.
`conn.virtual` is true when the connection's component is a virtual symbol (`#`-prefixed
ref-des, `#PWR`, `#FLG`); rules that mean "a load taps this rail" (decoupling,
input-protection) exclude virtual power_in, because a rail-name tap is not a load.
`pin.declared` (`PinDeclared`) distinguishes a pin the part type declares, even typed
explicitly "unspecified", from a pin known only through net connections (a board
footprint's pads, a sub-sheet component in a root-only hierarchy read): the
unspecified-pin-with-driver matrix row fires only on declared-but-untyped pins, so a read
gap never reads as an authoring gap.

### Pin DIRECTION is a hint, not ground truth — compose class/role

`pin.electrical_type` (INPUT/OUTPUT/…) comes from the source library and is UNRELIABLE across
formats: some libraries type a passive's pins INPUT (the Mentor EDIF corpus does for
capacitors), and diode/LED/TVS terminals arrive typed INPUT too. A rule that reads raw
direction alone will false-fire on exactly these. The RELIABLE intent facts are the DERIVED
ones the model already computes: `component.class` (resistor / diode / led / tvs / IC / …) and
`pin.role` (power / ground / anode / cathode, class-gated). So a direction-based rule must
GATE on class/role, not trust direction by itself: "is this a driven-or-floating logic input"
is `pin.electrical_type == input AND component.class not in {passives, diode-family}`, never
`pin.electrical_type == input` alone. This is one recurring false-positive family, not two: the
passive-INPUT exemption and the diode-terminal exclusion (a pair of steering-
diode cathodes read as an all-input net) are the same shortcut, patched in the same place. When
authoring a new direction-based rule, reach for the class/role facts first; the raw-direction
count is the trap.

The naming batch added: `net.name_leaf` (the leaf of a hierarchy-qualified
name, what convention patterns match by default), the collapsed-alias channel
(`netgraph.AttrAliases`: every label the naming pass folded into one net, with its scoping
rank; read via the `scoped_label_clash` / `rail_name_clash` FFIs), `Model.NetNameCount`
(exact-name claims, the `nets_sharing_name` FFI behind duplicate-net-name), and the first
CONFIG-CARRYING rule source: `check/naming` compiles an operator YAML convention
(allow/exempt regex sets) into ordinary namespaced catalog rules through the exported Spec
surface alone, config in, rules out, no second registry.

Rule PROSE is single-sourced: each built-in rule's Detail is one markdown file
under `check/docs/<name>.md` (plus its diagrams), embedded via go:embed, the examples'
walkthrough.md sidecar convention applied to rules. Embedding keeps single-binary, WASM,
and C1 intact; ListRules serves Detail as data either way, and external RuleSources carry
their own Detail (the naming source generates it from config). The rule↔doc 1:1 and every
image reference are harness-enforced (check/docs_test.go).

**reach** is the traversal primitive: `Model.Reach(net, hops)` walks from a net
through SERIES PASS ELEMENTS, two-net components classed R/L/ferrite/fuse, returning the
visited nets (BFS order) with the crossed elements and per-net paths (`PathTo`,
`ThroughOnPath`); `Between(from, to, class, hops)` is the on-path predicate. Stops are bus
evidence only: ground names, the `global` fact, rail-scale fan-out (>16 members), NOT
rail-looking names or `power_driven`, both of which mark exactly the power-entry paths
protection rules walk. Capacitors never pass (a series cap is a DC block), nor do diodes
(polarity, not a wire). Consumers: input-protection (a fuse crossed on the path to a
reached power input, or a TVS on a path net, protects it) and esd-protection (clamp
existence over 2-hop reach; power-pin classification over reach keeps the two rules' turf
split stable when a bead splits the entry net). The vocabulary is new, so the Go
implementations stay canonical and the spec twins call the shared walk through declared
FFIs (`unprotected_power_reach`, `tvs_reach`, `power_pin_reach`) per the twin discipline.
For a software-reader orientation (the walk as BFS across middleware that splits a
channel, and the four methods as an API), see [ANALOGY.md](ANALOGY.md#the-protection-walk-reach).

**pin.role** is the second class-style derived fact: anode/cathode/power/ground,
classified from the pin's declared NAME within the component's device-class context
(polarity only for the diode family, so an IC's "K" pin never reads as a cathode). The
format audit found no source that states polarity as data, KiCad diode pins are
electrically passive with "A"/"K" names, gEDA's pintype has no polarity, EDIF carries
directions only, so a typed `ir.Pin.role` field would fail C9 and every reader would be
guessing; the role is a Model projection instead, exactly the `component.class` reasoning.
First consumer: the led-polarity rule (the low-false-positive slice of the corpus
diode-orientation row; the general rule waits on net-polarity facts).

**component.class** classifies each placed component into a stable device class: `resistor`,
`capacitor`, `inductor`, `ferrite`, `diode`, `led`, `tvs`, `fuse`, `connector`, `test_point`,
`crystal`, `ic`, `transistor`, or `unknown`. It is a derived fact the Model computes, not an
IR field (C1: a projection, and C9: no format would populate it as source data). Derivation
order:

1. The ref-des letter prefix maps through a convention table (R and RN are resistor, FB is
   ferrite, TP is test point, and so on). A resolved PartType's `designator_prefix` replaces
   the ref-des guess when present, the same override the render-layer symbol registry uses.
2. Part-type text refines the base class within its device family: whole-token hints from the
   part name, part kind, and the component's `Value` attribute turn a D-prefix part into
   `led` or `tvs` and an L-prefix part into `ferrite`. A hint outside the family does not
   flip the class, so a resistor whose value string mentions an LED stays a resistor.
3. With no recognized prefix, a token hint classifies outright; otherwise the class is
   `unknown`. Ambiguous prefixes (X is a crystal in some house styles, a terminal block in
   others) stay `unknown` unless part data resolves them. Rules that quantify over a class
   therefore skip unfamiliar components rather than misfire.

`led`, `tvs`, and `ferrite` are deliberately distinct from `diode` and `inductor`: the
protection and decoupling rule batch quantifies over them separately. The fact is the shared
enabler for that batch (decoupling and bulk-cap presence, protection present, ESD, test-point
coverage, and the refined i2c-pull-up).

### The fact base — reads as materialized relations

The vocabulary above is what a rule *declares* it reads; `check.Facts(Model) []FactRow` is that
vocabulary *materialized* as named, typed, provenanced relation tuples — the substrate a rule
asserts over and an engineer's ad-hoc search queries over, so rules and search unify on one set of
relations. A `FactRow` is `relation(subject, object, value, num, conditions, cite)`: `Relation`
names the relation, `Subject`/`Object` are the entities, `Num` carries a numeric value for
range/compare, and **`Cite` is never empty** — a fact you cannot cite is not verifiable, and
verifiability is the point.

The seed schema is the four relations the cap-voltage rule reads, now emitted as facts:
`net.max_voltage(net, V)`, `component.mpn(ref_des, mpn)`, `param(mpn, symbol, value, conditions)`
(cited to the datasheet page), and `component-on-net(ref_des, net)`. A rule's declared `Reads` name
these relations (`on_net` is the `component-on-net` relation; `param.cap_rated_voltage` is `param`
filtered to the rated-voltage symbol) — the discipline is that a read is a relation.

Two properties hold it to the constraints. The projection is **derived, regenerated on demand,
never a second authoritative schema** (C8): there is no fact proto, `Facts` recomputes from the
Model, and a design read without a seeded datasheet set simply yields no `param`/`mpn` facts — the
same silent-by-construction posture the datasheet rules have. And the relation names are **neutral
IR/param concepts** (C9), not format specifics, so the fact base stays as neutral as the IR.

This is the fact-capture discipline only, not a query engine. More relations accrue as each rule
adopts the convention. No optimizer is planned: one design's fact base is small enough for naïve
evaluation, and the bounded expressiveness ceiling above is what keeps it that way.

**Which reader-diagnostics become query relations.** A diagnostic earns a `Facts()` relation
when it carries an **entity key to join on**; a point-geometry diagnostic (a bare coordinate) stays
rule-scoped, because a relation over `(x, y)` only duplicates the check panel's list with nothing to
join. So the entity-keyed diagnostics ARE relations — `ref_des_collision(ref_des)` and
`pin_net_conflict(ref_des, pin, net)` (one row per net the conflicted pin touches, so a query finds
every net a bad pin reaches) — while `dangling_endpoint` and `no_junction_endpoint` remain
Spec-`Over:` scopes only. `bus(label, kind)` fits the same rule (keyed by label), so it is
consistent, not an exception. Note the `pin_net_conflict` semantics: a ref-des that already collided
is excluded from conflict detection, because a duplicated designator legitimately spans nets — that IS
the collision, reported by `ref_des_collision`, not a pin-on-two-nets fault.

The same "name the shared predicate once" move applies to the reach walk: `net.bus_like(net)`
is a shared-distribution net (ground plane, global rail, or rail-scale fan-out `> maxWalkFan`) — the
exact predicate the series-reach walk stops crossing into, now named (`isBusLike`) and exposed as a
relation so it is one definition, not a hidden constant. It is distinct from `bus(label, kind)`, which
is a reader-detected unmodeled bus LABEL, not a high-fan-out net.

### Querying the fact base

The `query` package runs ad-hoc queries over these relations — "search your whole design as
relations, including datasheets", every answer carrying the provenance of the facts that produced
it. The IR is a small **declarative datalog** (relations + rules), NOT relational algebra: circuits
are graph-structured (the core queries are transitive closures), and a declarative logical query
says WHAT not HOW, so the `Evaluator` behind the IR is swappable. Datalog subsumes relational
algebra for the tabular side and adds native recursion for the graph side; it also subsumes the
`check.Spec` interpreter (Spec = datalog restricted to one EDB relation + FFIs), so "rules and
search share the evaluator" means the datalog engine is that shared evaluator — a rule becomes a
query on it. An external datalog engine was rejected (JSON impedance flattens provenance and
duplicates the schema, C8; and it breaks the WASM story — the naïve Go interpreter runs client-side,
verified for `GOOS=js`); the declarative IR keeps that swap cheap if scale ever demands it.

The shipped fragment is conjunction + comparison + the built-in `reaches` (bounded transitive
closure, bridged to `Model.Reach`) + **stratified negation** (`not R(...)` — a var only under
negation is an existential wildcard) + **aggregation** (`count/min/max/sum` in the projection,
grouped by the variable columns) + **string predicates** (`contains`/`prefix`/`suffix(?x,"...")`
as built-in filters, positive or negated — plain strings, WASM-clean) + **user-defined rules**
(`head(...) :- body; …; goal`, clauses separated by `;`), which define derived relations and may be
**recursive** — a rule whose body reads its own head is evaluated to a stratified fixpoint, so a
transitive closure like `connected(?a,?c) :- connected(?a,?b), link(?b,?c)` terminates (the fact base
is finite) and a program with recursion through negation is rejected.

An overlay adds its OWN relations through `query.RegisterRelation(name, fields, projector)` (the
open-core seam): the public engine ships the netlist/datasheet/board relations, and a private overlay
registers house part attributes, a compliance database, or an approved-vendor feed as first-class
query relations — the goal joins them, rules read them, negation ranges over them, with no evaluator
change (the evaluator already treats every EDB relation uniformly). A registered relation is a name, a
positional layout over `FactRow`, and a `Projector` that derives its rows from a `Model`, mirroring
the `check.Facts` discipline (a derived projection, never a second authoritative store, C8).

Under the hood every callable — a fact relation (EDB), a rule-derived relation (IDB), and a computed
built-in (`reaches`, the string filters, and overlay-registered predicates) — is one positive
primitive: **`extendAtom`** yields each binding that satisfies an atom. `solve` drives the positive
body through it, and negation reuses the *same* primitive (`atomHolds` asks only whether it yields
anything — negation as failure), so there is a single dispatch, not one per kind, and `not R(...)`
works uniformly for every relation including `reaches`. Overlays add a **pure filter predicate** with
`query.RegisterPredicate(name, arity, holds)` (like the built-in `contains`/`prefix`/`suffix`): it
keeps a binding when `holds` is true and `not name(...)` keeps it when false, both derived from the
one boolean so they cannot disagree. A predicate that could *enumerate* bindings from the Model (a
generator, like `reaches`) is deliberately not registrable — a value-producing generator would break
the finiteness guarantee — so `reaches` stays an internal builtin and a generator seam waits for a
real need.
`agni query <file> [--params dir] '<query>'` prints answers with provenance:

```
$ agni query regulator.fires.kicad_sch --params seed/ \
    'component.mpn(?r,?m), param(?m,"VIN",?vmax), component-on-net(?r,?n), net.max_voltage(?n,?rail), ?vmax < ?rail => ?r, ?m, ?vmax, ?n, ?rail'
r   m       vmax  n     rail  provenance
U1  LM1117  20    +24V  24    …/regulator.fires.kicad_sch ; datasheet "SNOS412Q …" page 4, "7.1 Absolute Maximum Ratings"
```

The Evaluator is tier-general (it joins relation tuples, agnostic to which IR tier produced a
fact): a tier becomes queryable by adding projectors, with no Evaluator change. The **board
tier** does exactly this — `board.track_width(net, mm)`, `board.via_drill(net, mm)`,
`board.layer(net, layer)`, derived per net (the minimum, in mm — not raw geometry) — and then
cross-tier joins work in one cited query:

```
$ agni query board.kicad_pcb --params seed/ \
    'board.track_width(?net,?w), component-on-net(?ref,?net), component.mpn(?ref,?mpn), param(?mpn,"IOUT",?i), ?w < 0.25 => ?net, ?ref, ?i'
```

a net routed thinner than 0.25 mm carrying a part rated for high current — spanning **board ⋈
netlist ⋈ datasheet**, each answer cited to the copper and the datasheet. Manufacturing tiers add
the same way.

## Primitive set (from the rule corpus)

The survey's open question was whether `select / traverse / exists / count` is enough for
Phase 1. Working a real design-rule corpus (naming conventions, differential-pair integrity,
logic-level compatibility, component-value-vs-recommendation, ESD protection, part
orientation, connection-role compatibility, test-point coverage; the concrete set is maintained internally) back to the machinery each rule needs gives the answer: **no, four is
short by five.** The Phase-1 primitives are nine, and all nine stay inside the bounded ceiling
above (Datalog / relational + arithmetic, not Turing-complete).

Core (already named by the survey):
- **select**: filter nets / components / pins / sections by attribute.
- **traverse**: walk net ↔ pin ↔ component ↔ pin.
- **exists / all**: quantify over a selection.
- **count / aggregate**: Tier-A coverage and ratios.

Added (the corpus forces these):
- **pattern predicate**: regex over names; drives naming-convention and differential-pair rules.
- **arithmetic compare**: a value against a rating or threshold (`>=`, `<=`, `within`). The
  workhorse for datasheet-backed rules (cap derating, logic-level margin, Isat, passive value).
  Today's `check/` has none of this.
- **external join**: pull a component's parameter row from the parameter layer (doc 04) and
  compare. This is Tier X made first-class, the differentiated category: a heuristic reviewer
  cannot *prove* "this cap clears the rail with margin," a datasheet-joined rule can.
- **pin-role semantics**: signal direction, power / ground, anode / cathode. Some is present
  (`PinDirection`); the polarity roles need a small IR enrichment (see below).
- **pairing / correlation**: match entities by a key (`_P`/`_N`, expected-pin / actual-pin).
- **reachability / transitive closure**: "reachable through only passives," and similar.
  The survey listed this as open; the corpus confirms it is needed (ESD-path rules).

The nine stay a small, typed target, so the LLM-authoring property below (a bounded grammar is
cheap to synthesize into and cheap to validate) survives the expansion.

**One IR enrichment falls out of this.** Role-based rules that need anode / cathode or an
explicit power / ground role reference pin semantics the IR does not yet carry beyond
`PinDirection`. That is a small, additive IR change, sized separately. Rules that need
only `PinDirection` (unconnected, single-pin, output-drives-output) run without it, so this
does not gate Phase 1.

## Grammar sketch (Phase 2 target)

Illustrative syntax the Phase-1 library grows into. Rules select over the IR, quantify, and
report; severity + message drive the finding.

```
rule single-pin-net (warning):
  for net N where count(N.connections) < 2:
    report N "net has fewer than two connections"

rule i2c-pull-up (error):
  for net N where N.name matches "SDA|SCL":
    require exists pin P in N.connections
      where P.component.part_type in {"R", "Resistor"}
    else report N "I2C net has no pull-up resistor"

rule gnd-test-point (error):
  for net N where N.name == "GND":
    require exists pin P in N.connections
      where P.component.footprint_ref matches "TestPoint"
    else report N "GND has no test point"
```

## LLM-assisted rule authoring

Authoring even a well-designed DSL has a learning curve, and the rules that matter are held
by engineers, not language experts. Natural-language-to-rule translation removes that
barrier: an engineer writes intent in English ("every I2C net needs a pull-up to VCC") and
gets a draft rule in the grammar above.

The safety comes from the division of labor: **the LLM authors the rule; the engine
evaluates the design.** The model is used only for structured translation (NL -> a formal
rule), never as the judge. Its output is a **verifiable artifact**: it must parse, run, and
produce the expected findings on a labeled fixture before a human accepts it. The verdict
stays deterministic (the whole point of this layer); the LLM never enters the evaluation
path. This is the opposite of heuristic "AI review," where a model judges the design
directly and the result cannot be proven.

Two things make this reliable, and both fall out of decisions already made here:
- **The bounded expressiveness ceiling** (Datalog/relational, not general code) is a small,
  typed generation target, far more reliable to synthesize into, and cheap to validate,
  than free-form code.
- **The fixtures** close the loop: NL -> draft rule -> run on a labeled fixture ->
  show findings -> engineer confirms or edits -> commit. The committed rule is code, so it
  is reviewed in the PR (human-in-the-loop) and diffable; LLM assistance strengthens the
  design-as-code posture rather than bypassing it. It is bidirectional too: rule -> NL to
  explain an existing rule during review.

This is an authoring aid layered on the Phase-1 library / Phase-2 DSL, not a prerequisite.
It is noted here because it is a strong argument *for* the declarative DSL surface (it is
what makes the DSL approachable), and because the bounded, verifiable target is exactly what
makes LLM generation safe.

## Starter rule set

Validated by real use, the engineers' `netlist_comparison` workflow already runs these by
hand (captured in the private notes): GND test-point, passive test-point coverage (one-net /
both-net), passive MPN-missing (Tier X), I2C pull-up, test-coverage report. Plus the two
existing `check/` connectivity rules as the seed. These are the Phase-1 library's first rules
and the acceptance set for its primitives.

This starter set has since been broadened into a larger real design-rule corpus (kept internally) that drove the nine-primitive set above. That corpus is the working
acceptance set: each primitive earns its place by being what a real rule in the corpus needs.

## Open questions

- ~~Exact primitive set for Phase 1~~, answered above (nine primitives; reachability is in,
  arithmetic + external-join + pattern + pairing + pin-role are the additions).
- Finding model: severity levels, grouping, and how a rule references *two* revisions for
  diff-gates.
- Where Tier-X external data (approved parts) is sourced and joined.
- Whether Galore is the right DSL front-end when Phase 2 arrives, or a hand-written parser
  suffices (revisit then, not now).
