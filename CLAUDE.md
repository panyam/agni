# Agni: engine repo orientation

Agni is an EDA tooling engine: a Go engine, a protobuf IR, format readers, diff/checks, a geometry
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
| The checks contract (CLI/service seam) | `architecture/checks-contract.md` |
| Config: what a run is checked against, and where it comes from | `architecture/projects-and-designs.md` |
| Semantic diff | `architecture/semantic-diff.md` |
| The datasheet param/doc/derive layer | `architecture/datasheet-layer.md` |
| Extending the engine from outside | `build/overlay.md` |
| xschem / Lepton native tools | `build/native-verification.md` |
| Running the gate, and how it reads green when it is not | `build/the-gate.md` |
| Measuring something, or trusting a green test | `build/evidence.md` |
| Learning the domain from a software background | `reference/analogy.md`, `reference/edif-primer.md` |
| Why a rule exists at all, as engineering rather than as code | `learn/` (twelve chapters, EE1-EE7) |

`guide/` is the user-facing manual (getting-started, concepts, checks-and-reports,
comparing-revisions, querying, naming-conventions, interface-profiles, datasheets, cli-reference).
`tutorials/` walks one board from first read to a CI gate. `reference/` also holds the GENERATED
rule and relation catalogs.

`learn/` is the DOMAIN course, and it is the axis the other sections do not cover: twelve chapters
teaching what a hardware engineer knows, each ending in the rules that encode it. `tutorials/` teaches
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
rendered figure's geometry are in `docsite/README.md`.

`site/` is stale build output, not a source tree. Some older notes reference a retired `docs/NN-*.md`
mkdocs tree that was folded into `docsite/content/` with audience-first names.

## Package layout

Engine analysis under **`core/`** (`core/check`, `core/review`, `core/render`, `core/report`, `core/diff`,
`core/query`, `core/model`). Format readers under **`readers/`** (`readers/edif`, `readers/kicad`,
`readers/ipc2581`, `readers/xschem`, `readers/geda`, plus `readers/formats`, the registry/Loader).
The shipped rule catalog, fact relations, profiles, and intent under **`stdlib/`**
(`stdlib/rules/builtin/rule_*.go`, `stdlib/rules/datalog`, `stdlib/rules/intent`,
`stdlib/relations`, `stdlib/profiles`). The datasheet stack under **`datasheet/`** (`param`, `doc`,
`derive`). Plus `cmd/agni/`, `internal/`, `intake/`, `census/`, `protos/` + `gen/`, `docsite/`,
`web/`, `hack/`, `tools/`.

Notes written before this layout landed name the old directories. **Grep the SYMBOL or filename,
not the directory.**

`intake/` deserves a callout: it produces a sanitized design summary, and its confidentiality
guarantee is **structural**. The `Skeleton` type has no field that can hold a net name or a
connection, so an intake summary *cannot* express the confidential parts of a design. That turns a
policy ("do not paste a net name") into a property of the type ("there is no field for one"). Keep
it that way. Adding a free-text field to `Skeleton` would quietly dissolve the guarantee.

## Build, test, and the CLI

- `make build` / `make test` / `make agni` / `make install`.
- **Nothing here takes a private path, and a private workspace must never have to reimplement a
  target.** Two mechanisms carry someone's own designs in. Tier-1 config (mounts, symbol paths,
  native tools) belongs in an `agni.yaml`, which the CLI finds by walking up from the working
  directory and then in `~/.config/agni/`, so `agni check mount://corpus/...` works from anywhere
  with no flags: see `cmd/agni/envconfig.go` and the tier boundary it guards. Everything else is a
  variable on a target: `EXTRA_MOUNTS` and `OVERLAY_FLAGS` on `serve`, `DESIGNS` and `OVERLAY_DIR`
  on `dockserve`, `NATIVE_DOCKER_MOUNTS` on `natup`, `DATASHEET_DIR` on the datasheet targets. When
  a workflow only exists as a wrapper in someone's local Makefile, that is a missing target here.
- **A flag wins outright over `agni.yaml` rather than merging**, so a Makefile default that passes
  `--mount` shuts the file out. `make serve MOUNTS=` is how you hand the mount table back to it.
- **A project DISCOVERS its analysis tiers, so a flag naming one is redundant and dropping the flag
  does not turn it off.** `internal/projects/descriptor.go` defaults `conventions.yaml`, `profiles`,
  `params` and `review.yaml`, and `FSStore` composes each one it finds. Two consequences that have
  each cost a bug. Before adding a tier flag to a command, check whether the name is already
  defaulted: `--profile-path` naming the project's own directory double-loaded every profile rule
  (issue 450) and `--params` naming its own is merely redundant. And to reach a tier's "off" state
  you must MOVE THE DIRECTORY ASIDE, which is why rungs 4, 5 and 6 open with `mv <tier> <tier>-off`;
  rung 6 shipped a before/after whose two captures were byte-identical because both ran with the
  corpus in place.
- **Precedence between a project tier and its flag is `Overlay.SpecsOr`: the project wins.** That is
  the opposite of the mount rule above, deliberately, because a project owns its parameters the way
  it owns its profiles. A command that reads a tier from its flag alone is the bug shape: `intake`
  did, so inside a project its datasheet-gap section was absent rather than empty (issue 474).
  `readDesignWithConfig` returns the overlay the read already composed, which is where a non-service
  command should get a tier rather than resolving the project a second time.
- `make natrender FILE=... OUT=...` and `make natopen FILE=...` drive the native tools over the
  `natup` container. Both take paths INSIDE it, so they must fall under a `NATIVE_DOCKER_MOUNTS` dir.
- `make setup` builds the docling venv the datasheet tooling runs in, then `make pdf2doc`,
  `make pdf2doc-all`, and `make datasheets-status` work over `DATASHEET_DIR`. The venv is found by
  lookup (repo-local `.venv`, then the parent directory's), so worktrees sharing a root share one
  env instead of each carrying gigabytes of torch.
- `make -C docsite preview PAGE=learn/03-why-every-chip-needs-capacitors` folds one built page into a
  self-contained HTML file, for reviewing a branch before it merges. Use it rather than
  `make -C docsite gh-pages`, which is DEAD: Pages serves the `docs.yml` workflow artifact
  (`build_type: workflow`), so force-pushing that branch changes nothing.
- `make -C docsite figures` re-renders the schematics `learn/` embeds. Outside the gate, because a
  render depends on the engine build and nothing checks its output for staleness (agni issue 453).
  `make tutorial-runs` is no longer in that company: `tutorial-runs-check` regenerates every capture
  and fails on any difference, and it is in `testall`. A capture's stamp hashes the spec and the
  fixture but NOT the engine, so regenerating is the only way to see engine drift.
- CLI: `agni stats|check|diff|render|query|review|serve|open <file>`. `open` serves ONE design and
  prints its URL, minting the mount itself; `serve` takes `--mount` per folder and `--web-dir`. The reader is chosen by extension
  (case-insensitively), with `.xml`/`.sch` sniffed by root/header. `--symbol-path <dir>` resolves
  external symbol files and searches each dir's SUBTREE, so a dir can be a library root.
- **Two HTML reports, one stylesheet, different axes.** `check --format html` is the verdict report,
  rule-major, and implies `--verdicts`. `review --format html` is the checklist, question-major, in
  the manifest's order with every finding per item. Both take `--url-base` and share
  `core/report/style.css`. **A link is only emitted for a mount you DECLARED**, and `--url-base` then
  asks that server's `ListMounts` whether it serves that name from the same root; a withheld link
  always prints its reason. `agni open <design>` prints a matching `check --mount … --url-base …`
  line, and because one process mints the mount and serves it the two cannot disagree. **A link names
  the design's declared ENTRY whatever you pointed the command at, and carries the revision it was
  read at**, which the viewer checks before it draws. Semantics and the two ways the halves used to
  disagree are in `guide/checks-and-reports.md`.
- Toolchain: Go 1.26.4 and `buf` 1.61. **Both protoc plugins are pinned as `tool` directives in
  `go.mod` and invoked via `go tool`**, so their versions are data rather than something to match by
  hand. Only `buf` itself has to be on your PATH.
- **A command that reads a design goes through `readDesign` (or a service), never a bare
  `newLoader().ReadDesign`.** That function is where a design's PROJECT config enters the read for the
  six commands no service mediates (stats, diff, emit, render, intake, profilediag), and net roles are
  resolved once at ingestion — so a read that skips it silently uses the built-in naming vocabulary and
  none of the project's declared symbol libraries. All six bypassed it until agni issue 228, which is
  why it is one function rather than six.
- **After ANY proto change run BOTH `make proto` (Go) AND `make proto-web` (TS).** `make proto-check`
  fails the gate on either half being stale.
- **When you build a feature, ship an example** (CONSTRAINTS C10; how-to in `examples/CONVENTIONS.md`,
  and `examples/tutorial-project/README.md` for the fixture the docsite tutorial runs on).

**`make browser-test` is a separate suite and is NOT in the gate.** It drives a real Chromium against
a real server for the handful of assertions that need layout, because jsdom has none. Read
`docsite/content/build/the-gate.md` for what belongs in it, and `build/evidence.md` for the two ways
a layout assertion passes while proving nothing.

**`make testall` is the full gate, and CI runs exactly it.** Read
`docsite/content/build/the-gate.md` before trusting a run: it has three traps that make a red gate
read green (a pipe swallowing the exit code, a commit-first ordering rule, and a per-clone
`pnpm install`), plus what a run leaves behind and the generated-code rules.

**Before believing a measurement or a green test, read `docsite/content/build/evidence.md`.** A
negative result needs a positive control, a positive rate needs a precision check, and every new test
needs a red-check. Most of the expensive mistakes here have been correct-looking results nobody could
have falsified.

## Wiring, per subsystem

Each of these has a fixed edit-list where missing one edit is silent, and a test that catches it. The
note strip is the one exception, and it is listed so the gap is visible rather than discovered.

| Adding | Edits | Read | Enforced by |
|---|---|---|---|
| A docsite page | 4 (5 for a new section) | `docsite/README.md` | `docsite/nav_test.go` |
| A `learn/` chapter | 4, plus the level-index entries | `docsite/README.md` | `docsite/learn_levels_test.go` |
| A web viewer panel | 4 | `docsite/content/architecture/web-client.md` | `web/src/composition.test.ts` |
| A canvas note strip (undrawn, stale-link) | 5 | `web/src/undrawn.ts` and `web/src/stalelink.ts` as the two worked examples | the compiler, for the `ViewSink` channel; NOTHING for the template hole |
| A web page | 6 | `docsite/content/architecture/web-app.md` | its own boot test (one per page) |
| A format reader | — | `docsite/content/build/format-reader.md` | — |
| A check rule | — | `docsite/content/build/check-rule.md` | — |
| A query relation | 5, plus `make catalog-docs` | `stdlib/relations/facts/docs/_TEMPLATE.md` | `facts_docs_test.go`, `TestCatalogMatchesSchema`, `catalog-docs-check` |
| A glossary term | 2 (the term page, one index line) | `docsite/README.md` | `docsite/terms_test.go` |
| A hand-authored diagram | 2 (the file in `docsite/figures/`, one `{{ includeFile }}` in the page) | `docsite/README.md` | `docsite/includefile_test.go` |

## Working in this repo

`CONTRIBUTING.md` holds the workflow rules: running several checkouts in parallel (use
`git -C <abs-path>`, never `git add -A`), the PR workflow (verify a push by its exit code, verify
`merged: true` via the API, never `gofmt -w` a directory), the three shell traps that have burned
real work, and what agni ADDS to the PR body shape defined by the `start_pr` skill (the circuit and
a hardware primer ahead of the reviewer's guide, which docsite pages the prerequisite block names,
and the fixture-only rule for rendering captures). The general skeleton lives in the skill, so do
not copy it back into this repo.

## Architectural constraints

`CONSTRAINTS.md` holds the enforceable rules (C1–C27). Read it before proposing changes, and **push
back when a request would violate one**: quote the constraint by name, explain the conflict, and ask
whether to proceed and whether the constraint should change. The point of constraints is that they
survive everyone forgetting why the rule exists. Push back on architectural smell even without a
constraint, and if the direction was wrong, suggest capturing it as one.

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

**Sanitize at the point of writing rather than cleaning up later.** The engineering content nearly
always survives sanitizing and only the provenance goes. "Customer item 112 mock-failed against a
fake RSTRAP threshold" becomes "a strap resistor's value is a design choice, not a datasheet
parameter", and the second version is the better issue anyway because it states the general rule.

**If a learning cannot be stated without the private context, it does not belong here.** Write it in
the private workspace instead. When a rule's motivation is general EE practice, it belongs here; when
sanitizing would gut it, it does not.

## Writing style for docs and commits

Plain declarative prose. No em-dashes, no marketing cadence, no hype adjectives, and no rhetorical
"The result: X" constructions. Write separate sentences instead. This applies to the docsite, commit
messages, and PR bodies.
