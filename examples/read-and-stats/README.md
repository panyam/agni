# read-and-stats

The first rung of the examples ladder. Read a source file into Agni's neutral IR and
read off what came back: components (grouped by ref_des), sections, nets, and, for board
formats, the physical tier. It is the walkthrough form of `agni stats <file>`.

## What it shows

- `common.Load` picking a reader by extension, the same dispatch `agni`'s CLI does at its
  I/O edge. File paths stay at the edge; the core readers only see an `io.Reader`
  (CONSTRAINTS C1). The design input is a path (default `../common/designs/two-resistors.edn`),
  so you can point it at your own file.
- The neutral IR: one shape for every source format, so diff and checks downstream never
  care which format you started from.
- The `(ref_des, pin)` connection key that diff and checks match on, never a format-native
  id.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

Pick any of the bundled designs at the first step: `two-resistors.edn` (a bare EDIF
netlist), `demo-board.kicad_pcb`, or `demo-board.ipc2581.xml` (both add physical-tier rows).

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's
`FromMarkdown`. `main.go` only binds the three steps that run engine code (`pick`, `read`,
`nets`) and wires the renderer. Design loading, the bundled fixtures, and the stats
pretty-printers come from [`../common`](../common). See
[`../CONVENTIONS.md`](../CONVENTIONS.md) for the layout every example follows.
