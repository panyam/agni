# panyam/agni #1095 / #1096 — threshold on query counts

Fixture: `gnd_tp_ratio.edn`

| Quantity | Count |
|----------|------:|
| Named nets (excluding implicit power if your query excludes them) | 11 (`GND` + `SIG1`…`SIG10`) |
| Test points on `GND` | 2 (`TP1`, `TP2`) |

House rule example: **pass** when `count(ground_test_points) >= count(nets) / 30`.

Here `2 >= 11/30` → pass.

## Expected product behavior

Express the ratio in **data** (checklist item, query post-processing, or jaala arithmetic), not in a one-off Python driver between two `RunQuery` calls.

## Verify

Run queries for net count and GND-attached TPs; apply threshold in checklist when #1095 is implemented.
