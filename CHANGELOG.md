# Changelog

All notable changes to this project are recorded here.  The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project follows
[semantic versioning](https://semver.org).  Pre-1.0, breaking changes may ride a minor release; the
process and the versioning rules are in `RELEASING.md`.

Each entry summarizes.  The full per-release write-up lives in `RELEASES/<tag>.md` and is what the
GitHub Release body carries.

## [Unreleased]

## [0.3.12] - 2026-10-27

A pin-mux table kept as its own file merges into the part's published spec with `agnids merge-mux`,
so the pin map's function checks can read it (PR 1201). The Python client is published to PyPI as
`agni-eda` with each release (PR 1204).  The full write-up is
[RELEASES/v0.3.12.md](RELEASES/v0.3.12.md).

## [0.3.11] - 2026-10-26

A design intent's IO map is laid out one row per declared pin, beside the netlist and the functions
the part's datasheet lists for it (PRs 1192, 1195). The check report can carry the project's query
sets (PR 1199), and `agni trial` counts the pin map for a clean-room run (PR 1198). `trace` takes a
pin's functional name, and EDIF ports numbered only on their cell now connect (PR 1197). A
TypeScript or Python client imports `OverlayConfig` from `config` rather than `checks`.  The full
write-up is [RELEASES/v0.3.11.md](RELEASES/v0.3.11.md).

## [0.3.10] - 2026-10-26

A reader's question from a viewer tab reaches a Claude Code session as a turn of its own, through
SEP-3415 room events and mcpkit's `rooms-channel` (PR 1187). A Review finding opens onto the facts
that make it hold, and the MCP `query` text carries them (PR 1185). The stateless MCP wire now
behaves as the session wire does (PRs 1181, 1187).  The full write-up is
[RELEASES/v0.3.10.md](RELEASES/v0.3.10.md).

## [0.3.9] - 2026-10-25

An agent drives the viewer tabs a reader has open, and a reader asks it questions from the tab, under
`agni serve --mcp --viewer-rooms` (PRs 1174, 1175). `?tour=review` starts a guided review tour of the
viewer that the reader takes themselves, and `make exercise-browser-review` runs as that tour (PRs
1177, 1179).  The full write-up is [RELEASES/v0.3.9.md](RELEASES/v0.3.9.md).

## [0.3.8] - 2026-10-22

Shift-drag selects an area of a sheet and asks about everything in it (PR 1171). Another page, a
saved report or an MCP answer moves an open viewer without reloading it (PRs 1158, 1168). Query rows
show how they were derived (PR 1164), and a design declares a pull-up reach per net (PR 1163).  The
full write-up is [RELEASES/v0.3.8.md](RELEASES/v0.3.8.md).

## [0.3.7] - 2026-10-22

Every page of a hierarchical EDIF export gets its own sheet id and name, and the viewer's Sheets dock
nests sheets under their parents, names a shared sheet name by its path and explains its count pills
(PR 1145).  The full write-up is [RELEASES/v0.3.7.md](RELEASES/v0.3.7.md).

## [0.3.6] - 2026-10-22

`project.yaml` and `design.yaml` share one schema in which every tier a run reads is named, and a
design names its project with `extends`, so a board can carry its own convention or checklist and a
moved tier folder fails the load rather than vanishing (PR 1139).  The relation picker drills module
by module (PR 1131), trace endpoints are picked from the design and each rule links to its page
(PR 1135), and the analysis panels say what they mean (PR 1134).  Every existing descriptor must be
migrated, listed under Breaking with the steps.  The full write-up is
[RELEASES/v0.3.6.md](RELEASES/v0.3.6.md).

## [0.3.5] - 2026-10-22

A value a library writes as its part name (`R_10k_0402`) is read, so every passive on the Jetson
baseboard has one and a strap band that passed unchecked is judged (PR 1128).  A failed check run or
rule catalog says so in the viewer (PR 1129), and a diff has a viewer link, `?against=`, which
`agni diff --server` prints and the MCP `diff` tool returns (PR 1130).  `report.DesignURL` is removed
and the MCP `diff` answer's `links` becomes `link`, both listed under Breaking with migrations.  The
full write-up is [RELEASES/v0.3.5.md](RELEASES/v0.3.5.md).

## [0.3.4] - 2026-10-22

agni serves its analysis over MCP, from `agni serve --mcp` and `agni mcp` (PR 1122), and
`make exercise-mcp-ask` checks five review questions through an MCP client against the CLI's answers
(PR 1127).  A query variable can be bound to a list, and a rule name the catalog lacks is refused
rather than run as nothing, both listed under Breaking with migrations.  The full write-up is
[RELEASES/v0.3.4.md](RELEASES/v0.3.4.md).

## [0.3.3] - 2026-10-20

agni moves to jaala v0.1.85, where a comparison against a constant binds its variable and a goal
column can be a constant, with answers unchanged, and the README names what has shipped since it was
written (PR 1111).  The full write-up is [RELEASES/v0.3.3.md](RELEASES/v0.3.3.md).

## [0.3.2] - 2026-10-20

A hierarchical EDIF netlist can be read whole, from `--flatten-hierarchy`, `Snapshot.flatten_hierarchy`,
a design's `design.yaml` or the viewer's Read every block (PRs 1101, 1106, 1109).  KiCad pin types
tri-state, open-collector and open-emitter are kept, a power net's supply pins need a power source, no
rule judges an unfitted part (PR 1108), a KiCad board read skips undrawn footprint graphics while
parsing (PR 1110), and the security scanners join the gate (PR 1100).  The full write-up is
[RELEASES/v0.3.2.md](RELEASES/v0.3.2.md).

## [0.3.1] - 2026-10-20

v0.3.0 with its release check fixed: the compose smoke asks about a snapshot of the design, and a web
API proto change now runs the images workflow (PR 1084).  It is the release to use.  The full
write-up, v0.3.0's included, is [RELEASES/v0.3.1.md](RELEASES/v0.3.1.md).

## [0.3.0] - 2026-10-20

Every analysis rpc names a snapshot of the design, ingested once, which is why this is a minor
release.  The browser demo reads a design on a Load page first, checks a selected net on its own and
can cancel a run.  No rule credits a do-not-populate part, the seed triage is checked against KiCad's
own ERC and DRC, and agni moves to jaala v0.1.84, whose `list` answers JSON.  The full write-up is
[RELEASES/v0.3.0.md](RELEASES/v0.3.0.md).

## [0.2.5] - 2026-10-18

The browser engine collects between requests rather than beside them, so Jetson's first Run checks
on the deployed demo went from 11.5 s to 4.6 s (agni issue 1010).  The public-demo exercise reports a
design's load apart from its questions (agni issue 1060), and agni moves to jaala v0.1.69.  The full
write-up is [RELEASES/v0.2.5.md](RELEASES/v0.2.5.md).

## [0.2.4] - 2026-10-16

A compose bundle and `agni trial` for trying a release on your own machine and reporting back
without the design, a `looks wrong?` link per finding, parse-error positions, query links, verdict ids
on findings, checklists scored with no review store, and the hierarchy note on every page.  A failed
request under `--format json` now writes the Connect error.  The full write-up is
[RELEASES/v0.2.4.md](RELEASES/v0.2.4.md).

## [0.2.3] - 2026-10-16

A server keeps each rule's check results with the design's cached model, so a rule already run on
an unchanged design answers without evaluating (agni issue 390).  Findings of one rule that tie on
subject now keep the rule's order.  The full write-up is [RELEASES/v0.2.3.md](RELEASES/v0.2.3.md).

## [0.2.2] - 2026-10-16

The engine in the browser and a static public demo, the datasheet tooling as its own module and
image (`agnids`), project query sets and derived relations, query sets, tables and `--remote`, and a
viewer with a Properties panel, query tabs and saved reports.  Tagged a patch with output changes a
gate may read, as v0.2.1 was.  The full write-up is [RELEASES/v0.2.2.md](RELEASES/v0.2.2.md).

### Changed

- A length is published in metres and shown in millimetres, so a threshold over one writes its unit
  (`?w < 0.2mm`); a bare number is metres (agni issues 1035, 1037).
- Conventions and checklists are sections of `project.yaml`, and intent a section of `design.yaml`;
  the old files are load errors naming where their content goes.
- The datasheet workbench is `agnids`, its own binary, module and image.
- `agni query --set` takes a project set's name as well as a file (agni issue 745).
- In the viewer, a click selects into Properties, and Run checks lights nothing until a finding is
  clicked (agni issues 902, 356).
- **Breaking.** `review --format json` emits protojson of the `Review` the `CreateReview` rpc returns,
  and several designs emit a `ListReviewsResponse`, where both used to be hand-rolled shapes (agni issue
  734).  Items keep `id`, `outcome` and `findings`, spelled as protojson spells them, and gain the whole
  results document.  The counts a gate reads are `summary` on the `Review`, which every review rpc now
  carries.  The multi-design rollup's `total`/`automated`/`per_design` and outcome matrix are gone,
  since each design's `Review` and summary carry the same facts.  `results --format json` on a review
  document emits the same `Review`.

### Added

- `review --checklist -` reads the manifest from stdin, and the Python client's `create_review` works
  over the CLI transport.

## [0.2.1] - 2026-09-08

Full notes: [`RELEASES/v0.2.1.md`](RELEASES/v0.2.1.md).

Tagged as a patch although two new rules change what `check` reports on unchanged input, which would
ordinarily be a minor.  Deliberate, since there are no external consumers yet.

### Added

- `i2c-redundant-pull-up`: an I2C net reaching one rail through more than one resistor.  Parallel
  pull-ups are one smaller resistor, so the bus sinks more current than it was sized for.
- `i2c-pull-up-split-rail`: an I2C net reaching two different rails through its pull-ups, which
  back-feeds whichever supply is down through the bus.
- `examples/dft-coverage`.

### Fixed

- The viewer's default layout opened on the Trace tab, hiding the query surface behind it.  Every
  other dock stack set its opening tab; the query stack did not, and it became a stack in v0.2.0.

## [0.2.0] - 2026-09-07

Full notes: [`RELEASES/v0.2.0.md`](RELEASES/v0.2.0.md).

The first release with notes, covering roughly 300 pull requests since `v0.1.1`.  The engine got much
better at saying what it did: a rule states the set it considered, a passing verdict carries the value
that decided it, and a report links back to its proof.

### Breaking

- `check` exits 2 when a gate trips, matching `review`.
- `Eval` returns a verdict for every subject instead of filtering to failures.  Count outcomes, not
  the slice.
- The part number is a typed IR field (`ir.Component.mpn`, `ir.PartType.mpn`); the canonical-attribute
  convention is deleted.
- `Package.pin_count` removed.  `naming.Config` collapsed into a proto.  `Candidate` and `WorkItem`
  are protos.
- The fact registry is a composed value; reads go through a `*facts.Registry` a caller holds.
- `core/review` no longer knows a query language.  Register a `review.QueryCompiler`.
- `net.nominal_voltage` is rails only; a signal net's level has its own relation.
- `examples/overlay` is `examples/extension`, and `build/overlay.md` is `build/extending.md`.

### Added

- Commands: `agni trace`, `agni open`, `agni params`, `agni emit --format edif`, `agni start`.
- Verdicts: rules state their considered set, passes state the deciding value, subjects can be tuples,
  coverage is on by default, and every verdict format links to its proof or prints why it cannot.
- HTML reports on two axes: `check --format html` is rule-major, `review --format html` is
  question-major.
- Query: five output formats, derived relations, `having` and aggregates, `entity(name, kind)`, search
  mode, walk-from-a-row, and declared argument kinds.
- A root `agni` package that composes the engine and fails at startup on a missing registration seam.
- `service/` moved out of `internal/` as the embedding surface, with `artifact` alongside it.
- Projects and designs are modelled, config resolves through `agni.yaml` and `extends`, and naming an
  entry reads its declared companions.
- Datasheet layer: parameters bind to pins, state pin-to-pin constraints, and carry a verification
  keyed to the revision it checked.
- A Telesis reader.
- Web viewer: landing page, default dock layout, canvas picking, a trace panel with deep links.

### Changed

- Provenance is recorded relative to the design's mount, never the host path.
- A tier the project already composes is not a request, and supersession says so.
- `readDesign` resolves the design's project config for the six commands no service mediates.
- A malformed descriptor is fatal for the design it governs.
- Constraints C21, C25, C26, C28 and C29 landed, C15 retired, and eleven `Verify` blocks became tests.

### Fixed

- KiCad: mirrored symbols at 90 and 270 no longer swap pins onto each other's nets; bus vectors and
  group buses cross sheet boundaries; buses at a sheet pin pair off by bit position; a truncated
  s-expression parse is a failure rather than a small board.
- EDIF emit produces a file a reader other than ours can load.
- Schematic text sizing, font, ref-des placement, pin numbers and multi-line anchoring.
- `i2c-pull-up`, `decoupling-present` and `fet-vdss` scope corrections.

### Dependencies

- `pdfjs-dist` 6.1.200 to 6.2.108; `github.com/gorilla/websocket` 1.5.0 to 1.5.3.

## [0.1.1] - 2026-08-10

Tag-only, no release notes.  Added the GHCR publish workflow so a release tag builds and pushes the
container image, with the tag as the single source of the version stamped into the binary.

## [0.1.0] - 2026-08-10

Tag-only, no release notes.  The first tagged version.

[0.2.1]: https://github.com/panyam/agni/releases/tag/v0.2.1
[0.2.0]: https://github.com/panyam/agni/releases/tag/v0.2.0
[0.1.1]: https://github.com/panyam/agni/releases/tag/v0.1.1
[0.1.0]: https://github.com/panyam/agni/releases/tag/v0.1.0
