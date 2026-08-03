# Native verification: rendering a design with its own EDA tool

Agni's renderers (SVG and WebGL) are checked against each format's *own* tool, used as an
independent oracle. This doc lists the native tool per format, how to install it on macOS,
and the two CLI commands that drive it. For the golden-comparison workflow, see
[GETTING_STARTED.md](GETTING_STARTED.md).

## The commands

- `agni native render <file> [-o out.svg] [--page N]`: render a design to SVG with its
  native tool and write it to `-o` (default stdout). Put it next to `agni render <file>` to
  compare agni's output against the authoritative one.
- `agni native open <file> [--print]`: open a design in its native GUI. `--print` shows the
  launch command instead of running it.

Both dispatch by file extension (the `.sch` extension is sniffed for xschem vs Lepton, the
same rule the readers use). A format with no native tool reports that plainly rather than
failing obscurely.

Unlike the `serve` viewer's NATIVE mode, the CLI does not require `--enable-native`: that
allowlist is a guard for a shared server that would otherwise shell out on a request, while
running the CLI is itself your consent. The tool must still be registered for the format and
installed on `PATH`.

## Tools by format

| Format (extension) | Native tool | macOS install | `native render` | `native open` |
|---|---|---|---|---|
| KiCad schematic (`.kicad_sch`, `.kicad_pro`) | `kicad-cli` (+ KiCad.app) | `brew install --cask kicad` | yes (per page) | yes (`open -a KiCad`) |
| KiCad board (`.kicad_pcb`) | `kicad-cli` | (same) | yes (F.Cu/B.Cu/Edge.Cuts overview) | yes |
| xschem (`.sch`) | `xschem` | Linux/X11; on macOS via Docker + XQuartz | yes | yes (X11) |
| gEDA / Lepton (`.sch`) | `lepton-cli` (render), `lepton-schematic` (GUI) | Linux; on macOS via Docker | yes | yes (X11) |
| EDIF (`.edn`, `.eds`) | none open |, | no | no |
| IPC-2581 (`.xml`, `.cvg`) | none free on macOS | web visualizer (below) | no | no |
| ODB++ (directory / `.tgz`) | none free on macOS | web visualizer / VM | no | no |

`kicad-cli` is the only native renderer that runs first-class on macOS. The X11 schematic
tools (xschem, Lepton) install cleanly on Linux; on macOS run them under Docker with XQuartz,
or on a Linux box. The board interchange formats (IPC-2581, ODB++) and EDIF have no free
macOS-native renderer.

## What the tools are run as

For reference, the exact invocations (agni runs these into a temp dir under a timeout):

- KiCad schematic: `kicad-cli sch export svg --pages N --output DIR file`
- KiCad board: `kicad-cli pcb export svg --mode-single --layers F.Cu,B.Cu,Edge.Cuts --output OUT.svg file`
- xschem: `xschem --no_x --quit --svg file` (writes `plot.svg` in the working directory)
- Lepton: `lepton-cli export -o OUT.svg file`

## Docker + SSH: the X11 tools without a local install

xschem and Lepton are Linux/X11 tools with no macOS package. `Dockerfile.nattools` builds a
Debian tool host with kicad-cli, xschem, Lepton, and `agni`, reached over SSH. The agni
*server* still runs on the host; this container only supplies the tools. `agni native
render` runs inside it and writes the SVG back to the bind-mounted workspace, and `ssh -X`
carries an `agni native open` GUI to the host's XQuartz.

This repo's Makefile runs the container lifecycle (`natup`/`natdown`/`natlogs`). A workspace
Makefile can add file-driven `natrender`/`natopen` wrappers that bind-mount your design tree at
`/hw` and pick an output dir; the raw `ssh` forms below do the same and work anywhere.

```
make natup                                                   # sshd container up (this repo)
# render/open, as a workspace wrapper would run them against a mounted design tree:
#   make natrender FILE=path/to/design.sch OUT=design.svg    # -> <mounted-dir>/design.svg
#   make natopen  FILE=path/to/design.sch                    # GUI via XQuartz
make natdown
```

Under the hood these are `ssh -p 2222 agni@localhost agni native render /hw/... -o /hw/...`
and `ssh -X ... agni native open /hw/...`. Notes:
- **`natopen` needs XQuartz running** (`brew install --cask xquartz`, then launch it) and a
  `DISPLAY` set in the shell. Smoke test the forwarding with `ssh -X -p 2222
  -o SetEnv=LC_ALL=C.UTF-8 agni@localhost xeyes`: a pair of eyes should appear.
- `agni native open` blocks until the window closes (the `ssh -X` tunnel must stay up).
- `LC_ALL=C.UTF-8` is forced over ssh: Lepton's guile front end aborts on an unavailable
  forwarded locale, and the slim image has none.

Both `xschem` (2.8.x) and `lepton-eda` (`lepton-cli`, `lepton-schematic`) come from Debian
bookworm apt, so no source build is needed. Building the image was the first real exercise of
the xschem render path (2.8.x has no `--plotfile`; it writes `plot.svg` into the working
directory, which agni sets to a temp dir).

## Verifying formats with no native renderer

EDIF, IPC-2581, and ODB++ have no free macOS-native tool, so board/netlist correctness is
established without one:

1. **In-file oracle.** IPC-2581 authors component placements and copper pad lands on
   independent channels; a correct placement puts every pin on its copper. Measuring that
   distance is a ground truth the file carries itself, stronger than a third-party render
   (which re-derives from the same placement data). This is how the placement geometry was
   pinned (WS1-029/030).
2. **kicad-cli cross-check.** Where a KiCad equivalent exists, `kicad-cli pcb drc` gives a
   numeric oracle (the board DRC counts were cross-checked this way).
3. **Web visualizer.** For a visual sanity pass, a browser IPC-2581/ODB++ viewer
   (Eurocircuits PCB Visualizer, or the BoardUI parser run locally) renders the same file.
4. **Vendor viewer in a VM.** For pixel truth, the Siemens ODB++ Viewer or an IPC-2581 viewer
   on Linux/Windows.
