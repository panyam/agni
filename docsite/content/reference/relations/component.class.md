---
title: "component.class"
description: "a device class the part is in (a family tag too, e.g. a TVS is both tvs and diode)"
---

### What it is

`component.class(ref_des, class)` yields the device classes a part belongs to. It emits ONE ROW
PER TAG in the part's class set, not a single most-specific class, so a part with a family tag
answers more than once: a TVS diode answers both `component.class(D1, "tvs")` and
`component.class(D1, "diode")`, an LED answers both `led` and `diode`, and a ferrite bead answers
both `ferrite` and `inductor`. The `class` string is the canonical lowercase name (`resistor`,
`capacitor`, `crystal`, ...). An unclassified component produces no row (no class is guessed).

### For hardware engineers

This is the part's kind, decided once at ingestion from its designator and library type. The
family tags matter because a review often asks a family question, not a specific one: "every
diode-family part on this signal" should catch the plain diodes, the LEDs, and the TVS clamps,
because electrically they are all diodes. Querying the family tag gives you that set without
having to enumerate every subtype.

### For software engineers

Think of the class set as an interface hierarchy flattened onto each node: the part carries both
its concrete type and every base type it satisfies. That is why the relation is 1:many with a
component. Joining on the specific tag (`component.class(?r, "tvs")`) is `instanceof TVS`;
joining on a family tag (`component.class(?r, "diode")`) is `instanceof Diode` and matches every
subtype. The set is stamped at the read edge, so the relation is a projection over a precomputed
field, not a re-derivation per query. Empty rows for a component mean the classifier had no
evidence, distinct from "classified as none".

### Go projector

`componentClassFacts` in `check/facts.go` walks `Model.Components()` and, for each, emits one row
per class in `Model.Classes(ref)` (the full `device_classes` set: the most-specific class plus
its family tags). `Model.ComponentClass(ref)` returns just the most-specific one; the relation
uses the full set on purpose so family joins work. One row per tag; empty for an unclassified
component, and empty overall for a design the classifier could not tag.

### Datalog

Every component and each class tag it carries:

```
component.class(?r, ?c) => ?c
```

Match the whole diode family by its family tag (catches plain diodes, LEDs, and TVS clamps) and
report which nets they sit on:

```
component.class(?r, "diode"), component-on-net(?r, ?n) => ?n
```

### Schematic

![A TVS and an LED each carry the diode family tag; a ferrite carries the inductor family tag; each emits one component.class row per tag]({{.Site.PathPrefix}}/static/images/catalog/relations/component.class.svg)
