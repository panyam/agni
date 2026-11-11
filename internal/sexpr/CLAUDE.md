# internal/sexpr

This file covers the s-expression parser that the KiCad and EDIF readers share.

- **The parser shares every identical atom between nodes** (agni 945). The sharing holds a large board in under 4 times its file size.
- **Never assign a parsed node's `Atom`.** Every node holding that atom would change. Put a new node in its parent's `Kids` instead, as EDIF's `canonicalize` does.
- **`ParseSkipping` leaves named lists out of the tree.** The KiCad board reads skip `filled_polygon` (agni 946). The census parses unskipped.
- **`ParseWith` also skips a list by one child's value** (`SkipWhen`). It reads ahead without building the list. A board read drops footprint graphics on undrawn layers at parse this way (agni 1005).
- **A board keeps footprint graphics on the silkscreen, fab and courtyard layers alone** (`footprintLayerDrawn`, agni 946). One vendor library's user-layer artwork made up nearly all of Jetson's footprint lines.
