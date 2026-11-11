# readers/altium

This file covers the Altium `.PcbDoc` reader.

- **An Altium `.PcbDoc` is a Microsoft compound file** (agni 1349). The reader reads it over the MS-CFB reader in `internal/cfb`.
- **`TestAltiumPcbDocMatchesItsODBExport` holds the five foxBMS boards to their ODB++ jobs.** The boards come from the `altium` samples artifact and the jobs from `odb-altium`.
- **The board designator is the designator text, not `SOURCEDESIGNATOR`.** `SOURCEDESIGNATOR` repeats across a multi-channel design's channels.
- **Pads are stored at board positions**, so the reader solves their footprint-local positions.
- **The reader leaves out copper from a polygon pour or a graphical component**, as Altium's export does.
- **Part numbers come from the `PrimitiveParameters` stream.** Some parts spell the key `PartNumber`, and that spelling is an MPN alias.
