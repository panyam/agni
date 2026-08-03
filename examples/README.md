# Agni Examples

Runnable, narrated walkthroughs of the engine. Each reads a bundled design into the neutral
IR and shows one thing you can do with it. They are demokit walkthroughs: run one live at
the CLI, step through it in a TUI, or render it to markdown.

Every example shares [`common/`](common/) for design loading, the bundled synthetic
fixtures, and the narration helpers. Adding or changing an example? Follow
[CONVENTIONS.md](CONVENTIONS.md).

## The ladder

Read them in this order. Each rung builds on the idea before it, and the last one runs all
of them at once.

| # | Example | Shows | Status |
|---|---------|-------|--------|
| 1 | [read-and-stats/](read-and-stats/) | Read a source file into the IR; components / sections / nets, and the physical tier for board formats. The walkthrough form of `agni stats`. | ready |
| 2 | [multi-format/](multi-format/) | Read the same board from EDIF, KiCad, and IPC-2581 and watch the IR converge. Why the neutral IR is the whole point. | ready |
| 3 | [checks/](checks/) | Run structural rule checks (`check.Run`) over one design and narrate the findings. The walkthrough form of `agni check`. | ready |
| 4 | [diff/](diff/) | Semantic diff of two revisions (`diff.Designs`): renamed / hard / soft / new / deleted. The walkthrough form of `agni diff`. | ready |
| 5 | [convert/](convert/) | Read any of three formats into the IR and emit IPC-2581 (`N -> IR -> N`); proves the semantic round-trip. The walkthrough form of `agni emit`. | ready |
| 6 | [render-schematic/](render-schematic/) | Read schematic geometry (`.eds`) into the sidecar and render one sheet two ways: SVG (offline) and the tier-2 pack the `web/` WebGL2 viewer loads. | ready |
| 7 | [validate/](validate/) | Run the reader-health invariants (`validate.Design` / `validate.Geometry`) over a design and read the problem lists. The walkthrough form of `agni validate`. | ready |
| 8 | [whole-enchilada/](whole-enchilada/) | The capstone: all of the above end to end in one tour — convergence, checks, diff, emit, and both renderers. | ready |

New here? Start with `whole-enchilada` for the full tour, then use rungs 1-6 to go deep on
each step. Examples are tracked as roadmap tickets (workstream WS8).

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
- No network at run time: the fixtures are embedded and demokit is a normal module
  dependency.
