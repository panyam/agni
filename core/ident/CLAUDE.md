# core/ident

This file covers how agni compares an identifier across two documents, such as a pin map against a netlist.

- **Compare an identifier across two documents through `core/ident`, never with `==`.** A pin map uses the datasheet's vocabulary and a netlist uses package designators. Both carry zero-padded indices, invisible characters pasted out of a PDF, and cells that name several functions at once. A plain string comparison turns each of these into a false warning.
- **`Canonical` is the one canonical form, and both sides use it.**
- **`\s` does not match U+200B**, so stripping `\s` does not remove a zero-width space.
- **A pin name is compared loosely and a net name is not.** A vendor table is inconsistent with itself, and a net name is the design's own identifier.
- **`ident.PinName` is the loose key.** It folds underscores after the index rewrite, so `IO0_1` never meets a different `IO1`.
- **`ident.PinNumber` keeps a designator's zeros** (agni 702).
- **A fuzzy match needs both ends to agree** (`tokenSubsequence`). `GMAC0_TXD2` meets `GMAC0_MII_RGMII_TXD2` loosely. `TXD` never meets `UART0_TXD`, and a single token is never fuzzy.
- **A test of a loose match uses an anchored pair**, or it tests nothing.
- **A caller comparing many against many builds a map keyed on `Canonical`** and does not nest `Compare`. 200 rows against 1600 nets is 320,000 comparisons.
