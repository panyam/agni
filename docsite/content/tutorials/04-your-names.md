---
title: "4. Your names"
description: "Teach the tool which nets are rails, and what a legal name looks like on your boards."
---

Everything so far used only the shipped rules, which know nothing about your team. This rung is the
first of four that change that, and it is deliberately first because it changes what the later ones
can see at all.

The file is `conventions.yaml`, and it carries two halves that are wired very differently.

## The problem

This project already carries the file this rung is about, so to see the problem it solves you have to
move it out of the way first. The seeded parameter corpus moves aside with it, for a reason worth
knowing up front: a datasheet that types a pin as a supply establishes the rail on its own, so with
the corpus in place these rails classify whether or not the naming vocabulary reaches them. That is
the later rungs' subject. This one is about names, so both are out of the way. Ask the board which of
its nets are power rails:

{{ agniRun "content/tutorials/runs/04-query-designs-gateway-gateway-edn-rail-n-n.yaml" }}

One rail. The board has four. `PMIC_MAIN_12V0`, `PMIC_CORE_3V3`, and `PMIC_IO_1V8` are all supply
rails and the tool does not think any of them is.

That is not a bug. The built-in rail vocabulary is anchored on the names most boards use: `VCC`,
`VDD`, `+3V3`, and so on. This project names rails function-first, subsystem before voltage, so none
of them start the way the vocabulary expects.

The consequence is quiet and it is the important part. Every rule that quantifies over rails simply
finds fewer members. It does not warn you. It does not fail. A rail with no bulk capacitor is not
reported, because as far as the tool is concerned that net is not a rail.

## The two halves

```yaml
name: gateway
lexicon:
  net:
    rail:
      patterns: ["_[0-9]+V[0-9]$"]
rules:
  - name: signal-net-naming
    severity: warning
    why: "house style names a clock net CLK_*, so XTAL_IN / XTAL_OUT are off-convention"
    allow: ["^(PMIC|CAN[0-9]+|I2C|MCU|CLK|GND)"]
```

**`lexicon`** teaches the engine which of your names mean what. It is applied when the design is
read, before any rule runs, so it changes the input every rule sees. This is the half that matters
most and the half people skip.

It is grouped by what is being named, because that is what decides whether a pattern is right.
`net` holds `rail`, `ground` and `feedback`, matched against NET names. `pin` holds `supply`,
`gate`, `source` and `drain`, matched against a component's PIN names. A supply pin is called `VDD`
or `VIN` while a rail net is called `3V3` or `+5V`, so a pattern that belongs in one group is wrong
in the other. There is also a `class` map, which marks a PART as belonging to a component class.

**`rules`** adds checks. They join the catalog namespaced under the config's name, so this one
appears as `gateway/signal-net-naming`. Most people opening a conventions file expect only this half.

## Both halves, visible

Without the file:

```
findings by rule:
  decoupling-present     1
  esd-protection         2
  i2c-pull-up            2
  profile/can-esd-missing 2
  reverse-blocking-absent 1
  test-point-coverage    1
```

With it:

{{ agniRun "content/tutorials/runs/04-check-conventions.yaml" }}

Two things changed, one from each half.

`gateway/signal-net-naming` appears, with two findings. That is the rules half:

```
[warning] gateway/signal-net-naming: XTAL_IN (net name matches no allowed naming pattern)
[warning] gateway/signal-net-naming: XTAL_OUT (net name matches no allowed naming pattern)
```

And `test-point-coverage` went from 1 to 2. Nothing about that rule changed. A net that was
invisible to it became a rail, and that rail has no test point:

```
[info] test-point-coverage: GND (rail carries no test point; bring-up and factory test cannot probe it)
[info] test-point-coverage: PMIC_MAIN_12V0 (rail carries no test point; bring-up and factory test cannot probe it)
```

That second change is the lexicon half doing its job. It is worth sitting with, because it is the
shape of the whole tier system: supplying a tier does not just add its own findings, it can change
what the rules you already had are able to see.

## Reading the lexicon directly

Inferring the lexicon from a finding count that moved is indirect. You can ask the fact base itself,
putting the same question this rung opened with:

{{ agniRun "content/tutorials/runs/04-query-rails-conventions.yaml" }}

Four rails, where the same query without the flag found one. Nothing was added to the design and no
rule ran. The lexicon changed what the engine believes a rail *is*, and every relation derived from
that role now answers differently.

This is the loop to write a lexicon in: ask, compare against the rails you know the board has, adjust
the pattern, ask again. `--conventions` on `query` reads only the lexicon half, since a query runs no
rules.

## Writing your own

Start with the lexicon, not the rules. Run `agni query <design> 'rail(?n) => ?n' --conventions
<your file>` on a real board and compare the list against the rails you know it has. Whatever is
missing tells you the pattern you need. Repeat until the list is right, and only then write naming
rules.

A rule's `allow` is a list of patterns, and a net name passes if it matches any of them. Getting
this backwards is easy: `allow` describes what is legal, so a name matching none of them is the
finding.

Distinct name spaces need distinct lexicon entries. Rail net names and supply *pin* names are
usually named differently, so `rail:` and `supply_pin:` are separate dimensions rather than one
shared pattern list.

## Next

[Your interfaces](../05-your-interfaces/), which declares a bus once and checks every board against
it.
