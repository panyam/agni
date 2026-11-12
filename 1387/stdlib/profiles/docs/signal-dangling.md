## signal-dangling

### What it means

It reports a profile signal net that exists by name but carries fewer than two connections. The
net is present in the netlist, so a presence check passes. But it is wired to only one pin, so the
far end of the bus is not actually reached.

### Why engineers want it

"Present by name" and "actually connected" are different. A net can be labelled `SPI_IO2` and touch
only the flash, with the controller end forgotten. Signal-missing would not fire, because the net
exists. This rule catches the half-made connection by its fan-out.

### How it is checked

`net.pin_count(?net, ?c), ?c < 2` over the profile's signal nets, when the presence gate says the
bus is in use. A single-connection net is a stub, named but not wired through.

### For software readers

The net exists, so a null check passes, but it has one endpoint where it needs two, like a
reference that is non-null but points at an object with a required field unset. Presence is not
connectivity.
