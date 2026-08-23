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
- `make stats` / `make check` run the CLI against a committed fixture (`EDN=...` to override).
- `make -C docsite preview PAGE=learn/03-why-every-chip-needs-capacitors` folds one built page into a
  self-contained HTML file, for reviewing a branch before it merges. Use it rather than
  `make -C docsite gh-pages`, which is DEAD: Pages serves the `docs.yml` workflow artifact
  (`build_type: workflow`), so force-pushing that branch changes nothing.
- `make -C docsite figures` re-renders the schematics `learn/` embeds. Outside the gate, like
  `make tutorial-runs`, because a render depends on the engine build.
- CLI: `agni stats|check|diff|render|query|review|serve <file>`. The reader is chosen by extension
  (case-insensitively), with `.xml`/`.sch` sniffed by root/header. `--symbol-path <dir>` resolves
  external symbol files and searches each dir's SUBTREE, so a dir can be a library root.
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

Each of these has a fixed edit-list where missing one edit is silent, and a test that catches it.

| Adding | Edits | Read | Enforced by |
|---|---|---|---|
| A docsite page | 4 (5 for a new section) | `docsite/README.md` | `docsite/nav_test.go` |
| A `learn/` chapter | 4, plus the level-index entries | `docsite/README.md` | `docsite/learn_levels_test.go` |
| A web viewer panel | 4 | `docsite/content/architecture/web-app.md` | `web/src/composition.test.ts` |
| A web page | 6 | `docsite/content/architecture/web-app.md` | its own boot test (one per page) |
| A format reader | — | `docsite/content/build/format-reader.md` | — |
| A check rule | — | `docsite/content/build/check-rule.md` | — |
| A query relation | 5, plus `make catalog-docs` | `stdlib/relations/facts/docs/_TEMPLATE.md` | `facts_docs_test.go`, `TestCatalogMatchesSchema`, `catalog-docs-check` |

## Working in this repo

`CONTRIBUTING.md` holds the workflow rules: running several checkouts in parallel (use
`git -C <abs-path>`, never `git add -A`), the PR workflow (verify a push by its exit code, verify
`merged: true` via the API, never `gofmt -w` a directory), the three shell traps that have burned
real work, and the PR prose conventions (ELI12 on every PR, a hardware primer, before/after images
for anything visual).

## Architectural constraints

`CONSTRAINTS.md` holds the enforceable rules (C1–C26). Read it before proposing changes, and **push
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
