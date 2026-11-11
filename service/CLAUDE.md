# service

This file covers the transport-neutral services, their ports, snapshots, kept reads and tables.

- **An embedder composes against `service/`**, so it sits outside `internal/` (C13). A private deployment implements two ports, `ProjectStore` and `ConfigResolver`, to serve its own designs and config.
- **`ConfigResolver` turns config URIs into a `ResolvedConfig`.** It takes the URIs that a project's or a request's `AnalysisConfig` names. It returns rule sources, a param provider and symbol paths.
- **The package does no file I/O.** It imports no `os`, no `path/filepath` and no transport. Every I/O concern arrives as an injected port. `internal/projects` holds the directory-walking `FSStore`, and it is internal on purpose.
- **The backend answers, and a client renders** (agni 1045). When the viewer must still compute what the engine computes, a TWIN fixture holds the two to one answer, and both test suites read it. The twins are `core/review/testdata/tally_twin.json`, `core/results/testdata/severity_twin.json`, `core/query/testdata/binding_twin.json`, `core/facts/testdata/kind_order.json`, `core/ident/testdata/natural_twin.json` and `core/check/testdata/subject_param_twin.json`. A new client copy of engine logic gets a twin. Better, it comes off the wire, like `TextPosition.caret`.

## Snapshots and tiers

- **Every question about a design names a snapshot, and only `Ingest` names files** (agni 1061, `service/snapshot.go`, "The services" in `architecture/web-services.md`). Ingest resolves the tiers and composes the overlay once. Ids are random, so two ingests make two snapshots over one shared cache.
- **A snapshot refuses a question once a file it read changes.** It records the files that its questions read. A changed file answers stale (FAILED_PRECONDITION). A host that lost the snapshot answers NOT_FOUND. Both carry a `SnapshotUnavailable` detail, and the viewer recovers from it in `web/src/snapshots.ts`.
- **Snapshots live on the `ProjectResolver` that the services share.** A host that composes services over two resolvers holds two sets. Each service's `Snapshots()` returns the SnapshotService over its own loader and resolver.
- **Each browser worker holds its own snapshots.** `mountingFetch` replays the page's ingest into a worker before its first question. The CLI ingests, asks and deletes per command, locally or over `--remote`.
- **A design's tiers are resolved once, in `service.ResolveSources`** (C32, agni 656). The CLI and the server both call it. It decides whether a ref names the design, its entry, a declared companion or an unrelated file. `as_named` travels on the wire because the CLI is itself a client of these services.
- **A new rpc takes `snapshot` and reads `Snap.Netlist`, `Snap.Board` and `Snap.Geometry`** (agni 1061). `TestC22AnalysisRequestsNameASnapshot` holds every analysis request to that.
- **A loader can attach a companion that the service never resolved** (agni 646). Given no board, `BuildModel` asks the loader for one at the design's own URI. The CLI's `localLoader` resolves a descriptor inside `Board`. The served `osLoader` reads exactly what it is handed. So a CLI read is evidence about the loader, not about the request.
- **`service.SourcesFor` gives a design's tiers, and naming the ENTRY names the design.** A descriptor's `companions` supply the tiers the entry cannot, such as a schematic export for sheets and a board for copper. `NetlistURI` always points at the entry. The folder, the entry filename and a declared companion resolve identically. `--as-named` opts out.
- **An undeclared sibling is read exactly as named.** Companions are declared file by file, never inferred. A later netlist revision in the same folder is a separate analysis source.
- **A later revision is declared under `revisions`, with its own entry and companions** (agni 848). It reads with its own board. A file belongs to one revision at most, so a board is never borrowed across revisions.
- **An undeclared or board-less revision reads with no board.** `check` names the board-tier rules that did not run on stderr (`check.NoBoardReason`). `Tabulate` gives the run's `skipped` table.

## The board tier

- **Opening a design does not parse its board** (agni 943). `GetDesign` lists the Board sheet from the board tier's format (`formats.HasBoard`). The board is read when its sheet opens or a board-tier rule runs.
- **A design with a board and no drawn schematic lists the board alone** (`Snap.BoardOnly`, agni 1466). It opens on the board, and every locate answer names it. A new surface that says where a subject is drawn reads `snapGeometry` and `locateBoard`.
- **The Load page reads the board before the viewer opens** (agni 1072). It calls `GetDesign`, then an `Ingest` with `read_board` on a `prepare` worker. That worker leaves the parsed board in the shared store for the jobs worker. The viewer opens only a design the Load page recorded (`web/src/admit.ts`, `loaded.ts`). It sends any other to `/load` with its address whole.
- **A model is built with a `BoardTier`** (agni 987). Rule runs pass `ReadBoard`. A query passes it only when `query.ReadsFrom` names a `board.*` relation, through `boardTierFor`. It sees through rules and library members. Coverage and part parameters pass `SkipBoard`. A new surface that builds a model chooses its tier, because `BuildModelCached` reads the board.
- **The browser's serve worker gets no board bytes** (agni 992). `ListDesignFiles` flags the board file (`DesignFile.board`), and the serve lane gets a placeholder at its path. An engine that opens a placeholder refuses every question until the files arrive (`internal/wasmengine/withheld.go`). `mountingFetch` then brings the board and asks again. A page-side rule that decides which rpcs read the board is the bug shape, since a library member can read `board.*` under a query that never names it.
- **`DesignService.LocateSubjects` says where a part, pin or net is drawn** (agni 1325). It and a check run's findings read one index, so a subject locates where its finding badges. A pin answers the sheets that draw its own unit.
- **The board sheet joins a locate only when the request sets `board`.** It matches by ref-des and net NAME, since an EDIF schematic and an ODB++ job share no net id.
- **A per-click caller goes through `boardIndexFor`, never `Board`.** The caching loader's `Board` returns a COPY of the whole board. A run whose model read the board badges its findings with `"board"` after the schematic sheets.

## Kept reads

- **A server and the browser engine keep what a request read** (agni 895, `service.DesignCache`, "Kept reads" in `architecture/web-services.md`). Validity is judged on the files the read touched. `formats.Loader` records them through `service.LoaderIn`. A host that reads around `LoaderIn` is never cached.
- **The design is kept as a PARSE and a design DERIVED from it** (agni 1061). `formats.Loader.ParseDesign` reads no configuration and is keyed without the overlay. `formats.DeriveDesign` applies the vocabulary and corpus per overlay. A new ingestion pass that reads project config belongs in `DeriveDesign`, or one project's result is served to another.
- **Ask `param.Fetches` whether a corpus fetches over the network** (agni 915). Never use a `Prefetcher` type assertion. `SpecsOver` wraps every corpus in a `Layered`, and `Layered` always implements `Prefetch`.
- **A query's `work` is what THIS request spent.** The fact base is kept, so a warm server can report less work than a cold CLI.
- **`CheckDesign` keeps each rule's results on the model's cache entry** (agni 390, `service/keptresults.go`). A rule already run on an unchanged design under the same overlay answers without evaluating. The kept results have no key of their own, because the entry's key and touched-file check cover every tier and the overlay.
- **The join of kept and new rules relies on `check.RunAll` sorting STABLY.** `sort.Slice` there reorders tied findings, and the join then fails.

## Answers and tables

- **A query answer keeps each entity and citation once** (agni 916), in `RunQueryResponse.entities` and `sources`. Rows point in through `cell_entity` and `cite_index`. Read a cell's location with `service.CellLocation` and a row's citations with `service.RowCites`. Viewer test fixtures normalize through `web/src/testwire.ts`.
- **`omit_locations` on a query drops the drawing-derived fields** (agni 917), for a caller that draws nothing. The Python client speaks binary protobuf by default, and `ConnectTransport(url, binary=False)` gives JSON.
- **An answer's ROWS are the engine's, and every table encodes them** (C35, agni 862). `service/tabulate.go` projects a check run (`findings`, `verdicts`, `verdicts_by_rule`, `skipped`), a query or set, a diff and a review into `Table` messages. `TableService.Tabulate` serves them, and every `--format csv` encodes them.
- **A client asks `Tabulate` and does not lay an answer out.** The Python client's `agni.tables` only encodes, and a ratchet test holds that.
- **`order_by` sorts by each column's TYPE.** An example is `["rule", "-subject"]`, or `--order-by` on the CLI. A `name` column puts R2 before R10, and `column_types` overrides a column's type.
- **Cells travel raw.** The csv encoder guards formulas, and `tables_to_xlsx` keeps a cell starting `=` as text. `check --verdicts --format json` is the whole CheckDesign response, verdicts included.
