## has_netclass

### What it is

`has_netclass(present)` yields exactly one row, with the value `true`, when the design assigns net
classes at all, and zero rows otherwise. Like `has_nc_channel` and `types_power_out` it is a
design-level flag rather than a per-entity relation: there is never more than one row, and its presence
or absence is the whole signal. A KiCad project whose `net_settings` declares classes makes the row
appear; an EDIF netlist, an IPC-2581 board, a bare `.kicad_sch` read without its project, and a project
that declares no classes all produce no row.

### For hardware engineers

Net classes only exist if someone set them up in the project. This flag says whether that happened. It
matters because a review question scoped to a class ("every HV net must clear 2mm") has nothing to
examine on a design with no classes, and "nothing to examine" is not the same answer as "everything
passed". The flag is what lets the report say the check did not apply instead of quietly counting a
pass you did not earn.

### For software engineers

A capability probe over the whole design, closer to a feature flag than a row set. Because a rule reads
it as `has_netclass(?_)`, an absent row makes the enclosing conjunction yield nothing, so a guarded
query fails closed rather than returning a confident empty result.

The distinction it draws is between two empty results that look identical in the tuples: "no net is in
class HV" and "this design has no classes". The first is a real answer, the second is an unanswerable
question, and only the marker separates them.

### Go projector

`hasNetClassFacts` in `stdlib/relations/facts.go` calls `Model.HasNetClasses()`, which is true when any
net carries a non-empty `NetClass`. When true the projector returns a single row (subject `true`); when
false it returns nil, so the relation is one row or none, never more. The underlying flag is collected
once in the model's nets walk, alongside the no-connect channel, so the read is O(1).

It is the queryable twin of `check.CapNetClass`: a Go or Spec rule declares that capability and
`check.Available` gates it to not-applicable, while a datalog query reads this relation for the same
signal. The Spec-rule fact `design.has_netclass` is the third face of it. Deliberately content-derived
rather than format-derived, unlike `types_power_out`: for a class-scoped rule, a KiCad project that
declares no classes is in exactly the same position as an EDIF netlist that cannot declare any.

### Datalog

Probe the flag directly (one row `true`, or empty):

```
has_netclass(?present) => ?present
```

Guard a class-scoped question with it, so the query returns nothing on a design that cannot answer it
rather than an empty result that reads like a clean bill of health:

```
has_netclass(?_), net.netclass(?net, "HV"), component-on-net(?ref, ?net) => ?ref, ?net
```
