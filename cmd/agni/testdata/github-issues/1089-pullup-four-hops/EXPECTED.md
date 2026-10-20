# panyam/agni #1089 / #1090 — pull-up reach hop limit

Fixture: `four_hop_sda.edn`

Signal `SDA_MAIN` reaches rail `RAIL_3V3` through **four** series resistors `R1`→`R2`→`R3`→`R4` (no parallel shortcut).

## Expected (with hop limit ≥ 4)

Built-in I2C/MDIO pull-up reach (or project config `pull_up_reach_hops: 4`) should **pass**: `SDA_MAIN` has a pull-up path to a 3V3-class rail.

## Current behavior (default hop limit 3)

Rule typically **fails** pull-up reach at three hops even though a valid fourth resistor completes the path.

## Verify

Ingest with team/checksmith preset, run `CheckDesign`, inspect pull-up / I2C rules on `SDA_MAIN`.

## Priority

P2 unless a production board fails only for depth-4 topology.
