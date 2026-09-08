## io-map-net-absent

### What it checks

The IO map declares a net and the netlist does not have it.

One verdict per declared NET rather than per row, since a map naming one net on several rows is
ordinary and repeating an absence per row would inflate the number a reader uses to judge how bad the
disagreement is.

### For hardware engineers

This and an incomplete map are opposite defects, and they get confused constantly:

- **A net the map declares and the netlist does not have** is usually a real disconnection. Something
  was meant to be drawn and was not.
- **A net the netlist has and the map does not declare** is usually an incomplete map. The design is
  fine and the document is behind.

This rule reports the first. The second is a coverage question, because the interesting number there
is not a failure count but how much of the netlist the map covers at all: a design with sixteen
hundred nets and a map declaring two hundred has two hundred checked and fourteen hundred
UNEXAMINED, which is not the same as clean.

### A misspelling is not a disconnection

When no net carries the declared name but one differs from it only by spelling, the verdict is
`inconclusive` and names the net it found, rather than failing.

That distinction is the difference between a tool people trust and one they switch off. Sending a
reviewer to look for a missing net that is present, under a name differing by an invisible character
pasted out of a spreadsheet, costs more than the finding is worth. We measured a shipped checker of
this kind against a large production board and every warning it produced was a comparison artifact of
exactly this family.

Spelling here means the shared identifier comparison: case, invisible characters, zero-padded and
bracketed indices. It does not mean a fuzzy match, since dropping a token from a net name names a
different net.

### Declaring it

The same `io_map` section `intent/io-map-pin-mismatch` reads. This rule uses the `net` field alone.

### Fixing a finding

Check for a misspelling before drawing anything. A net present under a slightly different name is a
one-character fix in whichever document is wrong; a net that was never drawn is a schematic edit.
Reading the finding tells you which, because a suspected misspelling reports as inconclusive and
names its candidate.
