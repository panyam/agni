---
title: "Web service contract"
description: "Which Connect service behind agni serve owns which RPC, and the contract details a caller has to know."
---

This page is the reference for the wire contract of `agni serve`.
[Web app and presenter](../web-app/) covers how those calls reach the engine, and how a design
reaches the browser in the first place.

## The services

Everything is Connect, proto-first, in JSON or binary, with a generated TypeScript client. The
split follows resource lifetimes. Workspace navigation, one design's rendering, checks over a
design, the two-design diff, ad-hoc datalog search, stored reviews, declared projects and datasheet
documents are independent concerns with independent cadences.

Every question about a design names a snapshot of it, `snapshots/{id}`, rather than its files (agni
issue 1061). `SnapshotService.Ingest` takes the design's URI, any board or drawing override,
`as_named` and the overlay config. It resolves the design's tiers, composes its config once, and
returns the snapshot. It reads no design file. An open draws a KiCad design from its schematic's
geometry and parses its netlist only for the first check or query, so a design that cannot be read
fails at its first question. Each analysis rpc below then carries only `snapshot`, so no question
resolves a companion, a project or a convention again (C22, C32). Each ingest is a new snapshot with
a random id. Two snapshots of one design share their reads in the cache beneath them. A snapshot
records every file its questions read. Once one of those files changes, a question is refused as
stale (FAILED_PRECONDITION) rather than answered from files it never read; `UpdateSnapshot` reads the
snapshot again, whole or by `update_mask`. A host holds a snapshot until it is deleted or evicted, so
a missing one is NOT_FOUND, and both errors carry a `SnapshotUnavailable` detail. The viewer mints
each snapshot's id itself and keeps its definition (`web/src/snapshots.ts`), so it recovers from
either by ingesting again under that id or updating, and asks once more. Imagine a client sends
`Ingest` under an id already in use, with the same definition. The host answers with that snapshot,
after reading what `read_board` asks. So a client can send an `Ingest` again without first asking
whether the host holds it.

A client can also send a design's files instead of naming files the host serves (agni issue 1061,
step 4b). The definition then carries a manifest that lists each file's path within the design's
mount, its SHA-256 and its size. The contents travel separately. If the host is missing any file in
the manifest, the `Ingest` fails with FAILED_PRECONDITION and a `BlobsMissing` detail. The client
sends the contents with `UploadBlobs` and asks again. The paths and mount name are the client's own,
so every locator in an answer reads as it would on the client (C28). Each manifest is read through a
host of its own. That host sees only the manifest's files and has its own small cache, and every
snapshot of the same files shares both. A cache keyed on the URI would otherwise serve one upload's
reads, or the reads of the host's own mount of the same name, for another. A server keeps uploaded
contents up to `--upload-max-bytes` (1 GB) and drops the least recently used first. A snapshot keeps
its own files however many uploads come after it.
`agni <command> --remote` sends designs this way, and the browser engine takes no manifests, since
dropped files already reach it through its `local` mount.

| Service | RPC | What it does |
|---|---|---|
| Workspace | ListMounts | the mount names to root a tree on; `opens` drops each mount holding nothing that client can open and returns how many it dropped. So when an operator configured a mount and cannot find it, the sidebar can account for it |
| Workspace | ListDir | one directory level, each file labeled with its reader `format` and the `kind` of client that opens it (design, datasheet, or neither); `opens` declares what the caller can open. The server then drops each folder with none of it anywhere beneath, through a bounded server-side walk, since one level of listing cannot see that far. So the two trees prune the same mounts to opposite answers |
| Workspace | ListDesignFiles | every file read by a design's analysis (its folder, its project's descriptor, the config directories the project names), with a size and hash each. A page analysing the design in the browser fetches them from `/raw/<mount>/<path>`. Refused over 256 MB |
| Workspace | ProposeDesigns | the designs in a folder of files, each with the `design.yaml` that would declare it, and every file left unread by any design, with the reason; a folder holding a `design.yaml` is reported as declared, and a `.zip` reads as a folder on the browser's mount |
| Snapshot | Ingest | load a design once, its tiers resolved and its config composed, and name it `snapshots/{id}` for every question after; `read_board` builds the model with its board too |
| Snapshot | UploadBlobs | when a client sends a design as files, store their contents keyed by SHA-256, for the manifest named in an Ingest |
| Snapshot | GetSnapshot, ListSnapshots, UpdateSnapshot, DeleteSnapshot | the standard methods over a host's snapshots; an update re-reads the design, whole or by `update_mask`, under the same name |
| Design | GetDesign | load and summarize one design: sheet list, effective layout, available layouts, native availability |
| Design | GetSheet | one rendered sheet, where `format` picks PACKED (columnar bytes for WebGL), SVG (the verification backend), or NATIVE (the format's own tool) |
| Design | HighlightSheet | resolve highlight spec layers against one sheet: PACKED yields primitive-index groups, SVG a transparent same-frame overlay document |
| Design | GetLayoutReport | how an auto-layout drew each component (glyph, box, provided symbol, or unresolved) |
| Design | LocateSubjects | which sheets draw each named part, pin, net or bus, the board added on request, with a reason for one drawn nowhere (agni issue 1325) |
| Design | TraceDesign | walk from one pin to another through the series parts between them and return the route, the same answer `agni trace` prints |
| Lexicon | GetLexicon | the effective naming lexicon of a snapshot's read, every pattern, exclusion, prefix, family and switch with where it came from, and the whole lexicon stated as a convention, the same answer `agni lexicon` prints. A request's `candidate` lists that convention instead, so an editor never merges one itself. `convention` is the convention as written, rules included, and an editor has to start from it |
| Lexicon | ExplainNet | one net's stamped roles with their reasons, and every net-name pattern its name matches, exclusions included |
| Lexicon | CompareLexicons | read a snapshot again with a candidate convention in its own's place, run the catalog over both, and return the `role_changes` and `finding_changes` tables; the Lexicon service is built from the host's CheckService, so the two run one catalog over one loader |
| Lexicon | RenderConvention | a convention, sent as a value, written as a project's YAML file; `naming.Parse` reads that file back to the same message. The Lexicon panel saves through it, since the page holds no YAML writer |
| Check | ListRules | the rule catalog with tags and per-design availability. A rule is available exactly when a whole CheckDesign of the snapshot runs it. ListRules reads the netlist and never the board |
| Check | CheckDesign | run a rule subset and return findings, where each subject joins the packed primitive keys for highlighting |
| Check | GetExpectations | the design's expectation sidecar as its own resource, reconciled against findings client-side |
| Check | GetCheckReport | the severity-organized report, the same shape as `agni check --format report` |
| Check | GetInterfaceCoverage | per detected interface profile, each required signal with its matched net and state, computed from the profile rules' own compiled datalog. It reads the design through its overlay and declared tiers, as CheckDesign does, and walks the profiles that supplied the run's rules, so a project's own profiles replace the built-ins they supersede |
| Check | GetComponentParams | every component whose MPN resolves to a seeded PartSpec, with that spec's parameters; empty when no corpus is loaded |
| Check | GetNamingConvention | resolve a stored convention config into a value an OverlayConfig carries, parsed and validated |
| Diff | DiffDesigns | semantic diff of two designs plus the highlight maps, the wire form shared with `agni diff --format json` |
| Query | RunQuery | evaluate an ad-hoc datalog query over the design's fact base, returning columns and provenance-linked rows, the same engine as `agni query` |
| Query | RunQueries | a named set of queries over one read of the design, each result shaped as RunQuery would return it |
| Query | ListRelations | the relation catalog with arg labels, summary, and kind, or with a `path` one place in the namespace (`RelationEntry`, members one level deep); the panel's picker drills through the namespace this way under the page's snapshot |
| Query | ListQuerySets | the query sets in the `queries/` of the design's project, inherited ones first, each parsed |
| Query | ParseQuerySet | a set file's text, read into a QuerySet, the form RunQueries takes, for a caller with no parser of its own |
| Review | GetReviewManifest | resolve a checklist file into a manifest value, parsed and validated |
| Review | CreateReview | run a checklist against one design and store the result, returning the stored run |
| Review | GetReview | one stored run, by resource name |
| Review | ListReviews | stored runs newest first, paginated, filterable by design |
| Review | DeleteReview | remove a stored run |
| Review | RenderReviewReport | renders the caller's review as the checklist page, as `agni review --format html` does; nothing is re-run or read from the store |
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
- **A highlight must mirror its sheet.** An overlay is framed for one base render and only means
  something over it, so the sheet, layout, and symbols in a HighlightSheet call must match the
  GetSheet it overlays.
- **The board is a sheet.** A `.kicad_pcb` renders as a synthetic "board" sheet, so
  navigation, deep links, and both highlight paths needed no board-specific client plumbing.
- **Diff is all-or-nothing.** If either side fails to load, the call fails. There is no partial
  diff.
- **Query evaluates on the server over netlist facts.** RunQuery loads the design, builds the
  model, and runs the same evaluator the CLI runs. The datasheet parameter relation answers only
  when the server has a corpus, from `agni serve --params <dir>` or the design's own project
  `params/`, and returns no rows otherwise. The evaluator is the dependency-free Go of the `jaala`
  datalog module, so the in-browser engine below runs it unchanged.
- **A design is named, a checklist is sent.** CreateReview carries the review manifest as a value,
  and the design as an artifact URI. A design is megabytes, needs a reader chosen by
  extension, and is re-requested across many calls, so the call names it and the server reads it
  from the mount. A checklist is a small declaration, and the caller already holds it. A service
  that took a path for it would need a filesystem to do its job. A project's checklists already
  arrive as values, by name, on its `AnalysisConfig.checklists`, so the viewer runs one of those
  without another call. Now imagine a client with a URI and no filesystem, and a checklist FILE such
  as a YAML sibling of a design that belongs to no project. GetReviewManifest is the bridge for that
  case. The server reads and validates the checklist once, and the client sends back the value it
  got. The CLI skips it, because reading the file the user named is its own job.
- **A row can say how it was derived.** Setting `witnesses` on `RunQueryRequest` or
  `RunQueriesRequest` asks the engine to record, for each row, the fact or rule that proved each
  literal of the goal. The record is `QueryRow.witnesses`, a tree of `QueryWitness`. Its derived
  nodes carry the rule as written, and their children follow the rule's written order. A witness
  reuses the answer's tables rather than carrying copies, so its `value_entity` points into
  `entities` as `cell_entity` does and its `cite_index` into `sources` as a row's does. It is off by
  default because recording a derivation per derived fact costs the evaluation memory, and the query
  panel asks for it only when a reader opens a row (agni issue 794).
- **A query that does not parse says where.** RunQuery then fails with `invalid_argument` and a
  `TextPosition` error detail. The detail gives the byte `offset`, and the `line` and `column` from
  1, with the column counted in characters. The query panel turns it into a caret under the place.
  RunQueries answers the rest
  of the set and marks the bad query in its `NamedQueryResult`, with `error` and `error_at`. The CLI
  under `--format json` writes a failed request as this same Connect error JSON, so a script and a
  client parse one shape (C31).
- **Reviews and projects are resources, and everything else is a verb.** A review RUN outlives
  the call that made it, so it has a name and the four standard methods, with paging and filtering
  following AIP. An operator declared the names of a project and its designs, so they get `Get` and
  `List` and no mutators. Every other rpc here is addressed by an artifact URI, so its arguments are
  its whole identity and a resource name would be ceremony. That split is CONSTRAINTS C23.
- **Stored runs need a volume.** `agni serve --review-store <dir>` names a WRITABLE directory,
  separate from the design mounts, so persisting a run never writes into a mount. In a container it
  is a mounted volume. Without the flag the four review resource methods answer with a
  failed-precondition naming it, rather than running the checks and dropping the result. Runs stored
  there are visible to every client of the server; `agni serve` has no authentication yet.
- **The viewer can ask under its own vocabulary.** A naming convention is picked from the mount,
  resolved server-side into a value, and carried on every rule-running request as an `OverlayConfig`.
  It REPLACES the server's `--conventions` default for that request rather than adding to it, so the
  top bar names which vocabulary produced the answers on screen. Without that indicator a reader
  cannot tell a fixed design from a stopped rule, because replacement can stop a rule running, and a
  stopped rule produces no findings.
- **A stored run embeds the checklist it scored.** The document carries a manifest SNAPSHOT, not
  just the manifest's name. A checklist is an editable file, so a name would resolve to whatever it
  says today, and last quarter's review would re-render against this quarter's questions with its
  outcomes intact underneath.

## Kept reads

`service.DesignCache` keeps what a request read, in two layers (agni issue 895). `CachingLoader`
wraps a host's loader and keeps the design, each drawing and the board, handing every caller a copy,
since `check.NewModel` stamps datasheet classes into the design it is given. The design is kept at
two levels (agni issue 1061). `formats.Loader.ParseDesign` produces the parse. It reads no
configuration and is keyed on the URI and the symbol search path alone. `formats.DeriveDesign`
produces the derived design, adding the classes, roles, values and datasheet classes, and it is
keyed on the parse and the overlay's identity. So a second project's conventions, or a request's own
intent, derive from the kept parse and do not read the files again. Both levels are stored in the
browser. A worker under the same overlay restores the derived design. A worker under another overlay
restores the parse and only runs the derive, which takes about 22 ms natively on Jetson against 200
ms for the parse. Each level has its timing stages (`build.parse`, `restore.parse`, `derive.design`,
`build.design`) and a `parse=` and `design=` entry in the cache line. `BuildModelCached` keeps the
check model and its fact base on top, SHARED rather than copied. Sharing is safe because rules only
read a model and its memo is built for concurrent use. The query, check, report, coverage and review
surfaces read through it.

`CheckDesign` keeps each rule's findings and verdicts on that model's entry too (agni issue 390).
Imagine a check asks for a rule, and the model has already run that rule. The check answers from
that run, and only the rules the model has not run are evaluated. The results need no key of their
own. The entry is already keyed on the design's tiers and the overlay's identity, and it is checked
against every file its read touched. So after an edit, or under a different convention, a check
reaches an entry holding no results. A model that could not be kept holds none either. Under a work
budget an inconclusive rule is not kept, since the budget may have stopped it. `check.RunAll` sorts
stably, so a rule's results keep their order whichever rules ran beside them, and joining kept rules
to new ones answers as one run would. On Jetson a second check of the whole catalog spends 17 ms in
the engine rather than 103 ms.

The browser engine also collects garbage each time the cache reads, builds or restores a layer
(`DesignCache.AfterLoad`, agni issue 1000), shown as a `collect.<layer>` stage under `Agni-Timing`. A
worker's wasm memory never shrinks, so without it the rules stage grew past what a board's parse left
behind instead of reusing it, and a Jetson check peaked at 1343 MB rather than 805. A server sets no
hook, since its memory goes back to the system and a full collection per layer would cost every
request.

An entry is keyed on what was asked, and its validity depends on what the read touched. A host's
reads go through `service.LoaderIn`, which hands the `formats.Loader` a `formats.Touched` recorder,
and the recorder stamps every name the read opens, walks or fails to find. A hit re-stamps those
names, so a sub-sheet, a companion or a symbol library invalidates an entry without anyone listing
it. Two rules keep it from serving a wrong answer. First, read options carry an identity only when
`Overlay.ReadOptions` could compute one (`WithIdentity`). A read under options with no identity, or
with an option outside the identity, keeps only its parse, since the parse names no option, and it
derives again each time. Second, a read that recorded nothing went around the Loader, and it is
never kept either. A new host must read through `LoaderIn`, or its reads are simply never cached.

The server and the browser engine run the same cache. The browser engine is rebuilt on every mount
change, which drops it. The wire contract is still stateless, so no client holds a session.

## The same contract in the browser

`agni serve` is one host for these services and the browser is another. `cmd/agni-wasm` builds the
engine as WebAssembly, and the viewer runs it in a Web Worker, answering the same Connect paths with
the same messages. The page's clients reach the worker through the `fetch` function passed to a
Connect transport, so no panel knows which host answered (agni issue 178).

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
project names), with a hash per file. The page fetches each file from the read-only
`/raw/<mount>/<path>` route, and the worker mounts them under the same mount name. After that,
nothing about the design crosses the network. The server still answers listings and file reads,
since it owns the mount namespace, and every other service runs in the worker. If a file's transfer
sends no bytes for 30 s, the page abandons it and asks for it again, up to three attempts, and the
top bar names the file meanwhile (`web/src/wasm/fetchfile.ts`, agni issue 1002). A fetch has no
timeout of its own. Before this, seeds on the deployed demo timed out at 180 s on their first open
with nothing saying why. Under `?timing` the page logs each file as `agni fetch <file>` with its
size, time and attempts.

A visitor can drop files on the Load page's drop target (`/designs/local/load/`), or pick them with
its "Open files" and "Open folder" buttons. The files go into the browser mount `local` under a
folder of their own and into the browser's IndexedDB, and nowhere else (agni issues 854 and 1072). A
`.zip` among them reads as the folder it was made from. The page asks the worker for
`ProposeDesigns` over that folder and shows each proposal as its `design.yaml`, which the visitor
can edit; opening one writes that descriptor beside the kept files and reads the design on its Load
page. The descriptor is written under the mount's `.agni-overlay/` folder, and files there read over
the same paths (`fshost.OverlayDir`). A design that came out of a zip sits under the zip's path, the
mount holds that path as a file, and the mount refuses a file beneath a file. A kept drop survives a
reload and the navigation to the viewer, and the page gives the engine the drop's files before its
first question (`web/src/drops.ts`). The drop page lists what it keeps with a Delete. Every request
naming `local` goes to the worker, workspace calls included, and none is ever listed on the server.
On the `server` engine a `local` design is refused with a note rather than sent, because the server
has never seen it and would need it uploaded, and a drop must not upload by accident.

With no agni server at all, `agni site` writes the same pages as plain files for a static host
(agni issue 856). The shell then says it is static (`data-host`) and gives its path prefix
(`data-base`), so every asset and link goes under the prefix, workspace calls go to the worker too,
and a seeded mount's files come from `files/<mount>.json`, a listing in `ListDesignFiles`' shape,
and the files under `raw/<mount>/`. The file trees are the exception, and the worker does not
answer them. The worker holds only the files the page gave it, so asked which mounts exist, it
answers none. `agni site` asks its own `WorkspaceService` about each seed and writes the answers,
`mounts.json` for `ListMounts` and `dirs/<mount>.json` for `ListDir` of every folder in the mount.
The page answers both calls from those files (`web/src/staticworkspace.ts`, agni issue 978). A
static host serves a design's page as `…/view/index.html`, so on such a page the router reads an
address ending `/view/` as the design.

The public demo is this site, deployed with the docs. `make docs-site` builds the docsite into
`docsite/dist`, which Pages serves at `/agni/`, writes the demo into `dist/demo/` under the base
`/agni/demo/`, and runs `web/browser/site.spec.ts` against those files. The docs workflow runs that
target and uploads the result, on every merge to main, and `make docs-deploy` dispatches the same
run by hand. The spec opens every seed listed in the built site, so a seed added to `DEMO_SEEDS` is
checked on its first deploy.

A design too big for the browser goes to the server engine instead, and the page says so in its top
bar. The page learns a design's size from `ListDesignFiles` before it reads anything, and compares it
with `agni serve --wasm-max-bytes` (default 128 MB), which a page's `?wasm-max-bytes=` overrides. A
design whose listing fails goes to the server too. The default comes from measuring the engine in a
Chromium worker against the same composition run natively (agni issue 852, `make wasm-bench
BOARD=<folder> DESIGN=<path>`). On a 97 MB KiCad board of 1834 components the browser read it in
2.4 s, ran the catalog in 8.8 s and peaked at 451 MB, four to five times the native time with the
same 590 findings. The byte count stands in for cost. That calibration comes from KiCad, where the
board file is most of the size, so an EDIF netlist of the same size may cost more.

The host is goapplib's `wasmhost`. `cmd/agni-wasm` runs it with `wasmhost.ServeRebuild`, and the
page reaches it through `@panyam/tsappkit/wasmhost` (agni issue 863). Its choices follow from
the browser rather than from the contract.

- **Files are pushed in before a request, never read during one.** The page hands the worker a
  mount's files as bytes (`globalThis.agni.add`), and every read is then served from memory through
  an `fs.FS`. A browser can only produce bytes asynchronously, and an `fs.FS` read is synchronous, so
  reading mid-request would block Go on a JavaScript promise inside a callback. That callback pauses
  the event loop, and the promise needs the event loop to resolve.
- **A request is answered as a promise, on a goroutine of its own.** `globalThis.agni.http` returns
  at once and runs the handler beside it, for the same reason.
- **A mount change recomposes the engine.** Each `add` keeps the mount's existing files and
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
  (next bullet) rather than reading it. The Checks panel's Cancel and the Review panel's Cancel
  each send such an abort (agni issue 918). Each run has one `AbortController`, and its signal goes
  on every call the run makes. `mountingFetch` rejects at once when the signal fires. It has to,
  because a request waits for its design's files to reach the worker before the lane sees it, and
  only the lane stops on the signal. On Jetson the
  button comes back in about 10 ms and the next run, on the fresh worker, answers in about 3 s.
  A selection's check queued behind the stopped run dies with the worker, so Properties asks it
  again when the run ends. On a server the signal cancels the HTTP request, and the run stops
  through its context (agni issue 795). The page keeps files dropped on it (`WasmEngine.add`) and
  copies them into every worker, replacements included, since nothing else could bring them back.
- **The serve worker is given a placeholder for the board** (agni issue 992). Four requests build
  no board on the serve worker: opening a design, a query that names no `board.*` relation, a
  coverage and a trace (agni issue 987). Yet the worker used to hold the board file's bytes in its
  mount, 85 of the Jetson baseboard's 92 MB. When the design's board tier is a file of its own,
  `ListDesignFiles` flags it (`DesignFile.board`), resolving the tier as `Ingest` does, and `agni
  site` writes the same flag into a static listing. `mountingFetch` then gives the serve lane a short
  placeholder at that path, so the folder lists, stats and resolves as it does with the file in
  place. The jobs and prepare lanes get every file. Opening a placeholder fails. The engine then
  answers every question as `FAILED_PRECONDITION` with a `BlobsMissing` detail naming the file's hash
  (`internal/wasmengine/withheld.go`), whatever its service made of the failed read. Without that, a
  reader that treats an unreadable sibling as an absent one would answer from a design missing its
  copper, and a cache would keep that answer. The page then brings the board in, which rebuilds the
  engine, ingests the snapshot again and asks again. The engine decides rather than the page, because
  only the engine knows which questions read the board. The Board sheet reads it. So does a query
  whose library member reads a board relation, even when the query text never names one.
- **A parsed design outlives the worker that read it** (agni issue 911). The design cache keeps its
  proto layers (the design, its drawings, its board) in the origin's private file system as well,
  through goapplib's `wasmhost.BrowserCache`. Every worker of the page shares that cache, and it
  survives a reload. A second lane, a replacement worker or the next visit restores a design instead
  of reading it, and rebuilds the model and fact base from it, which have no serialized form (C8).
  On the Jetson baseboard a reloaded open took 100 to 140 ms against 2.1 to 2.7 s for a read, and
  left the serve worker at about 280 MB against about 390 MB. A stored entry carries a content
  fingerprint of every name its read touched (`formats.Touched.Fingerprint`, a size and CRC-64 per
  file). A restore checks the fingerprint, so a file edited, added or removed since is read again.
  The store is keyed on `wasmengine.StoreVersion`, the commit plus a hash of the uncommitted changes
  that `make wasm` stamps, so a build that reads a file differently never restores another build's
  parse. A dirty build without that stamp persists nothing. The serve lane also has a memory limit
  of 512 MB. Once the lane is past the limit and idle, a new worker replaces it and restores the
  design on its next request. If a replacement passes the limit for the first time, the limit rises
  to what the replacement holds plus 256 MB (half the limit). It rises because a successor would
  restore the same design to the same size. Before that (agni issue 974) a restored Jetson baseboard
  at 548 MB was replaced after every request, and each query fetched the design's files again and
  restored its board.
- **A design is read on its Load page before the viewer opens it** (agni issue 1072). A design's
  address has two pages, `/designs/<mount>/<path>/load` and `/view`, with the same query. The Load
  page asks `GetDesign`, which parses the drawing, and then sends an `Ingest` with `read_board`,
  which builds the same model as a `CheckDesign`, board included, and runs nothing. The page answers
  that `Ingest` on a third lane, `prepare`. The lane starts a worker for the request and ends it
  once the worker has answered. So only that worker reaches the parse's memory peak, and the parsed
  board goes into the shared store. The jobs worker then restores it, which allocates about 23 MB on
  Jetson where the parse allocated 412 MB. When both answer, the page records what it read in
  `localStorage` (the engine, the build that wrote the page, the design and its files' content
  hashes, `web/src/loaded.ts`) and opens the viewer. The viewer opens only a design with a matching
  record and sends any other to its Load page with the address whole (`web/src/admit.ts`). So if
  this browser has not read a linked design, the Load page reads it first and then opens the address
  the link named. The record is a convenience. The engine checks every stored read against the
  files' content, so a record that outlived its store means a slower open and never a wrong answer.
  The Load page also marks the viewer's address (`loaded=1`), so a browser that keeps no record does
  not send a reader back and forth. The viewer itself still never parses the board on open (agni
  issue 943). A server answers the same `Ingest` in its own process, where the model it builds is
  kept for the check that follows. In the cache, one request can wait on another's read. If the
  other request is cancelled, the waiting request builds the read again, so an abandoned load never
  fails the check waiting on it. On a static host the browser mount has one page of each kind, and a
  dropped design names its path in their query (`?d=<drop>/<entry>`), since such a host serves only
  the pages `agni site` wrote.
- **Every worker keeps its own snapshots.** For each snapshot, the page's `Ingest` passes through
  `mountingFetch`, and `mountingFetch` keeps it. When a question about that snapshot reaches a
  worker, `mountingFetch` brings the files the `Ingest` named into that worker. Before that worker's
  first question about the snapshot, it also sends the worker the same `Ingest`, without
  `read_board`. A replacement worker starts with none, so it is given them again.

The browser and the server read through different adapters over one set of ports, `fshost` over
in-memory trees and `osLoader` over host paths, and two adapters behind one port can disagree with
nothing erroring (C32). `TestWasmEngineAnswersAsTheServerDoes` asks both hosts the viewer's
questions on opening a design and requires identical answers, `make wasm-test` runs the exports under
Node, and the browser suite opens one design under both engines, requires the same findings, and
checks that on `wasm` only listings and file reads reach the network.
