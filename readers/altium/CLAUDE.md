# readers/altium

This file covers the Altium `.PcbDoc` reader.

- **An Altium `.PcbDoc` is a Microsoft compound file** (agni 1349). The reader reads it over the MS-CFB reader in `internal/cfb`.
- **`TestAltiumPcbDocMatchesItsODBExport` holds the five foxBMS boards to their ODB++ jobs.** The boards come from the `altium` samples artifact and the jobs from `odb-altium`.
- **The stack comes from Board6's `LAYER<n>` keys along the copper chain** (agni 1601). `LAYER<n>DIEL*` is the dielectric under layer n, and `TOPHEIGHT`/`BOTTOMHEIGHT` the masks. Board6 states no board thickness of its own. `TestAltiumStackMatchesItsASCIISave` holds the OSP boards to Altium's ASCII saves, and `TestAltiumStackMatchesItsODBExport` the foxBMS boards to their ODB++ jobs.
- **Altium's ODB++ export writes no dielectric rows.** It puts the dielectric under each copper layer in that layer's `.layer_dielectric`, and agni's ODB++ reader takes that for the copper's own thickness (#1660).
- **The board designator is the designator text, not `SOURCEDESIGNATOR`.** `SOURCEDESIGNATOR` repeats across a multi-channel design's channels.
- **Pads are stored at board positions**, so the reader solves their footprint-local positions.
- **The reader leaves out copper from a polygon pour or a graphical component**, as Altium's export does.
- **Part numbers come from the `PrimitiveParameters` stream.** Some parts spell the key `PartNumber`, and that spelling is an MPN alias.
