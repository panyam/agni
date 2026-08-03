# Getting started

How to build `agni`, install the external EDA tools it reads or compares against, and point
it at the symbol libraries that xschem and gEDA schematics need for a full netlist. Once
built, [WEB_WALKTHROUGH.md](WEB_WALKTHROUGH.md) tours the browser viewer over committed
fixtures.

## 1. Build agni

Requires Go 1.26, `pnpm` (the web viewer bundle; run `pnpm install` in `web/` once), and
(only if you change protos) `buf`.

```
make build            # go build ./... (builds the web bundle first)
make test             # engine (Go) tests only
make testall          # the full gate: vet + engine + example modules + web bundle + web unit tests
make install          # install the agni binary to $GOBIN
# or run without installing:
go run ./cmd/agni stats <file>
```

The CLI surface:

```
agni stats  <file>               # component/section/net counts
agni check  <file>               # structural rule checks; --rule/--tag narrow, --format json for tooling
agni diff   <old> <new>          # structural diff between two revisions
agni render <file> -o out.svg    # --layout faithful (default) or an auto-layout (grid, ...);
                                  # --format svg|pack; --report explains the auto-layout mapping
agni emit   <in> [out]           # write IPC-2581 from any readable input (stdout if out omitted)
agni validate <file|dir>...      # reader-health smoke over files or corpus dirs; exits non-zero
                                  # on failures; --format json emits webapi.ValidateReport
agni serve  <dir> --mount n=path # browser viewer + Connect API; --theme default|dark,
                                  # --enable-native <tool> for golden renders
```

The engine and every example module pin `go 1.26.4`; a newer directive than your local toolchain
is auto-fetched (`GOTOOLCHAIN=...+auto`). When regenerating protos, match the committed
`protoc-gen-go` version (`v1.36.11`) so the generated files don't churn their version stamp.

`agni` picks a reader by file extension:

| Extension | Format | Reader |
|-----------|--------|--------|
| `.edn` | EDIF 2.0.0 netlist | `edif` |
| `.kicad_sch` / `.kicad_pcb` / `.kicad_pro` | KiCad | `kicad` |
| `.sch` | xschem **or** gEDA gschem (sniffed by header) | `xschem` / `geda` |
| `.xml` / `.cvg` | IPC-2581 (sniffed for the IPC-2581 root) | `ipc2581` |

The `.sch` extension is shared. `agni` reads the file header to choose: an xschem file opens
with `v {xschem ...`, a gEDA file with `v <release-date> <version>` (e.g. `v 20200319 2`).
Legacy KiCad `.sch` (the pre-KiCad-6 `EESchema` format) is not supported.

## 2. Symbol libraries (needed for xschem/gEDA netlists)

An xschem or gEDA `.sch` file stores components by *reference* to a symbol (`res.sym`,
`resistor-1.sym`, ...); the pin coordinates live in those `.sym` files, not in the schematic.
`agni` needs them to join wires to pins and produce a connected netlist. Without them you
still get every component and every net *name*, but no pin-level connections.

`--symbol-path` also resolves KiCad `.kicad_sym` libraries (WS1-016) for schematics that
do not embed their symbols, including KiCad's own installed set, e.g.
`--symbol-path "/Applications/KiCad/KiCad.app/Contents/SharedSupport/symbols"`. A project
`sym-lib-table` beside the schematic is honored automatically, no flag needed.

Point `agni` at the libraries with `--symbol-path` (repeatable). The schematic's own
directory is always searched, so symbols sitting next to the `.sch` are found automatically.

```
agni stats --symbol-path /usr/share/xschem/xschem_library/devices amp.sch
agni check --symbol-path ~/gEDA/sym --symbol-path /usr/share/gEDA/sym board.sch
```

Where the standard libraries live once the tools below are installed:

| Tool | Symbol library location (typical) |
|------|-----------------------------------|
| xschem | `$PREFIX/share/xschem/xschem_library/devices` (Linux: `/usr/share/xschem/...`) |
| gEDA gschem | `/usr/share/gEDA/sym` (subdirs like `analog/`, `power/`) |
| Lepton EDA | `$PREFIX/share/lepton-eda/sym` |

Note on real-world samples: xschem/gEDA schematics reference their tools' standard-library
symbols, which are not shipped alongside the `.sch` files. Install the matching tool (below)
and pass its symbol path, or the netlist stays names-only. Some designs also use symbols from
the author's own library (e.g. `transistor.sym`, `vdc-1.sym`) that are not in the stock set;
those components will resolve only if you supply that library too.

## 3. Install the source/golden tools

`agni` reads the file formats on its own; you only need these tools to (a) open or export the
designs yourself and (b) run a golden comparison (see section 4). Pick per platform.

### KiCad (`kicad-cli`)

- macOS: `brew install --cask kicad`. `kicad-cli` ships inside the app; add it to PATH, e.g.
  `export PATH="/Applications/KiCad/KiCad.app/Contents/MacOS:$PATH"`.
- Debian/Ubuntu: `sudo apt install kicad` (KiCad 7+ includes `kicad-cli`).
- Verify: `kicad-cli version`.

### xschem

- Debian/Ubuntu: `sudo apt install xschem` (installs the `xschem_library` symbols too).
- macOS: no Homebrew formula. xschem needs Tcl/Tk and X11 (XQuartz), so a source build is
  involved; prefer Docker (section 4) or a Linux box. `nixpkgs#xschem` exists but is
  Linux-only.
- Verify: `xschem --version`.

### gEDA gschem or Lepton EDA

gEDA's `gschem`/`gnetlist` is the classic toolchain; Lepton EDA is the maintained fork with the
same file format and a `lepton-cli` / `lepton-netlist` front end.

- Debian/Ubuntu: `sudo apt install geda-gaf` (gives `gnetlist`, `gschem`, and `/usr/share/gEDA/sym`).
- macOS: no Homebrew formula for either; `nixpkgs#lepton-eda` is Linux-only (unsupported on
  darwin). Use Docker (section 4) or a Linux box.
- Verify: `gnetlist --version` or `lepton-cli --version`.

## 4. Golden comparison

A "golden" is the original tool's own output, used to check `agni`'s result. There are two
kinds.

### Golden render, inside the viewer (`agni serve --enable-native`)

`agni serve` can shell out to a native renderer to draw the *original* file next to agni's
IR-based render. Enable a tool by name (off by default):

```
agni serve --addr :8080 \
  --mount kicad=kicad/testdata \
  --enable-native kicad-cli
```

The tool binary must be on PATH. Registered native renderers, picked by file type (a `.sch` is
sniffed to tell xschem from gEDA):

| File | Tool to enable | Renders with |
|------|----------------|--------------|
| `.kicad_sch` / `.kicad_pcb` / `.kicad_pro` | `kicad-cli` | `kicad-cli sch/pcb export svg` |
| `.sch` (xschem) | `xschem` | `xschem --no_x --quit --svg` (writes `plot.svg`) |
| `.sch` (gEDA) | `lepton-cli` | `lepton-cli export -o ...` |

```
agni serve --addr :8080 --mount amps=./designs \
  --symbol-path /usr/share/xschem/xschem_library/devices \
  --enable-native kicad-cli --enable-native xschem --enable-native lepton-cli
```

Because the xschem and Lepton binaries are Linux/X11 tools (section 3), the `.sch` native
renderers are reachable on a Linux host or through a PATH shim that forwards to a container; on
a bare macOS host use the netlist golden below instead.

Note: you usually do not need the native renderer to *see the drawing*. With `--symbol-path`,
agni's own faithful renderer draws the real symbol artwork (resistor zig-zags, transistor
symbols, etc.) and wires from the `.sym` geometry, no external tool required. KiCad schematics
also render their embedded bitmap images (logos) this way. The native renderer is only for a
pixel-for-pixel golden against the original tool.

### Golden netlist, on the command line

Compare `agni`'s netlist against the tool's own netlister. Run `agni` and the tool on the same
file and diff the connectivity.

xschem (SPICE netlist):

```
xschem -n -s -q --no_x -o . amp.sch      # writes amp.spice
agni stats --symbol-path <xschem_library>/devices amp.sch
```

gEDA (via gnetlist):

```
gnetlist -g geda -o amp.net amp.sch
agni stats --symbol-path /usr/share/gEDA/sym amp.sch
```

### Docker (macOS, or to avoid a local install)

When the tools cannot run natively (xschem needs X11; Lepton/gEDA have no macOS package), run
them in a Linux container. gEDA is packaged in Debian:

```
docker run --rm -v "$PWD:/w" -w /w debian:bookworm bash -c '
  apt-get update -qq && apt-get install -y -qq geda-gaf >/dev/null &&
  gnetlist -g geda -o out.net amp.sch && cat out.net'
```

For xschem, the [IIC-OSIC-TOOLS](https://github.com/iic-jku/iic-osic-tools) image bundles xschem
with its symbol library and runs headless; use it to export a SPICE netlist for comparison. Note
the container has the standard libraries, so a design that referenced them will netlist fully
there even when your host copy did not.

## 5. Quick reference

```
# Structural read (components + net names), no symbols needed:
agni stats amp.sch

# Full netlist (pin-level connections), with a symbol library:
agni stats --symbol-path /usr/share/xschem/xschem_library/devices amp.sch

# Run design-rule checks over the netlist:
agni check --symbol-path <lib> amp.sch

# Diff two revisions (any mix of formats, since diff is on the neutral IR):
agni diff --symbol-path <lib> old.sch new.sch

# Browse a tree of designs in the web viewer:
agni serve --addr :8080 --mount samples=./designs --symbol-path <lib>
```
