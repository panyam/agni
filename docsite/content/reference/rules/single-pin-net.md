---
title: "single-pin-net"
description: "A net connects to fewer than two pins (a floating stub), and is not an intentional no-connect."
---

### Remedy

Wire the stub to whatever it was meant to reach, or mark the pin no-connect if it is genuinely unused, so the intent is recorded rather than left to be guessed at.

### What it means

A net that connects to fewer than two pins. A signal wired to exactly one
thing, or to nothing, is almost always a mistake.

### Why engineers want it

Schematic capture is manual wiring. It is trivial to leave one end
of a wire floating, misspell a net label so two wires that should join do not, or delete a
part and orphan the stub that fed it. Every ERC tool ships this check because it is the single
most frequent capture slip.

### Impact

High frequency, near-zero false positives once no-connect is handled. A dangling
net is a signal that silently does nothing.

![single-pin-net: a one-pin stub is flagged, a two-pin net is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/single-pin-net.svg)

### The one subtlety

A pin deliberately marked "not connected" is a single-member net on
purpose, so the rule excludes intentional no-connects: a tool-generated "unconnected-..." net
name, an "nc_"/"no_connect" tag, or a connected pin whose electrical type is NO_CONNECT. This
no-connect awareness removes the demo's biggest false positive (WS3-002).

### Query structure

select the nets, exclude the intentional no-connects, report the rest.

    select N in nets where count(N.connections) < 2 and not intentional_nc(N)

Reads: net.names (the no-connect name markers), net.pin_count, pin.no_connect (via pin
electrical type). Tier P.
