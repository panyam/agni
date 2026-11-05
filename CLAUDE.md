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
| Moving an open viewer from a parent page, a saved report or an agent (`--viewer-rooms`) | `architecture/viewer-control.md`, `build/embedding-the-viewer.md` |
| A sheet view beside the canvas, cross-probing a part between schematic and board, turning a board view | `architecture/web-client.md` ("Sheet views are panels made at run time"), `architecture/web-picking.md` ("Picking the board") |
| Net solving, hierarchy, net identity | `architecture/net-solving.md` |
| A check rule, datalog, interface profiles | `architecture/rules-and-checks.md`, `build/check-rule.md` |
| A declared pin map, or comparing an identifier across two documents | `guide/design-intent.md` (the seven forms), `core/ident`'s package comment |
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
| Why a rule exists at all, as engineering rather than as code | `learn/` (thirteen chapters, EE1-EE7) |

`guide/` is the user-facing manual (getting-started, concepts, running-the-server,
checks-and-reports, comparing-revisions, querying, naming-conventions, interface-profiles,
design-intent, datasheets, cli-reference).
`tutorials/` walks one board from first read to a CI gate. `reference/` also holds the GENERATED
rule and relation catalogs.

`learn/` is the DOMAIN course, the axis the other sections do not cover. Its thirteen chapters teach
what a hardware engineer knows, each ending in the rules that encode it. `tutorials/` teaches the
tool and assumes the domain; the rule pages explain the check and assume the instinct. `learn/` is
the layer between. `learn/levels.md` defines EE1 through EE7 (parts, nets, roles, failure modes,
numbers, systems, layout) and maps every section of the course to its level. The levels are also the
vocabulary for asking, so "explain `output-output-conflict` at EE4" wants the bench symptom rather
than the definition.

**When a change touches a rule that the course teaches, check whether a chapter needs updating**,
and cite the relevant pages as prerequisite reading in the PR.

**Never paste an `<svg>` into a markdown page.** A hand-authored diagram lives in `docsite/figures/`
and the page carries `{{ includeFile "figures/<name>.svg" }}`. That call inlines the figure at BUILD
time, so the figure still resolves `currentColor` and `--accent-color` against the page's theme, and
the page's prose is still readable. An `<img src>` cannot do that, because the image renders in its
own document and inherits nothing. `includefile_test.go` fails the gate on a path that does not
resolve (`IncludeFile` returns an empty string and the build still succeeds), on a figure that no
page includes, on a colour literal, and on a BLANK LINE inside the file. A blank line ends the
raw-HTML block and drops everything after it out of the `<svg>`. `docsite/README.md` has the house
style, the traps, and the browser sweep that checks a rendered figure's geometry. **Serve `dist`
under the `/agni/` prefix** when you check anything there in a browser, because at a server's root
every asset 404s, the page renders with no CSS while still returning 200, and the nav then measures
as its mobile layout at any viewport width.

`site/` is stale build output, not a source tree. Some older notes reference a retired `docs/NN-*.md`
mkdocs tree that was folded into `docsite/content/` with audience-first names.

## Package layout

Engine analysis under **`core/`** (`core/check`, `core/review`, `core/render`, `core/report`,
`core/diff`, `core/facts`, `core/query`, `core/model`, `core/classify`, `core/ident`, and others).
Format readers under **`readers/`** (`readers/edif`, `readers/kicad`, `readers/ipc2581`,
`readers/xschem`, `readers/geda`, `readers/telesis`, plus `readers/formats`, the registry/Loader).
The shipped rule catalog, fact relations, profiles, and intent under **`stdlib/`**
(`stdlib/rules/builtin/rule_*.go`, `stdlib/rules/datalog`, `stdlib/rules/intent`,
`stdlib/relations`, `stdlib/profiles`). The datasheet contract in **`core/param`** (the PartSpec the
engine reads), and the extraction pipeline that produces it in **`datasheet/`, a Go module of its
own** (`github.com/panyam/agni/datasheet`, agni issue 744): `doc`, `derive`, `docindex`,
`candidate`, the transport-neutral `dsservice`, its Connect adapter `dsserver`, and the **`agnids`**
binary (`datasheet/cmd/agnids`: `serve` for the workbench, `derive`). It depends on the root, never
the reverse, and C34 and C18 hold that. Its tests run in `make datasheet-test`, because `go test
./...` from the root never reaches a nested module. The embedding surface under **`service/`** (the
transport-neutral service impls and their ports), **`artifact/`** (the `mount://` URI those ports
speak) and **`mounts/`** (the named roots and the containment join). Plus `cmd/agni/`, `internal/`,
`intake/`, `census/`, `protos/` + `gen/`, `docsite/`, `web/`, `hack/`, `tools/`, and
`clients/python` (the typed Python client, its generated messages committed under `src/agni/v1`).

Some notes predate this layout and name the old directories. **Grep the SYMBOL or filename, not the
directory.**

**The Datalog engine is a separate module, `github.com/panyam/jaala/datalog`** (agni issue 731), and
`core/query` is agni's adapter over it. The adapter holds the `Source` projecting a Model through
the fact registry, a fact base per design over one vocabulary, `RuleFromQuery`, the wire form and
the catalog. **Every queryable name registers in `core/facts` at a path of jaala's `ns.Vocabulary`**
(agni issue 751), through `facts.RegisterRelation`, `facts.RegisterPredicate` and
`facts.RegisterModules`. A predicate is an `ns.Builtin`, such as the `net.reaches` generator in
`stdlib/relations`, which reaches the design through `facts.EnvOf`. `jaala/ns` holds names and
signatures, and `jaala/stdlib` holds the standard predicates. Neither holds an evaluator, so C29
lets the fact layer import those two and nothing more of jaala. **The shipped library of derived
relations is `stdlib/lib`**, one `.dl` file per module (`component.probed_both`,
`net.has_test_point`), and `agni query --relations <path>` prints any member's signature and
definition. Its modules register as ONE batch, because the vocabulary is checked at every
registration. A module that reads another module's member is refused if it arrives first. A library
reaches a query three ways, the shipped one, a project's `lib/`, and modules sent WITH a request
(`AnalysisConfig.library_modules`, the CLI's `--lib`, agni 788), and all three compose in
`Overlay.Registry`. **agni answers with `query.Default`**, which is jaala's `SemiNaive`. It plans
each rule body. When a body calls a relation with a bound argument, it derives that relation only
for that value, so the written clause order no longer decides cost. Tests compare against `Naive` as
the reference, and a new call site that evaluates for a user uses `query.Default`. A generator
declares the bindings it accepts (`ns.Builtin.Modes`); `net.reaches` and `net.route` declare `from`
bound or a deliberate full walk. A language change (parser, evaluator, index, aggregation) is a
jaala PR and a tag first, then a `go get` here. Its issues live on panyam/jaala.

`core/facts` is the fact/relation layer, and it depends on NO query engine (C29). A relation
projects a `check.Model` into tuples and registers with `facts.RegisterRelation`; an engine that
answers questions over those tuples imports `core/facts`, never the reverse. So datalog
(`core/query`, over jaala) is one query shape among several rather than the primitive. `check.Spec`
answers per-entity questions with no fact base at all, and datalog cannot express a path question
(issues 374, 518). A relation vocabulary is a composed VALUE (`facts.DefaultRegistry`, the twin of
`check.DefaultCatalog`), so the package globals are only an append-only registration buffer, and
every READ goes through a `*Registry` that a caller holds. **Installing no relation catalog still
builds and still runs**, leaving the fact base empty so every datalog rule reports clean;
`Registry.Installed` tells that case apart from a query that matched nothing. **That distinction
matters at INIT time**, because `stdlib/profiles` compiles its built-in profiles in an `init()` that
runs before any relation catalog has registered. So code that validates a query against
`facts.DefaultRegistry()` at construction sees an EMPTY registry. That code must stand its
vocabulary checks down rather than call every relation unknown (agni issue 540). `core/review` names
no query syntax either, because a manifest's inline query compiles through a registered
`review.QueryCompiler` (`stdlib/reviewquery` is the datalog one), so no core package outside
`core/query` knows a query language.

The **root `agni` package** is the composition facade and an embedder's entry point, and `agni.New`
returns an `Engine` holding one composed `*check.Catalog` and one `*facts.Registry`. Compose through
it rather than calling `check.DefaultCatalog` at a call site. There are FOUR global registration
points (`check.RegisterBuiltins`, `facts.RegisterRelation`, `check.RegisterSource` for the `dl`
suite, `review.RegisterQueryCompiler`). Three of them fail SILENTLY when a binary misses one, and
the overlay example ran for months with zero built-in rules that way. `New` refuses an empty fact
base and an uninstalled built-in catalog, and reports the two legitimate absences through
`Warnings()`. Options take VALUES, never paths (C22), so `profiles.LoadDir`/`intent.LoadFile` stay
in the caller. Note the registration check asks `check.BuiltinRules()` rather than measuring the
composed catalog, because `stdlib/profiles` registers from an init and a program missing the
built-ins still composes a NON-EMPTY catalog.

An EMBEDDER composes against `service/`, so it is not under `internal/` (C13). A private deployment
implements two ports, `ProjectStore` and `ConfigResolver`, to serve its own designs and its own
config. `ConfigResolver` takes the URIs that any `AnalysisConfig` names, a project's or a request's,
and turns them into a `ResolvedConfig` (rule sources, a param provider, symbol paths). It moved out
of `internal/service` with `artifact` in tow, because `artifact.URI` is in the loader-port
signatures. The same rule still keeps it free of file I/O. It imports no `os`, no `path/filepath`
and no transport, and every I/O concern arrives as an injected port. `internal/projects` holds the
directory-walking `FSStore` and is internal on purpose, because everything true only of storing
projects in DIRECTORIES lives behind the port.

`intake/` produces a sanitized design summary, and its confidentiality
guarantee is **structural**. The `Skeleton` type has no field that can hold a net name or a
connection, so an intake summary *cannot* express the confidential parts of a design. That turns a
policy ("do not paste a net name") into a property of the type ("there is no field for one"). Keep
it that way. Adding a free-text field to `Skeleton` would quietly dissolve the guarantee.

## Build, test, and the CLI

- `make build` / `make test` / `make agni` / `make install`. **`make agni` writes `bin/agni` and
  does NOT update the `agni` on your PATH**, which `make install` puts in `~/go/bin`. If you verify
  a CLI fix with the wrong one, the bug still shows, and the fix reads as not working.
- **Nothing here takes a private path, and a private workspace must never have to reimplement a
  target.** Two mechanisms carry someone's own designs in. Tier-1 config belongs in an `agni.yaml`,
  and covers mounts, symbol paths and native tools. The CLI finds that file by walking up from the
  working directory and then in `~/.config/agni/`, so `agni check mount://corpus/...` works from
  anywhere with no flags (see `cmd/agni/envconfig.go` and the tier boundary it guards). Everything
  else is a variable on a target, such as `EXTRA_MOUNTS` and `OVERLAY_FLAGS` on `serve`, `DESIGNS`
  and `OVERLAY_DIR` on `dockserve`, `NATIVE_DOCKER_MOUNTS` on `natup`, and `DATASHEET_DIR` on the
  datasheet targets. If a workflow only exists as a wrapper in someone's local Makefile, this repo
  is missing a target.
- **A relative path in `agni.yaml` resolves beside the file**, not against the working directory,
  so `agni stats mount://boards/...` reads the same from a subfolder of the checkout (agni 1373).
- **A flag wins outright over `agni.yaml` rather than merging**, so a Makefile default that passes
  `--mount` shuts the file out. `make serve MOUNTS=` hands the mount table back to the file.
- **`agni.yaml` wants a `web_dir` or `--server self` cannot run outside a checkout.** The lookup falls
  back to a relative `./web`, so a run from a design folder fails with `--web-dir "web" is not a
  directory`, and since agni 637 it fails BEFORE writing the artifact rather than after. The four keys
  are `mounts`, `symbol_paths`, `web_dir`, `native_tools`, and `web_dir` must name a BUILT bundle,
  because `checkWebAssets` stats the viewer's templates and `static/*.js` before the listener opens.
- **With NOTHING naming a web dir and no `./web`, an installed binary finds its own viewer**
  (`internal/webbundle`, WS16-003). A release binary embeds it (`-tags embedweb`, package `web`) and
  writes it under the user cache once. A `go install` binary downloads `agni-web-<tag>.tar.gz` from
  its release against `checksums.txt`, since its module carries no built JS. A build that is not a
  release tag (`make agni`, `@main`) has neither, so `serve` runs the API alone (agni 735) and
  `open` refuses. An absent dir is a choice and a broken one is a mistake, so a NAMED dir that is
  wrong, or a `./web` without its bundle, still fails and never reaches for the bundle. `make dist
  V=...` builds the release tarballs and `hack/dist_smoke.sh` runs one from an empty directory. The
  gate's `dist-check` runs both for the host platform, since v0.3.13's job failed where a Mac
  passed. macOS tar needs `COPYFILE_DISABLE=1`, or every file gets a `._` shadow and the template
  loader reads it. A loose file gets the minted mount `files`, never `local`. `local` is the
  browser's drop mount, and the Load page refuses it on the server engine. The datasheets workbench
  is NOT served by `agni` at all since agni 744; it is `agnids serve` (`make dsserve`), and `agni
  serve --datasheets-url <url>` makes the landing page link to it. A script starting a server for
  itself passes `--addr 127.0.0.1:0` and `--web-dir`, and reads the URL off the banner. The banner
  names the port that the kernel chose (agni 1121). Started outside the repo root with no
  `--web-dir`, the server serves the API alone and every MCP answer says it has no link.
- **A release is tried through `deploy/compose.yaml` on a machine whose boards we never see** (agni
  1057). `make compose-release V=...` pins that file and the release attaches it. Its `trial`
  service runs `agni trial`. The `feedback.Report` of `agni trial` has no field for design content,
  and `agni trial` names a rule only through `feedback.Shipped` against `service.ShippedRules()`,
  since a project's rule namespace can name the customer. A design's pin map is counted
  (`feedback.PinMapTally`, agni 1196). When a declared function went unchecked, the reason travels
  as a KIND, since the sentence names parts, and `intent.FunctionReasonOf` reads the KIND off the
  rule's fixed phrases. So a new not-considered path in `io-map-function` builds its reason from a
  new phrase or reads as `other`. `hack/compose_smoke.sh` runs the bundle in the `images` and
  `release` workflows. Pipe `compose run` output only with `-T`, or it arrives empty.
- **There are two images, published by one release tag**: `Dockerfile` (`ghcr.io/panyam/agni`, the
  engine, viewer and symbol libraries) and `Dockerfile.agnids` (`ghcr.io/panyam/agnids`, the
  workbench and the docling stack). `make image` and `make dsimage` build them. The `images`
  workflow builds and smoke-tests both through `hack/image_smoke.sh` on a PR that touches the files
  they are built from. `release` also runs that script against what it published. The agnids image
  prefetches its models by RUNNING pdf2doc over a synthetic page, and so does `make setup`
  (`datasheet-models`), because docling reads the Hugging Face cache and `docling-tools models
  download` fills a different one (agni 786).
- **The web code is ONE pnpm workspace with one lockfile at the root**: `web/` (the viewer),
  `datasheet/web/` (the workbench, which `agnids` serves, with pdf.js) and `web-shared/` (plain TS
  that both import as `@agni/web-shared/<name>.js`, with no npm dependency of its own, so neither
  page pulls the other's libraries in through it). Run `pnpm install` at the ROOT. `make ui`, `make
  web-test` and `make proto-web` cover all three. The workbench's TS generates from the whole buf
  workspace and the viewer's from `protos/` alone. The workbench has its OWN `BasePage.html`. It
  holds only the thirteen rules the workbench uses out of the viewer's 50KB of styles, so
  `datasheet/web` deploys on its own. A computed-style comparison of all 86 workbench elements,
  before and after, was identical. **mcpkit bumps in lock-step** across its six Go modules in the
  root `go.mod`, `examples/ask-over-mcp`, and `@panyam/mcpkit-rooms` in `web/package.json`. pnpm
  adds the new version to `minimumReleaseAgeExclude` in `pnpm-workspace.yaml`, and that file is
  committed with the bump. npm can lag mcpkit's publish job by a few minutes, so a 404 right after a
  tag is not a failed release.
- **An unknown key in `agni.yaml` is a hard ERROR, and the file is shared by every lane, every
  released binary and the container image.** So a new key goes in only after every reader on the
  machine understands it. The obvious probe is a trap, because `agni <cmd> --help` short-circuits
  before the config is loaded, so it accepts anything and proves nothing. Probe with a command that
  reads a design, and run the same probe with a deliberately bogus key first as a positive control.
- **Every config YAML binds through `internal/yamlpb` against its proto**. That covers a design's
  `intent:`, a project's `conventions:`, a `--conventions` file, and a profile file (`ProfileDef`,
  or `ProfileNamingMap` for an `override:` file; agni 827). Keys are proto field NAMES only, never
  the camelCase JSON spelling, and an unknown key fails with its line. A new config file binds the
  same way. Go structs with yaml tags are the bug shape, because they drop an unknown key silently.
  A misspelled `pull_up:` left a profile signal with no pull-up requirement and no error that way.
- **A descriptor NAMES every tier a run reads, and nothing is found by its folder's name** (agni
  1136). `project.yaml` and `design.yaml` share one schema (`descriptorYAML` in
  `internal/projects/descriptor.go`). In that schema `conventions:`, `checklists:` and `intent:` are
  sections, and `profiles`, `params`, `symbols`, `lib` and `queries` are lists of paths relative to
  the descriptor or `mount://` URIs, each a file or a directory. Only a design names its own files
  (`entry`, `companions`, `revisions`, `flatten_hierarchy`). A design names its project with
  `extends: ../../project.yaml`, as a project names shared config. A design is never found under a
  project by where it sits. If a design names no project and a `project.yaml` sits above it, the
  load is an ERROR (the forgotten extends). A design's own sections layer over its project's
  (`mergeConfig`), so a board can carry its own convention or checklist. A source that names nothing
  is a load ERROR rather than an absent tier, so moving a tier's folder aside now fails the load. To
  reach a tier's "off" state DELETE ITS LINE. Rungs 5, 6 and 11 open with `sed -i.orig '/^params:/d'
  project.yaml` for that reason. Rung 6 once shipped a before/after whose two captures were
  byte-identical because both ran with the corpus in place. A design folder copied out of its
  project names a parent that the copy lacks. So a test or a seed that mounts the folder alone
  strips its `extends` line (`gatewayAlone`, `gatewayInNoProject`). agni refuses a `--profile-path`
  that names a profiles directory the project already names, since it would load every profile rule
  twice (issue 450). A `--params` that names the project's own is merely redundant. `queries` names
  the query sets. `QueryService.ListQuerySets` lists them, the viewer's Sets menu opens them and
  `agni query --set <name>` runs them (agni 745). `lib` names the derived relations. Every query
  surface and a review manifest's inline queries read them through `Overlay.Registry` (agni 773,
  779). `--checklist` takes a project checklist's NAME, or a file ending `.yaml`/`.yml`. With
  neither, the first checklist that the project writes runs. **`ReviewService.ListChecklists` is the
  ONE answer to which checklists a design has** (`agni checklists <design>`). It lists inherited
  ones first, and the CLI's `--checklist`, the viewer's picker and clients alike read it (agni 859).
  A client runs one by sending its manifest to `CreateReview`, which takes values only (C22).
  `GetReviewManifest` is still there for a checklist FILE beside a design in no project.
- **Between a project tier and its flag, the project wins, and for the datasheet corpus it wins PER
  MPN (`Overlay.SpecsOver`, agni 749).** That is the opposite of the mount rule above, deliberately,
  because a project owns its parameters the way it owns its profiles. The project's `params/` is
  layered over `--params` or `--params-url` (`param.Layered`), and every datasheet citation names
  its corpus (`project` or `shared`, stamped by `check.Run` and `RunVerdicts` through the model,
  never by a rule); DECISIONS.md records the reversal from wholesale replacement. A command that
  reads a tier from its flag alone is the bug shape, and `intake` did, so inside a project its
  datasheet-gap section was absent rather than empty (issue 474). The params panel's
  `GetComponentParams` did too, until 749. `readDesignWithConfig` returns the overlay that the read
  already composed. A non-service command should get a tier from that overlay rather than resolving
  the project a second time.
- **kicad-cli's own SVG is an oracle for where KiCad draws text** (agni 1344). `sch export svg`
  writes every run as an invisible `<text>` at its BASELINE, half a glyph below its centre, with its
  anchor, and its rotation on the enclosing `<g transform="rotate(a x y)">` about a pivot that need
  not be the run's own point. kicad-cli draws only the library symbols a sheet embeds, so a fixture
  whose symbols live in a `.kicad_sym` (the tutorial gateway) needs them embedded in a scratch copy
  first. A rendering PR shows kicad-cli's frame beside ours (CONTRIBUTING.md).
- `make natrender FILE=... OUT=...` and `make natopen FILE=...` drive the native tools over the
  `natup` container. Both take paths INSIDE it, so they must fall under a `NATIVE_DOCKER_MOUNTS` dir.
  The image carries xschem, Lepton and gerbv and NOT KiCad, so `kicad-cli` for an oracle comes from a
  KiCad install on the machine (KiCad's Ubuntu PPA has no arm64 build; Flathub's does).
- `make setup` builds the docling venv that the datasheet tooling runs in, then `make pdf2doc`,
  `make pdf2doc-all`, `make datasheets-status` and `make dsserve` work over `DATASHEET_DIR`. The
  tools live in `datasheet/tools/`, so those targets `cd datasheet` first. The venv is found by
  lookup (repo-local `.venv`, then the parent directory's), so worktrees sharing a root share one
  env instead of each carrying gigabytes of torch.
- `make -C docsite preview PAGE=learn/03-why-every-chip-needs-capacitors` folds one built page into
  a self-contained HTML file, for reviewing a branch before it merges. Use it rather than `make -C
  docsite gh-pages`. That target is DEAD, because Pages serves the `docs.yml` workflow artifact
  (`build_type: workflow`), so force-pushing that branch changes nothing. Pages deploys run one at a
  time. If a run is stuck at `waiting` on the `github-pages` environment, where no reviewer exists
  to approve it, every later deploy waits at `pending`. Cancel the stuck run with `gh run cancel
  <id>`, since a later deploy carries its commit.
- **A shared link carries a card** (agni 1205). Every docsite page sets `og:*` and `twitter:card`
  from `SiteMetadata.json`'s `siteURL` and `socialImage`, and the demo landing page takes `agni site
  --social-image <url>`, because a card image needs an absolute URL and the site does not know its
  host; `make docs-site` passes the docsite's `static/images/social-preview.png`, which is drawn
  from the synthetic showcase fixture. If `make -C docsite build` fails in `components` with
  `@esbuild/darwin-arm64 could not be found`, the cause is that clone's `docsite/node_modules`, not
  the change. `AGNI_DOCS_ENV=production go run . -build` inside `docsite/` builds the pages alone.
- `make -C docsite images` re-renders every engine-drawn image that the docs embed, and
  `docsite-images-check` in `testall` fails when one is stale (agni issue 453). Never `--layout
  force` there, since it alone differs between arm64 and amd64 (agni 472). `tutorial-runs-check`
  likewise regenerates every capture and fails on any difference. A capture's stamp hashes the spec
  and the fixture but NOT the engine, so regenerating is the only way to see engine drift. **Editing
  any tracked file inside a fixture restamps every capture over it**, 61 of them for
  `examples/tutorial-project`, so commit the fixture edit, then `make tutorial-runs`, then check the
  diff is stamp lines alone. A fixture under `tools/samples/` has nothing tracked and hashes
  `hack/samples.pin` instead (agni 682).
- The CLI's commands include `agni stats|check|diff|render|query|trace|review|serve|open <file>`
  and `agni params <mpn>`. `open` serves ONE design and prints its URL, minting the mount itself;
  `serve` takes `--mount` per folder and `--web-dir`. The reader is chosen by extension
  (case-insensitively), with `.xml`/`.sch` sniffed by root/header. `--symbol-path <dir>` resolves
  external symbol files and searches each dir's SUBTREE, so a dir can be a library root.
- **The engine also runs in the browser** (mission #851). It is `cmd/agni-wasm` on goapplib's
  `wasmhost`, composed by `internal/wasmengine` over `fshost` (the service ports over `fs.FS`
  mounts). A design keeps one URL, `/designs/<mount>/<path>/view`, and the ENGINE is a separate
  per-page choice, made by `agni serve --engine wasm`, a page's `?engine=`, or a static page. On
  wasm the page lists a design's files (`ListDesignFiles`) and fetches them from
  `/raw/<mount>/<path>`. It sends a design over `--wasm-max-bytes` to the server instead. That limit
  is 128 MB, measured in #852 with `make wasm-bench`. The engine runs as a `serve` and a `jobs`
  worker (#911). Every worker keeps what it parsed in the origin's private file system, keyed on
  `wasmengine.StoreVersion`. So a wasm built with plain `go build` from a dirty tree persists
  nothing, and a wasm built by `make wasm` names its uncommitted diff. Dropped files go into the
  browser mount `local` and the browser's IndexedDB (`web/src/drops.ts`, agni 1072), so they survive
  a reload; nothing ever lists `local` on a server, and `ProposeDesigns` / `agni propose` group them
  into designs. `agni site` (`make demo-site`) writes the whole thing as static files under a
  prefix. Architecture is in `architecture/web-services.md`, "The same contract in the browser".
  `make wasm wasm-test` builds and smoke-tests it under Node.
- **Four browser-engine traps.** Never block inside a `js.FuncOf` waiting on a JS promise. Blocking
  pauses the event loop, and the promise needs that loop, so bytes are PUSHED into the worker before
  a request. goutils' `memfs` refuses a file under a path that is a file, so a descriptor beside a
  design that came out of a zip goes through `fshost`'s `.agni-overlay/`. The server and the worker
  read through different adapters (`osLoader`, `fshost`), so `TestWasmEngineAnswersAsTheServerDoes`
  holds them to identical answers; a new served question belongs in it. And `composition.test.ts`
  fills the viewer shell's template fields itself, so a new `{{ .Field }}` in `ViewerPage.html` must
  be added to its `servedFields` or the test throws.
- **A browser worker replaced after every request keeps nothing in memory.** `DesignCache`'s memory
  entries die with the worker, and its replacement fetches the design's files again, so the serve
  lane replaces a worker past its limit only when `wouldShrink` in `web/src/wasm/client.ts` says a
  fresh one would come back smaller, judged on the live Go heap that goapplib 0.7.4 reports (agni
  974, 990). The symptom is the console line `the serve worker held N MB, M MB of it live, and was
  replaced` after every query. Measure with `make wasm-bench` before tightening
  `SERVE_LANE_MAX_BYTES` or `SHRINK_RATIO`.
- **A command's `--format json` is protojson of that command's WIRE MESSAGE** (C31), so a script
  reading the CLI and a client reading the rpc parse one shape. `agni intake` and `agni trial` are
  the two declared exceptions, reasoned on `intake.Skeleton` and the `feedback` package, because
  each type's confidentiality guarantee is structural and a proto twin would have to carry it into a
  file that people edit for other reasons. `TestEveryJSONFormatEmitsAProto` enforces it over
  `cmd/agni`'s own source, and `TestNoCommandCallsAHandRolledEncoderInAnotherPackage` follows the
  commands' calls into the module's other packages. `report.TableJSON` and `review.RenderJSON` had
  hidden there (agni 734). `review --format json` is the `Review` that the rpc returns, with a
  `summary` computed on the way out. **A failed request under `--format json` is written as the
  Connect error** (`code`, `message`, `details`) through connect's own error writer
  (`cmd/agni/rpcerror.go`). A new service call in a command returns its error through `rpcFailed`,
  or a script gets text where it expected JSON. An unmarked error, such as a flag mistake or a set's
  after-the-answer status, is text-only on purpose. **`GetDesign` serves the IR on request** (agni
  836). A request sets a `read_mask` over the response (`design`, `design.nets`,
  `design.components.mpn`, `*`) plus `nets`/`ref_des` filters, and the CLI has `stats --format json
  --mask`. UNMASKED it is still the viewer's summary with no IR, deliberately against AIP-157's
  "empty mask is everything", because the viewer calls it on every open.
- **`agni trace <design> --from U7.3 --to U12.4` follows a signal through the series parts between
  them** and prints the route, the nets, and the probe points on each. `--render <file.svg>` draws
  it, on the design's own schematic where it has one and on an auto-layout where it does not, saying
  which. An endpoint's pin is a designator or the part type's functional name. `check.LookupPin`
  resolves it, and the IO-map rules use the same lookup (agni 1193). An ambiguous or loose-only name
  is refused with its candidates. `--server` mints a link that re-asks the question in the viewer.
  Unlike a verdict link, it carries the QUESTION, so it needs no content hash. Three outcomes are
  kept apart: a route, no route within the radius, and an endpoint that names nothing in the design.
  The last one exits non-zero, because a pin spelled wrong and two pins genuinely unconnected are
  opposite problems.
- **A rule NAME that the catalog lacks is refused** (agni 1121, `check.Catalog.CheckNames`), by
  `--rule`, `CheckDesign` and `GetCheckReport` alike, with the nearest names offered and a
  superseded rule named as superseded. Before, it ran nothing and answered an empty run that read as
  clean, and two tests had passed that way for months naming rules that never existed. Some rules
  only exist under some config; `intent/exposure-declared` needs a declared exposure. Such a rule is
  named only on the run that has that config.
- **A check run is SAVED through `CheckService.RenderCheckReport`, from the run that the caller
  holds** (agni 127). `check --results-out` (the `CheckResults` document) and `check --verdicts
  --format html` write through it, and so do the viewer's Save report and Save JSON, which send the
  panel's run instead of running anything again. A new way to save a run calls it rather than
  rendering. The document's JSON is protojson. Its whitespace varies between builds on purpose, so
  two builds' documents compare as parsed JSON, never as bytes. One build's two hosts compare as
  bytes (`TestWasmEngineAnswersAsTheServerDoes`).
- **There are two HTML reports, on different axes, sharing one stylesheet.** `check --format html`
  is the verdict report, rule-major, and implies `--verdicts`. `review --format html` is the
  checklist, question-major, in the manifest's order with every finding per item. Both take
  `--server` and share `core/report/style.css`. `ReviewService.RenderReviewReport` builds the
  checklist page from a review that the caller holds. `review --format html` and the viewer's Review
  panel Save report both call it (agni 1030). **Against a REMOTE server a link is only emitted for a
  mount you DECLARED**, because a mount minted for one run means nothing on a server that was not
  started with it. agni then asks that server through `ListMounts` whether it serves the name from
  the same root. A withheld link always states its reason, on stderr AND in the report itself. The
  reader who needs the reason opens the saved page later, and by then the terminal is gone (issue
  626). **`--server self` removes the question instead of answering it**. It reads the design and
  serves it from one process, so a minted mount is as linkable as a declared one. It blocks until
  Ctrl-C, because the links live exactly as long as the server does. `self:PORT` fails on a taken
  port rather than moving. **A link names the design's declared ENTRY whatever you pointed the
  command at, and carries the revision it was read at**. The viewer checks that revision before it
  draws. Both report rpcs resolve `ReportLinks.design_uri` to the entry themselves (`linkMountPath`,
  agni 1120), so a client may send the design's folder, as the viewer does. **A diff link is the
  older revision's view with `?against=<mount>/<path>`** (`report.DiffURL`, agni 1126). That
  parameter starts the viewer's Compare as the picker does. `agni diff --server` prints the link and
  the MCP `diff` tool returns it. A near-rename diff adds `rename_approx=1` so the comparison's
  tables pair them too. **A verdict link also carries `rule=`**, so the viewer resolves it by
  running ONE rule rather than the catalog. That took 21.78s down to 1.95s on a 3980-component
  board, and the cost comes on every click, because each click from a report is a fresh page load.
  The rule is minted rather than recovered, since `VerdictID` is generated and never parsed. A link
  without `rule=` still resolves by the slow route, so saved reports still work. A verdict link
  carries the verdict's subjects too (`subject=net:SDA`, `check.SubjectParam`'s spelling, agni 897).
  The viewer asks that rule about those subjects alone, and holds the answer apart from the run in
  `Document.scoped`. A TRACE link needs no such hint because it carries the question rather than a
  conclusion. `guide/checks-and-reports.md` has the semantics and the two ways the halves used to
  disagree.
- **`agni serve --mcp` and `agni mcp` serve the analysis over MCP** (agni 910,
  `internal/mcpserver`), the first at `/mcp` beside the Connect API and the second over stdio,
  composed by the same `runViewer`. The query language's tools (`query`, `explain`, `schema`,
  `look`, `values`) are jaala's `frontends/mcp` with a `design` argument that agni adds; agni
  registers `check`, `trace`, `diff`, `design`, `checklists`, `review`, `query_set` and `designs`.
  jaala evaluates a query and agni presents it through `QueryService.ReadQuery` and
  `QueryRead.Present`, which is `RunQuery`'s own projection, and `Tabulate` (C35), so a new query
  field reaches MCP with no change there. `Run.BaseFor` reads the board only for a program that
  reads `board.*`, as `RunQuery` does. `TestMCPAnswersAsConnectDoes` asks every tool and its rpc the
  same question over one server, so a new tool belongs in it. **The HTTP endpoint accepts both MCP
  wires** (`mcpserver.Wire`, pinned to mcpkit's `ModeDual` so a default change can't drop one,
  `MCPKIT_STATELESS_MODE` overriding it), and the MCP tests run on both through `eachWire` (agni
  1172). agni's own clients speak `adaptive`, the stateless wire when offered and a session
  otherwise, and `agni mcp` on stdio serves both, since mcpkit v0.8.13 (panyam/mcpkit#1637).
  **`--viewer-rooms` also serves a tab's page events as SEP-3415 events** through mcpkit's
  `roomsevents` (agni 1187), so `rooms-channel` puts a reader's Ask in front of a Claude Code
  session as a turn; the events need the session wire, and `TestARoomsChannelHearsATabsQuestion`
  runs that front end over a pipe. massrelay's Go dialer sends no `Origin`, and the relay refuses a
  connection without one, so a Go test joining as a tab sets it (`viewerRoomsServer`). The lists
  carry an hour's `ttlMs` (`listTTLMs`, through jaala's `Options.Server`). **`make exercise-mcp-ask`
  is mission #909's exercise** (agni 1121). `examples/ask-over-mcp exercise <spec>` asks a spec's
  questions over `serve --mcp` on a free port. It writes each reply's table through
  `service.WriteTableCSV` and holds it to the CLI's csv for the same request. It also holds each
  reply's links to a 200, and its rows to what the spec pins. A new tool's reply has to open with
  `design:` (or `diff:`) and carry `in the viewer: <url>` or `no viewer link: <why>`.
- **`--remote <url>` on `query`, `check`, `trace` and `review` sends the command's request to a
  running `agni serve`** (agni 998) and renders the response exactly as the local run does.
  `TestRemoteAnswersAsTheLocalCommandDoes` holds the two byte for byte. It is a different flag from
  `--server`, which only says where LINKS point. The design's files go with the request (agni 1061,
  step 4b). The client sends a manifest of the files that `ListDesignFiles` would list, under this
  machine's own mount names. Then it sends the contents that the server reports missing
  (`BlobsMissing`, then `UploadBlobs`). So the server needs no shared filesystem, and an unreachable
  server is an error, never a local read. A server reads an uploaded design through a host and a
  CACHE of its own per manifest, because the upload keeps the client's mount name and a shared cache
  keyed on the URI would serve one upload's reads for another. The command refuses flags that
  configure this process's engine (`--params`, `--profile-path`, `--timing`) rather than ignoring
  them. A remote review is `CreateReview` with `validate_only`, so the server stores nothing.
- **`review --coverage` renders as markdown ALONE and refuses an explicit `--format`.** The two are
  different axes and the render switch tests coverage first, so `--coverage --format html -o page.html`
  used to write a markdown table into a file named `.html`. If you want a coverage page, the per-item
  `--format html` already carries the same rollup in its header band.
- **`agni params <mpn>` prints the RECORD that a query answer cannot reach**, and it needs no
  design, because a spec library is not one. The datalog relations carry what a query can BIND. The
  command reads the rest off the `PartSpec`, namely the conditions that a value holds under, the pin
  bindings, the full provenance and the verification state. `--params <dir>` names a corpus,
  `--design <path>` lets a design's PROJECT supply one (layered over `--params` per MPN, per
  `Overlay.SpecsOver`), and `--format json` emits the bare `PartSpec`. Suppose someone verified a
  parameter, and the corpus then moved to a later revision. The parameter reports `stale` and names
  BOTH revisions. Staleness is decided on the content hash and NEVER on the printed revision, so the
  two strings are for the reader (`DECISIONS.md`, "A document revision is recorded for the reader,
  and never compared"). **A workbench draft is a DRAFT that no check reads**. It is saved
  unvalidated on purpose, keyed by MPN and citing its datasheets, in the corpus store
  (`<corpus>/drafts/<MPN>.draft.json`, `agnids serve --corpus`). `PublishDraft` or `agnids publish
  <mpn> --corpus <dir>` validates it into `<mpn>.textproto` (agni 209, 747, 749). Reading drafts in
  `LoadSet` looks like the fix and is the bug, because a half-finished transcription would fail the
  whole all-or-nothing load. The workbench saves nothing until an MPN is confirmed. **Publishing and
  the corpus index belong to the datasheet service** (`datasheet/corpus`). Publishing writes
  `corpus.index.json` (MPN, file, hash, a generation) beside the specs. `agnids index [--check]`
  rebuilds or verifies it, and `agnids serve --corpus` refreshes it at start. All of them build from
  `param.LoadCorpus`, the same walk that `LoadSet` makes, so the index and every eager load agree.
  The files are the source of truth; no check reads the index. **The engine reaches a SHARED corpus
  only through the contract's `PartSpecService`** (`protos/agni/v1/param/service.proto`). `agnids
  serve --corpus` answers it from the index, and `agni serve --params-url` reads it through
  `param.Remote`. `Lookup` returns no error, so a remote provider is a `param.Prefetcher`, and
  `service.BuildModel` fetches the design's MPNs there. At that point an unreachable corpus is
  `ErrUnavailable`, instead of every part reading as unseeded. A new model-building path that skips
  `Prefetch` gets no datasheet tier.
- **EDIF keywords are case-insensitive, and the reader folds them at parse** (agni 941). Altium's
  EDIF For PCB writes `(Net`, `(PortRef`, `(Instance`, and an exact match read its files as empty
  designs with no error. `readers/edif/sexpr.go` respells each list's head from `keywords`, and
  `TestEveryLookedUpKeywordIsCanonicalized` fails when a new lookup's keyword is missing from that
  table. Altium also writes no `(designator ...)` and names each instance by its ref-des. So when NO
  instance in a file carries a designator, each instance's name becomes its ref-des. **A `portRef`
  maps to a pin number through the instance's `portInstance` first and the cell port's own
  `(designator ...)` second** (agni 1193); before, a cell-only designator left every connection on a
  pin named after the port, which the part type did not have.
- **An EDIF part is keyed by the designator that `instanceBackAnnotate` gives it, in BOTH reads**
  (agni 1228, `readers/edif/backannotate.go`). An export can renumber parts after layout, so the
  page draws `R4` while the netlist and the design's `viewMap` say `R1001`. A `.eds` is read as a
  netlist and as geometry, so taking it in one read alone moves the mismatch rather than fixing it.
  Only a top-level instance is mapped, since a block's page is drawn once however often it is placed
  (#154), and `SchematicGeometry.back_annotated` lists every placement relabelled.
- **A net finding keys on the net's id, and only KiCad's geometry stamps one on a wire**, so on any
  other format a net subject locates by its name (`sheetsFor`, agni 1230). Until then every EDIF net
  finding read as drawn on no sheet while a query cell on the same net located. A schematic that
  fails to read answers `LOCATE_REASON_SCHEMATIC_UNREADABLE` rather than `NO_GEOMETRY`, and the CLI
  warns once. A companion once fell one `)` short, and every part read as drawn nowhere, with only
  `render` saying why.
- **ODB++X (ODX) is ODB++ in one XML file rooted at `<ODX>`, and it fills the job reader's own
  `parsedJob`** (agni 1372, `readers/odb/odx.go`), so its netlist and board come out of the job
  code. `.xml` is sniffed by root element. ODX names a pin by its NUMBER, since NAME can be a
  placeholder such as `un_1`. Allegro writes `encoding = "ISO-8859-1"` with spaces, which Go's
  decoder ignores, so `odxDecoded` reads the declaration itself. `TestODXReadsAsItsIPC2581Twin`
  holds parasdr's two Allegro exports (cadence-exports) to each other.
- **An ODB++ job is a DIRECTORY, entered by its `matrix/matrix` file** (agni 1250, `readers/odb`).
  The registry keys that path as `.odb++` (`lowerExt`), `.tgz` and `.zip` read the whole job from
  one archive, and the CLI and the proposer turn a job folder into its matrix entry. A host
  gathering a design's files asks `formats.DesignRoot`, or it fetches `matrix/` alone. A pin's net
  comes from its toeprint record and a copper feature's from eda/data's `FID` list, never from
  geometry. The fixtures are `kicad-cli pcb export odb` output (`make odb-fixtures`, KiCad 9+), and
  `TestODBReadsAsItsKiCadSource` holds each to the board it came from, pad positions included; `make
  odb-oracle` does the same for the six samples boards against `odb_oracle.baseline`, and the gate
  reads foxBMS's Altium-written jobs (`odb-altium`), whose pins are named `J500-1` and whose `FGR`
  groups are not nets. **ODB++ forbids spaces in a name**, so `/USB Hub/VBUS` arrives as
  `/USB_Hub/VBUS` and a schematic-against-layout diff reports it renamed. KiCad's exporter also
  leaves out a footprint marked `exclude_from_pos_files`, and nets copper by what touches, so a
  planted short moves a track onto the other net (demo-board).
- **An Allegro `.brd` is read in binary, for 17.2 alone** (agni 1350, `readers/allegro`). The format
  has no published spec; the layouts come from KiCad's reverse-engineered notes and were checked on
  the BeagleBone AI, and `e2e_allegro_test.go` holds that board to the extract Allegro wrote from it
  (`allegro` samples artifact): nets as a partition, every pin's board position, vias, widths and
  values. A pad is footprint-local and Allegro mirrors X before rotating, so a bottom part's angle is
  negated for the contract's rotate-then-mirror. A placed pad's net goes through its 0x04 net
  assignment, never straight to a 0x1B. Another release is refused by name (#1355), and an Eagle
  `.brd` is refused as Eagle. Read KiCad's GPL importer for format facts only, never its code. `TestAllegroCopperMatchesItsFilms` holds each copper layer to the film Allegro plotted
  (`internal/copperraster`, agni 1376), and only the inner layers' pads are logged rather than held,
  since Allegro drops a pad nothing on its layer connects to (#1356).
- **`internal/sexpr` shares every identical atom between nodes** (agni 945). That sharing holds an
  80 MB board in under 4 times its size, so never assign a parsed node's `Atom`; put a new node in
  its parent's `Kids` instead (EDIF's `canonicalize` does). `ParseSkipping` leaves named lists out
  of the tree, and the KiCad board reads skip `filled_polygon` (agni 946); the census parses
  unskipped. `ParseWith` also skips a list by one child's value (`SkipWhen`), reading ahead without
  building. A board read drops footprint graphics on undrawn layers at parse that way (agni 1005). A
  board keeps footprint graphics on the silkscreen, fab and courtyard layers alone
  (`footprintLayerDrawn`), since one vendor library's user-layer artwork was 322,600 of Jetson's
  326,626 footprint lines and most of its 273 MB board geometry (agni 946).
- **`emit --format edif` writes for a reader that is NOT ours, and that is a stricter target than
  the round trip.** Our reader resolves references after parsing the whole file, accepts any atom as
  an identifier, and reads a port reference as a pin designator when nothing maps it. So a writer
  that relies on all three round-trips perfectly and produces a file that nobody else can open (agni
  issues 563, 580). Four rules follow: libraries before the design node, a `portRef` naming the
  cell's PORT with a `portInstance` table carrying the pin, identifiers holding no character that a
  reader rejects, and everything the file references also declared. That includes cells, and a top
  cell for which a board read gives no source. `guide/cli-reference.md` has the semantics. The tests
  assert the properties over the emitted TEXT in `readers/formats/e2e_edif_conformance_test.go`,
  because a re-read goes back through the same forgiving reader and agrees with the writer whatever
  either does. `build/evidence.md` has the out-of-tree oracle that says which properties are the
  right ones.
- **`net.reaches` answers whether and `net.route` answers how.** `net.route(from, net, path)` is the
  same walk with the route bound as a rendered string (`VBUS -> [R5] -> VBUS_F`), so a connectivity
  answer carries its own evidence. It holds for exactly the pairs `net.reaches` holds for, so a
  route never ENDS on a rail; `agni trace` is the pin-to-pin form that does. It gives ONE route per
  pair, the BFS tree path, so it cannot speak about parallel paths. **The rendered string is a
  CONTRACT**, owned by `model.RenderRoute` and nowhere else, because it was written twice within a
  week and the copies agreed only by luck. DECISIONS.md records the reversal that admitted a path as
  a column, and what was accepted with it.
- **`query` emits five formats and two of them are DOCUMENTS.** It takes `--format
  text|csv|json|markdown|html` plus `--title`. markdown and html carry the title, the design and THE
  QUERY above the answer, so a saved view states the question it answers. csv deliberately carries
  no preamble, because its first row has to be the header that something binds to. An empty result
  is never an empty artifact. The markdown and html documents render `core/report.Table`; the csv
  encodes the engine's projection, as every csv does (next bullet). **`--set <file>` asks a named
  SET of queries over one read** (agni 729). It writes one document with a section per query,
  refuses csv, and exits non-zero AFTER writing when any query failed. `RunQueries` answers each
  query exactly as `RunQuery` would, because both call the same `answer` helper in
  `service/query.go`. **A value goes into a query as a BINDING, never as spliced text** (agni 793).
  The forms are `--bind name=value`, `RunQueryRequest.bindings` and a set's `bind:`. A variable can
  be bound to a LIST through `QueryValue.list`, `--bind` naming it twice, `bind: {n: [A, B]}`, or
  repeated `bind` params in a link. It then answers for each value, and `query.Bindings` is a
  variable's values everywhere in Go (agni 1121). The viewer's search and click presets bind too
  (`EntityQuery.binds`, `SearchQuery.bind`/`pattern`) and show the values as chips, so a served
  preset's text never changes. The engine refuses a binding that the goal does not use. A list
  holding ONE value is substituted as a constant, so it cannot be aggregated; a question counting a
  bound variable counts a copy of it instead (`?part = ?ref`, the area questions in
  `core/query/entity.go`, agni 904). **A position filled from a numeric slot is declared a NUMBER
  automatically** (`numericTypes` in `core/facts`, from the relation's `FieldNum`/`FieldMin`
  layout). Since jaala v0.1.15 the engine reads a constant or bound value there as a number, so
  `"3"` against a count is 3 and `"abc"` is refused. An UNDECLARED position keeps a value as given,
  so a text constant compared with an untyped number answers nothing rather than failing
  (panyam/jaala#65).
- **An answer's ROWS are the engine's, and every table encodes them** (C35, agni 862).
  `service/tabulate.go` projects a check run (`findings`, `verdicts`, `verdicts_by_rule`,
  `skipped`), a query or set, a diff and a review into `Table` messages. `TableService.Tabulate`
  serves those messages and every `--format csv` encodes them. A client asks `Tabulate` rather than
  laying an answer out; the Python client's `agni.tables` only encodes, and a ratchet test holds
  that. `order_by` (`["rule", "-subject"]`, `--order-by` on the CLI) sorts by each column's TYPE, so
  a `name` column puts R2 before R10, and `column_types` overrides one. Cells travel raw. The csv
  encoder guards formulas, and `tables_to_xlsx` keeps a cell starting `=` as text. `check --verdicts
  --format json` is the whole CheckDesign response, verdicts included.
- **Aggregation reduces BINDINGS, not values, unless you say `distinct`.** Since jaala v0.1.79
  `list` answers a list VALUE in its JSON text (`["R3","R4"]`, `[]` over nothing), so never split a
  cell on spaces. `count/min/max/sum/avg/list` group by the projection's plain columns;
  `count(distinct ?x)` reduces the SET of values instead. The trap is a goal that joins two things.
  It yields one binding per combination, so on a net carrying 7 test points and 20 capacitors
  `count(?tp)` is 140 and `count(distinct ?tp)` is 7. `distinct` is uniform across every function,
  `list` included, deliberately. An implicitly-distinct `list` would make `count(?r)` and `list(?r)`
  in one projection disagree about what the group holds. **`having` filters the GROUPS after the
  reduce**. A goal comparison cannot do that, because before grouping there is nothing to count. An
  example is `... => ?p having count(distinct ?n) = 1`. An aggregate may be filtered on without
  being projected, and the answer then lists the subjects rather than the tally (agni issue 613). A
  derived relation is the other route to a distinct reduce, by projecting the extra variable away
  before the group forms. **An aggregate-only projection always answers ONE row**, as SQL's
  `COUNT(*)` does, so a count over nothing is `0` rather than "no results". A grouped projection
  over nothing is still empty (agni 726). Every row that a finding query returns becomes a finding,
  so `RuleFromQuery` refuses a finding query whose subject is not a plain projected column.
- **`diff --rename-approx` is OFF by default**, so a net that was renamed AND changed reports as New
  plus Deleted unless you ask for it. The default is deliberate, because the pass ASSIGNS a best
  match rather than recovering a fact. It is also a false-finding shape, since a run without the
  flag reads as "we detect no approximate renames". One such run was written up as an engine gap
  before anyone noticed the flag.
- The toolchain is Go 1.26.9 and `buf` 1.61. **Both protoc plugins are pinned as `tool` directives in
  `go.mod` and invoked via `go tool`**, so their versions are data rather than something to match by
  hand. Only `buf` itself has to be on your PATH.
- **A command that reads a design goes through `readDesign` (or a service), never a bare
  `newLoader().ReadDesign`.** A design's PROJECT config enters the read in that function, for the
  six commands that no service mediates (stats, diff, emit, render, intake, profilediag). Net roles
  are resolved once at ingestion, so a read that skips it silently uses the built-in naming
  vocabulary and none of the project's declared symbol libraries. All six bypassed it until agni
  issue 228, so it is now one function rather than six. `stats`, `diff` and `render` take
  `--conventions` for a convention not yet in the project (agni 1275). `withConventions` sets
  `readOverlay` for one command, and `designOverlay` and `ingest` read it, so a new command with
  that flag calls `withConventions`.
- **A design's tiers come from `service.SourcesFor`, and naming the ENTRY is naming the design.** A
  descriptor's `companions` supply the tiers that the entry cannot, such as a schematic export for
  sheets and a board for copper. `NetlistURI` always points at the entry, and only the other tiers
  move. A design has three spellings, the folder, the entry filename and a declared companion, and
  all three now resolve identically. Until agni issue 528 they did not, because naming the entry
  skipped companions entirely. Take a design whose faithful geometry lived in a companion. It drew
  its auto-layout under one spelling and its real schematic under the other. `--as-named` is the
  opt-out. **An UNDECLARED sibling is still read exactly as named**, so companions are declared file
  by file rather than inferred. A later revision of the netlist sits in the same folder and is a
  legitimate analysis source, so inferring would turn a diff of two revisions into a diff of one
  against itself. **A later revision is declared under `revisions`, each with its own entry and
  companions** (agni 848), and reads with its own board. Before that it read with NO board and
  nothing said so. Rev B showed 11 findings to rev A's 28, and 13 of the 17 that vanished were
  board-tier rules not running. An undeclared or board-less revision still reads with none, and
  `check` now says on stderr which board-tier rules did not run (`check.NoBoardReason`), and
  `Tabulate` gives the run's `skipped` table. A file belongs to one revision at most, so a board is
  never borrowed across revisions.
- **A `.eds` is dual-capability, and its netlist is NOT the `.edn`'s.** An EDIF schematic export
  registers both a `Design` and a `Geometry` reader, because it carries nets joining portRefs in the
  same grammar, so every tool will parse it as a netlist without complaint. It counts DRAWN
  instances and per-sheet segments rather than resolved nets. One real board holds both files. It
  measured 3980 components and 1617 nets off the `.edn` against 5219 and 4572 off the `.eds`, so
  nets inflate by 183%. A design that ships only a `.eds` can be rendered and queried, and its
  counts must never be compared against a design read from a netlist. `formats.Format.DrawnNetlist`
  marks such a format, and `GetDesign` withholds counts only for one. A KiCad, xschem or gEDA
  schematic is its design's netlist and reports counts; until agni 1342 `agni trial` read every KiCad
  design as 0 components.
- **A saved report's link waits 2 s for an open viewer to speak, then loads the link** (agni 1156).
  The viewer posts `agni-viewer/v1/accepted` the moment it takes a request and the report then waits
  for the answer, since resolving a verdict link runs its rule (#1271, where a slow CI runner
  reloaded the viewer). Any new note from the page goes on its OWN envelope. A report saved earlier
  reads a reply on `agni-viewer/v1` with no `ok` as a refusal, and reloads.
- **A KiCad footprint's angle carries over unchanged, and a back footprint's items are un-mirrored**
  (agni 1261, `placementAngle`/`footprintLocal` in `readers/kicad/pcb_geom.go`). KiCad's angle is
  counter-clockwise as drawn and the Y flip preserves the picture; negating it put pad 1 where pad 2
  is. KiCad stores a flipped footprint's items already mirrored, so the reader negates their x and
  the angle, and the contract's composer mirrors once. No rule reads a pad's position, so the bug
  shipped. Only the board view and `TestODBReadsAsItsKiCadSource` (pad positions against kicad-cli's
  own export) see it.
- **Where a part, pin or net is drawn is `DesignService.LocateSubjects`** (agni 1325). It and a check
  run's findings read one index, so a subject locates where its finding badges. A pin answers the
  sheets that draw its own unit. The board sheet joins only when the request sets `board`, by
  ref-des and net NAME (an EDIF schematic and an ODB++ job share no net id), through a small index
  that the design cache keeps per board. The caching loader's `Board` returns a COPY, 385 MB on
  Jetson, so a new per-click caller goes through `boardIndexFor`, never `Board`. A run whose model
  read the board badges its findings with `"board"` after the schematic sheets.
- **kicad-cli is the oracle for a render, and two fixture traps make it draw less than KiCad would.**
  A hand-written fixture's schematic often has an empty `(lib_symbols)` cache and keeps its symbols
  in a `.kicad_sym`, and `design.yaml` names that library for agni. Eeschema plots from that cache alone, so
  kicad-cli draws the labels and wires with no symbol bodies. Copy the library's symbols into a
  SCRATCH copy as `<lib>:<name>`. Gateway's Edge.Cuts outline is a filled `gr_rect`, which plots over
  the copper, so export the copper and silk layers without Edge.Cuts. Export from a scratch copy in
  any case, since kicad-cli writes a `.kicad_prl` beside the board. A text is mirrored exactly when
  its effects say `(justify mirror)`, which kicad-cli confirmed on a back footprint that names none.
- **KiCad names a local label by its sheet path**, so a root-sheet net is `/SCL`, and every finding,
  query answer and link names it that way. `render --highlight net=SCL` matches nothing and still
  reports a highlight (#1225), so spell a hand-typed subject as `check` prints it.
- **KiCad's placement and bus rules are pinned against `kicad-cli`, and three of them are not what
  you would guess.** A symbol is rotated and THEN mirrored, where the shared transform composes
  mirror first, so a mirrored placement at 90 or 270 swapped its two pins onto each other's nets
  until agni issue 577. The fix is the INVERSE angle rather than a reordering, so the renderer
  needed no change. A bus VECTOR is spelled `[first..last]` and only that, so a KiCad label written
  `DATA[1:0]` is a plain scalar name, though xschem and gEDA do use that form. The reader records
  such a label in `range_labels` for `bus-spelling-not-a-bus` to report (agni 605). Two buses WIRED
  together share the members whose names match, while a bus crossing a SHEET PIN pairs off by bit
  position, ascending index. All of it is in `architecture/net-solving.md`, and **none of it is
  visible in a net COUNT**, because a pin swap moves one connection out of a net and another in, so
  one demo board read 47 nets against KiCad's 47 while 19 were wrong.
- **A locator records the path WITHIN the design's mount, never the host path.** Readers stamp
  `ir.Provenance.SourceFile` with whatever path they are handed, so the rename happens once, after
  the read. There `Loader.SourceName` maps a path to the name that provenance should carry, and
  `relocateSources` walks the message tree by reflection to apply that name to every locator in both
  `ir` and `geom`. A host reading through `Loader.FS` sets nothing, because an `fs.ValidPath` is
  unrooted already; the CLI sets it from its mount table. **A new output format inherits this and a
  NEW HOST does not**. If a new host opens files by absolute path and leaves `SourceName` nil, it
  publishes the machine that ran it. `--results-out` stored that until agni issue 501. Beware also
  that a query's citation list is SORTED, so anything that changes the source string reorders
  committed captures.
- **A reader records a part number wherever its grammar puts it and NEVER promotes it itself.**
  `classify.StampMPN` is the shared ingestion pass that fills `ir.Component.mpn`, from the component's
  own aliases first and then from `ir.PartType.mpn`. Both halves used to live privately inside the
  EDIF reader, so EDIF resolved part numbers and no other format did. Telesis records it on the PART
  TYPE while every consumer read the COMPONENT, so `component.mpn` came back empty for every
  component of every `.tel` design, silently disabling the whole datasheet tier on that format
  (agni issue 519). A new spelling goes in `classify.MPNAliases`, never in a reader, and a project's
  own goes in its lexicon's `attribute.mpn` (agni 1285). The pass runs in `DeriveDesign`, since it reads
  configuration, and KiCad keeps a symbol's other fields on the SECTION so classification never reads them.
- **Some boards are FETCHED, not committed.** `make samples` pulls a pinned tarball from
  `panyam/agni-samples` into gitignored `tools/samples/`, and `testall` depends on it. Those designs
  are other people's, under their own licences, so fetching them leaves this repo uniformly
  Apache-2.0. `hack/samples.pin` holds the version and a checksum per artifact. **There is no
  offline escape hatch, by design.** Every failure path exits non-zero, and a test reading the
  corpus fatals rather than skips. If a corpus silently failed to arrive, its tests would pass over
  an empty set. The gate takes `samples-oracle`, the 19MB both-views corpus, because the cross-view
  tests read a board file and a test whose fixture is absent skips rather than fails. There is one
  stamp PER ARTIFACT, so `make samples` and `make samples-oracle` compose instead of deleting each
  other's work. The gate fetches `tutorial-board`, `oracle-corpus`, `odb-altium` (foxBMS's
  Altium-written ODB++ jobs), `allegro` (the BeagleBone AI with its Allegro extract and copper films)
  and `cadence-exports`, and `make odb-oracle` adds `odb-kicad`. A pin bump needs the release to
  exist first, or every CI job fails at the fetch, so a samples PR merges and is tagged before the
  agni PR that pins it goes green. A release reuses the previous release's file for every artifact it
  does not change, renamed and byte for byte and checked against the old `SHA256SUMS`, because a
  rebuild on another machine records new file times and moves every checksum. A pin bump restamps every
  tutorial capture over `tools/samples/`, so commit that stamp-only diff with the pin, and two
  branches that both move the pin conflict there and in `hack/samples.pin`. `kicad-cli` writes a `.kicad_prl` beside any board it exports, so an export run over
  `tools/samples` or a fixture folder leaves one there (both export scripts remove it). Since v0.1.2
  the oracle corpus carries a `design.yaml` per real design (entry, board companion, internal
  connectors) and the tutorial tarball does not. That tarball leaves the boards out, and a declared
  companion missing from a tarball fails every read.
- **The public demo's seeds have a verdict for every finding** in
  `hack/seed-triage/<design>.triage.yaml` (agni 857), and `TestSeedTriage` fails on any finding that
  appears, disappears or sits untriaged. A change that moves a seed's findings regenerates the
  records with `make seed-triage` and classifies each new group (`defect`, `design-choice`,
  `needs-intent`, or `false-positive` with its ticket). A class is a judgement in its `why` unless
  an oracle backs it. KiCad's own ERC and DRC are one such oracle (agni 967). `hack/kicad_oracle.sh`
  writes `<seed>.kicad.oracle` beside each record. A group for a rule that KiCad mirrors says
  `kicad: agrees` or `disagrees`, and what KiCad alone reports goes under `kicad_only`.
  `TestSeedTriageAgainstKiCad` holds all of it. **Draw a planted copper fault as a near miss, never
  an overlap**. KiCad reads overlapping copper as one island and names it inconsistently between
  runs, so until agni 1082 it could not check the gateway's faults at all. **`make seed-triage`
  rewrites every record file whole**, reordering keys and requoting `'*'`, so a change that only
  REMOVES records edits them by hand and lets `TestSeedTriage` confirm the result. Restore the files
  before running the update a second time, because it diffs against the file on disk. The records
  that the first run removed would then turn into `untriaged` groups.
- **The samples tarballs are built in `panyam/agni-samples` with its `make dist`**, which since
  v0.1.1 runs on Linux as well as macOS. v0.1.0 was packed by macOS tar and carried a `._name`
  AppleDouble shadow beside every file, and a shadow sorts ahead of the file it shadows, so any walk
  over `tools/samples/` skips `._` names (the oracle's `globCorpus`, agni 812). Under GNU tar the
  old Makefile also wrote an EMPTY tarball that still passed `make verify`; it runs under `pipefail`
  now.
- **After ANY proto change run `make proto` (Go), `make proto-web` (TS) AND `make proto-py`
  (Python).** `make proto` and `make proto-check` set `GOTOOLCHAIN` themselves for their Go
  generation, to the Go version that `go.mod` names. `protoc-gen-go` formats comments with the Go
  running it, and 1.27 rewrote comments in generated files that the change never touched (agni 994).
  A `buf generate` typed by hand gets no such pin. `make proto-check` fails the gate if any of them
  is stale. The Python half uses buf's REMOTE plugins pinned by version, so it needs network, and
  `pyproject.toml`'s `protobuf>=` floor moves with the plugin version. **The protos are TWO buf
  modules in one workspace** (the root `buf.yaml`). One is the engine's `protos/`. The other is the
  datasheet producer's `datasheet/protos/` (`doc`, `derive`, `candidate`, `dsapi`). It generates
  into `datasheet/gen/go` and imports `param.proto` and `workspace.proto` through the workspace.
  `make proto` runs both. The datasheet half is generated from the repo ROOT, because its `go tool`
  plugins resolve against the go.mod of the directory buf runs in, and only the root go.mod pins
  them. The TS step reads the whole workspace (the workbench still builds from `web/`), and the
  Python client reads `protos/` alone, since it is the engine's client.
- **`render --report` prints to stdout and IGNORES `-o`**. This is the same axis confusion as
  `review --coverage`. The report replaces the drawing rather than being a format of it, so the flag
  that names the drawing's file has nothing to write. It exits 0 having written nothing.
- **`-o/--out` writes the `--format` output to a file** on `check`, `review`, `trace` and `query`.
  `-` means stdout and is the default, so nothing that omits it changed. The file is created at once,
  so a bad path fails early, and removed when the run wrote nothing, so a refused run leaves no
  empty file and announces none (agni 1342). It is distinct from
  `--results-out`, which writes the check-result DOCUMENT that `agni results` re-renders. The
  written-file note goes to stderr, so `-o` composes with a pipe.
- **The `--verdicts` list caps NON-ACTIONABLE rows at twenty per rule**, states the elided count, and
  never elides a fail or an inconclusive. One sample board went from 1977 lines to 799 with all 150
  actionable rows intact. A long list is a worse read, but a hidden failure is a wrong answer. csv and
  html are untouched, because a spreadsheet and a scrollable page have no reason to fold anything.
- **The Python client publishes to PyPI as `agni-eda`**, from the release workflow's `pypi` job by
  trusted publishing. The import is still `agni`, since PyPI's `agni` is someone else's. The tree's
  `pyproject.toml` says `0.0.0` and `make python-dist V=...` stamps the tag into a copy, so a
  version is still recorded nowhere but the tag. The gate's `python-dist-check` builds and installs
  the wheel outside the checkout, so a packaging break shows on the PR. RELEASING.md has the
  one-time PyPI setup.
- **The Python client is installed only in `clients/python/.venv`** (`make python-venv`), so a bare
  `python3 examples/…` fails with `No module named 'agni'`. Its CLI transport runs whatever `agni`
  is on PATH unless given `--agni` (or `AGNI_BIN`). That is the stale-binary trap above. A make
  target that runs a client script uses `$(PY_VENV)/bin/python` and `--agni $(CURDIR)/bin/agni`, as
  `python-test` and `exercise-revision-audit` do.
  `clients/python/examples/revision_audit.summary.md` is a golden file. The client's tests
  regenerate and compare it (also under `make examples-test`); `AGNI_UPDATE_GOLDEN=1` rewrites it,
  and its diff is the review.
- **agni's agent skills live in `plugins/agni/skills/<name>/SKILL.md` and nowhere else** (agni 1292).
  The repo root's `.claude-plugin/` and `.cursor-plugin/` marketplaces and `gh skill install` all
  find that folder, so a new skill is one file and reaches Claude Code, Cursor and Copilot without
  a checkout. `plugins/README.md` has the rules and `plugins/plugins_test.go` holds them.
- **When you build a feature, ship an example** (CONSTRAINTS C10; how-to in
  `examples/CONVENTIONS.md`, and `examples/tutorial-project/README.md` for the fixture that the
  docsite tutorial runs on).
- **`AGNI_EXAMPLE_DESIGN` points every example at a board that this repo cannot carry.** Each
  example asks for its design through `common.AskPath`, which defaults to a bundled synthetic
  fixture. The variable replaces that DEFAULT, so the prompt still shows it, a typed path still
  wins, and `--non-interactive`, `--record` and `--replay` pick it up. Those three cannot pick up a
  path typed at the prompt. A blank value is not a value. **A demo over someone's real board is an
  example driven this way**, not a script of its own. `design-review` is the checklist walk,
  `dft-coverage` the coverage one and `whole-enchilada` the tour. Orchestrating beats in bash
  duplicates demokit and produces no recording. **Run an example over a REAL board before trusting
  it**, because the fixtures are small enough to hide scale bugs, and printing every ref-des in a
  bucket read fine at three parts and buried the screen at 531 (agni issue 644).
- **`AGNI_EXAMPLE_REVIEW` is the same idea for a review manifest**, which `design-review` needs
  beside the design, since a team's checklist is as unshippable here as their board. **Both
  variables must be cleared in a package's `TestMain`**, because they replace the defaults that the
  tests assert against. A developer who exports either one to drive a walk over their own board then
  cannot run the gate, and the failure prints their path into the log. That was green in CI and red
  on the machine that had them.
- **The same is true of a personal `~/.config/agni/agni.yaml`.** A test that runs `rootCmd()` or the
  binary reads it, and its mounts or `web_dir` become part of the run. A valid `web_dir` there turned an
  `agni open` refusal test into a real server that hung the suite, and the Python client's tests saw
  its mounts. Point `HOME` and `XDG_CONFIG_HOME` at a temp dir and run from an empty working directory
  (`TestOpenStillRequiresTheViewer`, `clients/python/tests/conftest.py`). CI has no such file, so this
  is also green in CI and red on a developer's machine.
- **The backend (a server or the browser engine) answers, and a client renders.** Sometimes the
  viewer must still compute what the engine computes, because it ranks or spells a subset that the
  server never sent. Then a TWIN fixture holds the two to one answer, and both test suites read it
  (agni 1045). The twins are `core/review/testdata/tally_twin.json`,
  `core/results/testdata/severity_twin.json`, `core/query/testdata/binding_twin.json`,
  `core/facts/testdata/kind_order.json`, `core/ident/testdata/natural_twin.json` and
  `core/check/testdata/subject_param_twin.json`. A new client copy of engine logic gets one, or
  better, comes off the wire (a parse error's caret is `TextPosition.caret`, drawn by the engine).
- **Every question about a design names a snapshot, and only `Ingest` names files** (agni 1061,
  `service/snapshot.go`, "The services" in `architecture/web-services.md`). Ingest resolves the
  tiers and composes the overlay once; ids are random, so two ingests are two snapshots sharing one
  cache beneath. A snapshot records the files that its questions read, and refuses a question once
  one of those files changed (stale, FAILED_PRECONDITION). A host that lost a snapshot answers
  NOT_FOUND. Both answers carry a `SnapshotUnavailable` detail, and the viewer recovers from it
  (`web/src/snapshots.ts`). Snapshots live on the `ProjectResolver` that the services share. So a
  host composing services over two resolvers holds two sets, and a question names a snapshot that
  neither set holds; each service's `Snapshots()` is the SnapshotService over its own loader and
  resolver. Each browser worker holds its own, and `mountingFetch` replays the page's ingest into a
  worker before its first question about it. The CLI ingests, asks and deletes per command, locally
  or over `--remote`.
- **A server and the browser engine keep what a request read** (agni 895, `service.DesignCache`;
  "Kept reads" in `architecture/web-services.md`). Validity is judged on the files that the read
  touched. `formats.Loader` records them through `service.LoaderIn`, so a new host that reads around
  `LoaderIn` is never cached rather than served stale. **The design is kept as a PARSE and a design
  DERIVED from it** (agni 1061). `formats.Loader.ParseDesign` reads no configuration and is keyed
  without the overlay. `formats.DeriveDesign` applies the vocabulary and corpus per overlay. So a
  new ingestion pass that reads a project's config belongs in `DeriveDesign`, or it is kept under
  one project and served under another. **Ask `param.Fetches` whether a corpus fetches over the
  network, never a `Prefetcher` type assertion**. `SpecsOver` wraps every corpus in a `Layered`,
  which always implements `Prefetch`, and asking the interface kept no model for any project with
  `params/` (agni 915). With the fact base kept, **a query's `work` is what THIS request spent**, so
  a warm server can report less than a cold CLI, and the cross-transport test compares `work` only
  where they must agree. **`CheckDesign` keeps each rule's results on the model's cache entry**
  (agni 390, `service/keptresults.go`). So a rule already run on an unchanged design under the same
  overlay answers without evaluating. The kept results have no key of their own, on purpose. The
  entry's key and touched-file check already cover every tier and the overlay. Joining kept rules to
  new ones relies on `check.RunAll` sorting STABLY; `sort.Slice` there reorders tied findings and
  the join stops matching one run.
- **A query answer keeps each entity and citation once** (agni 916), in `RunQueryResponse.entities`
  and `sources`. Rows point in through `cell_entity` and `cite_index`. Read a cell's location with
  `service.CellLocation` and a row's citations with `service.RowCites`. The viewer's tests write
  fixtures row by row and normalize them through `web/src/testwire.ts`. The Python client speaks
  binary protobuf by default (`ConnectTransport(url, binary=False)` for JSON), and `omit_locations`
  on a query drops the drawing-derived fields for a caller that draws nothing (agni 917).

## Engine gotchas

**A declared pin map is an intent form, and it compiles to up to FIVE rules.** `io_map` in a
design's `intent:` section says which net connects to which pin of which device. Three rules ask
whether the design kept that promise (`io-map-pin-mismatch`, `io-map-net-absent`, `io-map-far-end`).
The fourth, `io-map-coverage`, inverts the question. It takes the NETLIST as its considered set, so
it reports how much of the design the map never mentioned. That number usually matters most. A map
declaring two hundred of sixteen hundred nets leaves fourteen hundred UNEXAMINED, and unexamined is
not clean. An undeclared net is `not-considered` and never a fail. Rails remain in the denominator,
and their reason says so, because excusing them would be the tool deciding which absences are
acceptable. The fifth, `io-map-function`, compiles when a row declares a `function`. It checks that
function against `Pin.alternates`, the functions that the part's pin-mux table lists (agni 667, a
third recipe table kind, `MuxTableRule`, beside the parameter and pin-function tables). It needs the
datasheet tier, so a run with no corpus SKIPS it and the pin map's `function_check` column reads
empty rather than not-considered. `check --verdicts --format csv --table pin_map` writes the map one
row per pin.

**Comparing an identifier across two documents goes through `core/ident`, never `==`.** A pin map is
authored in the datasheet's vocabulary and a netlist answers in package designators, and both carry
zero-padded indices, invisible characters pasted out of a PDF, and cells naming several functions at
once. We measured a shipped in-house checker on a real board. EVERY warning it produced was a
string-comparison artifact, and none was a design defect. `Canonical` is the one canonical form,
used on BOTH sides. `\s` does NOT match U+200B, so stripping `\s` is the defence that looks right
and is not. **A pin name is compared loosely and a NET name is not**, because a vendor table is
inconsistent with itself while a net name is the design's own identifier. `ident.PinName` is the
loose key. It folds underscores AFTER the index rewrite, so `IO0_1` never meets a different `IO1`.
`ident.PinNumber` keeps a designator's zeros (agni 702). **A fuzzy match needs BOTH ends to agree**
(`tokenSubsequence`). `GMAC0_TXD2` meets `GMAC0_MII_RGMII_TXD2` loosely, while `TXD` never meets
`UART0_TXD`, and a single token is never fuzzy. A test of a loose match has to use an anchored pair
or it tests nothing. **A caller comparing many against many builds a map keyed on `Canonical` rather
than nesting `Compare`**, since two hundred rows against sixteen hundred nets is 320,000 comparisons
however fast one is.

**`pin.name(ref_des, pin, name)` is the pin's FUNCTIONAL name**, beside the package designator that
every other pin relation keys on. Both have always been in `ir.Pin` and only the designator was
projected, so a question asked the way a datasheet asks it had nothing to read. It is projected
VERBATIM, so canonicalizing belongs on the comparison and never at the projector.

**The security scanners run through `go run` at versions that the Makefile pins** (`STATICCHECK`,
`GOSEC`, `GITLEAKS`, `GOVULNCHECK`), over the two shipped modules, the root and `datasheet`. `lint`
(staticcheck), `seccheck` (gosec, medium and up) and `secrets` (gitleaks over the whole history) are
in the gate. `vulncheck` is not in the gate, since the advisory database moves without the code. It
runs weekly and per tag in `vulncheck.yml`, and `make audit` runs all four. A gosec finding is fixed
or annotated on its own line as `// #nosec G<n> -- <why>`, never excluded in the Makefile, so the
reason reaches an outside scanner too. **gosec runs under the Go `go.mod` names (`GOTOOLCHAIN`)**,
because under a newer Go its SSA rules (G115, G703, G710, G112, G122) skip silently. A local 1.27
run read clean where CI found 100 findings. A narrowing of a count or coordinate goes through
`core/num`, which saturates, since a design file can come from anyone and a wrapped `int32` reads as
a real value. gitleaks installs as `github.com/zricethezav/gitleaks/v8`, its module path, whatever
its repo is called. `SECURITY.md` is the reporting policy.

**`make oracle` is a separate suite and is NOT in the gate.** It cross-checks the KiCad reader
against real boards, comparing the pin-to-net PARTITION against each board's own `.kicad_pcb` rather
than net names (auto-named nets differ by tool) or counts (compensating errors cancel). It asserts
`readers/kicad/oracle_corpus.baseline`, a committed list of the nets we still get wrong;
`AGNI_ORACLE_UPDATE=1 make oracle` rewrites it. It also runs `odb-oracle`, the ODB++ cross-check.

**The docsite is its own Go module**, so `go test ./docsite/` from the root fails with `[setup failed]`;
run `make docsite-test`.

**`make browser-test` is IN the gate**, and needs a Chromium on the machine (`cd web && pnpm exec
playwright-core install chromium`). It drives a real browser against a real server for the handful
of assertions that need layout, because jsdom has none and a panel can be present in the DOM and
invisible to a reader. v0.2.0 shipped a viewer that way, with its query surface booted hidden behind
the Trace tab. It was outside the gate until PR 629. Read `docsite/content/build/the-gate.md` for
what belongs in it, and `build/evidence.md` for the two ways a layout assertion passes while proving
nothing. **Suppose a spec times out on CI waiting for a result, in a different test each run, and
passes locally. The cause is often a click landing in a boot window**, which is 20 ms wide locally.
Sample the element's state every 20 ms from first paint with the wasm download held back by
`page.route`. PR 1114 found a Run button enabled before any design had opened that way. **`HEADED=1`
shows any browser spec in a visible Chrome** (`make browser-test HEADED=1`, `SLOWMO=<ms>` paces it).
**One spec run by hand uses whatever `web/static/agni.wasm` and `bin/agni` are on disk** (`cd web &&
pnpm exec vitest run -c vitest.browser.config.ts browser/<spec>`), while `make browser-test`
rebuilds both. So run `make wasm agni` after a Go change first; a stale wasm read a fixed proposal
as still broken (agni 1136). **Mission #845's walk is `web/browser/review-walk.spec.ts`**, in the
gate, and `make exercise-browser-review` runs it alone on Jetson (`WALK_DESIGN=gateway` for the
small board that the gate walks) and prints the step table for the mission log. It takes the
viewer's review tour (`web/src/reviewtour.ts`, `?tour=review&tourpace=host`, tsappkit's `Tour` over
the events in `viewerevents.ts`). HEADED by default the reader does each tour step, `STEP_BY_STEP=0`
has the walk do them, and an agent logging the mission passes `HEADED=0` or the run waits on a
reader. Headless the walk acts through the tour's holes, so a tour step whose hole or event broke
fails the gate. **A Playwright click scrolls its target into view first, so it passes over a tour
step that a person cannot take**. The finding and ask-about-the-block steps once showed the whole
page dimmed with no hole, since their controls sat below their panel's fold and the dimmed page
takes no wheel. Each step now scrolls its control into view (`reveals` in `reviewtour.ts`) and the
walk fails a released step with no hole. Look at a capture of each step (`page.screenshot` after the
release) before trusting a tour change, as #1179 did. A `:has()` locator over a large answer is
re-resolved on every action and cost Jetson's query-cell step 9 s, so find a row by index. A viewer
change that breaks the walk fails it on that PR, so update the spec there rather than loosening it
(agni 1118). **A headed run is a different page from a headless one, and four things only it
showed** (agni 1124). First, a headed page takes the window's own size (`viewportFor` in
`web/browser/browser.ts`), because a 1000 px page in a shorter laptop window made Chrome pan to
every click. Second, the guided walk freezes the page while a panel waits, and blocks clicks while a
step runs, since a reader exploring between stops changed the page under the walk. Third, the walk's
own clicks lift that shield through `act()`, because `Input.setIgnoreInputEvents` drops Playwright's
dispatched input as well as the reader's. Fourth, a query tab is opened in the page (`openTab`),
since a real click on one timed out in a scrolled strip. Headless CI still uses 1400x1000 and never
pauses. **The dock's boot sizes are fractions with floors** (agni 759, `web/src/dock.ts`). Query
opens at a fifth of the height but at least 320 px and at most 45%, and the east rail at least 260
px. **A layout that the browser saved keeps its old sizes** until the reader picks Panels → Reset
layout (`resetLayout`), so check a dock change in a fresh profile. A spec that needs a taller Query
panel drags the sash (`growQueryPanel` in `web/browser/panels.ts`); moving the panel's overlay
instead leaves the sash across the result rows, where it takes their clicks.

**A module that BUILDS is not a module that is tidy**, and the gate now asks both. `tidyall-check`
runs `go mod tidy` over the root and every example module and fails on any difference (~5s, near the
front so "updates to go.mod needed" is diagnosed there rather than as a later `examples-test`
failure). Eleven example modules had drifted before anything looked, because an untidy module keeps
building until some later change needs a requirement that it never recorded. `make tidyall` fixes
it. `.github/dependabot.yml` lists every module directory (examples by glob) and groups security
updates, because a bump that touched the root alone left every nested module stale and failed this
check (PR 782). A new module outside `examples/` needs a line there.

**`make testall` is the full gate, and CI runs exactly it**, as four parallel jobs, `testall-test`,
`testall-scan`, `testall-docs` and `browser-test` (agni 1147). `testall-core` is the first three. Read `docsite/content/build/the-gate.md` before
trusting a run. It lists three ways to misread the gate. A wrapper can report its own status rather
than make's and turn red green, there is a commit-first ordering rule, and there is a per-clone
`pnpm install`. It also lists one way a fine tree reads RED. A stray `agni serve` on :8080 fails
three verdict-link tests, so reproduce against unmodified `main` before reporting a regression. The
page also covers what a run leaves behind, and the generated-code rules. **From a worktree run `make
-k testall`**, since a plain `make` stops at the first failure before `python-test`,
`tutorial-runs-check` and `docsite-images-check` ever run. **Build in a worktree with the default
Go, never `GOTOOLCHAIN=go1.26.9`.** Go 1.26 finds a repository only by a `.git` DIRECTORY, and
1.26.9 does too. A linked worktree's `.git` is a file, so Go 1.26 runs no git and stamps `(devel)`
(go1.27 fixed it, golang/go#58218). A `(devel)` wasm has an empty `StoreVersion` and keeps nothing,
so `browser-test` and rung 12's capture fail, and a timed walk reports `design=built` where it
should read `design=store` (#957). **`tutorial-runs-check` regenerates captures and does not read
the prose quoting them**, so a tutorial can cite numbers that a change moved, and the gate is still
green. A capture step that exits non-zero and captures nothing is a build error unless its spec sets
`exit: true` (agni 825). A checkout from before agni 853 may hold an untracked `web/static/seed/`
that `fixture-copies-check` flags; delete it.

**A design's tiers are resolved ONCE, in `service.ResolveSources`, and both surfaces call it.** For
most of 2026 they did not, because only `SourcesFor` was shared. The half above it decides whether a
ref names the design, its entry, a declared companion or an unrelated file, and that half lived only
in `cmd/agni`. So the CLI attached the schematic companion that a descriptor declared, and the
server did not. One design read two ways gave 3980 components on a computed layout or 82 real sheets
with no counts, depending on which name you typed. Nothing errored, because "no geometry" is a
legitimate thing for a design to be. Four symptoms followed, and each read as its own defect. They
were findings with no sheet badges, query cells reporting `LOCATE_REASON_NO_GEOMETRY`, `trace
--render` writing the first sheet whatever the route crossed, and every minted link opening on an
auto-layout. C32 is the rule, agni 656 the fix. `as_named` travels on the wire because the CLI is
itself a client of these services. Resolving unconditionally overrode the CLI's own flag.

**656 left one call site and it took another ticket to find.** `CreateReview` never resolved tiers,
so a review scored in the browser read no board while the CLI read the declared one, and board-tier
checklist items answered `not-applicable` against the CLI's `fail` (agni 646). The MECHANISM lets
two surfaces disagree with no resolver involved. Given no board, `BuildModel` asks the loader for
one at the DESIGN's own URI, and the CLI's `localLoader` resolves a descriptor inside `Board` while
the served `osLoader` reads exactly what it is handed. So a loader can attach a companion that the
service never resolved. "The CLI reads it" is then evidence about the loader rather than about the
request. Since agni 1061 a question names a SNAPSHOT and the tiers are resolved once, by
`SnapshotService.Ingest`, so no analysis rpc can skip the call. A new rpc takes `snapshot` and reads
`Snap.Netlist`/`Board`/`Geometry`, and `TestC22AnalysisRequestsNameASnapshot` holds every analysis
request to that.

**A trace answer carries where each net and endpoint is DRAWN**, per net rather than one sheet for
the answer, because a route crossing three sheets is when a reader most wants to choose (agni 657).
An ENDPOINT resolves by its PLACEMENT and not by its net, because it is a pin on a part, and
resolving by net first put a route on a sheet carrying the middle net and none of the parts. It is
filled on a no-route too. The two nets that fail to join are drawn somewhere, and someone goes
looking for that picture. Empty then means drawn nowhere rather than nobody looked. The CLI takes
the sheet off the response rather than choosing, so C32 holds at one more call site.

**Opening a design must not parse its board** (agni 943). `GetDesign` lists the Board sheet from the
board tier's format (`formats.HasBoard`), and the board is read when its sheet is opened or a
board-tier rule runs. Reading it on open made the Jetson seed's first sheet and rule catalog wait
about 30 s in the browser, since everything on the worker queues behind the request that opens. The
Load page reads it before the viewer opens (agni 1072). It calls `GetDesign`, then an `Ingest` with
`read_board` on a `prepare` worker. That worker ends when it answers, and leaves the parsed board in
the shared store for the jobs worker to restore. The viewer opens only a design that the Load page
recorded (`web/src/admit.ts`, `loaded.ts`) and sends any other to `/load` with its address whole.
**Nor must a query that asks nothing of the copper** (agni 987). A model is built with a
`BoardTier`. Rule runs pass `ReadBoard`, a query passes it only when `query.ReadsFrom` names a
`board.*` relation (`boardTierFor`, which sees through its rules and library members), and coverage
and part parameters pass `SkipBoard`. On Jetson a model with the board holds 547 MB, and the board
is 385 MB of that. A new surface that builds a model chooses its tier, because `BuildModelCached`
reads the board. **The browser's serve worker is not even given the board's bytes** (agni 992).
`ListDesignFiles` flags the board tier's file (`DesignFile.board`), and the serve lane gets a
placeholder at its path. An engine that opens a placeholder refuses every question until the files
arrive (`internal/wasmengine/withheld.go`), so `mountingFetch` brings the board and asks again. A
new serve-lane surface that reads the board needs no change for this, and a page-side rule deciding
which rpcs read the board is the bug shape, since a library member can read `board.*` under a query
that never names it.

**A jaala bump can change a rule's plan, so time Jetson's Run checks across one.** v0.1.57 started
the ESD profile rules from the TVS index and walked the board once per TVS net, about 10 times
slower, until v0.1.60 (agni 1037). Bisect by building agni against each release and timing one rule
with `check --rule <name> --timing`; `stdlib/profiles/esdplan_test.go` holds that shape to a work
budget. **A length is published in metres and shown in millimetres** (agni 1035). The engine folds a
query literal to its SI base, so a threshold over a length writes its unit (`?w < 0.2mm`), a bare
`0.2` is metres, and `query.Display` turns a metre value back into mm wherever a value becomes text.

**Measure a slow request before optimising it** (agni 914). `--timing` on
`check`/`query`/`trace`/`review` prints each stage, which cache tier answered each read and the
slowest rules; `query --explain` prints the plan; `agni serve --slow-request 2s` logs the same per
request; and an `Agni-Timing: 1` header (the viewer's `?timing`, the exercise's `--timing`) returns
it from the server or the browser engine. A stage is recorded with `timing.Begin(ctx, "name")()`
from `core/timing`, a no-op when nobody asked, and the record travels in headers so no answer
changes run to run. Under `?timing` the page also logs each request's start and wall time on its own
clock, and the exercise's `--timing` turns them into a `checks breakdown` row, every request of a
Run checks plus the page's own time (agni 968). **A walk reports loading a design apart from asking
it questions** (agni 1060). `1. load` and `checks load` happen once per visit. `checks rules`,
`checks again` and the query rows show what a question costs on a loaded design, so a mission log
leads with those, and `wasm-bench` prints its rows in the same two groups. **A browser worker's
MEMORY is measured with `make wasm-layers` and `make wasm-bench`**. `make wasm-layers` shows what
each layer of an open design holds, native and wasm, with `GOGC` and `GOMEMLIMIT` passed through.
`make wasm-bench` shows each lane's wasm memory beside its live Go heap. Wasm memory never shrinks,
and collecting harder does not lower it reliably. A lower GOGC or a memory limit peaked HIGHER than
the default on Jetson (agni 988, recorded on `cmd/agni-wasm`'s `main`). One collection after each
layer that the cache reads does lower it. The hook is `DesignCache.AfterLoad`, and only the browser
engine sets it. Jetson's check went from 1343 to 805 MB (agni 1000). **The browser engine's
automatic collection is OFF** (agni 1010). Go's wasm port has one thread, so a concurrent cycle ran
beside a rule and took Jetson's check from 2.6 to 7.8 s. The engine now collects only after a layer
loads, after a request that doubled the heap, and between a read's files once the heap is 128 MB
past what last survived (`wasmengine.Options.CollectBetweenRequests`), since a wasm memory keeps its
largest size. If a rule suddenly costs seconds in the browser and milliseconds natively, it is this
shape, so read `make wasm-layers GODEBUG=gctrace=1` before the rule. **`Agni-Timing: memory` shows
which stage raised a peak** (the page's `?timing=memory`, the CLI's `--timing-memory`). Each stage
carries the memory as it ended and each rule what it allocated, and `make wasm-bench` prints both
for a Jetson check (agni 994). **Two of the bench's rows mislead.** The one-worker `check_ms` is a
check after an aborted one, so it reuses the first run's work or not depending on where the abort
landed, and moved from 2.3 s to 5.0 s between identical runs. `query_after_restart_ms` with
`lane_restarts` 0 is the same query repeated on a warm worker. Compare a page's wall time with the
engine's `totalMicros` before blaming the engine, since a warm Jetson query once spent 60 of its 100
ms outside it (agni 1007).

**A rule's `Eval` takes the caller's context, and a run stops when the context is done** (agni 795).
`check.Run`, `RunVerdicts` and `review.Run` return `ctx.Err()` rather than the findings so far,
since those would read as a clean remainder; `RunBackground` is the context-free form for tests and
one-shot tools. Every datalog evaluation reads its WORK BUDGET off the same context through
`query.EvalOptions(ctx)` (agni 792), so a new `Eval` call site that omits those options escapes a
served budget silently. `agni serve --query-budget` enforces one, OFF by default, and
`--query-budget-warn` logs each costly query with a suggested budget; a request's `work_budget`
narrows the server's and never raises it. A stopped query is `resource_exhausted` and a stopped rule
one inconclusive finding.

**A rule proves a PASS with entities, not only with prose, and there are two mechanisms.** A Go-walk
requirement returns its hops (`check.PullUpVerdict` emits every resistor and net as ordered context
with a role). A query-backed one needs `Domain.Evidence`. That is a third goal, run over the passing
set, and its rows project through the rule's own `ContextVars` (agni 662). It is a third goal rather
than a wider domain. The domain is the CONSIDERED SET, passing and failing alike, and a failing
subject has no proof to name. So no single query can both enumerate the failures and bind the
evidence. On the ESD requirement the evidence is a SECOND head (`esd_by`) rather than a widened
`esd_ok`, because the finding goal NEGATES `esd_ok` and a negated atom must be unary.

**A connector's exposure to the field is DECLARED in a design's intent, never classified** (agni
831), as `intent: components: {J1: {exposure: internal}}`. The same part is the module socket on one
carrier and the cable entry on another, so `internal_connector` was removed as a class. The exposure
rules ask `Model.ExposedConnector`, so intent reaches the MODEL as well as the catalog. It travels
as a value (`Overlay.DesignIntent`) through `ov.ReadOptions()` into `BuildModel`. So a model built
without the design's overlay checks every connector as exposed. A request's intent (`--intent-path`)
REPLACES the design's for that run, and there is no server-wide intent. Two neighbouring rules came
out of the same work. Only a request that sends its OWN convention replaces the server's
`--conventions`, where one carrying any other config used to drop it. And some surfaces read a
profile rather than running it, such as the coverage panel and a review's interface-presence gate.
Those surfaces ask `Overlay.ProfileIndex`, because a project's profile supersedes the built-in of
its name. They used to describe the built-in while the rules checked the project's (agni 833).

**A thermistor is a `thermistor` AND a `resistor`**, the family shape that `ferrite`/`inductor`
already uses (agni 627). Before, it classified UNKNOWN. UNKNOWN emits no `component.class` row at
all, so a thermistor fell out of every class-scoped rule and query silently. **A project can EXTEND
a class that the engine ships and cannot declare a new one.** `lexicon.class` takes `patterns` and
ref-des `prefixes` keyed by a shipped class, and a name the engine does not know is a load error.
The names that a config may use are derived from `model.ComponentClasses`, and a test reading the
const block holds that list to it. That list replaced a hand-kept copy. The copy lacked
`thermistor`, `zener` and `ideal_diode_controller`, so a project could not extend the class that 627
had just added (agni 677). Declaring a new class opens the vocabulary, and DECISIONS.md says that
happens as a registry, once a project asks.

**A device class has TWO evidence tiers and one ordering, and both halves fail silently.**
`ir.Component.device_classes` is a set of TAGS, each carrying a `ClassSource` (agni 710, the second
instance of C9's evidence-tier variant after `ir.Net.roles`). `classify.Stamp` is the convention
tier; `classify.StampClassesFromSpecs` is the datasheet tier. The datasheet tier is the only
evidence that separates a crystal from a ceramic resonator or names an `ideal_diode_controller`. The
structural path is documented as unable to resolve either one. **Two orderings inside
`Loader.ReadDesign` are required, and getting either wrong is silent**. The convention pass REPLACES
the set, so anything written before it is erased. The datasheet pass joins on the MPN that
`StampMPN` fills, so it must run after `StampMPN`. **The datasheet tier runs in TWO places on
purpose**. The Loader runs it when the read carries a corpus, and `check.Model` runs it when it is
handed a corpus that the read did not have. Two producers are safe only because the pass is additive
and idempotent. Adding a THIRD producer is the bug shape, not the pattern.
**`classify.BySpecificity` decides which of several true classes is the headline one, and
`MostSpecific` is its head rather than a second walk**. The drawing and the model ranked one set
independently for as long as the set had one author, and agni 710 was the result. An unranked vendor
class sorts BEHIND the keyword class rather than displacing it; `regulator` is one, and the tutorial
corpus really states it. **A second tier must never be the ONLY tier in the set**, because EMPTY is
the signal that tells a consumer to re-derive. So `WithParamProvider` runs the convention pass first
on a design that reaches it unstamped.

**A KiCad design reads its same-stem `.kicad_pro` whichever of its files names it** (agni 933). It
reads net classes, class definitions and the board's own minimums (`board_rules`, which the copper
rules check against before their fixed floors). A sub-sheet has a stem of its own, so it reads
alone. **A do-not-populate part stays in the netlist, and a rule crediting a part asks
`Model.IsFitted`** (agni 938, 961). The reach walk crosses no unfitted part, and a query says the
same with `not component.dnp(?r)`. Two exceptions are deliberate and each cost a wrong first
version. A TEST POINT keeps its credit. A pad-only test point is marked DNP because nothing is
placed, and a probe lands on the copper (every test point on the Feather seed). And crediting alone
fails a circuit that is unfitted as a whole. So a rule whose credit changed also leaves out a net
with no fitted part (`check.Unpopulated`, spec fact `net.unpopulated`). A rule that BLAMES a part
judges no unfitted part. A rating or orientation rule is wrapped in `check.OnFittedParts`, which is
opt-in, since two unfitted parts sharing a ref-des are still a defect. `floating-input` counts
fitted pins, a connector's included (agni 1082). `architecture/rules-and-checks.md` states the
policy.

**A 0 Ohm link splits one rail into two nets, and a rule asking what sits on a rail asks
`check.FirstOnRail`** (agni 1015), which crosses fitted zero-ohm links and never a ground. **A value
written as a library's part name is read from the name at ingestion** (`ValueVocab.valueFromName`,
agni 1115). `R_10k_0402` and `C_4u7_25V_0402` are examples, and every passive on Jetson is written
that way. The value is one token with an SI prefix or a unit, in the class's own unit. So a package
size (`0402`) or a rating (`25V`) never reads as the value, and a name with two different values
reads as none. **A connector on a net that the board drives is an outlet** (agni 1016).
`check.BoardSource` finds a switch node behind an inductor plus a feedback net, or a `power_out`
pin, and the power-entry rules mark the net not-considered. A vendor buck types every pin passive,
so the structural half matters. A `power_out` pin does not count when the walk also reaches another
power-converting part's input, so a dual-role USB-C port is still judged. A boost converter's input
also reaches its switch node, and the feedback requirement means it is still judged. Two more shapes
count (agni 1253). One is a part's switch pin behind the inductor with its feedback pin on the walk,
read from PIN names, since a fixed-output buck ties FB to its output. The other is a regulator fed
from a net that none of the parts it sources drives. That is a cascade rather than a port
(`independentDriver`). The walk crosses a `net_tie` as it does a 0 Ohm link, so a net tie no longer
splits a rail. The rule pages describe the shapes. **A connector or a clamp is never the load**.
`decoupling-present` skips a connector's own `power_in` pin and `hasPowerInput` skips a TVS's, so a
USB clamp's supply pin does not read as a load reached through passives alone (agni 1254, 1257).

**A rail-named net is not always a rail.** A regulator's pins are named for the supply they produce,
so `12V_FB`, `12V_SW`, `12V_MODE1` and `12V_VDRV` all match the rail vocabulary and none carries
12V. Four roles say so, and `Model.IsRailNet` subtracts them ONCE rather than per consumer. Before,
seven rail-quantified consumers each had to exclude them, and only one remembered (agni 679, 680).
`guide/naming-conventions.md` has the six net vocabularies and when to narrow one. DECISIONS.md says
why the two voltage relations stopped being exhaustive.

**A variable's entity kind survives a DERIVED relation, and the three rules of that walk are worth
knowing before you widen it.** The engine's `ColumnKinds` types a projected variable from the
catalog relations in the goal, and then follows user rules into their bodies. `ColumnKinds` is
jaala's, and `service/query.go` maps it since agni 751. So a bucket built with negation is clickable
without repeating a catalog atom to re-establish a kind that the rule already knew (agni 654). It
follows more than one hop. Rules that DISAGREE about a head position yield a scalar rather than the
first one written, because a column typed from whichever rule came first is wrong for half the rows.
A rule wrapping `entity(?name, ?kind)` yields a scalar, since that kind is per-row and a head
argument has no per-row identity to carry it. And a recursive clause ABSTAINS rather than vetoing,
so a transitive closure is typed by its base case instead of collapsing to a scalar.

**There are three ways to read a design and get a confident WRONG answer.** The first two were hit
in one sitting and are silent. A plain `check.NewModel` used to be a fourth, leaving `component.mpn`
empty, until agni 748 made every model join the design's MPNs. Each returns an empty or partial
answer rather than an error, and that answer reads as "the design does not have that".

- **The CLI's mount table is parsed ONCE per process** (`cliWSOnce`), so in a `cmd/agni` test the first
  test to resolve a mount fixes it for every later one. A test whose output names a design by its path
  within a mount, or that needs an `agni.yaml` mount, calls `freshWorkspace(t)` (or `withMount`).
  Without it a test passes alone and fails in the package run; agni 869 looked like a review scoring
  differently after a diff and was only `review/can-broken.edn` printed for `can-broken.edn`. One
  more order dependency is parked as #875; `-shuffle=on` finds them.
- **A bare reader skips what `formats.Loader` runs.** The format-neutral passes run in the Loader.
  So under a bare reader `classify.StampMPN` never fires, every component's `mpn` is empty, and the
  whole datasheet tier is empty. This is agni issue 228's shape, and it recurred in
  `examples/common` (issue 618) because the examples are their own Go modules, outside the root
  build and outside the wiring table.
- **A missing registration blank-import empties the catalog.** `check.BuiltinRules()` returns
  nothing without `_ "github.com/panyam/agni/stdlib/rules/builtin"`, and a verdict sweep then
  reports "0 pass, 0 fail, across 0 rules". Three of the four registration points fail this way;
  see the composition facade note above.
- **A HIERARCHICAL `.edn` reads as a flat design missing most of its netlist.** The EDIF reader
  scopes instance extraction to the design's root cell, correctly (WS1-004), so every sub-cell's
  contents are absent and every rule evaluates over the top cell alone. This one is no longer silent
  (agni issue 707). The reader lists each skipped cell with its instance count in
  `InputDiagnostics.unexpanded_hierarchy`. Every CLI read prints a stderr note naming them, `agni
  stats` adds a `not extracted:` line, both HTML reports state it in their header, and the viewer
  shows a canvas strip and lists the cells in Properties (agni 717). Every one of those says the
  sentence that `report.HierarchyNote` builds. A report gets the list off the RUN
  (`CheckDesignResponse` and `DesignRef` carry it), so it reads no design to say it. Unflattened it
  is still WRONG, so read the note rather than the counts. It is deliberately not a finding, since
  being hierarchical is a limit of the read rather than a defect. **`--flatten-hierarchy` reads the
  blocks** (`Snapshot.flatten_hierarchy`, agni 718). It is OFF by default because it renames every
  interior net and moves a diff. It travels in the OVERLAY so that its identity keys every cache
  tier, and the parse key names it too; a new read path that builds options without
  `Overlay.ReadOptions` drops it silently. A design declares it once with `flatten_hierarchy: true`
  in `design.yaml` (`ProjectResolver.Overlay` ORs it in). The viewer asks with `?flatten=1`. The
  strip's Read every block button adds that parameter, and the page's ONE snapshot registry carries
  it into every ingest (agni 1102). Its naming rules are in `architecture/ingestion-and-ir.md`, and
  it is unverified on a real multi-cell export.

**Before believing a measurement or a green test, read `docsite/content/build/evidence.md`.** A
negative result needs a positive control, a positive rate needs a precision check, and every new
test needs a red-check. Most of the expensive mistakes here have been correct-looking results that
nobody could have falsified. **A test that loops over rows passes over ZERO rows**, so give it a
count guard; the agni 727 red-check found two such tests still green with the projector turned off.

## Wiring, per subsystem

Each of these has a fixed edit-list where missing one edit is silent. Most have a test that catches
it, and where the last column says NOTHING the gap is listed so it is visible rather than
discovered.

| Adding | Edits | Read | Enforced by |
|---|---|---|---|
| A docsite page | 4 (5 for a new section) | `docsite/README.md` | `docsite/nav_test.go` |
| A `learn/` chapter | 4, plus the level-index entries | `docsite/README.md` | `docsite/learn_levels_test.go` |
| A web viewer panel | 4, plus 2 more if it docks | `docsite/content/architecture/web-client.md` | `web/src/composition.test.ts`, `dock.test.ts` |
| A canvas note strip (undrawn, stale-link, hierarchy) | 5 | `web/src/undrawn.ts` and `web/src/stalelink.ts` as the two worked examples; the hole goes inside `.canvas-notes` in `ViewerPage.html`, which stacks the strips so two never paint over each other | the compiler for the `ViewSink` channel, `composition.test.ts` for the template hole |
| A web page | 6 | `docsite/content/architecture/web-app.md` | its own boot test (one per page) |
| A field in the viewer shell (`ViewerPage.html`) | 3 (the template, the page struct in `cmd/agni/webpage.go`, `servedFields` in `web/src/composition.test.ts`) | `docsite/content/architecture/web-services.md` | `composition.test.ts` throws on an unfilled field; `agni site` renders the same struct |
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
| A new EVIDENCE TIER for a derived IR field | 7 (the pass, the source enum value, the `Loader` field, the `Loader.ReadDesign` call, `service.ReadOptions` + `Overlay.ReadOptions`, the `check.Model` call, `hack/ir_model_baseline.txt`) | C9's evidence-tier variant, `architecture/ingestion-and-ir.md` | the C19 ratchet catches the pass; NOTHING catches a tier that reaches one surface and not the other, and agni 710 was that bug |
| A host that reads designs | 1 (go through `formats.Loader`, never a bare reader, and through `service.LoaderIn` so its reads can be kept, agni 895) | `build/evidence.md` | `TestReadCarriesTheIngestionPasses` in `examples/common`; nothing guards a NEW host |
| An engine-drawn image that the docs embed | 1 (a line in `docsite/images.sh`, then `make -C docsite images`) | `build/the-gate.md` | `docsite-images-check` |
| A hand-authored diagram | 2 (the file in `docsite/figures/`, one `{{ includeFile }}` in the page) | `docsite/README.md` | `docsite/includefile_test.go` |
| An architectural constraint | 3 (the rule in `CONSTRAINTS.md`, a test in one of four homes, a `Verify` naming that test) | `build/the-gate.md`, and `CONSTRAINTS.md`'s own header | the test you wrote, and NOTHING checks that a rule has one |

**The relation row's sixth edit is `service/testdata/columnkinds.golden`**, which snapshots how every
query column types. The table said five for a year and CI caught the miss twice in one week (agni 687,
693). Regenerate with `UPDATE_GOLDEN=1 go test ./service/ -run TestColumnKindsMatchGolden`, and READ
the diff, because a new relation only adds lines and anything else means an existing column
retyped. **The seventh is `dimensionlessNumericRelations` in `stdlib/relations/facts_test.go`**, for
a relation whose number is a bare count, since `TestNoRelationPublishesAnUnlabelledNumber` fails
until the count is listed there on purpose (agni 727).

## Issues and missions

Work is aimed at MISSIONS, issues labelled `mission` that state something a person does with agni
and the command that exercises it. Several may be active at once, one per worktree. #1321
`mission_cross_probe` (find a schematic part on the board beside you, and back) is active, and `make
exercise-cross-probe` is its exercise. Its walk runs in the gate on a cut of Jetson
(`examples/cross-probe-project`) and names, per step, the tickets it waits on. A waiting step that
passes FAILS the walk, so the PR that makes a step pass also removes its wait in
`web/browser/cross-probe-walk.spec.ts`. The make target also asks GitHub which of those tickets are
closed (`hack/closed_tickets.sh`), and a step waiting only on closed ones must pass. That catches a
ticket closed by a PR that never touched the walk, which is how #1326 closed while the walk still
read its steps "not available". The gate runs offline and skips that half. #1194
`mission_pin_functions` (sign off a board's pin map against the netlist and the datasheet) is
active, with all six tickets closed, and `make exercise-pin-functions` is its exercise. It closes on
a team's real-board run in a clean room (`guide/trying-agni.md`). That run sends back only `agni
trial`'s report. #843 `mission_revision_audit` (2026-10-04), #851 `mission_public_demo`
(2026-10-20), #909 `mission_ask` (2026-10-22), #1157 `mission_viewer_page_api` and #845
`mission_browser_review` (both 2026-10-25) have closed. Their exercises still exist as the example
that #843 built, `make exercise-public-demo`, `make exercise-mcp-ask`, `make exercise-viewer-page`
and `make exercise-browser-review`. #1176 `mission_shared_viewer_rooms` (several people on one
server) is open and not active, since its one ticket, #1173, waits on a multi-user deployment. A
mission is blocked by each ticket it needs, and its tickets carry its `mission_<slug>` label. **A
new issue gets a priority (`P0`-`P3`) and either a mission link or `waiting` with its trigger when
it is filed.** A P1 states which P1 it displaces, since the cap is five per mission. NEXTSTEPS.md
names each mission's next ready ticket. Each mission has an exercise in its issue body, and a make
target where one exists. The targets are `make exercise-revision-audit`, `make exercise-mcp-ask` for
#909, and `make exercise-public-demo` for #851. `make exercise-public-demo` walks the built static
demo or, with `DEMO_URL`, the deployed one. After a mission PR merges, run its exercise and log the
result as a comment on the mission issue. Against the deployed demo, wait for the `docs.yml` run and
then about a minute more. A walk once started as Pages flipped, and the first seed hit a 180 s
timeout. Pages serves the design files with `max-age=600` and a fresh deploy's are uncached, so
Jetson's 85 MB board arrives at about 3 MB/s on the first walk after a deploy. The open row names
the slowest file fetch, and a timed-out open names the files still pending (agni 1002).

## Working in this repo

`CONTRIBUTING.md` holds the workflow rules. It covers running several checkouts in parallel (use
`git -C <abs-path>`, never `git add -A`) and the PR workflow (verify a push by its exit code, verify
`merged: true` via the API, never `gofmt -w` a directory). It covers the shell traps that have lost
real work. It also covers what agni ADDS to the PR body shape that the `start-pr` skill defines.
agni adds the circuit and a hardware primer ahead of the reviewer's guide, the docsite pages that
the prerequisite block names, and the fixture-only rule for rendering captures. **A rendering
before/after also carries a third frame from the format's native tool** (kicad-cli, xschem, Lepton,
gerbv) wherever one exists, because that frame is what finds the gaps nobody was looking for. The
general skeleton
lives in the skill, so do not copy it back into this repo.

**There are four ways to be told a thing worked when it did not. All were hit in one sitting, and
all exit 0.**

- **A stray `agni serve` answers instead of yours.** Two measurements were taken against a server on
  a port that a newer binary had failed to bind, and the "address already in use" line scrolled
  past. `pkill -f "agni serve"` before believing anything served, and re-check the version that the
  answer came from. **That `pkill` also matches the shell running it**, so typed ahead of `make
  testall` on one command line it kills the gate (exit 144, no output). `pgrep -af` first, and
  remember :8080 may be held by something that is not agni at all, such as a code-server, which is
  not yours to kill.
- **`go test` prints FAIL for a BUILD failure too.** A red-check greping for `^ok` read a package
  that would not compile as a pass, so a test that never ran looked like a test that could not fail.
  Confirm a red-check names the assertion it failed on. A red-check that reverts a DECLARATION
  rather than the behaviour fails the same way. Say a new test refers to a source file, and the
  red-check reverts that file. The test is then undefined, so it compiles to nothing and proves
  nothing. Disable the behaviour, keep the symbol.
- **An unquoted heredoc runs the backticks in your markdown.** `gh pr edit` then returns 0 having
  posted a body with a block silently triplicated and a flag eaten out of the prose. Quote the
  delimiter, and READ A PR BODY BACK after editing it.
- **`catalog-docs-check` reads `git status`, not a diff against a temp copy.** Regenerating is not
  enough, because the generated file has to be COMMITTED or the gate is still red on a tree that is
  correct. The same is true of `tutorial-runs`.

Five more that cost a session time:

- **A new test file can replace an existing one of the same name.** `service/boardtier_test.go` was
  written over in agni 987 and the gate was still green, because the test that the file held was
  simply gone. `git status` showing `M` for a file you meant to create is the sign; check the name
  first.

- **Undo a red-check by restoring a copy of the file, never with `git checkout <file>`.** The checkout
  also throws away the uncommitted change under test, and the next run passes over nothing. Copy the
  file aside before breaking it, and restore it in a command of its own. Chained after the test with
  `&&`, the restore is skipped when the broken file fails to build, and the next backup copies the
  broken file. Removing a call that is a package's last use of an import breaks the build the same
  way, so disable it in place (`&& false`) rather than deleting it.
- **`go test -cpuprofile` leaves `<pkg>.test`, a 30 MB binary, in the working directory**, untracked and
  not ignored. Delete it before staging, and never `git add -A`.
- **A baseline run of `main`'s web code relinks `node_modules`.** Stashing a branch's `web/` and
  lockfile to time `main` on the same clock works, but its `make ui` runs `pnpm install` against
  `main`'s lockfile, so after `git stash pop` the old package versions are linked and `tsc` reports
  exports missing. Run `pnpm install` at the root before reading anything else.
- **Time two tools on the same clock.** Say you start a timer inside Python, after the interpreter
  and the client library load. That time is not comparable with another tool's launch-to-exit time.
  Compare launch to exit on an idle machine (check `/proc/loadavg`; a shared box running other
  sessions' fuzz tests made every number meaningless), or say which clock a number is.

## Architectural constraints

`CONSTRAINTS.md` holds the enforceable rules (C1 to C35). Read it before proposing changes, and
**push back when a request would violate one**. Quote the constraint by name, explain the conflict,
and ask whether to proceed and whether the constraint should change. A constraint exists so that the
rule survives everyone forgetting why it exists. Push back on architectural smell even without a
constraint, and if the direction was wrong, suggest capturing it as one.

**A new rule owes a TEST, never a command typed into the document.** Twenty-one are enforced by the
gate and thirteen are review questions that say so (`CONSTRAINTS.md`'s header keeps the count). What
a test reads decides which of the four homes it goes in. A test of the package graph or the module
goes in the root `deps_test.go`. One package's own rule goes beside that package
(`service/transport_guard_test.go`, `core/facts`). A sweep over source goes in
`internal/constraints`. A parity test of what two surfaces print for one answer goes beside the
surface (C35 in `cmd/agni/tabulate_test.go`). The September 2026 audit is the reason, and `build/the-gate.md` has the full
account, with two shapes worth copying. A graph or single-writer check needs a POSITIVE CONTROL, so
that a pattern matching nothing fails rather than reading as clean. And an invariant narrower than
any sweep (C24's "never COMPARED") becomes a RATCHET with an allowlist rather than being weakened.

## What does not belong in this repo

This repo is public. A file committed here is world-readable from the moment it is pushed, and
`_hidden/` does not change that. Deleting it later does not remove it from history.

Never commit:

- **Customer or proprietary design data.** No real schematics or board files, no net names, no
  reference designators off a private design, no part numbers off a private BOM, no title blocks.
  This includes screenshots and PR images, so before/after captures come from the synthetic fixtures
  rather than a real board.
- **Market, competitor, strategy, or opportunity analysis.** This repo is the engine, not the
  business. The same holds for issues, missions, PR bodies and the wiki, so a mission states the
  outcome a person gets and never names a product that it is measured against.
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
the docsite, commit messages, PR bodies, and code comments. The gate REQUIRES one colon label, a
reader package's `// Fidelity: ...` line. `TestC6EveryReaderDeclaresFidelity` greps for the literal,
so a comment pass that folds it into a sentence turns the gate red.

**Prove a comment-only change by comparing code with the comments stripped, not by reading the
diff.** A sweep over hundreds of files hides one edited code line easily. For Go, compare the
`go/scanner` token streams of each file and `git show <base>:<file>` with comments skipped; for
TypeScript, print both through the compiler API's printer with `removeComments`. Give the check a
positive control (a deliberate `var _ = 0`), since one that only adds whitespace changes no token
and proves nothing. PR #754 verified every changed Go and TypeScript file this way.
