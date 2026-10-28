---
title: "Datasheets"
description: "Give the tool a part's real limits as data, and it checks every design that uses the part against them."
---

Most rules only need your design. A few can also check it against a **part's real limits**
from its datasheet, once you give the tool those limits as data. This page turns that on.

Transcribe a part's limits once (the
{{ explainable "absolute-maximum-rating" "Absolute Maximum Ratings" }}, the operating range)
into a small file, and the tool compares every design that uses the part against them. See
"datasheets as checkable data" in [Concepts](../concepts/).

## Load a parameter set

A parameter set is a directory of small text files, one per part, each holding that part's
specs. Point `check` at it with `--params`:

{{ agniRun "content/guide/runs/datasheet-abs-max.yaml" }}

In that finding, the design drives a `+24V` rail into `U1` pin 1, whose datasheet caps
VIN at 20V absolute maximum. The message carries a **dual citation**, your design side
(the rail and pin) and the datasheet side (document, page, and the exact table). You can
open the datasheet to page 4 and confirm it.

For the tool to match a datasheet to a placed part, the design has to name the part, either with an
MPN on the BOM line or with the MPN/Manufacturer properties a schematic symbol carries.

## What the tool will and will not auto-compare

The tool is deliberately conservative about when it compares a number automatically.

- A limit stated as a plain number the tool can act on (VIN abs-max = 20V) is
  **machine-comparable** and can fire a finding.
- A limit that only holds under a **text condition** the tool cannot evaluate ("20V at 25°C
  ambient, derate above") is shown to a human rather than auto-compared, because applying that
  {{ explainable "derating" }} is a judgement rather than a comparison.
- A part whose spec is missing the fields a rule needs is **under-specified** and is skipped,
  not guessed.

This is why an empty or partial parameter set makes datasheet rules go quiet rather than
wrong, because with no data the rule has nothing to compare (every other tier works the same way).

The last line of the run above is where that shows up. One subject was considered and one was
**not considered**, which is the rule declining rather than passing it. A findings report on its own
cannot tell you which of the two happened, so read that line alongside the count.

## Confidence and provenance

Each spec value records where it came from and how much to trust it. A limit typed in by a
person reads as the highest confidence. A value extracted automatically from a PDF carries a
lower confidence and its own page/table citation. The finding message surfaces this (the
`(hand, confidence 1)` tail above), so a reviewer can weigh a machine-extracted limit
differently from a hand-verified one.

## Where the specs come from

You author a parameter set by transcribing the limits you care about (facts from a datasheet
are not copyrightable, so cite the document revision and page). There is also a pipeline that
extracts specs from a PDF automatically, which is a separate tool covered in
[the datasheet layer](../../architecture/datasheet-layer/). Either way the result is the same
small per-part files this page loads.

Getting the PDFs is usually the slow part. Several vendor sites refuse a download that does not come
from a browser: one times out, one answers 403, and two return a bot-check page instead of the file.
Save the PDFs from a browser into the folder `DATASHEET_DIR` names, and the datasheet tooling reads
them from there.

### Publishing a workbench draft

The datasheets workbench (`agnids serve --corpus <dir>`) keeps what you transcribe as a DRAFT for one
part number, in the corpus store beside the published specs. Opening a datasheet lists the drafts that
cite it, one click apart, and New draft starts another, since a family datasheet covers several
parts. A new draft starts from an MPN the workbench suggests from the file name, and nothing is saved
until you confirm or edit it. After that, every edit saves, without validating, so a half-finished
transcription is never lost. No check reads a draft.

Publishing is the step between the two, the workbench's Publish button or `agnids publish`. It
validates the draft, refuses one that fails (listing every
problem), and refuses when another published file already seeds the same MPN, since one MPN in two
files fails every load. Otherwise it writes `<mpn>.textproto`, with any character outside
`A-Za-z0-9._-` replaced by `_`, and its text is the same from every build. Publishing an MPN again
replaces its earlier published spec, and the draft stays, as the start of the next edit.

```
agnids publish LM1117 --corpus params/
```

Publishing also records the spec in the corpus's index, `corpus.index.json`, which maps each MPN to
its file and a hash of what was validated, under a generation that advances on every change. The
files stay the source of truth. After editing a published spec by hand, rebuild it, and let a
corpus repository's CI catch an edit that skipped the rebuild:

```
agnids index params/
agnids index params/ --check
```

### Merging a pin-mux table

A microcontroller's pin-mux table often already exists as a file of its own, transcribed from the
reference manual by the team allocating its pins. `agnids merge-mux` merges one into the spec the
corpus publishes for that part, so the corpus keeps one file per MPN and nobody has to merge the
table into the spec by hand.

The table is written as a part spec in YAML holding only pins, with the same field names a published
spec uses. A pin lists every function its mux table gives it under `alternates`, and names its
designators when the table prints them:

```yaml
mpn: KW47
pins:
  - name: PTB3
    numbers: [{package_ref: 48qfn, number: "1"}]
    alternates: [{name: PTB3}, {name: LPUART1_RX}, {name: TPM1_CH3}]
  - name: PTB5
    alternates: [{name: PTB5}, {name: LPI2C1_SCL}, {name: I3C0_SCL}]
```

```
agnids merge-mux kw47.pin_mux.yaml --corpus params/
```

Each pin in the table joins the published pin it describes by `id`, else by a designator the two
share in a package, else by name. A joined pin takes the table's alternates in place of its own, so
merging a revised table again drops any function the revision dropped. Its name, provenance and id
stay as published, because the spec's parameters point at the id. A table pin that joins nothing is
added, and then needs its own `prov` and the `docs` entry it cites, as any published pin does.

The merge is refused when it cannot tell which pin a row means, as with a name such as `NC` that the
part prints on several pins, so give that row the published pin's `id`. It is also refused when the
table names a different part, or reuses a document or package id for something else. The merged spec
is validated before anything is written, exactly as `agnids publish` validates a draft, and replaces
the file that already seeds the part. A part the corpus does not seed yet is published from the
table alone. A table carrying parameters is refused, because a part's limits belong in its published
spec and a merge should not be able to override them.

### Serving a shared corpus

A project's own `params/` is read straight from disk, which suits the few dozen parts a project
seeds. A corpus shared across projects is served instead: `agnids serve --corpus <dir>` answers
lookups from its index, reading only the parts asked for, and `agni serve --params-url` reads through
it. It indexes the corpus at start when the index is missing or stale, and serves the API alone when
there is no workbench build, so a deployment that only publishes specs needs none.

```
agnids serve --addr :8090 --corpus params/
agni serve --mount boards=~/boards --params-url http://localhost:8090
```

Each request fetches its design's parts in one batch. A spec promoted into the corpus while both run
reaches requests within a few seconds, with no restart, because the server re-checks the index
generation. If the corpus cannot be reached, a check fails with an error naming it rather than
treating every part as unseeded. A file edited behind a running server's index is refused until
`agnids index` is run over it, or the server restarts, because serving a file the index did not
validate would hand out a spec nobody checked.

A project's own `params/` sits over the shared corpus, part by part. The project's spec answers for
every MPN it seeds and the shared corpus answers for the rest, the same way over `--params`. Every
datasheet citation says which corpus it came from: a verdict's citation carries `corpus` as
`project` or `shared`, the terminal marks a value from outside the project as "(shared corpus)", and
the params panel shows each spec's corpus. A project that wants a part judged only by its own
numbers seeds that part.

## Where to go next

- [Checks and reports](../checks-and-reports/) walks the general report-reading flow these
  findings appear in.
- [CLI reference](../cli-reference/) lists `--params` and the other flags.
