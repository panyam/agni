# Examples — Conventions

Single source of truth for how an Agni example is laid out. Read this before adding a new
example or upgrading an existing one. The reference example is
[`read-and-stats/`](read-and-stats/).

The style follows the demokit examples pattern (a shared `common` package plus one directory
per example, each a narrated walkthrough), adapted to an engine rather than a server: Agni
examples are **single-mode** (there is no server to run, unlike mcpkit's dual-mode
`--serve`), and the narration lives in a **sidecar markdown file**, not in Go string
literals.

## 1. Directory layout

```
examples/
├── CONVENTIONS.md        # this file
├── README.md             # the index (table of examples)
├── common/               # shared reuse package (its own module)
│   ├── go.mod            # module .../examples/common; replace agni => ../..
│   ├── designs.go        # ReadDesign / readByExt: reader dispatch at the I/O edge
│   ├── fixtures.go       # go:embed designs/*; Designs(), ReadFixture()
│   ├── present.go        # narration pretty-printers (StatsLines, NetLines, ...)
│   ├── walkthrough.go    # SetupRenderer: --mode dispatch (plain/tui/notebook)
│   ├── walkthrough.mk    # shared Makefile fragment
│   └── designs/          # bundled synthetic fixtures (see rule in §4)
└── <name>/               # one directory per example
    ├── main.go           # thin: embed the sidecar, Bind the steps that run code
    ├── walkthrough.md     # authored narration (demokit FromMarkdown sidecar)
    ├── README.md          # short: what it shows, how to run, how it is built
    ├── Makefile           # one line: include ../common/walkthrough.mk
    └── go.mod / go.sum    # own module; replaces for agni + common
```

## 2. main.go (thin by design)

Keep behavior in Go and prose in markdown. `main.go` should only:

1. `//go:embed walkthrough.md` and load it via `demokit.New(name).FromMarkdownBytes(md)`.
2. `demo.Bind("<step-id>").Run(...)` for each step that runs engine code. Steps that are
   pure narration need no bind.
3. `common.SetupRenderer(demo)` then `demo.Execute()`.

State shared between steps (e.g. the fixture the user picked) lives in a closure variable
captured by the bound `Run` funcs, defaulted so a `--non-interactive` run stays coherent.

## 3. walkthrough.md (the sidecar)

demokit's `FromMarkdown` decides step vs section by content shape:

- YAML frontmatter sets `title`, `description`, `actors`.
- `## Heading {#id}` starts an item; the `{#id}` is the `Bind` key.
- A blockquote (`> ...`) becomes the step's note.
- A ` ```mermaid ` block with `A ->> B: label` (solid) / `A -->> B: label` (dashed) lines
  becomes sequence-diagram arrows.
- A ` ```inputs ` block (YAML list of `{name, prompt, type, options, default}`) declares
  step inputs. Exception: a design-path input is declared in Go via `common.AskPath(...).Def()`
  (see §4), not here, so the prompt stays identical across examples; the sidecar step then
  carries only its note.
- A heading with any of [blockquote, mermaid, inputs, refs] is a **step**; a prose-only
  heading is a **section**.

Prose obeys the repo writing style (no em-dashes, plain declarative). The demokit renderer
may insert its own separators in generated output; that is out of our control, but the
authored `.md` stays clean.

## 4. Design inputs are paths, asked via `common.AskPath`

When an example asks the user which design to load, the input is a **filesystem path relative
to the example directory**, defaulting to the bundled fixture. Every example asks the same
way, through the shared **`common.AskPath(name, default)`** helper, so the prompt wording and
load semantics never diverge:

```go
design := common.AskPath("design", "../common/designs/foo.edn")
demo.Bind("pick").Input(design.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
    design.Capture(ctx)            // record what the user typed
    return nil
})
demo.Bind("run").Run(func(ctx demokit.StepContext) *demokit.StepResult {
    d, err := design.Load()        // read the chosen design in a later step
    ...
})
```

`Load` (which `design.Load()` calls) reads the path from disk first, so a user can point the
example at their own design, then falls back to the embedded fixture whose base name matches,
so the example still runs from any directory. A file that exists but fails to parse is
surfaced, not masked. List the bundled designs as their relative paths in the asking step's
note so it is clear where they live. This applies to every path an example prompts for.

An example that loads schematic **geometry** rather than the netlist IR uses the same
`AskPath` for the path, but calls `common.LoadSchematic(design.Path())` (disk-first, embedded
fixture fallback, EDIF `.eds` only) instead of `design.Load()`. See `render-schematic`.

## 5. Fixtures

Examples read bundled synthetic fixtures from `common/designs/`, and may also load any path
the user supplies (see §4). The bundled fixtures are hand-authored, not real customer boards,
so every example runs hermetically and the repo stays shareable. **Never bundle a
company-specific or otherwise protected netlist.** Bundle only hand-authored synthetic
fixtures or a design that is redistributable under a license that permits it. New fixtures go
in `common/designs/` and are reached through `common.Load` / `common.ReadFixture(name)` /
`common.Designs()`.

## 6. Modules

Each example and `common` is its own Go module with local `replace` directives (mcpkit
style). This keeps demokit and its terminal-UI dependency tree out of the shippable
`github.com/panyam/agni` go.mod. `go build ./...` at the repo root does not descend into
these nested modules, so the engine build stays clean.

## 7. Makefile targets (from common/walkthrough.mk)

`run` (plain), `demo` (TUI), `note` (notebook), `runquiet` (non-interactive), `record` /
`replay` (deterministic trace), `doc` (render to markdown), `build`.

`make build` writes a binary named after the directory; gitignore it with a one-line
`.gitignore` (`/<name>`) so it never gets committed.
