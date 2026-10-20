# panyam/agni #1093 / #1094 — Tabulate interface profile coverage

Fixture: `i2c_and_spi_ports.edn`

Synthetic buses:

- `I2C_ON_PORT0_SDA` / `I2C_ON_PORT0_SCL` — should match an I2C interface profile instance (e.g. port 0).
- `SPI_ON_PORT1_MOSI` / `MISO` / `SCK` — SPI on port 1.

## Expected Tabulate output

A workbook/RPC projection with rows akin to:

| Profile | Instance / bus | Signals |
|---------|----------------|---------|
| I2C | port 0 | SDA, SCL |
| SPI | port 1 | MOSI, MISO, SCK |

## Current behavior

`Tabulate` exposes check findings and verdicts; there is no standard **coverage** sheet for “which profile instances were evaluated.”

## Verify

Run `CheckDesign` with house interface profiles enabled, then `Tabulate` (or team driver). Compare to manual profile→net mapping.
