---
title: "CLI reference"
description: "The commands and flags a user reaches for, with a pointer to per-command help."
---

The commands and flags a user reaches for. `agni <command> --help` always prints the
authoritative, up-to-date detail. This page is the map. The reader for a file is chosen by
its extension, so you pass the design file directly and never name a format.

Where a `<file>` is expected you may also name a **design folder** — one holding a `design.yaml`
that declares which file is the design's entry. Agni then reads that entry and picks up the
companion views the descriptor lists, so a netlist's connectivity rules and a board's copper rules
run from one argument. See [Projects and designs](../../architecture/projects-and-designs/).

## Commands

### `stats <file>`

Print component, section, and net counts for one design. Run it first to confirm the tool
read your design the way you expect before trusting anything downstream.

### `check <file>`

Run the rule catalog and report findings. The workhorse. See
[Checks and reports](../checks-and-reports/).

| flag | what it does |
|---|---|
| `--rule <name>` | run only this rule (repeatable) |
| `--tag <key>=<value>` | run only rules with this tag, e.g. `--tag category=power` (repeatable) |
| `--format <fmt>` | `text` (default), `markdown`, `json`, or `report` |
| `--fail-on <sev>` | exit non-zero when a finding sits at or above `error` / `warning` / `info`. This is the **severity** axis. For the coverage axis, see `review --fail-on-outcome` and `--min-answered` below |
| `--conventions <file>` | compose a naming-convention config into the run (see [Naming conventions](../naming-conventions/)) |
| `--profile-path <dir>` | compose a directory of YAML interface-profile declarations into the catalog, namespaced `profile-overlay/` (see [Interface profiles](../interface-profiles/)) |
| `--params <dir>` | load a datasheet parameter set, enabling datasheet-backed rules (see [Datasheets](../datasheets/)) |

### `review <file>...`

Run a review checklist over one or more designs and report one outcome per item. Where `check` asks
"what is wrong with this board", `review` answers "which of our questions did we actually answer".
Its outcome vocabulary distinguishes a check that passed from one that never ran.

| flag | what it does |
|---|---|
| `--checklist <file>` | the review manifest (YAML) declaring areas and their items. Optional when the design belongs to a project that declares one; passing it overrides the project's |
| `--conventions <file>` | a naming-convention config, whose rules join the catalog and whose lexicon reaches the design read |
| `--profile-path <dir>` | interface-profile declarations added to the catalog |
| `--params <dir>` | a datasheet parameter set, enabling datasheet-backed items |
| `--intent-path <file>` | a design-intent declaration, so intent-bound items resolve instead of reading `needs-design-intent` |
| `--board-path <file>` | a board-geometry file attached to a netlist design, so board-tier items resolve instead of `n/a` |
| `--coverage` | a per-area rollup of how many items each area decided, instead of the per-item report |
| `--ratified-floor <n>` | datasheet-confidence floor below which a fail reports as `provisional` (default 0.9) |
| `--fail-on-outcome <list>` | exit non-zero when any item sits at one of these outcomes, e.g. `fail` or `fail,provisional`. Off by default |
| `--min-answered <n>` | exit non-zero when fewer than `n` items produced an answer. Off by default |
| `--format <fmt>` | `markdown` (default) or `json` |
| `--results-out <file>` | also write the run as a self-contained check-result document |
| `--render <dir>` | also write an annotated schematic SVG per design, each finding highlighted in place |
| `--companion <file>` | a geometry file to draw `--render` images on, joined to netlist findings by net name |

#### Gating a pipeline on a review

`check --fail-on` and the two flags above gate on different axes, and the difference is not a matter
of taste. `--fail-on` pivots on finding **severity**, which states how bad an answer was.
`--fail-on-outcome` and `--min-answered` pivot on item **outcome**, which states whether the question
was answered at all. A checklist can stop answering four of its items with its failure count unchanged
at zero, and no severity predicate can see that.

`--min-answered` counts the items that produced an answer: `pass`, `fail`, `provisional`, and
`computed-n/a`. It is deliberately stricter than the covered count the report also shows. Covered
subtracts only `not-automated`, which moves when a rule leaves the catalog; it does not move when a
rule is present and its inputs are gone. A datasheet-backed item whose corpus moved reads
`not-applicable`, which still counts as covered and does not count as answered. That gap is the
regression worth gating on.

A `provisional` does not trip `--fail-on-outcome fail`. It is a failure resting on mock or
below-floor datasheet data, so gating on it by default fails a pipeline on data quality rather than on
design quality. Name it explicitly when you want it: `--fail-on-outcome fail,provisional`.

With several designs the gate reads every one of them, and the first design to violate it stops the
run. Exit codes:

| code | meaning |
|---|---|
| `0` | the run completed and no gate tripped |
| `2` | a gate tripped |
| `1` | the run itself failed: an unreadable design, an invalid manifest, an unknown outcome name |

`check` still returns `1` for both a tripped gate and a failed run.

### Machine configuration: `agni.yaml`

The flags that say *where bytes are* can live in a file instead of on every command. `agni.yaml` is
searched for beside the working directory, upward a few levels, then in `$XDG_CONFIG_HOME/agni/`
(or `~/.config/agni/`). The first one found wins outright.

```yaml
# agni.yaml
mounts:
  boards: /srv/boards
  shared: /srv/shared
symbol_paths:
  - /usr/share/kicad/symbols
```

A run says which file it took config from, on stderr. An explicit `--mount` or `--symbol-path` wins
outright rather than merging: naming a mount table is answering for the whole table.

**It carries only tier-1 config, and that is a boundary rather than a to-do.** Naming conventions,
interface profiles, seeded parameters, design intent and a review checklist decide *what a design is
checked against*, so they belong to a project where they are scoped to the designs that declared them.
A machine-wide conventions file applying to every design a CLI opened is the bug per-design config
fixed. Unknown keys are rejected, so reaching for `conventions:` here is told no rather than quietly
becoming a global analysis tier. For those, see [Projects and designs](https://panyam.github.io/agni/architecture/projects-and-designs/).

### `start <design-file> [dir]`

Scaffold a review project around an existing design file, so the commands above can stop taking
flags. `dir` defaults to the current directory.

```
agni start boards/gateway.edn ./gateway-review
```

```
gateway-review/
├── project.yaml            declares the project's id
├── conventions.yaml        stub — your team's naming vocabulary
├── review.yaml             seeded from the shipped catalog
└── designs/gateway/
    ├── design.yaml         names the entry and its companion views
    ├── gateway.edn         copied
    └── gateway.kicad_pcb   copied, declared as a companion
```

After it, `agni check gateway-review/designs/gateway` and `agni review …` resolve the whole
configuration from the descriptors.

| flag | what it does |
|---|---|
| `--name <id>` | the project's declared id; defaults to the target directory's name, lowercased with anything outside `[a-z0-9._-]` replaced by `-` |
| `--title <text>` | the human-readable label; defaults to the id |

**The design is copied, and the project owns its copy.** Editing the original afterwards does not
reach the project. The command prints what it copied and from where, and the generated `design.yaml`
records the origin in a comment.

**Companions are detected, not guessed at.** A sibling is declared a companion only when it shares
the design's stem *and* carries schematic geometry or a board. That excludes a later revision
(`gateway-rev-b.edn`, a different stem and a legitimate analysis source of its own) and a second
netlist encoding (`gateway.edf`, same stem but no view to contribute). Check the generated
`design.yaml` and edit it: membership is declared per file precisely because it cannot be inferred
reliably.

**Nothing is overwritten.** An existing file stops the command and is named, and every planned write
is checked before any write happens, so a refusal leaves nothing half-created. Pointing at a folder
that already holds a `project.yaml` **adds** the design to that project rather than nesting a second
one.

### `intake <file>`

Extract a sanitized summary of a design: counts, class census, rail voltages, anomalies, and the
parts list. It carries the shape of the design and structurally cannot carry a net name or a
connection, so it is safe to hand to someone who should not see the design itself.

| flag | what it does |
|---|---|
| `--params <dir>` | a parameter set, which populates the MPN and datasheet-gap columns |
| `--parts <view>` | `types` (BOM by distinct part type, default) or `full` (per-component AVL) |
| `--format <fmt>` | `md` (default) or `json` |

### `results <file>`

Render a check-result document written earlier by `check --results-out` or `review --results-out`.
The document is self-contained, so this works with the design deleted.

| flag | what it does |
|---|---|
| `--format <fmt>` | the same output formats the live run offers |
| `--compare <file>` | compare against another results document and print the three-way entity split instead of a report |

### `import-results <report.json>`

Read another tool's check report (a `kicad-cli` DRC or ERC JSON report) as a check-result document,
so it can be rendered and compared against a run of this engine with `results --compare`.

### `query <file> <query>`

Search the design as data with an ad-hoc datalog query. Each answer prints with its provenance. See
[Querying your design](../querying/).

| flag | what it does |
|---|---|
| `--params <dir>` | a parameter set, to query datasheet facts (`param(...)`) |
| `--conventions <file>` | apply a naming convention's LEXICON to the read, so `rail`/`feedback`/`pin.type` answer under your project's vocabulary (see [Naming conventions](../naming-conventions/)). The rules half is unused here: a query runs no rules |
| `--board-path <file>` | attach a separate board export so the `board.*` relations have facts; without it they are empty |

### `diff <old> <new>`

Structural diff between two revisions, over the IR. See
[Comparing revisions](../comparing-revisions/).

| flag | what it does |
|---|---|
| `--format <fmt>` | `text` (default human summary) or `json` |

### `render <file>`

Draw a design's schematic or board view.

| flag | what it does |
|---|---|
| `--layout <name>` | `faithful` (default, the design's own geometry) or an auto-layout (`force`, `grid`, `layered`, `orthogonal`, `stress`) computed from the netlist |
| `--format <fmt>` | `svg` (default) or `pack` (for the WebGL viewer) |
| `-o <file>` | output path |

### `serve [dir]`

Host the browser viewer and the web API on one port. Build the web bundle first.

| flag | what it does |
|---|---|
| `--addr <addr>` | listen address (default `:8080`) |
| `--mount <name>=<path>` | expose a design folder in the file browser (repeatable) |
| `--theme <name>` | render palette: `default` or `dark` |
| `--profile-path <dir>` | compose interface profiles into the catalog every rule-running surface uses, the check panel included (see [Interface profiles](../interface-profiles/)) |
| `--review-store <dir>` | a writable directory that keeps review runs, created if absent; without it the review endpoints report that this server stores none (see [Running the server](../running-the-server/)) |

### `emit <in> [out]`

Convert any design the tool reads into an IPC-2581 file (stdout if `out` is omitted).

### `version`

Print this build's identity: the version, the commit it was built from, and the Go toolchain and
platform. `agni --version` prints just the first line.

This is the same string a results document records as its producer, so a report archived months
ago and the build in front of you can be compared directly. Worth capturing alongside any report
you keep.

```
agni v0.1.1
  built:    2026-08-10T14:46:21Z
  go:       go1.26.4
  platform: darwin/arm64
```

A build from a source clone reports the commit rather than a release (`b020fea02643`, suffixed
`+dirty` when the tree had uncommitted changes), because that is what it honestly is.

## Global flags

| flag | what it does |
|---|---|
| `--symbol-path <dir>` | directory to search for symbol files (`.sym` for xschem/gEDA, `.kicad_sym` for KiCad) so schematics that name rather than embed their symbols resolve to pin-level nets and faithful artwork. Repeatable, searched recursively, and the schematic's own directory is always searched. Defaults to `AGNI_SYMBOL_PATH` (colon-separated) when the flag is absent, which is how the container image supplies its bundled libraries to every subcommand. |
| `--mount <name>=<path>` | expose a folder as a named mount (repeatable). Every command takes it, not just `serve`. An artifact is addressed as `mount://<mount>/<path>`, and with a matching `--mount` the CLI and a server produce **identical URIs for the same design**, so a review created either way records the same design and the two are directly comparable. Without it the CLI mints a mount per argument, rooted at the enclosing project when there is one, so `agni check some/board.edn` still works with no configuration. You may also pass a full `mount://...` URI in place of a path. |
| `--as-named` | read exactly the file named, even when its `design.yaml` declares it a companion view of a different entry. Without it, analysing a declared companion (a schematic export, a board) reads the design's entry instead, because a companion is a view of the design rather than a second source of it. Reach for this when reading a view as a netlist is the point, such as checking that two views of one design still agree. |
| `--version` | print the build version and exit. |

## Advanced and developer commands

`agni` also has `native` (render/open with the design's own EDA tool), `validate`
(reader-health smoke over many files), `census`, and `derive` (datasheet extraction). `validate` is
worth knowing as a user: point it at a folder of exports and it reports which of them this tool can
actually read. The rest sit closer to the engine and are covered in the developer docs.

## Which formats are read

The reader is picked by file extension (case-insensitive): EDIF netlists (`.edn`/`.edf`/
`.edif`) and geometry (`.eds`), KiCad (`.kicad_sch`/`.kicad_pcb`/`.kicad_pro`), IPC-2581
(`.xml`/`.cvg`), and `.sch` (xschem / gEDA / legacy KiCad, sniffed by header). A schematic
that references external symbols needs `--symbol-path` (or, for a KiCad project, its
`sym-lib-table`) for full pin-level resolution.
