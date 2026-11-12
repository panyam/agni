---
title: "intent/io-map-net-absent"
description: "The design intent's IO map declares a net the netlist does not have."
---

### Remedy

Add the missing net, or amend the IO map. Check for a misspelling first: a net present under a slightly different name is a different fix from one that was never drawn.

### What it checks

The IO map declares a net and the netlist does not have it.

The rule gives one verdict per declared NET rather than per row. It is ordinary for a map to name
one net on several rows. Repeating an absence per row would inflate the count, and a reader uses
that count to judge how bad the disagreement is.

### For hardware engineers

This and an incomplete map are opposite defects, and they get confused constantly:

- **A net the map declares and the netlist does not have** is usually a real disconnection. Something
  was meant to be drawn and was not.
- **A net the netlist has and the map does not declare** is usually an incomplete map. The design is
  fine and the document is behind.

This rule reports the first. The second is a coverage question, and `intent/io-map-coverage`
reports it. There the interesting number is how much of the netlist the map covers at all, rather
than a failure count. Imagine a design with sixteen hundred nets and a map declaring two hundred.
Two hundred nets are checked and fourteen hundred are UNEXAMINED, and that is not the same as
clean.

### A misspelling is not a disconnection

When no net carries the declared name but one differs from it only by spelling, the verdict is
`inconclusive` and names the net it found, rather than failing.

Imagine a net that is present, under a name that differs by one invisible character pasted out of a
spreadsheet. Sending a reviewer to look for it as missing costs more than the finding is worth. We
measured a shipped checker of this kind against a large production board. Every warning it produced
was a comparison artifact of exactly this family.

The shared identifier comparison forgives case, invisible characters, and zero-padded and bracketed
indices, and spelling here means those differences. It does not mean a fuzzy match, since dropping a
token from a net name names a different net.

### Declaring it

Declare the same `io_map` section that `intent/io-map-pin-mismatch` reads. This rule uses the `net`
field alone.

### Fixing a finding

Check for a misspelling before drawing anything. If a net is present under a slightly different
name, the fix is one character in whichever document is wrong. If a net was never drawn, the fix is
a schematic edit. The finding tells you which, because a suspected misspelling reports as
inconclusive and names its candidate.
