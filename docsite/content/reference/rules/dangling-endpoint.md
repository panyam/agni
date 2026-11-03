---
title: "dangling-endpoint"
description: "A wire endpoint terminates on nothing (no pin, junction, label, or other wire)."
---

### Remedy

Extend the wire to the pin, junction, or label it was meant to reach, or delete it. A wire that ends in space is never what the author intended.

### What it means

A schematic wire ends on empty space, with no pin, no junction dot, no label, and no
other wire endpoint at that point. The author drew a connection and stopped a
hair short, so the wire connects nothing.

### Why engineers want it

A .kicad_sch (and xschem/gEDA) stores connectivity as geometry, so two
things connect only when their points coincide. A wire dropped a grid step away from a pin looks
connected on screen but is not. This is the geometric sibling of single-pin-net. single-pin-net
catches a net wired to one pin, and this rule catches a wire wired to zero. That link never became
a net at all, so no net-level rule can see it.

### Impact

The author meant to make a connection, and it is missing. Nothing downstream reveals it,
because the wire produced no net. KiCad's ERC, like most, ships a dangling-end check for this reason.

![Wire ending on empty space is flagged; wire ending on a pin is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/dangling-endpoint.svg)

### Scope note

The rule reports an endpoint on nothing, and only that. wire-no-junction (WS1-012) reports the
disguised sibling, a wire end that touches another wire's body mid-span with no junction dot.

### Query structure

the reader computes the dangling endpoints from wire geometry; the rule
reports them.

    select E in dangling_endpoints

Reads: wire.endpoint, wire.junction. Tier P.
