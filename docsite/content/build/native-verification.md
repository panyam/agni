---
title: "Native verification"
description: "Check a reader and its renderers against the format's own EDA tool, used as an independent oracle."
---

Agni's SVG and WebGL renderers are checked against each format's own tool, used as an independent
oracle. This page lists the native tool per format, how to install it on macOS, and the two CLI
commands that drive it.

## The commands

- `agni native render <file> [-o out.svg] [--page N]` renders a design to SVG with its native
  tool and writes it to `-o` (default stdout). Put it next to `agni render <file>` to compare
  Agni's output against the authoritative one.
- `agni native open <file> [--print]` opens a design in its native GUI. `--print` shows the launch
  command instead of running it.

Both dispatch by file extension. The `.sch` extension is sniffed for xschem versus Lepton, the
same rule the readers use. A format with no native tool reports that plainly rather than failing
obscurely.

The CLI does not require `--enable-native`, unlike the `serve` viewer's native mode. That
allowlist guards a shared server that would otherwise shell out on a request. Running the CLI is
itself your consent. The tool still has to be registered for the format and installed on `PATH`.

## Tools by format

<div style="overflow-x:auto">

| Format (extension) | Native tool | macOS install | native render | native open |
|---|---|---|---|---|
| KiCad schematic (`.kicad_sch`, `.kicad_pro`) | `kicad-cli` (+ KiCad.app) | `brew install --cask kicad` | yes (per page) | yes (`open -a KiCad`) |
| KiCad board (`.kicad_pcb`) | `kicad-cli` | (same) | yes (F.Cu/B.Cu/Edge.Cuts overview) | yes |
| xschem (`.sch`) | `xschem` | Linux/X11, on macOS via Docker + XQuartz | yes | yes (X11) |
| gEDA / Lepton (`.sch`) | `lepton-cli` (render), `lepton-schematic` (GUI) | Linux, on macOS via Docker | yes | yes (X11) |
| EDIF (`.edn`, `.eds`) | none open | | no | no |
| IPC-2581 (`.xml`, `.cvg`) | none free on macOS | web visualizer (below) | no | no |
| ODB++ (directory / `.tgz`) | none free on macOS | web visualizer / VM | no | no |

</div>

`kicad-cli` is the only native renderer that runs first-class on macOS. The X11 schematic tools
(xschem, Lepton) install cleanly on Linux. On macOS run them under Docker with XQuartz, or on a
Linux box. The board interchange formats (IPC-2581, ODB++) and EDIF have no free macOS-native
renderer.

## What the tools are run as

The exact invocations, for reference. Agni runs these into a temp dir under a timeout.

- KiCad schematic: `kicad-cli sch export svg --pages N --output DIR file`
- KiCad board: `kicad-cli pcb export svg --mode-single --layers F.Cu,B.Cu,Edge.Cuts --output OUT.svg file`
- xschem: `xschem --no_x --quit --svg file` (writes `plot.svg` in the working directory)
- Lepton: `lepton-cli export -o OUT.svg file`

## Docker plus SSH: the X11 tools without a local install

{{ includeFile "figures/native-tool-container.svg" }}

xschem and Lepton are Linux/X11 tools with no macOS package. `Dockerfile.nattools` builds a
Debian tool host with kicad-cli, xschem, Lepton, and `agni`, reached over SSH. The Agni server
still runs on the host. This container only supplies the tools. `agni native render` runs inside
it and writes the SVG back to the bind-mounted workspace, and `ssh -X` carries an
`agni native open` GUI to the host's XQuartz.

The engine repo's Makefile runs the container lifecycle (`natup`, `natdown`, `natlogs`). A
workspace Makefile can add file-driven `natrender` and `natopen` wrappers that bind-mount your
design tree at `/hw` and pick an output dir. The raw `ssh` forms below do the same and work
anywhere.

```
make natup                                                   # sshd container up (this repo)
# render/open, as a workspace wrapper would run them against a mounted design tree:
#   make natrender FILE=path/to/design.sch OUT=design.svg    # -> <mounted-dir>/design.svg
#   make natopen  FILE=path/to/design.sch                    # GUI via XQuartz
make natdown
```

Under the hood these are `ssh -p 2222 agni@localhost agni native render /hw/... -o /hw/...` and
`ssh -X ... agni native open /hw/...`. Notes:

- `natopen` needs XQuartz running (`brew install --cask xquartz`, then launch it) and a `DISPLAY`
  set in the shell. Smoke test the forwarding with
  `ssh -X -p 2222 -o SetEnv=LC_ALL=C.UTF-8 agni@localhost xeyes`. A pair of eyes should appear.
- `agni native open` blocks until the window closes. The `ssh -X` tunnel has to stay up.
- `LC_ALL=C.UTF-8` is forced over ssh. Lepton's guile front end aborts on an unavailable forwarded
  locale, and the slim image has none.

Both `xschem` (2.8.x) and `lepton-eda` (`lepton-cli`, `lepton-schematic`) come from Debian
bookworm apt, so no source build is needed. Building the image was the first real exercise of the
xschem render path. Version 2.8.x has no `--plotfile`. It writes `plot.svg` into the working
directory, which agni sets to a temp dir.

## Verifying formats with no native renderer

EDIF, IPC-2581, and ODB++ have no free macOS-native tool, so board and netlist correctness is
established without one.

1. In-file oracle. IPC-2581 authors component placements and copper pad lands on independent
   channels. A correct placement puts every pin on its copper. Measuring that distance is a ground
   truth the file carries itself, stronger than a third-party render, which re-derives from the
   same placement data. This is how the placement geometry was pinned.
2. kicad-cli cross-check. Where a KiCad equivalent exists, `kicad-cli pcb drc` gives a numeric
   oracle. The board DRC counts were cross-checked this way.
3. Web visualizer. For a visual sanity pass, a browser IPC-2581/ODB++ viewer (Eurocircuits PCB
   Visualizer, or the BoardUI parser run locally) renders the same file.
4. Vendor viewer in a VM. For pixel truth, the Siemens ODB++ Viewer or an IPC-2581 viewer on
   Linux or Windows.
