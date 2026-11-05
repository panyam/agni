# allegro-board

Read a Cadence Allegro board straight from its `.brd`, with no ODB++ or IPC-2581 export, and run the
checks over it. It is the walkthrough form of `agni stats board.brd` and `agni check board.brd`.

## What it shows

- `common.ReadDesign` and `common.ReadBoard` reading one `.brd` both ways, as a netlist and as board
  geometry, through `formats.Loader`.
- The copper layers renamed into the contract's `F.Cu` to `B.Cu` vocabulary, and bottom parts
  mirrored, so the board reads the way every other board format does.
- The checks run with the board in the model, so the copper rules (`track-width`,
  `copper-clearance`) judge it too. The findings print as a tally per rule, since a real board
  yields hundreds.

## The board it reads

Nobody can author a synthetic Allegro board without Allegro, so this example reads a real one
(CONSTRAINTS C10). The default is the BeagleBone AI from BeagleBoard.org, under CC-BY-4.0, which the
pinned `allegro` samples artifact carries. Fetch it once from the repo root:

```bash
make samples-oracle
```

Give the path to your own `.brd` at the first step to read that instead. Only boards saved by
Allegro 17.2 are read so far.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's `FromMarkdown`.
`main.go` binds the four steps that run engine code (`pick`, `netlist`, `board`, `check`), and
`summary.go` holds the two printers the test also calls. See
[`../CONVENTIONS.md`](../CONVENTIONS.md) for the layout every example follows.
