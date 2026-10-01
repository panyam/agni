# convert

Read any supported source format into the neutral IR, then emit it back out. A conversion is
just `A -> IR -> B` with the IR as the pivot. There are two writers, IPC-2581 and the EDIF
netlist, and the CLI picks one from the output file's extension. This walkthrough drives the
IPC-2581 one, so it converts EDIF (`.edn`) or KiCad (`.kicad_pcb`) into IPC-2581, or round-trips
IPC-2581 into itself.

The walkthrough picks one of three bundled synthetic designs (one per reader), reads it into
the IR, emits IPC-2581, and reads the emitted document straight back. The re-read stats match
the input's, which is the semantic round-trip (geometry lives in a separate sidecar, WS1-006, and
this round-trip covers the IR only).

This is the narrated form of `agni emit <in> [out]`.

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's `FromMarkdown`.
`main.go` binds the `pick`, `to-ir`, `pick-format` and `emit` steps (`common.ReadFixture`,
`ipc2581.Write`, and a re-read of the emitted document) and wires the renderer. See
[`../CONVENTIONS.md`](../CONVENTIONS.md) for the shared layout.
