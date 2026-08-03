## What this changes

A short description of the change and why.

## Testing

How you verified it. `make testall` should pass; note anything you exercised beyond it (a
real design, a rendered output, a corpus sweep).

## Checklist

- [ ] `make testall` passes locally
- [ ] New behavior has a test (readers: a fixture in the package's `testdata/`)
- [ ] A new rule ships its `check/docs/<name>.md`; user-facing changes update the relevant `docs/`
- [ ] The change reads the IR, not source files, and adds no format-specific IR field (CONSTRAINTS C9)
