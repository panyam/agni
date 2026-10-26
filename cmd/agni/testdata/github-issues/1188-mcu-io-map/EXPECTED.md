# panyam/agni #1188 — MCU pin assignment vs IO map

Fixtures: `demo_mcu.edn`, `io_map.yaml` (illustrative input; not ingested today).

## Intended mapping

| Ball | Declared function | Expected net | Netlist (U1) | Desired result |
|------|-------------------|--------------|--------------|----------------|
| PTA0 | GPIO_LED | LED_NET | pin 3 on `LED_NET` | pass |
| PTA1 | UART_TX | UART_TX_NET | pin 4 not on `UART_TX_NET` | fail |

## Current behavior

`agni stats --as-named demo_mcu.edn` ingests the netlist only. Check HTML has no rows joined to `io_map.yaml`.

## Desired behavior

Documented ingest + rule or Tabulate/HTML rows comparing `io_map.yaml` to wired nets per ball/refdes pin.

## Verify

```bash
agni stats --as-named cmd/agni/testdata/github-issues/1188-mcu-io-map/demo_mcu.edn
agni check … --format html   # after feature lands
```
