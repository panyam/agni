// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { workbenchIsland } from "./regionview.jsx";
import type { PdfSource, PDFDocumentProxy, RenderedPage } from "./pdfsource.js";
import { fitInto } from "./panzoom.js";

// The datasheet workbench's first component test. What kept it untested was one dependency: pdf.js
// rasterizes onto a 2d canvas context, which jsdom does not have, so nothing could render this
// component at all. With the PdfSource injected, a stub page is enough, and the wiring that was
// exercised only by hand comes under test: the non-passive wheel listener, the live-transform /
// settled-rasterize split, and fit-on-first-page.
//
// The rasterizing itself is NOT under test here and cannot be. This asserts what the workbench does
// around a renderer, never what the renderer draws.

const client = vi.hoisted(() => ({
  getDocument: vi.fn(async () => ({ extracted: false, extractAvailable: false, document: undefined })),
  getPartSpec: vi.fn(async () => ({ found: false, version: "v1" })),
  getAnnotations: vi.fn(async () => ({ sets: [] })),
  savePartSpec: vi.fn(async () => ({ version: "v2", problems: [] })),
  extractDocIR: vi.fn(async () => ({})),
}));
vi.mock("./api.js", () => ({ datasheetClient: () => client }));

// PAGE_PTS is the stub page's size in PDF points, roughly US Letter, so the fit assertion below is
// a real ratio rather than a number that happens to come out even.
const PAGE_W = 612;
const PAGE_H = 792;
const VIEWPORT_W = 900;
const VIEWPORT_H = 600;

const renders: { page: number; scale: number }[] = [];

function stubPdf(): PdfSource {
  return {
    rawDatasheetUrl: (mount, path) => `/datasheets/raw/${mount}/${path}`,
    loadPdf: async () => ({ numPages: 3 }) as unknown as PDFDocumentProxy,
    renderPage: async (_doc, pageNumber, scale) => {
      renders.push({ page: pageNumber, scale });
      // A canvas element jsdom is happy to create and this component only ever inserts.
      const canvas = document.createElement("canvas");
      canvas.width = Math.ceil(PAGE_W * scale);
      canvas.height = Math.ceil(PAGE_H * scale);
      return { pageNumber, canvas, widthPts: PAGE_W, heightPts: PAGE_H, scale } satisfies RenderedPage;
    },
  };
}

// The viewport sizes itself from layout, which jsdom does not do, so the box is stated here. Without
// it every fit computes against a zero-sized host and the assertion would be vacuous.
function stubLayout(): void {
  Object.defineProperty(HTMLElement.prototype, "clientWidth", { configurable: true, value: VIEWPORT_W });
  Object.defineProperty(HTMLElement.prototype, "clientHeight", { configurable: true, value: VIEWPORT_H });
  Element.prototype.getBoundingClientRect = () =>
    ({ left: 0, top: 0, right: VIEWPORT_W, bottom: VIEWPORT_H, width: VIEWPORT_W, height: VIEWPORT_H, x: 0, y: 0, toJSON: () => ({}) }) as DOMRect;
}

function openWorkbench() {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const { island, view } = workbenchIsland(el, null, () => {}, stubPdf());
  island.activate();
  view.load("ds", "vendor/txb0104.pdf");
  return { el, view };
}

const pageEl = (el: HTMLElement): HTMLElement | null => el.querySelector(".ds-page-canvas");
// settled waits for the crisp raster to catch up with the live view: the stretch factor returns to
// 1 when the bitmap on screen was rasterized at the scale the page is currently shown at. Opening a
// datasheet is itself a zoom (the fit), so a test that skipped this would be racing that settle.
const settled = async (el: HTMLElement): Promise<void> => {
  await vi.waitFor(() => expect(cssScaleOf(el)).toBeCloseTo(1, 3), { timeout: 2000 });
};
const transformOf = (el: HTMLElement): string => pageEl(el)?.style.transform ?? "";
// scale(N) out of the page's transform: how much the already-rasterized bitmap is being stretched.
const cssScaleOf = (el: HTMLElement): number => Number(/scale\(([\d.]+)\)/.exec(transformOf(el))?.[1] ?? NaN);

beforeEach(() => {
  document.body.replaceChildren();
  renders.length = 0;
  stubLayout();
});

describe("workbench viewport", () => {
  it("opens a datasheet and rasterizes its first page", async () => {
    const { el } = openWorkbench();
    await vi.waitFor(() => expect(pageEl(el)).toBeTruthy());

    expect(renders[0].page).toBe(1);
    expect(client.getDocument).toHaveBeenCalled();
  });

  // A newly opened datasheet fits the viewport, so the first thing an author sees is the whole page
  // rather than its top-left corner at whatever scale the last one used.
  it("fits the first page to the viewport", async () => {
    const { el } = openWorkbench();
    await settled(el);

    // Height is the binding dimension for a portrait page in a landscape viewport, less the 4%
    // margin fitInto leaves. The arithmetic is panzoom's and covered in panzoom.test.ts; what this
    // asserts is the WIRING — that opening a datasheet fits it to the viewport's real size — so it
    // calls the same function rather than restating the formula. The second line is what keeps that
    // honest: the fit is not BASE_SCALE, so this cannot pass with no fit happening at all.
    const fit = fitInto(PAGE_W, PAGE_H, VIEWPORT_W, VIEWPORT_H).scale;
    expect(renders[renders.length - 1].scale).toBeCloseTo(fit, 3);
    expect(renders[0].scale).not.toBeCloseTo(fit, 2);
  });

  // The split the ledger named: a wheel notch moves the page NOW, by stretching the bitmap already
  // on screen, and only asks pdf.js for a crisp raster once the gesture holds still. Rasterizing per
  // notch would blank the page on every one, because each render is async with nothing to show while
  // it runs.
  it("zooms on the live transform first and re-rasterizes only after the zoom settles", async () => {
    const { el } = openWorkbench();
    // Let the open's own fit settle first, or its re-raster is indistinguishable from the wheel's.
    await settled(el);
    const rendersBefore = renders.length;
    const scaleBefore = renders[rendersBefore - 1].scale;

    const viewport = el.querySelector(".ds-viewport") as HTMLElement;
    viewport.dispatchEvent(new WheelEvent("wheel", { deltaY: -120, clientX: 450, clientY: 300, bubbles: true, cancelable: true }));

    // Immediately: the bitmap on screen is being stretched, and no new raster was asked for.
    expect(cssScaleOf(el)).toBeGreaterThan(1);
    expect(renders.length).toBe(rendersBefore);

    // Then the settle lands one render, at the scale the gesture ended on.
    await vi.waitFor(() => expect(renders.length).toBe(rendersBefore + 1), { timeout: 2000 });
    expect(renders[renders.length - 1].scale).toBeGreaterThan(scaleBefore);
    // And once the crisp raster arrives the stretch is retired rather than compounding.
    await vi.waitFor(() => expect(cssScaleOf(el)).toBeCloseTo(1, 3));
  });

  // The listener has to be non-passive, or the browser scrolls the page out from under the zoom.
  it("consumes the wheel event rather than letting the page scroll", async () => {
    const { el } = openWorkbench();
    await vi.waitFor(() => expect(pageEl(el)).toBeTruthy());

    const viewport = el.querySelector(".ds-viewport") as HTMLElement;
    const e = new WheelEvent("wheel", { deltaY: -120, clientX: 10, clientY: 10, bubbles: true, cancelable: true });
    viewport.dispatchEvent(e);
    expect(e.defaultPrevented).toBe(true);
  });
});
