# trace

Follow one pin to another through the series parts between them and read the route. It is the
walkthrough form of `agni trace`.

## What it shows

- `check.TracePins` over the neutral IR, returning the outcome and the route from one call, so a
  caller cannot report a connection without the evidence for it.
- Why a path is not a net, since a resistor in the middle of a signal splits the net, so the two
  pins a reader sees as one wire are on two nets and no per-net query joins them.
- What the walk crosses and what it will not. It crosses resistors, inductors, ferrites and fuses,
  which split a net without breaking the path. A capacitor is a DC block, and a rail or plane can END
  a route but is never passed through.
- The three outcomes kept apart: a route, no route within the radius, and an endpoint that names
  nothing the design has, which is a failed question rather than a disconnection.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

The default design is the bundled `i2c-sensor`, a declared design whose folder holds the netlist as
its entry and a drawn schematic as a companion. The default endpoints follow its SDA line from the
sensor to the connector's supply pin, which runs through the pull-up resistor. Point `--from` and
`--to` at any other pins on it, or give the first step a path to your own design.

The last step writes `route.svg`, the route drawn on that schematic. From the CLI the same thing is
`agni trace ../common/designs/i2c-sensor --from U1.3 --to J1.1 --render route.svg`, which finds the
schematic through the design's descriptor and falls back to an auto-layout for a design that has
none. To browse the whole design instead, `agni open ../common/designs/i2c-sensor`.

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's `FromMarkdown`.
`main.go` binds the six steps that run engine code and wires the renderer. The fixture loader lives
in [`../common`](../common). See [`../CONVENTIONS.md`](../CONVENTIONS.md) for the shared layout.

`trace_test.go` holds the walkthrough's claims against the fixture, since prose cannot be checked by
building. It exists mainly for step 4, which tells the reader that this fixture's `VCC` is a supply
by name and not rail-scale by measure. That is a fact about the fixture's fan-out and can stop being
true without anyone editing the prose.
