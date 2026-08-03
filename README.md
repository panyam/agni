# Agni

[![CI](https://github.com/panyam/agni/actions/workflows/ci.yml/badge.svg)](https://github.com/panyam/agni/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/panyam/agni.svg)](https://pkg.go.dev/github.com/panyam/agni)
[![Go Report Card](https://goreportcard.com/badge/github.com/panyam/agni)](https://goreportcard.com/report/github.com/panyam/agni)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go 1.26](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)

Agni is an engine for electronic design files. It reads schematics and PCBs from several
formats into one neutral, protobuf-defined IR, then checks, diffs, renders, and queries
them. The front-end normalizes formats the way a compiler normalizes languages into one AST,
so every analysis downstream is written once and works on all of them.

![The agni viewer: a schematic renders, structural checks run, and each finding locates on the canvas](docsite/static/images/demo-viewer.gif)

## What it does

- **Reads many formats into one IR.** EDIF netlists and schematics, KiCad schematics and
  boards, IPC-2581, xschem, and gEDA all parse into the same `ir.Design`. Adding a reader is
  one entry in `formats/registry.go`.
- **Structural checks (ERC/DRC-like).** Missing I2C pull-ups, unprotected exposed signals,
  power rails without decoupling, boards that fail track-width rules. Findings come out in
  plain language and cite the net or component they fire on.
- **Revision diff over the IR.** Compares two revisions structurally (components, nets,
  connectivity), not as a text diff of the source files, so it survives reformatting and
  rename churn.
- **Rendering.** Faithful schematic and board geometry, or an auto-laid-out netlist graph,
  to SVG or a WebGL canvas.
- **A browser viewer.** `agni serve` opens the tree, renders a design, runs the checks, and
  locates each finding on the canvas.
- **A datalog query surface.** Ask arbitrary questions of the design fact base
  (`agni query`), the same fact base the rules are built on.
- **A datasheet parameter layer.** Join a design against extracted datasheet limits and
  check, for example, that a rail stays inside a part's recommended operating range.

## Try it in 60 seconds

No private data needed. The `demo/` folder holds two shareable KiCad boards: a clean one and
the same board with deliberate design issues.

```
git clone https://github.com/panyam/agni
cd agni
make agni
./bin/agni check demo/showcase.fires.kicad_pro
```

```
findings by rule:
  bulk-cap               2
  decoupling-present     2
  esd-protection         2
  i2c-pull-up            1
  input-protection       1
  test-point-coverage    2

  [error]   i2c-pull-up: SCL (I2C net has no pull-up resistor)
  [warning] input-protection: VBUS (connector feeds a power input with no fuse or TVS in the path)
  [info]    esd-protection: USB_D+ (externally-exposed signal net has no ESD protection)
  ...
```

Then open the browser viewer on the same boards:

```
make demo
```

Load `showcase.fires.kicad_pro` in the left tree, press Run checks, and click a finding to
locate its net on the schematic. See [demo/README.md](demo/README.md).

## How it works

One contract sits in the middle: the protobuf IR (`protos/`, generated into `gen/`). Readers
(`edif/`, `kicad/`, `ipc2581/`, and the xschem/gEDA readers) are the only code that knows a
file format; they produce `ir.Design`. Everything else — `check/`, `diff/`, `render/`, the
query engine, the web service — consumes the IR and never looks at a source file. Add a
reader and every analysis works on the new format for free. Add an analysis and it works on
every format for free.

The same shape repeats at two more contracts: a geometry IR that N producers fill and N
renderers draw, and a parameter IR that N datasheet extractors fill and the checks read.

## Philosophy

- **One neutral IR, many formats.** A schematic is a schematic whether it came from KiCad,
  EDIF, or IPC-2581. Normalize each format once, and write every analysis once against the
  IR. Add a reader and every check, diff, render, and query works on the new format; add an
  analysis and it works on every format.
- **Format-neutrality is enforced, not aspirational.** Analyses read the IR, never source
  files, and the IR carries no field a second format could not populate. Architectural
  constraints checked in CI keep the core from accreting format-specific special cases.
- **Silence is never coverage.** A check that cannot evaluate reports "not applicable" or
  flags what it could not model; it never returns a false pass. Findings cite the net,
  component, or datasheet page they come from, and unverified data is marked as such. You can
  always tell "clean" from "not checked".
- **Verify against reality.** Readers and rules are checked against the native tools
  (`kicad-cli` ERC/DRC) and real design exports, not only hand-written fixtures. A feature is
  done when it works on a real file.
- **Open core with a clear boundary.** The engine is shareable under Apache-2.0. Proprietary
  formats, house rules, and confidential designs live in a private overlay that depends on
  the engine without forking it. Company-specific material stays in the overlay, never in the
  shared engine.
- **Legible to software engineers.** EDA carries decades of domain vocabulary. Agni maps it
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

## Documentation

Full documentation lives at [panyam.github.io/agni](https://panyam.github.io/agni/).

- [Getting started](https://panyam.github.io/agni/guide/getting-started/) — build, symbol
  libraries for xschem/gEDA, native EDA tools, golden comparisons.
- [User guide](https://panyam.github.io/agni/guide/) — concepts, the CLI, checks, diff, and
  the query language, written for someone new to the tool.
- [Software-analogy map](https://panyam.github.io/agni/reference/analogy/) — the
  hardware-to-software analogy map. If you read code but not schematics, start here.
- [Overview](https://panyam.github.io/agni/overview/) — the engineering docs: IR and
  ingestion, geometry and rendering, semantic diff, format primers.
- [examples/README.md](examples/README.md) — runnable walkthroughs, one per feature.
- [CONSTRAINTS.md](CONSTRAINTS.md) — the enforceable architectural rules. Read before
  proposing changes.
- [Open core](https://panyam.github.io/agni/decisions/open-core/) — the open-core split: this
  public engine, and how a private overlay adds proprietary readers and rules without forking
  it.

## Status

Agni reads real exports from every listed format and runs its full analysis over them. It is
young: the format coverage is bounded (see each reader's notes), the rule catalog is growing,
and the datasheet extraction pipeline is early. The architecture is the settled part; the
breadth is the work in progress. Issues and readers for new formats are welcome.

## Building

Requires Go 1.26 and pnpm (for the web viewer bundle).

```
cd web && pnpm install && cd ..   # once
make build                        # web bundle + go build ./...
make install                      # install the agni CLI to $GOBIN
make testall                      # the full gate: vet, tests, bundle, web unit tests
```

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
