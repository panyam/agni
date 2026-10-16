// The Properties panel's model: what is selected, at which level, and what the design says about it
// (agni issue 902).
//
// A canvas click used to write a query into the query panel and run it, which mixed looking around
// with asking a question and overwrote whatever the reader had typed. A click now selects, and this
// module describes the selection from data the viewer already has: the design's own IR (one masked
// GetDesign per open), the findings of the run on screen, and the datasheet join. Asking a question
// about the selection is a separate act, an Ask row, which opens the served question in a query tab.
//
// Selection is a hierarchy with something always selected. The design is the root, the sheet on
// screen is under it, and an entity (a component, net, pin or bus) is under the sheet it was picked
// on. Everything here is pure, so the panel and its tests share one derivation.

import type { Component, Design, Net } from "./gen/agni/v1/ir/ir_pb.js";
import { ClassSource, Role } from "./gen/agni/v1/ir/ir_pb.js";
import type { PartSpec } from "./gen/agni/v1/param/param_pb.js";
import type { ConversionReport } from "./gen/agni/v1/webapi/design_pb.js";
import { checkedState, findingsFor, type FindingItem, type FindingsState } from "./findings.js";
import type { EntityQueryItem } from "./query.js";
import { labelFor, type Selection } from "./selection.js";

// Focus is what Properties describes. An entity carries the sheet it was picked on, so its breadcrumb
// reads Design › Sheet › J1 › pin 2 rather than jumping straight from the design to the pin.
export type Focus =
  | { level: "design" }
  | { level: "sheet"; sheetId: string }
  | { level: "entity"; sel: Selection; sheetId?: string };

// PIN_SEP joins a component and a pin designator into one map key. A designator can contain any
// printable character, and NUL is the one no reader writes.
const PIN_SEP = "\u0000";

// PinNet is one of a component's pins and the net it lands on, "" when the pin is unconnected.
export interface PinNet {
  pin: string;
  name: string;
  net: string;
}

// Inventory indexes the design's IR for the questions Properties asks, built once per open.
export interface Inventory {
  components: Map<string, Component>;
  nets: Map<string, Net>;
  // pinsOf lists every connected pin of a component with its net, sorted by designator.
  pinsOf: Map<string, PinNet[]>;
  // pinName maps ref + PIN_SEP + designator to the pin's functional name off its part type, which is
  // what a datasheet calls it, beside the designator every other pin relation keys on.
  pinName: Map<string, string>;
  // buses holds each bus a reader recorded, by label: the constructs it was drawn as and the member
  // signals the reader could name. The netlist has no bus as such, so this is the read's own record.
  buses: Map<string, { constructs: string[]; members: string[] }>;
}

// INVENTORY_PATHS is the read_mask for the one GetDesign Properties makes per open. It names the
// fields the panel shows and nothing else, so provenance and section attributes stay on the server:
// on the largest seed that is about 1.8 MB of JSON against 3.9 MB for the three collections whole.
export const INVENTORY_PATHS = [
  "design.components.ref_des",
  "design.components.mpn",
  "design.components.value",
  "design.components.device_classes",
  "design.components.attributes",
  "design.components.sections.part_ref",
  "design.components.sections.library_ref",
  "design.nets.name",
  "design.nets.id",
  "design.nets.roles",
  "design.nets.net_classes",
  "design.nets.attributes",
  "design.nets.connections.component_ref",
  "design.nets.connections.pin_ref",
  "design.libraries.name",
  "design.libraries.parts.name",
  "design.libraries.parts.pins.designator",
  "design.libraries.parts.pins.name",
  "design.input_diagnostics.unmodeled_buses.label",
  "design.input_diagnostics.unmodeled_buses.kind",
  "design.input_diagnostics.unmodeled_buses.members",
];

// byDesignator orders pin designators the way a datasheet does, so pin 2 sorts before pin 10.
const byDesignator = new Intl.Collator(undefined, { numeric: true }).compare;

// inventoryFromDesign indexes a design read with INVENTORY_PATHS.
export function inventoryFromDesign(d: Design): Inventory {
  const components = new Map(d.components.map((c) => [c.refDes, c]));
  const nets = new Map(d.nets.map((n) => [n.name, n]));
  const pinsOf = new Map<string, PinNet[]>();
  const pinName = new Map<string, string>();
  const partPins = new Map<string, Map<string, string>>();
  for (const lib of d.libraries) {
    for (const part of lib.parts) {
      partPins.set(lib.name + PIN_SEP + part.name, new Map(part.pins.map((p) => [p.designator, p.name])));
    }
  }
  for (const c of d.components) {
    for (const s of c.sections) {
      for (const [designator, name] of partPins.get(s.libraryRef + PIN_SEP + s.partRef) ?? []) {
        if (name && name !== designator) pinName.set(c.refDes + PIN_SEP + designator, name);
      }
    }
  }
  for (const n of d.nets) {
    for (const conn of n.connections) {
      const list = pinsOf.get(conn.componentRef) ?? [];
      list.push({ pin: conn.pinRef, name: pinName.get(conn.componentRef + PIN_SEP + conn.pinRef) ?? "", net: n.name });
      pinsOf.set(conn.componentRef, list);
    }
  }
  for (const list of pinsOf.values()) list.sort((a, b) => byDesignator(a.pin, b.pin));
  const buses = new Map<string, { constructs: string[]; members: string[] }>();
  for (const b of d.inputDiagnostics?.unmodeledBuses ?? []) {
    if (!b.label) continue;
    const at = buses.get(b.label) ?? { constructs: [], members: [] };
    if (!at.constructs.includes(b.kind)) at.constructs.push(b.kind);
    for (const m of b.members) if (!at.members.includes(m)) at.members.push(m);
    buses.set(b.label, at);
  }
  for (const b of buses.values()) b.members.sort(byDesignator);
  return { components, nets, pinsOf, pinName, buses };
}

// Datasheet is the datasheet section's outcome. The three a reader acts on differently are kept
// apart: nothing was asked (no corpus), the corpus was asked and has no spec for this part number,
// or here is the spec. A part with no MPN is a fourth case, since there was nothing to look up.
export type Datasheet =
  | { kind: "no-corpus" }
  | { kind: "no-mpn" }
  | { kind: "no-spec"; mpn: string }
  | { kind: "spec"; mpn: string; spec: PartSpec; corpus: string };

// DatasheetJoin is what the parts panel already fetched (GetComponentParams), shared with Properties.
export interface DatasheetJoin {
  corpusConnected: boolean;
  parts: { refDes: string; mpn: string; spec: PartSpec | undefined; corpus?: string }[];
}

// Ref is one reference in a group. A target makes it clickable, and clicking moves the selection to
// that entity, or to that sheet for a sheet reference.
export interface Ref {
  label: string;
  detail?: string;
  target?: Selection;
  sheet?: string;
}

export interface Group {
  name: string;
  refs: Ref[];
  // empty is what the group says when it has no refs, so an absent answer reads as a statement.
  empty?: string;
}

export interface Row {
  label: string;
  value: string;
}

export interface Crumb {
  label: string;
  focus: Focus;
}

// PropertiesState is everything the panel renders for one focus.
export interface PropertiesState {
  crumbs: Crumb[];
  kind: string;
  title: string;
  rows: Row[];
  groups: Group[];
  datasheet?: Datasheet;
  // report is the auto-layout conversion report, shown at the design level as Details showed it.
  report?: ConversionReport | null;
  // ask is the selection the Ask rows ask about and the served questions for its kind, absent where
  // nothing is selected that a question can bind.
  ask?: { sel: Selection; questions: EntityQueryItem[] };
  // note explains a gap in what the panel can say, such as the inventory still loading.
  note?: string;
}

// PropertiesInput is what the panel's host holds when it asks for a state.
export interface PropertiesInput {
  focus: Focus;
  designName: string;
  summary: string;
  sheets: { id: string; name: string }[];
  report: ConversionReport | null;
  inventory: Inventory | null;
  findings: FindingsState | null;
  datasheet: DatasheetJoin | null;
  // drawn is what the sheet on screen draws, read off the rendered document, since a sheet's
  // membership is a property of its drawing rather than of the netlist.
  drawn: { refs: string[]; nets: string[] } | null;
  // questions are the served questions about a selection, every kind's.
  questions: EntityQueryItem[];
  // drawnOn is the sheets the selected entity is drawn on, as the engine located it: null while that
  // answer is on its way, and absent where nobody asks (a host with no engine to ask).
  drawnOn?: string[] | null;
}

// propertiesFor derives the panel's state for one focus.
export function propertiesFor(input: PropertiesInput): PropertiesState {
  const crumbs = crumbsFor(input);
  const f = input.focus;
  if (f.level === "design") return { crumbs, ...designView(input) };
  if (f.level === "sheet") return { crumbs, ...sheetView(input, f.sheetId) };
  return { crumbs, ...entityView(input, f.sel) };
}

function sheetName(input: PropertiesInput, id: string | undefined): string {
  return input.sheets.find((s) => s.id === id)?.name || id || "";
}

function crumbsFor(input: PropertiesInput): Crumb[] {
  const out: Crumb[] = [{ label: input.designName || "Design", focus: { level: "design" } }];
  const f = input.focus;
  const sheetId = f.level === "sheet" ? f.sheetId : f.level === "entity" ? f.sheetId : undefined;
  if (sheetId) out.push({ label: sheetName(input, sheetId), focus: { level: "sheet", sheetId } });
  if (f.level !== "entity") return out;
  if (f.sel.kind === "pin") {
    out.push({ label: f.sel.ref ?? "", focus: { level: "entity", sel: { kind: "component", ref: f.sel.ref }, sheetId } });
    out.push({ label: `pin ${f.sel.pin}`, focus: f });
  } else {
    out.push({ label: labelFor(f.sel), focus: f });
  }
  return out;
}

// findingsLine states a count with the run state that gives it meaning, since a zero before any run
// is not a clean design (web-picking.md, "A count needs its state or it lies").
function findingsLine(fs: FindingsState | null, items: FindingItem[]): string {
  if (!fs) return "not checked yet";
  switch (checkedState(fs)) {
    case "loading":
      return "rules loading";
    case "no-rules":
      return "no rules selected";
    case "running":
      return "running";
    case "not-run":
      return "not checked yet";
    case "partial":
      return `at least ${countWords(items)}, some selected rules not run`;
    case "complete":
      return countWords(items);
  }
}

function countWords(items: FindingItem[]): string {
  const unresolved = items.filter((f) => f.inconclusive).length;
  const defects = items.length - unresolved;
  const head = `${defects} finding${defects === 1 ? "" : "s"}`;
  return unresolved ? `${head}, ${unresolved} unresolved` : head;
}

function findingRefs(items: FindingItem[]): Ref[] {
  return items.map((f) => ({ label: f.rule, detail: f.message }));
}

function designView(input: PropertiesInput): Omit<PropertiesState, "crumbs"> {
  const all = input.findings?.findings ?? [];
  const rows: Row[] = [];
  if (input.summary) rows.push({ label: "read", value: input.summary });
  if (input.inventory) {
    rows.push({ label: "components", value: String(input.inventory.components.size) });
    rows.push({ label: "nets", value: String(input.inventory.nets.size) });
  }
  rows.push({ label: "sheets", value: String(input.sheets.length) });
  rows.push({ label: "findings", value: findingsLine(input.findings, all) });
  if (input.datasheet) {
    const seeded = input.datasheet.parts.length;
    rows.push({
      label: "datasheets",
      value: input.datasheet.corpusConnected ? `${seeded} part${seeded === 1 ? "" : "s"} with a spec` : "no corpus connected",
    });
  }
  const groups: Group[] = [
    {
      name: "sheets",
      refs: input.sheets.map((s) => ({ label: s.name || s.id })),
    },
  ];
  return { kind: "design", title: input.designName || "Design", rows, groups, report: input.report };
}

function sheetView(input: PropertiesInput, sheetId: string): Omit<PropertiesState, "crumbs"> {
  const name = sheetName(input, sheetId);
  const all = input.findings?.findings ?? [];
  const onSheet = all.filter((f) => f.sheets.some((s) => s.id === sheetId));
  const refs = [...(input.drawn?.refs ?? [])].sort(byDesignator);
  const nets = [...(input.drawn?.nets ?? [])].sort(byDesignator);
  return {
    kind: "sheet",
    title: name,
    rows: [{ label: "findings", value: findingsLine(input.findings, onSheet) }],
    groups: [
      { name: "components", refs: refs.map((r) => ({ label: r, target: { kind: "component", ref: r } })), empty: "none drawn" },
      { name: "nets", refs: nets.map((n) => ({ label: n, target: { kind: "net", net: n } })), empty: "none drawn" },
      { name: "findings", refs: findingRefs(onSheet), empty: "none" },
    ],
    note: input.drawn ? undefined : "What this sheet draws is read off its SVG drawing, and none is on screen.",
  };
}

function entityView(input: PropertiesInput, sel: Selection): Omit<PropertiesState, "crumbs"> {
  const mine = input.findings ? findingsFor(input.findings.findings, [sel]) : [];
  const findingsGroup: Group = { name: "findings", refs: findingRefs(mine), empty: input.findings && checkedState(input.findings) === "complete" ? "none" : "not checked yet" };
  const questions = input.questions.filter((q) => q.kind === sel.kind);
  const base = { kind: sel.kind, title: labelFor(sel), ask: questions.length ? { sel, questions } : undefined };
  const sheetsGroup: Group[] =
    input.drawnOn === undefined
      ? []
      : [
          {
            name: "sheets",
            refs: (input.drawnOn ?? []).map((id) => ({ label: sheetName(input, id), sheet: id })),
            empty: input.drawnOn ? "drawn on no sheet" : "locating…",
          },
        ];
  const inv = input.inventory;
  const loading = inv ? undefined : "Reading the design…";
  switch (sel.kind) {
    case "component": {
      const c = inv?.components.get(sel.ref ?? "");
      const pins = inv?.pinsOf.get(sel.ref ?? "") ?? [];
      const nets = [...new Set(pins.map((p) => p.net))].sort(byDesignator);
      return {
        ...base,
        rows: c ? componentRows(c) : [],
        groups: [
          ...sheetsGroup,
          { name: "pins", refs: pins.map((p) => pinRef(sel.ref ?? "", p)), empty: "no connected pins" },
          { name: "nets", refs: nets.map((n) => ({ label: n, target: { kind: "net", net: n } })), empty: "none" },
          findingsGroup,
          { name: "attributes", refs: attributeRefs(c?.attributes), empty: "none" },
        ],
        datasheet: c ? datasheetFor(c, input.datasheet) : undefined,
        note: loading ?? (c ? undefined : "This design's netlist has no component by that name."),
      };
    }
    case "net": {
      const n = inv?.nets.get(sel.net ?? "");
      const conns = [...(n?.connections ?? [])].sort((a, b) => byDesignator(a.componentRef, b.componentRef) || byDesignator(a.pinRef, b.pinRef));
      const comps = [...new Set(conns.map((c) => c.componentRef))];
      const testPoints = comps.filter((r) => inv?.components.get(r)?.deviceClasses.some((t) => t.class === "test_point"));
      return {
        ...base,
        rows: n ? netRows(n) : [],
        groups: [
          ...sheetsGroup,
          {
            name: "pins",
            refs: conns.map((c) => ({
              label: `${c.componentRef}.${c.pinRef}`,
              detail: inv?.pinName.get(c.componentRef + PIN_SEP + c.pinRef),
              target: { kind: "pin", ref: c.componentRef, pin: c.pinRef },
            })),
            empty: "no pins",
          },
          { name: "components", refs: comps.map((r) => ({ label: r, target: { kind: "component", ref: r } })), empty: "none" },
          { name: "test points", refs: testPoints.map((r) => ({ label: r, target: { kind: "component", ref: r } })), empty: "none" },
          findingsGroup,
          { name: "attributes", refs: attributeRefs(n?.attributes), empty: "none" },
        ],
        note: loading ?? (n ? undefined : "This design's netlist has no net by that name."),
      };
    }
    case "pin": {
      const ref = sel.ref ?? "";
      const pin = sel.pin ?? "";
      const at = inv?.pinsOf.get(ref)?.find((p) => p.pin === pin);
      const name = inv?.pinName.get(ref + PIN_SEP + pin) ?? "";
      const rows: Row[] = [{ label: "designator", value: pin }];
      if (name) rows.push({ label: "name", value: name });
      rows.push({ label: "net", value: at?.net || "unconnected" });
      return {
        ...base,
        rows,
        groups: [
          { name: "part", refs: [{ label: ref, target: { kind: "component", ref } }] },
          { name: "net", refs: at?.net ? [{ label: at.net, target: { kind: "net", net: at.net } }] : [], empty: "unconnected" },
          findingsGroup,
        ],
        note: loading,
      };
    }
    case "bus": {
      const b = inv?.buses.get(sel.busId ?? "");
      return {
        ...base,
        rows: b ? [{ label: "drawn as", value: b.constructs.join(", ") }] : [],
        groups: [
          {
            name: "members",
            // A member the netlist also names is a net to walk to. One it does not is the bus's own
            // name for a signal the read never connected, which is worth seeing rather than hiding.
            refs: (b?.members ?? []).map((m) =>
              inv?.nets.has(m) ? { label: m, target: { kind: "net", net: m } } : { label: m, detail: "no net by this name" },
            ),
            empty: "the read recorded no members for this bus",
          },
          findingsGroup,
        ],
        note: loading ?? (b ? undefined : "The netlist read records no bus by this name, which happens when the drawing comes from a companion file."),
      };
    }
  }
}

// pinRef is a component's pin as a reference: the designator, its functional name when the part
// type gives one, and the net it lands on.
function pinRef(ref: string, p: PinNet): Ref {
  const detail = [p.name, p.net ? `→ ${p.net}` : "unconnected"].filter(Boolean).join(" ");
  return { label: p.pin, detail, target: { kind: "pin", ref, pin: p.pin } };
}

function componentRows(c: Component): Row[] {
  const rows: Row[] = [];
  if (c.mpn) rows.push({ label: "mpn", value: c.mpn });
  // The value as the design wrote it ("10k"), which is what a reader compares against the drawing.
  const value = c.value?.input || (c.value?.value !== undefined ? `${c.value.value}${c.value.unit ? " " + c.value.unit : ""}` : "");
  if (value) rows.push({ label: "value", value });
  if (c.deviceClasses.length) {
    rows.push({ label: "classes", value: c.deviceClasses.map((t) => `${t.class} (${classSourceLabel(t.source)})`).join(", ") });
  }
  return rows;
}

// classSourceLabel names the evidence tier a class came from, since a class from a datasheet is a
// stronger claim than one from a ref-des prefix.
export function classSourceLabel(s: ClassSource): string {
  switch (s) {
    case ClassSource.CONVENTION:
      return "convention";
    case ClassSource.DATASHEET:
      return "datasheet";
    default:
      return "unstated";
  }
}

function netRows(n: Net): Row[] {
  const rows: Row[] = [];
  const roles = n.roles.map((r) => roleLabel(r.roleKind)).filter(Boolean);
  rows.push({ label: "roles", value: roles.length ? roles.join(", ") : "none" });
  if (n.netClasses.length) rows.push({ label: "net classes", value: n.netClasses.join(", ") });
  rows.push({ label: "pins", value: String(n.connections.length) });
  return rows;
}

// roleLabel turns ROLE_RAIL into "rail", the spelling the query language uses.
export function roleLabel(r: Role): string {
  const name = Role[r];
  if (!name || r === Role.UNSPECIFIED) return "";
  return name.toLowerCase();
}

function attributeRefs(attrs: { [k: string]: string } | undefined): Ref[] {
  return Object.entries(attrs ?? {})
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => ({ label: k, detail: v }));
}

function datasheetFor(c: Component, join: DatasheetJoin | null): Datasheet | undefined {
  if (!join) return undefined;
  if (!join.corpusConnected) return { kind: "no-corpus" };
  const hit = join.parts.find((p) => p.refDes === c.refDes);
  if (hit?.spec) return { kind: "spec", mpn: hit.mpn, spec: hit.spec, corpus: hit.corpus ?? "" };
  if (!c.mpn) return { kind: "no-mpn" };
  return { kind: "no-spec", mpn: c.mpn };
}

// ReportLines is the conversion report as the design level shows it: a count per label, then the
// components drawn as plain boxes and the ones whose symbol did not resolve, which need a
// --symbol-path. Empty for a faithful drawing, which classifies nothing.
export interface ReportLines {
  counts: string;
  box: string[];
  unresolved: string[];
}

export function reportLines(report: ConversionReport | null | undefined): ReportLines | null {
  if (!report || report.components.length === 0) return null;
  // Each component's group is the engine's label (a glyph's device class, else how it was drawn).
  const byLabel = new Map<string, string[]>();
  for (const c of report.components) {
    const label = c.label;
    const list = byLabel.get(label) ?? [];
    list.push(c.refDes);
    byLabel.set(label, list);
  }
  const counts = [...byLabel.entries()]
    .sort((a, b) => a[0].localeCompare(b[0]))
    .map(([label, refs]) => `${label} ${refs.length}`)
    .join(" · ");
  return { counts, box: byLabel.get("box") ?? [], unresolved: byLabel.get("unresolved") ?? [] };
}
