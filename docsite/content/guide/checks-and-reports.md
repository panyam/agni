---
title: "Checks and reports"
description: "Narrow what runs, read the report in different shapes, and follow a finding back to its source."
---

[Getting started](../getting-started/) ran the whole catalog and read the first few
findings. This page is about working the report: narrowing to what you care about, reading
it in different shapes, and following a finding back to what the tool actually saw.

## Narrow what runs

By default every rule runs. Two flags cut that down.

**One rule by name** (`--rule`, repeatable):

```
agni check showcase.fires.kicad_pro --rule i2c-pull-up
```

```
findings by rule:
  i2c-pull-up            1

first 1:
  [error] i2c-pull-up: SCL (I2C net has no pull-up resistor)

1 finding(s) total
```

**A whole group by tag** (`--tag key=value`, repeatable). Every rule carries catalog tags
(category, tier, and more), so you can run one family at a time:

```
agni check showcase.fires.kicad_pro --tag category=power
```

This runs only the power-related rules and skips the rest. `--tag category=connectivity`
runs the connectivity group, and so on. Combine `--rule` and `--tag` to build exactly the
set you want.

## Read the report in the shape you need

The default text output is a per-rule summary. Three other forms via `--format`:

- **`markdown`**: a severity-organized report, worst first, for pasting into a review:

  ```
  agni check showcase.fires.kicad_pro --format markdown
  ```

  ```
  # agni check — showcase.fires.kicad_pro

  | severity | findings |
  |---|---|
  | error | 1 |
  | warning | 5 |
  | info | 4 |

  10 finding(s), 29 rule(s) run.

  ## error

  ### i2c-pull-up — An I2C net (SDA/SCL) has no pull-up resistor.
  - `SCL` — I2C net has no pull-up resistor (showcase.fires.kicad_sch)
  ```

- **`json`**: one object per finding, for tooling (see provenance below).
- **`report`**: the same report as JSON (the wire shape the web viewer consumes).

The header line (`10 finding(s), 29 rule(s) run`) is your coverage receipt. It says how many
rules actually ran, so a clean report is distinguishable from a report that had little to
check. See "silence is not a pass" in [Concepts](../concepts/).

## Follow a finding to its source

Every finding carries **provenance**: the record of exactly what the tool saw. `--format
json` exposes it:

```
agni check showcase.fires.kicad_pro --format json
```

```json
{
  "rule": "bulk-cap",
  "severity": "warning",
  "subject": { "kind": "net", "ref": "+3V3", "pin": "" },
  "message": "power rail has no bulk capacitor",
  "provenance": { "sourceFile": "showcase.fires.kicad_sch" }
}
```

- `subject` is what the finding is about: a `net` (`+3V3`), a `component` (its ref des), or
  a component `pin`.
- `provenance.sourceFile` is the file the fact came from. For datasheet-backed rules the
  message also names the datasheet page and table (see [Datasheets](../datasheets/)).

This is why you never have to take a finding on faith. It points at the net, pin, or
datasheet row you can go open yourself.

## Understand a rule

Each rule ships with a short explainer describing what it looks for and why the condition
matters electrically. In the viewer it appears beside the finding. On the command line the
rule name is the handle you look it up by. The catalog is open, so your team can add house
rules on top of the built-ins. [Naming conventions](../naming-conventions/) is the simplest
form of that, configured without any code, and [interface profiles](../interface-profiles/)
are the same idea for a bus: declare its signals, get a rule per requirement.

Here is one, in full, so you know what to expect before you go looking. Every rule in the
[rule catalog](../../reference/rules/) carries the same shape: what it checks, why it matters in the
circuit, and what its silence does and does not mean. That last part is the one worth reading
before you trust a green result.

<details>
<summary><strong><code>cap-voltage</code></strong> — a capacitor's rating against the rail it sits on</summary>

{{ includeCard "content/reference/rules/cap-voltage.md" }}

</details>

A rule that could not evaluate never reports a pass. It reports not-applicable, needs-data, or
inconclusive, each naming what was missing. That distinction is the whole reason a clean report from
this tool is worth something, and it is why the cards spend as much space on absence as on the check
itself.

## Gate a build

`--fail-on <severity>` exits non-zero when any finding sits at or above the threshold, so
`check` gates CI:

```
agni check showcase.fires.kicad_pro --fail-on error   # non-zero here (1 error)
agni check showcase.passes.kicad_pro --fail-on error  # exit 0 (clean)
```

That gate reads **severity**, which is a statement about consequence. It has nothing to say about the
distinction the section above is built on: a check that decided, versus one that never ran. A design
whose datasheet corpus moved keeps passing this gate, because a rule that could not evaluate produces
no findings and no findings is what clean looks like.

`review` gates on the other axis:

```
agni review designs/gateway --checklist review.yaml --fail-on-outcome fail
agni review designs/gateway --checklist review.yaml --min-answered 13
```

`--min-answered` counts the items that produced an answer (`pass`, `fail`, `provisional`,
`computed-n/a`), which is stricter than the covered count the report also prints. Covered subtracts
only `not-automated`, so an item whose rule is present and whose inputs are gone still counts as
covered. Both gates are off by default, and a tripped one exits `2` where a failed run exits `1`. See
the [CLI reference](../cli-reference/#gating-a-pipeline-on-a-review) for the full vocabulary.

## Where to go next

- [Datasheets](../datasheets/): turn on the rules that compare your design against a part's
  real limits.
- [Comparing revisions](../comparing-revisions/): diff two versions of a design.
- [CLI reference](../cli-reference/): every flag in one place.
