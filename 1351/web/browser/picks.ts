// Helpers for driving sheet views (agni issue 1326) by point, as a reader's click is. A click lands
// wherever the picker says the point is, so a part is clicked at a point clear of its pads and its
// neighbours rather than at its element. sheetview.spec.ts and the cross-probe walk use them.

import type { Page } from "playwright-core";
import { inject } from "vitest";

const base = (): string => inject("baseUrl");

export const PANE = "[data-sheetview]";
export const TITLE = '[data-component="properties"] .pr-title';

export async function openViewer(page: Page, design: string): Promise<void> {
  await page.goto(`${base()}/designs/${design}/view`, { waitUntil: "networkidle" });
  await page.waitForSelector("#svg-view svg", { timeout: 30_000 });
}

// openBeside opens the sheet named exactly `name` in a new view from its Sheets tile, and waits for
// the view to draw.
export async function openBeside(page: Page, name: string): Promise<void> {
  const row = page.locator(".sheet-tile-row", { has: page.locator(".sheet-tile-name", { hasText: new RegExp(`^${name}$`) }) });
  await row.locator(".sheet-tile-beside").click();
  await page.waitForSelector(`${PANE} .sheetview-canvas svg`, { timeout: 30_000 });
}

// centreOf is an element's centre in client pixels.
export async function centreOf(page: Page, sel: string): Promise<{ x: number; y: number }> {
  return page.evaluate((sel) => {
    const el = document.querySelector(sel);
    if (!el) throw new Error(`no element matches ${sel}`);
    const r = el.getBoundingClientRect();
    return { x: Math.round(r.left + r.width / 2), y: Math.round(r.top + r.height / 2) };
  }, sel);
}

// clearPoint finds a whole-pixel point inside `sel` where every probe of the picker lands on `ref` or
// on nothing keyed, the search board-pick.spec.ts uses. `kind` "net" looks for the net named `ref`
// instead of the part.
export async function clearPoint(page: Page, sel: string, ref: string, kind: "component" | "net" = "component"): Promise<{ x: number; y: number }> {
  return page.evaluate(
    ({ sel, ref, kind }) => {
      const key = kind === "net" ? "data-net" : "data-ref";
      const keyed = (e: Element | null) => e?.closest("[data-kind]") ?? null;
      const el = document.querySelector(sel);
      if (!el) throw new Error(`no element matches ${sel}`);
      const r = el.getBoundingClientRect();
      for (let fy = 0.1; fy < 0.95; fy += 0.05) {
        for (let fx = 0.1; fx < 0.95; fx += 0.05) {
          const x = Math.round(r.left + r.width * fx);
          const y = Math.round(r.top + r.height * fy);
          const hits = [-5, 0, 5].flatMap((dx) => [-5, 0, 5].map((dy) => keyed(document.elementFromPoint(x + dx, y + dy))));
          const centre = keyed(document.elementFromPoint(x, y));
          if (centre?.getAttribute(key) === ref && centre.getAttribute("data-kind") === kind &&
            hits.every((h) => !h || (h.getAttribute(key) === ref && h.getAttribute("data-kind") === kind))) {
            return { x, y };
          }
        }
      }
      throw new Error(`no point inside ${sel} picks only ${ref}`);
    },
    { sel, ref, kind },
  );
}

// firstClear is the first part inside `within` with a clear point, skipping `skip`, since at fit
// zoom most bodies have pads or a neighbour within the probe ring, and a floating view can cover part
// of the canvas.
export async function firstClear(page: Page, within: string, skip: string[] = []): Promise<{ ref: string; x: number; y: number }> {
  const refs = await page.$$eval(`${within} [data-kind="component"][data-ref]`, (rs) => [...new Set(rs.map((r) => r.getAttribute("data-ref")!))]);
  for (const ref of refs.filter((r) => !skip.includes(r))) {
    const at = await clearPoint(page, `${within} [data-kind="component"][data-ref="${ref}"]`, ref).catch(() => null);
    if (at) return { ref, ...at };
  }
  throw new Error(`no part inside ${within} has a clear point`);
}

export async function picked(page: Page, want: string): Promise<void> {
  await page.waitForFunction(({ sel, want }) => document.querySelector(sel)?.textContent === want, { sel: TITLE, want }, { timeout: 30_000 });
}

// lit waits for a host's highlight overlay to draw something.
export async function lit(page: Page, host: string): Promise<void> {
  await page.waitForFunction((h) => document.querySelectorAll(`${h} .highlight-overlay svg *`).length > 0, host, { timeout: 30_000 });
}

// centredIn says whether an element's centre lies within the middle third of a host, which is where
// a reveal puts it.
export async function centredIn(page: Page, el: string, host: string): Promise<boolean> {
  return page.evaluate(
    ({ el, host }) => {
      const e = document.querySelector(el)!.getBoundingClientRect();
      const h = document.querySelector(host)!.getBoundingClientRect();
      const cx = e.left + e.width / 2;
      const cy = e.top + e.height / 2;
      return cx > h.left + h.width / 3 && cx < h.right - h.width / 3 && cy > h.top + h.height / 3 && cy < h.bottom - h.height / 3;
    },
    { el, host },
  );
}

