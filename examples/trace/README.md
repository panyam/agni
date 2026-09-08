# trace

Follow one pin to another through the series parts between them and read the route. It is the
walkthrough form of `agni trace`.

## What it shows

- `check.TracePins` over the neutral IR, returning the outcome and the route from one call, so a
  caller cannot report a connection without the evidence for it.
- Why a path is not a net: a resistor in the middle of a signal splits the net, so the two pins a
  reader sees as one wire are on two nets and no per-net query joins them.
- What the walk crosses and what it will not. Resistors, inductors, ferrites and fuses split a net
  without breaking the path; a capacitor is a DC block; a rail or plane can END a route and is never
  passed through.
- The three outcomes kept apart. A route, no route within the radius, and an endpoint that names
  nothing the design has, which is a failed question rather than a disconnection.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

The default design is `i2c-sensor.edn`, and the default endpoints follow its SDA line from the
sensor to the connector's supply pin, which runs through the pull-up resistor. Point `--from` and
`--to` at any other pins on it, or give the first step a path to your own design.

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's `FromMarkdown`.
`main.go` binds the four steps that run engine code and wires the renderer. The fixture loader lives
in [`../common`](../common). See [`../CONVENTIONS.md`](../CONVENTIONS.md) for the shared layout.

`trace_test.go` holds the walkthrough's claims against the fixture, since prose cannot be checked by
building. One of them is the reason it exists: step 4 tells the reader that this fixture's `VCC` is a
supply by name and not rail-scale by measure, which is a fact about the fixture's fan-out and can
stop being true without anyone editing the prose.
