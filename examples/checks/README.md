# checks

Run Agni's structural rule checks over a design and read the findings. It is the
walkthrough form of `agni check`.

## What it shows

- `check.RunDesign` applied to the neutral IR. Checks are pure IR operations, so an EDIF netlist, a
  KiCad board, and an IPC-2581 export all go through the same rules (CONSTRAINTS C1).
- The three severities: `single-pin-net` (info), `unconnected-component` (warning),
  `i2c-pull-up` (error).
- Some nets look wrong but are intentional, such as an I2C net with a pull-up or a single-pin net
  named as a no-connect. The rules do not flag them. WS3-002 added this no-connect awareness to
  cut false positives.
- A pull-up reach declared in a design's intent. `pullup-reach.edn` has a bus whose pull-up sits
  four series resistors out. That bus fails `i2c-pull-up` at the default reach of 3 and passes
  once `pullup-reach.intent.yaml` declares `pull_up_reach_hops: 4` for that net. The file also has
  a bus with two pull-ups, and its pass lists both paths (agni issues 820 and 821).

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

The default design is `i2c-sensor.edn`, built to trip one finding at each severity. Two
deliberately benign nets go unflagged, SDA with a pull-up and NC_SPARE as a no-connect.
`two-resistors.edn` is the clean contrast (no findings); the KiCad and IPC-2581 boards each
trip one rule.

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's `FromMarkdown`.
`main.go` binds the `pick` and `run` steps to `check.RunDesign` and `common.FindingsLines`. It binds
the `reach` step to `intent.ParseProto`, then `check.WithIntent` on the model the rule evaluates. It
also wires the renderer. The fixture and the finding printer live in [`../common`](../common). See
[`../CONVENTIONS.md`](../CONVENTIONS.md) for the shared layout.
