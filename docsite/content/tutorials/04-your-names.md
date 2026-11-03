---
title: "4. Your names"
description: "Teach the tool which nets are rails, and what a legal name looks like on your boards."
---

Everything so far used only the shipped rules, which know nothing about your team. This rung is the
first of four that change that, and it is deliberately first because it changes what the later ones
can see at all.

From here on the ladder runs inside a project, `examples/jetson-project`, which wraps a team's
configuration around the same Jetson board. Copy the board into it once, after `make samples-oracle`
at the repo root:

```
cd examples/jetson-project
make board
```

Run the commands on this page and the next ones from that folder. The board is copied into
`designs/jetson/`, beside the project's own `design.yaml`.

The vocabulary is the `conventions:` section of `project.yaml`, and it carries two halves that are
wired very differently.

## The problem

This project already carries the `conventions:` section. To show what goes wrong without it, the
runs below first delete it from their scratch copy of the project. Ask the board how many of its
nets are power rails:

{{ agniRun "content/tutorials/runs/04-query-rails-off.yaml" }}

The built-in vocabulary finds a lot. Most boards name their rails `VCC`, `VDD`, `+3V3` or `5V_OUT`,
the vocabulary is anchored on those names, and most of this board's rails are named that way. Now
look at the nets whose names mention a voltage or a supply but which it did not call a rail:

{{ agniRun "content/tutorials/runs/04-query-missed-rails.yaml" }}

Two kinds of net are in that list. `I2C0_SCL_1V8` and `VSYNC_CAM0_3V3` are signals named for their
logic level, and the vocabulary is right to skip them, because the voltage in the name is a fact
about the wire's swing rather than a supply. `CSIA_3V3`, `CSIA_5V`, the `USBCn_VBUS` nets, `FAN_VDD`
and `PDC_LDO_3V3` are rails, named for what they feed, and the vocabulary cannot tell them from the
signals by spelling alone.

The consequence is quiet. A rule that finds its rails BY NAME simply sees fewer of them. It does not
warn you, and it does not fail. A rail with no {{ explainable "test-point" }} is not reported,
because as far as that rule is concerned the net is not a rail.

## The two halves

```yaml
conventions:
  name: jetson
  lexicon:
    net:
      rail:
        patterns:
          - "^CSI[AB]_([0-9]+V[0-9]*|I2C_VCC)$"
          - "^USBC[0-9]_VBUS$"
          - "^FAN_VDD$"
          - "^(PDC_LDO|FTDI)_[0-9]+V[0-9]*$"
  rules:
    - name: active-low-overbar
      severity: warning
      why: "house style writes an active-low signal with KiCad's overbar, ~{NAME}, never a trailing n"
      allow: ["[^n]$", "[a-z]n$"]
```

**`lexicon`** teaches the engine which of your names mean what. It is applied when the design is
read, before any rule runs, so it changes the input every rule sees. It is also the half
people skip. Each pattern here names a rail by its function rather than matching any trailing
voltage, since a pattern as loose as `_3V3$` would sweep the logic-level signals in too.

It is grouped by what is being named, because the named thing decides whether a pattern is right.
`net` holds `rail`, `ground` and `feedback`, matched against NET names. `pin` holds `supply`,
`gate`, `source` and `drain`, matched against a component's PIN names. A supply pin is called `VDD`
or `VIN` while a rail net is called `3V3` or `+5V`, so a pattern that belongs in one group is wrong
in the other. There is also a `class` map, which marks a PART as belonging to a component class.

**`rules`** adds checks. They join the catalog namespaced under the config's name, so this one
appears as `jetson/active-low-overbar`. Most people opening a conventions section expect only this
half. This board writes most active-low signals with KiCad's overbar, `~{CLKREQ_E}`, and a few with
a trailing `n`, and the rule asks for one spelling.

## Both halves, visible

Both runs leave out rung 6's parameter corpus. A datasheet that names a part's supply pins also
tells the engine which nets are rails, and here we want to see what the conventions alone change.
Without the section:

{{ agniRun "content/tutorials/runs/04-check-conventions-off.yaml" }}

With it:

{{ agniRun "content/tutorials/runs/04-check-conventions.yaml" }}

Several things changed, and they come from both halves.

`jetson/active-low-overbar` appears, with two findings. That is the rules half:

{{ agniRun "content/tutorials/runs/04-check-naming.yaml" }}

The rest is the lexicon half. `test-point-coverage` found more rails without a test point, because
nets that were invisible to it became rails. `rail-not-classified` is gone too. It is the shipped
catalog's own tripwire for this gap, and it had named `FTDI_3V3`. That net's name declares 3.3 V,
and it feeds four supply pins, but it carries no rail role. It is already in the count above, since
`net.rail` also counts any net that something drives as a power net. The rules read the rail ROLE,
though, and only the lexicon gives the net one. And two `i2c-pull-up` errors disappeared:

{{ agniRun "content/tutorials/runs/04-verdicts-ftdi.yaml" }}

Both lines were pulled up all along, through R113 and R114 to `FTDI_3V3`. The rule asks for a
pull-up to a RAIL, and until the lexicon named that net a rail, the resistors led nowhere the rule
recognized. Two of rung 2's thirty errors were the vocabulary's blind spot rather than the design's.

Every tier follows that pattern. Supplying a tier adds its own findings. It can also change what
your existing rules are able to see, in both directions.

## Reading the lexicon directly

Inferring the lexicon from finding counts that moved is indirect. You can ask the fact base itself,
with the same question that opened this rung:

{{ agniRun "content/tutorials/runs/04-query-rails-conventions.yaml" }}

Nothing was added to the design and no rule ran. The lexicon changed what the engine believes a rail
*is*, and every relation derived from that role now answers differently.

Write a lexicon in a loop. Ask, compare the answer against the board's known rails, adjust the
pattern, and ask again. A query reads only the lexicon half, since it runs no rules.

## Writing your own

Start with the lexicon, not the rules. Run `agni query <design> 'net.rail(?n) => ?n' --conventions
<your file>` on a real board and compare the list against the board's known rails. The file holds
the section's keys (`name`, `lexicon`, `rules`) at its top level. For that run it replaces any
conventions declared by the design's project, so you can try a candidate without editing the
project. Whatever is missing tells you the pattern you need. Repeat until the list is right, move it
into the project's `conventions:` section, and only then write naming rules.

A rule's `allow` is a list of patterns, and a net name passes if it matches any of them. Getting
this backwards is easy, because `allow` describes what is legal and the finding is a name matching
none of them.

Distinct name spaces need distinct lexicon entries. Rail net names and supply *pin* names are
usually named differently, so `net.rail` and `pin.supply` are separate dimensions rather than one
shared pattern list.

## Next

[Your interfaces](../05-your-interfaces/), which declares a bus once and checks every board against
it.
