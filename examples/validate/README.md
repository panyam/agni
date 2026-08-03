# validate: reader-health invariants

Runs the `validate` package's structural invariants (the engine behind `agni validate`)
over a bundled design: the netlist tier (components and nets exist), the drawing tier
(sheets/placements/wires exist and placements resolve to symbols), and what a failure
reads like.

Run it:

```
make run       # plain text
make demo      # TUI boxes
make runquiet  # non-interactive defaults (CI)
```

Built per [../CONVENTIONS.md](../CONVENTIONS.md): thin `main.go` binding steps, narration
in `walkthrough.md`.
