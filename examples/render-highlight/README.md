# render-highlight

The explainability rung of the examples ladder. Run the rule catalog over a design, then bake
each finding's subject into a single rendered SVG so a report finding becomes a picture of the
actual design. It is the offline, no-server twin of the web viewer's click-to-locate, and the
walkthrough form of `agni render --highlight`.

## What it shows

- `check.RunDesign` → `[]check.Finding`: the built-in rules over the netlist model; each finding
  names a subject (net, ref-des, or pin).
- `specsForFindings` maps each finding subject to a `geom.HighlightSpec` — the same net/component/pin
  vocabulary the web click-to-locate builds. Findings computed on the netlist locate on the geometry
  by name/ref-des, never merged as a second component source (CONSTRAINTS C21).
- `render.SheetSVGHighlighted` — draws the base sheet and composites the highlights onto ONE canvas,
  the same projection the server serves as a separate overlay. One code path for the CLI static
  picture and the live viewer.
- The bundled default (`duplicate-refdes.kicad_sch`) flags U1 as a duplicate ref-des, so both
  offending symbols are framed on the faithful KiCad render.

The `--highlight` CLI flag drives the same primitive by hand: `net=<name>`, `ref=<refdes>`, or
`pin=<refdes>:<pin>`, repeatable, with optional `shape=outline|rect|circle|path`, `color=#rrggbb`,
`alpha=0..1`.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```
