// The heights a board view draws its layers at when it shows the board's stack apart (agni issue
// 1593). Every height here is a depth in millimetres below the board's top face. The engine places
// each stack row in depth, with a typical thickness where the file gives none (BoardStackRow.top_mm,
// render.TypicalMm, agni issue 1635), and stackZ turns a depth into a world z for the camera.

// SPREAD_OF_WIDTH is how tall the stack draws at full explode, as a share of the board's width.
export const SPREAD_OF_WIDTH = 0.6;

// StackRowIn is the part of a BoardStackRow the layout reads.
export interface StackRowIn {
  name: string;
  kind: string;
  layer: string;
  color: string;
  topMm: number;
  bottomMm: number;
  thicknessDefaulted: boolean;
}

// StackPlane is one stack row placed in depth. top and bottom are millimetres below the board's top
// face, and defaulted says the row draws at a typical thickness rather than the file's.
export interface StackPlane {
  name: string;
  kind: string;
  layer: string;
  color: string;
  top: number;
  bottom: number;
  defaulted: boolean;
}

export interface StackLayout {
  planes: StackPlane[];
  totalMm: number;
}

// layoutStack lays the rows at the depths the engine placed them, and the stack's total at the last
// row's bottom.
export function layoutStack(rows: readonly StackRowIn[]): StackLayout {
  const planes = rows.map((r) => ({ name: r.name, kind: r.kind, layer: r.layer, color: r.color, top: r.topMm, bottom: r.bottomMm, defaulted: r.thicknessDefaulted }));
  return { planes, totalMm: planes.reduce((at, p) => Math.max(at, p.bottom), 0) };
}

// LayerRowIn is the part of a BoardLayerRow that groupDepths reads.
export interface LayerRowIn {
  name: string;
  kind: string;
  side: string;
  groups: readonly number[];
}

// GroupDepths places each packed group of the board at a depth, and lists the groups the stack
// view draws another way: the through copper and drill holes, which draw as barrels.
export interface GroupDepths {
  depth: Map<number, number>;
  barrels: Set<number>;
}

// groupDepths puts each layer row's groups at the middle of the stack row naming its layer. A row the
// stack does not name sits on the face of its side, the top for the front and the bottom for the back,
// so F.Fab draws on the top face. The copper rows go first, so a zone or pour group that a copper row
// shares with the Zones row sits at its copper's depth. Edge draws on the top face.
export function groupDepths(layout: StackLayout, rows: readonly LayerRowIn[]): GroupDepths {
  const depth = new Map<number, number>();
  const barrels = new Set<number>();
  const plane = new Map<string, StackPlane>();
  for (const p of layout.planes) if (p.layer && !plane.has(p.layer)) plane.set(p.layer, p);
  const rank = (r: LayerRowIn) => (r.kind === "copper" ? 0 : r.kind === "zones" || r.kind === "pours" ? 2 : 1);
  for (const r of [...rows].sort((a, b) => rank(a) - rank(b))) {
    if (r.kind === "through") {
      for (const g of r.groups) barrels.add(g);
      continue;
    }
    const p = plane.get(r.name);
    const d = p ? (p.top + p.bottom) / 2 : r.side === "back" ? layout.totalMm : 0;
    for (const g of r.groups) if (!depth.has(g)) depth.set(g, d);
  }
  return { depth, barrels };
}

// perMm is how many world units one millimetre of depth draws as. Explode 0 draws the board's true
// proportions, unitsPerMm. Explode 1 draws the stack SPREAD_OF_WIDTH of the board's width tall. Every
// depth scales by the same factor, so each gap stays in proportion to its layer's thickness.
export function perMm(explode: number, layout: StackLayout, widthUnits: number, unitsPerMm: number): number {
  const e = Math.min(Math.max(explode, 0), 1);
  const full = layout.totalMm > 0 ? Math.max(unitsPerMm, (SPREAD_OF_WIDTH * widthUnits) / layout.totalMm) : unitsPerMm;
  return unitsPerMm + e * (full - unitsPerMm);
}

// stackZ is the world z of a depth, with the stack centred on z = 0 and the top face highest.
export function stackZ(depthMm: number, layout: StackLayout, mmScale: number): number {
  return (layout.totalMm / 2 - depthMm) * mmScale;
}
