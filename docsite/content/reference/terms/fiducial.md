---
title: "Fiducial"
label: "fiducial"
summary: "A small copper dot on a board, connected to nothing, that an assembly machine's camera finds to learn exactly where the board sits before it places parts."
level: EE7
---

A small round dot of bare copper, usually about a millimetre across, with a clear ring of board
around it so it stands out. It carries no signal and joins no net. It is there for the assembly
machine.

A pick-and-place machine holds the board in a clamp, and the clamp is never perfectly square. So
before it places any part, its camera finds two or three fiducials at known spots and works out how
far the board has shifted and turned. Every part after that lands where the layout says. A fine-pitch
chip often gets a pair of its own fiducials beside it, so the machine can line up that one part even
more closely.

A design tool places a fiducial as a part, with a designator such as `FID1` or `E97` and a footprint
of one pad. That pad is no pin, because it connects to nothing. So a fiducial reads as a part with no
pins, like a mounting hole or a logo, and
[`unconnected-component`](../../rules/unconnected-component/) leaves it out rather than reporting it
as a part someone forgot to wire.

For a software reader, a fiducial is a calibration marker, like the checkerboard a camera
system photographs to correct its own lens before it measures anything.
