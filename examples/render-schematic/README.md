# render-schematic

The render rung of the examples ladder. Read an EDIF schematic (`.eds`) into the geometry
sidecar and drive both render backends over it: the SVG one (offline/verification) and the
tier-2 packer that feeds the WebGL2 viewer in `web/`. It is the walkthrough form of
`agni render` (the default SVG backend) and `agni render --format=pack`.

## What it shows

- `common.ReadSchematicFixture` → `geom.SchematicGeometry`: a symbol library plus sheets of
  placements, wires, and labels, keyed to the netlist IR but separate from it (CONSTRAINTS C1).
- `render.SheetSVG` — the offline backend; writes `render.svg` you can open in any viewer.
- `render.PackSheet` — the tier-2 columnar projection (int32 vertices + primitive records)
  the browser uploads once; the `web/` viewer loads it via `?src=`.
- One geometry, two backends over the same render layer.

EDIF `.eds` only for now: it is the only reader that emits the geometry sidecar. KiCad and
IPC-2581 produce the netlist IR (see `read-and-stats`), not schematic geometry.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```
