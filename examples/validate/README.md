# validate

Runs the `validate` package's reader-health invariants (the engine behind `agni validate`)
over a bundled design, covering the netlist tier (components and nets exist) and the drawing
tier (sheets/placements/wires exist and placements resolve to symbols), and shows what a
failure reads like.

## Run it

```
make run       # plain text
make demo      # TUI boxes
make runquiet  # non-interactive defaults (CI)
make doc       # render the walkthrough to markdown
```

## How it is built

Built per [../CONVENTIONS.md](../CONVENTIONS.md), with a thin `main.go` binding the steps and
the narration in `walkthrough.md`. `main.go` binds the four steps that run engine code (`pick`,
`netlist`, `geometry`, `failure`) and wires the renderer.
