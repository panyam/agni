# multi-format

Read the same board from EDIF, KiCad, and IPC-2581 and watch the neutral IR converge. This
example demonstrates the "many readers, one IR" thesis.

## What it shows

- Three readers normalizing three source formats of one board (R1, R2, U1; nets VCC, GND,
  SIG) into the same `ir.Design`.
- `diff.Designs` used as a convergence oracle, where EDIF-read vs KiCad-read and EDIF-read vs
  IPC-read report **zero net changes** and the same component set. Same connectivity, keyed on
  `(ref_des, pin)`, regardless of format.
- Component attributes legitimately differ, such as a Value or a part reference. The physical
  tier differs too, since footprints, layers, stackup and BOM appear only for KiCad and
  IPC-2581. The semantic netlist converges, and format-specific detail is kept in attributes
  and the physical tier (CONSTRAINTS C9).

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

This example reads a fixed bundled trio (`mixer.edn`, `mixer.kicad_pcb`,
`mixer.ipc2581.xml`) rather than prompting for a path, since the point is the matched set.

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's
`FromMarkdown`. `main.go` binds the `read` step (stats per format) and the `converge` step
(`diff.Designs` over the pairs). The fixtures live in [`../common/designs`](../common/designs);
see [`../CONVENTIONS.md`](../CONVENTIONS.md) for the shared layout.
