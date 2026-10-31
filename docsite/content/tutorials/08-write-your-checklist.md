---
title: "8. Write your checklist"
description: "The questions your team asks of every board, bound to the engine so they answer themselves."
---

Most teams already have a review checklist. It is usually a spreadsheet, and going through it is
usually somebody's afternoon. Much of it is mechanical, asking questions whose definite answer is
already sitting in the design file.

The `review` checklist in the Jetson project's `project.yaml` is that checklist, written so the mechanical
items answer themselves and the rest stay visible as work for a human.

`agni check` answers "what is wrong with this board".
`agni review` answers "which of our questions did we actually answer", which is a different question
and a more useful one when you are deciding whether a board is ready.

## The shape

```yaml
checklists:
  review:
    name: Jetson baseboard design review
    areas:
      - name: Power
        items:
          - id: "P1"
            title: every rail carries a bulk capacitor
            description: A rail with no bulk capacitance browns out on a load step.
            rule: bulk-cap
```

`checklists` maps a name to a checklist, so a project can carry several. The first one written is the
project's default.

Areas group items the way your existing checklist groups them. Each item keeps its own `id`, so an
item that has been "P1" in your process for years stays P1 here and nobody has to relearn numbering.

## Four ways to bind an item

**`rule:`** hands the item to a catalog rule. It can be any rule from any tier, whether
built in, added by your conventions section, contributed by a profile, or declared as intent.

```yaml
- {id: "P2", title: every rail carries decoupling, rule: decoupling-present}
- {id: "H1", title: active-low signals use the overbar, rule: jetson/active-low-overbar}
- {id: "A1", title: each rail sits at its declared voltage, rule: intent/voltage-domain-mismatch}
```

**`profile:`** hands it to an interface profile, so the item covers everything that profile
requires, and reads as unevaluated rather than passed when the interface is absent.

```yaml
- {id: "I1", title: every PCIe link is complete, profile: PCIe}
- {id: "I2", title: the CAN interface is complete and terminated, profile: CAN}
```

**`query:`** is for a question no shipped rule asks. You write the datalog inline.

```yaml
- id: "H2"
  title: every test point sits on a net somebody named
  query:
    match: 'component.class(?r, "test_point"), pin.net(?r, ?p, ?n), str.prefix(?n, "Net-(") => ?r, ?n'
    subject: r
    message: '{r} probes {n}, a net nobody named'
```

KiCad names a net nobody labelled after one of its pins, `Net-(J18-PadH01)`. A bring-up engineer
holding a probe and a schematic cannot find that net by name, so this team asks that every test point
land on a net someone named.

**`note:`** is for a question nothing automated can answer. The item stays on the checklist and
reports that no automated check answered it.

```yaml
- {id: "H3", title: the assembly drawing lists a torque spec for every standoff,
   note: manual review against the mechanical drawing package}
```

The temptation with an unautomatable item is to drop it,
and then it is not on the checklist at all. A `note:` keeps the question visible and says who owns
it.

## The query trap

Read the `H2` query again. It matches {{ explainable "test-point" "test points" }} on nets that
nobody named.

A query binding reports whatever it matches as findings, and findings mean the item failed. So the
query has to match the violation. Write the healthy case instead, phrasing it the natural way as
"every test point sits on a named net", and you get an item that fails on a good board and passes on a
bad one.

{{ includeFile "figures/query-binding-polarity.svg" }}

This authoring mistake is easy to make, and it is quiet, because on a healthy board an inverted
item just looks like a finding you have not got round to.

## Running it

Naming the design is the whole command. Every input a review needs is something this project
already declares: `project.yaml` carries its checklists and its naming vocabulary, `profiles/` its
interfaces, `params/` its part limits, and `designs/jetson/design.yaml` names the schematic to read,
the board beside it and the board's intent.

{{ agniRun "content/tutorials/runs/08-review.yaml" }}

Each item carries its outcome and the evidence behind it. An item bound to a rule that fired shows
what fired and on which net, so the checklist row and the debugging detail are the same artifact. Every
tier you added appears in it. P4 passes on the transcribed NTS0102, I1 on the project's PCIe profile,
H1 fails on the naming rule, and A1 and A2 pass on the declared intent. I2 reads `not-applicable`,
because this board carries no CAN transceiver and an absent interface is not a passed one.

It says which checklist it picked, on stderr:

{{ agniRun "content/tutorials/runs/08-review-note.yaml" }}

Which checklist scored a run is not recoverable from the outcomes it produced, so a checklist you
did not type has to announce itself. `--checklist <name>` runs another one the project declares, and a
value ending in `.yaml` is read as a manifest file instead, for a checklist that lives outside the
project.

## A page you can walk

The markdown above is for reading in a terminal and pasting into a review. `--format html` writes the
same checklist as a self-contained page instead:

```
agni review designs/jetson --format html --server self > review.html
```

The page differs from the markdown in two ways. Every finding is listed per item, where the Detail
cell above caps at three, so a rule that fired on thirty-six rails shows thirty-six. And each finding
links to its proof, so clicking the net opens the board with the checks run and that subject in focus.

`--server self` starts the viewer over this run's own mount table and blocks until Ctrl-C, so the
links cannot point somewhere the report was not read from. It needs the viewer's assets the way
`agni open` does in [rung 3](../03-see-it/). Naming a server someone else is running works too, and
then the link half needs the same two things `check --server <url>` needs, a viewer address and a
mount you DECLARED, and it prints the reason when it withholds one.

## Items about the board, not the netlist

Some questions are about copper, and a schematic has none:

```yaml
- {id: "B1", title: no track is below the board's minimum width, rule: track-width}
```

In the run above it resolves, because `design.yaml` declares the board as a companion of the schematic
and the run reads both. To see what the item does with no copper, read the schematic on its own with
`--as-named`, the flag that says "exactly the file I named, not the design it belongs to":

{{ agniRun "content/tutorials/runs/08-review-as-named.yaml" }}

Nothing about the item changed. Only its input changed. `--board-path` does the same job for a board
that is *not* a declared companion: a fab's returned file, or a layout under review that has not
landed in the design yet.

The `n/a` is not hiding a failure and it is not standing in for a pass. The question had nothing to
evaluate, and the only way to find out which it would have been was to supply the copper. An item
that scored `pass` on the schematic alone would have been wrong, and nobody would ever have checked it
again.

## Growing it

Start with the items your team already argues about, not with the ones that are easiest to automate.
An item bound to nothing is still worth adding as a `note:`, because the checklist is then complete
and the automation gap is measurable rather than invisible.

For a checklist of this size, hand-authoring the YAML is right. Teams running a few hundred items
usually move to one file per item with the manifest generated from them, so items can be edited in
parallel without conflicting.

## Next

[Read the verdicts](../09-read-the-verdicts/), which is about why "7 fail" is a much better number
than it looks.
