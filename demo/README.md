# Demo boards

Two shareable KiCad schematics that exercise the engine end to end. They carry no private
data, so this is the "try it in 60 seconds" path on a fresh clone.

- `showcase.passes.*` — a clean board. `agni check` finds nothing to report.
- `showcase.fires.*` — the same board with deliberate design issues, so the rule checks
  have something to say.

Each design is a KiCad project pair: the `.kicad_pro` is a stub, the `.kicad_sch` holds the
schematic. `agni` reads the project by its stem.

## Run the checks

```
make agni
./bin/agni check demo/showcase.fires.kicad_pro
```

You should see structural findings in plain language: a missing I2C pull-up, no ESD
protection on the exposed USB data lines, power rails without decoupling or bulk caps, and
rails with no test point for bring-up.

## Open the browser viewer

```
make demo
```

This builds the web bundle and serves the viewer with this folder mounted. Open the printed
URL, pick `showcase.fires.kicad_pro` in the left tree, and you get the faithful schematic
render, a checks panel that locates each finding on the canvas, and a datalog query panel.
Load `showcase.passes.kicad_pro` to compare against the clean version.
