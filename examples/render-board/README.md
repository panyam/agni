# render-board

The board rung of the examples ladder. Read a KiCad board (`.kicad_pcb`) into the WS1-006
board geometry sidecar and render the physical board — outline, per-layer copper, pads,
vias, zones — plus a net-highlight overlay. It is the walkthrough form of
`agni render <file>.kicad_pcb` (which draws the board by default) and of the viewer's
"Board" sheet.

## What it shows

- `kicad.ReadBoardGeometry` → `geom.BoardGeometry`: layers, outline, placements with
  footprint-local pads, copper grouped per net — a peer sidecar to the schematic geometry.
- `render.BoardSVG` — the SVG backend; the document is stratified into classed layer
  groups, so layer visibility is a CSS toggle (the web viewer's front/back/all selector).
- `render.HighlightBoardSVG` — the board face of the highlight contract: net → routed
  copper + connected pads, ref_des → pads. Framed exactly like the base document.

`.kicad_pcb` only for now: it is the only producer of the board sidecar (IPC-2581 is the
ticketed second format). The packed/WebGL board tier is WS7-035.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```
