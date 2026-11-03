## net.declared_track_width

### What it is

`net.declared_track_width(net, mm)` yields the track width a net SHOULD be routed at, according to
the project's own net-class declarations. The value is held in metres and shown in mm. There is one
row for each net the project actually constrained. It is the resolved answer, computed after the
cascade across the net's classes.

Pair it with `board.track_width(net, mm)`, the ACTUAL routed value in the same units. Declared
versus actual, with no number the engine invented.

### For hardware engineers

A net can sit in several classes at once, and the classes can disagree. KiCad does not pick one class
and use it wholesale. It fills each constraint from the highest-priority class that states that
particular constraint, and the Default class fills whatever is left over. So a net can take its
clearance from one class and its track width from another, exactly as the tool does when it routes.

This relation runs that same resolution, so your layout tool would enforce the number here. Ask
it when you want to know what a net was supposed to be, not what it is.

### For software engineers

**Join THIS, not `netclass.track_width`.** Membership is 1:many (WS1-050). Joining
`net.netclass(?net, ?class)` to the per-class relation fans out over every class the net belongs to,
and a comparison then fires against classes that lost the cascade. Imagine a net routed correctly to
its winning class's value. That join reports it as a confident, wrong finding.

Rows are 1:1 with constrained nets. If none of a net's classes states this quantity, the net yields
no row. A rule joining it then selects only nets the project genuinely constrained, and `not
net.declared_track_width(?n, ?_)` reads as unconstrained.

### Go projector

`netDeclaredFacts` in `stdlib/relations/facts.go` projects it. For each net it orders the net's
classes by the project's `priority`, ascending, with unstated and unknown classes last. It then takes
the first class that states the field, independently per field. The citation records WHICH class
supplied the value (`net_settings:<class>`), so a finding can say where the limit came from.

### Absence is not a pass

Only a KiCad project read populates this, and only when the project declares class definitions.
Take a rule comparing declared against actual. On every other source it finds nothing to compare
and reports clean, and a review cannot tell that from a genuine pass. Such a rule gates on
`design.has_netclass_defs`, and on the board tier for the actual side, so an absent declaration
reads not-applicable.

### Datalog

What every constrained net should route at:

```
net.declared_track_width(?net, ?mm) => ?net, ?mm
```

Nets routed narrower than their own project demands:

```
net.declared_track_width(?net, ?declared), board.track_width(?net, ?actual), ?actual < ?declared => ?net, ?actual, ?declared
```
