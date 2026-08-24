---
title: "intent/strap-address-collision"
description: "two devices on one bus strap to the same address"
---

### Remedy

Re-strap one of the two devices to a free address, taking the address map from each part's datasheet rather than from the schematic.

### What it checks

Two declared strap groups on the **same bus** that encode the **same number**. Two devices answering
to one address.

### For hardware engineers

This is the check that most needs to be a rule rather than a reviewer.

Each strap is individually correct. Every resistor is the right value, on the right net, pulling the
right way. Nothing on either page of the schematic looks wrong, because the fault is not on a page: it
is the relationship between two parts that are drawn separately and reviewed separately.

On the bench it does not present as a strap problem either. Two devices drive the bus whenever either
is addressed, so it reads as noise, marginal timing, or an intermittent peripheral. People chase
signal integrity for a long time before they suspect an address.

### Undecidable groups are excluded, deliberately

A group whose encoded value could not be read is left out of this check entirely, rather than decoded
with its missing bits assumed.

That is the load-bearing guard here. An invented address can invent a collision, and a confident
report that two innocent parts clash is worse than saying nothing: it sends someone to re-spin straps
that were correct. Those groups are already reported `inconclusive` by their own `strap-group` rule,
so the gap is visible rather than silent.

The fix, when it happens, is the same one that fixes the inconclusive: declare the group's `default`
level so the value can be read.

### When this rule exists at all

It is compiled only when at least two declared groups share a bus. Below that a collision is not
expressible, and a rule that could never fail would let a review item bound to it read a pass it did
not earn.

### Fixing a finding

One of the two devices needs a different address. Check both parts' strap tables before moving a
resistor: the addresses each part can take are usually constrained, and sometimes only one of the two
has any freedom.
