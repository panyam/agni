// The assertions that need a real browser, and nothing else.
//
// jsdom has no layout engine. Every element reports a zero-sized box, `elementFromPoint` answers
// nothing useful, and no CSS rule has any effect. So the 700-odd unit tests can prove what the
// panels RENDER and can prove nothing about what a reader can SEE, and a CSS bug shipped through a
// fully green suite (agni issue 337: a badge strip painting over the two columns beside it, on a
// table whose fixed layout does not clip).
//
// This file is deliberately small. A browser test is slow, flaky by nature and expensive to debug,
// so it earns its place only where pixels are the claim. Anything assertable in jsdom belongs in
// `src/*.test.ts` instead, and that is where it should be added.
//
// The technique matters as much as the coverage. `getBoundingClientRect` reports layout position and
// knows nothing about clipping, so a child of a scrolling cell reports coordinates far outside that
// cell while being perfectly contained on screen: reading those numbers as "it overflows" is wrong
// in the safe direction, which is the worst kind. Ask the document what is actually painted at a
// point instead (see `paintedAt`).

import { beforeAll, afterAll, describe, expect, it, inject } from "vitest";
import type { Browser, Page } from "playwright-core";
import { launch, withPage } from "./browser.js";

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

// openViewer loads a design and waits for the query panel to mount.
async function openViewer(page: Page, mount: string, file: string): Promise<void> {
  await page.goto(`${base()}/designs/${mount}/${file}/view`, { waitUntil: "networkidle" });
  await page.waitForSelector(".query textarea.query-text", { timeout: 30_000 });
}

// growQueryPanel enlarges the docked query panel, the same thing dragging its sash does. The dock
// opens it a couple of rows tall, which is not enough for the results table to be on screen at all.
async function growQueryPanel(page: Page): Promise<void> {
  await page.evaluate(() => {
    const overlay = document.querySelector(".query")?.closest<HTMLElement>(".dv-render-overlay");
    if (!overlay) throw new Error("query panel is not in a dock overlay");
    overlay.style.top = "420px";
    overlay.style.height = "560px";
    window.dispatchEvent(new Event("resize"));
  });
  await page.waitForTimeout(200);
}

// runQuery types a query into the box and runs it, for the questions find-by-name cannot ask.
async function runQuery(page: Page, text: string): Promise<void> {
  await page.fill("textarea.query-text", text);
  await page.click("button.query-run");
  await page.waitForSelector(".query-row", { timeout: 30_000 });
}

// search runs the panel's find-by-name mode and waits for rows.
async function search(page: Page, term: string): Promise<void> {
  await page.click(".query-mode:nth-of-type(2)");
  await page.fill("input.query-term", term);
  await page.press("input.query-term", "Enter");
  await page.waitForSelector(".query-row", { timeout: 30_000 });
}

// paintedAt asks the document what is actually drawn at a point inside `probe`, and reports whether
// the answer belongs to `probe` or to something else. This is the question a reader's eye asks, and
// the one a rectangle cannot answer.
// `fromLeft`, when given, probes that many pixels inside the element's left edge instead of at its
// centre. Bleed from the element to the LEFT lands at the boundary and fades out well before the
// middle: measured on a squeezed name column, the escaping chip reached 66px past its own cell while
// the neighbouring column was 301px wide, so a centre probe sat 84px clear of the damage and
// reported everything fine. The first version of this file did exactly that.
async function paintedAt(page: Page, probeSel: string, opts: { fromLeft?: number } = {}): Promise<{ ownedByProbe: boolean; painted: string }> {
  return page.evaluate(
    ({ sel, fromLeft }) => {
      const probe = document.querySelector(sel);
      if (!probe) throw new Error(`no element matches ${sel}`);
      const r = probe.getBoundingClientRect();
      const x = fromLeft === undefined ? r.left + r.width / 2 : r.left + fromLeft;
      const hit = document.elementFromPoint(x, r.top + r.height / 2);
      return {
        ownedByProbe: hit !== null && (hit === probe || probe.contains(hit)),
        painted: hit === null ? "nothing" : `${hit.tagName.toLowerCase()}.${hit.className}`,
      };
    },
    { sel: probeSel, fromLeft: opts.fromLeft },
  );
}

describe("query results table", () => {
  // agni issue 337. The table is fixed-layout, which keeps the columns equal and makes a dragged
  // width stick, and fixed layout does NOT clip: a cell whose content will not wrap draws straight
  // over the columns to its right. On a 21-sheet design the badge strip on a ground net was one
  // nowrap line whose last chip sat 1927px past its own cell.
  //
  // The public fixtures top out at three sheets, so the size of that content cannot be reproduced
  // here. The CONDITION can: dragging the name column down to its 48px minimum leaves a name and a
  // badge strip with nowhere to go, which is the same squeeze from the other direction. Without
  // that squeeze this test passes with the containment rules deleted entirely, which is how the
  // first version of it was written and what mutating the CSS revealed.
  it("never paints a cell's content over the column beside it", async () => {
    await withPage(browser, async (page) => {
      await page.setViewportSize({ width: 700, height: 1000 });
      await openViewer(page, "kicad", "hier_bus_root.kicad_sch");
      await growQueryPanel(page);
      await search(page, "DATA");

      // Drag the name column's grip far to the left. The panel clamps at MIN_COL_PX, so the exact
      // distance does not matter as long as it overshoots.
      const grip = page.locator(".query-table th:nth-child(2) .query-col-grip");
      const box = await grip.boundingBox();
      if (box === null) throw new Error("no column grip to drag");
      await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
      await page.mouse.down();
      await page.mouse.move(box.x - 600, box.y + box.height / 2, { steps: 8 });
      await page.mouse.up();
      await page.waitForTimeout(150);

      // The kind column sits to the right of the name column, so bleed arrives at its LEFT EDGE.
      // Probing there asks the question a reader's eye asks: is the thing painted here mine, or has
      // the name cell spilled into me?
      const kindCell = ".query-row:first-child td:nth-child(3)";
      const hit = await paintedAt(page, kindCell, { fromLeft: 6 });
      expect(hit.painted).not.toContain("sheet-badge");
      expect(hit.painted).not.toContain("query-locate");
      expect(hit.ownedByProbe).toBe(true);
    });
  }, 90_000);

  // agni issue 341. The mark tells a reader which of a net's sheets they are looking at. A mark
  // nobody can see is worse than no mark, because the unmarked strip reads as being somewhere else,
  // so what matters is not that the class is applied but that the chip is painted.
  it("paints the current-sheet badge where it can be seen", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "kicad", "hier_bus_root.kicad_sch");
      await growQueryPanel(page);
      await search(page, "DATA");

      await page.locator(".query-row .sheet-badge:not(.sheet-badge-more)").first().click();
      await page.waitForSelector(".sheet-badge.on", { timeout: 30_000 });

      const marked = ".sheet-badge.on";
      const hit = await paintedAt(page, marked);
      expect(hit.ownedByProbe).toBe(true);

      // It has to LOOK different from its neighbours, not merely carry a class. Comparing against an
      // unmarked chip is the assertion; checking the marked one has some background would pass on
      // the plain badge colour and prove nothing.
      const colours = await page.evaluate(() => {
        const on = document.querySelector(".sheet-badge.on");
        const off = document.querySelector(".sheet-badge:not(.on):not(.sheet-badge-more)");
        const bg = (el: Element | null) => (el === null ? "" : getComputedStyle(el).backgroundColor);
        return { on: bg(on), off: bg(off) };
      });
      expect(colours.on).not.toBe("");
      expect(colours.off).not.toBe("");
      expect(colours.on).not.toBe(colours.off);
    });
  }, 90_000);

  // agni issue 259. The count is the one number a reviewer might act on, so it has to be legible
  // rather than merely present in the DOM.
  it("shows the findings count for a selection without anything covering it", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "kicad", "hier_bus_root.kicad_sch");
      await growQueryPanel(page);
      await search(page, "DATA");
      await page.locator(".query-row .query-locate").first().click();
      await page.waitForSelector(".query-selection .query-findings", { timeout: 30_000 });

      const count = ".query-selection .query-findings";
      const hit = await paintedAt(page, count);
      expect(hit.ownedByProbe).toBe(true);

      // Before any run it must say so rather than report a clean entity, which is the claim the
      // whole affordance risks over-stating.
      const label = await page.locator(`${count} .query-findings-label`).innerText();
      expect(label).toBe("not checked yet");
      // The caveat is the part a reader must not miss, so what is asserted is that it OCCUPIES
      // SPACE and is painted, not that it exists.
      //
      // innerText is no good for this and looked like it was: for an element CSS has hidden, the
      // spec says innerText falls back to textContent, so a `display: none` caveat reads back its
      // full string and the assertion passes. Found by mutating the rule and watching the test stay
      // green.
      const caveat = `${count} .query-findings-caveat`;
      expect(await page.locator(caveat).textContent()).toBe("selected rules, this subject only");
      const box = await page.locator(caveat).boundingBox();
      expect(box?.width ?? 0).toBeGreaterThan(0);
      expect((await paintedAt(page, caveat)).ownedByProbe).toBe(true);
    });
  }, 90_000);
});

// A pin is the one entity whose identity does not fit in a table cell, so it is worth one browser
// assertion that the whole path holds: server types the column, carries the ref, client renders a
// link, and clicking it selects a pin rather than a thing called "5".
//
// jsdom covers each half of that. What it cannot see is the link being drawn at all.
describe("pin cells", () => {
  it("draws a pin result as a link and walks from it", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "kicad", "bus_resolved.kicad_sch");
      await growQueryPanel(page);
      await runQuery(page, "pin.net(?ref, ?pin, ?net)");

      // The pin column is the second cell (after the provenance toggle), and it has to be a link.
      const pinLink = page.locator(".query-row:first-child td:nth-child(3) .query-locate");
      expect(await pinLink.count()).toBe(1);
      expect((await paintedAt(page, ".query-row:first-child td:nth-child(3) .query-locate")).ownedByProbe).toBe(true);

      await pinLink.click();
      await page.waitForSelector(".query-selection-kind", { timeout: 30_000 });
      // Lowercased because innerText reports RENDERED text and this label is uppercased by CSS, so
      // it reads back "PIN". textContent would say "pin"; neither is wrong, they answer different
      // questions, and here the DOM's answer is the one being asserted.
      expect((await page.locator(".query-selection-kind").innerText()).toLowerCase()).toBe("pin");
      // Both halves, joined: a pin that lost its component would read as a bare designator here.
      expect(await page.locator(".query-selection-name").innerText()).toMatch(/^.+\..+$/);
    });
  }, 90_000);
});
