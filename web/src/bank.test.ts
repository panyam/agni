import { describe, it, expect, beforeEach } from "vitest";
import { LimitKind, ConditionCoverage } from "./gen/agni/v1/param/param_pb.js";
import type { BBox } from "./gen/agni/v1/doc/doc_pb.js";
import type { Region } from "./regions.js";
import {
  newParameter,
  emptySpec,
  exportSpecJson,
  importSpecJson,
  loadUiState,
  saveUiState,
  paramsForRegion,
  docId,
  getAuthor,
  uiToSet,
  setToUi,
  otherUserRegions,
  loadLayers,
  saveLayers,
  DEFAULT_LAYERS,
  REGION_ATTR,
  type NewParamFields,
  type UiState,
} from "./bank.js";

const bbox = (): BBox => ({ x: 0, y: 0, width: 1, height: 1 }) as unknown as BBox;
const bb = (x: number, y: number, w: number, h: number): BBox => ({ x, y, width: w, height: h }) as unknown as BBox;
const region: Region = { id: "p1.t1", kind: "table", label: "Abs Max", bbox: bbox(), page: 4 };
const fields = (over: Partial<NewParamFields> = {}): NewParamFields => ({
  name: "VIN",
  symbol: "VIN",
  limitKind: LimitKind.ABSOLUTE_MAX,
  max: 20,
  unit: "V",
  condition: "",
  ...over,
});

describe("bank", () => {
  beforeEach(() => localStorage.clear());

  it("newParameter stamps provenance and the region link", () => {
    const p = newParameter(fields(), region, 4, "LM1117");
    expect(p.value?.max).toBe(20);
    expect(p.value?.min).toBeUndefined(); // explicit presence, not a zero
    expect(p.limitKind).toBe(LimitKind.ABSOLUTE_MAX);
    expect(p.conditionCoverage).toBe(ConditionCoverage.UNCONDITIONAL); // no condition given
    expect(p.attributes[REGION_ATTR]).toBe("p1.t1");
    expect(p.prov?.docRef).toBe("LM1117");
    expect(p.prov?.page).toBe(4);
    expect(p.prov?.tableOrFigure).toBe("Abs Max"); // the citation is the region label
    expect(p.prov?.method).toBe("hand");
    expect(p.prov?.confidence).toBe(1);
  });

  it("uiToSet/setToUi round-trip user regions (geometry) and doc-IR type tags", () => {
    const ui: UiState = {
      userRegions: [{ id: "u1", kind: "user", label: "my box", bbox: bb(1, 2, 3, 4), page: 5 }],
      types: { u1: "schematic", "p1.t1": "table" }, // a user-region tag and a doc-IR-region tag
    };
    const set = uiToSet("LM1117", "alice", ui);
    expect(set.docId).toBe("LM1117");
    expect(set.author).toBe("alice");
    const back = setToUi(set);
    expect(back.userRegions).toHaveLength(1);
    expect(back.userRegions[0].bbox.width).toBe(3);
    expect(back.userRegions[0].page).toBe(5);
    expect(back.types.u1).toBe("schematic");
    expect(back.types["p1.t1"]).toBe("table"); // the doc-IR region's tag survived
  });

  it("otherUserRegions collects others' boxes namespaced, skipping me and type-only tags", () => {
    const mine = uiToSet("d", "me", { userRegions: [{ id: "u1", kind: "user", label: "mine", bbox: bb(0, 0, 1, 1), page: 1 }], types: {} });
    const theirs = uiToSet("d", "bob", { userRegions: [{ id: "u1", kind: "user", label: "bobbox", bbox: bb(0, 0, 1, 1), page: 2 }], types: { "p1.t1": "table" } });
    const others = otherUserRegions([mine, theirs], "me");
    expect(others).toHaveLength(1); // only bob's drawn box: not mine, not bob's doc-IR type tag
    expect(others[0].id).toBe("bob:u1"); // namespaced so it can't collide with my own u1
    expect(others[0].label).toContain("bob");
    expect(others[0].page).toBe(2);
  });

  it("loadLayers defaults doc-IR text OFF and everything else on", () => {
    const L = loadLayers();
    expect(L.text).toBe(false); // the extractor emits a region per text block; hidden by default
    expect(L.table).toBe(true);
    expect(L.figure).toBe(true);
    expect(L.mine).toBe(true);
    expect(L.others).toBe(true);
  });

  it("saveLayers round-trips, and a persisted partial fills missing keys from defaults", () => {
    saveLayers({ ...DEFAULT_LAYERS, text: true, others: false });
    const L = loadLayers();
    expect(L.text).toBe(true);
    expect(L.others).toBe(false);
    expect(L.table).toBe(true);
    // an older/partial persisted value must not read a missing key as false:
    localStorage.setItem("agni.ds.layers", JSON.stringify({ text: true }));
    const L2 = loadLayers();
    expect(L2.text).toBe(true);
    expect(L2.others).toBe(true); // default fills the gap
  });

  it("getAuthor generates then persists a stable id", () => {
    const a = getAuthor();
    expect(a).toBeTruthy();
    expect(getAuthor()).toBe(a); // stable across calls (persisted in localStorage)
  });

  it("docId derives a stable stem id from the corpus path, not the content hash", () => {
    expect(docId("ti/LM1117/LM1117.pdf")).toBe("LM1117");
    expect(docId("BSS138.pdf")).toBe("BSS138");
    expect(docId("")).toBe("");
  });

  it("provenance.doc_ref resolves to the spec's SourceDoc id (referential integrity)", () => {
    const spec = emptySpec("ti/LM1117/LM1117.pdf", "SNOS412Q");
    const p = newParameter(fields(), region, 4, spec.docs[0].id);
    expect(spec.docs[0].id).toBe("LM1117");
    expect(spec.docs[0].locator).toBe("ti/LM1117/LM1117.pdf");
    expect(p.prov?.docRef).toBe(spec.docs[0].id); // the foreign key param.Validate enforces
  });

  it("a text-only condition is captured but stays non-comparable (coverage COMPLETE, no eq)", () => {
    const p = newParameter(fields({ condition: "TA = 25C" }), region, 4, "LM1117");
    expect(p.conditions).toHaveLength(1);
    expect(p.conditions[0].raw).toBe("TA = 25C");
    expect(p.conditions[0].eq).toBeUndefined();
    expect(p.conditionCoverage).toBe(ConditionCoverage.COMPLETE);
  });

  it("exports and re-imports a PartSpec via protojson", () => {
    const spec = emptySpec("ti/LM1117/LM1117.pdf", "SNOS412Q");
    spec.mpn = "LM1117";
    spec.parameters.push(newParameter(fields(), region, 4, spec.docs[0].id));
    const back = importSpecJson(exportSpecJson(spec));
    expect(back.mpn).toBe("LM1117");
    expect(back.docs[0].id).toBe("LM1117");
    expect(back.docs[0].title).toBe("SNOS412Q");
    expect(back.parameters).toHaveLength(1);
    expect(back.parameters[0].prov?.page).toBe(4);
  });

  it("saveUiState/loadUiState persist per datasheet and stay isolated (per-user localStorage)", () => {
    saveUiState("ds", "ti/LM1117.pdf", {
      userRegions: [{ id: "u1", kind: "user", label: "user region", bbox: bbox(), page: 2 }],
      types: { "p1.f1": "chart" },
    });
    const loaded = loadUiState("ds", "ti/LM1117.pdf");
    expect(loaded.userRegions).toHaveLength(1);
    expect(loaded.userRegions[0].page).toBe(2);
    expect(loaded.types["p1.f1"]).toBe("chart");

    // A different datasheet is a fresh, empty UI state.
    const other = loadUiState("ds", "other.pdf");
    expect(other.userRegions).toEqual([]);
    expect(other.types).toEqual({});
  });

  it("paramsForRegion filters by the region link", () => {
    const spec = emptySpec("x/y.pdf", "d");
    spec.parameters.push(newParameter(fields({ name: "a", symbol: "a" }), region, 4, spec.docs[0].id));
    spec.parameters.push(newParameter(fields({ name: "b", symbol: "b" }), { ...region, id: "p1.t2" }, 4, spec.docs[0].id));
    const got = paramsForRegion(spec, "p1.t1");
    expect(got).toHaveLength(1);
    expect(got[0].name).toBe("a");
  });
});
