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
**Verify:** no client-side router; `solid-js` appears only in island/adapter package
dependencies; each interactive surface is an island mounted into a server-rendered hole.

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
**Verify:** no `"#rrggbb"` or `font-family` literals in `render/*.go` outside `style.go`;
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
`Reads` (its fact dependencies), and `Eval`, plus the prose (`Summary`/`Impact`/`Detail`). Every
classificatory axis — category, tier, distribution, and any provider-defined one — lives in an open
`Tags map[string]string`, never as a typed struct field. Availability derives from `Reads` (a rule
that reads a fact whose provider layer is absent is unavailable), not a stored track/label field.
Consumers take the rule catalog as an injected `[]*check.Rule`, not the `check.Rules` global.
**Why:** the set of classification axes is open and provider-specific — rules will come from a
Phase-2 DSL and from integrators embedding Agni (customer suites outside `check/`), so a closed
column schema would force a core change per axis and force external authors to populate
Agni-internal fields. Tags keep the catalog extensible (a browsable UI groups/filters by whatever
keys are present) and keep the rule model from overfitting to today's axes — the earn-its-place
discipline of C9 applied to the rule catalog. (This reversed a first cut that typed `Track` and
`Distribution` as fields.)
**Verify:** the `check.Rule` struct carries a `Tags map[string]string` and no per-axis
classification fields beyond the behavioral core; `check.Available` reads `r.Reads`, not a track
field; `NewDesignService` takes a `[]*check.Rule` parameter.

## C15: Readers never import the presentation tier
**Rule:** Format readers (`edif/`, `kicad/`, `ipc2581/`, `xschem/`, `geda/`, and any future
reader) must not import `render/` or `svg/`. Geometry math a reader and a renderer both need
(placement transforms, pin world positions) lives in `internal/geomath`, imported by both
sides. Dependencies point one way: readers produce IR/geom; the presentation tier consumes it.
**Why:** "pins land where symbols are drawn" must hold by shared code, not by a reader
reaching up into the renderer for its helper (which couples ingestion to presentation and
drags drawing code into every entrypoint that only wants netlists). One implementation of the
transform contract (the geometry doc) serves producers and consumers alike.
**Verify:** `grep -rl '"github.com/panyam/agni/render"\|"github.com/panyam/agni/svg"'
edif/ kicad/ ipc2581/ xschem/ geda/` returns nothing.

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
(`edif/`, `kicad/`, `ipc2581/`, `xschem/`, `geda/`) and the reader registry (`formats/`) depend
only downward — on the contract and shared parse/geom helpers — never on the application tiers
(`internal/service/`, `internal/server/`, the web transport, `servicekit`, `connectrpc`).
`formats/` is public (not `internal/`) precisely so an out-of-module reader registers through it
(WS12-003); that is the ONE reader extension seam. This subsumes C15 (readers ⊅ `render`/`svg`)
and generalizes it to the whole heavy tail.
**Why:** the open-core overlay, and any future ecosystem reader, depends on the contract plus the
registry — not on the web/serve tier. Go module-graph pruning already keeps that dependency light
*because* the layering holds; a stray import from a reader up into `internal/server` would pull
servicekit/connect into every consumer and foreclose extracting the reader tier as a module. Keep
the seam clean now so the split stays a rename, not a refactor.
**Verify:** `go list -deps ./edif ./kicad ./ipc2581 ./xschem ./geda ./formats | grep -E
'servicekit|connectrpc|panyam/agni/(render|svg|serve|internal/service|internal/server)'`
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
categories: (1) **producing** the IR — the readers (`edif/`, `kicad/`, `ipc2581/`, `xschem/`,
`geda/`), the `formats` loader, `internal/netgraph` (IR emission); (2) **constructing** the Model or
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

## C21: The netlist is the analysis source of truth; schematic/board files are geometry companions
**Rule:** Component and connectivity ANALYSIS (the BOM, nets, rules, findings) is sourced from the
NETLIST the design team produces (for OrCAD/Allegro that is the `.edn` netlist view; for KiCad the
schematic/PCB, which carry the netlist inline). A schematic-view or board file supplied ALONGSIDE it
(an OrCAD `.eds`, a `.kicad_pcb`, an IPC-2581 board) is a GEOMETRY companion — a canvas for rendering
and for locating/highlighting findings — and MUST NOT be treated as a second, independent COMPONENT
source to be reconciled or merged against the netlist. Findings computed on the netlist join to a
companion's geometry by NET NAME (primary, drift-resistant) and ref-des (secondary, degrading to a
"couldn't locate" note), never by inventing identity the companion lacks. A netlist SYNTHESIZED from an
`.eds` (when no real netlist exists) is a degraded, explicitly-labeled fallback with tool-synthesized
ref-des — net-level findings authoritative, component-identity cross-reference not — and is never
promoted to the source of truth.
**Why:** an exported schematic is a lossy, point-in-time SNAPSHOT, not the design's system of record.
Reference designators (`C1`, `R3005`) are the OUTPUT of a stateful authoring process — annotation +
layout back-annotation performed inside the native project, with the tool's netlister assigning numbers
as it emits the netlist — so a schematic export can carry placeholder (`C?`) or per-sheet-duplicate
designators for parts the netlist numbers uniquely (observed: a real automotive EDIF `.eds` export ≈ 50% un-back-annotated
vs a flat, fully-numbered `.edn`, WS1-046). The identity simply is not in the file, and no reader can
recover it — like regenerating a database's auto-increment keys from a keyless dump. Treating the
netlist as truth and the schematic/board as a companion to highlight (WS1-047) makes the join
drift-resistant (net names are stable across annotation), keeps analysis off an assumption we cannot
verify (that two exports are from the same instant), and matches the real workflow: the team gets the
netlist by one click on the project, we render their drawing beside it.
**Verify:** analysis/rule code sources components + nets from the loaded netlist model, not from a
geometry reader; a geometry companion contributes only render + locate. (Checkable once the
companion-file association lands, WS1-047: geometry-only readers feed no component/net into the rule
model.)
