// The workbench's view of pdf.js: types only, no import of the library itself.
//
// This file exists because of what `import` costs at module load. pdfrender.ts sets
// GlobalWorkerOptions and touches pdf.js's canvas module the moment it is loaded, and that module
// reaches for DOMMatrix, which jsdom does not have. So any file importing pdfrender — even for a
// type, if it also takes a value from it — becomes a file no component test can load. Keeping the
// PORT here and the IMPLEMENTATION there is what lets regionview.tsx be rendered by a test at all.
//
// The library's own types are re-exported below with `import type`, which is erased at compile
// time, so naming a pdf.js type does not load pdf.js.

import type { PDFDocumentProxy } from "pdfjs-dist";

export type { PDFDocumentProxy };

// RenderedPage is one datasheet page rasterized at a chosen scale, carrying the source page size in
// PDF points so a doc-IR BBox (page-local, top-left origin, y-down, points) maps to canvas pixels
// by multiplying by scale.
export interface RenderedPage {
  pageNumber: number; // 1-based
  canvas: HTMLCanvasElement;
  widthPts: number;
  heightPts: number;
  scale: number; // effective device pixels per PDF point
}

// PdfSource is the whole of pdf.js as the workbench sees it: open a document, rasterize one page,
// and name the URL the source bytes come from. `realPdfSource` in pdfrender.ts is the shipped
// implementation, and the composition root passes it in, so the workbench never names the library.
export interface PdfSource {
  loadPdf(url: string): Promise<PDFDocumentProxy>;
  renderPage(doc: PDFDocumentProxy, pageNumber: number, scale: number): Promise<RenderedPage>;
  rawDatasheetUrl(mount: string, path: string): string;
}
