---
title: "12. Reconcile with the tools you already run"
description: "Bring your existing DRC or ERC report into the same model, and see where the two tools agree, differ, and cannot see each other's work."
---

You already run a design-rule check. Your CAD tool ships one, you have tuned it, and it gates your
release. The question to ask of anything new is "what does it add to what I already have,
and where do the two disagree".

You could answer it by reading two reports side by side, but that does not scale and nobody does it
twice. So bring
the other tool's report into the same model and ask directly.

## Import the other tool's report

Run your existing check and keep the JSON. The project keeps KiCad's own report for this board in
`reports/kicad-drc.json`, so you can follow along without KiCad installed. It came from:

```
kicad-cli pcb drc --format json -o kicad-drc.json designs/jetson/jetson-agx-thor-baseboard.kicad_pcb
```

KiCad 10.0.6 found 578 violations. Import them:

{{ agniRun "content/tutorials/runs/12-import.yaml" }}

Read the summary slowly. In it the import tells you how far to trust what follows.

**946 of 957 attached to an entity.** A vendor report names things in prose, so importing it means
resolving each name to an entity in your design. "Track [/CSI/CAM0.CSI2_CLK+] on In2.Cu", or a
curved one written "Track (arc) [...]", resolves to your design's net of that name. "NPTH pad of
J12" is an unplated hole with no pin or net, and it resolves to its part. The resolution works only
because agni names a KiCad schematic's nets exactly as KiCad does, including the sheet path on a bus
member drawn on a sub-sheet. There are more findings than violations because a clearance violation
names two items, and each becomes a finding.

**The rest are named, with a reason.** Ten are board outline geometry, which belongs to no component
or net. One names a footprint, `REF**`, that is on the board and in no schematic. Nothing is
dropped quietly. That matters, because a silently discarded finding is worse than one that was never
imported.

**The document carries no coverage axis.** This is the same idea as [rung
9](../09-read-the-verdicts/), applied to somebody else's tool. A vendor report is a flat list of
violations. It does not say which checks ran, so it cannot distinguish "checked and fine" from
"never checked". The import records that limitation rather than papering over it, and every reader
of the document inherits the caveat.

## Compare the two runs

Now produce your own run over the same design and compare them:

{{ agniRun "content/tutorials/runs/12-compare.yaml" }}

## Reading the split

The instinct is to compare 91 against 957 and conclude something about which tool is better. That
reading is wrong, and the three-way split is there to stop you making it.

**Theirs only** is mostly components. Nearly all of them are KiCad's `lib_footprint_issues`, where a
footprint differs from the library copy KiCad has installed, and courtyard and silkscreen checks add
to them. Agni has no
opinion about most of that and should not pretend to. Your existing DRC is not being replaced.

The nets in that column are KiCad's `clearance` findings, and the two tools genuinely disagree about
them. KiCad reports 357 clearance violations, and agni, checking the same copper against the board's
declared 0.1 mm minimum, reports none. The board also carries a `.kicad_dru` file of custom rules,
wider gaps for differential pairs and keep-outs between high-speed pairs and copper pours, and agni
does not read it. So KiCad is checking against rules unknown to agni, and the split tells you which
tool to believe for that question.

**Ours only** is nets, and each is a statement about what the circuit *means*: an I2C bus with no
{{ explainable "pull-up" }}, a rail with no {{ explainable "test-point" }}, a supply entering through a
connector with no reverse protection, a net named against house style. A board DRC structurally
cannot reach any of them. It is checking copper against fabrication limits, and it has no model in
which "this bus needs a pull-up" is expressible.

**Both** is two rails, and the line under the list says what that overlap is worth. Agni flags them
for reverse protection and test points, KiCad for clearance. That is the same entity under two
unrelated questions, so the tool calls it an observation about two documents rather than agreement.

{{ includeFile "figures/tool-overlap-axes.svg" }}

The useful answer to "what does this add" is a different axis, not a bigger number.

## About this board's copper

The numbers above are a real board's. It was laid out by people, passes its own manufacturer's
review, and still collects hundreds of DRC findings. Most of them are library bookkeeping, or rules
that were never given to the second tool. That is normal, so the comparison reads the columns rather
than the totals. The columns are complementary, because the two tools are answering different
questions, and no amount of careful layout gives a copper checker access to the fact that an I2C bus
has no pull-up.

## Using it as a gate

`agni import-results` writes an ordinary check-result document, so everything from [rung
11](../11-archive-and-gate/) applies: archive it, re-render it later, diff it against next month's
run. A vendor report that was a terminal scroll becomes an artifact in the same format as your own.

The comparison is the more useful gate. Suppose a finding appears in "theirs only" and is still
there months later. That is a check you are not doing. And if a finding moves from "both" into
"theirs only", one of your two tools stopped seeing something it used to.

## Next

[Drive it in the browser](../13-drive-it-in-the-browser/), the last rung.
