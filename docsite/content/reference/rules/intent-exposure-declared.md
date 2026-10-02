---
title: "intent/exposure-declared"
description: "A component the design intent gives an exposure is absent from the design or is not a connector."
---

### Remedy

Correct the ref-des in the declaration, or remove the entry. A declaration naming no connector on the board leaves every connector checked as external, which is the opposite of what it was written to say.

### What it means

A design's intent can declare a connector `internal`, which tells the exposure rules
(`esd-protection`, `esd-clamp-not-tvs`, `input-protection` and `reverse-blocking-absent`) that the
connector joins this board to another inside the same product and faces nothing a user touches. This
rule checks each such declaration against the design. It fails when the declared ref-des is not a
component of the design, or when the component is not a connector.

```yaml
intent:
  components:
    J1:  {exposure: internal}   # the module connector
    J18: {exposure: internal}   # the expansion mezzanine
```

### Why engineers want it

Exposure is a fact about one product rather than about the part. The same board-to-board connector
is the module socket on one carrier and the cable entry on another, and its part number and footprint
rarely say which. So the declaration has to come from the people who know the enclosure. A
declaration that names the wrong ref-des changes nothing at all, and without this rule it would read
as a declaration that took effect.

### Impact

A misspelled or stale declaration leaves the connector it meant to name checked as external. The
exposure rules then keep reporting it, or, if the ref-des now names a different connector, stop
reporting one that does face the field.

### Scope note

An undeclared connector is external, so a design with no `components:` section checks every connector
exactly as before. `external` can be written to state the default explicitly, and this rule checks it
the same way.
