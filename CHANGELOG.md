# Changelog

All notable changes to this project are recorded here.  The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project follows
[semantic versioning](https://semver.org).  Pre-1.0, breaking changes may ride a minor release; the
process and the versioning rules are in `RELEASING.md`.

Each entry summarizes.  The full per-release write-up lives in `RELEASES/<tag>.md` and is what the
GitHub Release body carries.

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
