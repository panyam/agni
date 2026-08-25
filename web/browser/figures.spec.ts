// The docsite's hand-authored figures, measured as a browser actually renders them.
//
// A figure under `docsite/figures/` is inlined into its page at build time, so it inherits the
// page's theme and its `currentColor`. What that costs is that nothing in the Go gate can see the
// RESULT: `includefile_test.go` proves the path resolves, that no figure is uncalled, that no colour
// is a literal and that no blank line splits the raw-HTML block, and every one of those reads the
// file rather than the rendering. A caption printed past the canvas, two labels stacked on each
// other, or a label sitting on the wire it names all pass the whole gate and are obvious on screen.
//
// Three checks, and each one has caught something the other two passed:
//
//   bounds        an element outside its own viewBox. `max-width: 100%` then scales the figure to
//                 fit the overflow, so every label shrinks to pay for one stray caption.
//   text on text  two runs printed over each other. Both are inside the viewBox, so bounds is
//                 silent. Found one in a diagram whose every element was in bounds.
//   text on wire  a label a few pixels off, sitting exactly on the rail it names. Inside the
//                 viewBox and not touching another label, so the first two both pass it. This one
//                 found two figures on its first run.
//
// And a fourth that is not about geometry: every file under `figures/` has to be REACHED by the
// sweep. The sweep used to select on the `<title>` element's id, which silently skipped the two
// figures that named themselves with `aria-label` instead, and a skipped figure is indistinguishable
// from a clean one in the output. Selecting on `role="img"` is what closed that, and the coverage
// assertion is what stops it reopening.
//
// Aspect ratio is deliberately NOT asserted. `docsite/README.md` sets roughly 4:1 as a ceiling, and
// it is a judgement about whether a diagram earns its width rather than a defect: one shipped figure
// sits at 4.23:1 on purpose. Mermaid blocks are not covered here at all, because they render
// client-side from a `<pre>` and a fetched page never runs them; `README.md` says to render those
// with `mmdc` before committing.

import { beforeAll, afterAll, describe, expect, it, inject } from "vitest";
import type { Browser } from "playwright-core";
import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve, join } from "node:path";
import { launch, withPage } from "./browser.js";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, "..", "..");
const contentDir = resolve(repoRoot, "docsite", "content");
const figuresDir = resolve(repoRoot, "docsite", "figures");

const base = (): string => inject("docsiteUrl");

// markdownUnder walks the content tree. A hand-rolled walk rather than `fs.globSync`, which needs
// Node 22 and would make this file the one thing in the repo with a floor on the runtime.
function markdownUnder(dir: string, prefix = ""): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const rel = prefix ? `${prefix}/${entry.name}` : entry.name;
    if (entry.isDirectory()) out.push(...markdownUnder(join(dir, entry.name), rel));
    else if (entry.name.endsWith(".md")) out.push(rel);
  }
  return out;
}

// pagesIncludingAFigure derives the URLs to sweep from the content that includes a figure, rather
// than from a list someone maintains. A page added with a figure joins the sweep by existing.
function pagesIncludingAFigure(): string[] {
  const urls = new Set<string>();
  for (const file of markdownUnder(contentDir)) {
    if (!/includeFile\s+"figures\//.test(readFileSync(resolve(contentDir, file), "utf8"))) continue;
    const rel = file.replace(/\.md$/, "");
    urls.add(rel.endsWith("/index") || rel === "index" ? `/agni/${rel.replace(/index$/, "")}` : `/agni/${rel}/`);
  }
  return [...urls].sort();
}

function figureFiles(): string[] {
  return readdirSync(figuresDir).filter((f) => f.endsWith(".svg")).sort();
}

type Problem = { figure: string; kind: string; detail: string };
type SweepResult = { seen: string[]; problems: Problem[] };

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

// sweep fetches every page into a detached div in ONE browser context and measures each figure
// there. A detached div is the right harness for this and the wrong one for anything the page grid
// decides: `getBBox()` reports SVG user-space coordinates, which do not depend on where the element
// sits, while a table's width very much does.
async function sweep(): Promise<SweepResult> {
  const urls = pagesIncludingAFigure();
  let out: SweepResult = { seen: [], problems: [] };
  await withPage(browser, async (page) => {
    await page.goto(`${base()}/agni/`, { waitUntil: "domcontentloaded" });
    out = (await page.evaluate(async (paths: string[]) => {
      const seen = new Set<string>();
      const problems: { figure: string; kind: string; detail: string }[] = [];
      for (const path of paths) {
        const res = await fetch(path);
        if (!res.ok) {
          problems.push({ figure: path, kind: "page did not load", detail: `HTTP ${res.status}` });
          continue;
        }
        const div = document.createElement("div");
        div.style.cssText = "position:absolute;left:-99999px;top:0;width:800px";
        div.innerHTML = await res.text();
        document.body.appendChild(div);
        // role="img" rather than the title's id: a figure may name itself with aria-label, and
        // keying on the title is what let two of them go unswept for their whole life.
        for (const svg of Array.from(div.querySelectorAll<SVGSVGElement>('svg[role="img"][viewBox]'))) {
          const title = svg.querySelector("title");
          const name = title?.id || svg.getAttribute("aria-label")?.slice(0, 40) || "(unnamed)";
          if (seen.has(name)) continue;
          seen.add(name);
          const vb = svg.viewBox.baseVal;
          const runs: { b: DOMRect; s: string }[] = [];
          for (const t of Array.from(svg.querySelectorAll<SVGTextElement>("text"))) {
            runs.push({ b: t.getBBox(), s: (t.textContent || "").slice(0, 40) });
          }
          for (const el of Array.from(svg.querySelectorAll<SVGGraphicsElement>("text,rect,circle,line,path"))) {
            let b: DOMRect;
            try {
              b = el.getBBox();
            } catch {
              continue;
            }
            if (b.width === 0 && b.height === 0) continue;
            const over = Math.max(
              vb.x - b.x, vb.y - b.y,
              b.x + b.width - (vb.x + vb.width),
              b.y + b.height - (vb.y + vb.height),
            );
            if (over > 1) {
              problems.push({
                figure: name, kind: "outside its viewBox",
                detail: `${(el.textContent || el.tagName).slice(0, 40)} by ${over.toFixed(1)}px`,
              });
            }
          }
          for (let i = 0; i < runs.length; i++) {
            for (let j = i + 1; j < runs.length; j++) {
              const a = runs[i].b, c = runs[j].b;
              const ox = Math.min(a.x + a.width, c.x + c.width) - Math.max(a.x, c.x);
              const oy = Math.min(a.y + a.height, c.y + c.height) - Math.max(a.y, c.y);
              if (ox > 1.5 && oy > 1.5) {
                problems.push({ figure: name, kind: "text on text", detail: `${runs[i].s} / ${runs[j].s}` });
              }
            }
          }
          // A stroke's own box is thin on one axis, which is what separates a wire from a panel.
          //
          // getBBox() reports the GEOMETRY and excludes the stroke, so a horizontal line's box is
          // zero tall and a vertical one's is zero wide. Comparing a text box against that raw box
          // can never overlap on the thin axis, which is a check that cannot fail: this fired on
          // nothing at all until a red-check moved a label onto a rail and the suite stayed green.
          // Inflate the thin axis by half the stroke, with a floor, so the box covers the ink.
          for (const el of Array.from(svg.querySelectorAll<SVGGraphicsElement>("line,path"))) {
            const raw = el.getBBox();
            if (!(raw.width < 3 || raw.height < 3)) continue;
            const stroke = parseFloat(getComputedStyle(el).strokeWidth) || 1;
            const pad = Math.max(stroke / 2, 3);
            const lb = {
              x: raw.x - pad, y: raw.y - pad,
              width: raw.width + 2 * pad, height: raw.height + 2 * pad,
            };
            for (const r of runs) {
              const ox = Math.min(r.b.x + r.b.width, lb.x + lb.width) - Math.max(r.b.x, lb.x);
              const oy = Math.min(r.b.y + r.b.height, lb.y + lb.height) - Math.max(r.b.y, lb.y);
              if (ox > 6 && oy > 4) {
                problems.push({ figure: name, kind: "text sitting on a wire", detail: r.s });
              }
            }
          }
        }
        div.remove();
      }
      return { seen: [...seen], problems };
    }, urls)) as SweepResult;
  });
  return out;
}

describe("docsite figures", () => {
  let result: SweepResult;
  beforeAll(async () => {
    result = await sweep();
  }, 180_000);

  it("reaches every figure under docsite/figures", () => {
    // The count, not the names: a figure's accessible name is prose and need not match its filename.
    // What matters is that nothing is silently skipped, which a count catches and a spot check does not.
    expect(result.seen.length).toBe(figureFiles().length);
  });

  it("draws nothing outside its own canvas, on any figure", () => {
    const bad = result.problems.filter((p) => p.kind === "outside its viewBox");
    expect(bad.map((p) => `${p.figure}: ${p.detail}`)).toEqual([]);
  });

  it("prints no label over another label", () => {
    const bad = result.problems.filter((p) => p.kind === "text on text");
    expect(bad.map((p) => `${p.figure}: ${p.detail}`)).toEqual([]);
  });

  it("prints no label on top of a wire", () => {
    const bad = result.problems.filter((p) => p.kind === "text sitting on a wire");
    expect(bad.map((p) => `${p.figure}: ${p.detail}`)).toEqual([]);
  });

  it("loads every page that includes a figure", () => {
    const bad = result.problems.filter((p) => p.kind === "page did not load");
    expect(bad.map((p) => `${p.figure}: ${p.detail}`)).toEqual([]);
  });
});
