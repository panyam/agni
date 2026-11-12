# panyam/agni #1089 / #1090: pull-up reach hop limit

Fixture: `four_hop_sda.edn`, with `intent.yaml` beside it.

Signal `SDA_MAIN` reaches rail `VCC_3V3` through **four** series resistors `R1`→`R2`→`R3`→`R4` (no
parallel shortcut). `VCC_3V3` is a name the default rail vocabulary recognises, so the hop count is
the only thing the verdict turns on.

## At the default reach

`i2c-pull-up` fails `SDA_MAIN`, and the witness states the limit it searched to:

```
no rail is reachable from SDA_MAIN through a resistor within 3 hops
```

## With the declared reach

`intent.yaml` declares `SDA_MAIN: {pull_up_reach_hops: 4}` (agni issue 820). `i2c-pull-up` passes
`SDA_MAIN` through all four resistors:

```
SDA_MAIN reaches rail VCC_3V3 through R1 then R2 then R3 then R4
```

and `intent/pull-up-reach-declared` passes, since the design carries a net named `SDA_MAIN`.

## Verify

```bash
agni check --verdicts --rule i2c-pull-up four_hop_sda.edn
agni check --verdicts --rule i2c-pull-up four_hop_sda.edn --intent-path intent.yaml
```

`TestGitHubIssueReproducers` in `cmd/agni` runs both and holds them to this file.
