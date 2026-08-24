---
title: "8. Write your checklist"
description: "The questions your team asks of every board, bound to the engine so they answer themselves."
---

Most teams already have a review checklist. It is usually a spreadsheet, and going through it is
usually somebody's afternoon. Much of it is mechanical: questions with a definite answer that is
already sitting in the design file.

`review.yaml` is that checklist, written so the mechanical items answer themselves and the rest stay
visible as work for a human.

The shift from `check` is worth naming. `agni check` answers "what is wrong with this board".
`agni review` answers "which of our questions did we actually answer", which is a different question
and a more useful one when you are deciding whether a board is ready.

## The shape

```yaml
name: Sample Board design review
areas:
  - name: Power
    items:
      - id: "P1"
        title: every rail carries a bulk capacitor
        description: A rail with no bulk capacitance browns out on a load step.
        rule: bulk-cap
```

Areas group items the way your existing checklist groups them. Each item keeps its own `id`, so an
item that has been "P1" in your process for years stays P1 here and nobody has to relearn numbering.

## Four ways to bind an item

**`rule:`** hands the item to a catalog rule. Any rule, from any tier: built-in, one your
conventions file added, a profile rule, or an intent rule.

```yaml
- {id: "P2", title: every rail carries decoupling, rule: decoupling-present}
- {id: "H1", title: net names follow house convention, rule: gateway/signal-net-naming}
- {id: "A1", title: each rail sits at its declared voltage, rule: intent/voltage-domain-mismatch}
```

**`profile:`** hands it to an interface profile, so the item covers everything that profile
requires, and reads as unevaluated rather than passed when the interface is absent.

```yaml
- {id: "I1", title: the CAN interface is complete and terminated, profile: CAN}
```

**`query:`** is for a question no shipped rule asks. You write the datalog inline.

```yaml
- id: "H2"
  title: every test point sits on a recognized rail
  query:
    match: 'component.class(?r, "test_point"), pin.net(?r, ?p, ?n), not rail(?n) => ?r, ?n'
    subject: r
    message: '{r} probes {n}, which is not a recognized rail'
```

**`note:`** is for a question nothing automated can answer. The item stays on the checklist and
reports honestly.

```yaml
- {id: "H3", title: the assembly drawing lists a torque spec for every fastener,
   note: manual review against the mechanical drawing package}
```

That last one matters more than it looks. The temptation with an unautomatable item is to drop it,
and then it is not on the checklist at all. A `note:` keeps the question visible and says who owns
it.

## The query trap

Read the `H2` query again. It matches {{ explainable "test-point" "test points" }} on nets that are **not** rails.

A query binding reports whatever it matches as findings, and findings mean the item failed. So the
query has to match the violation. Write the healthy case instead, phrasing it the natural way as
"every test point sits on a rail", and you get an item that fails on a good board and passes on a
bad one.

This is the most common authoring mistake, and it is quiet, because on a healthy board an inverted
item just looks like a finding you have not got round to.

## Running it

```
agni review designs/gateway
```

That is the whole command. Every input a review needs is something this project already declares:
`review.yaml` is its checklist, `conventions.yaml` its naming vocabulary, `profiles/` its interfaces,
`params/` its part limits, and `designs/gateway/design.yaml` names the netlist to read and the board
beside it. Naming the design is enough because the project answers the rest.

It says which checklist it picked, on stderr:

```
note: running the checklist projects/gateway declares (mount://gateway/review.yaml); pass --checklist to run a different one.
```

That note matters more than it looks. Which checklist scored a run is not recoverable from the
outcomes it produced, so a checklist you did not type has to announce itself.

```
# Review: Sample Board design review

Design: `designs/gateway`

**3 pass, 9 fail, 0 n/a, 2 not-automated, 1 provisional (of 15)**

## Power

1 pass, 3 fail, 0 n/a, 0 not-automated, 1 provisional (of 5)

| # | Title | Outcome | Detail |
|---|-------|---------|--------|
| P1 | every rail carries a bulk capacitor | pass |  |
| P2 | every rail carries decoupling | fail | decoupling-present: PMIC_MAIN_12V0 (power rail has no decoupling capacitor) |
| P3 | the input rail is protected against reverse polarity | fail | reverse-blocking-absent: PMIC_MAIN_12V0 (connector feeds a power input with no reverse-blocking element in the path) |
| P4 | no part is operated above its absolute-maximum supply voltage | provisional | supply-exceeds-abs-max: U2 (... mock, confidence 0.3) |
| P5 | every rail is probeable during bring-up | fail | test-point-coverage: GND (rail carries no test point...); test-point-coverage: PMIC_MAIN_12V0 (...) |
```

Each item carries its outcome and the evidence behind it. An item bound to a rule that fired shows
what fired and on which net, so the checklist row and the debugging detail are the same artifact.

## Items about the board, not the netlist

Some questions are about copper, and a netlist has none:

```yaml
- {id: "B1", title: no track is below the fab's minimum width, rule: track-width}
```

On this project it resolves, because `design.yaml` declares the board as a companion of the netlist
and the run reads both:

```
| B1 | no track is below the fab's minimum width | fail | track-width: CAN1_CANH (net has 1 track segment(s) narrower than the 0.127mm fabrication floor) |
```

To see what the item does with no copper, read the netlist on its own with `--as-named`, the flag
that says "exactly the file I named, not the design it belongs to":

```
agni review --as-named designs/gateway/gateway.edn --checklist review.yaml
```

```
| B1 | no track is below the fab's minimum width | not-applicable | design carries no board geometry (WS1-006 sidecar) |
```

Nothing about the item changed. What changed is what it had to work with, and a question that could
not be asked became a defect with a named net.

`--board-path` does the same job for a board that is *not* a declared companion: a fab's returned
file, or a layout under review that has not landed in the design yet.

Worth being precise about what that proves. The `n/a` was not hiding a failure and it was not
standing in for a pass. It was the honest report of a question with nothing to evaluate, and the
only way to find out which it would have been was to supply the copper. An item that had scored
`pass` on the netlist alone would have been wrong, and nobody would ever have checked it again.

## Growing it

Start with the items your team already argues about, not with the ones that are easiest to automate.
An item bound to nothing is still worth adding as a `note:`, because the checklist is then complete
and the automation gap is measurable rather than invisible.

For a checklist of this size, hand-authoring the YAML is right. Teams running a few hundred items
usually move to one file per item with the manifest generated from them, so items can be edited in
parallel without conflicting.

## Next

[Read the verdicts](../09-read-the-verdicts/), which is about why "9 fail" is a much better number
than it looks.
