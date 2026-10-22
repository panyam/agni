---
title: "6. Part limits"
description: "Compare the design against what a part's datasheet actually allows, and read the verdict for data nobody has checked."
---

Every rule so far reasoned about structure, about what is connected to what. A whole class of real defects
is not structural at all. A part sitting on a rail above its absolute maximum is wired
correctly and will still fail. Deciding that needs a number that exists only on the part's
datasheet.

A parameter set is that number, in a form a rule can compare against. `params/` holds one file per
part worth checking. The project names that directory in its `project.yaml` (`params: [params]`), as
it names `profiles/`, so naming the design is enough for `check` and `review` to pick the corpus up,
and neither carries a `--params` flag below.

## Silent without it

This run deletes the `params` line from the project's `project.yaml` first, so what you see is the
rule with nothing to compare against:

{{ agniRun "content/tutorials/runs/06-check-params-off.yaml" }}

Both rules ran. They had no datasheet limits, so they decided nothing and could not even say what
they would have looked at. A bare "no findings" reads the same here as it would on a board where the rule ran
and everything was fine, and rung 9 exists to resolve that ambiguity.

## With the parameter set

The project seeds one part, the NTS0102 level translator. Four of them sit between the module's 1.8 V
I/O and the board's 3.3 V and 5 V peripherals, and one has its B side on `+5V`, which is the reason to
check it at all. With the corpus named, as `project.yaml` names it, the same rules decide, and with `--verdicts` they say what
each pin was compared against:

{{ agniRun "content/tutorials/runs/06-check-abs-max-params.yaml" }}

Every supply pin passes against the datasheet's 6.5 V absolute maximum, including the one on `+5V`.
The answer comes from `pin-exceeds-abs-max`, which judges each pin against the row bound to it, and
`supply-exceeds-abs-max` stays silent, because a part whose spec binds its rows to pins is left to the
per-pin rule. The four `not-considered` rows are the part's ground pins, which the board's KiCad symbol
types as a supply input. The spec declares only the two supplies, so a ground pin resolves to nothing
in it and the rule declines it rather than guessing which limit applies.

## What a seeded part looks like

```
mpn: "NTS0102GT"
manufacturer: "NXP Semiconductors"

docs {
  id: "nts0102-rev-4"
  title: "NTS0102 Dual supply translating transceiver; open drain; auto direction sensing, Rev. 4, 23 January 2013"
  vendor: "NXP Semiconductors"
}

pins {
  id: "vccb"
  name: "VCC(B)"
  function: PIN_FUNCTION_POWER_INPUT
  prov { doc_ref: "nts0102-rev-4" page: 5 table_or_figure: "Table 5. Limiting values" method: "hand" confidence: 1 }
}

parameters {
  name: "Supply voltage B, VCC(B)"
  symbol: "VCC(B)"
  limit_kind: LIMIT_KIND_ABSOLUTE_MAX
  value { min: -0.5 max: 6.5 }
  unit: "V"
  condition_coverage: CONDITION_COVERAGE_UNCONDITIONAL
  pin_refs: "vccb"
  prov { doc_ref: "nts0102-rev-4" page: 5 table_or_figure: "Table 5. Limiting values" method: "hand" confidence: 1 }
}
```

The file carries one such row per supply, for both the absolute maximum and the recommended range,
each citing the table and page it came from. A claim about a part is only as good as the document
behind it, and "which page of which revision" is the first question anyone asks.

Parts join to the design by `mpn`, so a component with no MPN cannot be checked no matter how well
seeded the corpus is.

`limit_kind` separates an absolute maximum (exceed it and the part is damaged) from a recommended
operating range (exceed it and behavior is no longer guaranteed). They are different questions and
conflating them produces either false alarms or missed defects, as the next section shows.

`pin_refs` binds a row to the pin it is about. The NTS0102 has two supplies with one absolute maximum
and different recommended ranges, 1.65 to 3.6 V on `VCC(A)` and 2.3 to 5.5 V on `VCC(B)`, so a row
that named neither would leave the rules unable to say which range belongs to which pin. The pins are
declared under the names the datasheet prints, and the board's symbol, which writes `VCC_{A}`, still
matches, because a pin name is compared with its subscript folded. A row with no `pin_refs` is about
the part as a whole, and its `symbol` must then be one of a short list of supply spellings (`VIN`,
`VDD`, `VCC` and a few more) for `supply-exceeds-abs-max` to read it.

`condition_coverage` is a gate rather than documentation. A limit that holds only under conditions
nobody recorded cannot be compared automatically, and a row marked anything other than complete or
unconditional is skipped rather than guessed at. This trips people writing their first spec by hand,
because a row that omits it is silently ignored. The NTS0102's limiting values carry no conditions at
all, so they are unconditional.

## Data nobody has checked

The project keeps a first pass at the same part in `drafts/`, where nothing reads it. Somebody typed
it before going to the document, took the 3.6 V at the top of VCC(A)'s recommended range, entered it as
the absolute maximum, and marked it honestly:

```
prov { doc_ref: "nts0102-first-pass" method: "mock" confidence: 0.3 }
```

Most parameter corpora look like this early on, and pretending otherwise is how a review loses
credibility. Copy it over the transcribed file and run the rule again:

{{ agniRun "content/tutorials/runs/06-check-first-pass.yaml" }}

The finding is real, and it is shown to you in full, with its provenance. Chase it and you find that
3.6 V was never the limit, that it is the recommended ceiling for the OTHER supply, and that the
part is fine at 5 V on its B side. That is the case the provenance exists for, because the finding
told you exactly how far to trust it.

A checklist makes the same distinction a verdict. An item resting on this finding reads
**`provisional`** rather than `fail`, because the engine declines to call a defect on the strength of
data nobody has verified, and `--ratified-floor` sets where that line sits (0.9 by default).
[Rung 9](../09-read-the-verdicts/) shows it. With no seeded spec at all the same item reads
`not-applicable`, with a reason naming what is missing.

One item can land in three distinct states: decided on trusted data, decided on untrusted data, and not
decidable at all. None of them is a pass.

## Where to start

`agni intake <design>` lists every MPN on the board with no seeded spec. That list is your work
queue, and it shrinks as you seed parts. Start with the parts whose limits you would
actually worry about, which is usually regulators, {{ explainable "transceiver" "transceivers" }}, and anything near a rail boundary,
rather than working alphabetically.

## Next

[Your architecture](../07-your-architecture/), the last of the four tiers.
