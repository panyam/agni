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

| Service | RPC | What it does |
|---|---|---|
| Workspace | ListMounts | the mount names a tree roots on; `opens` drops the ones holding nothing that client can open and returns how many, so the sidebar can account for a mount an operator configured and cannot find |
| Workspace | ListDir | one directory level, each file labeled with its reader `format` and the `kind` of client that opens it (design, datasheet, or neither); `opens` declares what the caller can open, which drops folders with none of it anywhere beneath them (a bounded server-side walk, since one level of listing cannot see that far) and is what lets the two trees prune the same mounts to opposite answers |
| Workspace | ListDesignFiles | every file a design's analysis reads (its folder, its project's descriptor, the config directories the project names) with a size and hash each, for a page analysing it in the browser to fetch from `/raw/<mount>/<path>`; refused over 256 MB |
| Workspace | ProposeDesigns | the designs a folder of files makes, each with the `design.yaml` that would declare it, and every file no design reads with the reason; a folder holding a `design.yaml` is reported as declared, and a `.zip` reads as a folder on the browser's mount |
| Design | GetDesign | load and summarize one design: sheet list, effective layout, available layouts, native availability |
| Design | GetSheet | one rendered sheet, where `format` picks PACKED (columnar bytes for WebGL), SVG (the verification backend), or NATIVE (the format's own tool) |
| Design | HighlightSheet | resolve highlight spec layers against one sheet: PACKED yields primitive-index groups, SVG a transparent same-frame overlay document |
| Design | GetLayoutReport | how an auto-layout drew each component (glyph, box, provided symbol, or unresolved) |
| Design | TraceDesign | walk from one pin to another through the series parts between them and return the route, the same answer `agni trace` prints |
| Check | ListRules | the rule catalog with tags and per-design availability, static per build, fetched once |
| Check | CheckDesign | run a rule subset and return findings, where each subject joins the packed primitive keys for highlighting |
| Check | GetExpectations | the design's expectation sidecar as its own resource, reconciled against findings client-side |
| Check | GetCheckReport | the severity-organized report, the same shape as `agni check --format report` |
| Check | GetInterfaceCoverage | per detected interface profile, each required signal with its matched net and state, from the same datalog the profile rules compile to. It reads the design through its overlay and declared tiers, as CheckDesign does, and walks the profiles the run's rules came from, so a project's own profiles replace the built-ins they supersede |
| Check | GetComponentParams | every component whose MPN resolves to a seeded PartSpec, with that spec's parameters; empty when no corpus is loaded |
| Check | GetNamingConvention | resolve a stored convention config into a value an OverlayConfig carries, parsed and validated |
| Diff | DiffDesigns | semantic diff of two designs plus the highlight maps, the wire form shared with `agni diff --format json` |
| Query | RunQuery | evaluate an ad-hoc datalog query over the design's fact base, returning columns and provenance-linked rows, the same engine as `agni query` |
| Query | RunQueries | a named set of queries over one read of the design, each result shaped as RunQuery would return it |
| Query | ListRelations | the relation catalog with arg labels, summary, and kind, driving the panel's click-to-insert picker |
| Review | GetReviewManifest | resolve a checklist file into a manifest value, parsed and validated |
| Review | CreateReview | run a checklist against one design and store the result, returning the stored run |
| Review | GetReview | one stored run, by resource name |
| Review | ListReviews | stored runs newest first, paginated, filterable by design |
| Review | DeleteReview | remove a stored run |
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
wraps a host's loader and keeps the parsed design, each drawing and the board, handing every caller
a copy, since `check.NewModel` stamps datasheet classes into the design it is given.
`BuildModelCached` keeps the check model and its fact base on top, SHARED rather than copied, which
is safe because rules only read a model and its memo is built for concurrent use. The query, check,
report, coverage and review surfaces read through it.

An entry's key is what was asked, and its validity is what the read touched. A host's reads go
through `service.LoaderIn`, which hands the `formats.Loader` a `formats.Touched` recorder, and the
recorder stamps every name the read opens, walks or fails to find. A hit re-stamps those names, so a
sub-sheet, a companion or a symbol library invalidates an entry without anyone listing it. Two rules
keep it from serving a wrong answer. Read options carry an identity only when `Overlay.ReadOptions`
could compute one (`WithIdentity`), and a read under options with none, or with an option the
identity does not cover, is never kept. A read that recorded nothing went around the Loader and is
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
and every other service runs in the worker.

Files dropped on the page, or picked with its "Open files" button, go into the browser mount `local`
under a folder of their own and nowhere else (agni issue 854). A `.zip` among them reads as the folder
it was made from. The page asks the worker for `ProposeDesigns` over that folder and shows each
proposal as its `design.yaml`, which the visitor can edit; opening one writes that descriptor into
the mount and shows the design at `/designs/local/<folder>/<entry>/view` in the same page, without a
reload, since the files exist only in that tab's worker. The descriptor is written under the mount's
`.agni-overlay/` folder, whose files read over the same paths (`fshost.OverlayDir`), because a design
that came out of a zip sits under a path the mount holds as a file, and the mount refuses a file
beneath a file. Every request naming `local` goes to the
worker, workspace calls included, and none is ever listed on the server. On the `server` engine a
`local` design is refused with a note rather than sent, because the server has never seen it and
would need it uploaded, which a drop must not do by accident.

With no agni server at all, `agni site` writes the same pages as plain files for a static host
(agni issue 856). The shell then says it is static (`data-host`) and gives its path prefix
(`data-base`), so every asset and link goes under the prefix, workspace calls go to the worker too,
and a seeded mount's files come from `files/<mount>.json`, a listing in `ListDesignFiles`' shape,
and the files under `raw/<mount>/`. A static host serves a design's page as `…/view/index.html`, so
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
  the design's files in again on its next request. Files dropped on the page are kept by the page
  (`WasmEngine.add`) and copied into every worker, replacements included, since nothing else could
  bring them back.

The browser and the server read through different adapters over one set of ports, `fshost` over
in-memory trees and `osLoader` over host paths, and two adapters behind one port can disagree with
nothing erroring (C32). `TestWasmEngineAnswersAsTheServerDoes` asks both the questions the viewer
asks on opening a design and requires identical answers, `make wasm-test` runs the exports under
Node, and the browser suite opens one design under both engines, requires the same findings, and
checks that on `wasm` only listings and file reads reach the network.
