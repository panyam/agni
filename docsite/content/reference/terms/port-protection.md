---
title: "Port protection"
label: "protection"
summary: "Parts placed at a connector to absorb what arrives from outside the board, chiefly static discharge from a person and interference picked up by the cable."
level: EE4
---

Anything leaving the board through a connector is exposed. A cable is an antenna, and a person
touching the connector is a static discharge of a few kilovolts arriving at a pin rated for a few
volts.

Protection is whatever is placed at the connector to absorb that before it reaches anything expensive:
a TVS diode or diode array for the discharge, a common-mode choke or ferrite for the interference.

```mermaid
flowchart TB
    C["connector<br/>ESD, and whatever<br/>the cable picked up"] --> P["protection<br/>TVS diode,<br/>common-mode choke"]
    P --> X["transceiver"]
    X --> M["processor"]
    C -.->|"with no protection,<br/>this is the path"| X
```

The ordering is the point. Protection has to sit between the connector and the part it defends, so a
board that has the right parts in the wrong order has no protection at all, and nothing in the netlist
distinguishes the two.

That is why it is declared in a profile rather than inferred, the same as
[termination](../termination/).

**Where the course teaches it:**
[chapter 10](../../../learn/10-interfaces-and-what-they-require/), a bus as a contract.
