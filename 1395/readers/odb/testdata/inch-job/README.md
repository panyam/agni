A hand-written ODB++ job in inches, the way a Valor-lineage producer writes one rather than the way
KiCad does: an inner plane layer, a part on each side, a legacy orient code, an arc track, a via, a
plane with a hole, and silkscreen text. U2 carries what eda/data leaves to the reader: a custom land
drawn as a surface, a pad drawn by the job's own symbol (`symbols/mypad`), a non-plated hole on a
drill layer eda/data never names, Altium's designator-prefixed pin names (`U2-1`), and a feature
group (`FGR`). `odb_test.go` asserts what it reads to.
