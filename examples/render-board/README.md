# render-board

The board rung of the examples ladder. Read a KiCad board (`.kicad_pcb`) into the WS1-006
board geometry sidecar and render the physical board (outline, per-layer copper, pads,
vias, zones) plus a net-highlight overlay. It is the walkthrough form of
`agni render <file>.kicad_pcb` (which draws the board by default) and of the viewer's
"Board" sheet.

## What it shows

- `kicad.ReadBoardGeometry` → `geom.BoardGeometry`, a peer sidecar to the schematic geometry
  holding layers, outline, placements with footprint-local pads, and copper grouped per net.
- `render.BoardSVG`, the SVG backend, which stratifies the document into classed layer
  groups so layer visibility is a CSS toggle (the web viewer's front/back/all selector).
- `render.HighlightBoardSVG`, the board face of the highlight contract, which maps a net to
  its routed copper and connected pads and a ref_des to its pads, framed exactly like the base
  document.

This example reads `.kicad_pcb` through `kicad.ReadBoardGeometry`. IPC-2581 is the board
sidecar's second producer (`ipc2581.ReadBoardGeometry`), and `core/render/packboard.go` packs a
board for the WebGL viewer (WS7-035).

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
`highlight`) and wires the renderer. See [`../CONVENTIONS.md`](../CONVENTIONS.md) for the layout
every example follows.
