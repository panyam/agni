---
title: "6. Part limits"
description: "Compare the design against what a part's datasheet actually allows, and read the verdict for data nobody has checked."
---

Every rule so far reasoned about structure: what is connected to what. A whole class of real defects
is not structural at all. A part sitting on a rail above its absolute maximum is wired perfectly
correctly and will still fail. Deciding that needs a number that exists only on the part's
datasheet.

A parameter set is that number, in a form a rule can compare against. `params/` holds one file per
part worth checking. This project declares that directory the same way it declares
`conventions.yaml` and `profiles/`, so naming the design is enough for `check` and `review` to pick
the corpus up, and neither carries a `--params` flag below.

## Silent without it

This run moves the project's `params/` aside first, so what you see is the rule with nothing to
compare against:

{{ agniRun "content/tutorials/runs/06-check-designs-gateway-gateway-edn-rule-suppl.yaml" }}

The rule ran. It had no datasheet limits, so it decided nothing. Note that a bare "no findings" reads
the same here as it would on a board where the rule ran and everything was fine, reproducing exactly
the ambiguity rung 9 exists to resolve.

## With the parameter set

Put the corpus back and the same command decides:

{{ agniRun "content/tutorials/runs/06-check-abs-max-params.yaml" }}

U2's input sits on a 3.3 V rail and its datasheet says 3.0 V is the absolute maximum. The finding
carries where that limit came from, which matters more than it looks: a claim about a part is only
as good as the document behind it, and "which page of which revision" is the first question anyone
asks.

## What a seeded part looks like

```
mpn: "ACME-BUCK-3V3"
manufacturer: "Agni Tutorial Works"
device_class: "regulator"

docs {
  id: "acme-buck-3v3-rev-a"
  title: "ACME-BUCK-3V3 Datasheet Rev A"
}

parameters {
  name: "Input voltage"
  symbol: "VIN"
  limit_kind: LIMIT_KIND_ABSOLUTE_MAX
  value { max: 36 }
  unit: "V"
  conditions { symbol: "TA" eq: 25 unit: "C" raw: "TA = 25C" }
  condition_coverage: CONDITION_COVERAGE_COMPLETE
  prov { doc_ref: "acme-buck-3v3-rev-a" page: 3 table_or_figure: "Absolute Maximum Ratings" method: "hand" confidence: 1 }
}
```

Parts join to the design by `mpn`, so a component with no MPN cannot be checked no matter how well
seeded the corpus is.

`limit_kind` separates an absolute maximum (exceed it and the part is damaged) from a recommended
operating range (exceed it and behavior is no longer guaranteed). They are different questions and
conflating them produces either false alarms or missed defects.

`condition_coverage` is a gate rather than documentation. A limit that holds only under conditions
nobody recorded cannot be compared automatically, and a row marked anything other than complete or
unconditional is skipped rather than guessed at. This trips people writing their first spec by hand:
omit it and the row is silently ignored.

## Data nobody has checked

`params/acme-ldo-1v8.textproto` is the other half of the pair. Same shape, one difference:

```
prov { doc_ref: "acme-ldo-1v8-placeholder" method: "mock" confidence: 0.3 }
```

It is a placeholder. Somebody typed a plausible number to get the pipeline working and never went to
the datasheet. That is the honest state of most parameter corpora early on, and pretending otherwise
is how a review loses credibility.

Run the checklist and the verdict says so:

```
| P4 | no part is operated above its absolute-maximum supply voltage | provisional | supply-exceeds-abs-max: U2 (... mock, confidence 0.3) |
```

**`provisional`**, not `fail`. The finding is real and it is shown to you in full. What the engine
declines to do is call it a defect on the strength of data nobody has verified. Chase it and you may
find a genuine problem, or you may find the placeholder was wrong.

`--ratified-floor` sets where that line sits. The default is 0.9, so anything below that confidence
reports provisional.

And with the corpus moved aside again, so the rule has no seeded spec for U2 at all:

```
| P4 | no part is operated above its absolute-maximum supply voltage | not-applicable | needs a seeded datasheet parameter set (check --params) |
```

The remedy names the flag because that is the route for a design read on its own. Inside a project
that declares `params/`, the fix is to seed the part rather than to pass anything.

Three distinct states for one item: decided on trusted data, decided on untrusted data, and not
decidable at all. None of them is a pass.

## Where to start

`agni intake <design>` lists every MPN on the board with no seeded spec. That list is your work
queue, and it shrinks as you seed parts. Start with the parts whose limits you would
actually worry about, which is usually regulators, {{ explainable "transceiver" "transceivers" }}, and anything near a rail boundary,
rather than working alphabetically.

## Next

[Your architecture](../07-your-architecture/), the last of the four tiers.
