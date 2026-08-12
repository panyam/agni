// The extraction bank helpers (WS13-006 PR 2, the manual backend). The bank splits by ownership:
// the PartSpec is the SHARED artifact (persisted server-side with optimistic locking, see
// regionview), while the workbench UI state (user-drawn regions + per-region routing types) is
// PER-USER working view kept in localStorage, so two people transcribing one datasheet do not
// clobber each other's boxes and tags. This module holds the pure builders, the protojson
// export/import, the empty-spec seed, and the UI-state localStorage codec.
import { create, toJsonString, fromJson } from "@bufbuild/protobuf";
import {
  PartSpecSchema,
  ParameterSchema,
  RangeValueSchema,
  ConditionSchema,
  ParamProvenanceSchema,
  PinSchema,
  PackageSchema,
  PinNumberSchema,
  LimitKind,
  ConditionCoverage,
  PinFunction,
  type PartSpec,
  type Parameter,
  type Pin,
  type Package,
} from "./gen/agni/v1/param/param_pb.js";
import {
  AnnotationSetSchema,
  RegionAnnotationSchema,
  type AnnotationSet,
} from "./gen/agni/v1/webapi/datasheet_pb.js";
import type { Region, RegionType } from "./regions.js";

// docId derives a source document's STABLE identity from its corpus path: the file stem
// (foo/LM1117.pdf -> "LM1117"). This is the id a SourceDoc carries and provenance.doc_ref cites,
// and it is deliberately NOT the doc-IR content_hash: content_hash is volatile (any byte change
// flips it, which is what makes it a freshness signal), so joining human work to it would orphan
// that work on re-extraction. The stem is deterministic and coordination-free, so two annotators
// derive the same id for one document with no shared registry, and it is unique within a part
// (the only scope doc_ref needs). A rename changes it; a future per-part manifest is the
// rename-safe upgrade (WS13-010).
export function docId(path: string): string {
  const base = path.split("/").pop() ?? path;
  const dot = base.lastIndexOf(".");
  return dot > 0 ? base.slice(0, dot) : base;
}

// REGION_ATTR is the Parameter.attributes key that links a parameter back to the region it was
// transcribed from, so coverage can mark that region done. provenance.table_or_figure carries the
// human citation (the region label); this carries the stable region id, which the citation is not.
export const REGION_ATTR = "region";

// emptySpec is a fresh PartSpec for a datasheet with no saved extraction: one SourceDoc keyed by
// the document's stable id (docId of its corpus path), with the path as its locator and the title
// pre-filled from the document (editable later), no parameters.
export function emptySpec(path: string, docTitle: string): PartSpec {
  return create(PartSpecSchema, { docs: [{ id: docId(path), title: docTitle, locator: path }] });
}

// exportSpecJson renders a PartSpec as pretty param protojson for download, the corpus format a
// param.LoadSet can ingest, not a UI-private shape.
export function exportSpecJson(spec: PartSpec): string {
  return toJsonString(PartSpecSchema, spec, { prettySpaces: 2 });
}

// importSpecJson parses a param protojson document back into a PartSpec (the inverse of export).
export function importSpecJson(json: string): PartSpec {
  return fromJson(PartSpecSchema, JSON.parse(json));
}

// UiState is the per-user, per-datasheet working view kept in localStorage: the regions the user
// drew with the marquee and the routing type assigned to each region id. Never shared, so it never
// conflicts; a co-editor sees their own boxes and tags.
export interface UiState {
  userRegions: Region[];
  types: Record<string, RegionType>;
}

// uiKey is the localStorage key for a datasheet's UI state. A JSON-encoded [mount, path] tuple is
// injective (no separator can collide with a mount name or path).
function uiKey(mount: string, path: string): string {
  return "agni.ds.ui/" + JSON.stringify([mount, path]);
}

// loadUiState reads a datasheet's UI state from localStorage, or an empty state when none exists or
// the stored value is unreadable.
export function loadUiState(mount: string, path: string): UiState {
  const raw = localStorage.getItem(uiKey(mount, path));
  if (!raw) return { userRegions: [], types: {} };
  try {
    const s = JSON.parse(raw) as UiState;
    return { userRegions: s.userRegions ?? [], types: s.types ?? {} };
  } catch {
    return { userRegions: [], types: {} };
  }
}

// saveUiState persists a datasheet's UI state to localStorage.
export function saveUiState(mount: string, path: string, ui: UiState): void {
  localStorage.setItem(uiKey(mount, path), JSON.stringify(ui));
}

// LayerVis is the workbench's overlay-visibility preference: which region layers are drawn. The
// doc-IR (auto) layers are split by kind so the exhaustive TEXT regions the extractor emits can be
// hidden without losing tables/figures; "mine"/"others" gate the two annotation layers. Visibility
// only — coverage counts stay over ALL regions regardless.
export interface LayerVis {
  table: boolean;
  figure: boolean;
  text: boolean;
  mine: boolean;
  others: boolean;
}

// DEFAULT_LAYERS: everything on EXCEPT doc-IR text, which is off by default because docling emits a
// region for every text block and the result is visually excessive; the user opts back into it.
export const DEFAULT_LAYERS: LayerVis = { table: true, figure: true, text: false, mine: true, others: true };

const LAYERS_KEY = "agni.ds.layers";

// loadLayers reads the per-browser layer-visibility preference, merged over DEFAULT_LAYERS so a
// newly added layer key picks up its default rather than reading as false.
export function loadLayers(): LayerVis {
  try {
    const raw = localStorage.getItem(LAYERS_KEY);
    return raw ? { ...DEFAULT_LAYERS, ...(JSON.parse(raw) as Partial<LayerVis>) } : { ...DEFAULT_LAYERS };
  } catch {
    return { ...DEFAULT_LAYERS };
  }
}

// saveLayers persists the layer-visibility preference (per browser, not per datasheet).
export function saveLayers(v: LayerVis): void {
  localStorage.setItem(LAYERS_KEY, JSON.stringify(v));
}

const AUTHOR_KEY = "agni.ds.author";

// getAuthor returns this browser's self-asserted annotation author id, generating and persisting
// one on first use. It NAMESPACES a user's overlay so co-editors compose (WS13-011); it is not
// authentication (the server treats it as an opaque namespace, mounts are the security boundary).
// A name-entry UI is future; today it is a stable per-browser token.
export function getAuthor(): string {
  let a = localStorage.getItem(AUTHOR_KEY);
  if (!a) {
    a = "user-" + Math.random().toString(36).slice(2, 10);
    localStorage.setItem(AUTHOR_KEY, a);
  }
  return a;
}

// uiToSet projects a localStorage UiState into the server AnnotationSet for one author: each
// user-drawn region becomes a RegionAnnotation carrying its geometry (kind "user"), and each type
// tag on a doc-IR region (one not user-drawn, so no geometry — the doc-IR holds it) becomes a
// region_id -> type annotation. This is the inverse of setToUi.
export function uiToSet(docIdVal: string, author: string, ui: UiState): AnnotationSet {
  const userIds = new Set(ui.userRegions.map((r) => r.id));
  const annotations = [
    ...ui.userRegions.map((r) =>
      create(RegionAnnotationSchema, {
        regionId: r.id,
        type: ui.types[r.id] ?? "",
        bbox: r.bbox,
        page: r.page ?? 0,
        kind: "user",
        label: r.label,
      }),
    ),
    ...Object.entries(ui.types)
      .filter(([id]) => !userIds.has(id))
      .map(([id, t]) => create(RegionAnnotationSchema, { regionId: id, type: t })),
  ];
  return create(AnnotationSetSchema, { docId: docIdVal, author, annotations });
}

// setToUi reconstructs a UiState from one author's AnnotationSet: user-drawn regions (kind "user"
// with geometry) rebuild userRegions; every annotation's non-empty type rebuilds the type map. The
// inverse of uiToSet, used to load an author's own overlay back into the editable working view.
export function setToUi(set: AnnotationSet): UiState {
  const userRegions: Region[] = [];
  const types: Record<string, RegionType> = {};
  for (const a of set.annotations) {
    if (a.type) types[a.regionId] = a.type as RegionType;
    if (a.kind === "user" && a.bbox) {
      userRegions.push({ id: a.regionId, kind: "user", label: a.label || "user region", bbox: a.bbox, page: a.page });
    }
  }
  return { userRegions, types };
}

// otherUserRegions collects the user-DRAWN boxes from every author except `me`, for read-only
// display so a teammate's marquee is visible (WS13-011 compose). Their ids are namespaced by author
// so they never collide with the caller's own region ids, and they are never added to the editable
// userRegions. Other authors' TYPE tags are deliberately not merged: reconciling conflicting tags is
// canonicalization's job (WS13-012), not the viewer's.
export function otherUserRegions(sets: AnnotationSet[], me: string): Region[] {
  const out: Region[] = [];
  for (const s of sets) {
    if (s.author === me) continue;
    for (const a of s.annotations) {
      if (a.kind === "user" && a.bbox) {
        out.push({
          id: `${s.author}:${a.regionId}`,
          kind: "user",
          label: `${a.label || "region"} (${s.author})`,
          bbox: a.bbox,
          page: a.page,
        });
      }
    }
  }
  return out;
}

// NewParamFields is the transcribe editor's input for one parameter row, before it becomes a
// param.Parameter. Empty numeric fields are left unset (RangeValue has explicit presence); a
// condition is captured as raw text only, which stays a captured condition but is not machine-
// comparable (no structured eq/min/max), so coverage is COMPLETE, never silently a scalar.
export interface NewParamFields {
  name: string;
  symbol: string;
  limitKind: LimitKind;
  min?: number;
  typ?: number;
  max?: number;
  unit: string;
  condition: string;
}

// newParameter builds a param.Parameter from editor fields and the region it was transcribed from,
// stamping provenance (page + region label as the citation, method "hand", confidence 1.0) and the
// region-id link. This is the manual backend's output: a value with conditions and provenance,
// never a bare scalar.
export function newParameter(f: NewParamFields, region: Region, page: number, docRef: string): Parameter {
  const conditions = f.condition.trim()
    ? [create(ConditionSchema, { raw: f.condition.trim() })]
    : [];
  return create(ParameterSchema, {
    name: f.name,
    symbol: f.symbol,
    limitKind: f.limitKind,
    value: create(RangeValueSchema, { min: f.min, typ: f.typ, max: f.max }),
    unit: f.unit,
    conditions,
    conditionCoverage: conditions.length ? ConditionCoverage.COMPLETE : ConditionCoverage.UNCONDITIONAL,
    attributes: { [REGION_ATTR]: region.id },
    prov: create(ParamProvenanceSchema, {
      docRef,
      page,
      tableOrFigure: region.label || region.id,
      method: "hand",
      confidence: 1.0,
    }),
  });
}

// paramsForRegion returns the parameters transcribed against a region id (via REGION_ATTR), so the
// transcribe panel lists a region's own rows and coverage can tell a worked region from a pending one.
export function paramsForRegion(spec: PartSpec, regionId: string): Parameter[] {
  return spec.parameters.filter((p) => p.attributes[REGION_ATTR] === regionId);
}

// NewPinFields is the transcribe editor's input for one pin, before it becomes a param.Pin. The id
// is the author's rather than generated: it is what Parameter.pin_refs points at and what a
// validation message names, so an opaque generated key would make both harder to read.
export interface NewPinFields {
  id: string;
  name: string;
  fn: PinFunction;
  description: string;
}

// newPin builds a param.Pin from editor fields and the region it was transcribed from, stamping the
// same provenance newParameter does (page + region label, method "hand", confidence 1.0).
//
// Provenance is not decoration here: param.Validate REQUIRES it on every pin, on the same grounds it
// requires it on every value. A pin function is an extracted claim, and one nobody can check against
// a page is a liability. Anchoring to the region the author is already looking at makes that free
// rather than a form field they would fill in twice.
export function newPin(f: NewPinFields, region: Region, page: number, docRef: string): Pin {
  return create(PinSchema, {
    id: f.id.trim(),
    name: f.name.trim(),
    function: f.fn,
    description: f.description.trim(),
    attributes: { [REGION_ATTR]: region.id },
    prov: create(ParamProvenanceSchema, {
      docRef,
      page,
      tableOrFigure: region.label || region.id,
      method: "hand",
      confidence: 1.0,
    }),
  });
}

// derivePinId turns a pin's printed name into a spec-local id, suffixing when the obvious id is
// already taken: NC, then nc2, then nc3.
//
// The suffix is not a nicety. A part that prints ONE NAME ON SEVERAL TERMINALS is precisely the case
// pin binding exists for, and the seeded TXB0104 is one: it prints NC twice. Deriving from the name
// alone hands those two pins the same id, which two pins may never share, so the author would be
// walked into a rejected save on the exact part the contract was designed around. Returns "" for an
// empty name, which the caller treats as "not ready to add" rather than as an id.
export function derivePinId(name: string, taken: Iterable<string>): string {
  const base = name.trim().toLowerCase().replace(/\s+/g, "_");
  if (!base) return "";
  const used = new Set(taken);
  if (!used.has(base)) return base;
  for (let n = 2; ; n++) {
    const candidate = `${base}${n}`;
    if (!used.has(candidate)) return candidate;
  }
}

// newPackage declares one body the part ships in. It carries no provenance: a package is the label a
// pin number is relative to rather than a claim about the part's behaviour, and param.Validate asks
// nothing of it beyond a unique id.
export function newPackage(id: string, name: string, mpnSuffix = ""): Package {
  return create(PackageSchema, { id: id.trim(), name: name.trim(), mpnSuffix: mpnSuffix.trim() });
}

// pinsForRegion returns the pins transcribed against a region id, the pin counterpart of
// paramsForRegion, so a panel can show what a region has yielded so far.
export function pinsForRegion(spec: PartSpec, regionId: string): Pin[] {
  return spec.pins.filter((p) => p.attributes[REGION_ATTR] === regionId);
}

// setPinNumber records a pin's designator within one package, REPLACING any existing entry for that
// package rather than appending. An empty number removes the entry, which is how a mistyped
// designator is cleared; leaving it would have the pin claim a terminal it does not have, and two
// pins claiming one number in a package is exactly what ValidatePins rejects.
export function setPinNumber(pin: Pin, packageRef: string, number: string): void {
  const rest = pin.numbers.filter((n) => n.packageRef !== packageRef);
  const trimmed = number.trim();
  pin.numbers = trimmed ? [...rest, create(PinNumberSchema, { packageRef, number: trimmed })] : rest;
}

// bindParam binds a parameter to a terminal, idempotently. Several calls express a row the datasheet
// states once for a group of pins.
export function bindParam(p: Parameter, pinId: string): void {
  if (!p.pinRefs.includes(pinId)) p.pinRefs = [...p.pinRefs, pinId];
}

// unbindParam removes one terminal from a parameter's binding. Removing the last one returns the row
// to part-wide, which is a meaningful state (a die-level rating) rather than an error.
export function unbindParam(p: Parameter, pinId: string): void {
  p.pinRefs = p.pinRefs.filter((r) => r !== pinId);
}

