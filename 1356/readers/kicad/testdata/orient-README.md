# Orientation fixtures

`orientation_test.go` reads these. They hold a drawn pin to where the pin connects (agni issue 1322).

`orient-matrix.kicad_sch` is hand-made. A polarized 2-pin part, with its origin on pin 1, and an asymmetric 3-pin part are each placed at 0, 90, 180 and 270 degrees, plain, with `(mirror x)` and with `(mirror y)`. Every pin runs a short wire to a label named `<ref>_<pin>`. `orient-matrix.kicad-nets.txt` is kicad-cli's netlist of it, and every pin sits on its own label there, so the labels mark where KiCad puts each pin.

`orient-jetson-caps.kicad_sch` and `orient-jetson-fet.kicad_sch` are cuts of the Antmicro baseboard for Jetson AGX Thor, Copyright (c) 2025-2026 Antmicro, licensed under the Apache License, Version 2.0. Each keeps a few parts of one sheet unchanged, with their wires, power symbols and library symbols. The caps cut holds C133, C134, C135 and C44 from "USB Debug, PD". Three of them sit at 270 degrees with no mirror and drew lifted off their GND stubs. The FET cut holds Q5, TP76 and U61 from "Power". Q5 and TP76 sit at 90 degrees and drew onto U61.

The `.kicad-*.txt` files beside a fixture are kicad-cli 10.0.6's own drawing of it, read out of `kicad-cli sch export svg`, and the tests hold agni's drawing to them (agni issue 1344). kicad-cli writes every text run as an invisible `<text>` element at its baseline, with its anchor and its rotation on the enclosing group. `orient-jetson-fet.kicad-text.txt` holds every pin name and number on U61, Q5 and TP76.

`orient-labels.kicad_sch` is hand-made. It places a local, a hierarchical and a global label at 0, 90, 180 and 270 degrees, and a global label of every flag shape, each at the end of a short wire. `orient-labels.kicad-text.txt` holds kicad-cli's text run for each, `orient-labels.kicad-flags.txt` each global label's outline, and `orient-labels.kicad-arrows.txt` each hierarchical label's arrow.
