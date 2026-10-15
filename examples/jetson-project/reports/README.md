# KiCad's own DRC of the Jetson board

`kicad-drc.json` is KiCad's design-rule check of the board, kept so the tutorial's rung 12 can bring
another tool's report into agni without needing KiCad installed. It is the unmodified output of

```
kicad-cli pcb drc --format json -o kicad-drc.json designs/jetson/jetson-agx-thor-baseboard.kicad_pcb
```

run with KiCad 10.0.6 on macOS, on the board from agni-samples v0.1.2 (`hack/samples.pin`). It lists
578 violations: 357 `clearance`, 199 `lib_footprint_issues`, 10 `silk_edge_clearance`, 5
`silk_over_copper`, 3 `courtyards_overlap`, 2 `pth_inside_courtyard` and 2 `npth_inside_courtyard`.
Re-run the command after a samples bump and commit the result.
