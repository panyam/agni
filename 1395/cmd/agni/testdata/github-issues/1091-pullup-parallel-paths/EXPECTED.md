# panyam/agni #1091 / #1092: enumerate all pull-up paths

Fixture: `parallel_scl_pullups.edn`

Net `SCL_MAIN` has **two** parallel resistors `R_A` and `R_B` to the same rail `VCC_3V3`.

## Expected evidence

`i2c-pull-up` passes `SCL_MAIN` and lists **both** paths as `path` terms (agni issue 821):

1. `SCL_MAIN -> [R_A] -> VCC_3V3`
2. `SCL_MAIN -> [R_B] -> VCC_3V3`

`i2c-redundant-pull-up` fails `SCL_MAIN` with `pull-ups=2` and the same two `path` terms (agni issue
1165), since two resistors in parallel to one rail are one smaller pull-up.

## Verify

```bash
agni check --verdicts --format csv --rule i2c-pull-up --rule i2c-redundant-pull-up parallel_scl_pullups.edn
```

`TestGitHubIssueReproducers` in `cmd/agni` runs it and holds it to this file.
