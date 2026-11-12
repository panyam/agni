# Cross-probe walk project

`make exercise-cross-probe` walks the cross-probe mission (agni issue 1321) on a board: open the
schematic and the board as two linked views, click a part in one and find it in the other, turn the
board over. The gate walks this project, and the mission's real-board run walks the whole Jetson board in
`examples/jetson-project`.

`designs/jetson-cut` is a cut of the Antmicro baseboard for Jetson AGX Thor, Copyright (c)
2025-2026 Antmicro, licensed under the Apache License, Version 2.0. `make cut` regenerates it from
the fetched samples corpus with `tools/kicadcut`. It keeps two sheets and the board's copper around
the parts on them, so it stays small enough to commit.

| Sheet | Part | Board side | Angle on the board |
| --- | --- | --- | --- |
| USB Debug, PD | C133, C134 | back | 0 |
| USB Debug, PD | C135 | back | 90 |
| USB Debug, PD | C44 | back | -90 |
| USB Debug, PD | R119, R120, D20, D21 | front | 180 |
| USB Debug, PD | R123, R124 | front | 0 |
| Power | Q5, U61, U6 | back | 90 |
| Power | C261, C34 | back | 0 |
| Power | C30, C38 | back | 180 |
| Power | TP76 | front | 0 |
| Power | Q2 | front | 90 |

The board keeps those nineteen footprints, the 397 track segments and 202 vias on their nets inside
4 mm of them, and the board outline. It drops the copper pours, which reach across the whole board,
and the footprint artwork on the User layers, which agni does not draw.

A cut ends some of the design's nets, and the cut schematic names those for itself, so `make cut`
renames each board net to the name kicad-cli's netlist of the cut gives its pads, as KiCad's Update
PCB from Schematic would. agni reads every part pin onto the net kicad-cli's netlist names, and
every pad onto its pin's net. `TestTheCrossProbeCutJoinsItsSchematicToItsBoard` holds the second.
