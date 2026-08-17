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
termination". The class is assignment, not measurement: a net in `HighSpeed` is one someone declared high-speed,
not one the engine verified is routed that way.

A net can be in several classes at once, and the engine reports all of them. If you assigned `VBUS`
to both `Power` and `HighCurrent`, it shows up under both, and a query scoped to either one finds it.
That matters when you are checking coverage: a net missing from a scope you expected it in is a real
finding, and it used to be possible for the engine to lose the membership rather than report it.

### For software engineers

A filtered projection over `Nets()`, 1:1 with classed nets and absent for the rest. It joins to
everything else keyed by net name (`component-on-net`, `pin.net`, `net.max_voltage`), so it composes as
a scope filter on any existing question.

**`?net` is NOT unique in this projection.** Membership is a set, so a net in two classes emits two
rows and a join on `?net` fans out, the same 1:many shape `component.class` has. Count rows and you
are counting memberships, not nets; `net.netclass(?n, ?c) => ?n` on a design where half the nets carry
two classes returns more results than the design has classed nets.

WS1-050 settled this against the formats rather than against our first reader. KiCad stores
`map<netname, set<netclass>>` and resolves a net's membership by unioning its explicit assignment with
EVERY matching pattern, then cascades the per-class VALUES by priority to build one effective class.
Altium likewise lets a net join several classes, though its clearance matrix treats that as an
ambiguity to flag rather than a feature. Same data model, opposite house reading, so the set
is carried as fact here and whether a second class is a *problem* is left to a rule.

Order is sorted, and that is a determinism guarantee only. It is NOT the tool's precedence order: in
KiCad precedence is a per-class `priority` that lives with the class DEFINITIONS (clearance, track
width, via), which nothing reads yet (WS3-111). Precedence decides whose track width wins, never who
is a member, so it cannot change the answer to a membership question.

The value is a foreign label, not a closed enum: it comes from the project file, so string comparisons
are exact and case-sensitive, and two projects can use different vocabularies for the same intent. Do
not treat an unrecognized class as an error, and do not derive meaning from the string beyond what the
project declares.

### Go projector

`netNetClassFacts` in `stdlib/relations/facts.go` walks `Model.Nets()` and emits a row per entry of
each net's `NetClasses`. The field is populated in the I/O layer, not by any analysis:
`readers/formats/registry.go` reads `net_settings.netclass_{assignments,patterns}` out of the sibling
`.kicad_pro` and calls `kicad.AnnotateNetClasses` (WS1-037). One row per (net, class) pair; zero rows
when the design has no classes, the common case, and the reason the companion marker below
exists.

Do not populate `ir.Net.net_classes` from IPC-2581. Its `LogicalNet/@netClass` is spelled the same but
means something else: a singular CLOSED enum (`CLK`/`FIXED`/`GROUND`/`SIGNAL`/`POWER`/`UNUSED`)
describing what a net IS. That is the derived-role space `net.ground` and `ir.Net.roles` occupy, not
user-named constraint groups, and mixing the two would put `GROUND` and `HighSpeed` in one relation
where a class-scoped query would silently select the wrong nets.

### Absence is not a pass

**Only a KiCad project read populates net class.** An EDIF netlist, an IPC-2581 board, a bare
`.kicad_sch` opened without its project, and a KiCad project that simply declares no classes all leave
this relation empty. A rule SCOPED by net class then selects nothing, finds nothing, and reports clean
and a review cannot tell that from a genuine pass. That is the false-pass family (WS3-090 / 096 / 097 /
098 / 099) reached by a new route: not an empty datasheet join and not a requirement that compiles to
nothing, but a **scoping** relation that is empty because the source carries no such data.

The route this relation takes is the capability gate. A netclass-scoped rule declares
`check.CapNetClass`, and `check.Available` reports it not-applicable, with the reason "design carries
no net-class assignments (only a KiCad project file supplies them)", wherever the design assigns no
classes. The gate is content-derived, not format-derived, because for a scoped rule "this project
declares no classes" and "this format has no classes" are the same answer: there is nothing in scope
either way. `has_netclass` is the queryable twin of that capability, so an ad-hoc query can ask whether
a class-scoped question is even answerable on this design before trusting its result.

### Datalog

Every classed net and its class:

```
net.netclass(?net, ?class) => ?net, ?class
```

Scope an existing question by the project's own class, the way a vendor rule deck does, over the
parts sitting on a high-speed net:

```
net.netclass(?net, "HighSpeed"), component-on-net(?ref, ?net) => ?ref, ?net
```

Ask the honest version, which returns nothing on a design with no classes rather than a clean-looking
empty result:

```
has_netclass(?_), net.netclass(?net, "HighSpeed"), component-on-net(?ref, ?net) => ?ref, ?net
```
