# whole-enchilada

The capstone example: the whole engine end to end over bundled synthetic designs. One
walkthrough shows every surface at once.

1. **Converge** — read the `mixer` board from EDIF, KiCad, and IPC-2581; diff them to prove the netlists agree.
2. **Check** — structural rules over `i2c-sensor`.
3. **Diff** — the full change taxonomy on the `rev-a` -> `rev-b` revision pair.
4. **Emit** — convert the mixer's EDIF netlist to IPC-2581 and round-trip it.
5. **Schematic** — render `demo-schematic.eds` to `schematic.svg`.
6. **Graph** — lay out a netlist as a connectivity graph (`graph.svg`) from the IR alone.

The per-feature examples (read-and-stats, checks, convert, render-schematic) go deeper on
each step and accept your own files. This one is the tour.

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

`make run` writes `schematic.svg` and `graph.svg` in this directory; open them in a browser.
