---
title: "profile/missing-pullup"
description: "An interface signal that needs a pull-up reaches no rail."
---

### What it means

A signal the profile marks as needing a pull-up (a chip-select, an open-drain line) whose net
reaches no power/ground rail, so no pull-up resistor sits in its path.

### Why engineers want it

An open-drain or chip-select line with no pull-up floats between drives. At power-up, before any
driver takes it, it can sit at an undefined level and select or clock the device spuriously. The
pull-up is what holds it inactive.

### How it is checked

`reaches(?net, ?rail), rail(?rail)` means the net reaches a rail by crossing a series pass element (the
pull-up resistor). If no such rail is reachable, the line is unpulled. Uses the merged reach walk
and the `rail` relation, so no geometry is needed.

### For software readers

A pull-up is a default value: absent it, the line has no defined idle state, the hardware analogue
of reading an uninitialised variable. The rule checks the default is wired in.
