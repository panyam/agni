# CLI reference

The commands and flags a user reaches for. `agni <command> --help` always prints the
authoritative, up-to-date detail; this page is the map. The reader for a file is chosen by
its extension, so you pass the design file directly and never name a format.

## Commands

### `stats <file>`

Print component, section, and net counts for one design. Run it first to confirm the tool
read your design the way you expect before trusting anything downstream.

### `check <file>`

Run the rule catalog and report findings. The workhorse; see
[Checks and reports](checks-and-reports.md).

| flag | what it does |
|---|---|
| `--rule <name>` | run only this rule (repeatable) |
| `--tag <key>=<value>` | run only rules with this tag, e.g. `--tag category=power` (repeatable) |
| `--format <fmt>` | `text` (default), `markdown`, `json`, or `report` |
| `--fail-on <sev>` | exit non-zero when a finding sits at or above `error` / `warning` / `info` |
| `--conventions <file>` | compose a naming-convention config into the run (see [Naming conventions](naming-conventions.md)) |
| `--params <dir>` | load a datasheet parameter set, enabling datasheet-backed rules (see [Datasheets](datasheets.md)) |

### `query <file> <query>`

Search the design as data with an ad-hoc datalog query; each answer prints with its provenance. See
[Querying your design](querying.md).

| flag | what it does |
|---|---|
| `--params <dir>` | a parameter set, to query datasheet facts (`param(...)`) |

### `diff <old> <new>`

Structural diff between two revisions, over the IR. See
[Comparing revisions](comparing-revisions.md).

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

### `emit <in> [out]`

Convert any design the tool reads into an IPC-2581 file (stdout if `out` is omitted).

## Global flags

| flag | what it does |
|---|---|
| `--symbol-path <dir>` | directory to search for symbol files (`.sym` for xschem/gEDA, `.kicad_sym` for KiCad) so schematics that name rather than embed their symbols resolve to pin-level nets and faithful artwork. Repeatable; searched recursively; the schematic's own directory is always searched. |

## Advanced and developer commands

`agni` also has `native` (render/open with the design's own EDA tool), `validate`
(reader-health smoke over many files), `census`, and `derive` (datasheet extraction). These
sit closer to the engine and are covered in the developer docs.

## Which formats are read

The reader is picked by file extension (case-insensitive): EDIF netlists (`.edn`/`.edf`/
`.edif`) and geometry (`.eds`), KiCad (`.kicad_sch`/`.kicad_pcb`/`.kicad_pro`), IPC-2581
(`.xml`/`.cvg`), and `.sch` (xschem / gEDA / legacy KiCad, sniffed by header). A schematic
that references external symbols needs `--symbol-path` (or, for a KiCad project, its
`sym-lib-table`) for full pin-level resolution.
