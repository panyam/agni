/** SheetRef-like ids for picking a drawable sheet from a net name when checks carry no sheet badges. */

export type SheetHint = { id: string; name: string };

// parseIndexedNet reads Interior-style net names into a sheet title stem and a channel index.
// EFUSE_25_01 → stem EFUSE25, index 1. OPH0750_12 → stem OPH0750, index 12.
export function parseIndexedNet(net: string): { stem: string; index: number } | null {
  const triple = net.match(/^([A-Za-z]+)_(\d+)_(\d+)$/);
  if (triple) {
    return { stem: (triple[1] + triple[2]).toUpperCase(), index: parseInt(triple[3], 10) };
  }
  const pair = net.match(/^([A-Za-z0-9]+)_(\d+)$/);
  if (pair) {
    return { stem: pair[1].toUpperCase(), index: parseInt(pair[2], 10) };
  }
  return null;
}

function sheetMatchesIndexedNet(sheetName: string, stem: string, index: number): boolean {
  const sn = sheetName.toUpperCase();
  if (!sn.includes(stem)) return false;
  const range = sheetName.match(/\[\s*(\d+)\s*-\s*(\d+)\s*\]/);
  if (!range) return true;
  const lo = parseInt(range[1], 10);
  const hi = parseInt(range[2], 10);
  return index >= lo && index <= hi;
}

export function sheetIdForNet(net: string, sheets: SheetHint[]): string {
  if (!net || sheets.length <= 1) return "";
  const parsed = parseIndexedNet(net);
  if (parsed) {
    const hit = sheets.find((s) => sheetMatchesIndexedNet(s.name || s.id, parsed.stem, parsed.index));
    if (hit) return hit.id;
  }
  const compact = net.replace(/_/g, "").toLowerCase();
  const fuzzy = sheets.find((s) => {
    const sn = (s.name || s.id).replace(/[\s_\[\].-]/g, "").toLowerCase();
    return sn.includes(compact);
  });
  return fuzzy?.id ?? "";
}
