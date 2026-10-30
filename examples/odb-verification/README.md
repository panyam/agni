# odb-verification

Check a layout that came back from layout as ODB++ against the schematic it was drawn from. The
walkthrough reads the job's netlist and board, runs the checks over both (the copper rules
included), and diffs the layout's connectivity against the schematic netlist. It is the walkthrough
form of `agni check <job>` and `agni diff <schematic> <job>`.

## What it shows

- An ODB++ job read as a folder (a `.tgz` or `.zip` of one works the same), through the same format
  registry every other file goes through, so checks and diff do not know the layout arrived as
  ODB++.
- The board tier built from the job's copper, which the copper-clearance and track-width rules
  read.
- A schematic-against-layout diff. The default layout, the gateway's rev B, adds two I2C pull-ups
  and renames the crystal nets, and `../common/designs/odb/gateway-layout` (rev A) matches its
  schematic exactly.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

`make exercise-odb` at the repo root runs it non-interactively.

## How it is built

The layouts in `../common/designs/odb` are KiCad's own ODB++ exports of the tutorial gateway board,
written by `hack/odb_fixtures.sh`, and `gateway.edn` there is the tutorial's schematic netlist. The
narration lives in [`walkthrough.md`](walkthrough.md), and `main.go` binds the five steps that run
engine code. See [`../CONVENTIONS.md`](../CONVENTIONS.md) for the layout every example follows.
