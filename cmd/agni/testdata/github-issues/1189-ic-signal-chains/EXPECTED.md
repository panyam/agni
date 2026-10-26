# panyam/agni #1189 — Per-signal connectivity chains (anchor IC)

Fixture: `demo_phy.edn`

Topology:

- `U2.MDIO` → `R1.1` → `R1.2` → `U3.MDIO` (MDIO_NET / MDIO_OUT)
- `U2.~RST` on net `RST_N` with no path to a partner (intentional fail row)

## Desired HTML / Tabulate row (anchor `U2`)

| Signal | Local pin | Partner | Status | Chain |
|--------|-----------|---------|--------|-------|
| MDIO | MDIO | U3.MDIO | pass | U2.MDIO → R1.1 → R1.2 → U3.MDIO |
| RESET | ~RST | (none) | fail | U2.~RST |

## Current behavior

`agni stats --as-named demo_phy.edn` ingests; check HTML lists rule findings only, not this inventory.

## Verify

```bash
agni stats --as-named cmd/agni/testdata/github-issues/1189-ic-signal-chains/demo_phy.edn
```

Trace interactively (optional):

```bash
agni trace U2.MDIO U3.MDIO --snapshot …
```
