---
title: "intent/protection-ovp"
description: "A rail the design intent declares needs OV protection has no TVS/zener clamp."
---

### Remedy

Fit the over-voltage clamp the intent declares for this rail, chosen to conduct below the lowest absolute maximum the rail feeds.

### What it means

The design intent declares that a named rail must be over-voltage protected. This rule probes that
exact net for a TVS or a zener among its components. It fails when the declared rail carries no such
clamp. It keys on the declared net NAME, not on the rail-role heuristic. That heuristic misses names
like `VBATT01` that carry no voltage token. Keying on the name makes an input rail checkable once the
declaration names it explicitly.

### Why engineers want it

An input that can see a transient (a battery rail, an off-board connector) is supposed to have a clamp
that shunts the spike to ground before it reaches downstream silicon. The netlist does not state
whether a given rail was intended to be protected. That is an architecture decision, so the intent
declaration names the rail and this rule verifies the clamp is actually on it.

### Impact

The design was intended to protect a rail, and that rail has no OV clamp. So a transient reaches
downstream parts and causes latch-up, degraded silicon, or an immediate failure on a hot-plug or
load-dump event.

![A declared input rail with no clamp is flagged; the same rail with a TVS to ground is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/protection-ovp.svg)

### Scope note

`ovp` is realized by a TVS or a zener on the rail (`component.class` tvs/zener). It is one of the
per-kind protection rules (`intent/protection-ovp`, `intent/protection-discharge`), each emitted only
when the declaration carries that kind, so distinct review items bind independently. The rule iterates
the declared protections and probes the design; it never derives the protected-rail set from the
netlist.
