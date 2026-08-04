---
title: "pin_net_conflict"
description: "a pin the read placed on more than one net; one row per net (reader integrity diagnostic)"
---

### What it is

`pin_net_conflict(ref_des, pin, net)` yields one row per net that a single `(ref_des, pin)` was
placed on when the read put that pin on more than one net. Because a pin belongs to exactly one
net by definition (a net is the equivalence class of joined pins), any pin that appears in two
nets is malformed input — a reader bug or a corrupt export, not a design a person drew. The
multi-row shape (one row per claiming net) lets a query name every net the conflicted pin touches
and join to those nets.

This is the query-relation face of the `pin-net-conflict` integrity rule: the rule fires a
finding, the relation lets you interrogate the same condition ad hoc.

### For hardware engineers

You should almost never see this. A pin is one physical node; it cannot legitimately be part of
two separate nets at once. When it shows up, the tool's *read* of the file disagreed with itself
— its first corpus runs surfaced two reader gaps (unannotated placeholder refs merged, WS1-024;
duplicate port designators collapsed, WS1-025). Treat a row as "fix the read," not "fix the
board."

### For software engineers

A net is an **equivalence class** over pins, so membership should be a function: each pin maps to
one net. This relation reports the keys where that function became one-to-many — the invariant
break behind every per-pin answer downstream (`pin.net`, diff keys, viewer highlights). Rows are
1:many with the offending pin (one per claiming net); an empty result means the read is clean and
every pin resolved to a single net.

### Go projector

`pinNetConflictFacts` in `check/facts.go` iterates `Model.PinNetConflicts()` and, for each
conflicted pin, emits one row per net in its `Nets` list. Empty when the read is clean. A pin of a
collided ref-des is *not* reported here — that root cause belongs to the `duplicate-ref-des`
finding, so one authoring slip yields one finding, not two.

### Datalog

Every conflicted pin and the nets claiming it:

```
pin_net_conflict(?r, ?p, ?n) => ?r
```

Join to the components on each claiming net (what else the malformed read tangled together):

```
pin_net_conflict(?r, ?p, ?n), component-on-net(?other, ?n) => ?other
```

### Schematic

![One pin claimed by two nets is a conflict; the same pin on one net is clean]({{.Site.PathPrefix}}/static/images/catalog/relations/pin_net_conflict.svg)
