# readers/allegro

This file covers the binary Allegro `.brd` reader.

- **An Allegro `.brd` is read in binary, for releases 16.6 and 17.2** (agni 1388). The format has no published spec. The layouts come from KiCad's reverse-engineered notes and were checked on the BeagleBone AI.
- **Read KiCad's GPL importer for format facts only, never its code.**
- **Each release reads through its own `layout`** (`readers/allegro/layout.go`). 17.2 added words to most block types, and every later field moved. A new release is a new table, with every offset taken from KiCad's version-conditional fields.
- **Another release is refused by name** (#1355). An Eagle `.brd` is refused as Eagle.
- **A pad is footprint-local, and Allegro mirrors X before rotating.** The reader negates a bottom part's angle for the contract's rotate-then-mirror.
- **A placed pad's net goes through its 0x04 net assignment**, never straight to a 0x1B.
- **`readers/formats/e2e_allegro_test.go` holds the BeagleBone AI to the extract Allegro wrote from it** (`allegro` samples artifact). It checks nets as a partition, every pin's board position, vias, widths and values.
- **`readers/formats/e2e_allegro166_test.go` holds 16.6 to parasdr's ODX and to Clay R6's `pstxnet.dat`.** Capture handed that netlist to Allegro.
- **Clay's EDIF is not the board's second view.** It holds only the top page, and its designators name other parts.
- **`TestAllegroCopperMatchesItsFilms` holds each copper layer to the film Allegro plotted** (`internal/copperraster`, agni 1376). The test logs the inner layers' pads rather than holding them, since Allegro drops a pad that nothing on its layer connects to (#1356).
- **The cross-section is the board's physical stack, stored differently by each release** (agni 1601, `stackup.go`). 16.6 writes one 0x21 block of sub-type 0x0518 per row, with its values as text. A thickness is metres as a bare number, or the length the user typed with its unit ("8 MIL" on Clay R6). 17.2 writes one 0x36 table of code 0x10, with numbers as doubles stored high word first. parasdr's IPC-2581 twin holds 16.6, and the BeagleBone's extract holds 17.2's total and layer count.
- **A part's height is a field on its place-bound outline** (agni 1643, `heights.go`). The outline is a 0x28 shape or a 0x0E rectangle naming its part at +12, and its 0x03 field chain starts at +44 in 16.6, and at +48 (0x28) or +52 (0x0E) in 17.2. The height field is sub-type 0x66 with 0x10 0x01 at +2. parasdr's ODX holds 284 parts' heights, and 17.2 has no oracle.
- **A part's block stands on the place-bound outline that carries its height** (agni 1668). A 0x0E rectangle is stored unturned, with its corners at +28 (16.6) or +36 (17.2) and its turn in thousandths of a degree at +56 or +64, about its first corner. A 0x28 shape gives its segments. parasdr's ODX holds 282 blocks to its package outlines, and 17.2 is held only to its pads.
- **A board's declared differential pairs and match groups become `ir.Design.net_groups`** (agni 1360, `netGroups` in `readers/allegro/netclass.go`).
- **A 17.2 net chains one 0x26 per table it sits in**, the next at +8, and each names a 0x2C table. The table's third byte says 0x03 for a pair and 0x06 for a match group. A DDR strobe pair reads in its pair and in its byte lane. Both kinds were read off the BeagleBone AI, not a spec.
- **`diff-pair-naming` passes a declared pair before it reads a name** (`Model.DeclaredPair`). `net.group(group, kind, net)` queries the groups.
