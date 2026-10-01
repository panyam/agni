# census, the element-coverage guard (WS6-011)

Readers deliberately drop constructs they do not yet consume. Silkscreen text, silk graphics, buses
and `net=` taps were all dropped silently until someone compared our render to KiCad by eye. This
package makes those drops **visible and reviewed**. For each format it holds a manifest classifying
every source construct as `consumed` or as a known drop, with a reason and, where tracked, a roadmap
ticket.

## Two tiers

- **The CI gate** is `census_test.go`, which walks the committed `*/testdata` fixtures and fails if
  a construct is not classified. A new fixture that introduces an unclassified element forces a human
  to decide (consume it or mark it a known drop) instead of dropping it silently. It runs in
  `make test` and `make testall`.
- **The corpus report** is `agni census <dir>`, which runs the same audit over a local corpus that
  CI cannot see, and a workspace Makefile can wrap it to sweep your own corpus directories. A clean
  run means every real-world construct is classified. It reports and never fails the gate.

Both tiers read the same manifest, and it is the one reviewed record of what each reader drops.

## Workflow

- When **a reader starts consuming a construct**, flip its entry to `Consumed` in `manifests.go`, and
  the diff shows the coverage change.
- When **`agni census` reports an unclassified construct**, classify it in the format's manifest as
  `co` (consumed), `dc`, `da` or `dl` (dropped as cosmetic, analysis-gap or correctness-latent, each
  with a ticket), or `dd` (dropped by design, for editor and tool metadata).
- **A new corpus file** surfaces its new constructs on the next `agni census` run over it.

## What it does NOT do

It asserts *classification* coverage, not behavioral consumption. It catches "a construct we never
decided about appeared", not "the reader extracts it correctly". Behavioral correctness belongs to
the conformance harness (`check`, WS6-004), and the two complement each other.
