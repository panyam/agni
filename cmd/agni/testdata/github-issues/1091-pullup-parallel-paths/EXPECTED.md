# panyam/agni #1091 / #1092 — enumerate all pull-up paths

Fixture: `parallel_scl_pullups.edn`

Net `SCL_MAIN` has **two** parallel resistors `R_A` and `R_B` to the same rail `RAIL_3V3`.

## Expected evidence (legacy audit columns)

Check evidence or tabulated output lists **both** paths:

1. `SCL_MAIN` → `R_A` → `RAIL_3V3`
2. `SCL_MAIN` → `R_B` → `RAIL_3V3`

Parallel pull-up count = 2.

## Current behavior

Pull-up rules usually emit **one** shortest witness path (either `R_A` or `R_B`).

## Verify

Run pull-up / redundant-pull-up checks; inspect JSON/HTML evidence paths.
