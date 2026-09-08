# design-review

A team's schematic-review checklist, run against a board, with every item resolving to an outcome
including the ones nothing can answer.

A review checklist is a list of questions someone decided to ask before a board is signed off. Most
tooling answers some of them and is silent about the rest, so a clean report and an unasked question
look identical. This example makes them look different.

## What it shows

It is the rung that **composes the others**. A checklist item binds to a rule (rung 3), to a query
over the fact relations (rung 11), or to nothing at all:

| step | what it asks | mechanism |
|---|---|---|
| 2 | how much of the list can be answered | the rollup: covered, answered, and by outcome |
| 3 | what a failing item rests on | the rule behind the item, and its findings |
| 4 | an item nobody wrote a rule for | an inline datalog query on the item itself |
| 5 | the items nothing answers | three reasons, only one of which is a gap in the tool |

Step 5 is why it exists. Three items go unanswered for three different reasons: a rule that ran and
found no tier to read, a rule waiting on a declaration, and a question no shipped mechanism covers.
Folding those into "not passing" would say the same thing about a missing rule and a missing
datasheet.

## Run it

    make run          # plain text
    make demo         # TUI boxes
    make runquiet     # non-interactive defaults, CI-safe
    make doc          # render to markdown

## Point it at your own work

    AGNI_EXAMPLE_DESIGN=/abs/path/board.edn AGNI_EXAMPLE_REVIEW=/abs/path/review.yaml make runquiet

Both variables replace a DEFAULT, so the prompt still shows the path and a typed path still wins.
Neither is committed here, which is the point: a team's checklist is as unshippable in this repo as
their board.

**Expect a lower covered count here than `agni review` reports on the same two files.** This runs the
BUILT-IN catalog; the CLI composes a project's own overlay on top, so its interface profiles, its
design intent and its naming conventions each answer items this cannot. On one real board the CLI
covers 123 of 302 items where this covers 76. That is the overlay being visible rather than a defect,
and it is worth knowing before you read the difference as one.

## How it is built

`main.go` is thin. The narration is `walkthrough.md` (demokit reads it as the step list) and the
bundled checklist is `checklist.yaml`, embedded so the walk runs from any directory.

The checklist is the pedagogy rather than scaffolding. It is sized so each outcome occurs at least
once against the bundled design, and two of them come from what that design LACKS rather than from
what it has: a board-tier item on a netlist is not-applicable, and an intent item with nothing
declared needs a declaration. `needs-data` is deliberately absent, because it is the state of a
datasheet item whose corpus exists and lacks the symbol, and showing it would mean bundling a seeded
corpus for one row.

Four blank imports matter here, and three of them fail silently when missing. The rule catalog, the
fact relations, and the **review query compiler**, without which an inline query item cannot compile
at all. That last one is the seam this example is most likely to teach you about, because it is the
only surface that needs it.
