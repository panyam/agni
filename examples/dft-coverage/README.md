# dft-coverage

Design for test: which nets a probe can reach, and which parts a tester can actually measure.

A board can be electrically correct and impossible to debug. A **test point** is a bare pad whose only
job is to expose a net so something can touch it, a scope probe at bring-up or a bed-of-nails in the
factory. This example asks what that layer covers.

## What it shows

Steps 2 to 5 are **queries over the design's fact relations**, not rule checks. It is the
first example in the ladder to run a query, so it doubles as a tour of derived relations, aggregates
and negation:

| step | what it asks | query shape |
|---|---|---|
| 2 | what is on the board | `=> ?k, count(?c)` |
| 3 | which nets a probe can reach | `net.has_test_point`, negated over every net |
| 4 | which parts a tester can measure | `component.probed_both`, `component.probed_one`, negation |
| 5 | the unmeasurable parts, by part number | negation, `count(distinct)`, `list(distinct)` |
| 6 | what a coverage report cannot say | `check.RunVerdicts` |

In step 4, measuring a two-terminal part needs BOTH ends reachable, so one end covered measures
nothing. The three buckets (both ends, one end, neither) come from the shipped library, whose
members each have a reference page, starting with
[component.probed_both](https://panyam.github.io/agni/reference/relations/component.probed_both/).
The walk defines only what it counts as a passive, a resistor or a capacitor, which is narrower than
the library's two-terminal parts.

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

`main.go` is thin, embedding `walkthrough.md` and binding only the steps that run engine code. The prose
lives in the sidecar. See [../CONVENTIONS.md](../CONVENTIONS.md).

Two details matter if you copy this example.

**`component.mpn` needs no datasheet corpus.** Every model joins the part numbers the design carries
(a BOM line, else the component's own MPN), so `check.NewModel(d)` is enough for step 5. Only the
datasheet relations need specs, through `check.WithParamProvider`. Before agni issue 748 this took a
separate constructor, and building the model the other way silently emptied the relation.

**The rule catalog is a blank import.** `check.BuiltinRules()` returns nothing without
`_ "github.com/panyam/agni/stdlib/rules/builtin"`, and it fails silently, so step 6 read "0 pass, 0 fail,
across 0 rules" until the import was added.

## The design

The bundled fixture `../common/designs/probe-coverage.edn` is synthetic and small, built so each
coverage case occurs exactly once: one passive with both ends probed, three with one end, two with
neither. The two with neither share a part number, so the grouping in step 5 has something to find.
GND deliberately carries no test point.

Point it at your own board by typing a path at the first step, or by setting
`AGNI_EXAMPLE_DESIGN` to change the default, which `make runquiet` and `make record` pick up too.
