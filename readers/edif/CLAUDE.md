# readers/edif

This file covers the EDIF reader, `.edn` netlists and `.eds` schematic exports, hierarchy, and the EDIF writer behind `emit --format edif`.

- **EDIF keywords are case-insensitive, and the reader folds them at parse** (agni 941). Altium's EDIF For PCB writes `(Net`, `(PortRef` and `(Instance`. `readers/edif/sexpr.go` respells each list's head from `keywords`.
- **A new keyword lookup needs an entry in `keywords`.** `TestEveryLookedUpKeywordIsCanonicalized` fails when one is missing.
- **When no instance in a file carries a designator, each instance's name becomes its ref-des.** Altium writes no `(designator ...)` and names each instance by its ref-des.
- **A `portRef` maps to a pin number through the instance's `portInstance` first** and the cell port's own `(designator ...)` second (agni 1193).
- **A part is keyed by the designator that `instanceBackAnnotate` gives it, in both reads** (agni 1228, `readers/edif/backannotate.go`). An export can renumber parts after layout. The page then draws `R4` while the netlist and the design's `viewMap` say `R1001`.
- **Back-annotation maps only a top-level instance.** A block's page is drawn once however often the block is placed (#154). `SchematicGeometry.back_annotated` lists every placement relabelled.
- **A `.eds` is read as a netlist and as geometry, so a fix to one read alone only moves a mismatch.**
- **A `.eds` netlist is not the `.edn`'s.** An EDIF schematic export registers both a `Design` and a `Geometry` reader. Its nets join portRefs in the same grammar, so any tool parses it as a netlist without complaint. It counts drawn instances and per-sheet segments rather than resolved nets, so its net count runs far higher.
- **Never compare a `.eds`-only design's counts with a design read from a netlist.** Such a design can still be rendered and queried. `formats.Format.DrawnNetlist` marks the format, and `GetDesign` withholds counts only for such a format. A KiCad, xschem or gEDA schematic is its design's netlist and reports counts (agni 1342).
- **A net subject locates by its name on every format but KiCad** (`sheetsFor`, agni 1230). A net finding keys on the net's id, and only KiCad's geometry stamps an id on a wire.
- **A schematic that fails to read answers `LOCATE_REASON_SCHEMATIC_UNREADABLE`**, not `NO_GEOMETRY`, and the CLI warns once. `render` reports the parse error itself.
- **EDIF has no comment syntax.** A `;` note in a `.eds` fixture reads as a construct.
- **A hierarchical `.edn` reads as a flat design missing most of its netlist.** The reader scopes instance extraction to the design's root cell (WS1-004). Every sub-cell's contents are absent, and every rule evaluates over the top cell alone.
- **The reader lists each skipped cell with its instance count in `InputDiagnostics.unexpanded_hierarchy`** (agni 707). Every CLI read prints a stderr note naming the cells, and `agni stats` adds a `not extracted:` line. Both HTML reports state it in their header. The viewer shows a canvas strip and lists the cells in Properties (agni 717).
- **Every surface says the sentence that `report.HierarchyNote` builds.** A report takes the list off the run, since `CheckDesignResponse` and `DesignRef` carry it, so the report reads no design.
- **An unflattened hierarchical read is wrong, so read the note rather than the counts.** It is not a finding, since hierarchy is a limit of the read and not a defect.
- **`--flatten-hierarchy` reads the blocks** (`Snapshot.flatten_hierarchy`, agni 718). It is off by default, because it renames every interior net and moves a diff.
- **The flatten flag travels in the overlay**, so its identity keys every cache tier, and the parse key names it too. A new read path that builds options without `Overlay.ReadOptions` drops it silently.
- **A design declares flattening once with `flatten_hierarchy: true` in `design.yaml`.** `ProjectResolver.Overlay` ORs it in. The viewer asks with `?flatten=1`. The strip's Read every block button adds that parameter, and the page's one snapshot registry carries it into every ingest (agni 1102).
- **Flattening's naming rules are in `architecture/ingestion-and-ir.md`.** Nobody has verified flattening on a real multi-cell export.
- **`emit --format edif` writes for a reader other than ours** (agni 563). Our reader resolves references after parsing the whole file, accepts any atom as an identifier, and reads a port reference as a pin designator when nothing maps it. A writer relying on those three round-trips perfectly and writes a file nobody else can open.
- **The writer follows four rules.** It writes libraries before the design node. A `portRef` names the cell's port, with a `portInstance` table carrying the pin. An identifier holds no character a reader rejects. The file declares everything it references, cells included, and a top cell for which a board read gives no source.
- **The writer's tests assert those properties over the emitted text** in `readers/formats/e2e_edif_conformance_test.go`. A re-read goes back through the same forgiving reader, so it agrees with the writer whatever either does. `guide/cli-reference.md` has the semantics, and `build/evidence.md` has the out-of-tree oracle.
