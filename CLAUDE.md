# Agni engine repo orientation

Agni is EDA tooling built from a Go engine, a protobuf IR, format readers, diff and checks, a
geometry sidecar, a web viewer and a CLI. It reads electronic design files in several formats into
one intermediate representation, then runs checks, diffs, queries and renders over that IR.

**This repo is public, under Apache-2.0.** Anything committed here is world-readable the moment it
is pushed. Read "What does not belong in this repo" below before writing docs, tests or fixtures.

This file is a router, capped at 10,000 characters by C36. A fact that matters in one directory
goes in that directory's `CLAUDE.md`, and Claude Code loads it when a session works there.
Workflow rules go in `CONTRIBUTING.md`, design goes in `docsite/content/`, and status goes in
issues or `NEXTSTEPS.md`.

## Where to read

`docsite/content/` is the engineering source of truth. Read the page before working in its area.

| Working on | Read first |
|---|---|
| Ingestion, the IR, a format reader | `architecture/ingestion-and-ir.md`, `build/format-reader.md` |
| Geometry, rendering, the viewer | `architecture/geometry-and-rendering.md`, `architecture/web-app.md`, `web-client.md`, `web-picking.md` |
| The wire contract, services | `architecture/web-services.md` |
| Driving a viewer from outside | `architecture/viewer-control.md`, `build/embedding-the-viewer.md` |
| Net solving, hierarchy | `architecture/net-solving.md` |
| Rules, datalog, profiles | `architecture/rules-and-checks.md`, `build/check-rule.md`, `architecture/checks-contract.md` |
| Pin maps, comparing identifiers | `guide/design-intent.md`, `core/ident`'s package comment |
| Project and design config | `architecture/projects-and-designs.md` |
| Semantic diff | `architecture/semantic-diff.md` |
| Datasheets | `architecture/datasheet-layer.md` |
| Extending, other languages | `build/extending.md`, `build/other-languages.md` |
| The gate, evidence | `build/the-gate.md`, `build/evidence.md` |
| Queries past one clause | `guide/querying.md` |
| xschem and Lepton native tools | `build/native-verification.md` |
| The domain | `learn/` (EE1-EE7, `learn/levels.md`), `reference/analogy.md`, `reference/edif-primer.md` |

`guide/` is the user manual, `tutorials/` walks one board to a CI gate, and `reference/` holds the
generated rule and relation catalogs. When a change touches a rule that `learn/` teaches, check the
chapter and cite it in the PR. `site/` is stale build output.

## Directory notes

These hold the detail. Each loads when a session reads files in its directory.

- `cmd/agni`: CLI flags, `agni.yaml`, output formats, `--server`, `--remote`, `-o`.
- `service`, `internal/projects`: snapshots, tier resolution, caches, descriptors.
- `core/check`, `core/classify`, `core/ident`, `core/query`, `core/facts`, `core/report`,
  `core/timing`: engine rules, classes, identifiers, datalog, reports, measuring.
- `readers` and each reader under it, `internal/sexpr`: per-format traps and oracles.
- `web`, `web/browser`, `internal/wasmengine`, `internal/webbundle`: viewer, walks, browser engine.
- `internal/mcpserver`, `intake`, `datasheet`, `protos`, `clients/python`, `examples`, `docsite`,
  `hack`: MCP, sanitized summaries, datasheets, proto generation, the Python client, examples,
  docs, samples and seeds.
- `CONTRIBUTING.md`: the gate, the wiring edit-lists per subsystem, shell and test traps.

## Package layout

Engine analysis lives under `core/`, format readers under `readers/` (`readers/formats` is the
registry and Loader), and the shipped rules, relations, profiles and intent under `stdlib/`. The
embedding surface is `service/`, `artifact/` and `mounts/`. `datasheet/` is its own Go module and
depends on the root, never the reverse (C34, C18). The Datalog engine is the separate module
`github.com/panyam/jaala`. A language change is a jaala PR and tag first. Notes older than this
layout name old directories, so grep the symbol, not the directory.

The root `agni` package is the composition facade. `agni.New` returns one composed catalog and
fact registry, so compose through it rather than calling `check.DefaultCatalog` at a call site.
The four global registration points are `check.RegisterBuiltins`, `facts.RegisterRelation`,
`check.RegisterSource` and `review.RegisterQueryCompiler`. Three fail silently when a binary
misses one, and `New` refuses that case.

`intake/` keeps its confidentiality guarantee in the type. `Skeleton` has no field for a net name
or a connection. Never add a free-text field to it.

## Commands and traps for every session

- `make build`, `make test`, `make agni`, `make install`. `make agni` writes `bin/agni` and does NOT
  update the `agni` on PATH, which `make install` writes. Verify a CLI fix with the right one.
- `make testall` is the full gate and CI runs exactly it. From a worktree run `make -k testall`, and
  build with the default Go, never `GOTOOLCHAIN=go1.26.9` (CONTRIBUTING.md has why).
- After any proto change run `make proto`, `make proto-web` and `make proto-py` (`protos/CLAUDE.md`).
- A stray `agni serve` can answer instead of yours. `pgrep -af "agni serve"` before trusting a
  served answer, and never `pkill` from the shell that runs the gate.
- `go test` prints FAIL for a build failure too. A red-check names the assertion it failed on.
- Restore a red-checked file from a copy, never `git checkout <file>`, in a command of its own.
- Never `git add -A`, and never `gofmt -w` a directory. Use `git -C <abs-path>` across checkouts.
- Before believing a measurement or a green test, read `build/evidence.md`. A negative result needs
  a positive control, and a test that loops over rows needs a count guard.
- When you build a feature, ship an example (C10, `examples/CONVENTIONS.md`).
- agni's agent skills live in `plugins/agni/skills/<name>/SKILL.md` and nowhere else (agni 1292).
  `plugins/README.md` has the rules.
- Work is aimed at missions, issues labelled `mission`. NEXTSTEPS.md names each one's next ticket.
  A new issue gets `P0`-`P3`, or `waiting` with its trigger, when it is filed.

## Architectural constraints

`CONSTRAINTS.md` holds C1 to C36. Read it before proposing a change, and push back when a request
would break one. Quote it by name, explain the conflict, and ask whether to proceed and whether the
rule should change. A new rule owes a test, never a command typed into the document.
`build/the-gate.md` says which of four homes a test goes in.

## What does not belong in this repo

A committed file is world-readable at once, `_hidden/` included, and deleting it later leaves it in
history. Never commit these.

- Customer or proprietary design data. That means no real schematics or boards, net names,
  designators, part numbers or title blocks off a private design, in screenshots and PR images too.
- Market, competitor or strategy analysis, in code, issues, missions, PR bodies or the wiki.
- Paths into private folders, private corpus locations or customer names, anywhere.
- Vendor-licensed material such as datasheet PDFs. Facts transcribed into a fixture are fine with
  the document revision and page (`architecture/datasheet-layer.md`).

A generated report is customer data. `check --format html` on a real board carries every net name.
Write reports to `/tmp`, and never `git add` a file you did not author. Sanitize as you write, and
if a learning cannot be stated without the private context, keep it in the private workspace.

## Writing style for docs and commits

Plain declarative prose. No em-dashes, no marketing cadence, no hype adjectives, and no "The
result: X" constructions. No colon-definitions in prose, nav lists, headings or bold labels. Never
use honest, load-bearing or seam. The gate requires one colon label, a reader package's
`// Fidelity: ...` line, so leave it alone. CONTRIBUTING.md says how to prove a comment-only change.
