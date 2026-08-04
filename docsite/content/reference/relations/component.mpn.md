---
title: "component.mpn"
description: "the design-side part identity (manufacturer part number)"
---

### What it is

`component.mpn(ref_des, mpn)` yields one row per component that resolves to a manufacturer
part number: the design-side part identity. The `mpn` value is the exact orderable part
("BSS138", "LM1117MPX-3.3"), not a class or a generic value. A component with no resolved
part number produces no row.

### For hardware engineers

This is the BOM answer for a reference designator: which physical part R7 or U3 will be built
as. Two schematics can be electrically identical and place different parts; only the part
number distinguishes them, and it is what the datasheet checks join against. You query it to
list what a design actually orders, or to find the components a datasheet-backed rule can reach
(a component with no part number has no datasheet to check against).

### For software engineers

The part number is the lockfile entry (see ANALOGY.md): `component-on-net` and `pin` describe
the graph structure, `component.mpn` binds a node to a concrete pinned artifact
(`lodash@4.17.21`). Rows are 1:1 with components that carry a resolved part number, so it is a
partial projection over `Components()` (unresolved components are simply absent). It is the
design half of the datasheet join key; the datasheet half is `param(mpn, symbol, max)`, keyed
by the same string.

### Go projector

`componentMPNFacts` in `check/facts.go` walks `Model.Components()` and emits a row for each
component where `Model.ComponentMPN(ref)` is non-empty. `ComponentMPN` returns the `BomLine`
part number when a BOM covers the ref-des, else the component's `MPN`/`mpn` attribute, else "".
It never parses the free-text Value field, so the identity is only ever what the design
declared. Each reader normalizes its own vendor part-number property into the canonical `MPN`
attribute (OrCAD/Allegro carry it under `Manufacturer_PN`, which the EDIF reader maps to `MPN`),
so this one relation reads uniformly across formats.

One row per component with a resolved part number. Empty result: the model was built without a
params tier. The MPN index is populated only by `NewModelWithParams`, so the relation is empty
unless `agni` was run with `--params` (an empty params directory is enough to build the index).

### Datalog

Every component and its part number:

```
component.mpn(?r, ?m) => ?r
```

Join to the datasheet parameters seeded for that part (the components a datasheet rule can
reach):

```
component.mpn(?r, ?m), param(?m, ?sym, ?max) => ?r
```
