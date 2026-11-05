# orcad-design

Read an OrCAD Capture design straight from its `.DSN` files, every hierarchical block included, and
run the checks over it. It is the walkthrough form of `agni stats design.DSN` and
`agni check design.DSN`.

## What it shows

- `common.ReadDesign` reading a top `.DSN` and opening each block's `.DSN` that it names, through
  `formats.Loader`.
- Every part named by its occurrence designator, as Capture annotated it across the hierarchy.
- Each page drawn as a sheet through `formats.Loader.FaithfulGeometry`, each block's under the page
  that places it.
- The note `report.HierarchyNote` prints when a block's file is missing, rather than a design that
  reads as smaller than it is.
- The checks over the flattened netlist, printed as a tally per rule.

## The design it reads

Nobody can author a synthetic Capture design without Capture, so this example reads a real one
(CONSTRAINTS C10). The default is Clay R6 by Michael Gubbels, under MIT, which the pinned
`cadence-exports` samples artifact carries. Fetch it once from the repo root:

```bash
make samples-oracle
```

Give the path to your own top `.DSN` at the first step to read that instead. Only Capture's modern
format is read, the one Clay R6's Capture 16.6 wrote. A design in the legacy format is refused.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's `FromMarkdown`.
`main.go` binds the four steps that run engine code (`pick`, `netlist`, `pages`, `check`). See
[`../CONVENTIONS.md`](../CONVENTIONS.md) for the layout every example follows.
