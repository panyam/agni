---
title: "Pins and packages"
description: "Why a pin number is a fact about a plastic body rather than about the chip, what sits between the silicon and the leg you solder, and what that means for anything that joins a design to a datasheet."
---

A datasheet states limits about a part. A schematic states connections to that part's pins. Joining
the two looks like a matter of matching pin numbers. It is not, and the reason is physical rather
than notational. This page starts from the article on the bench and works outward to what a tool
has to model.

The short version: **a pin number belongs to the package, not to the chip.** The same silicon is
sold in several bodies, each wiring it to differently numbered legs, so a number means different
things in different bodies. A pin name survives the change and a number does not.

## What is actually in there

An integrated circuit is a stack of five objects rather than one, and the words for them get used
interchangeably in conversation, which is where the confusion starts.

| Layer | What it is |
|---|---|
| die | The silicon itself, a rectangle of processed wafer a few millimetres across. Every number a datasheet prints is ultimately a fact about this. |
| bond pad | A metal contact on the die's top surface. One electrical terminal of the circuit. |
| bond wire | A wire thinner than a hair, running from a bond pad out to a lead. |
| lead | The metal leg that solders to the board. Also called a pin, a leg, or a terminal. |
| package | The moulded body plus its lead frame: the whole black rectangle you can pick up. |

The die is the part. The package is how it was shipped. A vendor sells the same die in a handful of
packages, because a phone needs the small one and a lab instrument wants the one you can solder by
hand.

Here is a real one, the TXB0104 in its 14-lead TSSOP body, drawn the way the datasheet draws it.

<figure>
{{ includeFile "figures/txb0104-tssop14-pinout.svg" }}
<figcaption>One package. The names outside belong to the die. The numbers inside belong to this body.</figcaption>
</figure>

## The same part, three bodies

The TXB0104 also ships as a 12-lead UQFN and a 12-ball DSBGA. Same die, same behaviour, same
datasheet. The pin table prints one column per body, and reading across a row follows one terminal
between them.

| Terminal | Function | PW (TSSOP-14) | RUT (UQFN-12) | YZT (DSBGA-12) |
|---|---|---|---|---|
| `VCCA` | supply in | 1 | 1 | B2 |
| `VCCB` | supply in | 14 | **11** | A2 |
| `B3` | data I/O | **11** | 8 | C1 |
| `B4` | data I/O | 10 | 7 | D1 |
| `OE` | input | 8 | 12 | C2 |
| `GND` | ground | 7 | 6 | D2 |
| `NC` | no connect | 6 | absent | absent |
| `NC` | no connect | 9 | absent | absent |

Three things fall out of that table, and each one breaks a plausible-sounding shortcut.

**Number 11 means different terminals in different bodies.** In the TSSOP it is `B3`, a data line.
In the UQFN it is `VCCB`, the main supply input. A tool that joined a design to a datasheet by pin
number, on a part seeded from one body and placed in another, would compare a power rail against a
data line's limits. It would not error while doing it, because both are real terminals with real
limits. It would report that everything looks fine. That failure is worse than no answer at all,
so the name serves as the primary key and the number is only ever a tie-breaker.

**A number is not always a number.** Ball-grid packages designate by row and column, so `A2` and
`B2` are pin "numbers" in the DSBGA. Anything storing a designator as an integer has already lost.

**A name is not unique.** `NC` is printed on two terminals of the 14-pin bodies. Large parts do
this with real terminals too, printing `VDD` on three pins that may carry different limits. So the
name channel resolves most cases and needs the number to settle the rest.

## One pad, several legs

A terminal on the die can reach more than one leg of the same package. This is not a labelling
convenience. Three physical reasons drive it, roughly in order of how often they decide the matter.

**Current.** A bond wire and a lead can only carry so much before the resistance and the heating
become the circuit's problem. A part drawing several amps fans its supply and ground out across
several legs in parallel.

**Heat.** The legs are the die's main escape route for heat, out into the board's copper. More
ground legs means a better thermal path. The exposed metal pad on the underside of a QFN is that
idea taken as far as it goes.

**Inductance.** At high switching speeds, several short return paths behave better than one long
one. It costs a leg and buys real margin.

<figure>
{{ includeFile "figures/one-bond-pad-three-legs.svg" }}
<figcaption>Three legs, one bond pad. The datasheet prints this as a single row reading GND 2, 5, 7.</figcaption>
</figure>

So the multiplicity runs in two directions at once. One die terminal has a different number in each
body, and it may have several numbers within one body. Both are addressing rather than anatomy: the
terminal is still one terminal.

The rule that follows is asymmetric, and worth stating on its own. **One number identifies one
terminal. One terminal may hold many numbers.** A package cannot send leg 5 to two different places,
but it can bring three legs to the same place.

## What this means for a tool

Anything joining a design to a datasheet has to pick which channel it trusts, and the choice is not
symmetric.

- **Match on the name.** Both sides copy it from the same pin function table, the datasheet directly
  and the symbol library by transcription, so it survives a change of body.
- **Use the number to break ties**, and only inside a body you have actually identified. It is the
  answer to the name's one weakness rather than a competing key.
- **Refuse when the two disagree.** That is a repackaging mismatch or a wrong symbol, and either
  channel taken alone would have produced a confident wrong answer.
- **Refuse when a name is ambiguous and no body is known.** Reporting nothing is recoverable.
  Reporting about the wrong terminal is not, because nothing downstream looks wrong.

A design carries both channels already. A schematic connection names a component and a pin
designator, the component resolves to a part type through its section, and the part type's pin
carries a name alongside that designator. So the join has a name and a number available on both
sides, and the only question is which one leads.

How Agni models this, and the exact precedence its resolver implements, is in
[the datasheet layer](../../architecture/datasheet-layer/#pin-binding). The wider map from circuit
concepts to software ones is in [the software analogy](../analogy/).
