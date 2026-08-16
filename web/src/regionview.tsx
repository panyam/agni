import { createEffect, createResource, createSignal, onCleanup, For, Show } from "solid-js";
import { artifactUri } from "./uri.js";
import type { EventBus } from "@panyam/tsappkit";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import { Code, ConnectError } from "@connectrpc/connect";
import type { Document } from "./gen/agni/v1/doc/doc_pb.js";
import type { PartSpec, Parameter, Pin, PinRelation } from "./gen/agni/v1/param/param_pb.js";
import type { ValidationProblem } from "./gen/agni/v1/webapi/datasheet_pb.js";
import { datasheetClient } from "./api.js";
import type { PdfSource, PDFDocumentProxy, RenderedPage } from "./pdfsource.js";
import {
  wheelZoomFactor,
  zoomAboutClamped,
  panBy,
  fitInto,
  clampScale,
  type PanZoom,
} from "./panzoom.js";
import {
  classifyPointerDown,
  selectionAfterPointerDown,
  classifyKey,
  isFormField,
  ZOOM_KEY_FACTOR,
  type HitTarget,
} from "./pagegestures.js";
import {
  regionsForPage,
  defaultType,
  pxRectToBBox,
  coverageOf,
  clampPage,
  moveBBox,
  resizeBBox,
  type Handle,
  type Region,
  type RegionType,
} from "./regions.js";
import {
  emptySpec,
  docId,
  loadUiState,
  saveUiState,
  getAuthor,
  uiToSet,
  setToUi,
  otherUserRegions,
  loadLayers,
  saveLayers,
  type LayerVis,
  exportSpecJson,
  importSpecJson,
  newParameter,
  adoptDocRevision,
  handVerification,
  today,
  paramsForRegion,
  REGION_ATTR,
  newPin,
  newPackage,
  newRelation,
  setPinNumber,
  bindParam,
  unbindParam,
  type NewParamFields,
  type NewPinFields,
  type NewRelationFields,
} from "./bank.js";
import { TranscribePanel } from "./transcribe.js";

// RegionViewState is what the tree pushes to the workbench: which datasheet to open.
export interface RegionViewState {
  mount: string;
  path: string;
}

// RegionView is the presenter-facing handle the boot code wires the tree and the params panel to:
// open a datasheet, or jump to a parameter's page and select its region (click-to-locate).
export interface RegionView {
  load(mount: string, path: string): void;
  locate(page: number, regionId: string): void;
}

// BASE_SCALE is device pixels per PDF point at 100% zoom, so a zoom readout is view.scale /
// BASE_SCALE. MIN/MAX bound the zoom at 25% and 600%.
const BASE_SCALE = 1.3;
const MIN_SCALE = BASE_SCALE * 0.25;
const MAX_SCALE = BASE_SCALE * 6;
// RENDER_SETTLE_MS is how long the zoom has to hold still before pdf.js re-rasterizes. Between the
// gesture and the settle the already-rendered bitmap is stretched by a CSS transform, so zooming
// stays at pointer speed; rasterizing per wheel event instead would blank the page on every notch,
// because each render is async and there is nothing to show while it runs.
const RENDER_SETTLE_MS = 140;
const PLACEHOLDER_SPEC = emptySpec("", "", "");
const r1 = (n: number): number => Math.round(n * 10) / 10;

// Drag is the in-flight pointer interaction over the page: pan the view, rubber-band a new region,
// move the selected user region, or resize one by a corner handle. px0/py0 are overlay pixels at
// grab time; pan tracks client pixels instead, because it moves the overlay rather than moving
// within it.
type Drag =
  | { mode: "pan"; lastX: number; lastY: number }
  | { mode: "draw"; x0: number; y0: number }
  | { mode: "move"; region: Region; startBBox: Region["bbox"]; px0: number; py0: number }
  | { mode: "resize"; region: Region; handle: Handle; startBBox: Region["bbox"]; px0: number; py0: number };

const HANDLES: Handle[] = ["nw", "ne", "sw", "se"];

// LAYER_TOGGLES drives the toolbar visibility checkboxes, in display order. The doc-IR (auto) kinds
// come first, then the two annotation layers.
const LAYER_TOGGLES: { key: keyof LayerVis; label: string; title: string }[] = [
  { key: "table", label: "Tables", title: "doc-IR tables" },
  { key: "figure", label: "Figures", title: "doc-IR figures" },
  { key: "text", label: "Text", title: "doc-IR text blocks (off by default — the extractor emits one per block)" },
  { key: "mine", label: "Mine", title: "my drawn regions" },
  { key: "others", label: "Others", title: "other authors' regions (read-only)" },
];

function downloadText(name: string, text: string): void {
  const url = URL.createObjectURL(new Blob([text], { type: "application/json" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}

function Workbench(props: {
  state: () => RegionViewState | null;
  onParamsChange: (p: Parameter[]) => void;
  bridge: { locate: (p: number, r: string) => void };
  pdf: PdfSource;
}) {
  const [selected, setSelected] = createSignal("");
  const [rev, setRev] = createSignal(0);
  const [note, setNote] = createSignal("");
  const [status, setStatus] = createSignal("");
  const [pageNum, setPageNum] = createSignal(1);
  // view is the live pan/zoom of the page within the viewport (see panzoom.ts): scale is device
  // pixels per PDF point, tx/ty place the page's top-left corner. renderScale is the scale pdf.js
  // last rasterized at, which trails the live scale by RENDER_SETTLE_MS.
  const [view, setView] = createSignal<PanZoom>({ tx: 0, ty: 0, scale: BASE_SCALE });
  const [renderScale, setRenderScale] = createSignal(BASE_SCALE);
  // drawMode is the sticky alternative to holding Shift: on, a drag draws regions instead of
  // panning. Transcribing a datasheet is mostly drawing, and holding a modifier for every box in a
  // long session is the kind of friction that makes people stop using the tool.
  const [drawMode, setDrawMode] = createSignal(false);
  const [panning, setPanning] = createSignal(false);
  const [pdfDoc, setPdfDoc] = createSignal<PDFDocumentProxy | null>(null);
  const [numPages, setNumPages] = createSignal(1);
  const [draftPx, setDraftPx] = createSignal<{ x: number; y: number; w: number; h: number } | null>(null);
  // layers gates which overlay layers render (visibility only; coverage stays over all regions).
  const [layers, setLayers] = createSignal<LayerVis>(loadLayers());
  const toggleLayer = (k: keyof LayerVis): void => {
    const next = { ...layers(), [k]: !layers()[k] };
    setLayers(next);
    saveLayers(next);
  };

  let spec: PartSpec | null = null;
  let version = "";
  let userRegions: Region[] = [];
  let types: Record<string, RegionType> = {};
  // otherRegions are OTHER authors' user-drawn boxes, shown read-only (compose); the author id
  // namespaces this browser's own overlay so co-editors do not clobber each other (WS13-011).
  let otherRegions: Region[] = [];
  const author = getAuthor();
  let saveTimer: ReturnType<typeof setTimeout> | undefined;
  let drag: Drag | null = null;
  let overlayEl: HTMLDivElement | undefined;
  let viewportEl: HTMLDivElement | undefined;
  let renderTimer: ReturnType<typeof setTimeout> | undefined;
  // needsFit makes the first rendered page of a NEWLY opened datasheet fit the viewport. A page
  // change does not refit: staying at the same pan/zoom is what lets you follow one table's columns
  // across a page break.
  let needsFit = true;

  // reload bumps to re-run the outer load after an extraction (so the new doc-IR + regions appear);
  // extracting drives the "Extract (first pass)" button's spinner.
  const [reload, setReload] = createSignal(0);
  const [extracting, setExtracting] = createSignal(false);

  // Outer load: the PDF document, its doc-IR, the saved PartSpec (shared, server), and the per-user
  // UI state (localStorage). The pages themselves render on demand (pageImg, below). `reload` is part
  // of the source so bumping it (post-extract) re-fetches.
  const [data] = createResource(
    () => {
      const s = props.state();
      return s ? { mount: s.mount, path: s.path, k: reload() } : null;
    },
    async (s: { mount: string; path: string; k: number }) => {
      const client = datasheetClient();
      const [doc, docResp, part, ann] = await Promise.all([
        props.pdf.loadPdf(props.pdf.rawDatasheetUrl(s.mount, s.path)),
        client.getDocument({ uri: artifactUri(s.mount, s.path) }),
        client.getPartSpec({ uri: artifactUri(s.mount, s.path) }),
        client.getAnnotations({ uri: artifactUri(s.mount, s.path) }),
      ]);
      const docIR = docResp.document as Document | undefined;
      const docHash = docIR?.contentHash ?? "";
      spec = part.found && part.spec ? part.spec : emptySpec(s.path, docIR?.title || s.path, docHash);
      // A spec saved before the workbench recorded a revision has none, and a verification written
      // onto it would be uninvalidatable. Backfill on load so the first transcription in an old
      // spec is anchored like any other; adoptDocRevision leaves a recorded hash alone.
      adoptDocRevision(spec, docHash);
      version = part.version;
      // The server overlay is the source of truth for MY set; the localStorage buffer is the
      // fallback (offline / never-saved). Others' drawn boxes render read-only. Reseed the buffer
      // from the loaded state so an immediate local write starts consistent with the server.
      const mine = ann.sets.find((x) => x.author === author);
      const ui = mine ? setToUi(mine) : loadUiState(s.mount, s.path);
      userRegions = ui.userRegions;
      types = ui.types;
      otherRegions = otherUserRegions(ann.sets, author);
      saveUiState(s.mount, s.path, ui);
      setPdfDoc(doc);
      setNumPages(doc.numPages);
      setPageNum(1);
      setShown(null);
      setRenderScale(BASE_SCALE);
      setView({ tx: 0, ty: 0, scale: BASE_SCALE });
      needsFit = true;
      setSelected("");
      setNote("");
      return { extracted: docResp.extracted, doc: docIR, extractAvailable: docResp.extractAvailable };
    },
  );

  // doExtract runs the server-side doc-IR producer, then re-loads so the auto-detected regions show.
  const doExtract = async (): Promise<void> => {
    const s = props.state();
    if (!s) return;
    setExtracting(true);
    setNote("");
    try {
      await datasheetClient().extractDocIR({ uri: artifactUri(s.mount, s.path) });
      setReload((r) => r + 1);
    } catch (e) {
      console.error("extract failed", e);
      setNote("Extraction failed (see console).");
    } finally {
      setExtracting(false);
    }
  };

  // On-demand single-page render, re-run on page change or once a zoom settles. Each render makes
  // its own canvas, so overlapping renders never contend for one canvas.
  const [pageImg] = createResource(
    () => {
      const d = pdfDoc();
      return d ? { d, pn: pageNum(), s: renderScale() } : null;
    },
    async (src: { d: PDFDocumentProxy; pn: number; s: number }) => props.pdf.renderPage(src.d, src.pn, src.s),
  );
  // The resource goes undefined while it refetches, which would blank the page mid-zoom. `shown`
  // holds the last completed render so the previous bitmap stays up, stretched to the live scale,
  // until the new one lands. A render for a DIFFERENT page is not a stand-in for this one, so
  // `visible` withholds it and the page-change fallback shows instead.
  const [shown, setShown] = createSignal<RenderedPage | null>(null);
  createEffect(() => {
    const p = pageImg();
    if (p) setShown(p);
  });
  const visible = (): RenderedPage | null => {
    const s = shown();
    return s && s.pageNumber === pageNum() ? s : null;
  };
  // cssScale is the transform that carries the on-screen size from whatever scale the visible
  // bitmap was rasterized at to the live scale. It is keyed off the SHOWN render rather than the
  // renderScale signal: the signal moves when the re-render is requested, and using it would resize
  // the page the moment a render started rather than when it finished.
  const shownScale = (): number => visible()?.scale ?? renderScale();
  const cssScale = (): number => view().scale / shownScale();

  // applyView commits a pan/zoom and, when the zoom moved, schedules the crisp re-rasterize.
  const applyView = (v: PanZoom): void => {
    const zoomed = v.scale !== view().scale;
    setView(v);
    if (!zoomed) return;
    if (renderTimer) clearTimeout(renderTimer);
    renderTimer = setTimeout(() => setRenderScale(view().scale), RENDER_SETTLE_MS);
  };
  onCleanup(() => {
    if (renderTimer) clearTimeout(renderTimer);
  });

  const fitPage = (): void => {
    const p = visible();
    const host = viewportEl;
    if (!p || !host) return;
    applyView(
      fitInto(p.widthPts, p.heightPts, host.clientWidth, host.clientHeight, {
        minScale: MIN_SCALE,
        maxScale: MAX_SCALE,
      }),
    );
  };
  createEffect(() => {
    if (visible() && needsFit) {
      needsFit = false;
      fitPage();
    }
  });

  createEffect(() => {
    data();
    setRev((r) => r + 1);
    if (spec) props.onParamsChange([...spec.parameters]);
  });

  // Expose click-to-locate: jump to a parameter's page and select its region.
  props.bridge.locate = (page: number, regionId: string): void => {
    setPageNum(clampPage(page, numPages()));
    setSelected(regionId);
  };

  // problems is the server's verdict on the last save. It starts empty, which reads as "nothing
  // known yet" rather than "nothing wrong" — correct, because no save has happened.
  const [problems, setProblems] = createSignal<ValidationProblem[]>([]);

  const serverSave = async (): Promise<void> => {
    const s = props.state();
    if (!s || !spec) return;
    const client = datasheetClient();
    try {
      const resp = await client.savePartSpec({ uri: artifactUri(s.mount, s.path), spec, baseVersion: version });
      version = resp.version;
      // The judgement rides the save. The client does NOT recompute these: one implementation lives
      // in Go (param.Problems) and this renders what it said, so the two cannot drift.
      setProblems(resp.problems);
      setRev((v) => v + 1);
    } catch (e) {
      if (e instanceof ConnectError && e.code === Code.Aborted) {
        const resp = await client.getPartSpec({ uri: artifactUri(s.mount, s.path) });
        spec = resp.found && resp.spec ? resp.spec : spec;
        version = resp.version;
        setSelected("");
        setNote("Reloaded: another save landed for this datasheet.");
        setRev((v) => v + 1);
        if (spec) props.onParamsChange([...spec.parameters]);
      } else {
        console.error("save failed", e);
        setNote("Save failed (see console).");
      }
    }
  };
  // annSave persists THIS author's overlay (drawn boxes + type tags) to the server, best-effort and
  // debounced alongside the PartSpec save. No optimistic concurrency: each author owns their own
  // file, so a save never conflicts (WS13-011). localStorage stays the immediate live buffer.
  const annSave = async (): Promise<void> => {
    const s = props.state();
    if (!s) return;
    try {
      await datasheetClient().saveAnnotations({
        uri: artifactUri(s.mount, s.path),
        set: uiToSet(docId(s.path), author, { userRegions, types }),
      });
    } catch (e) {
      console.error("annotation save failed", e);
    }
  };
  const scheduleSave = (): void => {
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      void serverSave();
      void annSave();
    }, 700);
  };
  const commit = (): void => {
    const s = props.state();
    if (s) saveUiState(s.mount, s.path, { userRegions, types });
    scheduleSave();
    if (spec) props.onParamsChange([...spec.parameters]);
    setRev((r) => r + 1);
  };

  // Regions on the current page: doc-IR regions (stamped with the page) plus user regions drawn on
  // it. The page stamp flows into a transcribed parameter's provenance.
  const pageRegions = (): Region[] => {
    rev();
    const pn = pageNum();
    const L = layers();
    const kindOn = (k: Region["kind"]): boolean =>
      k === "table" ? L.table : k === "figure" ? L.figure : k === "text" ? L.text : true;
    const docRegions = regionsForPage(data()?.doc, pn).filter((r) => kindOn(r.kind)).map((r) => ({ ...r, page: pn }));
    const mine = L.mine ? userRegions.filter((u) => u.page === pn) : [];
    return [...docRegions, ...mine];
  };
  // pageOtherRegions are other authors' user-drawn boxes on this page, rendered read-only (compose).
  // Kept separate from pageRegions so they are never selectable, movable, or hit-tested.
  const pageOtherRegions = (): Region[] => {
    rev();
    if (!layers().others) return [];
    const pn = pageNum();
    return otherRegions.filter((u) => u.page === pn);
  };
  const isDone = (id: string): boolean => {
    rev();
    return !!spec && paramsForRegion(spec, id).length > 0;
  };
  const selectedRegion = (): Region | null => pageRegions().find((r) => r.id === selected()) ?? null;
  const typeOf = (r: Region): RegionType => {
    rev();
    return (types[r.id] as RegionType) ?? defaultType(r.kind);
  };
  // coverage is over ALL regions (every page), so it reflects the whole datasheet, not just this page.
  const allRegions = (): Region[] => {
    rev();
    const d = data();
    if (!d) return [];
    const docRegions = (d.doc?.pages ?? []).flatMap((p) => regionsForPage(d.doc, p.number).map((r) => ({ ...r, page: p.number })));
    return [...docRegions, ...userRegions];
  };
  const coverage = () => coverageOf(allRegions(), isDone);
  const paramCount = (): number => {
    rev();
    return spec?.parameters.length ?? 0;
  };

  const addUserRegion = (bbox: Region["bbox"], pn: number): void => {
    const id = `u${userRegions.length + 1}`;
    userRegions.push({ id, kind: "user", label: "user region", bbox, page: pn });
    commit();
    setSelected(id);
  };
  const deleteRegion = (): void => {
    const r = selectedRegion();
    if (!r || r.kind !== "user" || !spec) return;
    userRegions = userRegions.filter((u) => u.id !== r.id);
    spec.parameters = spec.parameters.filter((p) => p.attributes[REGION_ATTR] !== r.id); // its rows go with it
    delete types[r.id];
    setSelected("");
    commit();
  };

  const goto = (n: number): void => {
    setPageNum(clampPage(n, numPages()));
  };

  // zoomStep is the keyboard's zoom, anchored at the middle of the viewport since there is no
  // cursor to aim at.
  const zoomStep = (factor: number): void => {
    const host = viewportEl;
    if (!host) return;
    applyView(zoomAboutClamped(view(), host.clientWidth / 2, host.clientHeight / 2, factor, MIN_SCALE, MAX_SCALE));
  };

  // Keyboard: page navigation, zoom, region delete, and the Draw-mode toggle. The bindings
  // themselves live in pagegestures.classifyKey; this only carries them out.
  const onKey = (e: KeyboardEvent): void => {
    const a = classifyKey(e, isFormField(document.activeElement));
    if (!a) return;
    switch (a.kind) {
      case "page":
        e.preventDefault();
        goto(a.to === "first" ? 1 : a.to === "last" ? numPages() : a.to === "prev" ? pageNum() - 1 : pageNum() + 1);
        return;
      case "zoom":
        e.preventDefault();
        if (a.to === "fit") fitPage();
        else zoomStep(a.to === "in" ? ZOOM_KEY_FACTOR : 1 / ZOOM_KEY_FACTOR);
        return;
      case "deleteRegion":
        // Only a user region is deletable, and Backspace has to stay a plain Backspace otherwise.
        if (selectedRegion()?.kind === "user") {
          e.preventDefault();
          deleteRegion();
        }
        return;
      case "toggleDraw":
        e.preventDefault();
        setDrawMode((d) => !d);
        return;
      case "exitDraw":
        if (drawMode()) {
          e.preventDefault();
          setDrawMode(false);
        }
        return;
    }
  };
  document.addEventListener("keydown", onKey);
  onCleanup(() => document.removeEventListener("keydown", onKey));

  // scale is the RENDER scale of the visible bitmap: overlay-local pixels per PDF point. Region
  // boxes are laid out in that space and the CSS transform carries them to the live zoom, so
  // nothing below has to know what the zoom currently is.
  const scale = (): number => visible()?.scale ?? renderScale();
  // relPx is the pointer in overlay-local pixels. getBoundingClientRect reports the TRANSFORMED
  // box, so the client offset is in screen pixels and has to be divided back out by the transform
  // to land in the space the region boxes and the draft rectangle live in.
  const relPx = (e: PointerEvent): { x: number; y: number } => {
    const rect = overlayEl!.getBoundingClientRect();
    const k = cssScale() || 1;
    return { x: (e.clientX - rect.left) / k, y: (e.clientY - rect.top) / k };
  };
  const bboxStatus = (b: Region["bbox"]): string => `x ${r1(b.x)}, y ${r1(b.y)} · ${r1(b.width)}×${r1(b.height)} pt`;

  // hitUnder reads what the pointer landed on out of the DOM, so classifyPointerDown can stay pure.
  const hitUnder = (el: HTMLElement): HitTarget => {
    const regionEl = el.closest("[data-region]") as HTMLElement | null;
    const id = regionEl?.dataset.region ?? null;
    return {
      deleteButton: !!el.closest("[data-del]"),
      handle: ((el.closest("[data-handle]") as HTMLElement | null)?.dataset.handle ?? null) as Handle | null,
      regionId: id,
      regionKind: id ? (pageRegions().find((r) => r.id === id)?.kind ?? null) : null,
    };
  };

  const onDown = (e: PointerEvent): void => {
    if (!visible()) return;
    const hit = hitUnder(e.target as HTMLElement);
    // Shift and the sticky Draw mode are two ways to say the same thing, and only one of them
    // reaches the router.
    const intent = classifyPointerDown(hit, { selectedId: selected(), drawIntent: e.shiftKey || drawMode() });
    const sel = selectionAfterPointerDown(hit, intent);
    if (sel !== null) setSelected(sel);
    if (intent.kind === "delete") {
      deleteRegion();
      return;
    }
    if (intent.kind === "pan") {
      drag = { mode: "pan", lastX: e.clientX, lastY: e.clientY };
      setPanning(true);
    } else {
      const { x, y } = relPx(e);
      if (intent.kind === "draw") {
        drag = { mode: "draw", x0: x, y0: y };
        setDraftPx({ x, y, w: 0, h: 0 });
      } else {
        // move and resize both act on the selected region, which is what classifyPointerDown
        // matched the hit against.
        const r = selectedRegion();
        if (!r) return;
        drag =
          intent.kind === "resize"
            ? { mode: "resize", region: r, handle: intent.handle, startBBox: { ...r.bbox }, px0: x, py0: y }
            : { mode: "move", region: r, startBBox: { ...r.bbox }, px0: x, py0: y };
      }
    }
    viewportEl!.setPointerCapture(e.pointerId);
  };
  const onMove = (e: PointerEvent): void => {
    if (!visible()) return;
    if (drag?.mode === "pan") {
      // Pan works in client pixels: it moves the page under a still pointer, so an overlay-local
      // delta would be measured against a box that is itself moving.
      applyView(panBy(view(), e.clientX - drag.lastX, e.clientY - drag.lastY));
      drag.lastX = e.clientX;
      drag.lastY = e.clientY;
      return;
    }
    const s = scale();
    const { x, y } = relPx(e);
    if (!drag) {
      setStatus(`x ${r1(x / s)}, y ${r1(y / s)} pt`);
      return;
    }
    if (drag.mode === "draw") {
      const dx = Math.min(drag.x0, x);
      const dy = Math.min(drag.y0, y);
      const w = Math.abs(x - drag.x0);
      const h = Math.abs(y - drag.y0);
      setDraftPx({ x: dx, y: dy, w, h });
      setStatus(`draw (${r1(dx / s)}, ${r1(dy / s)}) · ${r1(w / s)}×${r1(h / s)} pt`);
    } else if (drag.mode === "move") {
      drag.region.bbox = moveBBox(drag.startBBox, (x - drag.px0) / s, (y - drag.py0) / s);
      setRev((v) => v + 1);
      setStatus(bboxStatus(drag.region.bbox));
    } else {
      drag.region.bbox = resizeBBox(drag.startBBox, drag.handle, (x - drag.px0) / s, (y - drag.py0) / s);
      setRev((v) => v + 1);
      setStatus(bboxStatus(drag.region.bbox));
    }
  };
  const onUp = (): void => {
    const s = scale();
    if (drag?.mode === "draw") {
      const d = draftPx();
      if (d && d.w > 6 && d.h > 6) addUserRegion(pxRectToBBox(d.x, d.y, d.x + d.w, d.y + d.h, s), pageNum());
    } else if (drag && (drag.mode === "move" || drag.mode === "resize")) {
      commit();
    }
    drag = null;
    setPanning(false);
    setDraftPx(null);
  };

  // The wheel listener is attached by hand rather than through onWheel, because preventDefault needs
  // a non-passive listener. It also swallows the macOS pinch gesture (which arrives as ctrl+wheel),
  // which would otherwise zoom the whole browser page.
  const attachViewport = (el: HTMLDivElement): void => {
    viewportEl = el;
    el.addEventListener(
      "wheel",
      (e: WheelEvent) => {
        e.preventDefault();
        const rect = el.getBoundingClientRect();
        applyView(
          zoomAboutClamped(
            view(),
            e.clientX - rect.left,
            e.clientY - rect.top,
            wheelZoomFactor(e.deltaY),
            MIN_SCALE,
            MAX_SCALE,
          ),
        );
      },
      { passive: false },
    );
  };

  const handlers = {
    spec: () => {
      rev();
      return spec ?? PLACEHOLDER_SPEC;
    },
    problems: (): ValidationProblem[] => {
      rev();
      return problems();
    },
    region: selectedRegion,
    regionType: (): RegionType => {
      const r = selectedRegion();
      return r ? typeOf(r) : "other";
    },
    deletableRegion: (): boolean => selectedRegion()?.kind === "user",
    onDeleteRegion: deleteRegion,
    setType: (t: RegionType): void => {
      const r = selectedRegion();
      if (r) {
        types[r.id] = t;
        commit();
      }
    },
    setMeta: (patch: Partial<{ mpn: string; manufacturer: string; deviceClass: string; docTitle: string }>): void => {
      if (!spec) return;
      if (patch.mpn !== undefined) spec.mpn = patch.mpn;
      if (patch.manufacturer !== undefined) spec.manufacturer = patch.manufacturer;
      if (patch.deviceClass !== undefined) spec.deviceClass = patch.deviceClass;
      // The document's own identity, which the contract wants stated as the vendor prints it
      // (number + revision) rather than as a part name. Editing it does NOT re-date existing
      // verifications: each snapshotted the title as it stood when it was performed, which is the
      // whole reason the snapshot lives on the verification and not here.
      if (patch.docTitle !== undefined && spec.docs[0]) spec.docs[0].title = patch.docTitle;
      commit();
    },
    addParam: (f: NewParamFields): void => {
      const r = selectedRegion();
      if (r && spec) {
        // The verification is built from the document AS IT STANDS NOW, so the revision it pins to
        // is the one the author is looking at rather than whatever the spec is re-read against later.
        const v = handVerification(spec.docs[0], author, today());
        spec.parameters.push(newParameter(f, r, r.page ?? pageNum(), spec.docs[0]?.id ?? "", v));
        commit();
      }
    },
    deleteParam: (p: Parameter): void => {
      if (!spec) return;
      spec.parameters = spec.parameters.filter((x) => x !== p);
      commit();
    },
    addPin: (f: NewPinFields): void => {
      const r = selectedRegion();
      if (r && spec) {
        spec.pins.push(newPin(f, r, r.page ?? pageNum(), spec.docs[0]?.id ?? ""));
        commit();
      }
    },
    deletePin: (p: Pin): void => {
      if (!spec) return;
      // Unbind before removing, or every parameter that named this pin is left dangling — which
      // ValidatePins rejects on the next save, turning a delete into a stuck document.
      for (const param of spec.parameters) unbindParam(param, p.id);
      // A relation naming this pin is dangling for the same reason, and unlike a parameter it
      // cannot be repaired by dropping one ref: a relation with one end gone says nothing, so the
      // whole relation goes.
      spec.relations = spec.relations.filter(
        (r) => r.subjectPinRef !== p.id && r.referencePinRef !== p.id,
      );
      spec.pins = spec.pins.filter((x) => x !== p);
      commit();
    },
    setPinNumber: (pin: Pin, packageRef: string, number: string): void => {
      setPinNumber(pin, packageRef, number);
      commit();
    },
    addPackage: (id: string, name: string, suffix: string): void => {
      if (!spec) return;
      spec.packages.push(newPackage(id, name, suffix));
      commit();
    },
    deletePackage: (id: string): void => {
      if (!spec) return;
      // Drop the package's numbers with it, for deletePin's reason: a PinNumber pointing at a
      // package that no longer exists fails the save.
      for (const pin of spec.pins) pin.numbers = pin.numbers.filter((n) => n.packageRef !== id);
      spec.packages = spec.packages.filter((p) => p.id !== id);
      commit();
    },
    toggleBinding: (p: Parameter, pinId: string): void => {
      if (p.pinRefs.includes(pinId)) unbindParam(p, pinId);
      else bindParam(p, pinId);
      commit();
    },
    addRelation: (f: NewRelationFields): void => {
      const r = selectedRegion();
      if (r && spec) {
        spec.relations.push(newRelation(f, r, r.page ?? pageNum(), spec.docs[0]?.id ?? ""));
        commit();
      }
    },
    deleteRelation: (rel: PinRelation): void => {
      if (!spec) return;
      spec.relations = spec.relations.filter((x) => x !== rel);
      commit();
    },
  };

  const doExport = (): void => {
    if (!spec) return;
    const stem = (spec.mpn || props.state()?.path.split("/").pop() || "partspec").replace(/\.pdf$/i, "");
    downloadText(`${stem}.partspec.json`, exportSpecJson(spec));
  };
  const doImport = async (file: File): Promise<void> => {
    try {
      spec = importSpecJson(await file.text());
      commit();
    } catch (e) {
      console.error("import failed", e);
      setNote("Import failed (not valid PartSpec JSON).");
    }
  };

  return (
    <div class="ds-workbench">
      <Show when={props.state()} fallback={<div class="ds-empty">Select a datasheet to scan.</div>}>
        <Show when={!data.loading} fallback={<div class="ds-empty">Loading datasheet…</div>}>
          <Show when={data()} fallback={<div class="ds-error">Failed to load datasheet.</div>}>
            {(d) => (
              <>
                <div class="ds-toolbar">
                  <span class="ds-doc-name" title={props.state()?.path}>{props.state()?.path.split("/").pop()}</span>
                  <span class="ds-pager">
                    <button onClick={() => goto(pageNum() - 1)} disabled={pageNum() <= 1}>‹</button>
                    <input class="ds-pageinput" type="number" min={1} max={numPages()} value={pageNum()} onChange={(e) => goto(Number(e.currentTarget.value))} />
                    <span class="ds-pagetotal">/ {numPages()}</span>
                    <button onClick={() => goto(pageNum() + 1)} disabled={pageNum() >= numPages()}>›</button>
                  </span>
                  <button
                    class="ds-drawtoggle"
                    classList={{ on: drawMode() }}
                    onClick={() => setDrawMode((v) => !v)}
                    title="Draw mode (R). On: drag draws a region. Off: drag pans, shift+drag draws."
                  >
                    ▭ Draw
                  </button>
                  <Show
                    when={d().extracted}
                    fallback={
                      <span class="ds-notextracted">
                        Not yet extracted
                        <Show when={d().extractAvailable}>
                          <button class="ds-extract-btn" disabled={extracting()} onClick={doExtract}>
                            {extracting() ? "Extracting…" : "Extract (first pass)"}
                          </button>
                        </Show>
                      </span>
                    }
                  >
                    <span class="ds-extracted">Extracted</span>
                  </Show>
                  <span class="ds-cov">{paramCount()} params · {coverage().done}/{coverage().total} regions</span>
                  <Show when={note()}><span class="ds-note">{note()}</span></Show>
                  <span class="ds-tools">
                    <button onClick={doExport}>Export</button>
                    <label class="ds-import">Import<input type="file" accept="application/json" onChange={(e) => { const f = e.currentTarget.files?.[0]; if (f) void doImport(f); e.currentTarget.value = ""; }} /></label>
                  </span>
                </div>
                <div class="ds-subtoolbar">
                  <span class="ds-layers-label">Layers</span>
                  <span class="ds-layers" title="show/hide overlay layers (visibility only; coverage counts every region)">
                    <For each={LAYER_TOGGLES}>
                      {(t) => (
                        <label title={t.title}>
                          <input type="checkbox" checked={layers()[t.key]} onChange={() => toggleLayer(t.key)} />{t.label}
                        </label>
                      )}
                    </For>
                  </span>
                </div>
                <div class="ds-body">
                  <div class="ds-viewport-wrap">
                    <div
                      ref={attachViewport}
                      class="ds-viewport"
                      classList={{ drawing: drawMode(), panning: panning() }}
                      onPointerDown={onDown}
                      onPointerMove={onMove}
                      onPointerUp={onUp}
                      onPointerLeave={() => { if (!drag) setStatus(""); }}
                    >
                      <Show when={visible()} fallback={<div class="ds-empty">Rendering page…</div>}>
                        {(img) => (
                          <div
                            class="ds-page-canvas"
                            style={{
                              width: `${img().canvas.width}px`,
                              height: `${img().canvas.height}px`,
                              transform: `translate(${view().tx}px, ${view().ty}px) scale(${cssScale()})`,
                            }}
                          >
                            {img().canvas}
                            <div ref={overlayEl} class="ds-overlay">
                              <For each={pageRegions()}>
                                {(r) => (
                                  <div
                                    class={`ds-region kind-${r.kind}${selected() === r.id ? " selected" : ""}${isDone(r.id) ? " done" : ""}`}
                                    data-region={r.id}
                                    style={{ left: `${r.bbox.x * img().scale}px`, top: `${r.bbox.y * img().scale}px`, width: `${r.bbox.width * img().scale}px`, height: `${r.bbox.height * img().scale}px` }}
                                    title={`${r.kind}: ${r.label}`}
                                  >
                                    <span class="ds-region-tag">{r.id}{isDone(r.id) ? " ✓" : ""}</span>
                                    <Show when={selected() === r.id && r.kind === "user"}>
                                      <span class="ds-del-btn" data-del="1" title="delete region (or press Delete)">×</span>
                                      <For each={HANDLES}>{(h) => <span class={`ds-handle ds-handle-${h}`} data-handle={h} />}</For>
                                    </Show>
                                  </div>
                                )}
                              </For>
                              <For each={pageOtherRegions()}>
                                {(r) => (
                                  <div
                                    class="ds-region ds-other"
                                    style={{ left: `${r.bbox.x * img().scale}px`, top: `${r.bbox.y * img().scale}px`, width: `${r.bbox.width * img().scale}px`, height: `${r.bbox.height * img().scale}px` }}
                                    title={r.label}
                                  >
                                    <span class="ds-region-tag">{r.label}</span>
                                  </div>
                                )}
                              </For>
                              <Show when={draftPx()}>
                                {(d2) => <div class="ds-draft" style={{ left: `${d2().x}px`, top: `${d2().y}px`, width: `${d2().w}px`, height: `${d2().h}px` }} />}
                              </Show>
                            </div>
                          </div>
                        )}
                      </Show>
                    </div>
                    <div class="ds-statusbar">
                      <span>page {pageNum()} / {numPages()}</span>
                      <button class="ds-zoomval" onClick={fitPage} title="fit page (0)">
                        {Math.round((view().scale / BASE_SCALE) * 100)}%
                      </button>
                      <span class="ds-status-coords">{status()}</span>
                      <span class="ds-status-hint">
                        {drawMode() ? "drag draws · wheel zooms" : "drag pans · wheel zooms · shift+drag draws"}
                        {" · PgUp/PgDn, Home/End (or shift+arrows) page"}
                      </span>
                    </div>
                  </div>
                  <TranscribePanel {...handlers} />
                </div>
              </>
            )}
          </Show>
        </Show>
      </Show>
    </div>
  );
}

// workbenchIsland mounts the datasheet workbench (page viewer + marquee edit + transcribe panel).
// onParamsChange pushes the current parameter list to the params panel; the returned view lets the
// tree open a datasheet and the params panel locate one. Framework reactivity stays in this leaf (C11).
//
// pdf is passed in rather than imported, and is not optional. A default would put an import of
// pdfrender.js in this file, and importing that module runs pdf.js's canvas setup, which needs a
// DOMMatrix jsdom does not have — so the default would make this component unrenderable by a test
// even when the test supplies its own source. It is the only injected dependency: the datasheet
// client is reached through api.js, which a test mocks at the module boundary like every other page.
export function workbenchIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  onParamsChange: (p: Parameter[]) => void,
  pdf: PdfSource,
): { island: SolidIsland; view: RegionView } {
  const [state, setState] = signalView<RegionViewState | null>(null);
  const bridge = { locate: (_p: number, _r: string) => {} };
  const island = new SolidIsland("ds-view", el, () => <Workbench state={state} onParamsChange={onParamsChange} bridge={bridge} pdf={pdf} />, eventBus);
  return {
    island,
    view: { load: (mount, path) => setState({ mount, path }), locate: (p, r) => bridge.locate(p, r) },
  };
}
