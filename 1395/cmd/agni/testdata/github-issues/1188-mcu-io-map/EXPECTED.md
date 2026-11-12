# panyam/agni #1188, MCU pin assignment vs IO map

Fixtures: `demo_mcu.edn`, and `design.yaml`, whose `intent: io_map:` is the IO assignment. The
issue's `mcu`, `ball`, `assigned_function` and `expected_net` columns are an `io_map` row's
`device`, `pin`, `function` and `net`.

## Result

| Ball | Declared function | Expected net | Netlist (U1) | Result |
|------|-------------------|--------------|--------------|--------|
| PTA0 | GPIO_LED | LED_NET | pin 3 on `LED_NET` | pass |
| PTA1 | UART_TX | UART_TX_NET | pin 4 on no net | fail (`io-map-net-absent`) |

`agni check --format html` writes it as the "Pins on U1" section, and Tabulate's `pin_map` table
carries the same rows. `TestPinMapTableAnswersBothIssues` in `cmd/agni/check_pinmap_test.go` holds
both.

## Verify

```bash
agni check cmd/agni/testdata/github-issues/1188-mcu-io-map --format html -o /tmp/out.html
agni check cmd/agni/testdata/github-issues/1188-mcu-io-map --verdicts --format json | jq '{check: .}' | agni tabulate -
```
