# checks

Run Agni's structural rule checks over a design and read the findings. It is the
walkthrough form of `agni check`.

## What it shows

- `check.Run` applied to the neutral IR. Checks are pure IR operations, so an EDIF netlist, a
  KiCad board, and an IPC-2581 export all go through the same rules (CONSTRAINTS C1).
- The three severities: `single-pin-net` (info), `unconnected-component` (warning),
  `i2c-pull-up` (error).
- No-connect awareness: a net that looks wrong but is intentional (an I2C net with a pull-up,
  or a single-pin net named as a no-connect) is not flagged. This is the false-positive
  reduction from WS3-002.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

The default design is `i2c-sensor.edn`, built to trip one finding at each severity while
leaving two deliberately-benign nets (SDA with a pull-up, NC_SPARE a no-connect) unflagged.
`two-resistors.edn` is the clean contrast (no findings); the KiCad and IPC-2581 boards each
trip one rule.

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's
`FromMarkdown`. `main.go` binds the `pick` and `run` steps (`check.Run` + `common.FindingsLines`)
and wires the renderer. The fixture and the finding printer live in
[`../common`](../common). See [`../CONVENTIONS.md`](../CONVENTIONS.md) for the shared layout.
