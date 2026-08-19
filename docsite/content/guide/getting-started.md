---
title: "Getting started"
description: "From nothing to a first checked design in a few minutes."
---

This page gets you from nothing to a first checked design in a few minutes. It assumes the
vocabulary from [Concepts](../concepts/) (what a *finding*, a *tier*, and *provenance* mean).
If a word here is unfamiliar, that page is the glossary.

## Install

If you have a Go toolchain, install the CLI straight onto your `PATH`:

```
go install github.com/panyam/agni/cmd/agni@v0.1.1
```

Pin a released version rather than `@latest` for anything whose output you keep. A check report
is only reproducible if you can say which build produced it, and `@latest` moves under you
between runs. Releases are plain git tags, so
[the tag list](https://github.com/panyam/agni/tags) is the version list.

Or build from a clone, which also gives you the sample designs used below:

```
git clone https://github.com/panyam/agni
cd agni
make install      # installs `agni` into your GOBIN
# or: make agni  # builds ./bin/agni without installing
```

Confirm it runs:

```
agni --help
```

## Your first check

`agni check` runs the rule catalog over one design and prints what it found. Point it at
your own schematic or board, or at a sample from the clone. Here is a sample that
deliberately trips several rules:

```
agni check cmd/agni/testdata/conformance/showcase.fires.kicad_pro
```

```
findings by rule:
  bulk-cap               2
  decoupling-present     2
  esd-protection         2
  i2c-pull-up            1
  input-protection       1
  test-point-coverage    2

first 10:
  [warning] bulk-cap: +3V3 (power rail has no bulk capacitor)
  [warning] decoupling-present: +3V3 (power rail has no decoupling capacitor)
  [info] esd-protection: USB_D+ (externally-exposed signal net has no ESD protection)
  [error] i2c-pull-up: SCL (I2C net has no pull-up resistor to a rail)
  [warning] input-protection: VBUS (connector feeds a power input with no fuse or TVS in the path)
  [info] test-point-coverage: GND (rail carries no test point; bring-up and factory test cannot probe it)
  ...

10 finding(s) total
```

Read a finding as three parts: the **severity** (`error` / `warning` / `info`), the
**rule** that fired (`i2c-pull-up`), and the **subject** it fired on (the net `SCL`), with a
plain-language reason in parentheses. Each finding also carries its provenance: run
`--format json` (below) to see exactly which net or pin, and for datasheet rules which page
and table, the finding came from.

Severity is a policy signal, not a measure of certainty. An `error` is something you almost
certainly must fix (an I2C bus with no pull-up will not communicate). `info` is a note worth
a look.

## A clean run, and why "no findings" has a number in it

Run the passing twin of that board:

```
agni check cmd/agni/testdata/conformance/showcase.passes.kicad_pro
```

```
no findings (29 rule(s) run)
```

The `29 rule(s) run` is the important half. It tells you the check actually *exercised* 29
rules and none fired, rather than staying quiet because it had nothing to work with. This is
the "silence is not a pass" idea from [Concepts](../concepts/): a real all-clear names how
many rules ran. If you load only a schematic and no board file, the copper rules simply do
not appear in that count, because their tier is empty.

## Sanity-check the read first

Before trusting any finding, confirm the tool read your design the way you expect. `agni
stats` summarizes what it ingested:

```
agni stats cmd/agni/testdata/conformance/showcase.fires.kicad_pro
```

```
design:              Showcase Board (fires)
source format:       kicad-sch
libraries:           2
components:          13 (unique ref_des)
nets:                11
```

If the component or net counts look wrong, the findings downstream will too. Fix the read
(often a missing symbol library, see the `--symbol-path` note in the CLI reference) before
chasing a surprising finding.

## Other output formats

The default text form is a summary. Two others are useful early:

- `agni check <file> --format markdown` renders a severity-organized report, worst first,
  for pasting into a review.
- `agni check <file> --format json` emits one object per finding with its full subject and
  provenance, for tooling.

## Gate a build on it

`--fail-on` makes `check` exit non-zero when anything at or above a severity is present, so
it can sit in CI:

```
agni check <file> --fail-on error   # fails the build only on errors
```

## Stop passing flags

Everything above addresses a single file, and the flags pile up as you turn things on: your naming
conventions, your interface profiles, your parameter corpus, your checklist. A **project** is where
those live instead, declared once beside the design.

`agni start` builds one from a design you already have:

```
agni start boards/gateway.edn ./gateway-review
```

```
Created project "gateway-review".

  gateway-review/project.yaml
  gateway-review/conventions.yaml        (stub — your team's naming vocabulary)
  gateway-review/review.yaml             (seeded from the shipped catalog — edit it)
  gateway-review/designs/gateway/design.yaml
  gateway-review/designs/gateway/gateway.edn   (copied)
```

From then on the commands take a design and nothing else, because the project answers the rest:

```
agni check gateway-review/designs/gateway
agni review gateway-review/designs/gateway
```

The design is **copied** into the project, which now owns its copy, so edits to the original do not
reach it. And the generated `review.yaml` is a starting point seeded from the shipped catalog, not a
finished checklist; [Write your checklist](../../tutorials/08-write-your-checklist/) is about turning
it into your team's.

## Where to go next

- [Checks and reports](../checks-and-reports/): narrow to one rule or category, read the
  full report, and follow a finding's provenance.
- [Datasheets](../datasheets/): add a parameter set so checks can compare your design
  against a part's real limits.
- [CLI reference](../cli-reference/): the full command and flag surface.
