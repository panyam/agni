// pdfrender wraps pdf.js so the region viewer depends on a small, mockable surface instead of the
// library's API and worker wiring. The worker is served as a static bundle (see build.mjs) and runs
// entirely in the browser, so nothing about the datasheet leaves the deployment boundary (C16).
import { getDocument, GlobalWorkerOptions, type PDFDocumentProxy } from "pdfjs-dist";
import type { PdfSource, RenderedPage } from "./pdfsource.js";

// The pdf.js worker is bundled to /static/pdf.worker.js by build.mjs. Same-origin: no CDN, no
// network beyond the local `agni serve`.
GlobalWorkerOptions.workerSrc = "/static/pdf.worker.js";

export type { PDFDocumentProxy };
export type { PdfSource, RenderedPage } from "./pdfsource.js";

// loadPdf opens the document; the caller renders pages on demand (page navigation and zoom each
// re-render a single page rather than rasterizing the whole document up front).
export async function loadPdf(url: string): Promise<PDFDocumentProxy> {
  return getDocument({ url }).promise;
}

// renderPage rasterizes one page at the given scale (device pixels per PDF point). Each call makes
// its own canvas, so overlapping renders (a burst of zoom clicks) never contend for one canvas; a
// stale result is simply discarded by the caller.
export async function renderPage(doc: PDFDocumentProxy, pageNumber: number, scale: number): Promise<RenderedPage> {
  const page = await doc.getPage(pageNumber);
  const viewport = page.getViewport({ scale });
  const canvas = document.createElement("canvas");
  canvas.width = Math.ceil(viewport.width);
  canvas.height = Math.ceil(viewport.height);
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("pdfrender: no 2d canvas context");
  await page.render({ canvas, canvasContext: ctx, viewport }).promise;
  const unscaled = page.getViewport({ scale: 1 });
  return { pageNumber, canvas, widthPts: unscaled.width, heightPts: unscaled.height, scale };
}

// rawDatasheetUrl builds the same-origin URL the raw endpoint serves the source PDF at
// (/datasheets/raw/<mount>/<path...>), each segment percent-encoded to match the server's decode.
export function rawDatasheetUrl(mount: string, path: string): string {
  const segs = [mount, ...path.split("/")].filter((s) => s !== "").map(encodeURIComponent);
  return "/datasheets/raw/" + segs.join("/");
}

// realPdfSource is the shipped implementation. The composition root (datasheets.ts) passes it to
// the workbench, which knows only the port, so a test supplies a stub without loading pdf.js.
export const realPdfSource: PdfSource = { loadPdf, renderPage, rawDatasheetUrl };
