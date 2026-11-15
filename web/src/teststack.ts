// jetsonRows is the Jetson cut's stack for tests (agni issue 1593). Used only by tests.
import type { StackRowIn } from "./stacklayout.js";

// The Jetson cut's stack as agni stackup prints it (examples/cross-probe-project). Silk and paste
// give no thickness, and the rest sum to the board's stated 1.711 mm.
const JETSON: [string, string, string, number | undefined][] = [
  ["F.SilkS", "silk", "F.SilkS", undefined],
  ["F.Paste", "paste", "F.Paste", undefined],
  ["F.Mask", "mask", "F.Mask", 0.01],
  ["F.Cu", "copper", "F.Cu", 0.035],
  ["dielectric 1", "dielectric", "", 0.12],
  ["In1.Cu", "copper", "In1.Cu", 0.018],
  ["dielectric 2", "dielectric", "", 0.2],
  ["In2.Cu", "copper", "In2.Cu", 0.018],
  ["dielectric 3", "dielectric", "", 0.07],
  ["dielectric 3", "dielectric", "", 0.07],
  ["In3.Cu", "copper", "In3.Cu", 0.018],
  ["dielectric 4", "dielectric", "", 0.2],
  ["In4.Cu", "copper", "In4.Cu", 0.018],
  ["dielectric 5", "dielectric", "", 0.07],
  ["dielectric 5", "dielectric", "", 0.07],
  ["In5.Cu", "copper", "In5.Cu", 0.018],
  ["dielectric 6", "dielectric", "", 0.2],
  ["In6.Cu", "copper", "In6.Cu", 0.035],
  ["dielectric 7", "dielectric", "", 0.07],
  ["dielectric 7", "dielectric", "", 0.07],
  ["In7.Cu", "copper", "In7.Cu", 0.018],
  ["dielectric 8", "dielectric", "", 0.2],
  ["In8.Cu", "copper", "In8.Cu", 0.018],
  ["dielectric 9", "dielectric", "", 0.12],
  ["B.Cu", "copper", "B.Cu", 0.035],
  ["B.Mask", "mask", "B.Mask", 0.01],
  ["B.Paste", "paste", "B.Paste", undefined],
  ["B.SilkS", "silk", "B.SilkS", undefined],
];

export const jetsonRows = (): StackRowIn[] => JETSON.map(([name, kind, layer, thicknessMm]) => ({ name, kind, layer, thicknessMm, color: "#888888" }));

