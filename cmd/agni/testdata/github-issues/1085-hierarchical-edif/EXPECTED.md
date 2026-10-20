# panyam/agni #1085 / #1086 — hierarchical EDIF

Fixture: `hier_two_subcells.edn`

TOP places two hierarchical instances (`U1` → cell `SUB_A`, `U2` → cell `SUB_B`). Inside `SUB_A`, net `NET_LED_CATHODE` connects `D1-K` to `R1-1`.

## Golden model (after flatten)

| Metric | Expected |
|--------|----------|
| Components (unique ref_des) | 3 (`D1`, `R1`, `C1`) or 5 if hierarchy refs counted — document agni’s ref policy |
| Nets (named) | 3 (`NET_LED_CATHODE`, `NET_LED_ANODE`, `NET_DECAP`) |
| `NET_LED_CATHODE` pins | `D1-K`, `R1-1` |

## Current behavior (v0.3.0)

`agni stats --as-named hier_two_subcells.edn` reports:

- `components: 2` (only `U1`, `U2` at TOP)
- `nets: 0` at TOP
- `not extracted: 2 sub-cells holding 3 instances (SUB_A: 2, SUB_B: 1)`
- Hierarchy note: top cell only; `D1`/`R1`/`C1` not in the model

Checks and diff run on the empty TOP shell, not the LED/decap content.

## Desired behavior

No `not extracted` line (or explicit opt-in to top-cell-only). Stats and ingest match the golden table. `DiffDesigns` sees `NET_LED_CATHODE` on `D1-K`.

## Related upstream

agni `readers/edif/testdata/hier.edn` is a smaller single-subcell variant of the same class (issue #718).
