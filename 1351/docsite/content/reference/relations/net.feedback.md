---
title: "net.feedback"
description: "the net is a regulator feedback / sense node (must not be probed)"
---

### What it is

`net.feedback(net)` yields one row per net whose name reads as a regulator feedback or sense node: a
leaf name ending in `_FB`, `_VFB`, `_FEEDBACK`, `_VSENSE`, `_SENSE`, `_SNS`, or a bare `FB` / `VFB`.
It is name-derived. The test-point rule applies a feedback exclusion, and this relation is the
datalog face of that exclusion.

### For hardware engineers

A feedback net is the divider tap that feeds a switching or linear regulator's control loop. It is a
high-impedance sense node, so anything you hang on it changes the voltage that the regulator reads
and shifts regulation. A scope probe or a test point on a feedback node is a review finding, because
the probe's capacitance and loading disturb the loop it is measuring. During a review you query
`net.feedback` to list the sense nodes. `net.rail` already leaves a feedback net out unless the net
is marked global or power-driven (agni 679), so subtracting `net.feedback` from `net.rail` only
matters for those.

### For software engineers

Think of a feedback net as a node that you may read but must not tap. Observing it changes its
value, so it is off-limits to the instrumentation that a normal rail allows. `net.feedback` is a
filtered projection over `Nets()` with the naming predicate, so rows are 1:1 with feedback-named
nets, and an empty result means no net name matched the sense-node lexicon.

### Go projector

`feedbackFacts` in `stdlib/relations/facts.go` walks `Model.Nets()` and emits a row for each net
that carries the feedback role (`check.NetHasRole`). A net with a stamped role set answers from it.
A net without one falls back to `Model.IsFeedbackName`. That delegates to the active naming
lexicon's `IsFeedback` (`core/classify/rolenames.go`), which matches the `_FB` / sense-suffix
patterns on the hierarchy leaf, case-insensitive. One row per feedback-named net; empty when no net
matches.

### Datalog

List every feedback / sense node:

```
net.feedback(?n) => ?n
```

Every supply rail that is not a sense node. The answer differs from `net.rail` alone only for a
global or power-driven feedback net:

```
net.rail(?n), not net.feedback(?n) => ?n
```

### Schematic

![A divider tap into a regulator FB pin is a sense node a probe would disturb; a plain output rail is probe-safe]({{.Site.PathPrefix}}/static/images/catalog/relations/net.feedback.svg)
