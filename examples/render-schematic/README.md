# render-schematic

The render rung of the examples ladder. Read an EDIF schematic (`.eds`) into the geometry
sidecar and drive both render backends over it: the SVG one (offline/verification) and the
tier-2 packer that feeds the WebGL2 viewer in `web/`. It is the walkthrough form of
`agni render` (the default SVG backend) and `agni render --format=pack`.

## What it shows

- `common.LoadSchematic` → `geom.SchematicGeometry`, which holds a symbol library plus sheets
  of placements, wires, and labels, keyed to the netlist IR but separate from it (CONSTRAINTS C21).
- `render.SheetSVG`, the offline backend, which writes a `render.svg` you can open in any viewer.
- `render.PackSheet`, the tier-2 columnar projection (int32 vertices + primitive records)
  the browser uploads once.
- One geometry, two backends over the same render layer.

This example reads EDIF `.eds` only, because `common.LoadSchematic` does. KiCad, xschem and
gEDA schematics also carry the geometry sidecar, and `agni render` draws any of them
(`render-highlight` uses a KiCad one).

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's
`FromMarkdown`. `main.go` binds the four steps that run engine code (`pick`, `read`, `svg`,
`pack`) and wires the renderer. See [`../CONVENTIONS.md`](../CONVENTIONS.md) for the layout
every example follows.
