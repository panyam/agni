---
title: "Web service contract"
description: "Which Connect service behind agni serve owns which RPC, and the contract details a caller has to know."
---

This page is the reference for the wire contract of `agni serve`. How those calls reach the engine,
and how a design reaches the browser in the first place, is on
[Web app and presenter](../web-app/).

## The services

Everything is Connect, proto-first, in JSON or binary, with a generated TypeScript client. The
split follows resource lifetimes. Workspace navigation, one design's rendering, checks over a
design, the two-design diff, ad-hoc datalog search, stored reviews, declared projects and datasheet
documents are independent concerns with independent cadences.

Every question about a design names a snapshot of it, `snapshots/{id}`, rather than its files (agni
issue 1061). `SnapshotService.Ingest` takes the design's URI, any board or drawing override,
`as_named` and the overlay config, resolves the design's tiers and composes its config once, and
returns the snapshot. It reads no design file, since an open draws a KiCad design from its
schematic's geometry and parses its netlist only for the first check or query, so a design that
cannot be read fails at its first question. Each analysis rpc below then carries only `snapshot`, so no
question resolves a companion, a project or a convention again (C22, C32). Each ingest is a new
snapshot with a random id, and what two snapshots of one design read is shared beneath them in the
cache. A snapshot records every file its questions read, and a question asked after one changed is
refused as stale (FAILED_PRECONDITION) rather than answered from files it never read;
`UpdateSnapshot` reads it again, whole or by `update_mask`. A host holds a snapshot until it is
deleted or evicted, so a missing one is NOT_FOUND, and both errors carry a `SnapshotUnavailable`
detail. The viewer mints each snapshot's id itself and keeps its definition (`web/src/snapshots.ts`),
so it recovers from either by ingesting again under that id or updating, and asks once more.
`Ingest` under an id in use with the same definition answers that snapshot, after reading what
`read_board` asks, which is what lets a client send it again without asking first.

A client can also send a design's files instead of naming files the host serves (agni issue 1061,
step 4b). The definition then carries a manifest, each file's path within the design's mount, its
SHA-256 and its size, and the contents travel apart: an `Ingest` naming contents the host does not
hold fails with FAILED_PRECONDITION and a `BlobsMissing` detail, the client sends them with
`UploadBlobs`, and asks again. The paths and mount name are the client's own, so every locator an
answer records reads as it would on the client (C28). Each manifest is read through a host of its
own, over those files alone, with its own small cache, shared by every snapshot of the same files; a
cache keyed on the URI would otherwise serve one upload's reads, or the host's own mount of the same
name, for another. A server keeps uploaded contents up to `--upload-max-bytes` (1 GB), the least
recently used dropped first, and a snapshot keeps its own files however many uploads come after it.
`agni <command> --remote` sends designs this way, and the browser engine takes no manifests, since
dropped files already reach it through its `local` mount.

| Service | RPC | What it does |
|---|---|---|
| Workspace | ListMounts | the mount names a tree roots on; `opens` drops the ones holding nothing that client can open and returns how many, so the sidebar can account for a mount an operator configured and cannot find |
| Workspace | ListDir | one directory level, each file labeled with its reader `format` and the `kind` of client that opens it (design, datasheet, or neither); `opens` declares what the caller can open, which drops folders with none of it anywhere beneath them (a bounded server-side walk, since one level of listing cannot see that far) and is what lets the two trees prune the same mounts to opposite answers |
| Workspace | ListDesignFiles | every file a design's analysis reads (its folder, its project's descriptor, the config directories the project names) with a size and hash each, for a page analysing it in the browser to fetch from `/raw/<mount>/<path>`; refused over 256 MB |
| Workspace | ProposeDesigns | the designs a folder of files makes, each with the `design.yaml` that would declare it, and every file no design reads with the reason; a folder holding a `design.yaml` is reported as declared, and a `.zip` reads as a folder on the browser's mount |
| Snapshot | Ingest | load a design once, its tiers resolved and its config composed, and name it `snapshots/{id}` for every question after; `read_board` builds the model with its board too |
| Snapshot | UploadBlobs | store the contents of a design a client sends as files, keyed by SHA-256, for the manifest an Ingest names |
| Snapshot | GetSnapshot, ListSnapshots, UpdateSnapshot, DeleteSnapshot | the standard methods over the snapshots a host holds; an update re-reads the design, whole or by `update_mask`, under the same name |
| Design | GetDesign | load and summarize one design: sheet list, effective layout, available layouts, native availability |
| Design | GetSheet | one rendered sheet, where `format` picks PACKED (columnar bytes for WebGL), SVG (the verification backend), or NATIVE (the format's own tool) |
| Design | HighlightSheet | resolve highlight spec layers against one sheet: PACKED yields primitive-index groups, SVG a transparent same-frame overlay document |
| Design | GetLayoutReport | how an auto-layout drew each component (glyph, box, provided symbol, or unresolved) |
| Design | TraceDesign | walk from one pin to another through the series parts between them and return the route, the same answer `agni trace` prints |
| Check | ListRules | the rule catalog with tags and per-design availability, which is true exactly for the rules a whole CheckDesign of the snapshot runs; it reads the netlist and never the board |
| Check | CheckDesign | run a rule subset and return findings, where each subject joins the packed primitive keys for highlighting |
| Check | GetExpectations | the design's expectation sidecar as its own resource, reconciled against findings client-side |
| Check | GetCheckReport | the severity-organized report, the same shape as `agni check --format report` |
| Check | GetInterfaceCoverage | per detected interface profile, each required signal with its matched net and state, from the same datalog the profile rules compile to. It reads the design through its overlay and declared tiers, as CheckDesign does, and walks the profiles the run's rules came from, so a project's own profiles replace the built-ins they supersede |
| Check | GetComponentParams | every component whose MPN resolves to a seeded PartSpec, with that spec's parameters; empty when no corpus is loaded |
| Check | GetNamingConvention | resolve a stored convention config into a value an OverlayConfig carries, parsed and validated |
| Diff | DiffDesigns | semantic diff of two designs plus the highlight maps, the wire form shared with `agni diff --format json` |
| Query | RunQuery | evaluate an ad-hoc datalog query over the design's fact base, returning columns and provenance-linked rows, the same engine as `agni query` |
| Query | RunQueries | a named set of queries over one read of the design, each result shaped as RunQuery would return it |
| Query | ListRelations | the relation catalog with arg labels, summary, and kind, or with a `path` one place in the namespace (`RelationEntry`, members one level deep), which the panel's picker drills through under the page's snapshot |
| Query | ListQuerySets | the query sets the design's project keeps in `queries/`, inherited ones first, each parsed |
| Query | ParseQuerySet | a set file's text read into the QuerySet RunQueries takes, for a caller with no parser of its own |
| Review | GetReviewManifest | resolve a checklist file into a manifest value, parsed and validated |
| Review | CreateReview | run a checklist against one design and store the result, returning the stored run |
| Review | GetReview | one stored run, by resource name |
| Review | ListReviews | stored runs newest first, paginated, filterable by design |
| Review | DeleteReview | remove a stored run |
| Review | RenderReviewReport | a review the caller holds, written as the checklist page `agni review --format html` writes; nothing is re-run or read from the store |
| Project | GetProject, ListProjects | the declared projects visible across the server's mounts |
| Project | GetDesign, ListDesigns | a declared design's identity and which file around it is which (a different question from Design.GetDesign) |
| Project | ResolveDesign | whether a file belongs to a declared design, and which; resolving to nothing is a normal answer |

`DatasheetService` is not in this table, because it is the datasheet producer's API (`agni.v1.dsapi`)
rather than the engine's, and `agnids` serves it rather than `agni serve` (C34, agni issue 744). It
carries the workbench's whole surface: the doc-IR and running the configured `--pdf2doc` producer
(`GetDocument`, `ExtractDocIR`), the drafts keyed by MPN (`GetDraft`, `ListDrafts`, `SaveDraft`,
`PublishDraft`, which need `agnids serve --corpus`), the
per-author annotations (`GetAnnotations`, `SaveAnnotations`), and the folder tree (`ListMounts`,
`ListDir`, which take the engine's workspace messages and answer exactly as `WorkspaceService` does).

A few contract details bite if missed.

- **Effective values echo back.** GetDesign returns the layout actually used, since a request for
  an unavailable layout resolves rather than erroring, and the client adopts the echo. The same
  holds for the sheet id. The client navigates by the ids GetDesign returned, never by index
  guesses, because sheet ids are non-numeric on purpose and a numeric selector means a positional
  index.
- **A highlight must mirror its sheet.** An overlay is only meaningful over the base render it was
  framed for, so the sheet, layout, and symbols in a HighlightSheet call must match the GetSheet
  it overlays.
- **The board is a sheet.** A `.kicad_pcb` renders as a synthetic "board" sheet, so
  navigation, deep links, and both highlight paths needed no board-specific client plumbing.
- **Diff is all-or-nothing.** If either side fails to load, the call fails. There is no partial
  diff.
- **Query evaluates on the server over netlist facts.** RunQuery loads the design, builds the
  model, and runs the same evaluator the CLI runs. The datasheet parameter relation answers only
  when the server has a corpus, from `agni serve --params <dir>` or the design's own project
  `params/`, and returns no rows otherwise. The evaluator is the dependency-free Go of the `jaala`
  datalog module, which is why the in-browser engine below runs it unchanged.
- **A design is named, a checklist is sent.** CreateReview carries the review manifest as a value
  while the design stays an artifact URI. A design is megabytes, needs a reader chosen by
  extension, and is re-requested across many calls, so the call names it and the server reads it
  from the mount. A checklist is a small declaration the caller already holds, and a service
  that took a path for it would need a filesystem to do its job. A project's checklists already
  arrive as values, by name, on its `AnalysisConfig.checklists`, so the viewer runs one of those
  without another call. GetReviewManifest is the bridge for a checklist FILE, such as a YAML sibling
  of a design that belongs to no project, held by a client with a URI and no filesystem. The server
  reads and validates the checklist once, and the client sends back the value it got. The CLI skips it, because reading the file the user named is its own job.
- **A query that does not parse says where.** RunQuery then fails with `invalid_argument` carrying
  a `TextPosition` error detail (byte `offset`, `line` and `column` from 1, the column counted in
  characters), which the query panel turns into a caret under the place. RunQueries answers the rest
  of the set and marks the bad query in its `NamedQueryResult`, with `error` and `error_at`. The CLI
  under `--format json` writes a failed request as this same Connect error JSON, so a script and a
  client parse one shape (C31).
- **Reviews and projects are resources, and everything else is a verb.** A review RUN outlives
  the call that made it, so it has a name and the four standard methods, with paging and filtering
  following AIP. A project and its designs carry names an operator declared, so they get `Get` and
  `List` and no mutators. Every other rpc here is addressed by an artifact URI, so its arguments are
  its whole identity and a resource name would be ceremony. That split is CONSTRAINTS C23.
- **Stored runs need a volume.** `agni serve --review-store <dir>` names a WRITABLE directory,
  separate from the design mounts, so persisting a run never writes into a mount. In a container it is a mounted volume. Without the flag the four review resource methods
  answer with a failed-precondition naming it, rather than running the checks and dropping the
  result. Runs stored there are visible to every client of the server; `agni serve` has no
  authentication yet.
- **The viewer can ask under its own vocabulary.** A naming convention is picked from the mount,
  resolved server-side into a value, and carried on every rule-running request as an `OverlayConfig`.
  It REPLACES the server's `--conventions` default for that request rather than adding to it, so the
  top bar names which vocabulary produced the answers on screen. Without that indicator a reader
  cannot tell a fixed design from a stopped rule, because replacement can stop a rule running and a
  rule that stops running produces no findings.
- **A stored run embeds the checklist it scored.** The document carries a manifest SNAPSHOT, not
  just the manifest's name. A checklist is an editable file, so a name would resolve to whatever it
  says today, and last quarter's review would re-render against this quarter's questions with its
  outcomes intact underneath.

## Kept reads

`service.DesignCache` keeps what a request read, in two layers (agni issue 895). `CachingLoader`
wraps a host's loader and keeps the design, each drawing and the board, handing every caller a copy,
since `check.NewModel` stamps datasheet classes into the design it is given. The design is kept at two
levels (agni issue 1061). The parse (`formats.Loader.ParseDesign`) reads no configuration and is keyed
on the URI and the symbol search path alone, and the derived design (`formats.DeriveDesign`, the
classes, roles, values and datasheet classes) is keyed on the parse and the overlay's identity. So a
second project's conventions, or a request's own intent, derive from the kept parse and do not read
the files again. Both are stored in the browser, so a worker under the same overlay restores the
derived design, and one under another overlay restores the parse and pays only the derive, about 22
ms of Jetson's 200 ms parse natively. Each level has its timing stages (`build.parse`,
`restore.parse`, `derive.design`, `build.design`) and a `parse=` and `design=` entry in the cache
line.
`BuildModelCached` keeps the check model and its fact base on top, SHARED rather than copied, which
is safe because rules only read a model and its memo is built for concurrent use. The query, check,
report, coverage and review surfaces read through it.

`CheckDesign` keeps each rule's findings and verdicts on that model's entry too (agni issue 390), so
a check asking for a rule the model has already run answers from that run, and only the rules it has
not run are evaluated. The results need no key of their own, because the entry is already keyed on
the design's tiers and the overlay's identity and checked against every file its read touched, so an
edit or a different convention lands on an entry holding no results. A model that could not be kept
holds none either. Under a work budget an inconclusive rule is not kept, since the budget may be what
stopped it. `check.RunAll` sorts stably, so a rule's results keep their order whichever rules ran
beside them, and joining kept rules to new ones answers as one run would. On Jetson a second check
of the whole catalog spends 17 ms in the engine rather than 103 ms.

The browser engine also collects garbage after each layer the cache reads, builds or restores
(`DesignCache.AfterLoad`, agni issue 1000), shown as a `collect.<layer>` stage under `Agni-Timing`. A
worker's wasm memory never shrinks, so without it the rules stage grew past what a board's parse left
behind instead of reusing it, and a Jetson check peaked at 1343 MB rather than 805. A server sets no
hook, since its memory goes back to the system and a full collection per layer would cost every
request.

An entry's key is what was asked, and its validity is what the read touched. A host's reads go
through `service.LoaderIn`, which hands the `formats.Loader` a `formats.Touched` recorder, and the
recorder stamps every name the read opens, walks or fails to find. A hit re-stamps those names, so a
sub-sheet, a companion or a symbol library invalidates an entry without anyone listing it. Two rules
keep it from serving a wrong answer. Read options carry an identity only when `Overlay.ReadOptions`
could compute one (`WithIdentity`), and a read under options with none, or with an option the
identity does not cover, keeps only its parse, which names no option, and derives again each time. A read that recorded nothing went around the Loader and is
never kept either. A new host must read through `LoaderIn`, or its reads are simply never cached.

The server and the browser engine run the same cache. The browser engine is rebuilt on every mount
change, which drops it. The wire contract stays stateless, so no client holds a session.

## The same contract in the browser

`agni serve` is one host for these services and the browser is another. `cmd/agni-wasm` builds the
engine as WebAssembly, and the viewer runs it in a Web Worker, answering the same Connect paths with
the same messages. The page's clients reach the worker through the `fetch` a Connect transport takes,
so no panel knows which host answered (agni issue 178).

A design keeps one URL whichever engine reads it, because the two choices are separate (agni issue
853). The MOUNT in `/designs/<mount>/<path>` says where the design's files live, and the ENGINE,
`server` or `wasm`, says who analyses them. `agni serve --engine wasm` writes the engine into the
viewer's shell, and a page's `?engine=` overrides it.

```
make wasm serve EXTRA_MOUNTS="--mount tut=examples/tutorial-project"
open http://localhost:8080/designs/tut/designs/gateway/view?engine=wasm
```

An engine can only analyse files it can reach. On the `wasm` engine the page brings a design's
files over from the server the first time a request names it. `ListDesignFiles` lists exactly what
the analysis reads (the design's folder, its project's descriptor, and the config directories the
project names) with a hash per file, the page fetches each from the read-only `/raw/<mount>/<path>`
route, and the worker mounts them under the same mount name. After that, nothing about the design
crosses the network. Listings and file reads stay with the server, which owns the mount namespace,
and every other service runs in the worker. A file whose transfer sends no bytes for 30 s is
abandoned and asked for again, up to three attempts, and the top bar names the file meanwhile
(`web/src/wasm/fetchfile.ts`, agni issue 1002), because a fetch has no timeout of its own and
seeds on the deployed demo timed out at 180 s on their first open with nothing saying why. Under `?timing` the page logs each file as `agni fetch <file>`
with its size, time and attempts.

Files dropped on the Load page's drop target (`/designs/local/load/`), or picked with its "Open
files" and "Open folder" buttons, go into the browser mount `local` under a folder of their own and
into the browser's IndexedDB, and nowhere else (agni issues 854 and 1072). A `.zip` among them reads
as the folder it was made from. The page asks the worker for `ProposeDesigns` over that folder and
shows each proposal as its `design.yaml`, which the visitor can edit; opening one writes that
descriptor beside the kept files and reads the design on its Load page. The descriptor is written
under the mount's `.agni-overlay/` folder, whose files read over the same paths (`fshost.OverlayDir`),
because a design that came out of a zip sits under a path the mount holds as a file, and the mount
refuses a file beneath a file. A kept drop survives a reload and the navigation to the viewer, which
gives the engine the drop's files before its first question (`web/src/drops.ts`), and the drop page
lists what it keeps with a Delete. Every request naming `local` goes to the worker, workspace calls
included, and none is ever listed on the server. On the `server` engine a `local` design is refused
with a note rather than sent, because the server has never seen it and would need it uploaded, which
a drop must not do by accident.

With no agni server at all, `agni site` writes the same pages as plain files for a static host
(agni issue 856). The shell then says it is static (`data-host`) and gives its path prefix
(`data-base`), so every asset and link goes under the prefix, workspace calls go to the worker too,
and a seeded mount's files come from `files/<mount>.json`, a listing in `ListDesignFiles`' shape,
and the files under `raw/<mount>/`. The file trees are the exception to the worker answering
workspace calls, because the worker holds only the files the page gave it, so asking it which mounts
exist answers none. `agni site` writes the answers its own `WorkspaceService` gives over each seed,
`mounts.json` for `ListMounts` and `dirs/<mount>.json` for `ListDir` of every folder in the mount, and
the page answers both from those (`web/src/staticworkspace.ts`, agni issue 978). A static host serves a design's page as `…/view/index.html`, so
on such a page the router reads an address ending `/view/` as the design.

The public demo is this site, deployed with the docs. `make docs-site` builds the docsite into
`docsite/dist`, which Pages serves at `/agni/`, writes the demo into `dist/demo/` under the base
`/agni/demo/`, and runs `web/browser/site.spec.ts` against those files. The docs workflow runs that
target and uploads the result, on every merge to main, and `make docs-deploy` dispatches the same run
by hand. The spec opens every seed the built site lists, so a seed added to `DEMO_SEEDS` is checked
on its first deploy.

A design too big for the browser goes to the server engine instead, and the page says so in its top
bar. The page learns a design's size from `ListDesignFiles` before it reads anything, and compares it
with `agni serve --wasm-max-bytes` (default 128 MB), which a page's `?wasm-max-bytes=` overrides. A
design whose listing fails goes to the server too. The default comes from measuring the engine in a
Chromium worker against the same composition run natively (agni issue 852, `make wasm-bench
BOARD=<folder> DESIGN=<path>`): on a 97 MB KiCad board of 1834 components the browser read it in
2.4 s, ran the catalog in 8.8 s and peaked at 451 MB, four to five times the native time with the
same 590 findings. The bytes stand in for cost, and that calibration is KiCad's, where the board file
is most of the size, so an EDIF netlist of the same size may cost more.

The host is goapplib's `wasmhost`, which `cmd/agni-wasm` runs with `wasmhost.ServeRebuild` and the
page reaches through `@panyam/tsappkit/wasmhost` (agni issue 863). Three of its choices follow from
the browser rather than from the contract.

- **Files are pushed in before a request, never read during one.** The page hands the worker a
  mount's files as bytes (`globalThis.agni.add`), and every read is then served from memory through
  an `fs.FS`. A browser can only produce bytes asynchronously, and an `fs.FS` read is synchronous, so
  reading mid-request would block Go on a JavaScript promise inside a callback that pauses the event
  loop the promise needs.
- **A request is answered as a promise, on a goroutine of its own.** `globalThis.agni.http` returns
  at once and runs the handler beside it, for the same reason.
- **A mount change recomposes the engine.** Each `add` keeps the files the mount already holds and
  builds the services again (`wasmengine.Build`), through `agni.New`, which refuses a build missing
  the rule catalog. A browser bundle that shipped without it would otherwise report every design
  clean.
- **The engine runs as two workers, one per kind of request** (agni issue 911). Go's wasm build has
  one thread per worker, so a request that runs the rule catalog holds its worker until it finishes,
  and on a single worker a query asked during "Run checks" waited for the whole run. The page starts
  two lanes (`startLane`): `jobs` answers the requests in `JOB_RPCS` (`CheckDesign`,
  `GetCheckReport`, `GetInterfaceCoverage`, `CreateReview`, `DiffDesigns`) and `serve` answers
  everything else. Each lane holds its own copy of the design and reads it on its first request,
  so memory doubles. On the Jetson AGX Thor baseboard in Chromium, a query asked half a second into
  the first check answered in 135 to 145 ms on two lanes against 2.6 to 5.9 s on one worker, and the
  two workers held about 850 MB against one's 455 MB (`make wasm-bench` prints both configurations).
  Aborting a request ends its lane's worker and starts an empty one, which brings
  the design's files in again on its next request and restores the design from the browser's cache
  (next bullet) rather than reading it. The Checks panel's Cancel and the Review panel's are such
  an abort (agni issue 918): one `AbortController` per run, its signal on every call the run makes,
  and `mountingFetch` rejecting at once when it fires, since a request waits on its design's files
  reaching the worker before the lane sees it and only the lane stops on the signal. On Jetson the
  button comes back in about 10 ms and the next run, on the fresh worker, answers in about 3 s.
  A selection's check queued behind the stopped run dies with the worker, so Properties asks it
  again when the run ends. On a server the signal cancels the HTTP request, and the run stops
  through its context (agni issue 795). Files dropped on the page are kept by the page
  (`WasmEngine.add`) and copied into every worker, replacements included, since nothing else could
  bring them back.
- **The serve worker is given a placeholder for the board** (agni issue 992). Opening a design, a
  query that names no `board.*` relation, a coverage and a trace build no board there (agni issue
  987), yet the worker used to hold the board file's bytes in its mount, 85 of the Jetson baseboard's
  92 MB. `ListDesignFiles` flags the design's board tier when it is a file of its own
  (`DesignFile.board`, resolved as `Ingest` resolves it, and `agni site` writes the same flag into a
  static listing), and `mountingFetch` gives the serve lane a short placeholder at that path, so the
  folder lists, stats and resolves as it does with the file in place. The jobs and prepare lanes get
  every file. Opening a placeholder fails, and the engine then answers every question as
  `FAILED_PRECONDITION` with a `BlobsMissing` detail naming the file's hash
  (`internal/wasmengine/withheld.go`), whatever its service made of the failed read, because a reader
  that treats an unreadable sibling as an absent one would otherwise answer from a design missing its
  copper, and a cache would keep that answer. The page then brings the board in, which rebuilds the
  engine, ingests the snapshot again and asks again. The engine decides rather than the page because
  only it knows which questions read the board: the Board sheet does, and so does a query whose
  library member reads a board relation the query text never names.
- **A parsed design outlives the worker that read it** (agni issue 911). The design cache keeps its
  proto layers (the design, its drawings, its board) in the origin's private file system as well,
  through goapplib's `wasmhost.BrowserCache`, which every worker of the page shares and a reload
  keeps. A second lane, a replacement worker or the next visit restores a design instead of reading
  it, and rebuilds the model and fact base from it, which have no serialized form (C8). On the Jetson
  baseboard a reloaded open took 100 to 140 ms against 2.1 to 2.7 s for a read, and left the serve
  worker at about 280 MB against about 390 MB. A stored entry carries a content fingerprint of every
  name its read touched (`formats.Touched.Fingerprint`, a size and CRC-64 per file), checked on
  restore, so a file edited, added or removed since is read again. It is keyed on
  `wasmengine.StoreVersion`, the commit plus a hash of the uncommitted changes that `make wasm`
  stamps, so a build that reads a file differently never restores another build's parse. A dirty
  build without that stamp persists nothing. The serve lane also has a memory limit (512 MB), past
  which, once idle, it is replaced by a worker that restores on its next request. A replacement that
  passes the limit for the first time raises it to what it holds plus 256 MB (half the limit), because a
  successor would restore the same design to the same size. Before that (agni issue 974) a restored
  Jetson baseboard at 548 MB was replaced after every request, and each query fetched the design's
  files again and restored its board.
- **A design is read on its Load page before the viewer opens it** (agni issue 1072). A design's
  address has two pages, `/designs/<mount>/<path>/load` and `/view`, with the same query. The Load
  page asks `GetDesign`, which parses the drawing, and then sends an `Ingest` with `read_board`,
  which builds the model a `CheckDesign` builds, board included, and runs nothing. The page answers
  that one on a third lane, `prepare`, which starts a worker for the request and ends it once it has
  answered, so the parse leaves its peak in that worker alone and the parsed board goes into the
  shared store. The jobs worker then restores it, which allocates about 23 MB on Jetson where the
  parse allocated 412 MB. When both answer, the page records what it read in `localStorage` (the
  engine, the build that wrote the page, the design and its files' content hashes, `web/src/loaded.ts`)
  and opens the viewer. The viewer opens only a design with a matching record and sends any other to
  its Load page with the address whole (`web/src/admit.ts`), so a link to a design this browser has
  not read is read first and lands where it pointed. The record is a convenience: the engine checks
  every stored read against the files' content, so a record that outlived its store costs a slower
  open and never a wrong answer, and the Load page marks the viewer's address (`loaded=1`) so a
  browser that keeps no record does not send a reader back and forth. The viewer itself still never
  parses the board on open (agni issue 943). A server answers the same `Ingest` in its own process,
  where the model it builds is kept for the check that follows. In the cache, a request waiting on
  another's read builds it again when the other request was cancelled, so an abandoned load never
  fails the check waiting on it. On a static host the browser mount has one page of each kind, and a
  dropped design names its path in their query (`?d=<drop>/<entry>`), since such a host serves only
  the pages `agni site` wrote.
- **Every worker keeps its own snapshots.** The page's `Ingest` of each passes through
  `mountingFetch`, which keeps it, brings the files it named into any worker a question about that
  snapshot reaches, and sends the worker the same `Ingest`, without `read_board`, before that
  worker's first question about it. A replacement worker starts with none, so it is given them again.

The browser and the server read through different adapters over one set of ports, `fshost` over
in-memory trees and `osLoader` over host paths, and two adapters behind one port can disagree with
nothing erroring (C32). `TestWasmEngineAnswersAsTheServerDoes` asks both the questions the viewer
asks on opening a design and requires identical answers, `make wasm-test` runs the exports under
Node, and the browser suite opens one design under both engines, requires the same findings, and
checks that on `wasm` only listings and file reads reach the network.
