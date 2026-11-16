// How well a design's schematic and its board agree (agni issue 1590), as the canvas bar shows it.
// Imagine a reader about to trust a cross-probe on a board of three thousand parts. Before they
// click, the bar says "3,310/3,320 parts", and its panel lists the ten that only one view holds.
// The engine compares the views (DiffDesigns with views_snapshot), and this module only reads its
// answer into words and the entries a reader clicks.

import type { DiffDesignsResponse, ViewAgreement_Tally } from "@panyam/agni-client/gen/agni/v1/webapi/diff_pb";
import type { Selection } from "./selection.js";

// AgreementView names the view that draws an entry, so a click selects it there.
export type AgreementView = "schematic" | "board";

// AgreementEntry is one thing that did not match. label is what the panel lists, sel what a click
// selects, and view the view that draws it.
export interface AgreementEntry {
  label: string;
  sel: Selection;
  view: AgreementView;
}

// AgreementSection is one list in the panel, with every entry it holds.
export interface AgreementSection {
  title: string;
  entries: AgreementEntry[];
}

// Agreement is the panel's whole content. compared is false when the engine did not compare the
// views, and reason then says why. summary is the bar's short form, and lines are the panel's
// counts, one per kind.
export interface Agreement {
  compared: boolean;
  reason: string;
  summary: string;
  lines: string[];
  sections: AgreementSection[];
}

// NOT_COMPARED is what the bar shows when the views were not compared. It never shows a count, so
// a design with one view never reads as agreeing.
export const NOT_COMPARED = "not compared";

// notCompared is the Agreement for views that were not compared, saying why.
export function notCompared(reason: string): Agreement {
  return { compared: false, reason, summary: NOT_COMPARED, lines: [`Not compared: ${reason}.`], sections: [] };
}

// readAgreement reads a DiffDesignsResponse for views_snapshot.
export function readAgreement(resp: DiffDesignsResponse): Agreement {
  const v = resp.views;
  if (!v?.compared) return notCompared(v?.notCompared || "the engine gave no comparison");
  const rep = resp.report;
  const nets = rep?.nets ?? [];
  const ofKind = (k: string) => nets.filter((n) => n.kind === k);
  const net = (name: string, view: AgreementView, label = name): AgreementEntry => ({ label, sel: { kind: "net", net: name }, view });
  const part = (ref: string, view: AgreementView): AgreementEntry => ({ label: ref, sel: { kind: "component", ref }, view });
  const pin = (p: string, view: AgreementView): AgreementEntry => {
    const dot = p.lastIndexOf(".");
    return { label: p, sel: { kind: "pin", ref: p.slice(0, dot), pin: p.slice(dot + 1) }, view };
  };
  const sections: AgreementSection[] = [
    { title: "Parts only on the schematic", entries: (rep?.componentsRemoved ?? []).map((r) => part(r, "schematic")) },
    { title: "Parts only on the board", entries: (rep?.componentsAdded ?? []).map((r) => part(r, "board")) },
    { title: "Nets only on the schematic", entries: ofKind("deleted").map((n) => net(n.name, "schematic")) },
    { title: "Nets only on the board", entries: ofKind("new").map((n) => net(n.name, "board")) },
    { title: "Nets whose pins differ", entries: ofKind("hard").map((n) => net(n.name, "schematic", `${n.name}: ${pinDelta(n.added, n.removed)}`)) },
    // The board draws a renamed net under the schematic's name, so the schematic's name selects it in
    // both views.
    { title: "Nets named differently", entries: ofKind("renamed").map((n) => net(n.oldName, "schematic", `${n.oldName} is ${n.name} on the board`)) },
    { title: "Pins on a net only on the schematic", entries: v.schematicOnlyPins.map((p) => pin(p, "schematic")) },
    { title: "Pins on a net only on the board", entries: v.boardOnlyPins.map((p) => pin(p, "board")) },
  ].filter((s) => s.entries.length > 0);
  return {
    compared: true,
    reason: "",
    summary: `${short(v.parts)} parts · ${short(v.nets)} nets`,
    lines: [
      `${long(v.parts)} parts on the board`,
      `${long(v.pins)} pins on a net on the board`,
      `${long(v.nets)} nets matched on the board`,
    ],
    sections,
  };
}

// pinDelta says how a net's pins differ, from the board's side.
function pinDelta(added: string[], removed: string[]): string {
  const parts: string[] = [];
  if (added.length > 0) parts.push(`the board adds ${added.join(" ")}`);
  if (removed.length > 0) parts.push(`the board lacks ${removed.join(" ")}`);
  return parts.join(" and ");
}

const n = (x: number) => x.toLocaleString("en-US");
const short = (t: ViewAgreement_Tally | undefined) => `${n(t?.matched ?? 0)}/${n(t?.schematic ?? 0)}`;
const long = (t: ViewAgreement_Tally | undefined) => `${n(t?.matched ?? 0)} of ${n(t?.schematic ?? 0)}`;
