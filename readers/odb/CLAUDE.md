# readers/odb

This file covers the ODB++ job reader, ODB++X (ODX), and the Allegro export quirks that the ODX and IPC-2581 reads share.

- **An ODB++ job is a directory, entered by its `matrix/matrix` file** (agni 1250). The registry keys that path as `.odb++` (`lowerExt`). A `.tgz` or `.zip` holds the whole job in one archive. The CLI and the proposer turn a job folder into its matrix entry.
- **A host gathering a design's files asks `formats.DesignRoot`.** Otherwise it fetches `matrix/` alone.
- **A pin's net comes from its toeprint record, and a copper feature's from eda/data's `FID` list**, never from geometry.
- **The fixtures are `kicad-cli pcb export odb` output** (`make odb-fixtures`, KiCad 9+). `TestODBReadsAsItsKiCadSource` holds each fixture to its source board, pad positions included.
- **`make odb-oracle` holds the six samples boards to `readers/formats/testdata/odb_oracle.baseline`.**
- **The gate reads foxBMS's Altium-written jobs** (`odb-altium`). Their pins are named `J500-1`, and their `FGR` groups are not nets.
- **ODB++ forbids spaces in a name.** `/USB Hub/VBUS` arrives as `/USB_Hub/VBUS`, and a schematic-against-layout diff reports it renamed.
- **KiCad's exporter leaves out a footprint marked `exclude_from_pos_files`.** It nets copper by what touches, so a planted short moves a track onto the other net (demo-board).
- **ODX is ODB++ in one XML file rooted at `<ODX>`** (agni 1372, `readers/odb/odx.go`). It fills the job reader's own `parsedJob`, so its netlist and board come out of the job code. `.xml` is sniffed by root element.
- **ODX names a pin by its number**, since the name can be a placeholder such as `un_1`.
- **Allegro writes `encoding = "ISO-8859-1"` with spaces**, and Go's decoder ignores that form. `odxDecoded` reads the declaration itself.
- **A `CAD_SET` attribute holds until the next one changes it.** Allegro writes a via's tracks after its `USAGE="VIA"` pad without a reset, so `USAGE` marks pads alone. A surface on no net is kept on `$NONE$`.
- **`TestODXReadsAsItsIPC2581Twin` holds parasdr's two Allegro exports to each other** (`cadence-exports`). `TestParasdrsCopperAgreesAcrossItsReads` holds both exports' tracks, vias and pads to the `.brd` (agni 1383).
- **Allegro's IPC-2581 export writes a track's width in an inline `LineDesc`**, and a straight run as a `<Line>`.
- **It puts each drill hit in a `Set` with no net**, so a via takes its land's net.
- **It names a drill layer for its copper span** (`IPC2581DRILL_1-6`) in place of a `<Span>`.
- **It names an unconnected pin's net `Unused_<8 hex>`.** Both IPC-2581 reads take that as no net.
