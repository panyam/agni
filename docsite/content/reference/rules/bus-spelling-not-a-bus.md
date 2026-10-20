---
title: "bus-spelling-not-a-bus"
description: "A label is spelled like a bus, and this format reads it as one plain net."
---

### Remedy

Respell the label in the format's own bus syntax (in KiCad, `DATA[0..7]` rather than `DATA[7:0]`) if it names a bus, or rename it without the range if it names one signal.

### What it means

A label whose name carries an index range, which is how a schematic names a bus, in a spelling
the source format reads as one ordinary net instead. KiCad writes a bus as `DATA[0..7]` and only
that. A KiCad label written `DATA[7:0]`, the spelling xschem and gEDA use, is a plain scalar net
whose name happens to contain brackets and a colon.

### Why engineers want it

Whoever typed `DATA[7:0]` meant eight signals. KiCad makes one net of it, so wherever that label
joins wires, the signals the author meant to keep apart are joined instead, and the drawing looks
the same either way. Nothing else in the read can reveal it, because a single net with a strange
name is a perfectly valid design.

### Impact

Signals the author believed were separate share a net, or member taps that were meant to land on a
bus land on nothing that carries them. Warning severity, because it is a defect in the design as
the tool reads it, not a gap in the read.

### Scope

Only the KiCad reader records how it reads a range-spelled label, so the rule reports
not-applicable on other formats. xschem and gEDA read the colon spelling as a bus, so the same label
there is correct.

A label in KiCad's own bus syntax is a pass, so a sheet whose every bus is spelled correctly says
so. Whether that bus's members resolve into nets is `bus-not-modeled`'s question, not this one. A
name used on several sheets is one verdict, since respelling it is one change.

### Query structure

decide each range-spelled label by whether its format reads the spelling as a bus.

    select L in range_labels where not reads_as_bus(L)

Reads: bus.construct (a reader diagnostic). Tier P.

### For software readers

The two spellings are two dialects of one idea, the way `a[1:3]` slices a list in Python and is a
syntax error in C. A KiCad schematic using the other dialect is still a valid file, so nothing fails
to parse. The bracket text simply becomes part of a name, much as a string that looks like code is
still a string.
