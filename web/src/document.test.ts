import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, it, expect, vi } from "vitest";
import { DocumentStore } from "./document.js";

const here = dirname(fileURLToPath(import.meta.url));
const source = (name: string) => readFileSync(join(here, name), "utf8");
// code drops comments, so a comment explaining the split cannot trip the check or satisfy it.
const code = (text: string) => text.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/.*$/gm, "");

// What the interaction half holds: who is selected and focused, what is lit, the camera, the busy
// state, the panel tabs. The document half must name none of it.
const INTERACTION_MODULES = ["./selection.js", "./highlights.js", "./highlightstyle.js", "./camera.js", "./busy.js", "./querytabs.js", "./viewer.js"];
const INTERACTION_STATE = /\b(selectedSubject|selectedNetId|focusedVerdict|highlights|highlightStyle|viewMemory|busyDepth|tracedPins|ViewerPresenter)\b/;

// agni issue 390: the dependency runs one way. Interaction reads the document; the document never
// reads interaction, or the two halves form a cycle across what may become a WebAssembly boundary.
describe("the document half reads no interaction state", () => {
  it("imports no interaction module and names no interaction state", () => {
    const doc = code(source("document.ts"));
    const imports = [...doc.matchAll(/from "([^"]+)"/g)].map((m) => m[1]);
    expect(imports.length).toBeGreaterThan(3);
    for (const m of INTERACTION_MODULES) expect(imports).not.toContain(m);
    expect(doc.match(INTERACTION_STATE)?.[0] ?? null).toBeNull();
    expect(doc).not.toMatch(/\.tsx"/);
  });

  // The positive control: the presenter does hold interaction state and does read the store, so the
  // checks above can fail and the store is the one it uses.
  it("is what the interaction half reads, and the patterns above match the interaction half", () => {
    const viewer = code(source("viewer.ts"));
    expect(viewer).toMatch(/from "\.\/document\.js"/);
    expect(viewer).toMatch(INTERACTION_STATE);
    expect([...viewer.matchAll(/from "([^"]+)"/g)].map((m) => m[1])).toEqual(expect.arrayContaining(["./selection.js"]));
  });
});

// The store works with no presenter at all: it loads a catalog and folds runs on its own.
describe("DocumentStore on its own", () => {
  it("loads the catalog, runs only the rules it lacks, and folds each answer into one run", async () => {
    const doc = new DocumentStore();
    doc.mount = "m";
    doc.path = "b.edn";
    const listRules = vi.fn(async () => ({
      rules: [
        { name: "a", severity: "info", summary: "", impact: "", remedy: "", detail: "", reads: [], tags: {}, available: true, unavailableReason: "" },
        { name: "b", severity: "info", summary: "", impact: "", remedy: "", detail: "", reads: [], tags: {}, available: true, unavailableReason: "" },
      ],
    }));
    await doc.loadCatalog({ listRules } as never, "mount://m/b.edn", undefined);
    expect(doc.selectedRules).toEqual(["a", "b"]);
    const checkDesign = vi.fn(async (req: { rules: string[] }) => ({
      findings: req.rules.map((r) => ({ rule: r, severity: "info", message: r, subject: { kind: "net", ref: "N" } })),
      verdicts: [],
      skipped: [],
    }));
    doc.selectedRules = ["a"];
    await doc.runMissing({ checkDesign } as never, undefined);
    doc.selectedRules = ["a", "b"];
    await doc.runMissing({ checkDesign } as never, undefined);
    await doc.runMissing({ checkDesign } as never, undefined);
    expect(checkDesign.mock.calls.map((c) => c[0].rules)).toEqual([["a"], ["b"]]);
    expect(doc.run?.resp.findings.map((f) => f.rule)).toEqual(["a", "b"]);
    expect(doc.heldRun()?.names).toEqual(["a", "b"]);
  });
});
