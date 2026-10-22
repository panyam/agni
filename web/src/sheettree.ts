// The sheet hierarchy as a forest, shared by the file tree and the Sheets dock (agni issue 1141), so
// the two lists nest a design's sheets the same way.

// SheetNode is one sheet and the sheets nested under it.
export interface SheetNode<T> {
  sheet: T;
  children: SheetNode<T>[];
}

// sheetForest groups a flat sheet list into a parent/child forest by parent id, keeping design order
// among siblings. A sheet with an empty or unknown parent is a root, and there may be several: EDIF
// pages are all roots, while a KiCad design has one root with descendants.
export function sheetForest<T extends { id: string; parentId?: string }>(sheets: T[]): SheetNode<T>[] {
  const nodes = sheets.map((s) => ({ sheet: s, children: [] as SheetNode<T>[] }));
  const byId = new Map(nodes.map((n) => [n.sheet.id, n]));
  const roots: SheetNode<T>[] = [];
  for (const n of nodes) {
    const parent = n.sheet.parentId ? byId.get(n.sheet.parentId) : undefined;
    if (parent && parent !== n) parent.children.push(n);
    else roots.push(n);
  }
  return roots;
}
