---
title: "Differential pair"
label: "differential pair"
summary: "Two wires carrying the same signal in opposite senses, so a receiver reading the difference between them rejects any interference that hit both equally."
level: EE2
---

Two nets carrying one signal, one rising as the other falls by the same amount. The receiver does not
read either wire against ground. It reads the **difference** between them.

That is the whole trick. Interference picked up along the run hits both wires more or less equally,
which moves both by the same amount and leaves the difference untouched.

```mermaid
flowchart TB
    D["driver"] -->|"CANH rises"| R["receiver<br/>reads CANH − CANL"]
    D -->|"CANL falls by<br/>the same amount"| R
    N["interference along the run<br/>hits both wires equally"] -.-> R
    R --> O["the difference is unchanged,<br/>so the interference cancels"]
```

The two halves are conventionally named as a pair, `X_P` and `X_N`, or for CAN `_CANH` and `_CANL`.
A pair with only one half wired is not a pair at all, and the layout tool will route the survivor as
an ordinary signal, which is what [`diff-pair-naming`](../../rules/diff-pair-naming/) exists to catch.

A pair almost always needs [termination](../termination/) at each end, for a reason that has nothing
to do with noise.

**Where the course teaches it:** [chapter 1](../../../learn/01-what-a-board-is-made-of/), reading a
resistor's job from the nets it touches.
