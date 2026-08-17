---
title: "param.pin_relation"
description: "a datasheet constraint BETWEEN two pins of one part: bounds on (subject - reference) in the SI base unit, with the vendor's modality (required/recommended). The pin order is load-bearing, so swapping the two inverts the requirement (needs --params)"
---

### What it is

`param.pin_relation(mpn, subject_pin, reference_pin, modality, min, max)` is a datasheet constraint
that holds **between two pins of one part**, rather than about either pin on its own. `min` and `max`
bound the quantity `subject - reference`, in the SI base unit whatever the vendor printed, and an
absent bound on a side means unbounded there.

`modality` is the vendor's own modal verb reduced to a token: `required` for "shall never exceed",
`recommended` for "should be at least 1 V higher for best operation", and `unspecified` for a spec
that did not record one. Every row carries a citation back to the page and table.

**The pin order is the fact, not a presentation choice.** The bound is on subject *minus* reference,
so a query that swaps the two columns reads the opposite requirement. They occupy two distinct slots
for exactly that reason.

Only tracking relations are projected. `PinRelationKind` has one member today and the proto's own
comment says a second arrives only once a second vendor can populate it, so the kind is not spent on
a column; a second kind has to revisit this projection.

### For hardware engineers

A part with several supplies routinely constrains them against **each other**. A voltage translator
requires one side's supply to stay at or below the other's. A PHY requires one analog supply never to
exceed another by more than half a volt. A level shifter recommends an enable sit a volt above its
reference. Each terminal can be comfortably inside its own rating while the pair is still wrong, and
no per-pin relation can see that, because the fact is not about a pin.

The bound is a **value**, not a comparison operator. "Less than or equal to" is a maximum of zero,
"never exceed by more than 0.5 V" is a maximum of 0.5, and a symmetric tolerance is a minimum and a
maximum together. Most real instances carry a non-zero allowance, so an operator-shaped relation
could not hold them.

`modality` is worth filtering on. A requirement broken is a stress violation; a recommendation broken
is lost margin. They are different conversations and the relation keeps them apart.

### For software engineers

This borrows `param.pin_range`'s field layout for a different fact. There, Object and Value are a pin
and a symbol; here they are two pin ids, and `Min`/`Num` bound the difference between them rather
than one terminal's own quantity.

The pin ids join to `param.pin`, and that join turns them into the names a datasheet prints. Resolving
a *design* terminal onto one of these ids is `param.ResolvePin`'s job and deliberately not a datalog
join, because it can refuse (an ambiguous name, a name and number that disagree) and a join cannot.

A relation whose unit has no known scale keeps its pins, modality and citation with both bounds
absent, the posture the sibling projections take: an unmeasurable bound must not become orderable.

### Go projector

`paramPinRelationFacts` in `stdlib/relations/facts.go` shares the per-MPN join and dedup of the other
param projectors and emits through `specParamPinRelationRows`. `Object` and `Value` are the subject
and reference pin ids, `Qualifier` the modality token, `Min` and `Num` the reduced bounds, and `Cite`
the relation's datasheet provenance. Empty without `--params`, and empty for every spec that declares
no relations.

### Datalog

All queries need `--params`. Every pin-to-pin bound in the corpus, named the way the datasheet prints
the terminals:

```
param.pin_relation(?mpn, ?s, ?r, ?mod, ?min, ?max),
param.pin(?mpn, ?s, ?sname, ?sfn), param.pin(?mpn, ?r, ?rname, ?rfn)
  => ?mpn, ?sname, ?rname, ?mod, ?min, ?max
```

The requirements alone, all a CI gate cares about:

```
param.pin_relation(?mpn, ?s, ?r, "required", ?min, ?max) => ?mpn, ?s, ?r, ?min, ?max
```

The ordering constraints specifically, where one supply must not rise above another:

```
param.pin_relation(?mpn, ?s, ?r, ?mod, ?min, ?max), absent(?min), ?max <= 0
  => ?mpn, ?s, ?r, ?max
```
