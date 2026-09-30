# Agni Examples

Runnable, narrated walkthroughs of the engine. Each reads a bundled design into the neutral
IR and shows one thing you can do with it. They are demokit walkthroughs: run one live at
the CLI, step through it in a TUI, or render it to markdown.

Every example shares [`common/`](common/) for design loading, the bundled synthetic
fixtures, and the narration helpers. [CONVENTIONS.md](CONVENTIONS.md) says how to add or change
an example.

## The ladder

Read them in this order. Each rung builds on the idea before it, and the last one runs the
first six end to end in one tour.

| # | Example | Shows | Status |
|---|---------|-------|--------|
| 1 | [read-and-stats/](read-and-stats/) | Read a source file into the IR; components / sections / nets, and the physical tier for board formats. The walkthrough form of `agni stats`. | ready |
| 2 | [multi-format/](multi-format/) | Read the same board from EDIF, KiCad, and IPC-2581 and watch the IR converge. | ready |
| 3 | [checks/](checks/) | Run structural rule checks (`check.RunDesign`) over one design and narrate the findings. The walkthrough form of `agni check`. | ready |
| 4 | [diff/](diff/) | Semantic diff of two revisions (`diff.Designs`): renamed / hard / soft / new / deleted. The walkthrough form of `agni diff`. | ready |
| 5 | [convert/](convert/) | Read any of three formats into the IR and emit IPC-2581 (`N -> IR -> N`); proves the semantic round-trip. The walkthrough form of `agni emit`. | ready |
| 6 | [render-schematic/](render-schematic/) | Read schematic geometry (`.eds`) into the sidecar and render one sheet two ways: SVG (offline) and the tier-2 pack the `web/` WebGL2 viewer loads. | ready |
| 6b | [render-board/](render-board/) | Read a KiCad board (`.kicad_pcb`) into the board geometry sidecar and render the physical board (outline, per-layer copper, pads, vias, zones) plus a net-highlight overlay. The walkthrough form of `agni render <file>.kicad_pcb`. | ready |
| 7 | [render-highlight/](render-highlight/) | Run the rule catalog, then bake each finding's subject into one rendered SVG so a report finding becomes a picture of the real design. The walkthrough form of `agni render --highlight`. | ready |
| 8 | [validate/](validate/) | Run the reader-health invariants (`validate.Design` / `validate.Geometry`) over a design and read the problem lists. The walkthrough form of `agni validate`. | ready |
| 9 | [resolve-design/](resolve-design/) | Which design does this file belong to? `project.yaml` / `design.yaml` descriptors, and resolving a file to the design that declares its entry. What is behind `agni check <design-folder>`. | ready |
| 10 | [trace/](trace/) | Follow one pin to another through the series parts in the way (`check.TracePins`) and read the route, the nets it passes through, and what else sits on them. The walkthrough form of `agni trace`. | ready |
| 11 | [dft-coverage/](dft-coverage/) | Design for test: which nets a probe can reach, and which parts a tester can measure. Queries over the fact relations rather than the rule catalog, so it is also the tour of derived relations, aggregates and negation. | ready |
| 12 | [netlist-audit/](netlist-audit/) | A netlist audit as one query set: the tables a review workbook holds, as named queries in `audit.yaml` sharing a preamble of derived relations, all answered over one read of the design (`query.ParseQuerySet`, `agni query --set`). | ready |
| 13 | [design-review/](design-review/) | A review checklist run against a board: every item resolves to pass, fail, not-applicable, needs-a-declaration, or nothing covers it. The rung that composes the others, and the only one whose point is what it CANNOT answer. | ready |
| 14 | [whole-enchilada/](whole-enchilada/) | The capstone: rungs 2 to 6 end to end in one tour (convergence, checks, diff, emit, and both renderers). | ready |

A first read should start with `whole-enchilada` for the tour, then use rungs 1-13 to go deep
on each step.

## `tutorial-project/`, off the ladder

[`tutorial-project/`](tutorial-project/) is a different kind of artifact and does not belong in the
table above. The examples are Go programs that narrate one engine capability. That folder is a
worked *review project*, meaning a synthetic board plus the checklist, conventions, interface
profiles, datasheet parameters, and design intent a team wraps around one. It is what the docs site tutorials
run against, and it is a copyable starting point for your own project.

It is not a Go module and has no `main.go`. Drive it with `make review` from inside the folder.

## The extension modules, also off the ladder

[`extension/`](extension/) and [`extension-template/`](extension-template/) are Go modules that
extend the engine from outside, with their own format reader and rules, rather than narrated
walkthroughs. `extension` is the worked example and `extension-template` is the bare scaffold to
copy. The docsite page `docsite/content/build/extending.md` walks through both.

## Run one

```bash
cd read-and-stats
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

## Prerequisites

- Go 1.26+
- No network at run time, because the fixtures are embedded and demokit is a normal module
  dependency.
