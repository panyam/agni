---
title: "profile/signal-dangling"
description: "An interface signal net has fewer than two connections (a dangling stub)."
---

### Remedy

Connect the far end of the signal. The net exists, so presence checks pass, but only one end of it is actually wired.

### What it means

A profile signal net that exists by name but carries fewer than two connections. It is present in
the netlist, so a presence check passes, but wired to only one pin, so the far end of the bus is not
actually reached.

### Why engineers want it

"Present by name" and "actually connected" are different. A net can be labelled `SPI_IO2` and touch
only the flash, with the controller end forgotten. Signal-missing would not fire (the net exists);
this catches the half-made connection by its fan-out.

### How it is checked

`net.pin_count(?net, ?c), ?c < 2` over the profile's signal nets. A single-connection net is a stub:
named, but not wired through.

### For software readers

The net exists, so a null check passes, but it has one endpoint where it needs two, like a
reference that is non-null but points at an object with a required field unset. Presence is not
connectivity.
