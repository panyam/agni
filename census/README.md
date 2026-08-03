# census — the element-coverage guard (WS6-011)

Readers deliberately drop constructs they do not yet consume (silkscreen text, silk graphics,
buses, `net=` taps were all dropped silently until someone compared our render to KiCad by eye).
This package makes those drops **visible and reviewed**: for each format it holds a manifest
classifying every source construct as `consumed` or a known drop (with a reason and, where
tracked, a roadmap ticket).

## Two tiers

- **CI gate** — `census_test.go` walks the committed `*/testdata` fixtures and fails if a
  construct is not classified. A new fixture that introduces an unclassified element forces a
  human to decide (consume it or mark it a known drop) instead of dropping it silently. Runs in
  `make test` / `make testall`.
- **Corpus report** — `agni census <dir>` runs the same audit over a local corpus (which CI
  cannot see); a workspace Makefile can wrap it as `make census` to sweep your private corpus
  directories. A clean run means every real-world construct is classified. Report-only, like
  `make realtest`.

The manifest is the single reviewed source of truth both tiers drive.

## Workflow

- **A reader starts consuming a construct** → flip its entry to `Consumed` in `manifests.go`; the
  diff shows the coverage change.
- **`agni census` reports an unclassified construct** → classify it in the format's manifest:
  `co` (consumed), `dc`/`da`/`dl` (dropped — cosmetic / analysis-gap / correctness-latent, with a
  ticket), or `dd` (dropped by design — editor/tool metadata).
- **A new corpus file** surfaces its new constructs automatically on the next `make census`.

## What it does NOT do

It asserts *classification* coverage, not behavioral consumption — "a construct we never decided
about appeared", not "the reader extracts it correctly". Behavioral correctness is the
conformance harness's job (`check`, WS6-004). The two are complementary.

Seed classifications and the motivating audit are in the private research repo (`docs/18`,
reader-coverage audit).
