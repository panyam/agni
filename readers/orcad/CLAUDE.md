# readers/orcad

This file covers the OrCAD Capture `.DSN` reader and its page geometry.

- **An OrCAD Capture `.DSN` is a compound file, and agni reads it flattened** (agni 1399). The reader reads it over the shared `internal/cfb`.
- **A block keeps its pages in a `.DSN` of its own**, named by a path relative to its parent (`.\MPU9250\MPU9250.DSN`). The Loader's opener matches that path without case.
- **A dropped folder groups a `.DSN` with its block files** through `formats.Format.Blocks`.
- **The reader never follows the `Library` stream's absolute `E:\...` paths.**
- **Parts take their designators from the root's `Hierarchy` stream.** A placed part's own field can still say `C?`.
- **A pin joins the wire its stored position sits on.**
- **A port, power symbol or off-page connector joins the wire ends inside its stored box**, never at a corner.
- **A folder with a port named like one of its power symbols binds that power net to the port.** Clay's charger block puts GND on the board's GND_EARTH this way.
- **A part's `PACK_SHORT` joins its pins.** A pin's own stored net id joins it even when the pin is flagged no-connect.
- **Capture names an unnamed net `N%05d` after the lowest wire id on it**, not its net id (agni 1406). The shallowest name wins, so the root's generated name beats a block's label. One wire-less pin-to-pin net is still numbered another way.
- **`pstxnet.dat` and the board cut a name at 31 characters.** The reader keeps the name whole.
- **`TestOrCADReadsAsClaysNetlist` holds Clay R6 to `pstxnet.dat`**, with only the seven `NC` pins listed. `TestOrCADNamesNetsAsClaysNetlist` holds every net name to `pstxnet.dat`. `TestOrCADJoinsClaysBoard` holds the design to `CLAYR6.brd` by designator.
- **The pages draw too** (agni 1400, `readers/orcad/geometry.go`). Every placed pin stores its page position, so the file checks the transform itself. `TestEveryPlacedPinLandsWhereThePageStoresIt` holds all of Clay's placed pins to `geomath.PlacePin`.
- **A power symbol or port is placed from its stored box's corner**, never its anchor. A supply's anchor sits above its box.
- **No file checks the mirror bit.** Clay places nothing mirrored.
