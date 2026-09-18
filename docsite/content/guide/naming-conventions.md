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

- **`allow`**: a name is fine if it matches any of these patterns.
- **`exempt`**: names matching any of these are never checked (here,
  {{ explainable "rail" "rails" }} starting with `+`, and `GND`).
- **`name`** (top level): the namespace. The rule above appears in the catalog as
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

## Tool-generated names are exempt by default

CAD tools invent stub names for connections you never named: KiCad `unconnected-(...)`,
`Net-(...)`, gEDA/EDIF `N$…` autonames. These are never your house style and are **exempt
automatically**, so a convention only ever flags a name a human actually chose. You do not
have to list them in `exempt`.

## Scope of a match

By default a pattern matches the **leaf** of a qualified name. A hierarchical net like
`/amp2/CTRL` is matched on `CTRL`, so one convention works across every sheet without
encoding the hierarchy into the pattern.

## On a server: a default, and how a request replaces it

`agni serve --conventions house.yaml` makes that config the deployment's default. Its rules join the
catalog every rule-running surface uses, and its lexicon becomes the default naming vocabulary, so
everyone asking that server questions gets the house answer without doing anything.

A request may carry its own convention instead, and when it does it **replaces** the server's for
that request. Both halves go together: the request's rules replace the server's rules, and its
lexicon replaces the server's vocabulary. Nothing of the deployment's convention survives into a
request that named its own.

Two consequences worth knowing.

**Reusing the server's name is fine, and is the natural way to refine it.** A config named `house`
sent to a server whose default is also named `house` simply replaces it. (Before this was settled,
that combination failed outright with a duplicate-source error.)

**Replacing is not the same as adding.** If your project wants the house rules *plus* its own, the
config it sends has to contain both. That is deliberate: a request asking "what does this board look
like under MY vocabulary" should get exactly that, and a caller who could not turn the deployment's
rules off could not ask the question. It does mean that a finding which disappeared after switching
conventions may have disappeared because the rule stopped running, not because the design improved.

Only the convention is replaced. The built-in rules, and anything from `--profile-path` or
`--intent-path`, are unaffected.

### In the browser

The viewer has a **vocabulary** control in the top bar. It lists the convention configs sitting beside
the open design, and picking one applies it to everything that runs rules from then on: the checks
panel, the report, and a review run.

The bar always says which vocabulary the answers on screen were computed under, and looks different
while a request convention is in effect. That is deliberate, and it is the same caution as above: a
finding that vanished when you switched conventions may have vanished because the rule stopped
running. The bar is what lets you tell the two apart.

Switching discards the findings already on screen rather than keeping them, since they were computed
by asking a different question.

## Teach it your vocabulary

The `rules` block above checks names against your policy. The `lexicon` block is the other half: it
tells the engine what a name *means*, so the built-in rules reason about your board in your words.

A net can carry several roles at once, and six of them are configurable:

```yaml
lexicon:
  net:
    rail:      { patterns: ["^HV_"] }        # added to the built-ins
    ground:    { patterns: ["^CHASSIS_"] }
    feedback:  { patterns: ["_FBK$"] }
    switching: { patterns: ["_HSD$"] }
    control:   { patterns: ["_SHDN$"] }
    gate_drive: { patterns: ["_VBOOST$"] }
  pin:
    supply: { patterns: ["^PWR_"] }
    gate:   { patterns: ["^DRV$"] }
    source: { patterns: ["^SRC_"] }
    drain:  { patterns: ["^DRN_"] }
```

Patterns are RE2, matched case-insensitively against the **leaf** of a hierarchical name, so
`/DCDC/12V_SW` is tested as `12V_SW`. They are ADDED to the built-in set unless you say
`replace: true`, which makes yours the whole vocabulary. `replace: true` with no patterns is how you
turn a built-in role off entirely.

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

These are the same six vocabularies the engine ships with defaults for, in the same schema. There is
no separate list of built-ins to consult.

## Where to go next

- [Checks and reports](../checks-and-reports/): conventions findings read like any other,
  and `--fail-on` can gate on them.
- [CLI reference](../cli-reference/): the `--conventions` flag.
