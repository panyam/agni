// The extraction bank helpers, the manual backend (WS13-006). The PartSpec is SHARED, saved
// server-side with optimistic locking (see regionview). The workbench UI state (user-drawn regions
// and per-region routing types) is PER-USER and lives in localStorage, so two people transcribing
// one datasheet do not clobber each other's boxes and tags.
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
  PinRelationSchema,
  VerificationSchema,
  LimitKind,
  ConditionCoverage,
  PinFunction,
  PinRelationKind,
  Modality,
  type PartSpec,
  type Parameter,
  type Pin,
  type Package,
  type PinRelation,
  type SourceDoc,
  type Verification,
} from "./gen/agni/v1/param/param_pb.js";
import {
  AnnotationSetSchema,
  RegionAnnotationSchema,
  type AnnotationSet,
} from "./gen/agni/v1/webapi/datasheet_pb.js";
import type { Region, RegionType } from "./regions.js";

// docId derives a source document's STABLE identity from its corpus path, the file stem
// (foo/LM1117.pdf -> "LM1117"). It is the id a SourceDoc carries and provenance.doc_ref cites.
// It is NOT the doc-IR content_hash, which any byte change flips, so human work joined to it would
// be orphaned on re-extraction. Two annotators derive the same stem with no shared registry, and it
// is unique within a part, the only scope doc_ref needs. A rename changes it (WS13-010).
export function docId(path: string): string {
  const base = path.split("/").pop() ?? path;
  const dot = base.lastIndexOf(".");
  return dot > 0 ? base.slice(0, dot) : base;
}

// REGION_ATTR is the Parameter.attributes key that links a parameter back to the region it was
// transcribed from, so coverage can mark that region done. provenance.table_or_figure carries the
// human citation (the region label), and this carries the stable region id.
export const REGION_ATTR = "region";

// emptySpec is a fresh PartSpec for a datasheet with no saved extraction. It holds one SourceDoc
// keyed by docId(path), with the path as its locator, the document's title (editable later), and
// the revision the corpus currently holds.
//
// contentHash is the doc-IR Document.content_hash, and recording it lets a human verification
// EXPIRE when the vendor reissues the document (param.VerificationOfIn compares the two). With no
// revision a verification can only answer "unknown", which the review layer distrusts. Empty is
// allowed, since an un-extracted datasheet has no doc-IR yet, and means staleness cannot be decided.
export function emptySpec(path: string, docTitle: string, contentHash: string): PartSpec {
  return create(PartSpecSchema, {
    docs: [{ id: docId(path), title: docTitle, locator: path, contentHash }],
  });
}

// adoptDocRevision records the revision the corpus now holds on the spec's first SourceDoc,
// reporting whether it changed anything.
//
// A spec saved before the workbench recorded a hash has none (#289), and a verification added to it
// would read "unknown" forever, carrying a human's name the review layer distrusts. So the hash is
// filled in on load as well as at creation.
//
// It does NOT overwrite an existing hash that disagrees. A disagreement is a real re-seed, and
// adopting the new hash would re-validate every verification pinned to the old revision.
export function adoptDocRevision(spec: PartSpec, contentHash: string): boolean {
  const d = spec.docs[0];
  if (!d || !contentHash || d.contentHash) return false;
  d.contentHash = contentHash;
  return true;
}

// handVerification is the record that a PERSON transcribed a value off the page, against the
// revision in front of them.
//
// Hand transcription IS a human confirmation (confidence 1.0). Pinning the revision makes the claim
// expire when the datasheet is reissued (#289). Contrast candidate.Accept, which refuses to mark a
// machine proposal verified, because nobody read that page.
//
// Returns undefined when the document records no revision, because a verification that cannot be
// invalidated is the failure the type exists to prevent. The value then saves unverified.
export function handVerification(doc: SourceDoc | undefined, by: string, at: string): Verification | undefined {
  if (!doc?.contentHash || !by) return undefined;
  return create(VerificationSchema, {
    by,
    docContentHash: doc.contentHash,
    // Snapshotted here because a re-seed rewrites SourceDoc.title, and the checked revision's name
    // would be lost.
    docRevision: doc.title,
    at,
  });
}

// today is the verification date in the ISO-8601 form Verification.at wants. Separate so a test can
// assert the record's shape without depending on the day it runs.
export function today(): string {
  return new Date().toISOString().slice(0, 10);
}

// docRevisionNote says whether this document's revision is recorded, which decides whether anything
// transcribed against it can be confirmed at all.
//
// It reports the CONSEQUENCE rather than the hash, since an author can act on that. The short hash
// prefix only shows that a revision is pinned.
export function docRevisionNote(contentHash: string): string {
  if (!contentHash) return "No revision recorded for this document: transcriptions save unverified.";
  const short = contentHash.startsWith("sha256:") ? contentHash.slice(7, 19) : contentHash.slice(0, 12);
  return `Revision ${short} — transcriptions are confirmed against it and expire when it changes.`;
}

// exportSpecJson renders a PartSpec as pretty param protojson for download, the corpus format
// param.LoadSet ingests.
export function exportSpecJson(spec: PartSpec): string {
  return toJsonString(PartSpecSchema, spec, { prettySpaces: 2 });
}

// importSpecJson parses a param protojson document back into a PartSpec (the inverse of export).
export function importSpecJson(json: string): PartSpec {
  return fromJson(PartSpecSchema, JSON.parse(json));
}

// UiState is the per-user, per-datasheet working view kept in localStorage, holding the regions the
// user drew with the marquee and the routing type assigned to each region id.
export interface UiState {
  userRegions: Region[];
  types: Record<string, RegionType>;
}

// uiKey is the localStorage key for a datasheet's UI state. JSON-encoding [mount, path] means no
// separator can collide with a mount name or path.
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

// LayerVis says which region layers the workbench draws. The doc-IR layers are split by kind so the
// extractor's TEXT regions can be hidden without losing tables and figures, and "mine"/"others" gate
// the two annotation layers. Coverage counts stay over ALL regions regardless.
export interface LayerVis {
  table: boolean;
  figure: boolean;
  text: boolean;
  mine: boolean;
  others: boolean;
}

// DEFAULT_LAYERS turns everything on EXCEPT doc-IR text, because docling emits a region for every
// text block.
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
// one on first use. It NAMESPACES a user's overlay so co-editors compose (WS13-011). It is not
// authentication, since the server treats it as opaque and mounts are the security boundary.
export function getAuthor(): string {
  let a = localStorage.getItem(AUTHOR_KEY);
  if (!a) {
    a = "user-" + Math.random().toString(36).slice(2, 10);
    localStorage.setItem(AUTHOR_KEY, a);
  }
  return a;
}

// uiToSet projects a UiState into one author's AnnotationSet, the inverse of setToUi. A user-drawn
// region becomes a RegionAnnotation with its geometry (kind "user"). A type tag on a doc-IR region
// becomes a bare region_id -> type annotation, since the doc-IR holds that region's geometry.
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

// setToUi rebuilds a UiState from one author's AnnotationSet, the inverse of uiToSet, to load an
// author's own overlay back into the editable view. Annotations of kind "user" with geometry become
// userRegions, and every non-empty type goes into the type map.
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
// display (WS13-011). Ids are prefixed with the author so they never collide with the caller's own,
// and they never enter the editable userRegions. Other authors' TYPE tags are not merged, because
// reconciling conflicting tags belongs to canonicalization (WS13-012).
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
// param.Parameter. Empty numeric fields stay unset, since RangeValue has explicit presence. A
// condition is captured as raw text only, so it is not machine-comparable, but coverage is still
// COMPLETE rather than the row reading as a bare scalar.
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
// stamping provenance (page + region label as the citation, method "hand", confidence 1.0), the
// region-id link, and the verification recording who transcribed it against which revision.
//
// verification is undefined when the document records no revision (see handVerification), and the
// value then saves unverified rather than being refused.
export function newParameter(
  f: NewParamFields,
  region: Region,
  page: number,
  docRef: string,
  verification?: Verification,
): Parameter {
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
    verification,
  });
}

// paramsForRegion returns the parameters transcribed against a region id (via REGION_ATTR), so the
// transcribe panel lists a region's own rows and coverage can tell a worked region from a pending one.
export function paramsForRegion(spec: PartSpec, regionId: string): Parameter[] {
  return spec.parameters.filter((p) => p.attributes[REGION_ATTR] === regionId);
}

// NewPinFields is the transcribe editor's input for one pin, before it becomes a param.Pin. The id
// is the author's rather than generated, because Parameter.pin_refs points at it and validation
// messages name it.
export interface NewPinFields {
  id: string;
  name: string;
  fn: PinFunction;
  description: string;
}

// newPin builds a param.Pin from editor fields and the region it was transcribed from, stamping the
// same provenance newParameter does (page + region label, method "hand", confidence 1.0).
//
// param.Validate REQUIRES provenance on every pin, as it does on every value. Anchoring to the region
// the author is looking at supplies it without another form field.
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
// already taken (nc, then nc2, then nc3).
//
// A part can print ONE NAME ON SEVERAL TERMINALS (the seeded TXB0104 prints NC twice), and two pins
// sharing an id is a rejected save. Returns "" for an empty name, which the caller treats as "not
// ready to add".
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

// newPackage declares one body the part ships in. It carries no provenance, because a package is the
// label a pin number is relative to, and param.Validate asks only for a unique id.
export function newPackage(id: string, name: string, mpnSuffix = ""): Package {
  return create(PackageSchema, { id: id.trim(), name: name.trim(), mpnSuffix: mpnSuffix.trim() });
}

// pinsForRegion returns the pins transcribed against a region id, the pin counterpart of
// paramsForRegion.
export function pinsForRegion(spec: PartSpec, regionId: string): Pin[] {
  return spec.pins.filter((p) => p.attributes[REGION_ATTR] === regionId);
}

// setPinNumber records a pin's designator within one package, REPLACING any existing entry for that
// package rather than appending. An empty number removes the entry, which is how a mistyped
// designator is cleared. A stale entry could leave two pins claiming one number, which ValidatePins
// rejects.
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

// NewRelationFields is the editor's input for one pin-to-pin constraint. The bound is a min and a
// max ON THE DIFFERENCE (subject minus reference), which is what the contract stores. One shape then
// holds both "VCCA <= VCCB" (max 0) and "never exceeds by more than 0.5 V" (max 0.5), and there is no
// translation step here for a sign error to hide in.
export interface NewRelationFields {
  subjectPinRef: string;
  referencePinRef: string;
  min: number | undefined;
  max: number | undefined;
  unit: string;
  modality: Modality;
  raw: string;
}

// newRelation builds a param.PinRelation from editor fields and the region it was read in, stamping
// the same provenance newPin does. The source text is a pin table's description column, so the
// region is the page the citation needs.
//
// Kind is always TRACKING, the only member the contract admits.
export function newRelation(f: NewRelationFields, region: Region, page: number, docRef: string): PinRelation {
  return create(PinRelationSchema, {
    subjectPinRef: f.subjectPinRef,
    referencePinRef: f.referencePinRef,
    kind: PinRelationKind.TRACKING,
    difference: create(RangeValueSchema, { min: f.min, max: f.max }),
    unit: f.unit.trim(),
    modality: f.modality,
    raw: f.raw.trim(),
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

// relationsForRegion returns the relations transcribed against a region id, like paramsForRegion
// and pinsForRegion.
export function relationsForRegion(spec: PartSpec, regionId: string): PinRelation[] {
  return spec.relations.filter((r) => r.attributes[REGION_ATTR] === regionId);
}

// fmtRelation renders a relation the way the datasheet states it, rather than as the difference
// bound it is stored as, because "max 0" makes a correct "VCCA <= VCCB" look wrong and a sign error
// look plausible. A one-sided bound renders as a comparison and only a two-sided one as a range.
export function fmtRelation(r: PinRelation, nameOf: (pinId: string) => string): string {
  const subject = nameOf(r.subjectPinRef);
  const reference = nameOf(r.referencePinRef);
  const d = r.difference;
  const unit = r.unit ? ` ${r.unit}` : "";
  if (!d || (d.min === undefined && d.max === undefined)) return `${subject} ? ${reference}`;
  if (d.min === undefined && d.max !== undefined) {
    return d.max === 0
      ? `${subject} <= ${reference}`
      : `${subject} <= ${reference} + ${d.max}${unit}`;
  }
  if (d.max === undefined && d.min !== undefined) {
    return d.min === 0
      ? `${subject} >= ${reference}`
      : `${subject} >= ${reference} + ${d.min}${unit}`;
  }
  return `${subject} - ${reference} within ${d.min} .. ${d.max}${unit}`;
}

