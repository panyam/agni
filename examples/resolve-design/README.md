# resolve-design

Which design does this file belong to, and which file should analysis actually read? The
walkthrough form of the descriptors behind `agni check <design-folder>` and
`projects/{project}/designs/{design}`.

## What it shows

- Two YAML descriptors, `project.yaml` and `design.yaml`, turning a folder of openable files
  into a design with a declared **entry** and declared **companion** views (CONSTRAINTS C21:
  the netlist is the analysis source, the schematic and board are geometry companions).
- `ProjectService` resolving a file to the design that declares it. The example builds the
  service over a filesystem-backed store pointed at the bundled fixture; a server builds the
  same service over its mounts. Every client asks the service, so none of them can disagree
  about which companion supplies the board.
- A file belonging to no design being a miss, not an error, because that is the ordinary
  state of a mounted folder.
- Why the fix is a declaration rather than a warning: reading the companion and reading the
  entry produce two answers, and neither carries any trace of which file it came from.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

At the first step, give any path inside the bundled project (relative to its root). The
bundled project is [`../common/designs/demo-project`](../common/designs/demo-project): one
`project.yaml` and one design under `designs/mixer/`. Try `designs/mixer/mixer.edn` (the
entry), `designs/mixer` (the design folder), and a path under no design at all.

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's
`FromMarkdown`. `main.go` binds the four steps that run engine code (`pick`, `list`,
`resolve`, `read`) and wires the renderer. Descriptor parsing and discovery sit behind the port in
[`internal/projects`](../../internal/projects), the service in
[`service`](../../service), design loading in [`../common`](../common). See
[`../CONVENTIONS.md`](../CONVENTIONS.md) for the layout every example follows.
