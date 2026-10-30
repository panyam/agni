# Agni

[![CI](https://github.com/panyam/agni/actions/workflows/ci.yml/badge.svg)](https://github.com/panyam/agni/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/panyam/agni.svg)](https://pkg.go.dev/github.com/panyam/agni)
[![Go Report Card](https://goreportcard.com/badge/github.com/panyam/agni)](https://goreportcard.com/report/github.com/panyam/agni)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go 1.26](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)

Agni checks schematics and boards for design mistakes, such as an I2C bus with no pull-up, a power
rail with no decoupling, a USB connector with no ESD protection, or a track narrower than the
board's own rules. It reads KiCad, EDIF, IPC-2581, xschem and gEDA designs, and every finding names the net or
part it fired on and locates it on the schematic.

**[Open the demo in your browser](https://panyam.github.io/agni/demo/)** and drop a KiCad or EDIF
design on the page, or open one of the example boards. The engine runs in the page as WebAssembly,
so your design is never uploaded.

![The agni viewer: a schematic renders, structural checks run, and each finding locates on the canvas](docsite/static/images/demo-viewer.gif)

## What does it do

- **Runs ERC/DRC-like structural checks.** They catch missing I2C pull-ups, unprotected
  exposed signals, power rails without decoupling, and boards that fail track-width rules. Findings come out in
  plain language and cite the net or component they fire on.
- **Runs a browser viewer.** `agni open <design>` serves one board and prints its URL; `agni serve` opens a
  tree of them. Either renders the design, runs the checks, and locates each finding on the canvas.
- **Runs in the browser with nothing installed.** The engine also compiles to WebAssembly, so the
  [public demo](https://panyam.github.io/agni/demo/) opens a KiCad or EDIF design you drop on the
  page, checks it, queries it and saves a report, without sending the design anywhere.
- **Walks a review checklist and traces signals.** `agni review` scores a design against a team's
  checklist and saves the page, and `agni trace --from U7.3 --to U12.4` follows a signal through
  the series parts between two pins.
- **Checks against datasheets.** It joins a design against extracted datasheet limits and
  checks, for example, that a rail stays inside a part's recommended operating range.
- **Diffs revisions over the IR.** It compares two revisions structurally (components, nets,
  connectivity), not as a text diff of the source files, so it survives reformatting and
  rename churn.
- **Renders to SVG or a WebGL canvas**, drawing either the faithful schematic and board
  geometry or an auto-laid-out netlist graph.
- **Answers datalog queries.** `agni query` asks arbitrary questions of the design fact base,
  the same fact base the rules are built on. The engine is
  [jaala](https://github.com/panyam/jaala).
- **Reads many formats into one IR.** EDIF netlists and schematics, KiCad schematics and
  boards, IPC-2581, xschem, and gEDA all parse into the same `ir.Design`. Adding a reader is
  one entry in `readers/formats/registry.go`.
- **Serves the same answers to other tools.** The services the CLI calls are Connect rpcs that
  `agni serve` exposes, `--remote` sends `query`, `check`, `trace` and `review` to a running server,
  and [`clients/python`](clients/python) is a typed Python client over the CLI or a server.

## Try it in 60 seconds

With nothing installed, the [public demo](https://panyam.github.io/agni/demo/) checks a seed
board or a file you drop on it, in your browser.

To run the CLI, install it one of three ways:

```
go install github.com/panyam/agni/cmd/agni@latest
```

or download the tarball for your platform from the
[latest release](https://github.com/panyam/agni/releases/latest) and put `agni` on your PATH, or
run the container image with no install at all:

```
docker run --rm -v "$PWD":/work -w /work ghcr.io/panyam/agni check <board>.kicad_sch
```

Then check the demo board, which has deliberate design issues:

```
curl -sLO https://raw.githubusercontent.com/panyam/agni/main/demo/showcase.fires.kicad_pro
curl -sLO https://raw.githubusercontent.com/panyam/agni/main/demo/showcase.fires.kicad_sch
agni check showcase.fires.kicad_pro
```

```
findings by rule:
  bulk-cap               2
  decoupling-present     2
  esd-protection         2
  i2c-pull-up            1
  input-protection       1
  reverse-blocking-absent 1
  test-point-coverage    2

first 11:
  [warning] bulk-cap: +3V3 (power rail has no bulk capacitor)
  ...
  [error] i2c-pull-up: /SCL (I2C net has no pull-up resistor to a rail)
  [warning] input-protection: VBUS (connector feeds a power input with no fuse or TVS in the path)
  ...
```

`agni open showcase.fires.kicad_sch` serves the board in the browser viewer and prints its URL. Press
Run checks and click a finding to locate its net on the schematic. A release binary carries the
viewer, and a `go install` build downloads it from its release the first time it needs it.

From a checkout, `make agni` builds the same binary into `bin/`, and `make demo` opens both demo
boards, the clean one and the one with issues. See [demo/README.md](demo/README.md).

To run a release over your own boards, each
[GitHub release](https://github.com/panyam/agni/releases) attaches a compose bundle pinned to it,
over the `ghcr.io/panyam/agni` image:

```
AGNI_DESIGNS=./designs docker compose up -d     # the viewer at http://localhost:8080
docker compose run --rm trial /designs/<board>  # a trial report that holds nothing from the board
```

## How it works

Agni normalizes design formats the way a compiler normalizes languages into one AST, so every
analysis downstream is written once and works on all of them. The protobuf IR (`protos/`, generated into `gen/`) is the one contract in the middle. Readers
(`readers/edif/`, `readers/kicad/`, `readers/ipc2581/`, and the xschem/gEDA readers) are the only code that knows a
file format; they produce `ir.Design`. Everything else (`check/`, `diff/`, `render/`, the
query engine, the web service) consumes the IR and never looks at a source file.

The same shape repeats at two more contracts: a geometry IR that N producers fill and N
renderers draw, and a parameter IR that N datasheet extractors fill and the checks read.

## Philosophy

- **One neutral IR serves every format.** A schematic is a schematic whether it came from KiCad,
  EDIF, or IPC-2581. Normalize each format once, and write every analysis once against the
  IR. Add a reader and every check, diff, render, and query works on the new format; add an
  analysis and it works on every format.
- **Format-neutrality is enforced in CI.** Analyses read the IR, never source
  files, and the IR carries no field a second format could not populate. Architectural
  constraints checked in CI keep the core from accreting format-specific special cases.
- **Silence is never coverage.** A check that cannot evaluate reports "not applicable" or
  flags what it could not model; it never returns a false pass. Findings cite the net,
  component, or datasheet page they come from, and unverified data is marked as such. You can
  always tell "clean" from "not checked".
- **Verify against reality.** Readers and rules are checked against the native tools
  (`kicad-cli` ERC/DRC) and real design exports, not only hand-written fixtures. A feature is
  done when it works on a real file.
- **The core is open, with a clear boundary.** The engine is shareable under Apache-2.0.
  Proprietary formats, house rules, and confidential designs live in a private extension that
  depends on the engine without forking it.
- **Agni stays legible to software engineers.** EDA carries decades of domain vocabulary. It maps it
  to concepts software engineers already know (an IR, a linter, a semantic diff, a lockfile),
  so you can contribute without an EE degree. See
  [the software-analogy map](https://panyam.github.io/agni/reference/analogy/).

## Formats read today

| Format | Extensions | Netlist | Faithful geometry |
| --- | --- | --- | --- |
| EDIF 2.0.0 | `.edn` `.edf` `.edif` (netlist), `.eds` (schematic) | yes | schematic |
| KiCad | `.kicad_sch` `.kicad_pcb` `.kicad_pro` | yes | schematic + board |
| IPC-2581 | `.xml` `.cvg` | yes | board |
| xschem | `.sch` (sniffed) | yes | schematic |
| gEDA gschem | `.sch` (sniffed) | yes | schematic |
| Telesis | `.tel` | yes | none |

A hierarchical EDIF netlist reads its top cell by default and says which blocks it left out.
`--flatten-hierarchy`, `flatten_hierarchy: true` in a design's `design.yaml`, or the viewer's
**Read every block** reads every block it places.

## Documentation

Full documentation lives at [panyam.github.io/agni](https://panyam.github.io/agni/).

- [Getting started](https://panyam.github.io/agni/guide/getting-started/) covers installing,
  a first check, reading the report, and gating CI.
- [User guide](https://panyam.github.io/agni/guide/) covers concepts, the CLI, checks, diff, and
  the query language, for someone new to the tool.
- [Software-analogy map](https://panyam.github.io/agni/reference/analogy/) maps
  hardware ideas onto software ones. If you read code but not schematics, start here.
- [Overview](https://panyam.github.io/agni/overview/) opens the engineering docs on IR and
  ingestion, geometry and rendering, semantic diff, and the format primers.
- [examples/README.md](examples/README.md) lists runnable walkthroughs, one per feature.
- [CONSTRAINTS.md](CONSTRAINTS.md) holds the enforceable architectural rules. Read it before
  proposing changes.
- [SECURITY.md](SECURITY.md) says how to report a vulnerability.
- [Open core](https://panyam.github.io/agni/decisions/open-core/) explains the split between this
  public engine and a private extension that adds proprietary readers and rules without forking
  it.

## Status

Agni reads real exports from every listed format and runs its full analysis over them. It is
young, with bounded format coverage (see each reader's notes), a growing rule catalog, and an
early datasheet extraction pipeline. The architecture is settled and the breadth is still
growing. Issues and readers for new formats are welcome.

## Building

`make build` requires Go 1.26.9 and pnpm (for the web viewer bundle). `make testall` also needs
`buf` 1.61 on your PATH, `python3`, a Chromium for the browser tests, and network access to fetch
the pinned sample corpus. `CONTRIBUTING.md` has the details.

```
pnpm install                      # once: the web workspace (web/, web-shared/, datasheet/web/)
make build                        # web bundle + go build ./...
make install                      # install the agni CLI to $GOBIN
make testall                      # the full gate: vet, lint, gosec, gitleaks, Go, wasm, web, browser and Python tests, docs checks
```

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
