---
title: "wire-no-junction"
description: "A wire endpoint lands mid-span on another wire with no junction dot."
---

### What it means

A wire's endpoint lies strictly inside another wire segment's body, and no
junction dot (or label) sits at the touch point. KiCad connects wires only where a dot is
placed, so the two wires are electrically separate despite touching on screen.

### Why engineers want it

This is the sibling of dangling-endpoint with the opposite
disguise: a dangling end is visibly incomplete if you look closely, but an undotted T-tap
looks EXACTLY like a connected one. The mistake survives visual review by construction and
surfaces at bring-up as a missing connection between two otherwise-healthy nets.

### Impact

Net A and net B behave normally in isolation, and every net-level rule sees two
well-formed nets, so nothing else can catch it. The drawing is the only witness, and the
drawing lies.

![undotted T-tap flagged as two nets vs a dotted T that is one net]({{.Site.PathPrefix}}/static/images/catalog/rules/wire-no-junction.svg)

### Scope

Endpoint-on-body only. Two wires that merely CROSS (no endpoint at the meet)
never flag: KiCad does not connect crossings regardless, so the drawing tells no lie. A
label at the touch point joins the wires by name (both attach to it) and the reader has
already split the segment there, so labeled taps do not flag either. KiCad-family
semantics: formats whose tools auto-connect T-touches (gEDA) never emit the diagnostic.

### Query structure

the reader computes endpoint-on-body from wire geometry after
splitting at junctions and labels; the rule reports them.

    select E in no_junction_endpoints

Reads: wire.endpoint, wire.junction. Tier P.

### For software readers

KiCad stores no connection list; connectivity is GEOMETRIC, so things connect where their
coordinates coincide, the way whitespace-significant syntax gives meaning to layout. Two
touching wires join only at endpoints or where the author placed a junction dot (the
explicit "yes, these join" marker). A wire ENDING on the MIDDLE of another wire, with no
dot, is a file that LOOKS imported because it sits in the right directory but was never
added to the build: every human reads it as connected, the toolchain disagrees, and both
halves are individually healthy so nothing else can reveal the gap.

![t-tap cases]({{.Site.PathPrefix}}/static/images/catalog/rules/tjunc-cases.png)
