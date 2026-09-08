# dft-coverage

Design for test: which nets a probe can reach, and which parts a tester can actually measure.

A board can be electrically correct and impossible to debug. A **test point** is a bare pad whose only
job is to expose a net so something can touch it, a scope probe at bring-up or a bed-of-nails in the
factory. This example asks what that layer covers.

## What it shows

The first five steps are **queries over the design's fact relations**, not rule checks. It is the
first example in the ladder to run a query, so it doubles as a tour of derived relations, aggregates
and negation:

| step | what it asks | query shape |
|---|---|---|
| 2 | what is on the board | `=> ?k, count(?c)` |
| 3 | which nets a probe can reach | one join, `=> ?net, count(?tp)` |
| 4 | which parts a tester can measure | two derived relations, `having` |
| 5 | the unmeasurable parts, by part number | negation, `count(distinct)`, `list(distinct)` |
| 6 | what a coverage report cannot say | `check.RunVerdicts` |

Step 4 is the one worth reading. Measuring a two-terminal part needs BOTH ends reachable, so one end
covered measures nothing. Three buckets fall out of one pair of derived relations: both ends, one end,
neither.

Step 5 is where the answer stops being a list. One uncovered part is an oversight; several of one part
number is a placement habit, which is a different thing to fix.

Step 6 leaves coverage behind. A findings list names what is wrong. A verdict names what was ASKED,
including the subjects a rule could not decide and why, in the rule author's words. "No findings" and
"nobody looked" print identically in a spreadsheet.

## Run it

```
make run        # plain text
make demo       # TUI
make runquiet   # non-interactive, CI-safe
make doc        # render the walkthrough to markdown
```

Every step prints the `agni` command that reproduces it, so nothing here is reachable only from Go.

## How it is built

`main.go` is thin: it embeds `walkthrough.md` and binds the steps that run engine code. The prose
lives in the sidecar. See [../CONVENTIONS.md](../CONVENTIONS.md).

Two details are worth knowing before you copy this example.

**The model is built with `check.NewModelWithParams`, not `check.NewModel`.** The model's MPN map is
filled by the params constructor alone, and `component.mpn` reads that map rather than
`ir.Component.mpn`. Built the other way the relation is empty on a design whose components all carry a
part number, so step 5 reports a clean board. A nil spec provider is fine; only the datasheet
relations need a real one.

**The rule catalog is a blank import.** `check.BuiltinRules()` returns nothing without
`_ "github.com/panyam/agni/stdlib/rules/builtin"`, and it fails silently: step 6 read "0 pass, 0 fail,
across 0 rules" until the import was added.

## The design

The bundled fixture `../common/designs/probe-coverage.edn` is synthetic and small, built so each
coverage case occurs exactly once: one passive with both ends probed, three with one end, two with
neither. The two with neither share a part number, so the grouping in step 5 has something to find.
GND deliberately carries no test point.

Point the first step at any design you can read.

Point it at your own board by typing a path at the first step, or by setting
`AGNI_EXAMPLE_DESIGN` to change the default, which `make runquiet` and `make record` pick up too.
