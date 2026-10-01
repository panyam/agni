# Agni engine repo orientation

Agni is EDA tooling built from a Go engine, a protobuf IR, format readers, diff/checks, a geometry
sidecar, a web viewer, and a CLI. It reads electronic design files in several formats into one
intermediate representation, then runs checks, diffs, queries, and renders over that IR.

**This repo is public, under Apache-2.0.** Anything committed here is world-readable the moment it
is pushed. Read "What does not belong in this repo" below before writing docs, tests, or fixtures.

This file is a ROUTER. It holds orientation, commands, and where-to-find-what; the subsystem detail
lives in `docsite/content/`, the workflow rules in `CONTRIBUTING.md`, and the enforceable rules in
`CONSTRAINTS.md`. If something here grows past a few paragraphs, it belongs in one of those.

## Read the docsite first

`docsite/content/` is the engineering source of truth and usually explains a subsystem better than
any summary. **Read the relevant page before working in that area**. Each holds gotchas that are
expensive to rediscover.

| Working on | Read first |
|---|---|
| Ingestion, the IR, a new format reader | `architecture/ingestion-and-ir.md`, `build/format-reader.md` |
| Geometry, rendering, the web viewer | `architecture/geometry-and-rendering.md`, `architecture/web-app.md` |
| The web wire contract, the viewer's interaction model, changing a panel | `architecture/web-services.md`, `architecture/web-picking.md`, `architecture/web-client.md` |
| Net solving, hierarchy, net identity | `architecture/net-solving.md` |
| A check rule, datalog, interface profiles | `architecture/rules-and-checks.md`, `build/check-rule.md` |
| A declared pin map, or comparing an identifier across two documents | `guide/design-intent.md` (the nine forms), `core/ident`'s package comment |
| The checks contract (the CLI/service boundary) | `architecture/checks-contract.md` |
| The config a run is checked against, and where it comes from | `architecture/projects-and-designs.md` |
| Semantic diff | `architecture/semantic-diff.md` |
| The datasheet param/doc/derive layer | `architecture/datasheet-layer.md` |
| Extending the engine from outside | `build/extending.md` |
| Calling agni from Python or another language | `build/other-languages.md`, `clients/python` |
| xschem / Lepton native tools | `build/native-verification.md` |
| Running the gate, and how it reads green when it is not | `build/the-gate.md` |
| Measuring something, or trusting a green test | `build/evidence.md` |
| Learning the domain from a software background | `reference/analogy.md`, `reference/edif-primer.md` |
| Asking a "no related Y" question, or any query past one clause | `guide/querying.md` (derived relations: `;` splits clauses, `:-` names one) |
| Why a rule exists at all, as engineering rather than as code | `learn/` (twelve chapters, EE1-EE7) |

`guide/` is the user-facing manual (getting-started, concepts, running-the-server,
checks-and-reports, comparing-revisions, querying, naming-conventions, interface-profiles,
design-intent, datasheets, cli-reference).
`tutorials/` walks one board from first read to a CI gate. `reference/` also holds the GENERATED
rule and relation catalogs.

`learn/` is the DOMAIN course, the axis the other sections do not cover. Its twelve chapters teach
what a hardware engineer knows, each ending in the rules that encode it. `tutorials/` teaches
the tool and assumes the domain; the rule pages explain the check and assume the instinct. `learn/`
is the layer between. `learn/levels.md` defines EE1 through EE7 (parts, nets, roles, failure modes,
numbers, systems, layout) and maps every section of the course to its level, which is also the
vocabulary for asking: "explain `output-output-conflict` at EE4" wants the bench symptom rather than
the definition.

**When a change touches a rule the course teaches, check whether a chapter needs updating**, and cite
the relevant pages as prerequisite reading in the PR.

**Never paste an `<svg>` into a markdown page.** A hand-authored diagram lives in `docsite/figures/`
and the page carries `{{ includeFile "figures/<name>.svg" }}`, which inlines it at BUILD time, so the
figure still resolves `currentColor` and `--accent-color` against the page's theme while the prose
stays readable. An `<img src>` cannot do that, because the image renders in its own document and
inherits nothing. `includefile_test.go` fails the gate on a path that does not resolve (`IncludeFile`
returns an empty string and the build still succeeds), on a figure nothing includes, on a colour
literal, and on a BLANK LINE inside the file, which ends the raw-HTML block and drops everything
after it out of the `<svg>`. The house style, the traps, and the browser sweep that checks a
rendered figure's geometry are in `docsite/README.md`. **Serve `dist` under the `/agni/` prefix** when
you check anything there in a browser, because at a server's root every asset 404s, the page renders
with no CSS while still returning 200, and the nav then measures as its mobile layout at any
viewport width.

`site/` is stale build output, not a source tree. Some older notes reference a retired `docs/NN-*.md`
mkdocs tree that was folded into `docsite/content/` with audience-first names.

## Package layout

Engine analysis under **`core/`** (`core/check`, `core/review`, `core/render`, `core/report`, `core/diff`,
`core/facts`, `core/query`, `core/model`, `core/classify`, `core/ident`, and others). Format readers
under **`readers/`** (`readers/edif`, `readers/kicad`, `readers/ipc2581`, `readers/xschem`,
`readers/geda`, `readers/telesis`, plus `readers/formats`, the registry/Loader). The shipped rule
catalog, fact relations, profiles, and intent under **`stdlib/`** (`stdlib/rules/builtin/rule_*.go`,
`stdlib/rules/datalog`, `stdlib/rules/intent`, `stdlib/relations`, `stdlib/profiles`). The
datasheet contract in **`core/param`** (the PartSpec the engine reads), and the extraction pipeline
that produces it in **`datasheet/`, a Go module of its own** (`github.com/panyam/agni/datasheet`,
agni issue 744): `doc`, `derive`, `docindex`, `candidate`, the transport-neutral `dsservice`, its
Connect adapter `dsserver`, and the **`agnids`** binary (`datasheet/cmd/agnids`: `serve` for the
workbench, `derive`). It depends on the root, never the reverse, which C34 and C18 hold. Its tests
run in `make datasheet-test`, because `go test ./...` from the root never reaches a nested module. The embedding
surface under **`service/`** (the transport-neutral service impls and their ports), **`artifact/`**
(the `mount://` URI those ports speak) and **`mounts/`** (the named roots and the containment join). Plus `cmd/agni/`, `internal/`, `intake/`, `census/`, `protos/` +
`gen/`, `docsite/`, `web/`, `hack/`, `tools/`, and `clients/python` (the typed Python client, its
generated messages committed under `src/agni/v1`).

Notes written before this layout landed name the old directories. **Grep the SYMBOL or filename,
not the directory.**

**The Datalog engine is a separate module, `github.com/panyam/jaala/datalog`** (agni issue 731), and
`core/query` is agni's adapter over it: the `Source` projecting a Model through the fact registry, a
fact base per design over one vocabulary, `RuleFromQuery`, the wire form and the catalog. **Every
queryable name registers in `core/facts` at a path of jaala's `ns.Vocabulary`** (agni issue 751):
`facts.RegisterRelation`, `facts.RegisterPredicate` (an `ns.Builtin`, such as the `net.reaches`
generator in `stdlib/relations`, which reaches the design through `facts.EnvOf`) and
`facts.RegisterModules`. `jaala/ns` holds names and signatures, and `jaala/stdlib` the standard
predicates, with no evaluator in either, which is why C29 lets the fact layer import those two and
nothing more of jaala. **The shipped library of derived relations
is `stdlib/lib`**, one `.dl` file per module (`component.probed_both`, `net.has_test_point`), and
`agni query --relations <path>` prints any member's signature and definition. Its modules register
as ONE batch, because the vocabulary is checked at every registration and a module reading another
module's member is refused if it arrives first. A library reaches a query three ways, the shipped
one, a project's `lib/`, and modules sent WITH a request (`AnalysisConfig.library_modules`, the
CLI's `--lib`, agni 788), and all three compose in `Overlay.Registry`. **agni answers with `query.Default`**, jaala's
`SemiNaive`, which plans each rule body and derives a relation called with a bound argument only for
that value, so written clause order no longer decides cost. `Naive` is the reference tests compare
against, and a new call site that evaluates for a user uses `query.Default`. A generator declares the
bindings it accepts (`ns.Builtin.Modes`); `net.reaches` and `net.route` declare `from` bound or a
deliberate full walk. A language change
(parser, evaluator, index, aggregation) is a jaala PR and a tag first, then a `go get` here. Its
issues live on panyam/jaala.

`core/facts` is the fact/relation layer, and it depends on NO query engine (C29). A relation
projects a `check.Model` into tuples and registers with `facts.RegisterRelation`; an engine that
answers questions over those tuples imports `core/facts`, never the reverse. That is what keeps
datalog (`core/query`, over jaala) one query shape among several rather than the primitive, since
`check.Spec` answers per-entity questions with no fact base at all, and a path question is a shape
datalog cannot express (issues 374, 518). A relation vocabulary is a composed VALUE
(`facts.DefaultRegistry`, the twin of `check.DefaultCatalog`), so the package globals are only an
append-only registration buffer and every READ goes through a `*Registry` a caller holds.
**Installing no relation catalog still builds and still runs**, leaving the fact base empty so every
datalog rule reports clean; `Registry.Installed` is what separates that from a query that matched
nothing. **That distinction matters at INIT time**, because `stdlib/profiles` compiles its built-in
profiles in an `init()` that runs before any relation catalog has registered, so anything
validating a query against `facts.DefaultRegistry()` at construction sees an EMPTY registry and
must stand its vocabulary checks down rather than call every relation unknown (agni issue 540).
`core/review` names no query syntax either, because a manifest's inline query compiles through a
registered `review.QueryCompiler` (`stdlib/reviewquery` is the datalog one), so no core package
outside `core/query` knows a query language.

The **root `agni` package** is the composition facade and an embedder's entry point, and `agni.New`
returns an `Engine` holding one composed `*check.Catalog` and one `*facts.Registry`. Compose through
it rather than calling `check.DefaultCatalog` at a call site. There are FOUR global registration
points (`check.RegisterBuiltins`, `facts.RegisterRelation`, `check.RegisterSource` for the `dl` suite,
`review.RegisterQueryCompiler`) and three fail SILENTLY when a binary misses one, which is how the
overlay example ran for months with zero built-in rules. `New` refuses an empty fact base and an
uninstalled built-in catalog, and reports the two legitimate absences through `Warnings()`. Options
take VALUES, never paths (C22), so `profiles.LoadDir`/`intent.LoadFile` stay in the caller. Note the
registration check asks `check.BuiltinRules()` rather than measuring the composed catalog, because
`stdlib/profiles` registers from an init and a program missing the built-ins still composes a
NON-EMPTY catalog.

`service/` is what an EMBEDDER composes against, which is why it is not under `internal/` (C13).
`ProjectStore` and `ConfigResolver` are the two ports a private deployment implements to serve its
own designs and its own config. `ConfigResolver` turns the URIs any `AnalysisConfig` names, a
project's or a request's, into a `ResolvedConfig` (rule sources, a param provider, symbol paths). It
moved out of `internal/service` with `artifact` in tow, because `artifact.URI` is in the loader-port
signatures. The rule that keeps it free of file I/O is unchanged: no `os`, no `path/filepath`, no
transport imports, and every I/O concern arrives as an injected port. `internal/projects` (the
directory-walking `FSStore`) stays internal on purpose, because everything true only of storing
projects in DIRECTORIES lives behind the port.

`intake/` produces a sanitized design summary, and its confidentiality
guarantee is **structural**. The `Skeleton` type has no field that can hold a net name or a
connection, so an intake summary *cannot* express the confidential parts of a design. That turns a
policy ("do not paste a net name") into a property of the type ("there is no field for one"). Keep
it that way. Adding a free-text field to `Skeleton` would quietly dissolve the guarantee.

## Build, test, and the CLI

- `make build` / `make test` / `make agni` / `make install`. **`make agni` writes `bin/agni` and does
  NOT update the `agni` on your PATH**, which `make install` puts in `~/go/bin`. Verifying a CLI fix
  with the wrong one reports the bug still present and reads as the fix not working.
- **Nothing here takes a private path, and a private workspace must never have to reimplement a
  target.** Two mechanisms carry someone's own designs in. Tier-1 config (mounts, symbol paths,
  native tools) belongs in an `agni.yaml`, which the CLI finds by walking up from the working
  directory and then in `~/.config/agni/`, so `agni check mount://corpus/...` works from anywhere
  with no flags (see `cmd/agni/envconfig.go` and the tier boundary it guards). Everything else is a
  variable on a target: `EXTRA_MOUNTS` and `OVERLAY_FLAGS` on `serve`, `DESIGNS` and `OVERLAY_DIR`
  on `dockserve`, `NATIVE_DOCKER_MOUNTS` on `natup`, `DATASHEET_DIR` on the datasheet targets. A
  workflow that only exists as a wrapper in someone's local Makefile is a missing target here.
- **A flag wins outright over `agni.yaml` rather than merging**, so a Makefile default that passes
  `--mount` shuts the file out. `make serve MOUNTS=` is how you hand the mount table back to it.
- **`agni.yaml` wants a `web_dir` or `--server self` cannot run outside a checkout.** The lookup falls
  back to a relative `./web`, so a run from a design folder fails with `--web-dir "web" is not a
  directory`, and since agni 637 it fails BEFORE writing the artifact rather than after. The four keys
  are `mounts`, `symbol_paths`, `web_dir`, `native_tools`, and `web_dir` must name a BUILT bundle,
  because `checkWebAssets` stats the viewer's templates and `static/*.js` before the listener opens.
- **`agni serve` with NOTHING naming a web dir and no `./web` serves the API alone** (agni 735),
  which is what a `go install` binary has, since the built JS is not in the Go module. An absent dir
  is a choice and a broken one is a mistake, so a NAMED dir that is wrong, or a `./web` without its
  bundle, still fails. `open` and `--server self` always need the viewer. The datasheets workbench is
  NOT served by `agni` at all since agni 744; it is `agnids serve` (`make dsserve`), and
  `agni serve --datasheets-url <url>` is what makes the landing page link to it.
- **There are two images, published by one release tag**: `Dockerfile` (`ghcr.io/panyam/agni`, the
  engine, viewer and symbol libraries) and `Dockerfile.agnids` (`ghcr.io/panyam/agnids`, the
  workbench and the docling stack). `make image` and `make dsimage` build them. The `images` workflow
  builds and smoke-tests both on a PR that touches what they are made from, through
  `hack/image_smoke.sh`, which `release` also runs against what it published. The agnids image
  prefetches its models by RUNNING pdf2doc over a synthetic page, because docling reads the Hugging
  Face cache and `docling-tools models download` fills a different one (agni 786).
- **The web code is ONE pnpm workspace with one lockfile at the root**: `web/` (the viewer),
  `datasheet/web/` (the workbench `agnids` serves, with pdf.js) and `web-shared/` (plain TS both
  import as `@agni/web-shared/<name>.js`, with no npm dependency of its own, so neither page pulls
  the other's libraries in through it). Run `pnpm install` at the ROOT. `make ui`, `make web-test`
  and `make proto-web` cover all three. The workbench's TS generates from the whole buf workspace
  and the viewer's from `protos/` alone. The workbench has its OWN `BasePage.html`, holding only the
  thirteen rules it uses of the viewer's 50KB of styles, so `datasheet/web` deploys on its own; a
  computed-style comparison of all 86 workbench elements, before and after, was identical.
- **An unknown key in `agni.yaml` is a hard ERROR, and the file is shared by every lane, every
  released binary and the container image.** So a new key goes in only after every reader on the
  machine understands it. The obvious probe is a trap, because `agni <cmd> --help` short-circuits
  before the config is loaded, so it accepts anything and proves nothing. Probe with a command that
  reads a design, and run the same probe with a deliberately bogus key first as a positive control.
- **A project DISCOVERS its analysis tiers, so a flag naming one is redundant and dropping the flag
  does not turn it off.** `internal/projects/descriptor.go` defaults `conventions.yaml`, `profiles`,
  `params`, `review.yaml` and `lib`, and `FSStore` composes each one it finds. `lib/` is the
  project's own derived relations, which every query surface and a review manifest's inline queries
  read through `Overlay.Registry` (agni 773, 779). That has two consequences, and
  each has cost a bug. Before adding a tier flag to a command, check whether the name is already
  defaulted, because `--profile-path` naming the project's own directory double-loaded every
  profile rule (issue 450) and `--params` naming its own is merely redundant. And to reach a tier's
  "off" state you must MOVE THE DIRECTORY ASIDE, which is why rungs 4, 5 and 6 open with
  `mv <tier> <tier>-off`; rung 6 shipped a before/after whose two captures were byte-identical
  because both ran with the corpus in place.
- **Between a project tier and its flag, the project wins (`Overlay.SpecsOr`).** That is the
  opposite of the mount rule above, deliberately, because a project owns its parameters the way it
  owns its profiles. A command that reads a tier from its flag alone is the bug shape, and `intake`
  did, so inside a project its datasheet-gap section was absent rather than empty (issue 474).
  `readDesignWithConfig` returns the overlay the read already composed, which is where a non-service
  command should get a tier rather than resolving the project a second time.
- `make natrender FILE=... OUT=...` and `make natopen FILE=...` drive the native tools over the
  `natup` container. Both take paths INSIDE it, so they must fall under a `NATIVE_DOCKER_MOUNTS` dir.
- `make setup` builds the docling venv the datasheet tooling runs in, then `make pdf2doc`,
  `make pdf2doc-all`, `make datasheets-status` and `make dsserve` work over `DATASHEET_DIR`. The
  tools live in `datasheet/tools/`, so those targets `cd datasheet` first. The venv is found by
  lookup (repo-local `.venv`, then the parent directory's), so worktrees sharing a root share one
  env instead of each carrying gigabytes of torch.
- `make -C docsite preview PAGE=learn/03-why-every-chip-needs-capacitors` folds one built page into a
  self-contained HTML file, for reviewing a branch before it merges. Use it rather than
  `make -C docsite gh-pages`, which is DEAD because Pages serves the `docs.yml` workflow artifact
  (`build_type: workflow`), so force-pushing that branch changes nothing.
- `make -C docsite figures` re-renders the schematics `learn/` embeds. Outside the gate, because a
  render depends on the engine build and nothing checks its output for staleness (agni issue 453).
  `make tutorial-runs` is no longer outside it, because `tutorial-runs-check` regenerates every
  capture and fails on any difference, and it is in `testall`. A capture's stamp hashes the spec and the
  fixture but NOT the engine, so regenerating is the only way to see engine drift. **Editing any
  tracked file inside a fixture restamps every capture over it**, 61 of them for
  `examples/tutorial-project`, so commit the fixture edit, then `make tutorial-runs`, then check the
  diff is stamp lines alone. A fixture under `tools/samples/` has nothing tracked and hashes
  `hack/samples.pin` instead (agni 682).
- The CLI's commands include `agni stats|check|diff|render|query|trace|review|serve|open <file>`
  and `agni params <mpn>`. `open` serves ONE design and prints its URL, minting the mount itself;
  `serve` takes `--mount` per folder and `--web-dir`. The reader is chosen by extension
  (case-insensitively), with `.xml`/`.sch` sniffed by root/header. `--symbol-path <dir>` resolves
  external symbol files and searches each dir's SUBTREE, so a dir can be a library root.
- **A command's `--format json` is protojson of that command's WIRE MESSAGE** (C31), so a script
  reading the CLI and a client reading the rpc parse one shape. `agni intake` is the one declared
  exception and its reasoning is on `intake.Skeleton`, because that type's confidentiality guarantee is
  structural and a proto twin would have to carry it into a file people edit for other reasons.
  `TestEveryJSONFormatEmitsAProto` enforces it and reads SOURCE in `cmd/agni`, so it cannot see an
  encoder reached through a helper in another package; that is why `report.TableJSON` was deleted
  rather than pattern-matched.
- **`agni trace <design> --from U7.3 --to U12.4` follows a signal through the series parts between
  them** and prints the route, the nets, and the probe points on each. `--render <file.svg>` draws it,
  on the design's own schematic where it has one and on an auto-layout where it does not, saying which.
  `--server` mints a link that re-asks the question in the viewer, and unlike a verdict link it
  carries the QUESTION, so it needs no content hash. Three outcomes stay apart: a route, no route
  within the radius, and an endpoint naming nothing the design has, which exits non-zero because a pin
  spelled wrong and two pins genuinely unconnected are opposite problems.
- **There are two HTML reports, on different axes, sharing one stylesheet.** `check --format html`
  is the verdict report, rule-major, and implies `--verdicts`. `review --format html` is the
  checklist, question-major, in the manifest's order with every finding per item. Both take
  `--server` and share `core/report/style.css`. **Against a REMOTE server a link is only emitted for
  a mount you DECLARED**, because a mount minted for one run means nothing on a server not started
  with it, and that server is then asked through `ListMounts` whether it serves the name from the
  same root; a withheld link always states its reason, on stderr AND in the report itself, because
  the reader who needs it is the one opening the saved page later, by which time the terminal is
  gone (issue 626).
  **`--server self` removes the question instead of answering it** by reading the design and
  serving it from one process, so a minted mount is as linkable as a declared one, and it blocks
  until Ctrl-C because the links live exactly as long as the server does. `self:PORT` fails on a
  taken port rather than moving. **A link names the design's declared ENTRY whatever you pointed the
  command at, and carries the revision it was read at**, which the viewer checks before it draws.
  **A verdict link also carries `rule=`**, so the viewer resolves it by running ONE rule rather than
  the catalog (21.78s to 1.95s on a 3980-component
  board, and paid per click, because each click from a report is a fresh page load). The rule is
  minted rather than recovered, since `VerdictID` is generated and never parsed; a link without it
  still resolves by the slow route, which is what keeps saved reports working. A TRACE link needs no
  such hint because it carries the question rather than a conclusion. Semantics and the two ways the
  halves used to disagree are in `guide/checks-and-reports.md`.
- **`review --coverage` renders as markdown ALONE and refuses an explicit `--format`.** The two are
  different axes and the render switch tests coverage first, so `--coverage --format html -o page.html`
  used to write a markdown table into a file named `.html`. If you want a coverage page, the per-item
  `--format html` already carries the same rollup in its header band.
- **`agni params <mpn>` prints the RECORD a query answer cannot reach**, and it needs no design,
  because a spec library is not one. The datalog relations carry what a query can BIND; the conditions
  a value holds under, the pin bindings, the full provenance and the verification state are read off
  the `PartSpec`. `--params <dir>` names a corpus, `--design <path>` lets a design's PROJECT supply
  one (and the project WINS, per `Overlay.SpecsOr`), `--format json` emits the bare `PartSpec`. A
  parameter someone verified reports `stale` when the corpus moved to a later revision, naming BOTH
  revisions, though staleness is decided on the content hash and NEVER on the printed one, so the
  two strings are for the reader (`DECISIONS.md`, "A document revision is recorded for the reader,
  and never compared"). **A workbench draft is a DRAFT that no check reads**, saved
  unvalidated on purpose, keyed by MPN and citing its datasheets, in the corpus store
  (`<corpus>/drafts/<MPN>.draft.json`, `agnids serve --corpus`); `PublishDraft` or `agnids publish
  <mpn> --corpus <dir>` validates it into `<mpn>.textproto` (agni 209, 747, 749). Reading drafts in
  `LoadSet` looks like the fix and is the bug, because a half-finished transcription would fail the
  whole all-or-nothing load. The workbench saves nothing until an MPN is confirmed. **Publishing
  and the corpus index belong to the datasheet service** (`datasheet/corpus`): publishing writes
  `corpus.index.json` (MPN, file, hash, a generation) beside the specs, `agnids index [--check]`
  rebuilds or verifies it, `agnids serve --corpus` refreshes it at start, and all of them build from
  `param.LoadCorpus`, the walk `LoadSet` makes, so the index and every eager load agree. The files
  are the source of truth; no check reads the index. **The engine reaches a SHARED corpus only
  through the contract's `PartSpecService`** (`protos/agni/v1/param/service.proto`): `agnids serve
  --corpus` answers it from the index, and `agni serve --params-url` reads it through `param.Remote`.
  `Lookup` returns no error, so a remote provider is a `param.Prefetcher` and `service.BuildModel`
  fetches the design's MPNs there, where an unreachable corpus is `ErrUnavailable` instead of every
  part reading as unseeded. A new model-building path that skips `Prefetch` gets no datasheet tier.
- **`emit --format edif` writes for a reader that is NOT ours, and that is a stricter target than the
  round trip.** Our reader resolves references after parsing the whole file, accepts any atom as an
  identifier, and reads a port reference as a pin designator when nothing maps it, so a writer leaning
  on all three round-trips perfectly and produces a file nobody else can open (agni issues 563, 580).
  Four rules follow: libraries before the design node, a `portRef` naming the cell's PORT with a
  `portInstance` table carrying the pin, identifiers holding no character a reader rejects, and
  everything the file references also declared, including cells and a top cell a board read gives no
  source for. Semantics are in `guide/cli-reference.md`; the properties are asserted over the emitted
  TEXT in `readers/formats/e2e_edif_conformance_test.go`, because a re-read goes back through the same
  forgiving reader and agrees with the writer whatever either does. `build/evidence.md` carries the
  out-of-tree oracle that says which properties are the right ones.
- **`net.reaches` answers whether and `net.route` answers how.** `net.route(from, net, path)` is the same walk with
  the route bound as a rendered string (`VBUS -> [R5] -> VBUS_F`), so a connectivity answer carries
  its own evidence. It holds for exactly the pairs `net.reaches` holds for, so a route never ENDS on a
  rail; `agni trace` is the pin-to-pin form that does. ONE route per pair, the BFS tree path, so it
  cannot speak about parallel paths. **The rendered string is a CONTRACT**, owned by
  `model.RenderRoute` and nowhere else, because it was written twice within a week and the copies
  agreed only by luck. DECISIONS.md carries the reversal that admitted a path as a column and what
  was accepted with it.
- **`query` emits five formats and two of them are DOCUMENTS.** `--format text|csv|json|markdown|html`
  plus `--title`. markdown and html carry the title, the design and THE QUERY above the answer, so a
  saved view states the question it answers; csv deliberately carries no preamble, because its first
  row has to be the header something binds to. An empty result is never an empty artifact. The
  renderer is `core/report.Table`, which is also where the csv escaping for every command now lives,
  having moved down out of `cmd/` rather than being copied a third time (agni issue 380).
  **`--set <file>` asks a named SET of queries over one read** (agni 729): one document with a
  section per query, csv refused, and a non-zero exit AFTER writing when any query failed.
  `RunQueries` answers each query exactly as `RunQuery` would, because both call the same `answer`
  helper in `service/query.go`.
  **A value goes into a query as a BINDING, never as spliced text** (agni 793): `--bind name=value`,
  `RunQueryRequest.bindings`, a set's `bind:`. The viewer's search and click presets bind too
  (`EntityQuery.binds`, `SearchQuery.bind`/`pattern`) and show the values as chips, so a served
  preset's text never changes. A binding the goal does not use is refused by the engine.
- **Aggregation reduces BINDINGS, not values, unless you say `distinct`.** `count/min/max/sum/list`
  group by the projection's plain columns; `count(distinct ?x)` reduces the SET of values instead.
  The trap is that a goal joining two things yields one binding per combination, so on
  a net carrying 7 test points and 20 capacitors `count(?tp)` is 140 and `count(distinct ?tp)` is 7.
  `distinct` is uniform across every function, `list` included, deliberately, because an
  implicitly-distinct `list` would put `count(?r)` and `list(?r)` in one projection disagreeing
  about what the group holds. **`having` filters the GROUPS after the reduce**, which a goal
  comparison cannot do, because before grouping there is nothing to count:
  `... => ?p having count(distinct ?n) = 1`. An aggregate may be filtered on without being
  projected, which answers with the subjects rather than the tally (agni issue 613). A derived
  relation is the other route to a distinct reduce, by projecting the extra variable away before the
  group forms. **An aggregate-only projection always answers ONE
  row**, as SQL's `COUNT(*)` does, so a count over nothing is `0` rather than "no results", while a
  grouped projection over nothing stays empty (agni 726). That is why `RuleFromQuery` refuses a
  finding query whose subject is not a plain projected column, since every row it returns becomes a
  finding.
- **`diff --rename-approx` is OFF by default**, so a net that was renamed AND changed reports as New
  plus Deleted unless you ask for it. Deliberate, because the pass ASSIGNS a best match rather than
  recovering a fact. It is also a false-finding shape, since a run without the flag reads as "we
  detect no approximate renames", which is how one got written up as an engine gap before the flag
  was noticed.
- The toolchain is Go 1.26.4 and `buf` 1.61. **Both protoc plugins are pinned as `tool` directives in
  `go.mod` and invoked via `go tool`**, so their versions are data rather than something to match by
  hand. Only `buf` itself has to be on your PATH.
- **A command that reads a design goes through `readDesign` (or a service), never a bare
  `newLoader().ReadDesign`.** That function is where a design's PROJECT config enters the read for the
  six commands no service mediates (stats, diff, emit, render, intake, profilediag), and net roles are
  resolved once at ingestion, so a read that skips it silently uses the built-in naming vocabulary and
  none of the project's declared symbol libraries. All six bypassed it until agni issue 228, which is
  why it is one function rather than six.
- **A design's tiers come from `service.SourcesFor`, and naming the ENTRY is naming the design.** A
  descriptor's `companions` supply the tiers the entry cannot: a schematic export for sheets, a board
  for copper. `NetlistURI` always stays on the entry, and only the other tiers move. All three
  spellings of one design (the folder, the entry filename, a declared companion) now resolve
  identically. Until agni issue 528 they did not, because naming the entry skipped companions entirely,
  so a design whose faithful geometry lived in a companion drew its auto-layout under one spelling and
  its real schematic under the other. `--as-named` is the opt-out. **An UNDECLARED sibling is still
  read exactly as named**, which is why companions are declared file by file rather than
  inferred. A later revision of the netlist sits in the same folder and is a legitimate analysis
  source, so inferring would turn a diff of two revisions into a diff of one against itself.
- **A `.eds` is dual-capability, and its netlist is NOT the `.edn`'s.** An EDIF schematic export
  registers both a `Design` and a `Geometry` reader, because it carries nets joining portRefs in the
  same grammar, so every tool will parse it as a netlist without complaint. It counts DRAWN instances
  and per-sheet segments rather than resolved nets. Measured on one real board holding both files:
  3980 components and 1617 nets off the `.edn` against 5219 and 4572 off the `.eds`, so nets inflate
  by 183%. A design that ships only a `.eds` can be rendered and queried, and its counts must never be
  compared against a design read from a netlist.
- **KiCad's placement and bus rules are pinned against `kicad-cli`, and three of them are not what you
  would guess.** A symbol is rotated and THEN mirrored, where the shared transform composes mirror
  first, so a mirrored placement at 90 or 270 swapped its two pins onto each other's nets until agni
  issue 577; the fix is the INVERSE angle rather than a reordering, which is why the renderer needed
  no change. A bus VECTOR is spelled `[first..last]` and only that, so a KiCad label written
  `DATA[1:0]` is a plain scalar name (xschem and gEDA do use that form). Two buses WIRED together
  share the members whose names match, while a bus crossing a SHEET PIN pairs off by bit position,
  ascending index. All of it is in `architecture/net-solving.md`, and **none of it is visible in a net
  COUNT**, because a pin swap moves one connection out of a net and another in, so one demo board
  read 47 nets against KiCad's 47 while 19 were wrong.
- **A locator records the path WITHIN the design's mount, never the host path.** Readers stamp
  `ir.Provenance.SourceFile` with whatever path they are handed, so the rename happens once after the
  read, where `Loader.SourceName` maps a path to the name provenance should carry and `relocateSources`
  walks the message tree by reflection to apply it to every locator in both `ir` and `geom`. A host
  reading through `Loader.FS` sets nothing, because an `fs.ValidPath` is unrooted already; the CLI
  sets it from its mount table. **A new output format inherits this and a NEW HOST does not**, and one
  that opens files by absolute path and leaves `SourceName` nil publishes the machine that ran it,
  which is what `--results-out` stored until agni issue 501. Beware also that a query's citation list
  is SORTED, so anything that changes the source string reorders committed captures.
- **A reader records a part number wherever its grammar puts it and NEVER promotes it itself.**
  `classify.StampMPN` is the shared ingestion pass that fills `ir.Component.mpn`, from the component's
  own aliases first and then from `ir.PartType.mpn`. Both halves used to live privately inside the
  EDIF reader, so EDIF resolved part numbers and no other format did. Telesis records it on the PART
  TYPE while every consumer read the COMPONENT, so `component.mpn` came back empty for every
  component of every `.tel` design, silently disabling the whole datasheet tier on that format
  (agni issue 519). A new spelling goes in `classify.MPNAliases`, never in a reader.
- **Some boards are FETCHED, not committed.** `make samples` pulls a pinned tarball from
  `panyam/agni-samples` into gitignored `tools/samples/`, and `testall` depends on it. Those designs
  are other people's, under their own licences, which is what keeps this repo uniformly Apache-2.0.
  `hack/samples.pin` holds the version and a checksum per artifact. **There is no offline escape hatch,
  by design.** Every failure path exits non-zero, and a test reading the corpus fatals rather than
  skips, because a corpus that silently fails to arrive turns its tests into tests that pass over
  an empty set. The gate takes `samples-oracle`, the 19MB both-views corpus, because the cross-view
  tests read a board file and a test whose fixture is absent skips rather than fails. One stamp PER
  ARTIFACT, so `make samples` and `make samples-oracle` compose instead of deleting each other's work.
- **After ANY proto change run `make proto` (Go), `make proto-web` (TS) AND `make proto-py` (Python).**
  `make proto-check` fails the gate on any of them being stale. The Python half uses buf's REMOTE
  plugins pinned by version, so it needs network, and `pyproject.toml`'s `protobuf>=` floor moves with
  the plugin version. **The protos are TWO buf modules in one workspace** (the root `buf.yaml`): the
  engine's `protos/` and the datasheet producer's `datasheet/protos/` (`doc`, `derive`, `candidate`,
  `dsapi`), which generates into `datasheet/gen/go` and imports `param.proto` and `workspace.proto`
  through the workspace. `make proto` runs both. The datasheet half is generated from the repo ROOT,
  because its `go tool` plugins resolve against the go.mod of the directory buf runs in, and only the
  root go.mod pins them. The TS step reads the whole workspace (the workbench still builds from
  `web/`), and the Python client reads `protos/` alone, since it is the engine's client.
- **`render --report` prints to stdout and IGNORES `-o`**, which is the same axis confusion as
  `review --coverage`, since the report replaces the drawing rather than being a format of it, so the
  flag that names the drawing's file has nothing to write. It exits 0 having written nothing.
- **`-o/--out` writes the `--format` output to a file** on `check`, `review`, `trace` and `query`,
  `-` meaning stdout and being the default, so nothing that omits it changed. Distinct from
  `--results-out`, which writes the check-result DOCUMENT `agni results` re-renders. The written-file
  note goes to stderr, so `-o` composes with a pipe.
- **The `--verdicts` list caps NON-ACTIONABLE rows at twenty per rule**, states the elided count, and
  never elides a fail or an inconclusive. One sample board went from 1977 lines to 799 with all 150
  actionable rows intact. A long list is a worse read, but a hidden failure is a wrong answer. csv and
  html are untouched, because a spreadsheet and a scrollable page have no reason to fold anything.
- **When you build a feature, ship an example** (CONSTRAINTS C10; how-to in `examples/CONVENTIONS.md`,
  and `examples/tutorial-project/README.md` for the fixture the docsite tutorial runs on).
- **`AGNI_EXAMPLE_DESIGN` points every example at a board this repo cannot carry.** Each example asks
  for its design through `common.AskPath`, which defaults to a bundled synthetic fixture; the variable
  replaces that DEFAULT, so the prompt still shows it, a typed path still wins, and
  `--non-interactive`, `--record` and `--replay` pick it up, which typing a path at the prompt
  cannot do. A blank value is not a value. **A demo over someone's real board is an example driven
  this way**, not a script of its own: `design-review` is the checklist walk, `dft-coverage` the
  coverage one and `whole-enchilada` the tour, so orchestrating beats in bash duplicates demokit and
  produces no recording. **Run an example over a REAL board before trusting it**, because the
  fixtures are small enough to hide scale bugs, and printing every ref-des in a bucket read fine at
  three parts and buried the screen at 531 (agni issue 644).
- **`AGNI_EXAMPLE_REVIEW` is the same idea for a review manifest**, which `design-review` needs beside
  the design, since a team's checklist is as unshippable here as their board. **Both variables must be
  cleared in a package's `TestMain`**, because they replace the defaults the tests assert against, so
  a developer who exports either to drive a walk over their own board cannot run the gate and the
  failure prints their path into the log. That was green in CI and red on the machine that had them.
- **The same is true of a personal `~/.config/agni/agni.yaml`.** A test that runs `rootCmd()` or the
  binary reads it, and its mounts or `web_dir` become part of the run. A valid `web_dir` there turned an
  `agni open` refusal test into a real server that hung the suite, and the Python client's tests saw
  its mounts. Point `HOME` and `XDG_CONFIG_HOME` at a temp dir and run from an empty working directory
  (`TestOpenStillRequiresTheViewer`, `clients/python/tests/conftest.py`). CI has no such file, so this
  is also green in CI and red on a developer's machine.

## Engine gotchas

**A declared pin map is the ninth intent form, and it compiles to FOUR rules.** `io_map` on a design's
`intent.yaml` says which net lands on which pin of which device. Three rules ask whether the design
kept that promise (`io-map-pin-mismatch`, `io-map-net-absent`, `io-map-far-end`) and the fourth,
`io-map-coverage`, inverts the question by taking the NETLIST as its considered set, so it reports
how much of the design the map never mentioned. That number usually matters most, because a map
declaring two hundred of sixteen hundred nets leaves fourteen hundred UNEXAMINED, which is not
clean. An undeclared net is `not-considered` and never a fail, and rails stay in the denominator
with their reason saying so, because excusing them is the tool deciding which absences are
acceptable. `function` is carried and NOTHING READS IT (issue 667, which is the ALTERNATE-function
table; issue 188 is the pin function table and is done); every verdict on a row declaring one says
so outright.

**Comparing an identifier across two documents goes through `core/ident`, never `==`.** A pin map is
authored in the datasheet's vocabulary and a netlist answers in package designators, and both carry
zero-padded indices, invisible characters pasted out of a PDF, and cells naming several functions at
once. Measured against a shipped in-house checker on a real board, EVERY warning it produced was a
string-comparison artifact and none was a design defect. `Canonical` is the one canonical form, used
on BOTH sides; `\s` does NOT match U+200B, which is the defence that looks right and is not.
**A pin name is compared loosely and a NET name is not**, because a vendor table is inconsistent with
itself while a net name is the design's own identifier. **A caller comparing many against many
builds a map keyed on `Canonical` rather than nesting `Compare`**, since two hundred rows against
sixteen hundred nets is 320,000 comparisons however fast one is.

**`pin.name(ref_des, pin, name)` is the pin's FUNCTIONAL name**, beside the package designator every
other pin relation keys on. Both have always been in `ir.Pin` and only the designator was projected,
so a question asked the way a datasheet asks it had nothing to read. It is projected VERBATIM, so
canonicalizing belongs on the comparison and never at the projector.

**`make oracle` is a separate suite and is NOT in the gate.** It cross-checks the KiCad reader
against real boards, comparing the pin-to-net PARTITION against each board's own `.kicad_pcb` rather
than net names (auto-named nets differ by tool) or counts (compensating errors cancel). It asserts
`readers/kicad/oracle_corpus.baseline`, a committed list of the nets we still get wrong;
`AGNI_ORACLE_UPDATE=1 make oracle` rewrites it. Out of the gate because it needs the 19MB both-views
corpus, not the 3MB the gate fetches.

**`make browser-test` is IN the gate**, and needs a Chromium on the machine
(`cd web && pnpm exec playwright-core install chromium`). It drives a real browser against a real
server for the handful of assertions that need layout, because jsdom has none and a panel can be
present in the DOM and invisible to a reader, which is how v0.2.0 shipped a viewer whose query
surface booted hidden behind the Trace tab. It was outside the gate until PR 629. Read
`docsite/content/build/the-gate.md` for what belongs in it, and `build/evidence.md` for the two ways
a layout assertion passes while proving nothing.

**A module that BUILDS is not a module that is tidy**, and the gate now asks both. `tidyall-check`
runs `go mod tidy` over the root and every example module and fails on any difference (~5s, near the
front so "updates to go.mod needed" is diagnosed there rather than as a later `examples-test`
failure). Eleven example modules had drifted before anything looked, because an untidy module keeps
building until some later change needs a requirement it never recorded. `make tidyall` fixes it.

**`make testall` is the full gate, and CI runs exactly it.** Read
`docsite/content/build/the-gate.md` before trusting a run, since the gate has three ways to be
misread (a wrapper reporting its own status rather than make's, which turns red green, a
commit-first ordering rule, and a per-clone `pnpm install`), one that makes a fine tree read RED (a stray
`agni serve` on :8080 fails three verdict-link tests, so reproduce against unmodified `main` before
reporting a regression), plus what a run leaves behind and the generated-code rules.
**`tutorial-runs-check` regenerates captures and does not read the prose quoting them**, so a
tutorial can cite numbers a change moved and the gate stays green.

**A design's tiers are resolved ONCE, in `service.ResolveSources`, and both surfaces call it.** For
most of 2026 they did not, because `SourcesFor` alone was shared, and the half above it, deciding
whether a ref names the design, its entry, a declared companion or an unrelated file, lived only in
`cmd/agni`. So the CLI attached the schematic companion a descriptor declared and the server did
not, and one design read two ways gave 3980 components on a computed layout or 82 real sheets with
no counts, depending on which name you typed. Nothing errored, because "no geometry" is a
legitimate thing for a design to be. Four symptoms followed and each read as its own defect:
findings with no sheet badges, query cells reporting `LOCATE_REASON_NO_GEOMETRY`, `trace --render`
writing the first sheet whatever the route crossed, and every minted link opening on an
auto-layout. C32 is the rule, agni 656 the fix.
`as_named` rides the wire because the CLI is itself a client of these services, and resolving
unconditionally overrode its own flag.

**656 left one call site and it took another ticket to find.** `CreateReview` never resolved tiers,
so a review scored in the browser read no board while the CLI read the declared one, and board-tier
checklist items answered `not-applicable` against the CLI's `fail` (agni 646). The MECHANISM lets two
surfaces disagree with no resolver involved. Given no board,
`BuildModel` asks the loader for one at the DESIGN's own URI, and the CLI's `localLoader` resolves a
descriptor inside `Board` while the served `osLoader` reads exactly what it is handed. So a loader
can attach a companion the service never resolved, which makes "the CLI reads it" evidence about the
loader rather than about the request. Every analysis rpc now calls `TierURIs` first, and a new one
that reads a design owes that call.

**A trace answer carries where each net and endpoint is DRAWN**, per net rather than one sheet for the
answer, because a route crossing three sheets is when a reader most wants to choose (agni 657). An
ENDPOINT resolves by its PLACEMENT and not by its net, because it is a pin on a part, and resolving
by net first put a route on a sheet carrying the middle net and none of the parts. It is filled on a
no-route too, since the two nets that fail to join are drawn somewhere and that is the picture
someone goes looking for; empty then means drawn nowhere rather than nobody looked. The CLI takes
the sheet off the response rather than choosing, which is C32 at one more call site.

**A rule's `Eval` takes the caller's context, and a run stops when the context is done** (agni 795).
`check.Run`, `RunVerdicts` and `review.Run` return `ctx.Err()` rather than the findings so far, which
would read as a clean remainder; `RunBackground` is the context-free form for tests and one-shot
tools. Every datalog evaluation reads its WORK BUDGET off the same context through
`query.EvalOptions(ctx)` (agni 792), so a new `Eval` call site that omits those options escapes a
served budget silently. `agni serve --query-budget` enforces one, OFF by default, and
`--query-budget-warn` logs each costly query with a suggested budget; a request's `work_budget`
narrows the server's and never raises it. A stopped query is `resource_exhausted` and a stopped rule
one inconclusive finding.

**A rule proves a PASS with entities, not only with prose, and there are two mechanisms.** A Go-walk
requirement returns its hops (`check.PullUpVerdict` emits every resistor and net as ordered context
with a role). A query-backed one needs `Domain.Evidence`, a third goal run over the passing set whose
rows project through the rule's own `ContextVars` (agni 662). It is a third goal rather than a wider
domain because the domain is the CONSIDERED SET, passing and failing alike, and a
failing subject has no proof to name, so no single query can both enumerate the failures and bind the
evidence. On the ESD requirement the evidence is a SECOND head (`esd_by`) rather than a widened
`esd_ok`, because the finding goal NEGATES `esd_ok` and a negated atom must stay unary.

**A thermistor is a `thermistor` AND a `resistor`**, the family shape `ferrite`/`inductor` already
uses (agni 627). It classified UNKNOWN before, which emits no `component.class` row at all, so it fell
out of every class-scoped rule and query silently. **A project can EXTEND a class the engine ships
and cannot declare a new one.** `lexicon.class` takes `patterns` and ref-des `prefixes` keyed by a
shipped class, and a name the engine does not know is a load error. Which names a config may use is
derived from `model.ComponentClasses`, and a test reading the const block holds that list to it. The
hand-kept copy it replaced lacked `thermistor`, `zener` and `ideal_diode_controller`, so a project
could not extend the class 627 had just added (agni 677). Declaring a new class opens the vocabulary,
and DECISIONS.md says that happens as a registry, once a project asks.

**A device class has TWO evidence tiers and one ordering, and both halves fail silently.**
`ir.Component.device_classes` is a set of TAGS, each carrying a `ClassSource` (agni 710, the second
instance of C9's evidence-tier variant after `ir.Net.roles`). `classify.Stamp` is the convention
tier; `classify.StampClassesFromSpecs` is the datasheet tier, and it is the only evidence that
separates a crystal from a ceramic resonator or names an `ideal_diode_controller`, both of which the
structural path is documented as unable to resolve. **Two orderings inside `Loader.ReadDesign` are
required, and getting either wrong is silent**. The convention pass REPLACES the set, so anything
writing before it is erased, and the datasheet pass joins on the MPN `StampMPN` fills, so it must
run after that. **The datasheet tier runs in TWO places on purpose** (the Loader when the read
carries a corpus, `check.Model` when it is handed one the read did not have), which is safe only
because the pass is additive and idempotent. Adding a THIRD producer is the bug shape, not the
pattern. **`classify.BySpecificity` decides which of several true classes is the headline one, and
`MostSpecific` is its head rather than a second walk**, because the drawing and the model ranked one set
independently for as long as the set had one author, and 710 is what that cost. An unranked vendor
class (`regulator`, which the tutorial corpus really states) sorts BEHIND the keyword class rather
than displacing it. **A second tier must never be the ONLY tier in the set**, because EMPTY is the
signal that tells a consumer to re-derive, which is why `WithParamProvider` runs the convention pass
first on a design that reaches it unstamped.

**A rail-named net is not always a rail.** A regulator's pins are named for the supply they produce,
so `12V_FB`, `12V_SW`, `12V_MODE1` and `12V_VDRV` all match the rail vocabulary and none carries 12V.
Four roles say so and `Model.IsRailNet` subtracts them ONCE rather than per consumer, where before
seven rail-quantified consumers each had to exclude them and only one remembered (agni 679, 680).
The six net vocabularies and when to narrow one are in `guide/naming-conventions.md`; why the two
voltage relations stopped being exhaustive is in DECISIONS.md.

**A variable's entity kind survives a DERIVED relation, and the three rules of that walk are worth
knowing before you widen it.** The engine's `ColumnKinds` (jaala, which `service/query.go` maps since
agni 751) types a projected variable from the catalog relations in the goal and then follows user
rules into their bodies, so a bucket built with negation is clickable
without repeating a catalog atom to re-establish a kind the rule already knew (agni 654). It follows
more than one hop. Rules that DISAGREE about a head position yield a scalar rather than the first one
written, because a column typed from whichever rule came first is wrong for half the rows. A rule
wrapping `entity(?name, ?kind)` stays scalar, since that kind is per-row and a head argument has no
per-row identity to carry it. And a recursive clause ABSTAINS rather than vetoing, so a transitive
closure is typed by its base case instead of collapsing to a scalar.

**There are three ways to read a design and get a confident WRONG answer.** The first two were
hit in one sitting and are silent. A plain `check.NewModel` used to be a fourth, leaving
`component.mpn` empty, until agni 748 made every model join the design's MPNs. Each returns an empty or partial answer rather than an error,
which reads as "the design does not have that".

- **A bare reader skips what `formats.Loader` runs.** The Loader is where the format-neutral passes
  run, so `classify.StampMPN` never fires and every component's `mpn` is empty, which empties the
  whole datasheet tier. This is agni issue 228's shape, and it recurred in `examples/common` (issue 618)
  because the examples are their own Go modules, outside the root build and outside the wiring table.
- **A missing registration blank-import empties the catalog.** `check.BuiltinRules()` returns
  nothing without `_ "github.com/panyam/agni/stdlib/rules/builtin"`, and a verdict sweep then
  reports "0 pass, 0 fail, across 0 rules". Three of the four registration points fail this way;
  see the composition facade note above.
- **A HIERARCHICAL `.edn` reads as a flat design missing most of its netlist.** The EDIF
  reader scopes instance extraction to the design's root cell, correctly (WS1-004), so every
  sub-cell's contents are absent and every rule evaluates over the top cell alone. This one is no
  longer silent (agni issue 707): the reader lists each skipped cell with its instance count in
  `InputDiagnostics.unexpanded_hierarchy`, every CLI read prints a stderr note naming them, `agni
  stats` adds a `not extracted:` line, and `GetDesignResponse` carries the list. It is still WRONG,
  because descending into the sub-cells is the EDIF sibling of issue 134 and has not landed, so
  read the note rather than the counts. It is deliberately not a finding, since being hierarchical
  is a limit of the read rather than a defect.

**Before believing a measurement or a green test, read `docsite/content/build/evidence.md`.** A
negative result needs a positive control, a positive rate needs a precision check, and every new test
needs a red-check. Most of the expensive mistakes here have been correct-looking results nobody could
have falsified. **A test that loops over rows passes over ZERO rows**, so give it a count guard; the
agni 727 red-check found two such tests still green with the projector turned off.

## Wiring, per subsystem

Each of these has a fixed edit-list where missing one edit is silent. Most have a test that catches
it, and where the last column says NOTHING the gap is listed so it is visible rather than
discovered.

| Adding | Edits | Read | Enforced by |
|---|---|---|---|
| A docsite page | 4 (5 for a new section) | `docsite/README.md` | `docsite/nav_test.go` |
| A `learn/` chapter | 4, plus the level-index entries | `docsite/README.md` | `docsite/learn_levels_test.go` |
| A web viewer panel | 4, plus 2 more if it docks | `docsite/content/architecture/web-client.md` | `web/src/composition.test.ts`, `dock.test.ts` |
| A canvas note strip (undrawn, stale-link) | 5 | `web/src/undrawn.ts` and `web/src/stalelink.ts` as the two worked examples | the compiler for the `ViewSink` channel, `composition.test.ts` for the template hole |
| A web page | 6 | `docsite/content/architecture/web-app.md` | its own boot test (one per page) |
| A format reader | see the page | `docsite/content/build/format-reader.md` | see the page |
| A check rule | see the page | `docsite/content/build/check-rule.md` | see the page |
| A query relation | 7, plus `make catalog-docs` | `stdlib/relations/facts/docs/_TEMPLATE.md` | `facts_docs_test.go`, `TestCatalogMatchesSchema`, `catalog-docs-check`, `TestColumnKindsMatchGolden` |
| A library member (derived relation) | 4 (the rule in a `stdlib/lib/*.dl` file with a `#` summary and typed head, its page in `stdlib/lib/docs/`, `columnkinds.golden`, `make catalog-docs`) | `docsite/content/build/library-member.md` | `TestEveryMemberIsDocumentedAndTyped`, `TestEveryMemberHasItsDoc`, `TestDocQueriesValidate`, `TestColumnKindsMatchGolden`, `catalog-docs-check` |
| A glossary term | 2 (the term page, one index line) | `docsite/README.md` | `docsite/terms_test.go` |
| A hand-written `agni …` fence | 1, plus `docCommandCount` | `docsite/README.md` | `cmd/agni/doccommands_test.go` |
| A multi-command `agniRun` block | 1 (`steps:` in the spec, one per command) | `docsite/README.md` | `tutorial-runs-check` |
| A fixture copied from another directory | 1, plus a group in `hack/fixture_copies.txt` | `build/the-gate.md` | `hack/fixture_copies_check.sh` |
| A file added to a capture's fixture directory | 1, plus `make tutorial-runs` AFTER committing it | `build/the-gate.md` | `tutorial-runs-check`, but only once the file is committed |
| A format-neutral ingestion pass | 3 (the pass, the `Loader.ReadDesign` call, `hack/ir_model_baseline.txt` for C19) | `build/format-reader.md` | a cross-format e2e test you write; NOTHING catches a pass that is never called |
| A new EVIDENCE TIER for a derived IR field | 7 (the pass, the source enum value, the `Loader` field, the `Loader.ReadDesign` call, `service.ReadOptions` + `Overlay.ReadOptions`, the `check.Model` call, `hack/ir_model_baseline.txt`) | C9's evidence-tier variant, `architecture/ingestion-and-ir.md` | the C19 ratchet catches the pass; NOTHING catches a tier that reaches one surface and not the other, which is what agni 710 was |
| A host that reads designs | 1 (go through `formats.Loader`, never a bare reader) | `build/evidence.md` | `TestReadCarriesTheIngestionPasses` in `examples/common`; nothing guards a NEW host |
| A hand-authored diagram | 2 (the file in `docsite/figures/`, one `{{ includeFile }}` in the page) | `docsite/README.md` | `docsite/includefile_test.go` |
| An architectural constraint | 3 (the rule in `CONSTRAINTS.md`, a test in one of three homes, a `Verify` naming that test) | `build/the-gate.md`, and `CONSTRAINTS.md`'s own header | the test you wrote, and NOTHING checks that a rule has one |

**The relation row's sixth edit is `service/testdata/columnkinds.golden`**, which snapshots how every
query column types. The table said five for a year and CI caught the miss twice in one week (agni 687,
693). Regenerate with `UPDATE_GOLDEN=1 go test ./service/ -run TestColumnKindsMatchGolden`, and READ
the diff, because a new relation only adds lines and anything else means an existing column
retyped. **The seventh is `dimensionlessNumericRelations` in `stdlib/relations/facts_test.go`**, for
a relation whose number is a bare count, since `TestNoRelationPublishesAnUnlabelledNumber` fails
until the count is listed there on purpose (agni 727).

## Working in this repo

`CONTRIBUTING.md` holds the workflow rules: running several checkouts in parallel (use
`git -C <abs-path>`, never `git add -A`), the PR workflow (verify a push by its exit code, verify
`merged: true` via the API, never `gofmt -w` a directory), the shell traps that have burned
real work, and what agni ADDS to the PR body shape defined by the `start-pr` skill (the circuit and
a hardware primer ahead of the reviewer's guide, which docsite pages the prerequisite block names,
and the fixture-only rule for rendering captures). The general skeleton lives in the skill, so do
not copy it back into this repo.

**There are four ways to be told a thing worked when it did not. All were hit in one sitting, and
all exit 0.**

- **A stray `agni serve` answers instead of yours.** Two measurements were taken against a server on a
  port a newer binary had failed to bind, and the "address already in use" line scrolled past.
  `pkill -f "agni serve"` before believing anything served, and re-check the version the answer came
  from. **That `pkill` also matches the shell running it**, so typed ahead of `make testall` on one
  command line it kills the gate (exit 144, no output). `pgrep -af` first, and remember :8080 may be
  held by something that is not agni at all, such as a code-server, which is not yours to kill.
- **`go test` prints FAIL for a BUILD failure too.** A red-check greping for `^ok` read a package that
  would not compile as a pass, so a test that never ran looked like a test that could not fail.
  Confirm a red-check names the assertion it failed on. A red-check that reverts a DECLARATION rather
  than the behaviour fails the same way: reverting the source file a new test refers to leaves the
  test undefined, which compiles to nothing and proves nothing. Disable the behaviour, keep the
  symbol.
- **An unquoted heredoc runs the backticks in your markdown.** `gh pr edit` then returns 0 having
  posted a body with a block silently triplicated and a flag eaten out of the prose. Quote the
  delimiter, and READ A PR BODY BACK after editing it.
- **`catalog-docs-check` reads `git status`, not a diff against a temp copy.** Regenerating is not
  enough, because the generated file has to be COMMITTED or the gate stays red on a tree that is
  correct. The same is true of `tutorial-runs`.

## Architectural constraints

`CONSTRAINTS.md` holds the enforceable rules (C1 to C34). Read it before proposing changes, and **push
back when a request would violate one**: quote the constraint by name, explain the conflict, and ask
whether to proceed and whether the constraint should change. The point of constraints is that they
survive everyone forgetting why the rule exists. Push back on architectural smell even without a
constraint, and if the direction was wrong, suggest capturing it as one.

**A new rule owes a TEST, never a command typed into the document.** Twenty are enforced by the gate
and thirteen are review questions that say so (`CONSTRAINTS.md`'s header keeps the count). Which of
the three homes a test goes in follows from what it reads: the package graph or the module in the
root `deps_test.go`, one package's own rule beside that package (`service/transport_guard_test.go`,
`core/facts`), a sweep over source in `internal/constraints`. The September 2026 audit is why, and
`build/the-gate.md` carries the full account with the two shapes worth copying: a graph or
single-writer check needs a POSITIVE CONTROL so a pattern matching nothing fails rather than
reading as clean, and an invariant narrower than any
sweep (C24's "never COMPARED") becomes a RATCHET with an allowlist rather than being weakened.

## What does not belong in this repo

This repo is public. A file committed here is world-readable from the moment it is pushed, and
`_hidden/` does not change that. Deleting it later does not remove it from history.

Never commit:

- **Customer or proprietary design data.** No real schematics or board files, no net names, no
  reference designators off a private design, no part numbers off a private BOM, no title blocks.
  This includes screenshots and PR images, which is why before/after captures come from the
  synthetic fixtures rather than a real board.
- **Market, competitor, strategy, or opportunity analysis.** This repo is the engine, not the
  business.
- **Paths into private folders**, private corpus locations, or customer names, including in comments,
  test names, commit messages, and PR bodies.
- **Vendor-licensed material** such as datasheet PDFs and extracted parameter documents. Facts
  transcribed from a datasheet into a fixture are fine, because facts are not copyrightable, but
  cite the document revision and page.

  The fixture-versus-corpus rule is in `docsite/content/architecture/datasheet-layer.md`.

**A generated report IS customer data.** `check --format html` on a real board is tens of megabytes
carrying every net name, ref-des and the design's title, and `-o` makes writing one a keystroke. The
root `.gitignore` covers `report.html` and friends by SHAPE, because one sat untracked in the working
tree for a session before anyone looked. Write them to `/tmp` or outside the repo, and never `git add`
a file you did not author.

**Sanitize at the point of writing rather than cleaning up later.** The engineering content nearly
always survives sanitizing and only the provenance goes. "Customer item 112 mock-failed against a
fake RSTRAP threshold" becomes "a strap resistor's value is a design choice, not a datasheet
parameter", and the second version is the better issue anyway because it states the general rule.

**If a learning cannot be stated without the private context, it does not belong here.** Write it in
the private workspace instead. When a rule's motivation is general EE practice, it belongs here; when
sanitizing would gut it, it does not.

## Writing style for docs and commits

Plain declarative prose. No em-dashes, no marketing cadence, no hype adjectives, and no rhetorical
"The result: X" constructions. Write separate sentences instead. No colon-definitions in prose, nav
lists, headings, or bold noun-phrase labels; fold the explanation into the clause rather than
swapping the colon for a period. None of the words honest, load-bearing, or seam. This applies to
the docsite, commit messages, PR bodies, and code comments. The one colon label the gate REQUIRES is a
reader package's `// Fidelity: ...` line, because `TestC6EveryReaderDeclaresFidelity` greps for the
literal, so a comment pass that folds it into a sentence turns the gate red.

**Prove a comment-only change by comparing code with the comments stripped, not by reading the
diff.** A sweep over hundreds of files hides one edited code line easily. For Go, compare the
`go/scanner` token streams of each file and `git show <base>:<file>` with comments skipped; for
TypeScript, print both through the compiler API's printer with `removeComments`. Give the check a
positive control (a deliberate `var _ = 0`), since one that only adds whitespace changes no token
and proves nothing. PR #754 verified every changed Go and TypeScript file this way.
