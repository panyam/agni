---
title: "profile/missing-pullup"
description: "An interface signal that needs a pull-up reaches no rail."
---

### Remedy

Fit a pull-up from the signal to its rail, sized for the speed the bus runs at and the capacitance it carries.

### What it means

It reports a signal the profile marks as needing a pull-up (a chip-select, an open-drain line) whose
net reaches no power rail through a resistor, so no pull-up sits in its path.

### Why engineers want it

An open-drain or chip-select line with no pull-up floats between drives. At power-up, before any
driver takes it, it can sit at an undefined level and select or clock the device spuriously. The
pull-up is what holds it inactive.

### How it is checked

It runs the same bounded walk the built-in `i2c-pull-up` uses, called directly rather than
approximated, from the signal net out through resistors for up to three crossings, looking for a
rail. A rail is a legal DESTINATION and never a transit node, and ground is never crossed at all,
since a resistor to ground is a pull-down and counting it would pass exactly the line this
requirement exists to catch.

**The finding and the pass both carry the path.** A pass says "SPI_CS reaches rail +3V3 through R1"
and hands the viewer the resistor and the rail as clickable entities; a failure says no rail is
reachable within the hop limit, and names the limit, because "no pull-up" and "a pull-up four hops
away" are different situations a bare message cannot tell apart.

This requirement is the one that compiles to Go rather than to a query, and the reason is visible in
that paragraph. Datalog could state a reachability question and could not name what the walk crossed,
so the profile route proved a pass with the net's own name while the built-in proved one with the
route (agni issue 516). Calling the same function makes the two reports identical rather than
merely similar.

### For software readers

A pull-up works like a default value, and without it the line has no defined idle state, the
hardware analogue of reading an uninitialised variable. The rule checks the default is wired in.
