---
title: "symbol-unresolved"
description: "A symbol reference did not resolve, so its placements carry no pins."
---

### Remedy

Re-read with the library that holds the symbol on the symbol search path. Until it resolves, the part has no pins, so every connectivity result over it rests on an incomplete read.

### What it means

The reader could not open or parse a symbol reference, such as an xschem or gEDA `.sym` file or a
KiCad external `.kicad_sym` library named by a `lib_id` that the schematic does not embed. The
schematic itself parsed fine. Only the file holding the part's pin definitions is missing.

### Why engineers want it

A symbol carries a part's terminals. When it fails to load, the placement keeps its reference
designator and gains no pins, and a part with no pins has no connections. So the netlist is missing
every connection those parts make. Nothing about the result looks wrong, since the design reads as
valid and simply emptier than it is.

A missing library produces a smaller netlist rather than an error.
Connectivity rules then run over the incomplete read and report a clean pass. That pass reads the
same as one on a design where those connections were never drawn.

### Impact

Every connection made by the affected parts is absent from the netlist. The reader makes this worse
by design. Whenever any symbol fails to resolve, it suppresses dangling-endpoint findings, because a
missing pin turns a real wire end into a phantom dangle. Without this rule, the only visible effect
of a lost symbol is that the design reports LESS than it would have.

![a resolved symbol contributes its pins and its component joins the net; an unresolved one leaves the component pinless and off the net, so the net reads as having fewer members than the drawing shows]({{.Site.PathPrefix}}/static/images/catalog/rules/symbol-unresolved.svg)

### What it does to other rules

If any symbol is unresolved, every rule that reads a pin or connectivity fact reports
`inconclusive` for the whole design, rather than passing over an incomplete netlist. The gate is
DESIGN-WIDE. That is deliberately the conservative reading. It matches what the readers already do
with dangling endpoints, where one unresolved placement suppresses the whole design's dangles.

A gate on only the affected parts would need a mapping from a finding back to the ref-des it
concerns. That mapping does not exist yet. Until it does, a narrower gate would amount to asserting
that the parts not flagged are unaffected, and the reader cannot support that claim.

The outcome is `inconclusive` rather than `not-applicable`. A missing source-format capability is
permanent. An unresolved symbol is a property of THIS read, and a `--symbol-path` can fix it.

### The one subtlety

The severity is warning, though the effect on the netlist is as corrupting as a ref-des collision,
which is an error. The difference is whose fault it is. A collision is an annotation slip in the
design. An unresolved symbol is almost always a defect in the invocation: no `--symbol-path`, a
library not checked out, a file not mounted. The board itself may be flawless, and every FAIL is
supposed to be a defect in the design.

A read without symbols at all reports nothing. That read is the plain `Read` entry with no opener.
It means a caller deliberately asked for names only, and it is not a resolution failure.
