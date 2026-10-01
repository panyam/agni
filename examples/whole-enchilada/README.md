# whole-enchilada

The capstone example, which runs the whole engine end to end over bundled synthetic designs.
One walkthrough shows the main steps at once.

1. **Converge** three reads of the `mixer` board (EDIF, KiCad, and IPC-2581) and diff them to prove the netlists agree.
2. **Check** `i2c-sensor` against the structural rules.
3. **Diff** the `rev-a` -> `rev-b` revision pair across the full change taxonomy.
4. **Emit** the mixer's EDIF netlist as IPC-2581 and round-trip it.
5. **Render** `demo-schematic.eds` to `schematic.svg`.
6. **Lay out** a netlist as a connectivity graph (`graph.svg`) from the IR alone.

The per-feature examples (read-and-stats, checks, convert, render-schematic) go deeper on
each step and accept your own files. This one is the tour.

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

`make run` writes `schematic.svg` and `graph.svg` in this directory; open them in a browser.

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's
`FromMarkdown`. `main.go` binds the six steps that run engine code (`converge`, `check`, `diff`,
`emit`, `schematic`, `graph`) and wires the renderer. See [`../CONVENTIONS.md`](../CONVENTIONS.md)
for the layout every example follows.
