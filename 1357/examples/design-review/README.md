# design-review

A team's schematic-review checklist, run against a board, with every item resolving to an outcome,
including the ones nothing can answer.

A review checklist is a list of questions. Someone decided to ask each one before a board is signed
off. Most tooling answers some of them and is silent about the rest, so a clean report and an
unasked question look identical. This example makes them look different.

## What it shows

This rung **composes the others**. A checklist item binds to a rule (rung 3), to a query
over the fact relations (rung 11), or to nothing at all:

| step | what it asks | mechanism |
|---|---|---|
| 2 | how much of the list can be answered | the rollup: covered, answered, and by outcome |
| 3 | what a failing item depends on | the rule behind the item, and its findings |
| 4 | an item nobody wrote a rule for | an inline datalog query on the item itself |
| 5 | the items nothing answers | three reasons, and only one is a gap in the tool |

Three items go unanswered, each for a different reason. One rule ran and found no tier to read,
another rule waits on a declaration, and one question has no shipped mechanism covering it.
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
Neither is committed here, because a team's checklist is as unshippable in this repo as
their board.

**Expect a lower covered count here than `agni review` reports on the same two files.** This runs
the BUILT-IN catalog. The CLI composes a project's own overlay on top, and the overlay's interface
profiles, design intent and naming conventions each answer items that this example cannot. On one
real board the CLI covers 123 of 302 items where this covers 76. The gap is the overlay, not a
defect.

## How it is built

`main.go` is thin. The narration is `walkthrough.md`, which demokit reads as the step list. The
bundled checklist is `checklist.yaml`, embedded so the walk runs from any directory.

The checklist is the pedagogy rather than scaffolding. It is sized so each outcome occurs at least
once against the bundled design, and two of them come from what that design LACKS rather than from
what it has, since a board-tier item on a netlist is not-applicable and an intent item with nothing
declared needs a declaration. `needs-data` is deliberately absent. A datasheet item reads
`needs-data` when its corpus exists and lacks the symbol, and showing that state would mean
bundling a seeded corpus for one row.

Three blank imports matter here. The rule catalog and the fact relations fail silently when
missing, and without the **review query compiler** an inline query item cannot compile at all. No
other rung on the ladder imports the compiler, so this example is where you are most likely to
learn about its registration point.
