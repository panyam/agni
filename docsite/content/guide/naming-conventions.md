---
title: "Naming conventions"
description: "Write your net and ref-des house style down once as patterns, and check enforces it on every design."
---

Your team probably has a house style for net and
{{ explainable "reference-designator" "reference-designator" }} names: power nets look like
`+3V3`, signal nets are `UPPER_SNAKE`, no ref des is reused. A conventions file writes
that style down once, as patterns, and `check` enforces it on every design. This is the
simplest way to add your own rules, and it needs no code.

## Write a conventions file

A conventions file is a small YAML document naming one or more rules. Each rule is a set of
allowed patterns and a set of exemptions:

```yaml
# example.yaml
name: example
rules:
  - name: signal-net-naming
    severity: info
    why: "signal nets are UPPER_SNAKE, rails and stubs are exempt"
    allow: ["^[A-Z][A-Z0-9_]*$"]
    exempt: ["^\\+", "^GND"]
```

- **`allow`** passes a name that matches any of these patterns.
- **`exempt`** skips any name matching one of these patterns (here,
  {{ explainable "rail" "rails" }} starting with `+`, and `GND`).
- **`name`** (top level) is the namespace, so the rule above appears in the catalog as
  `example/signal-net-naming`.

## Run it

Compose the file into a run with `--conventions`. On a board whose names all conform, the rule runs
and stays quiet:

{{ agniRun "content/guide/runs/conventions-quiet.yaml" }}

Read the second line as well as the first. It says the rule looked at six names and had nothing to
report, which is a different statement from a rule that ran over nothing.

Point the same convention at a board that does not follow it and the non-conforming names surface:

{{ agniRun "content/guide/runs/conventions-findings.yaml" }}

Both flagged names carry a trailing character the `allow` pattern does not cover. Neither is exempt
either, because `^\+` exempts a name that *starts* with `+`, which neither of these does.
Tightening `allow`, or narrowing `exempt`, surfaces more names the same way.

## In a project

A project writes the same content as the `conventions:` section of its `project.yaml`, and every run
on one of its designs composes it with no flag. A team sharing one vocabulary across projects puts it
in a project of its own, which the others name with `extends`. See
[Projects and designs](../../architecture/projects-and-designs/). A `--conventions` file still works
on a project's design, and replaces the project's section for that run in the way the next section
describes.

## Tool-generated names are exempt by default

CAD tools invent stub names for connections you never named: KiCad `unconnected-(...)`,
`Net-(...)`, gEDA/EDIF `N$…` autonames. These are never your house style and are **exempt
automatically**, so a convention only ever flags a name a human actually chose. You do not
have to list them in `exempt`.

## Scope of a match

By default a pattern matches the **leaf** of a qualified name. A hierarchical net like
`/amp2/CTRL` is matched on `CTRL`, so one convention works across every sheet without
encoding the hierarchy into the pattern.

## A server default, and how a request replaces it

`agni serve --conventions house.yaml` makes that config the deployment's default. Its rules join the
catalog every rule-running surface uses, and its lexicon becomes the default naming vocabulary, so
everyone asking that server questions gets the house answer without doing anything.

A request may carry its own convention instead, and when it does it **replaces** the server's for
that request. Both halves go together, so the request's rules replace the server's rules and its
lexicon replaces the server's vocabulary. Nothing of the deployment's convention survives into a
request that named its own.

That has two consequences.

**Reusing the server's name is fine, and is the natural way to refine it.** A config named `house`
sent to a server whose default is also named `house` simply replaces it. (Before this was settled,
that combination failed outright with a duplicate-source error.)

**Replacing is not the same as adding.** If your project wants the house rules *plus* its own, the
config it sends has to contain both. That is deliberate, because a request asking "what does this board look
like under MY vocabulary" should get exactly that, and a caller who could not turn the deployment's
rules off could not ask the question. It does mean that a finding which disappeared after switching
conventions may have disappeared because the rule stopped running, not because the design improved.

Only the convention is replaced. The built-in rules, and anything from `--profile-path`, are
unaffected. A request's `--intent-path` replaces the design's intent the same way.

### In the browser

The viewer has a **vocabulary** control in the top bar. It lists the convention configs sitting beside
the open design, and picking one applies it to everything that runs rules from then on: the checks
panel, the report, and a review run. For a design whose project declares conventions it offers no
file, because the project's conventions are already in every run.

The bar always says which vocabulary the answers on screen were computed under, and looks different
while a request convention is in effect, so you can tell a finding the design fixed from one whose
rule stopped running (the same caution as above).

Switching discards the findings already on screen rather than keeping them, since they were computed
by asking a different question.

The **Lexicon** panel, tabbed beside Rules, lists every vocabulary the run reads names with and where
each entry came from. It edits a draft of the convention as written, so its rules stay with it. Add an
entry, take one out, turn a vocabulary off or switch the rail inference, and the list updates. Taking
a single built-in out writes that vocabulary with `replace: true` and its other entries stated, which
is the only way a convention can drop one. A built-in ref-des prefix cannot be taken out, since
`replace` leaves the prefix table alone.

**Try** shows the nets whose roles would change and the findings that would appear or disappear, the
same rows `agni lexicon --try` prints, and a row selects its net or part. A draft changes nothing on
the page until **Apply**, which puts it in the vocabulary bar for the rest of the session and reads a
comparison under it as well. **Save YAML** writes the file, and only the file in your project reaches
other people and other commands. Until then `--conventions` names it on `check`, `review`, `query`,
`stats`, `diff` and `render`.
It works in the browser engine too, so the public demo has it. Net Properties shows why each role
is there.

## Teach it your vocabulary

The `rules` block above checks names against your policy. The `lexicon` block is the other half and
tells the engine what a name *means*, so the built-in rules reason about your board in your words.

A net can carry several roles at once, and six of them are configurable. A seventh net vocabulary
names the I2C bus lines the pull-up rules judge, and the pin vocabularies name supply pins, transistor
terminals and a package's exposed pad:

```yaml
lexicon:
  net:
    rail:      { patterns: ["^HV_"] }        # added to the built-ins
    ground:    { patterns: ["^CHASSIS_"] }
    feedback:  { patterns: ["_FBK$"] }
    switching: { patterns: ["_HSD$"] }
    control:   { patterns: ["_SHDN$"] }
    gate_drive: { patterns: ["_VBOOST$"] }
    i2c:       { exclude: ["^DDC_"] }        # SDA/SCL by default, I2S lines excluded
    infer_rail_from_supply_pins: false       # on unless a convention turns it off
  pin:
    supply: { patterns: ["^PWR_"] }
    gate:   { patterns: ["^DRV$"] }
    source: { patterns: ["^SRC_"] }
    drain:  { patterns: ["^DRN_"] }
    exposed_pad: { patterns: ["^GND_PAD$"] }
```

Patterns are RE2, matched case-insensitively against the **leaf** of a hierarchical name, so
`/DCDC/12V_SW` is tested as `12V_SW`. They are ADDED to the built-in set unless you say
`replace: true`, which makes yours the whole vocabulary. `replace: true` with no patterns is how you
turn a built-in role off entirely. `exclude` takes a name back out even when a pattern matches it,
since RE2 cannot say "SDA, but not `I2S0_RX_SDA`" in one pattern. Exclusions are added and replaced
the same way patterns are.

Every role a net carries records why. A role from a pattern names the pattern, and an inferred rail
names the supply pin it feeds, so a rail you did not expect can be traced to the line that made it
one. The reason travels with the net's roles in the design's IR, so
`agni stats <design> --format json --mask design.nets` prints it.

### Rails inferred from the pins they feed

A rail named after its consumer, such as `LPC_VCC` or `FAN_VDD`, does not start with a supply word,
so the `rail` patterns miss it. With `infer_rail_from_supply_pins` on, which is the default, a net is
also a rail when a later token of its name matches the `supply` pin vocabulary AND the net feeds a
`power_in` pin of a fitted part that is not a connector. The role's reason names that pin. Both halves
are needed. `EN_VCC` feeds no supply pin and stays a signal, a USB receptacle's own VBUS pin marks
where a cable delivers power rather than a load, and a supply pin left unwired on its own stub is no
rail at all. Voltage-named rails such as `CSIA_3V3` are not inferred, which is why
`rail-not-classified` still reports them and the section below is still how you declare them.

### The four roles that mean "named after a rail, but not a rail"

A regulator's pins are conventionally named for the supply they produce, so a 12V converter's nodes
are `12V_FB`, `12V_SW`, `12V_MODE1`, `12V_VDRV`. Every one of those matches the `rail` vocabulary on
its prefix and none of them carries 12V. `feedback`, `switching`, `control` and `gate_drive` are how
the engine tells them apart, and a net carrying any of them is not treated as a rail:

| role | typical suffixes | what it is | safe to probe? |
|---|---|---|---|
| `feedback` | `_FB`, `_VSENSE` | the divider tap setting the output | no, probing shifts regulation |
| `switching` | `_SW`, `_BOOT`, `_PHASE`, `_LX` | the power-stage node | no, highest dV/dt net on the board |
| `control` | `_EN`, `_MODE1`, `_SEL` | enable and configuration inputs | yes |
| `gate_drive` | `_VDRV`, `_VGATE` | the gate driver's own supply | yes |

The first two are excluded from probe-point rules because probing them is harmful. The last two are
excluded because they are not rails, so a rule quantifying over rails would be naming the wrong
subject. If your house convention spells a switched output `3V3_SW` rather than a switch node, narrow
the `switching` vocabulary with `replace` and the rest keeps working.

The `feedback` and `switching` vocabularies also read a part's pin names, spelled with a leading
underscore, so `SW1` is checked as `_SW` and `FB2` as `_FB`. That is how a connector on a buck's
output reads as an outlet when no net is named for the converter. A house that calls the pin `FDBK`
adds `_FDBK$` to `feedback`.

These are the same six vocabularies the engine ships with defaults for, in the same schema. There is
no separate list of built-ins to consult.

### Component classes

A part's class comes from two signals: its ref-des prefix (`R`, `TP`, `RT`) and the words in its
part name and attributes (`ferrite`, `tvs`, `crystal`). The `class` block extends both, keyed by the
class it teaches.

A third signal is the part-number family. A part whose part number belongs to a common ESD or TVS
family (`TPD4E05U06`, `PESD5Z5.0F`, `ESDA25P35`, `USBLC6`, `VBUS54CV`, `SMAJ`, and their neighbours)
is a `tvs` whether it sits under a diode, an IC or an unknown symbol, because an ESD array is often
drawn as a `U` part and a TVS diode often carries nothing but its part number. The bare words `tvs`
and `esd` only refine a diode, since an IC whose description mentions ESD protection is still an IC.
`families` adds to the built-in list, and `replace: true` replaces it along with the patterns.

```yaml
lexicon:
  class:
    thermistor: { prefixes: ["TH"] }         # your house writes TH12, not RT12
    zener:      { prefixes: ["Z"] }
    tvs:        { families: ["^acmeesd"] }   # a house part-number family the built-in list lacks
```

A board that plugs into another one has connectors facing nothing a user touches, and those are not a
class. Whether a connector is the module socket or the cable entry depends on the product rather than
the part, so it is declared per design in the `components` section of its
[design intent](../design-intent/), which the exposure rules read. A lexicon naming the
`internal_connector` class it replaced fails to load and says where the declaration goes.

A prefix is the leading run of letters of a ref-des, so `TH` matches `TH12` and a part whose
library symbol declares `TH?`. Prefixes are matched case-insensitively and are ADDED to the built-in
table, and a prefix you list wins over a built-in one, so a house that writes `F` for its ferrites
re-points `F` away from fuse. `replace: true` applies to `patterns` only and leaves prefixes alone.

The family tag follows the class, so a `TH12` answers both `component.class(TH12, "thermistor")`
and `component.class(TH12, "resistor")`, exactly as an `RT12` does.

Three things fail at load rather than reading as a convention that matched nothing:

- a class the engine does not ship (`unknown component class "esd_array"`),
- a prefix holding anything but letters (`T1` can never match, because `T1` is a ref-des),
- one prefix listed under two classes.

The class must be one the engine already has. A project cannot yet declare a class of its own, such
as a house `esd_array` refining `diode`, because that opens a vocabulary the engine keeps closed, and
DECISIONS.md records what would change that. The class names are the ones `component.class`
answers with, listed in the [relation catalog](../../reference/relations/).

## See how your names were read

`agni lexicon <design>` lists the vocabulary a run reads names with, every pattern under its
vocabulary with where it came from: built in, or the one convention that won, from the server, the
project, the design or the request. A built-in a `replace: true` switched off stays in the list,
marked, so you can see it went.

`--net` says why one net has its roles, and `--try` reads the design again with a different
conventions file and says what moves. The design below runs its sensor from its own regulator, whose
output is `SENSOR_3V3`, and its `design.yaml` adds the rail pattern that name needs. Tried without it,
the sensor supply stops being a rail, and the I2C pull-ups that go to it stop counting:

{{ agniRun "content/guide/runs/lexicon.yaml" }}

Once the changes are the ones you meant, carry the file in the project's `conventions:` section.
A project's or design's convention reaches every command that reads the design. A `--conventions`
file reaches the commands that take that flag, which are `check`, `review`, `query` and `lexicon`.

`agni lexicon <design> --format yaml` writes the whole effective vocabulary as a conventions file,
every list stated in full, which is a starting point when you would rather edit the complete list
than add to it.

## Where to go next

- [Checks and reports](../checks-and-reports/) shows that conventions findings read like any
  other, and that `--fail-on` can gate on them.
- [CLI reference](../cli-reference/) documents the `--conventions` flag and `agni lexicon`.
