# altium-board

Read an Altium Designer board straight from its binary `.PcbDoc`, with no ODB++ or IPC-2581 export,
and run the checks over it. It is the walkthrough form of `agni stats board.PcbDoc` and
`agni check board.PcbDoc`.

## What it shows

- `common.ReadDesign` and `common.ReadBoard` reading one `.PcbDoc` both ways, as a netlist and as
  board geometry, through `formats.Loader`.
- The board's own designators, and the schematic parameters each part keeps (part number,
  description), which an ODB++ export drops.
- The checks run with the board in the model, so the copper rules judge it too. The findings print as
  a tally per rule.

## The board it reads

Nobody can author a synthetic Altium board without Altium, so this example reads a real one
(CONSTRAINTS C10). The default is the foxBMS master board by Fraunhofer IISB, under CC-BY-4.0 (this
uses parts of foxBMS), which the pinned `altium` samples artifact carries. Fetch it once from the repo
root:

```bash
make samples-oracle
```

Give the path to your own binary `.PcbDoc` at the first step to read that instead.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's `FromMarkdown`.
`main.go` binds the four steps that run engine code (`pick`, `netlist`, `board`, `check`), and the
two printers it shares with `allegro-board` live in `../common`. See
[`../CONVENTIONS.md`](../CONVENTIONS.md) for the layout every example follows.
