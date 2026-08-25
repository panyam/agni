# CONSTRAINTS

Enforceable architectural rules for this project. Background and rationale in
[stack and architecture](https://panyam.github.io/agni/architecture/stack/),
[presenter contract](https://panyam.github.io/agni/architecture/web-app/), and
[ingestion and IR](https://panyam.github.io/agni/architecture/ingestion-and-ir/).

## C1: Engine logic in Go, view in TS, core runtime-agnostic
**Rule:** All domain/business logic (parsing, IR, diff, rules, simulation) lives in Go
and is **runtime-agnostic**: no `syscall/js`, no DB handles, no file paths baked in (take
`io.Reader`/bytes). Platform effects and persistence are injected at the edge, so the same
code runs on the server, in WASM, and in CLI/tests. TypeScript never hosts domain logic;
it hosts the view adapter (rendering, input capture, DOM/CSS) and, on high-frequency
surfaces, an in-process presenter (C7). The presenter's *runtime* is per-surface, so it is
not necessarily Go; the domain logic behind it always is.
**Why:** Go-maximal; one source of logic, three runtimes (thesis Principle 1).
**Verify:** no `syscall/js` or DB imports in core packages; readers take `io.Reader`.

## C2: IR defined in protobuf only
**Rule:** The IR is defined in `.proto` and consumed via generated code in Go and TS.
No hand-written or duplicated IR types. This extends to the application/web API (WS9):
its request/response messages and services are defined in `.proto` and served over
**Connect** (connect-go on the server, connect-web in the browser), with no hand-written
JSON DTOs or ad-hoc `net/http` handlers for a proto-defined service.
**Why:** one schema, no drift between Go and TS, free unknown-field retention. The web API
is a cross-runtime contract just like the IR, so it earns the same guarantee.
**Verify:** IR types are imported from generated packages; web API endpoints are the
generated Connect handlers (`*connect` packages), not hand-rolled JSON routes.

## C3: Duplex presenter contract, semantic both ways, camera/picking view-local
**Rule:** The view<->presenter contract is a duplex of **semantic** messages expressed as
**typed view interfaces** (typed method calls, not an event bus and not a shared store):
**intents up** (what the user meant) and **commands down** (what should be true on
screen). The contract is proto services when it crosses a WASM/network boundary, and a
plain typed TS interface when the presenter is in-process. Intents and commands are
DOM-free and framework-free: no event objects, no nodes, no pixels, no framework types.
Picking/hit-testing and camera pan/zoom are **view-local** (the view turns a cursor
position into a semantic intent like `NetHovered(netId)`). Framework reactivity (e.g. a
Solid signal) is confined to the island that implements the interface; the presenter
imports no framework and no DOM. The **semantic command is the primitive**;
HTML-over-the-wire is one renderer on top, not the only shape. The presenter pattern is
**mandatory for interactive surfaces**; its runtime (Go/WASM vs in-process TS) is a
per-surface choice (see C7).
**Why:** decouple what/how to render from how actions are processed; keep the frontend
swappable and dragging smooth (thesis Principles 2-4).
**Verify:** presenter/core packages have no `solid-js` / `react` / `vue` / DOM imports;
contract methods carry domain data or HTML, never event objects, DOM nodes, or framework
types.
**Note:** this duplex presenter contract is a distinct proto surface from the
request/response application API (WS9: WorkspaceService and later DesignService /
DiffService). Both are proto-defined (C2); the presenter duplex carries view<->presenter
intents/commands, while the web API carries browse/load/diff calls.

## C4: Coarse WASM boundary
**Rule:** Cross the JS/WASM boundary O(a few) times per frame with bulk data (typed
arrays). No per-primitive or per-raw-event crossings. Static geometry uploaded once;
only the dynamic overlay crosses per frame. Input batched to requestAnimationFrame.
**Why:** Go/WASM per-call overhead; keep interop well under 1ms per frame.
**Verify:** no `js.Value` calls inside per-primitive loops in hot paths.

## C5: Sanctioned ingestion only
**Rule:** Ingest via open formats, official APIs, or official extractors.
Reverse-engineering a proprietary format requires explicit approval.
**Why:** EULA/DMCA risk plus enterprise-sales blocker (see the ingestion doc's legal-ingress ordering).

## C6: Readers declare a fidelity contract
**Rule:** Each reader declares its fidelity (lossless or lossy-bounded). Lossless
readers must pass the round-trip oracle (parse then emit is identity) on the corpus.
**Why:** honest losslessness; automated validation tames the parser treadmill.

## C7: Heavy ops server-side; presenter runtime is per-surface
**Rule:** Large-file parsing, simulation, and other heavy operations run server-side.
The presenter pattern is mandatory, but its runtime is chosen per surface: WASM (Go
reuse) for lower-frequency surfaces, TS for high-frequency ones (continuous dragging /
live editing) to avoid per-event boundary cost. Core domain logic stays Go and
authoritative regardless of where the presenter runs. A presenter is per-surface in a
second sense: a static or read-only surface has no interactive intent loop and therefore
no presenter, so the server just renders it.
**Why:** avoid shipping a large engine before first paint, and avoid WASM boundary
latency on high-frequency input (thesis cost section; lilbattle Viewer=WASM, Editor=TS).

## C8: Proto is the contract; optimized in-memory forms are derived projections
**Rule:** Proto is the single source of truth for any cross-runtime **contract** (IR and
sidecars like geometry). Performance-oriented in-memory representations (e.g. a columnar
Go struct for bulk geometry) are **derived projections** of that proto, not a second
hand-maintained schema, and must map to/from it at the edges. Bulk data may cross as an
opaque `bytes` blob with an explicit, versioned layout when per-element proto messages
would be too costly (see the geometry doc); the structural/logical tier stays modeled in proto.
**Why:** keep C2's one-schema no-drift guarantee while allowing allocation-light compute
and near-free wire serialization for high-volume geometry. Storage optimization must not
leak into the public API.
**Verify:** no parallel hand-written schema for a proto-defined contract; bytes blobs
carry a `layout_version`.

## C9: Semantic-layer fields earn their place (promotion rule)
**Rule:** A field enters the IR's normalized **semantic** layer only once a **second
format's** reader would populate it. Until then it lives in a node's `attributes` map or
the format-fidelity layer (`FidelityFragment`). Entities with no reader yet are labeled
**provisional** in the proto and are not relied on until verified against real files.

**Carve-out — DERIVED-NORMALIZATION fields (WS3-071).** A field that no reader populates but a
**format-neutral shared pass** derives once at ingestion (over the already-read IR, in
`formats`/`classify`, never per-reader) is admitted directly, because it is format-neutral by
construction rather than by the two-reader test — every format populates it via the same pass. Such a
field MUST: (a) be derived by one shared pass, not by any reader; (b) carry a doc comment marking it
DERIVED-NORMALIZATION and naming the pass; (c) degrade safely when absent (a design built without the
pass — a hand-authored test IR — leaves it empty and the consumer re-derives as a fallback, never
treats empty as a fact). First instance: `Component.device_classes` (the classify pass); second:
`ir.Net.roles` (the WS3-072 naming pass, `classify.StampNetRoles`). This is the **left-shift** rule:
interpret conventions at the edge, carry normalized facts in the core.

**Fill variant (WS3-072).** The same discipline extends to FILLING an EXISTING reader-populated field to
a more-specific value where the reader was UNDER-SPECIFIED — not only to ADDING a new field. A shared
ingestion pass may promote an under-specified value under the same (a)–(c) rules (one shared pass; a doc
comment naming it; degrade-safe — the reader's original value stands if the pass did not run), with one
added guard: it promotes ONLY where the reader was under-specified and NEVER overwrites a confident
value. First instance: `classify.StampPowerInPins` fills `ir.Pin.direction = POWER_IN` on a supply-named
pin whose direction is input/inout/unspecified (a format that cannot type power pins — EDIF — leaves a
VDD pin plain INPUT), so `PinDir == POWER_IN` works format-neutrally; a confident OUTPUT/POWER_OUT is left
untouched. The fill variant normalizes; it does not fabricate — the value it writes is the one the format
could not express, not a guess.

**Evidence-tier variant (agni issue 280).** "ONE shared pass" assumed every input a derived field
needs is available at ingestion. That stopped being true when the datasheet tier arrived: params are
attached at MODEL construction, after the read, so a field derivable from a vendor pin function
cannot be filled by the ingestion pass at all. The rule softens to **one shared pass PER EVIDENCE
TIER**, under two conditions that preserve everything it was protecting:

- **The field records WHICH tier established each value.** `ir.Net.roles` is the first instance: each
  role carries a `RoleSource` (convention / declared, with more to come), so a consumer can weigh a
  value instead of only reading it, and "how do we know this" is answerable at the point of use.
- **A later tier may only ADD, never remove or downgrade what an earlier one established.** This is
  the property that makes extension safe: admitting a new kind of evidence can never cost a value an
  earlier tier would have found, so no tier's absence can silently narrow an answer. It generalizes
  the rule the declared-role union already followed.

Everything else holds unchanged: still format-neutral, still never per-reader, still degrade-safe
when a tier is absent. What is dropped is only the assumption that one pass can see everything.

**Why:** keep the neutral IR from overfitting to whichever format we read most (EDIF
today). The two-layer split (C1, and the ingestion doc) only pays off if the semantic layer stays
format-neutral; this is the gate that keeps it so. Background: the IR-v0 discussion in the ingestion doc and the
cross-format survey it references.
**Verify:** each first-class semantic field is justified by >=2 formats in the
cross-format map OR is a DERIVED-NORMALIZATION field meeting (a)-(c) above;
provisional messages carry a "PROVISIONAL" marker in `ir.proto`. Every new-reader ticket
reconciles its concepts against the map before adding fields (the drift trigger).

## C10: Shippable features ship a runnable example
**Rule:** A user-facing engine capability (a reader, an analysis like diff/checks, a render
surface) ships with a runnable example under `examples/`, following
[examples/CONVENTIONS.md](examples/CONVENTIONS.md): a demokit walkthrough whose narration
lives in a sidecar `walkthrough.md`, reading only bundled synthetic fixtures, in its own Go
module so demokit and its terminal-UI deps stay out of the engine's go.mod.
**Why:** the examples are executable docs and a second consumer of the public API, which
keeps that API honest and gives every feature a legible entry point. Per-example modules
preserve the shippable-engine goal (the engine `go.mod` stays lean, C1).
**Verify:** each capability has an `examples/<name>/` with a `walkthrough.md` and its own
`go.mod`; examples read fixtures from `examples/common/designs/` only (synthetic and
redistributable, never a real board, C5).

## C11: Server-rendered shell, framework islands at the leaves
**Rule:** Web pages are server-rendered (goapplib + templar) and routing is server-owned:
one document per page, no client-side SPA router. Interactive UI mounts as **islands** into
declared holes in that page via tsappkit / tsappkit-solid (`SolidIsland` + `signalView`),
not as a full single-page app. Framework dependencies (e.g. `solid-js`) live only in leaf
island/adapter packages, never in the core, the presenter, or shared TS utilities. The
bespoke WebGL canvas is one such island and stays a semantic-command surface (C3).
**Why:** stay backend-controlled and keep the frontend swappable (the presenter contract,
one level up in the build system) while still using modern reactivity where it earns its
place. Server routing plus per-page islands, not an SPA (thesis Principles 3, 5, 7; the
goapplib presenter-contract reference).
**Corollary — the composition root is the fourth edit, and it is tested.** An island is only real
once `main.ts` constructs it and passes its view to the presenter. Because the `ViewSink` ports are
optional (so an embedding host may leave a panel out, C13 / `build/overlay.md`), an unwired port is a
silent no-op rather than a type error, and no panel-level test can see it: every one of them supplies
its own collaborators. That gap shipped a broken feature twice, once with a client never passed
(WS9-052) and once with a view never wired (agni issue 175). The rule is that the four registration
points move together: island, template hole, `ViewSink` field, `main.ts` wiring.

**Verify:** no client-side router; `solid-js` appears only in island/adapter package
dependencies; each interactive surface is an island mounted into a server-rendered hole;
`cd web && pnpm exec vitest run src/composition.test.ts src/browse.test.ts src/datasheets.test.ts`
boots each page's real entry point against its real template and fails on a hole that nothing mounts,
a `ViewSink` port nothing wires, or a client the presenter never receives. **Every page has a
composition root, so every page needs one of these**: the viewer's test was the only one for a while,
and in that time the browse page's root went untested and the workbench's shipped a deep link that
recorded nothing (agni issues 136 and 318).

## C12: Render style is injectable data, not scattered literals
**Rule:** Colors and the default font are view **policy expressed as data**: a single
`render.Style` (with `DefaultStyle`) is the one source of truth, and both the SVG backend and
the WebGL text-label path resolve from it (via `WithStyle` options), so the two renderers
agree by construction rather than by copied hex strings. No bare color/font literals in the
render layer; a caller overrides per render call. Colors the server decides travel on the
wire (`PackedLabel.color`, `PackedSheet.font_family`) so the client renders without a second
palette. Per-element font/style overrides wait until the geom IR carries them.
**Why:** kill the duplication that let the SVG and label colors drift (`#555` vs `#555555`),
and keep styling overridable (theming, dark mode, accessibility) without editing the engine.
**Verify:** no `"#rrggbb"` or `font-family` literals in `core/render/*.go` outside `style.go`;
`SheetSVG`/`PackSheet` take render options; label colors come from `Style`.

## C13: Service impls are importable, transport-neutral, and take I/O via injected ports
**Rule:** The service implementations (`WorkspaceService`, `DesignService`, `CheckService`,
`DiffService`, and any future service such as the WS10 parameter service) live in an
importable package (`internal/service/`), never in `package main`, and are
**transport-neutral**: every method carries a plain protobuf signature
(`(ctx, *pb.XRequest) (*pb.XResponse, error)`) and classifies its errors with the package's
sentinels — no `connectrpc`/gRPC/transport imports. Transports are thin adapters over them:
Connect today (`internal/server`, wrap/unwrap plus one sentinel-to-code table), grpc-gateway
or a real gRPC server later as siblings. The services take their I/O concerns as **injected
ports** — a filesystem/opener interface for reading mounted designs and resolving secondary
files (KiCad sibling schematics, xschem/gEDA `--symbol-path`), and later a persistence port
for the datasheet/parameter store — never `os`/`syscall/js` directly. `cmd/agni` (and any
other entrypoint: a WASM build, a cloud function) is thin wiring that constructs the platform
adapter and hands the services to a transport. Protos split **per service concern**
(`workspace.proto`, `design.proto`, `checks.proto`, `diff.proto`, ...), never a per-transport
bucket file. This is C1 applied to the application tier: the service orchestration is
runtime-agnostic; only the adapters (I/O and transport) are per-runtime.
**Why:** the services are reusable orchestration, not domain core. They were first grown onto
`agni serve` fused to the OS filesystem, then to the Connect generics, so the same code could
not run in WASM, be served over gRPC, or be reused from a second entrypoint. C1 wants "the
same code runs on the server, in WASM, and in CLI/tests"; that must hold for the service tier
too (the lilbattle services.go shape: gRPC-style impls, transports as translation layers).
**Verify:** no `os.`, `syscall/js`, or `connectrpc.com` imports in `internal/service/` impl
files (`internal/service/transport_guard_test.go` runs the transport check in CI); service
constructors take ports; `cmd/agni` builds the OS-backed adapters and registers
`internal/server` wrappers via the generated Connect handlers (C2); `protos/agni/v1/webapi/`
holds one file per service.

## C14: Rule classification is open tags, not typed fields
**Rule:** A `check.Rule`'s typed fields are only what the engine acts on — `Name`, `Severity`,
`Reads` (its fact dependencies), `Eval` (which MAPS every subject to a verdict), and
`StatesConsideredSet` (whether those verdicts are the full considered set or only the failures),
plus the prose (`Summary`/`Impact`/`Remedy`/`Detail`). Every
classificatory axis — category, tier, distribution, and any provider-defined one — lives in an open
`Tags map[string]string`, never as a typed struct field. Availability derives from `Reads` (a rule
that reads a fact whose provider layer is absent is unavailable), not a stored track/label field.
Consumers take the rule catalog as an injected `[]*check.Rule`, not the `check.Rules` global.
**Why:** the set of classification axes is open and provider-specific — rules will come from a
Phase-2 DSL and from integrators embedding Agni (customer suites outside `core/check`), so a closed
column schema would force a core change per axis and force external authors to populate
Agni-internal fields. Tags keep the catalog extensible (a browsable UI groups/filters by whatever
keys are present) and keep the rule model from overfitting to today's axes — the earn-its-place
discipline of C9 applied to the rule catalog. (This reversed a first cut that typed `Track` and
`Distribution` as fields.)
**Verify:** the `check.Rule` struct carries a `Tags map[string]string` and no per-axis
classification fields beyond the behavioral core; `check.Available` reads `r.Reads`, not a track
field; `NewDesignService` takes a `[]*check.Rule` parameter.

## C15: Readers never import the presentation tier
**Rule:** Format readers (all under `readers/`: `edif`, `kicad`, `ipc2581`, `xschem`, `geda`, and
any future reader) must not import `core/render/` or `core/svg/`. Geometry math a reader and a renderer both need
(placement transforms, pin world positions) lives in `internal/geomath`, imported by both
sides. Dependencies point one way: readers produce IR/geom; the presentation tier consumes it.
**Why:** "pins land where symbols are drawn" must hold by shared code, not by a reader
reaching up into the renderer for its helper (which couples ingestion to presentation and
drags drawing code into every entrypoint that only wants netlists). One implementation of the
transform contract (the geometry doc) serves producers and consumers alike.
**Verify:** `grep -rl '"github.com/panyam/agni/core/render"\|"github.com/panyam/agni/core/svg"'
readers/` returns nothing.

## C16: Internal-seed posture (datasheet data never leaves the customer boundary)
**Rule:** Datasheet documents, doc-IRs derived from them, and extracted parameter data
(PartSpecs, run manifests) are customer-boundary data: they are never committed to this
repo, never redistributed, and never assumed to exist outside one deployment
(`SourceDoc.locator` is corpus-local by design). The shippable artifacts of the
datasheet layer are the engine, the schemas (param/doc/derive protos), the rules, and
recipes — recipes carry vendor LAYOUT knowledge (which table headings mean what), never
extracted values. Stores for the customer-boundary artifacts sit behind injected ports
(C13). Committed fixtures are hand-authored or synthetic; hand-transcribed real values
in test fixtures cite their source (facts are not copyrightable; documents are).
**Why:** vendors license and watermark datasheets, so a redistributed spec DB is a
liability; the internal-seed / bring-your-own-corpus posture dissolves the licensing
problem and makes the customer's accumulated seed + patches their compounding asset
(the WS10-002 posture on component data).
**Verify:** no PDFs and no derived PartSpec/manifest corpora under version control here
(fixtures under `*/testdata/` excepted); `param`/`doc`/`derive` packages take `fs.FS`
or readers, never fetch.

## C17: Layered dependencies — the contract and reader tiers depend downward only
**Rule:** The dependency graph is layered so the low tiers can be consumed (and one day carved
into their own modules) without dragging the application tail. The generated contract
(`gen/`: IR + geom + param/doc protos) imports no first-party `agni` package. Format readers
(under `readers/`: `edif`, `kicad`, `ipc2581`, `xschem`, `geda`) and the reader registry
(`readers/formats`) depend
only downward — on the contract and shared parse/geom helpers — never on the application tiers
(`internal/service/`, `internal/server/`, the web transport, `servicekit`, `connectrpc`).
`readers/formats` is public (not `internal/`) precisely so an out-of-module reader registers through it
(WS12-003); that is the ONE reader extension seam. This subsumes C15 (readers ⊅ `render`/`svg`)
and generalizes it to the whole heavy tail.
**Why:** the open-core overlay, and any future ecosystem reader, depends on the contract plus the
registry — not on the web/serve tier. Go module-graph pruning already keeps that dependency light
*because* the layering holds; a stray import from a reader up into `internal/server` would pull
servicekit/connect into every consumer and foreclose extracting the reader tier as a module. Keep
the seam clean now so the split stays a rename, not a refactor.
**Verify:** `go list -deps ./readers/... | grep -E
'servicekit|connectrpc|panyam/agni/(core/render|core/svg|serve|internal/service|internal/server)'`
returns nothing; and `go list -deps ./gen/... | grep 'panyam/agni/' | grep -v '/gen/'` returns
nothing (the contract imports no first-party package).

## C18: The public engine never imports the overlay (dependencies point overlay → engine)
**Rule:** The open-core structure is a public Apache-2.0 engine and a private *overlay* that
depends on it (Go `require github.com/panyam/agni`) to add proprietary-format readers,
house-style/private rules, and private design data. Dependencies point **overlay → engine
only**: no engine package may import an overlay, and the engine `go.mod` requires no overlay
module. An overlay contributes exclusively through the public extension seams — `formats.Register`
(readers, WS12-003) and `check.RegisterSource` (rules, WS12-004) — never by the engine reaching
into it. The reference overlay lives at `examples/overlay/` (its own module, `replace => ../..`);
a real overlay is a separate private repo (the open-core doc).
**Why:** the split only holds if the arrow points one way. An engine that imported an overlay
would drag private/customer code into the shareable, open-source repo — the whole reason the
overlay exists (the C16 datasheet posture generalized to all of readers, rules, and data). It is
also what lets the engine be published while overlays stay closed. The seams are global registries
the overlay writes into at init/main, so the engine is composed *by* the overlay, never coupled to
one.
**Verify:** `go list -deps ./... 2>/dev/null | grep 'panyam/agni/examples/'` run from the engine
module returns nothing (no engine package imports the reference overlay or any example), and the
engine `go.mod` has no `require`/`replace` for an overlay module.

## C19: Engine processing reads the design through check.Model, not raw ir.Design
**Rule:** Engine/processing code reads a loaded design through the composed `check.Model` (a proto
projection with indexes and member-method reads), not by taking a raw `*ir.Design` and scanning its
slices. The target is a *helper handed the whole design to scan*, not an *analysis that takes designs
as its input*. A raw `*ir.Design` (or `*ir.Net`/`*ir.Component`) parameter is allowed in three
categories: (1) **producing** the IR — the readers (under `readers/`: `edif`, `kicad`, `ipc2581`, `xschem`,
`geda`), the `readers/formats` loader, `internal/netgraph` (IR emission); (2) **constructing** the Model or
**loading** the design — `check`'s `NewModel`/`NewModelWithBoard`/`NewModelWithParams`/`RunDesign`,
and the `cmd/agni`/`internal/service` loaders that read a file and build the Model; (3) a
**top-level analysis/transform that takes designs as its input and uses no Model index** — `diff`
(compares two designs, builds its own by-key match maps), `validate`, and `graph` (netlist→layout).
These consumer packages (`diff`, `validate`, `graph`) and the producers are excluded from the
`make ir-model-check` scan wholesale; `examples/` too (demos). Everywhere else — a helper in `check`,
`internal/service`, `cmd` handed a design to read — goes through `model.Model`; a read the Model lacks
is added as an indexed member method (the `HasComponent`/`IsPowerRail`/`SourceFormat` precedent),
never re-scanned inline.
**Why:** `ir.Design` is an index-less message, so every helper that scans it re-walks O(n) per call;
the Model builds the indexes once and hands out O(1) reads, and it is the single place a hot read
gets optimized without touching call sites. It is also a readability contract — a `Model` parameter
marks processing code, a `*ir.Design` parameter marks the I/O boundary. Enforced **incrementally** (a
ratchet, `make ir-model-check`): the rule binds new code immediately; the existing raw-`*ir.Design`
sites are grandfathered in `hack/ir_model_baseline.txt` and migrate opportunistically, each removal
ratcheting the baseline down — never a big-bang rewrite. The read-surface contract itself lives in
package `model` (WS1-043): the `Model` interface and its value types, importing only the generated
`ir`/`geom`/`param` protos, so a consumer depends on the contract, not the `check` implementation
(rules + `param` logic + `irModel`); `check` implements it and re-exports the names as aliases. The
genuine helper smells have been migrated (`LocateReason`, the `internal/service` sheet-annotate
helpers, `check.Available`); the baseline that remains is the sanctioned construction/loading sites
(`NewModel*`, `readDesign`, the loader) plus a CLI render helper (`compareLayouts`), which the
ratchet holds flat.
**Verify:** `make ir-model-check` returns clean — no `func … *ir.Design …` parameter outside the
allowed paths that is not already grandfathered in `hack/ir_model_baseline.txt`; a new one fails the
gate (which `make testall` runs).

## C20: Convention interpretation is left-shifted to ingestion; the check path reads normalized facts
**Rule:** Convention-specific interpretation — net names to roles (rail/ground/feedback), part text to
a class, house naming to meaning — happens once at ingestion/normalization and is stored as a
normalized IR fact (`net.role`, `device_classes`). The check path (rules, the query engine) reads those
facts and MUST NOT re-run convention matching per-entity-per-rule. Convention VOCABULARIES are config
with built-in defaults (the naming lexicon WS3-069, the class lexicon WS3-070); they are APPLIED at the
edge, not consulted in the hot path. Structural evidence (a power pin, a resistor's presence) is
preferred over names where available; the name/text lexicon is the fallback for what a directionless
netlist carries.
**Why:** the catalog runs many rules over many entities, so re-deriving "is this a rail / a TVS" in the
check path is O(rules x entities) of the same string parsing, and it couples the core to vendor
conventions. Interpreting once at the boundary keeps the core convention-agnostic (it reads facts),
makes the expensive path cheap, and puts house-style config in the overlay/edge (the C16/C18 posture
generalized). Interim in-check heuristics (WS3-065's attribute reading, WS3-069's per-rule lexicon
calls) are stepping stones; the left-shift tickets (WS3-071 `device_classes` at ingestion, WS3-072
`net.role` at ingestion) move them to the edge. A `Verify` (a grep that rule `Eval`s do not call the
name/class heuristics directly) becomes checkable once those land.

## C21: The netlist is the source of truth for connectivity and component IDENTITY; schematic/board files are geometry companions
**Rule:** Component IDENTITY and CONNECTIVITY (which components exist, their ref-des, the nets, and
the rules and findings computed over them) is sourced from the NETLIST the design team produces (for
OrCAD/Allegro that is the `.edn` netlist view; for KiCad the schematic/PCB, which carry the netlist
inline). A schematic-view or board file supplied ALONGSIDE it
(an OrCAD `.eds`, a `.kicad_pcb`, an IPC-2581 board) is a GEOMETRY companion — a canvas for rendering
and for locating/highlighting findings — and MUST NOT be treated as a second, independent COMPONENT
source to be reconciled or merged against the netlist. Findings computed on the netlist join to a
companion's geometry by NET NAME (primary, drift-resistant) and ref-des (secondary, degrading to a
"couldn't locate" note), never by inventing identity the companion lacks. A netlist SYNTHESIZED from an
`.eds` (when no real netlist exists) is a degraded, explicitly-labeled fallback with tool-synthesized
ref-des — net-level findings authoritative, component-identity cross-reference not — and is never
promoted to the source of truth.

Part ATTRIBUTES are a separate question and this rule does not answer it. MPN, manufacturer,
quantity as ordered, do-not-populate status and approved alternates may be sourced from a declared
BOM (`ir.BomLine`, keyed by ref-des), because the netlist frequently does not carry them and is not
authoritative for them where it does. That is an ATTRIBUTE JOIN onto components the netlist already
established: it may enrich a component, and it may be RECONCILED against the netlist and report the
disagreement as a finding, but it MUST NOT add, remove, or rename one. The component set stays the
netlist's.
**Why:** an exported schematic is a lossy, point-in-time SNAPSHOT, not the design's system of record.
Reference designators (`C1`, `R3005`) are the OUTPUT of a stateful authoring process — annotation +
layout back-annotation performed inside the native project, with the tool's netlister assigning numbers
as it emits the netlist — so a schematic export can carry placeholder (`C?`) or per-sheet-duplicate
designators for parts the netlist numbers uniquely (observed: a real EDIF `.eds` export ≈ 50% un-back-annotated
vs a flat, fully-numbered `.edn`, WS1-046). The identity simply is not in the file, and no reader can
recover it — like regenerating a database's auto-increment keys from a keyless dump. Treating the
netlist as truth and the schematic/board as a companion to highlight (WS1-047) makes the join
drift-resistant (net names are stable across annotation), keeps analysis off an assumption we cannot
verify (that two exports are from the same instant), and matches the real workflow: the team gets the
netlist by one click on the project, we render their drawing beside it.

The identity/attribute split is where this rule was previously over-broad. The argument above is
about IDENTITY, and it is strong: a ref-des is the output of a stateful annotation process, so a
file that did not run that process cannot carry it. None of that reasoning transfers to what a part
IS. A netlist does not know do-not-populate status, an approved alternate, or the quantity actually
ordered, and those live in a bill of materials maintained alongside the design. Reading the rule as
"the netlist is authoritative for part attributes too" would put analysis on the least authoritative
copy of a part number in the building, which is the opposite of what the rule is defending. Note
that the `.eds` measurement cited above was recorded outside this repository, so a reader here
cannot reproduce it. The annotation argument stands without it.
**Verify:** analysis/rule code sources components + nets from the loaded netlist model, not from a
geometry reader; a geometry companion contributes only render + locate. A BOM source contributes
only attributes onto an existing ref-des, so the component COUNT of a design is unchanged by
declaring one. (Checkable once the
companion-file association lands, WS1-047: geometry-only readers feed no component/net into the rule
model.)

## C22: Configuration travels as a value, never as ambient state or a locator the callee resolves
**Rule:** Configuration that changes what a run CHECKS or how it INTERPRETS a design — naming
conventions and their vocabularies, interface profiles, design intent, a review manifest, house policy
thresholds — is passed to the code that uses it as a VALUE, along the same call it configures. Two
things it must not be. It must not be **ambient process state**: a package-level vocabulary that a
caller installs before invoking (`SetActiveRoleVocab` and friends) may exist only as a startup DEFAULT,
never mutated per run, because ambient state cannot be scoped to one request and one caller's config
then reaches another caller's work. It must not be a **locator the callee resolves**: a wire request
carries the config as a message (`OverlayConfig.conventions` is a `NamingConvention`, not a
`conventions_path`), so `internal/service` composes it with NO file I/O and how it was obtained — a
YAML file the CLI read, a form a browser filled, a registry a deployment queried — stays the caller's
business.

**Amended (agni issue 224): a config tier that is a CORPUS travels as a ref, resolved through an
injected port, and a host that cannot resolve one refuses.** The value rule above holds for config
that is small enough to inline — a naming convention is a message, never a `conventions_path`, and
that is what lets a filesystem-free host honour one. It never held for interface profiles, seeded
parameters, or a design's intent, which are DIRECTORIES of many files: a project has always named
those as URIs that `ProjectConfigLoader` read, so "the service composes with no file I/O" described
the request tier only, and the schema froze that asymmetry into a request being able to carry one
config tier out of five.

`AnalysisConfig` is one shape for all of them, carried by a request and declared by a project alike,
and `ConfigResolver` is the one port that resolves the ref-shaped tiers of either. The no-I/O property
becomes a property of the DEPLOYMENT rather than of the schema: a host wired with no resolver still
composes a value-shaped config with no file access, and REFUSES a config naming a directory rather
than silently dropping the tier. Refusing is the load-bearing half. A dropped tier reports a clean run
against config that never loaded, which is the silent-pass failure this whole layer exists to prevent,
and it is the same posture `GetNamingConvention` already took for a host that cannot resolve a stored
convention.

What is still forbidden is unchanged: ambient process state, and a config tier whose only form is a
locator when a value would do.

ARTIFACTS are the deliberate exception: a design, a board export, and any large parsed input are named
by an opaque URI (`mount://<mount>/<path>`) that the injected Loader port resolves (C13), because they
are megabytes, need format-reader dispatch, and are re-requested across many RPCs. The authority is a
key in a server-defined namespace and the URI is not a host path; nothing above the Loader may treat
it as a filesystem path.

The URI replaced a `(mount, path)` PAIR (agni issue 177). The Loader was indifferent to which form it
got, but nothing above it was: two fields that mean one thing must travel together, and 24 request
messages repeated that pairing by hand. Two properties came with the collapse. **Parsing IS the
containment check**, so a parsed `artifact.URI` cannot name a location outside the mount it claims and
the 26 call sites that re-checked stopped needing to — an adapter can no longer forget, because it
cannot receive a bad value. And **relative resolution became specified rather than hand-rolled**: a
schematic naming its sub-sheets or a symbol library resolves against the file being read, and a URI
path is always slash-separated, so the `path`-vs-`filepath` split that `formats.Loader` warned was
"invisible on unix, breaks every sibling lookup on Windows" has nowhere to hide.

The authority is always `mount`, never `s3://` or `db://`. What a mount resolves to is the
deployment's business, and a per-store scheme would put the storage kind in the client's hands, which
is the indirection the Loader and ProjectStore ports exist to keep.

RESOURCE NAMES are a different system and are NOT URIs. `projects/{project}` and `reviews/{review}`
are AIP paths naming IDENTITY, not location: a project keeps its name when its folder is renamed or
moved between mounts, which is the whole reason its id is declared rather than derived. Addressing
answers "where are the bytes"; a resource name answers "what is this thing". Collapsing them would
give up that stability (C23).

**Why:** both failure modes were shipped and both cost real time. `agni review` could not load a
`--conventions` config at all (WS3-102), so any review item bound to a naming rule read `not-automated`
forever with no error to explain it, and the lexicon that teaches the engine a project's rail names
could not reach a review, so unrelated rules reported failures that config would have fixed. Making the
config per-request was blocked by the vocabulary being a process global (WS3-106): on serve, one
request's conventions would have reached another request's design read, silently producing wrong
`net.roles`. Passing a PATH instead was the second wrong answer: it forces the service to own file I/O
to do its job, contradicts C13's os-free posture, and bakes a deployment's filesystem into the API
contract — a host with no filesystem (WASM, an embedder, a test) then cannot call it. Carrying the
value instead DELETED a loader interface and seven methods. Values also compose: one
`service.ComposeOverlay` is pure, so CLI, serve, and web cannot drift, and a test needs no filesystem.

Corollary, from C20: a convention VOCABULARY is applied at the READ as a value carried on the loader
(`formats.Loader.Lexicon`), which is what makes "applied at the edge" scopeable rather than global.

A config value a client does not already hold is obtained through its OWN rpc, never resolved inside
the rpc that consumes it (WS9-050). `ReviewService.GetReviewManifest` turns a stored checklist into a
`ReviewManifest`, and `CreateReview` takes only the value. That split is what keeps the consuming call
free of I/O while still serving a browser, which holds a ref and no filesystem: the read happens, but
it is named in the contract instead of hiding inside a run, and a caller that already has the value
never triggers it.

**Verify:** no `mount` + `path` PAIR and no `*_path` or `*_ref` field in `protos/agni/v1/webapi/` — an
artifact is named by a single `uri` (or `*_uri` where a message names more than one), and a config
travels as a value; no `os`/`path/filepath`/`io/fs` import in `internal/service/`
impl files (`transport_guard_test.go`, `TestNoFilesystemImports`); the vocabulary installers
(`naming.ApplyLexicon`, `classify.SetActive*`) are called only from entrypoint startup wiring
(`cmd/agni`), never from a service method or any per-run path.


## C23: Stateful resources are AIP-shaped; stateless compute stays verb-shaped
**Rule:** An API surface that owns state with a lifetime of its own is modelled as a RESOURCE:
a `name`, standard `Create` / `Get` / `List` / `Delete` methods, AIP-160
`filter` and AIP-158 `page_size` / `page_token` on the list. Everything else in
`protos/agni/v1/webapi/` stays verb-shaped (`GetDesign`, `ListDir`, `RunQuery`, `DiffDesigns`,
`CheckDesign`), addressed by an artifact URI (C22). The resources today are `ReviewService`'s
`Review` (WS9-053) and `ProjectService`'s `Project` / `Design` (agni issue 170). A new rpc picks its
side by ONE question: does what it returns still exist after the call? A derived view of files on
disk does not, and is verb-shaped. Persisted state does, and is a resource.

**Third case — a DECLARED-identity resource is AIP-shaped and READ-ONLY.** A resource whose id is
declared by an OPERATOR (a `name:` an operator writes in a descriptor) rather than assigned by the
server or derived from a path is AIP-shaped with `Get` and `List` and NO mutators. This is a real
third case rather than a loosening of the first two, and the two-question test that separates it is:
is the identity the caller's to invent (verb-shaped, the arguments ARE the input), the server's to
assign (a full resource), or the operator's to declare (this case)?

A `Project` is derived from files, which by the question above sounds verb-shaped. It is not, for
two reasons that `GetDesign(mount, path)` cannot claim. Its id is DECLARED, so it is not a cache key
over its arguments: the project keeps its identity when its folder is renamed or moved between
mounts, and a `(mount, ref)` pair does not. And it is a PARENT — reviews nest under
`projects/{p}/reviews/{r}` — which a thing with no name cannot be. Mutators are absent because
creation is genuinely out of scope rather than pending: scaffolding a project means authoring design
intent, a judgment step with a confidentiality boundary, not a server operation.

The read-only carve-out is narrow on purpose. It does NOT license a stateless query to wear a
`name`; that failure is called out below and is unchanged. It licenses exactly the case where an
identity exists in the world, was written down by a person, and the server only reads it.

**Why:** two conventions in one API is a cost, so it is worth being explicit that this is
deliberate rather than drift. Every verb-shaped rpc here is a pure function of files: `GetDesign`
takes a mount and a path, and those two arguments ARE its whole input, so a resource name would be
ceremony over a cache key. A review run is different in kind. It is produced rather than read, it
outlives the request that made it, two runs over the same design at different times are different
things a team wants to compare, and none of that is expressible by naming its inputs. Retrofitting
the whole API to AIP would be a wire break across every service and every web client in exchange
for consistency rather than capability. Reaching for a bespoke shape on the one surface that
genuinely has resources would mean inventing pagination and filtering semantics that already have
a well-specified answer.

The boundary also has to be defended in the other direction: a stateless rpc dressed as a resource
is worse than either convention. It invites clients to hold a `name` that is really a query, cache
what was never stable, and eventually ask why deleting it does not work.

**Verify:** every message in `protos/agni/v1/webapi/` carrying a `name` field is served by the
standard methods and no bespoke mutator — the four of them for a server-assigned id, `Get` and
`List` alone for a declared one; every rpc that reads a design and returns a derived view takes
`(mount, ref)` and returns no `name`; a resource's `List` carries `page_size`, `page_token`, and
`filter`, and its service rejects a filter it does not implement rather than ignoring it
(`service.parseReviewFilter`, `service.parseProjectFilter`).

**Known limitation:** stored reviews are visible to every client of a server, because `agni serve`
has no authentication at all. That is a deployment assumption (one team, one trusted network), not
an access-control boundary, and it is recorded rather than implied so nobody reads the resource
model as having brought isolation with it. Auth is deliberately deferred; see `OUT_OF_SCOPE.md`.

## C24: A datasheet parameter is compared in SI base units, converted in one place
**Rule:** Any code that COMPARES a seeded datasheet parameter's value against anything reads the
row through `param.InBaseUnit` and gates on the CONVERTED row's unit. No package outside
`datasheet/param` reads a raw `Parameter.Unit`, and no rule or extractor contains a scale factor.
A unit the conversion table does not recognize is skipped, never scaled by a guess or assumed to
be the base unit. Storage is unaffected: a `PartSpec` keeps every row exactly as the datasheet
printed it, and only the value handed to a comparison is reduced.

**Why:** this is the C20 left-shift rule applied to units, and it is here because the alternative
already shipped a wrong answer. Before agni issue 148 each extractor gated on the printed unit
string, so a row in a prefixed unit was DROPPED from the slice it returned. A rule that receives an
empty list compares nothing, reports nothing, and the runner scores the item a **pass**. Neither
existing guard could catch it: `check.Available` saw a params tier attached, and the `needs-data`
gate saw the symbol seeded. Milliamps are the ordinary spelling for a sub-amp regulator and
millivolts for a controller's sense threshold, so a spec transcribed as printed hit this without
doing anything unusual, and five rule families silently passed designs with genuine defects.

The constraint is about LOCATION rather than caution. The original instinct, that a silent scale
factor inside a pass/fail rule is where a unit bug hides, was correct; refusing to convert did not
remove that risk, it relocated it from "wrong number" to "no check at all", which is the failure
this codebase treats as most serious. One table beside `UnderSpecified` and `MachineComparable` is
auditable in a way ten hand-written unit gates were not.

Returning a CONVERTED ROW rather than exposing a "convert this for me" accessor is what makes the
rule structural instead of advisory: membership in an extractor's result set IS the guarantee that
the number is in base units, so a consumer cannot forget to convert. An accessor would relocate the
same bug to whichever call site forgot it, silently.

**Verify:** `grep -rnE '\bp\.(Unit|GetUnit\(\))' --include='*.go' core/ stdlib/ readers/ internal/
cmd/ | grep -v '_test.go'` returns nothing. The naive sweep for `.Unit != "` does NOT work and must
not be substituted: the extractors legitimately compare `q.Unit` on the converted row, so the
invariant that actually discriminates is that the RAW row's unit is never read outside
`datasheet/param`. Also `TestUnitVocabulariesAgree` (core/check) holds the parameter layer's base
spellings to `core/classify`'s, which is the drift that would break cross-tier comparison.

**This covers the query surface too.** The `param(...)` and `param.range(...)` datalog relations
project their numbers through the same conversion, so a datalog-authored rule compares base units
without knowing it (agni issue 165). `param.unit(mpn, symbol, unit)` carries the printed spelling
separately, because a `FactRow` has no unit column and adding one would be advisory: a rule could
ignore it and compare raw numbers, which is the failure this constraint exists to prevent.

A row whose unit has no known scale keeps its symbol, kind, conditions and citation and loses only
its NUMBER, so a relation that answers "what does this part specify" never shortens its list
silently.

That is safe because ORDERING REFUSES TO MIX AN ABSENT NUMBER WITH A PRESENT ONE (`evalCompare`).
Absence is not otherwise representable in a bound value: `query.fieldValue` yields an empty `Value`
for a nil `Num`, and ordering used to fall back to string comparison, where `"" < "5.0"` is true and
`"" <= "-2"` is also true, since the empty string precedes everything. The answer depended on the
author's phrasing and on the sign of the constant. This is not a datasheet-tier concern:
`param.range` emits a one-sided row for any ordinary max-only datasheet limit, so the same guard is
what makes partial ranges safe at all. Equality is untouched (asking whether two values are the same
is meaningful across kinds) and so is ordering two non-numbers. AGGREGATION was never exposed:
`reduce` skips a nil `Num` rather than falling back.

Since `query.Value` carries `Absent` and `BaseUnit`, the guard is REPRESENTED rather than inferred
from a nil pointer, `absent(?x)` selects the rows with no number, and an ordering comparison across
unlike dimensions (volts against amps) refuses as well. Bare literals stay dimension-polymorphic,
since a query constant cannot state a unit. `Value.BaseUnit` holds an SI BASE symbol only, never a
prefixed spelling: scale is normalized once upstream by `param.InBaseUnit`, so the query layer checks
DIMENSION and never converts SCALE. `TestRelationBaseUnitsAreCanonical` enforces that on every
relation, including ones added later.

**Known limitation:** equality is deliberately NOT dimension-checked, because the same values also
unify implicitly when a variable repeats across atoms and unification is identity rather than
physics. And `absent = absent` is TRUE here rather than SQL's UNKNOWN, since full three-valued logic
would have to thread UNKNOWN through negation, aggregation and the index.

## C25: A run's recorded provenance is derived from the resolved overlay, never from the caller's flags
**Rule:** The `RunConfig` a results document records — which datasheet corpus, interface profiles,
design intent and naming convention a run had attached — is computed from the RESOLVED
`service.Overlay`, through `Overlay.Provenance` and `service.RunConfigProto`. No other code
constructs a `checkspb.RunConfig`. A surface must not derive it from its own flags, its startup
config, or the request message, because none of those is what the run used once a design resolves to
a project.
**Why:** the alternative already shipped a wrong document, and it was wrong in the reassuring
direction. `agni check designs/gateway --results-out` inside a project declaring `conventions.yaml`,
`profiles/` and `params/` composed all three and recorded `"run": {}`, because no flag named any of
them. `RunConfig` exists precisely so a reader can tell a design with no datasheet violations from a
run that had no datasheet corpus, so recording `false` for an attached corpus makes a clean report
read as better founded than it is. Nothing in the document contradicts it except the `catalog`
snapshot, which nobody cross-reads. This is C22's value-not-locator rule applied to the RECORD of a
run rather than to its inputs, and it is a single-writer constraint for the same reason
`service.FindingProto` is: two places that build one message agree until the day they do not.
**Verify:** `grep -rn 'checkspb.RunConfig{' --include='*.go' . | grep -v _test.go` returns only
`internal/service/projectoverlay.go`. Test files are excluded because a fixture legitimately builds a
document to render (`core/results/results_test.go`); the rule is about who WRITES a run's record. Quote
the `--include` glob, or zsh expands it and grep never sees the flag.
**Note:** which tier a rule source came from is NOT recoverable after composition — a compiled
interface profile and a compiled intent declaration are both just rules in a catalog — so the flags
travel on `service.ProjectConfig` rather than being derived from `Overlay.Sources`. Rationale in
[the checks contract](https://panyam.github.io/agni/architecture/checks-contract/).

**Second instance, outside `RunConfig` (agni issue 489).** A verdict link took its PATH from the
caller's argument and its HASH from the resolved design, which is the same "provenance from the flag
rather than from the resolution" shape one layer down. It cost the same kind of wrong-but-reassuring
artifact: a link built from a declared companion carried the entry's hash, so the viewer reported a
revision mismatch on a design nobody had edited. `verdictLinkTarget` now resolves once and returns
both halves, which is the cheaper enforcement than a rule, since a disagreeing pair stops being
representable. Reach for the same move whenever a value and the provenance qualifying it are computed
by two calls: make it one call returning both.

## C26: One schema per contract; a hand-written twin carries a round-trip guard
**Rule:** A contract with both a YAML/authoring form and a wire form has ONE schema, the `.proto`,
and YAML is authoring SYNTAX rather than a second schema (parse it by converting to JSON and binding
with `protojson`, which also gives strict unknown-field rejection for free). Where a hand-written Go
twin genuinely must exist — a domain type that carries behaviour, an AST, a struct whose zero values
mean something a message cannot express — the twin and its converter carry a **deep-equality
round-trip test**: build a fixture with EVERY field set to a distinguishable non-zero value, go
domain -> proto -> domain, and require `reflect.DeepEqual`. A tier with no wire form at all (design
intent today) has no twin and owes neither.
**Why:** two hand-maintained schemas for one contract drift, and they drift SILENTLY, because a field
the converter never learned is absent from both sides of any assertion made on the proto. This has
now shipped twice. `naming.Lexicon` grew gate/source/drain terminal vocabularies with no wire fields,
so a project declaring them had them dropped on every path except `serve`'s startup install and
`BuildRoleVocab` substituted the built-in names. `Profile.HostClass` (WS3-044) was added with no wire
field, so an overlay profile binding its host by datasheet device class lost the binding crossing
`stdlib/ruledef`, `HasHost` went false, and the host requirement compiled to nothing. Both failures
are indistinguishable from a legitimately quiet run, which is the silent-pass shape this whole layer
exists to prevent. `core/review`'s manifest conversion has had the guard since it was written and has
never drifted, which is the evidence that the cheap half works. Owning the converter is NOT enough on
its own: `stdlib/ruledef` claimed a body's wire form is owned beside its vocabulary so an omission is
a compile error, and that holds for a new NODE TYPE covered by a type switch, not for a new FIELD on
an already-mapped struct.
**Verify:** every `*Proto`/`*FromProto` pair over a config or rule-definition body has a test doing
`FromProto(Proto(full))` under `reflect.DeepEqual` with a fully-populated fixture. All six pairs are
covered: `TestManifestProtoRoundTrip` (`internal/service`), `TestProfileProtoRoundTrip`
(`stdlib/profiles`), `TestSpecProtoRoundTrip` (`core/check`), `TestQueryProtoRoundTrip`
(`core/query`), `TestRuleMetaProtoRoundTrip` (`core/check`), and `TestVerdictProtoRoundTrip`
(`internal/service`, guarded by `TestVerdictFieldCensus` beside it). A new body owes one before it
ships, not after it drifts.

`RuleMetaProto`/`RuleMetaFromProto` was the fifth pair and went uncovered while this list said four,
which is worth recording: the gap was found by adding a field (`Rule.Remedy`) that crosses it, not by
auditing against the constraint. An enumeration in a Verify block is only as current as the last
person who edited it.

The remaining asymmetry is `query.FindingQueryProto`, which has no `FindingQueryFromProto`: its decode
is inlined in `ruledef.Compile`, so there is no pair to round-trip and its field coverage rests on
that one call site. Give it an inverse if it grows.
**Verify (node vocabularies):** a body whose AST is a CLOSED vocabulary already fails loudly on a new
NODE, because `check.termProto`/`exprProto` panic on a type with no case. That protection does not
extend to a new FIELD, which is what the round-trip guard is for. Do not read the panic as covering
both.
**Note:** the fixture is the load-bearing part. A field left at its zero value round-trips cleanly
through a conversion that drops it, because zero in equals zero out, so a guard built on a sparse
fixture reports success while covering nothing.

**A REPEATED field needs more than one element in the fixture**, which is the same trap one level in.
A converter that drops everything past the first element round-trips a one-element slice perfectly, so
the guard passes over exactly the bug it exists to catch. `Verdict.subjects` became repeated when a
rule's subject grew to a tuple, and its fixture was widened to two entities in the same change for
this reason.

## C27: A committed generated artifact is checked by REGENERATING it
**Rule:** If a file is both committed and produced by a command in this repo, the gate regenerates it
from its inputs and fails on any difference. Inspecting `git status`, trusting a stamp the artifact
carries, or hashing the inputs does not count. Where regeneration cannot run in the gate yet, the
artifact carries a NAMED exemption in a checked-in ignore file and every entry cites the issue that
will remove it, rather than being quietly absent from the gate.
**Why:** a generated file nothing regenerates drifts from its source and nothing says so. A stamp
does not save you, because it hashes what the generator was pointed at rather than the generator. A
tutorial capture's stamp hashes its spec and its fixture and never the engine, so an engine change
that alters the output leaves every stamp valid. Changing one rule's coverage wording rewrote 0
captures on a plain docsite build and 12 on a forced regeneration. Before `tutorial-runs-check`
existed, a capture edited by hand passed the entire gate.
**Verify:** `proto-check`, `catalog-docs-check` and `tutorial-runs-check` are all in `testall`, and
each regenerates rather than reading `git status`. Exemptions live in
`hack/tutorial_runs_check.ignore`, and a line belongs there only when its command is not a function
of this repo, never because the artifact merely went stale.
**Outstanding violation:** `docsite/figures.sh` renders four committed SVGs under
`docsite/static/images/learn/` and is deliberately outside the gate (agni issue 453). Its stated
reason, that a render depends on the engine build, does not distinguish it from a capture, which
depends on the engine build too and is regenerated anyway. The real blocker is that the force layout
is not bit-identical across architectures (agni issue 472), so a regenerate-and-diff would fail on
the runner's architecture rather than on any change. Closing 472 is what lets this join the gate.
**Note:** a regeneration check must SNAPSHOT AND RESTORE rather than read `git status`. A
git-status check forces a regenerate-then-commit-then-gate ordering, so the first run after an edit
fails for a reason unrelated to the edit. `proto-check` spells this out and
`hack/tutorial_runs_check.sh` follows it.
