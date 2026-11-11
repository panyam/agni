# readers

This file covers the format readers as a group, and the `formats.Loader` passes that every read goes through.

- **A host goes through `formats.Loader`, never a bare reader.** The format-neutral passes run in the Loader. Under a bare reader `classify.StampMPN` never fires, so every component's `mpn` is empty and the whole datasheet tier is empty. No error says so (agni 228). The examples are Go modules of their own, outside the root build, so `examples/common` can repeat this mistake (agni 618).
- **A locator records the path within the design's mount, never the host path.** Readers stamp `ir.Provenance.SourceFile` with whatever path they are handed. The rename happens once, after the read. `Loader.SourceName` maps a path to the name provenance carries, and `relocateSources` walks the message tree by reflection to apply it to every locator in both `ir` and `geom` (agni 501).
- **A host that reads through `Loader.FS` sets no `SourceName`**, because an `fs.ValidPath` is already unrooted. The CLI sets it from its mount table.
- **A new output format inherits the rename, and a new host does not.** A new host that opens files by absolute path and leaves `SourceName` nil publishes the machine that ran it.
- **A query's citation list is sorted.** A change to the source string reorders committed captures.
- **gEDA's oracle is Lepton in the `agni-native` image** (`docker run --rm --entrypoint sh agni-native -c 'lepton-netlist -g geda ...'`). It confirmed gEDA's mirror-then-turn order on 24 pins. `lepton-cli export` writes text as outlines, so check a text angle by eye.

Each of these readers has its own CLAUDE.md:

- `readers/allegro/CLAUDE.md` covers the binary Allegro `.brd` reader.
- `readers/altium/CLAUDE.md` covers the Altium `.PcbDoc` reader.
- `readers/edif/CLAUDE.md` covers EDIF netlists, `.eds` schematics, hierarchy and `emit --format edif`.
- `readers/kicad/CLAUDE.md` covers KiCad schematics and boards, and kicad-cli as an oracle.
- `readers/odb/CLAUDE.md` covers ODB++ jobs, ODX, and Allegro's IPC-2581 export.
- `readers/orcad/CLAUDE.md` covers OrCAD Capture `.DSN` designs.
- `internal/sexpr/CLAUDE.md` covers the s-expression parser that the KiCad and EDIF readers share.
