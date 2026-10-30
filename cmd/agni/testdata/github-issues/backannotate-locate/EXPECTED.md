# Schematic locate: `instanceBackAnnotate` vs inline page designator

Sanitized reproducer for viewer/query locate when the **netlist ref-des** and the **on-sheet label** diverge, or when the logical ref appears only in `(viewMap (instanceBackAnnotate …))`.

Filed as [#1228](https://github.com/panyam/agni/issues/1228). `TestBackAnnotatedPartsLocateOnTheirSheet` in `service/` reads this folder.

## Fixtures

| File | Role |
|------|------|
| `demo_netlist.edn` | Connectivity entry: instances `R1001`, `U101`, net `I2C_SCL` |
| `demo_schematic.eds` | Faithful geometry: page labels `R4` (no inline ref for MCU), `viewMap` maps `&i_pull`→`R1001`, `&i_mcu`→`U101` |
| `design.yaml` | Declares schematic companion (same pattern as production EDIF split) |

Reader-only slices (same scenarios, single file each):

- `readers/edif/testdata/backannotate-inline-mismatch.eds` — inline `R4`, back-annotate `R1001`
- `readers/edif/testdata/backannotate-only.eds` — no inline ref-des, back-annotate `U101`

## Behavior before #1228

1. **`entity(?name, ?kind)`** with `^R1001$` or `^U101$` finds the component in the **netlist** tier.
2. Query/viewer **locate** uses schematic **placements** keyed by inline `(designator …)` on the page instance only.
3. **`R1001`**: geometry indexes the pull-up under **`R4`** → empty `sheetIds` for `R1001`, `LOCATE_REASON_NO_GEOMETRY`, no highlight.
4. **`U101`**: page instance has **no** component designator; back-annotate holds `U101` → same `NO_GEOMETRY` symptom.
5. Net **`I2C_SCL`** may still get sheet badges from wire geometry and/or netlist sheet attributes (component locate stays geometry-only).

## Behavior since #1228

After applying `instanceBackAnnotate` (prefer over conflicting inline text when joining to netlist identity):

- `entity` rows for `R1001` and `U101` carry non-empty `sheetIds` (`P1` / `P2`) and `LOCATE_REASON_UNSPECIFIED`.
- Find-by-name and check focus navigate to the correct sheet and highlight the symbol.
- `readers/edif` placement `RefDes` is `R1001` / `U101`, not `R4` / empty.

## Verify

From repo root (`main/`):

```bash
# Reader: placement ref-des comes from instanceBackAnnotate
go test ./readers/edif/ -run BackAnnotate -v

# Netlist + companion (project descriptor)
agni stats cmd/agni/testdata/github-issues/backannotate-locate/demo_netlist.edn

agni query cmd/agni/testdata/github-issues/backannotate-locate/demo_netlist.edn \
  'entity(?name, ?kind), str.match(?name, "^R1001$") => ?name, ?kind' --format json

agni query cmd/agni/testdata/github-issues/backannotate-locate/demo_netlist.edn \
  'entity(?name, ?kind), str.match(?name, "^U101$") => ?name, ?kind' --format json
```

Inspect `entities[].sheetIds` and `entities[].reason` in JSON output.

Optional: run from the fixture directory with a local `design.yaml` mount when exercising `agni serve` + viewer Find by name.

## Demo script (viewer)

1. Open the design (entry `demo_netlist.edn`, companion schematic loaded).
2. Query → Find by name → `R1001` → click the netlist cell (not only a sheet badge).
3. **Today:** stay on current sheet / no highlight; message *drawn on no sheet*.
4. **Fixed:** jump to **Pull-up sheet** and highlight the resistor keyed as `R1001`.

Repeat for `U101` → **MCU sheet**.
