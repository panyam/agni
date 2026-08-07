## net.netclass

### What it is

`net.netclass(net, class)` yields one row per net that the design tool put in a named class, with the
class string recorded verbatim: `Default`, `Power`, `HighSpeed`, whatever the project declares. A net
left in the tool's implicit default carries no class and produces no row, so `not net.netclass(?n, ?_)`
reads as "unclassed".

The name is deliberate. `net.netclass` is the **tool-assigned** label; it is not `net.class`, which
would belong to the **derived semantic role** space that `net.ground` and the `ir.Net.roles` stamp
(WS3-072) occupy. The two answer different questions. A net in the `Power` class is one an engineer
put there; a net that satisfies `net.ground` is one the naming lexicon read as ground. A rule that
conflated them would join against nothing and report clean.

### For hardware engineers

A net class is how you tell the layout tool that a group of nets shares rules: these are the high-speed
pairs, these are the high-voltage nets, these carry more current than the default track width allows.
It is the near-universal scope expression in vendor rule decks, and it is the label you already
maintain in the project rather than something the engine guesses from a name.

Query it to confirm the engine sees the classes you assigned, and to scope a review question the way
you would scope a design rule: "which parts sit on an HV net", "is any high-speed pair missing its
termination". The class is assignment, not measurement — a net in `HighSpeed` is one someone declared
high-speed, not one the engine verified is routed that way.

### For software engineers

A filtered projection over `Nets()`, 1:1 with classed nets and absent for the rest. It joins to
everything else keyed by net name (`component-on-net`, `pin.net`, `net.max_voltage`), so it composes as
a scope filter on any existing question.

`?net` is unique in the current projection: `ir.Net.net_class` is a singular field, so a net yields at
most one row and a join on it cannot fan out. Do not lean on that. Unlike `component.class`, which is
1:many by design, the arity here reflects what the IR happens to hold rather than a settled reading of
the format, and the KiCad reader resolves multiple matching patterns by taking the first. WS1-050
settles whether a second class is a legitimate tag or a conflict to flag; if it lands as a tag set
this relation becomes 1:many with no change to its name or shape.

The value is a foreign label, not a closed enum: it comes from the project file, so string comparisons
are exact and case-sensitive, and two projects can use different vocabularies for the same intent. Do
not treat an unrecognized class as an error, and do not derive meaning from the string beyond what the
project declares.

### Go projector

`netNetClassFacts` in `stdlib/relations/facts.go` walks `Model.Nets()` and emits a row for each net
whose `NetClass` is non-empty. The field is populated in the I/O layer, not by any analysis:
`readers/formats/registry.go` reads `net_settings.netclass_{assignments,patterns}` out of the sibling
`.kicad_pro` and calls `kicad.AnnotateNetClasses` (WS1-037). One row per classed net; zero rows when
the design has no classes, which is the common case and the reason for the companion marker below.

### Absence is not a pass

**Only a KiCad project read populates net class.** An EDIF netlist, an IPC-2581 board, a bare
`.kicad_sch` opened without its project, and a KiCad project that simply declares no classes all leave
this relation empty. A rule SCOPED by net class then selects nothing, finds nothing, and reports clean
— which a review cannot tell from a genuine pass. That is the false-pass family (WS3-090 / 096 / 097 /
098 / 099) reached by a new route: not an empty datasheet join and not a requirement that compiles to
nothing, but a **scoping** relation that is empty because the source carries no such data.

The route this relation takes is the capability gate. A netclass-scoped rule declares
`check.CapNetClass`, and `check.Available` reports it not-applicable — with the reason "design carries
no net-class assignments (only a KiCad project file supplies them)" — wherever the design assigns no
classes. The gate is content-derived, not format-derived, because for a scoped rule "this project
declares no classes" and "this format has no classes" are the same answer: there is nothing in scope
either way. `has_netclass` is the queryable twin of that capability, so an ad-hoc query can ask whether
a class-scoped question is even answerable on this design before trusting its result.

### Datalog

Every classed net and its class:

```
net.netclass(?net, ?class) => ?net, ?class
```

Scope an existing question by the project's own class, the way a vendor rule deck does — the parts
sitting on a high-speed net:

```
net.netclass(?net, "HighSpeed"), component-on-net(?ref, ?net) => ?ref, ?net
```

Ask the honest version, which returns nothing on a design with no classes rather than a clean-looking
empty result:

```
has_netclass(?_), net.netclass(?net, "HighSpeed"), component-on-net(?ref, ?net) => ?ref, ?net
```
