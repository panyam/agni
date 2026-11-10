import { describe, it, expect } from "vitest";
import { defaultHidden, layerHiding, layersKey, parseHidden, presetHidden, soloHidden, storageLayers, toggled } from "./boardlayers.js";
import type { BoardLayerRow } from "./gen/agni/v1/geom/geom_packed_pb.js";

const rows = [
  { name: "F.Cu", kind: "copper", side: "front", color: "", svgSelector: '[data-layer="F.Cu"], .picks-front', groups: [6] },
  { name: "In1.Cu", kind: "copper", side: "inner", color: "", svgSelector: '[data-layer="In1.Cu"]', groups: [13, 14] },
  { name: "B.Cu", kind: "copper", side: "back", color: "", svgSelector: '[data-layer="B.Cu"]', groups: [7] },
  { name: "Zones", kind: "zones", side: "", color: "", svgSelector: ".zones-front, .zones-inner", groups: [14] },
  { name: "B.SilkS", kind: "silk", side: "back", color: "", svgSelector: '[data-layer="B.SilkS"]', groups: [15] },
] as unknown as BoardLayerRow[];

const sorted = (s: Set<string>) => [...s].sort();

describe("board layer presets", () => {
  it("hides the other side and the inner layers, and keeps the rows of both sides", () => {
    expect(sorted(presetHidden(rows, "front"))).toEqual(["B.Cu", "B.SilkS", "In1.Cu"]);
    expect(sorted(presetHidden(rows, "back"))).toEqual(["F.Cu", "In1.Cu"]);
    expect(sorted(presetHidden(rows, "all"))).toEqual([]);
    expect(presetHidden(rows, "none").size).toBe(rows.length);
  });

  it("shows one row alone and flips one row", () => {
    expect(sorted(soloHidden(rows, "In1.Cu"))).toEqual(["B.Cu", "B.SilkS", "F.Cu", "Zones"]);
    expect(sorted(toggled(new Set(["B.Cu"]), "B.Cu"))).toEqual([]);
    expect(sorted(toggled(new Set(["B.Cu"]), "F.Cu"))).toEqual(["B.Cu", "F.Cu"]);
  });
});

// A board opens with its fab rows hidden, since their text covers a dense board (agni issue 1486).
describe("defaultHidden", () => {
  it("hides the fab rows and nothing else", () => {
    const withFab = [
      ...rows,
      { name: "F.Fab", kind: "fab", side: "front", color: "", svgSelector: '[data-layer="F.Fab"]', groups: [16] },
      { name: "B.Fab", kind: "fab", side: "back", color: "", svgSelector: '[data-layer="B.Fab"]', groups: [17] },
      { name: "F.CrtYd", kind: "courtyard", side: "front", color: "", svgSelector: '[data-layer="F.CrtYd"]', groups: [18] },
    ] as unknown as BoardLayerRow[];
    expect(sorted(defaultHidden(withFab))).toEqual(["B.Fab", "F.Fab"]);
    expect(defaultHidden(rows).size).toBe(0);
  });
});

describe("layerHiding", () => {
  it("writes one display:none rule per hidden row, scoped to the board host", () => {
    const h = layerHiding(rows, new Set(["F.Cu", "Zones"]), "#svg-view");
    expect(h.css.split("\n")).toEqual(['#svg-view :is([data-layer="F.Cu"], .picks-front) { display: none; }', "#svg-view :is(.zones-front, .zones-inner) { display: none; }"]);
    expect(h.groups).toEqual([6, 14]);
  });

  it("hides nothing for an empty set", () => {
    expect(layerHiding(rows, new Set(), "#svg-view")).toEqual({ css: "", groups: [] });
  });
});

describe("the kept choice", () => {
  it("reads back only an array of names", () => {
    expect(parseHidden('["In1.Cu"]')).toEqual(["In1.Cu"]);
    expect(parseHidden(null)).toBeNull();
    expect(parseHidden("{")).toBeNull();
    expect(parseHidden('"front"')).toBeNull();
    expect(parseHidden("[1]")).toBeNull();
  });

  it("keeps a design's set under its own key and survives storage that throws", () => {
    const mem = new Map<string, string>();
    const storage = { getItem: (k: string) => mem.get(k) ?? null, setItem: (k: string, v: string) => void mem.set(k, v), removeItem: (k: string) => void mem.delete(k) } as unknown as Storage;
    const store = storageLayers(() => storage);
    store.save("m", "b.kicad_pcb", ["In1.Cu"]);
    expect(mem.get(layersKey("m", "b.kicad_pcb"))).toBe('["In1.Cu"]');
    expect(store.load("m", "b.kicad_pcb")).toEqual(["In1.Cu"]);
    expect(store.load("m", "other.kicad_pcb")).toBeNull();
    // Showing every layer is a choice too, kept apart from never having chosen (agni issue 1486).
    store.save("m", "b.kicad_pcb", []);
    expect(mem.get(layersKey("m", "b.kicad_pcb"))).toBe("[]");
    expect(store.load("m", "b.kicad_pcb")).toEqual([]);

    const broken = storageLayers(() => {
      throw new Error("blocked");
    });
    expect(broken.load("m", "b")).toBeNull();
    expect(() => broken.save("m", "b", ["F.Cu"])).not.toThrow();
  });
});
