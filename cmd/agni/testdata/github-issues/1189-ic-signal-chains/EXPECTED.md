# panyam/agni #1189, per-signal connectivity chains for an anchor IC

Fixtures: `demo_phy.edn`, and `design.yaml`, which declares U2's signals as `io_map` rows, each
with the partner pin it must reach under `to`.

Topology:

- `U2.MDIO` on `MDIO_NET`, through `R1`, to `U3.MDIO` on `MDIO_OUT`
- `U2.RST` on `RST_N` with nothing else on it, so it never reaches `U3.RST` (the fail row)

## Result (anchor `U2`)

| Signal | Local pin | Partner | Status | Chain |
|--------|-----------|---------|--------|-------|
| MDIO | MDIO | U3.MDIO | pass | MDIO_NET -> [R1] -> MDIO_OUT |
| RESET | RST | U3.RST | fail | (no route) |

The chain is the route `agni trace` prints, net to net through each series part. `agni check
--format html` writes it as the "Pins on U2" section, and Tabulate's `pin_map` table carries the
same rows.

## Verify

```bash
agni check cmd/agni/testdata/github-issues/1189-ic-signal-chains --format html -o /tmp/out.html
agni trace cmd/agni/testdata/github-issues/1189-ic-signal-chains --from U2.1 --to U3.1   # trace takes designators, not pin names
```
