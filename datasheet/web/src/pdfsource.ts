// The workbench's port onto pdf.js, types only. Loading pdfrender.ts reaches for DOMMatrix, which
// jsdom lacks, so a file taking a value from it cannot load in a component test. The PORT lives here
// and the IMPLEMENTATION in pdfrender.ts, which is why regionview.tsx is testable. See
// docsite/content/architecture/web-client.md#a-library-that-cannot-load-is-a-component-that-cannot-be-tested.
//
// `import type` is erased at compile time, so re-exporting a pdf.js type does not load pdf.js.

import type { PDFDocumentProxy } from "pdfjs-dist";

export type { PDFDocumentProxy };

// RenderedPage is one datasheet page rasterized at a chosen scale. The page size is in PDF points,
// so a doc-IR BBox (page-local, top-left origin, y-down, points) times scale gives canvas pixels.
export interface RenderedPage {
  pageNumber: number; // 1-based
  canvas: HTMLCanvasElement;
  widthPts: number;
  heightPts: number;
  scale: number; // effective device pixels per PDF point
}

// PdfSource is everything the workbench asks of pdf.js. It opens a document, rasterizes one page,
// and names the URL the source bytes come from. `realPdfSource` in pdfrender.ts is the shipped
// implementation, passed in by the composition root.
export interface PdfSource {
  loadPdf(url: string): Promise<PDFDocumentProxy>;
  renderPage(doc: PDFDocumentProxy, pageNumber: number, scale: number): Promise<RenderedPage>;
  rawDatasheetUrl(mount: string, path: string): string;
}
