#!/usr/bin/env python3
"""pdf2doc: prototype doc-IR producer (WS10-006).

Converts a PDF into an agni.v1.doc Document in textproto form, using docling
(layout detection + TableFormer structure recognition). This is the derivation
pipeline's generic-stage prototype: it proves the doc-IR schema against a real
parser and real datasheets. It is run manually (the corpus PDFs are private and
never enter tests or CI); validate the output with:

    go run ./tools/pdf2doc/validate <out.textproto>

Requires: pip install docling  (heavy: pulls torch + layout models on first run).

Deliberate prototype boundaries:
  - table content hashes replicate doc.TableHash exactly (grid shape + cell
    position/span/text + footnotes); the Go validator recomputes and enforces.
  - footnote attachment is not attempted; footnotes remain page text blocks.
  - figure images are not extracted; figures carry caption + bbox only.
"""

import argparse
import hashlib
import sys
from importlib import metadata


def esc(s: str) -> str:
    return (s or "").replace("\\", "\\\\").replace('"', '\\"').replace("\n", "\\n")


def table_hash(rows, cols, cells, footnotes) -> str:
    h = hashlib.sha256()
    h.update(f"{rows}|{cols}".encode())
    for c in sorted(cells, key=lambda c: (c["row"], c["col"])):
        rs, cs = max(c["row_span"], 1), max(c["col_span"], 1)
        h.update(f"\x1f{c['row']},{c['col']},{rs},{cs}\x1e{c['text']}".encode())
    for fn in footnotes:
        h.update(f"\x1ffn\x1e{fn}".encode())
    return "sha256:" + h.hexdigest()


def bbox_fields(prov, page_sizes):
    bb = prov.bbox
    height = page_sizes[prov.page_no][1]
    tl = bb.to_top_left_origin(page_height=height)
    return tl.l, tl.t, tl.r - tl.l, tl.b - tl.t


def main() -> int:
    ap = argparse.ArgumentParser(description="PDF -> agni.v1.doc textproto (prototype)")
    ap.add_argument("pdf")
    ap.add_argument("-o", "--out", required=True)
    # Datasheets are born-digital PDFs with a real text layer, so OCR is off by default:
    # it is slower, lower-quality than the embedded text, and is the only reason docling
    # pulls the RapidOCR model weights. Pass --ocr for the rare scanned/raster datasheet.
    ap.add_argument("--ocr", action="store_true", help="enable OCR (for scanned PDFs)")
    args = ap.parse_args()

    from docling.document_converter import DocumentConverter, PdfFormatOption
    from docling.datamodel.base_models import InputFormat
    from docling.datamodel.pipeline_options import PdfPipelineOptions

    with open(args.pdf, "rb") as f:
        content_hash = "sha256:" + hashlib.sha256(f.read()).hexdigest()

    pipeline_options = PdfPipelineOptions()
    pipeline_options.do_ocr = args.ocr
    converter = DocumentConverter(
        format_options={InputFormat.PDF: PdfFormatOption(pipeline_options=pipeline_options)})
    result = converter.convert(args.pdf)
    d = result.document
    page_sizes = {no: (pg.size.width, pg.size.height) for no, pg in d.pages.items()}

    # Bucket every item by page, assigning deterministic per-page ids in document
    # order (p<N>.t<i> / p<N>.f<i> / p<N>.x<i>).
    pages = {no: {"tables": [], "figures": [], "texts": []} for no in sorted(page_sizes)}

    for item in d.texts:
        if not item.prov or not (item.text or "").strip():
            continue
        no = item.prov[0].page_no
        x, y, w, h = bbox_fields(item.prov[0], page_sizes)
        pages[no]["texts"].append(
            {"text": item.text, "kind": str(item.label), "bbox": (x, y, w, h)})

    for item in d.tables:
        if not item.prov:
            continue
        no = item.prov[0].page_no
        x, y, w, h = bbox_fields(item.prov[0], page_sizes)
        cells = []
        for c in item.data.table_cells:
            cells.append({
                "row": c.start_row_offset_idx,
                "col": c.start_col_offset_idx,
                "row_span": c.end_row_offset_idx - c.start_row_offset_idx,
                "col_span": c.end_col_offset_idx - c.start_col_offset_idx,
                "text": c.text or "",
                "is_header": bool(getattr(c, "column_header", False) or getattr(c, "row_header", False)),
            })
        pages[no]["tables"].append({
            "title": item.caption_text(d) or "",
            "rows": item.data.num_rows,
            "cols": item.data.num_cols,
            "cells": cells,
            "bbox": (x, y, w, h),
        })

    for item in d.pictures:
        if not item.prov:
            continue
        no = item.prov[0].page_no
        x, y, w, h = bbox_fields(item.prov[0], page_sizes)
        pages[no]["figures"].append({"caption": item.caption_text(d) or "", "bbox": (x, y, w, h)})

    producer = f"docling/{metadata.version('docling')}"
    # Document title: docling's document name (usually the source file stem); the
    # recipe layer matches doc_title_pattern against this, so it must not be empty.
    title = getattr(d, "name", "") or ""
    out = [f'content_hash: "{content_hash}"', 'source_format: "pdf"',
           f'title: "{esc(title)}"',
           f'producer: "{esc(producer)}"', f"page_count: {len(page_sizes)}", ""]

    def emit_bbox(indent, bb):
        x, y, w, h = bb
        out.append(f"{indent}bbox {{ x: {x:.2f} y: {y:.2f} width: {w:.2f} height: {h:.2f} }}")

    for no in sorted(pages):
        wpt, hpt = page_sizes[no]
        out.append("pages {")
        out.append(f"  number: {no}")
        out.append(f"  width: {wpt:.2f}")
        out.append(f"  height: {hpt:.2f}")
        for i, tb in enumerate(pages[no]["texts"], 1):
            out.append("  text_blocks {")
            out.append(f'    id: "p{no}.x{i}"')
            out.append(f'    text: "{esc(tb["text"])}"')
            emit_bbox("    ", tb["bbox"])
            out.append(f'    kind: "{esc(tb["kind"])}"')
            out.append("  }")
        for i, t in enumerate(pages[no]["tables"], 1):
            out.append("  tables {")
            out.append(f'    id: "p{no}.t{i}"')
            out.append(f'    title: "{esc(t["title"])}"')
            emit_bbox("    ", t["bbox"])
            out.append(f"    rows: {t['rows']}")
            out.append(f"    cols: {t['cols']}")
            for c in t["cells"]:
                parts = [f"row: {c['row']}", f"col: {c['col']}"]
                if c["row_span"] > 1:
                    parts.append(f"row_span: {c['row_span']}")
                if c["col_span"] > 1:
                    parts.append(f"col_span: {c['col_span']}")
                parts.append(f'text: "{esc(c["text"])}"')
                if c["is_header"]:
                    parts.append("is_header: true")
                out.append(f"    cells {{ {' '.join(parts)} }}")
            out.append("    confidence: 1")
            out.append(f'    content_hash: "{table_hash(t["rows"], t["cols"], t["cells"], [])}"')
            out.append("  }")
        for i, fg in enumerate(pages[no]["figures"], 1):
            out.append("  figures {")
            out.append(f'    id: "p{no}.f{i}"')
            out.append(f'    caption: "{esc(fg["caption"])}"')
            emit_bbox("    ", fg["bbox"])
            out.append("    confidence: 1")
            out.append("  }")
        out.append("}")

    with open(args.out, "w") as f:
        f.write("\n".join(out) + "\n")
    ntab = sum(len(p["tables"]) for p in pages.values())
    nfig = sum(len(p["figures"]) for p in pages.values())
    ntxt = sum(len(p["texts"]) for p in pages.values())
    print(f"{args.out}: {len(page_sizes)} pages, {ntab} tables, {nfig} figures, {ntxt} text blocks ({producer})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
