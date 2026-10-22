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
import { growQueryPanel } from "./panels.js";

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
      // The rules this design cannot run ride with it (agni issue 950), named so a clean count is never
      // read as a full one. They are the ones the Checks panel lists, so the count is read from there.
      const cannot = await page.locator(".checks-skipped-list li").count();
      expect(cannot, "the Checks panel names no rule this bare schematic cannot run").toBeGreaterThan(0);
      expect(await page.locator(caveat).textContent()).toBe(`selected rules, this subject only · ${cannot} rule(s) could not run`);
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

// agni issue 348: the focused finding was painted in the same color as the field it sits in.
//
// Both layers went out with no color, so both resolved to DEFAULT_HIGHLIGHT_COLOR and the entire
// distinction between "the thing I clicked" and "the other eleven on this sheet" was carried by
// alpha and shape within one hue. The unit tests can prove the presenter EMITS two different specs
// and the Go tests can prove the renderer HONORS them, and neither can prove that two colors reach
// the page: that needs a real client stamping a real spec, a real server rendering it, and a real
// browser painting the result.
//
// Asserted on distinct stroke colors in the overlay the reader is looking at, not on the spec. The
// spec assertion lives in src/findings.test.ts and would pass with the whole SVG path broken.
describe("highlight layers", () => {
  it("paints the focused finding in a different color from the rest of the field", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "kicad", "hier_bus_root.kicad_sch");

      // SVG mode: the overlay is a served document in the DOM, so its paint is inspectable. WebGL
      // resolves specs into buffers with nothing to read back short of pixels.
      await page.click(`.render-mode .mode-btn:has-text("SVG")`);
      // Scoped to #svg-view: the diff panel builds two more SvgViews, and their overlays are empty
      // and hidden, so an unscoped selector waits forever on the wrong one.
      await page.waitForSelector("#svg-view .highlight-overlay", { state: "attached", timeout: 30_000 });

      // The field is opt-in (agni issue 356), so the reader turns it on in the Highlight menu.
      await page.click(".highlight-btn");
      await page.check(".highlight-show-all");
      await page.click(".highlight-btn");

      await page.click(".checks-run");
      // This design raises 12 findings across two rules, so there is a field for a focus to sit in.
      // With one finding the base would be empty and the test would pass on a single layer.
      await page.waitForSelector(".check-locate", { timeout: 30_000 });
      const rows = await page.locator(".check-locate").count();
      expect(rows, "a single finding leaves no field, so this could not detect a shared color").toBeGreaterThan(1);

      // A DRAWN subject, chosen deliberately. Four of this design's findings are dangling-endpoint,
      // whose subject is a coordinate pair rather than an entity, so their focus layer matches
      // nothing and paints nothing. Clicking one of those makes this test go green for the wrong
      // reason: one layer on the page, one color, no way to tell a shared hue from a missing layer.
      const focus = page.locator(".check-locate", { hasText: "DATA0" }).first();
      await expect.poll(() => focus.count(), { timeout: 30_000 }).toBeGreaterThan(0);
      await focus.click();

      const strokes = await page.evaluate(async () => {
        // The overlay is fetched per focus, so give the round-trip a moment to land.
        for (let i = 0; i < 60; i++) {
          const found = new Set<string>();
          for (const el of document.querySelectorAll("#svg-view .highlight-overlay svg *")) {
            const s = el.getAttribute("stroke") ?? el.getAttribute("fill");
            if (s && s !== "none") found.add(s.toLowerCase());
          }
          if (found.size > 1) return [...found];
          await new Promise((r) => setTimeout(r, 100));
        }
        return [
          ...new Set(
            [...document.querySelectorAll("#svg-view .highlight-overlay svg *")]
              .map((el) => (el.getAttribute("stroke") ?? el.getAttribute("fill") ?? "").toLowerCase())
              .filter((s) => s && s !== "none"),
          ),
        ];
      });

      expect(strokes.length, `the overlay painted one color for both layers: ${strokes.join(", ")}`).toBeGreaterThan(1);
    });
  }, 120_000);
});

// agni issue 356: a run lights nothing until a finding is clicked, so a sheet with dozens of findings
// is not a wash of the focus colour before the reader has chosen anything.
describe("the run-wide field", () => {
  it("is off after a run, and a clicked finding is drawn", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "kicad", "hier_bus_root.kicad_sch");
      await page.click(`.render-mode .mode-btn:has-text("SVG")`);
      await page.waitForSelector("#svg-view .highlight-overlay", { state: "attached", timeout: 30_000 });
      await page.click(".checks-run");
      await page.waitForSelector(".check-locate", { timeout: 30_000 });
      expect(await page.locator(".check-locate").count()).toBeGreaterThan(1);
      // Give a field the same time a focus takes to land, so an empty overlay is not just a slow one.
      await page.waitForTimeout(1500);
      expect(await page.locator("#svg-view .highlight-overlay svg *").count()).toBe(0);

      const focus = page.locator(".check-locate", { hasText: "DATA0" }).first();
      await expect.poll(() => focus.count(), { timeout: 30_000 }).toBeGreaterThan(0);
      await focus.click();
      await page.waitForFunction(() => document.querySelectorAll("#svg-view .highlight-overlay svg *").length > 0, null, { timeout: 30_000 });
    });
  }, 120_000);
});

// agni issue 354: the notice that says the drawing in front of you is incomplete.
//
// It is here rather than in jsdom for the reason issue 337 was: a full-width strip over the canvas
// has neighbours, and jsdom reports every box as zero-sized so it cannot see one painting over
// another. The first version of this strip sat at top:8px and covered the render-mode buttons and the
// element readout completely. The unit tests were green throughout.
describe("the incomplete-drawing notice", () => {
  it("appears for a design whose symbols did not resolve, without covering the controls", async () => {
    await withPage(browser, async (page) => {
      // Two of this fixture's three placements name a library nothing supplies; the third resolves,
      // so the sheet still draws something and the notice is about a PARTIAL loss rather than a
      // total one.
      await openViewer(page, "kicad", "undrawn.kicad_sch");
      const note = page.locator(".undrawn-note.on");
      await note.waitFor({ timeout: 30_000 });
      expect(await note.textContent()).toContain("missinglib:MCU");

      // Boxes, not appearance: the strip must not cover the render-mode buttons or the readout. Both
      // sit in the first ~43px of the canvas, which is why the strip clears them rather than starting
      // at the top edge.
      const overlaps = await page.evaluate(() => {
        const box = (sel: string) => {
          const el = document.querySelector(sel);
          if (!el) return null;
          const r = el.getBoundingClientRect();
          return { l: r.left, t: r.top, r: r.right, b: r.bottom };
        };
        const hit = (a: ReturnType<typeof box>, x: ReturnType<typeof box>) =>
          !!a && !!x && !(a.r <= x.l || x.r <= a.l || a.b <= x.t || x.b <= a.t);
        const n = box(".undrawn-note");
        return { controls: hit(n, box(".controls")), readout: hit(n, box("#readout")) };
      });
      expect(overlaps.controls, "the notice paints over the render-mode buttons").toBe(false);
      expect(overlaps.readout, "the notice paints over the element readout").toBe(false);
    });
  }, 120_000);

  it("stays hidden for a design that draws completely", async () => {
    // The control that keeps the notice worth reading. Without it a strip that was always visible
    // would satisfy every assertion above.
    await withPage(browser, async (page) => {
      await openViewer(page, "kicad", "hier_bus_root.kicad_sch");
      await page.waitForTimeout(1500);
      expect(await page.locator(".undrawn-note.on").count()).toBe(0);
    });
  }, 120_000);
});

// agni issue 717: the notice that says the read left hierarchical blocks out.
//
// It is asserted by its painted box for the reason the incomplete-drawing notice is: an element with
// the right class and text can still be zero-sized, hidden behind the canvas, or painted over the
// controls, and jsdom sees none of that. The sentence is the server's, so it is compared whole.
describe("the hierarchical-read notice", () => {
  const sentence =
    "hier.edn is hierarchical and only its top cell was read. 1 sub-cell holding 1 instance was not extracted (SUB: 1). Counts and checks cover the top cell alone.";

  // agni issue 1102: the strip's button re-opens the design read whole, which the engine answers
  // with no unexpanded cell, so the strip is gone. The visible strip before the click is the control.
  it("reads every block when asked, and then has nothing left to say", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "edif", "hier.edn");
      const button = page.locator(".hierarchy-note.on button.hierarchy-read-all");
      await button.waitFor({ state: "visible", timeout: 30_000 });
      await Promise.all([page.waitForURL(/[?&]flatten=1/, { timeout: 30_000 }), button.click()]);
      await page.waitForSelector(".query textarea.query-text", { timeout: 30_000 });
      await page.waitForTimeout(1500);
      expect(await page.locator(".hierarchy-note.on").count()).toBe(0);
      expect(page.url()).toContain("flatten=1");
    });
  }, 120_000);

  it("is painted on a hierarchical design, below the controls and clear of them", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "edif", "hier.edn");
      const note = page.locator(".hierarchy-note.on");
      await note.waitFor({ state: "visible", timeout: 30_000 });
      expect((await note.locator("span").textContent())?.trim()).toBe(sentence);

      const geometry = await page.evaluate(() => {
        const box = (sel: string) => {
          const el = document.querySelector(sel);
          if (!el) return null;
          const r = el.getBoundingClientRect();
          return { l: r.left, t: r.top, r: r.right, b: r.bottom, w: r.width, h: r.height };
        };
        const hit = (a: ReturnType<typeof box>, x: ReturnType<typeof box>) =>
          !!a && !!x && !(a.r <= x.l || x.r <= a.l || a.b <= x.t || x.b <= a.t);
        const n = box(".hierarchy-note");
        const canvas = box("#view");
        // What the browser would paint at the note's centre, which is the note (or its text) only
        // when nothing sits on top of it. The notes take no pointer events, so a hit test skips
        // them; they take them for the one test and are put back.
        const el = document.querySelector<HTMLElement>(".hierarchy-note")!;
        const stack = el.parentElement as HTMLElement;
        el.style.pointerEvents = stack.style.pointerEvents = "auto";
        const r = el.getBoundingClientRect();
        const top = document.elementsFromPoint(r.left + r.width / 2, r.top + r.height / 2)[0];
        el.style.pointerEvents = stack.style.pointerEvents = "";
        return {
          note: n,
          inCanvas: !!n && !!canvas && n.l >= canvas.l && n.r <= canvas.r && n.t >= canvas.t && n.b <= canvas.b,
          onTop: !!top && (top === el || el.contains(top)),
          controls: hit(n, box(".controls")),
          readout: hit(n, box("#readout")),
        };
      });
      expect(geometry.note!.w, "the notice has no width").toBeGreaterThan(100);
      expect(geometry.note!.h, "the notice has no height").toBeGreaterThan(10);
      expect(geometry.inCanvas, "the notice is painted outside the canvas").toBe(true);
      expect(geometry.onTop, "something is painted over the notice").toBe(true);
      expect(geometry.controls, "the notice paints over the render-mode buttons").toBe(false);
      expect(geometry.readout, "the notice paints over the element readout").toBe(false);
    });
  }, 120_000);

  it("stays hidden for a flat design", async () => {
    // The control: a strip shown on every open would satisfy every assertion above.
    await withPage(browser, async (page) => {
      await openViewer(page, "edif", "basic.edn");
      await page.waitForTimeout(1500);
      expect(await page.locator(".hierarchy-note.on").count()).toBe(0);
      expect((await page.locator("#hierarchy-note").textContent())?.trim() ?? "").toBe("");
    });
  }, 120_000);
});

// agni issue 350. A severity mark is an empty span styled into a dot, and an INLINE span ignores
// width and height, so the dot measured 0x0 from the day WS9 moved it out of the old grid row into a
// table cell. Every unit test passed throughout: the class was on the element, the element was in
// the DOM, and nothing was painted. That is this file's whole reason to exist, and it is why the two
// marks are asserted by their PAINTED SIZE rather than by their class.
describe("the checks table's severity marks", () => {
  it("paints a defect's dot and an undecided row's mark, and does not paint them the same", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "conformance", "reverse-blocking.undecided.kicad_sch");
      await page.getByText("Checks", { exact: true }).first().click();
      await page.click(".checks-run");
      await page.waitForSelector(".check-row", { timeout: 60_000 });

      const boxes = async (sel: string) =>
        page.$$eval(sel, (els) =>
          els.map((e) => {
            const r = e.getBoundingClientRect();
            return { w: r.width, h: r.height, color: getComputedStyle(e).backgroundColor };
          }),
        );

      const dots = await boxes(".check-sev .sev-dot");
      expect(dots.length).toBeGreaterThan(0);
      for (const d of dots) {
        expect(d.w).toBeGreaterThan(0);
        expect(d.h).toBeGreaterThan(0);
      }

      // The undecided row takes a "?" instead, and it is the only one: this fixture reports three
      // defects and one inconclusive result.
      const marks = await boxes(".check-sev .sev-unresolved");
      expect(marks.length).toBe(1);
      expect(marks[0].w).toBeGreaterThan(0);
      expect(marks[0].h).toBeGreaterThan(0);
      expect(await page.$$eval(".check-row.inconclusive .sev-dot", (e) => e.length)).toBe(0);
    });
  }, 90_000);
});

// agni issue 902. A click on the drawing selects, and Properties describes the selection. jsdom can
// prove the panel renders a state, and only a browser can prove a real click on a painted part lands
// there, that the panel is on screen rather than present and hidden, and that the query box the reader
// typed in is untouched. The design is opened as a FOLDER, the spelling whose datasheet read failed
// with invalid_argument on every open until agni issue 736.
describe("Properties", () => {
  it("describes a clicked part, keeps the typed query, and asks about it from Ask", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut", "designs/gateway");
      await page.fill("textarea.query-text", "net.rail(?n) => ?n");

      const at = await page.evaluate(() => {
        const el = document.querySelector('#svg-view [data-kind="component"][data-ref="U1"]');
        if (!el) throw new Error("the drawing keys no U1");
        const r = el.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
      });
      await page.mouse.click(at.x, at.y);

      await page.waitForSelector('[data-component="properties"] .pr-title', { timeout: 30_000 });
      expect(await page.locator('[data-component="properties"] .pr-title').innerText()).toBe("U1");
      expect((await paintedAt(page, '[data-component="properties"] .pr-title')).ownedByProbe).toBe(true);
      // The tutorial project seeds U1's part number, so the datasheet line names a spec rather than
      // reporting no corpus, which is what the folder spelling showed before agni issue 736.
      await page.waitForFunction(() => /parameters/.test(document.querySelector(".pr-datasheet")?.textContent ?? ""), null, {
        timeout: 30_000,
      });
      expect(await page.inputValue("textarea.query-text")).toBe("net.rail(?n) => ?n");

      await page.click(".pr-ask-go");
      await page.waitForSelector(".query-row, .query-empty", { timeout: 30_000 });
      expect(await page.inputValue("textarea.query-text")).not.toBe("net.rail(?n) => ?n");
      expect(await page.locator(".query-selection-name").innerText()).toBe("U1");
    });
  }, 90_000);

  // Each Ask row opens a tab of its own, and the drawing follows the tab in front: a tab that located
  // a cell shows it again when it comes back, and one that located nothing shows no cell.
  it("opens an Ask row in a named tab, and the tab in front drives the highlight", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut", "designs/gateway");
      await page.fill("textarea.query-text", "net.rail(?n) => ?n");
      const at = await page.evaluate(() => {
        const r = document.querySelector('#svg-view [data-kind="component"][data-ref="U1"]')!.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
      });
      await page.mouse.click(at.x, at.y);
      await page.waitForSelector(".pr-ask-go", { timeout: 30_000 });
      const asks = await page.$$eval(".pr-ask-go", (bs) => bs.map((b) => b.textContent));
      expect(asks.length).toBeGreaterThan(1);

      await page.click('.pr-ask-go:has-text("What nets does it touch?")');
      const front = page.locator(".query-tab.on .query-tab-go");
      await page.waitForFunction(() => document.querySelector(".query-tab.on .query-tab-go")?.textContent === "U1: What nets does it touch?", null, {
        timeout: 30_000,
      });
      expect((await paintedAt(page, ".query-tab.on .query-tab-go")).ownedByProbe).toBe(true);
      await growQueryPanel(page);
      await page.waitForSelector('.query-row .query-locate:has-text("GND")', { timeout: 30_000 });

      await page.click('.query-row .query-locate:has-text("GND")');
      await page.waitForFunction(() => document.querySelectorAll("#svg-view .highlight-overlay svg *").length > 0, null, { timeout: 30_000 });

      // Query 1 still holds the typed query, located nothing, and no checks have run, so the
      // drawing carries no highlight at all while it is in front.
      await page.click('.query-tab-go:has-text("Query 1")');
      await page.waitForFunction(() => document.querySelectorAll("#svg-view .highlight-overlay svg *").length === 0, null, { timeout: 30_000 });
      expect(await page.inputValue("textarea.query-text")).toBe("net.rail(?n) => ?n");

      await page.click('.query-tab-go:has-text("U1: What nets does it touch?")');
      await page.waitForFunction(() => document.querySelectorAll("#svg-view .highlight-overlay svg *").length > 0, null, { timeout: 30_000 });
      expect(await front.innerText()).toBe("U1: What nets does it touch?");
      expect(await page.locator(".query-locate.on").first().innerText()).toBe("GND");
    });
  }, 90_000);

  // agni issue 745. A project's query set opens from the Sets menu as one tab per query, and a set
  // file from disk does too, with a query the engine cannot answer showing its error in its own tab.
  // The menu sits beside the tab strip, which scrolls, so this also proves it is painted unclipped.
  it("opens a project's query set, and a set file, as one tab per query", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut", "designs/gateway");
      await growQueryPanel(page);
      await page.click(".query-sets-open");
      await page.waitForSelector(".query-set-item", { timeout: 30_000 });
      expect((await paintedAt(page, ".query-set-item")).ownedByProbe).toBe(true);
      expect(await page.locator(".query-set-item").innerText()).toBe("Power review (3)");
      await page.click(".query-set-item");
      await page.waitForSelector(".query-row", { timeout: 30_000 });
      const titles = () => page.$$eval(".query-tab-go", (bs) => bs.map((b) => b.getAttribute("title")));
      expect(await titles()).toEqual(["Query 1", "Rails", "Rails with no test point", "Regulator power pins"]);
      expect(await page.locator(".query-row").count()).toBeGreaterThan(0);

      await page.click(".query-sets-open");
      await page.setInputFiles(".query-set-input", {
        name: "mixed.yaml",
        mimeType: "text/yaml",
        buffer: Buffer.from("queries:\n  - name: Good\n    query: net.rail(?n) => ?n\n  - name: Typo\n    query: net.rial(?n) => ?n\n"),
      });
      await page.waitForFunction(() => document.querySelector(".query-tab.on .query-tab-go")?.getAttribute("title") === "Good", null, { timeout: 30_000 });
      await page.waitForSelector(".query-row", { timeout: 30_000 });
      await page.click('.query-tab-go[title="Typo"]');
      await page.waitForSelector(".query-error", { timeout: 30_000 });
      expect(await page.locator(".query-error").innerText()).toContain("net.rial");
    });
  }, 90_000);

  it("selects the sheet for a click on empty paper", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut", "designs/gateway");
      // A corner of the canvas inside the sheet border, where nothing is keyed.
      const at = await page.evaluate(() => {
        const r = document.querySelector("#svg-view svg")!.getBoundingClientRect();
        return { x: r.left + r.width * 0.5, y: r.top + r.height * 0.97 };
      });
      await page.mouse.click(at.x, at.y);
      await page.waitForFunction(() => document.querySelectorAll('[data-component="properties"] .pr-crumb').length === 2, null, {
        timeout: 30_000,
      });
      expect(await page.locator('[data-component="properties"] .pr-kind').innerText()).toMatch(/sheet/i);
    });
  }, 90_000);
});

// agni issue 1030. A checklist run in the Review panel saves as the page `agni review --format html`
// writes, through the same rpc. The download is read back, so this proves a reader gets the page and
// not merely that a button exists.
describe("Review", () => {
  it("runs a checklist and saves it as the checklist page", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut", "designs/gateway");
      await page.waitForSelector("select.rv-checklist option[value]", { state: "attached", timeout: 30_000 });
      await page.selectOption("select.rv-checklist", { label: "review" });
      await page.click("button.rv-run");
      await page.waitForSelector(".rv-summary", { timeout: 60_000 });
      const tally = await page.locator(".rv-summary").innerText();
      expect((await paintedAt(page, ".rv-save")).ownedByProbe).toBe(true);

      const [download] = await Promise.all([page.waitForEvent("download", { timeout: 30_000 }), page.click(".rv-save")]);
      expect(download.suggestedFilename()).toBe("gateway.agni-review.html");
      const path = await download.path();
      const html = (await import("node:fs")).readFileSync(path!, "utf8");
      // The saved page's text as a browser reads it, since the page escapes what a title holds.
      const text = await page.evaluate((h) => new DOMParser().parseFromString(h, "text/html").body.textContent ?? "", html);
      const items = await page.$$eval(".rv-item-title", (els) => els.map((e) => e.textContent ?? ""));
      expect(items.length).toBeGreaterThan(0);
      for (const title of items) expect(text).toContain(title.trim());
      // The page and the panel word the tally differently, and agree on the counts.
      const counts = tally.match(/\d+ pass, \d+ fail/)?.[0];
      expect(counts, `the panel's tally ${tally} names no pass and fail counts`).toBeTruthy();
      expect(text).toContain(counts!);
    });
  }, 120_000);
});

// agni issue 759. The boot layout's sizes, which only a browser can measure. The Query panel opened at
// a fifth of the dock's height, too short for a single result row under its tab strip and query box,
// so the #845 walk and these specs enlarged it by hand before they could click one. The Checks
// table's headers wrapped a letter per line in the east rail. And Changes, re-added beside the diff
// after a reader closed it, never took its rail width, because a panel-level setSize is dropped.
describe("the default dock", () => {
  it("opens the query panel tall enough to show an answer's first rows", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut", "designs/gateway");
      await runQuery(page, "component.net(?part, ?net) => ?part, ?net");
      const seen = await page.evaluate(() => {
        const panel = document.querySelector(".query")?.closest(".dv-render-overlay") ?? document.querySelector(".query");
        const box = panel!.getBoundingClientRect();
        const bottom = Math.min(box.bottom, window.innerHeight);
        return Array.from(document.querySelectorAll(".query-row")).filter((r) => {
          const b = r.getBoundingClientRect();
          return b.height > 0 && b.top >= box.top && b.bottom <= bottom;
        }).length;
      });
      expect(seen, "result rows wholly on screen inside the query panel, with nothing enlarged").toBeGreaterThanOrEqual(3);
    });
  }, 90_000);

  it("keeps every checks-table header on one line", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut", "designs/gateway");
      await page.click(".checks-run");
      await page.waitForSelector(".check-row", { timeout: 60_000 });
      const lines = await page.$$eval(".checks-table th", (ths) =>
        ths
          .filter((th) => (th.textContent ?? "").trim() !== "")
          .map((th) => {
            // The text's block height against the header's font size. A sort arrow sits beside its
            // label at a smaller size, so the boxes' tops differ on one line; their span does not.
            const range = document.createRange();
            range.selectNodeContents(th);
            const rects = Array.from(range.getClientRects()).filter((r) => r.height > 0);
            const span = Math.max(...rects.map((r) => r.bottom)) - Math.min(...rects.map((r) => r.top));
            return { label: (th.textContent ?? "").trim(), lines: span / parseFloat(getComputedStyle(th).fontSize) };
          }),
      );
      expect(lines.length, "the table has headers to measure").toBeGreaterThan(2);
      for (const l of lines) expect(l.lines, `header "${l.label}" spans more than one line`).toBeLessThan(1.8);
    });
  }, 90_000);

  it("resets a rearranged dock to the default from the Panels menu, keeping what the panels hold", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut", "designs/gateway");
      const queryHeight = () =>
        page.evaluate(() => {
          const tab = Array.from(document.querySelectorAll(".tab")).find((t) => t.textContent?.trim().startsWith("Query"));
          return Math.round(tab?.closest(".groupview")?.getBoundingClientRect().height ?? 0);
        });
      const hasTab = (title: string) => page.locator(".tab", { hasText: title }).count();
      const defaultHeight = await queryHeight();

      await page.fill("textarea.query-text", "component.class(?c, \"resistor\") => ?c");
      const rules = page.locator(".tab", { hasText: "Rules" });
      await rules.hover();
      await rules.locator(".dv-default-tab-action").click();
      await expect.poll(() => hasTab("Rules")).toBe(0);
      await growQueryPanel(page);
      expect(await queryHeight(), "the drag made the query panel taller").toBeGreaterThan(defaultHeight + 100);

      await page.click(".panels-btn");
      await page.click(".panels-reset");
      await expect.poll(() => hasTab("Rules")).toBe(1);
      // Within a few pixels: at boot the dock is resized once more after the layout applies (the
      // header settles) and dockview shares that out, so boot lands a little under the 320px asked for.
      await expect.poll(async () => Math.abs((await queryHeight()) - defaultHeight)).toBeLessThan(8);
      expect(await page.inputValue("textarea.query-text"), "the query box keeps what was typed").toBe('component.class(?c, "resistor") => ?c');

      // The default is what the browser keeps now, so a reload opens it rather than the old arrangement.
      await page.reload({ waitUntil: "networkidle" });
      await page.waitForSelector(".query textarea.query-text", { timeout: 30_000 });
      await expect.poll(() => hasTab("Rules")).toBe(1);
      await expect.poll(async () => Math.abs((await queryHeight()) - defaultHeight)).toBeLessThan(8);
    });
  }, 90_000);

  // agni issue 774. The relation picker drills the namespace inside the drawer, which sits off-screen
  // until opened, so only a browser shows that the definition a reader drills to is on screen.
  it("drills the relation picker to a library member's definition, on screen", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut", "designs/gateway/gateway.edn");
      await page.click(".query-drawer-handle");
      const chip = (label: string) => page.locator(".query-drawer .query-relchip", { hasText: label }).first();
      await chip("component ›").click({ timeout: 30_000 });
      await chip("component.probed_both").click({ timeout: 30_000 });
      const definition = page.locator(".query-drawer .query-reldefinition");
      await definition.waitFor({ timeout: 30_000 });
      expect(await definition.textContent(), "the clauses as the module writes them").toContain("probed_both(?r");
      const box = await definition.boundingBox();
      const view = page.viewportSize()!;
      expect(box, "the definition has a box").not.toBeNull();
      expect(box!.x).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width).toBeLessThanOrEqual(view.width + 1);
      expect(box!.y + box!.height).toBeGreaterThan(0);
      expect(box!.y).toBeLessThan(view.height);
      const painted = await definition.evaluate((el) => {
        const r = el.getBoundingClientRect();
        const hit = document.elementFromPoint(r.left + Math.min(20, r.width / 2), r.top + Math.min(8, r.height / 2));
        return !!hit && (el === hit || el.contains(hit));
      });
      expect(painted, "the definition is what is painted where it sits").toBe(true);
      await page.click(".query-drawer .query-relinsert");
      expect(await page.inputValue("textarea.query-text")).toContain("component.probed_both(?");
    });
  }, 90_000);

  it("gives Changes its rail width when a comparison adds it back", async () => {
    await withPage(browser, async (page) => {
      await page.goto(`${base()}/designs/tut/designs/gateway/gateway.edn/view`, { waitUntil: "networkidle" });
      await page.waitForSelector(".query textarea.query-text", { timeout: 30_000 });
      const tab = page.locator(".tab", { hasText: "Changes" });
      await tab.hover();
      await tab.locator(".dv-default-tab-action").click();
      await expect.poll(() => page.locator(".tab", { hasText: "Changes" }).count()).toBe(0);
      await page.click("#compare button");
      await page.locator("#compare-tree").getByText("gateway-rev-c.edn").click();
      await page.locator(".tab", { hasText: "Changes" }).waitFor({ timeout: 30_000 });
      await page.waitForTimeout(500);
      const width = await page.evaluate(() => {
        const tab = Array.from(document.querySelectorAll(".tab")).find((t) => t.textContent?.includes("Changes"));
        return tab?.closest(".groupview")?.getBoundingClientRect().width ?? 0;
      });
      expect(width, "the Changes column's width").toBeGreaterThan(220);
      expect(width).toBeLessThan(300);
    });
  }, 90_000);
});

// agni issue 1132: on a crowded bar the menus shrank until "▾" wrapped under its label, so Panels and
// Highlight stood a line taller than every other button.
describe("the top bar's menu buttons", () => {
  it("stay one line on a narrow window", async () => {
    await withPage(browser, async (page) => {
      await page.setViewportSize({ width: 700, height: 900 });
      await openViewer(page, "kicad", "hier_bus_root.kicad_sch");
      const heights = await page.evaluate(() =>
        [".panels-btn", ".highlight-btn", ".render-mode .mode-btn"].map((sel) => {
          const b = document.querySelector<HTMLElement>(sel);
          return { sel, h: b ? b.getBoundingClientRect().height : -1 };
        }),
      );
      expect(heights.every((x) => x.h > 0)).toBe(true);
      const one = heights[2].h;
      for (const x of heights.slice(0, 2)) expect(x.h, x.sel).toBeLessThanOrEqual(one + 1);
    });
  });
});

// agni issue 1133: the trace pickers' list sits inside the docked Trace panel, where a dropdown
// floating over the panel would be clipped by its edge, so the reader has to be able to see and click it.
describe("the trace pickers", () => {
  it("fill an end from Properties, list the part's pins where they can be clicked, and trace the pick", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut", "designs/gateway");
      const at = await page.evaluate(() => {
        const r = document.querySelector('#svg-view [data-kind="component"][data-ref="U1"]')!.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
      });
      await page.mouse.click(at.x, at.y);
      await page.click(".pr-trace-from", { timeout: 30_000 });

      const from = page.locator('input.trace-pin[aria-label="Start pin"]');
      await page.waitForFunction(() => document.querySelectorAll(".trace-pick.pin").length > 0, null, { timeout: 30_000 });
      expect(await from.inputValue()).toBe("U1.");
      expect((await paintedAt(page, ".trace-pick.pin")).ownedByProbe).toBe(true);

      const first = (await page.locator(".trace-pick.pin .trace-pick-value").first().innerText()).trim();
      await page.locator(".trace-pick.pin").first().click();
      expect(await from.inputValue()).toBe(first);

      const to = page.locator('input.trace-pin[aria-label="End pin"]');
      await to.fill("J1.");
      await to.press("ArrowDown");
      await to.press("Enter");
      expect(await to.inputValue()).toMatch(/^J1\.\S+$/);
      await page.click("button.trace-run");
      await page.waitForSelector(".trace-headline", { timeout: 30_000 });
    });
  }, 90_000);
});

// agni issue 1133: a rule group's docs link sits on the header's line, beside the count, where jsdom
// can say it exists and cannot say where it lands. The first version wrapped under the name, and the
// next was pushed to the row's far end by the toggle growing into the line.
describe("a rule group's docs link", () => {
  it("sits beside the group's count, on the same line", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut", "designs/gateway");
      await page.click(".checks-run:not([disabled])", { timeout: 30_000 });
      await page.waitForSelector("tr.check-row", { timeout: 60_000 });
      await page.selectOption(".checks-groupby select", "rule");
      const at = await page.evaluate(() => {
        const row = [...document.querySelectorAll("tr.check-group")].find((r) => r.querySelector(".check-group-doc"));
        if (!row) throw new Error("no rule group carries a docs link");
        const badge = row.querySelector(".finding-group-badge, .finding-group-unresolved")!.getBoundingClientRect();
        const link = row.querySelector(".check-group-doc")!.getBoundingClientRect();
        return { gap: link.left - badge.right, dy: Math.abs(link.top + link.height / 2 - (badge.top + badge.height / 2)) };
      });
      expect(at.dy).toBeLessThan(4);
      expect(at.gap).toBeGreaterThanOrEqual(0);
      expect(at.gap).toBeLessThan(24);
    });
  }, 120_000);
});

// agni issues 1141 and 1143: a KiCad design whose sub-sheets under two parents are both named "1"
// nests in the Sheets dock, and the two read as Power/1 and IO/1 rather than as 1 and 1.
describe("the Sheets dock on a hierarchical design", () => {
  it("nests each sheet under its parent and tells two sheets named 1 apart", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "kicad", "hier_numeric_root.kicad_sch");
      await page.waitForFunction(() => document.querySelectorAll(".sheet-tile-name").length === 5, null, { timeout: 30_000 });
      const rows = await page.evaluate(() =>
        [...document.querySelectorAll(".sheet-tile-name")].map((n) => ({ name: n.textContent, left: Math.round(n.getBoundingClientRect().left) })),
      );
      expect(rows.map((r) => r.name)).toEqual(["Hier numeric root", "Power", "Power/1", "IO", "IO/1"]);
      const left = Object.fromEntries(rows.map((r) => [r.name, r.left]));
      expect(left["Power"]).toBeGreaterThan(left["Hier numeric root"]);
      expect(left["Power/1"]).toBeGreaterThan(left["Power"]);
      expect(left["IO"]).toBe(left["Power"]);
      await page.locator(".sheet-tile", { hasText: "IO/1" }).click();
      await page.waitForFunction(() => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === "IO/1", null, { timeout: 30_000 });
    });
  }, 90_000);
});
