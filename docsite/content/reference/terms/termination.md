---
title: "Termination"
label: "termination"
summary: "A resistor bridging a bus at its far end, sized to the line's characteristic impedance, so a signal arriving there is absorbed instead of reflecting back."
level: EE3
---

A bus long enough to matter is a transmission line, and a signal reaching the end of an unterminated
line does not simply stop. It reflects, travels back down the line, and collides with whatever is
following it.

A terminator is a resistor across the line at that end, sized to the line's characteristic impedance,
which absorbs the signal instead.

```mermaid
flowchart TB
    subgraph U["unterminated"]
        direction LR
        A["driver"] --> B["far end,<br/>nothing across the line"] --> C["the signal reflects and<br/>collides with what follows"]
    end
    subgraph T["terminated"]
        direction LR
        A2["driver"] --> B2["far end,<br/>120Ω bridging the pair"] --> C2["the resistor absorbs it"]
    end
    U ~~~ T
```

The value is not a choice. It is the impedance the bus standard specifies, which is why CAN
terminators are 120Ω on every board that has ever carried CAN. On a
[differential pair](../differential-pair/) the resistor bridges the two halves; on a single-ended bus
it goes to ground.

A profile declares the requirement rather than the board proving it, since a netlist cannot tell a
terminator from any other resistor of that value. See
[`profile-termination`](../../rules/profile-termination/).

**Where the course teaches it:** [chapter 1](../../../learn/01-what-a-board-is-made-of/) for the
resistor, [chapter 12](../../../learn/12-when-the-copper-matters/) for why the copper decides.
