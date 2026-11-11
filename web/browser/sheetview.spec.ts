// A sheet view beside the canvas follows what the reader selects (agni issue 1326). jsdom has no
// layout and no elementFromPoint, so only a browser shows that a pick in one view lights and centres
// the same part in the other, in either direction, and that a view keeps following once it floats.

import { beforeAll, afterAll, describe, expect, inject, it } from "vitest";
import type { Browser, Page, Request } from "playwright-core";
import { fromBinary, toBinary, toJson, type DescMessage } from "@bufbuild/protobuf";
import { HighlightSheetRequestSchema, LocateSubjectsRequestSchema, LocateSubjectsResponseSchema } from "../src/gen/agni/v1/webapi/design_pb.js";
import { launch, withPage } from "./browser.js";
import { PANE, TITLE, centreOf, centredIn, clearPoint, firstClear, lit, openBeside, openViewer, picked } from "./picks.js";

// bodyOf is a request's body in its JSON form. The viewer speaks binary Connect (agni issue 1502), so
// a route reads the body against its method's schema.
const bodyOf = (req: Request, schema: DescMessage): string => {
  const buf = req.postDataBuffer();
  if (!buf) return "";
  if (!(req.headers()["content-type"] ?? "").startsWith("application/proto")) return buf.toString();
  return JSON.stringify(toJson(schema, fromBinary(schema, new Uint8Array(buf))));
};

const base = (): string => inject("baseUrl");

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 120_000);
afterAll(async () => {
  await browser?.close();
});

describe("a sheet view beside the canvas", () => {
  it("finds a schematic part on the board, and a board footprint on the schematic", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut/designs/gateway");
      expect(await page.locator(PANE).count()).toBe(0);
      await openBeside(page, "Board");
      await page.waitForSelector(`${PANE} .picks-front`, { timeout: 30_000 });

      // The board in the view to the schematic on the canvas. Both are fitted, and the part found on
      // the board is the first whose body has a point clear of its pads.
      const onBoard = await firstClear(page, `${PANE} .picks-front`);
      await page.mouse.click(onBoard.x, onBoard.y);
      await picked(page, onBoard.ref);
      await lit(page, "#svg-view");
      await page.waitForFunction(
        (ref) => {
          const e = document.querySelector(`#svg-view [data-kind="component"][data-ref="${ref}"]`)!.getBoundingClientRect();
          const h = document.querySelector("#svg-view")!.getBoundingClientRect();
          const cx = e.left + e.width / 2;
          return cx > h.left + h.width / 3 && cx < h.right - h.width / 3;
        },
        onBoard.ref,
        { timeout: 30_000 },
      );
      expect(await centredIn(page, `#svg-view [data-kind="component"][data-ref="${onBoard.ref}"]`, "#svg-view")).toBe(true);

      // The schematic on the canvas to the board in the view. The canvas has zoomed to the part, so it
      // is the part clicked again, and the board view, still fitted from before, has to move to it.
      const onSheet = { ref: onBoard.ref, ...(await clearPoint(page, `#svg-view [data-kind="component"][data-ref="${onBoard.ref}"]`, onBoard.ref)) };
      expect(await centredIn(page, `${PANE} .picks-front [data-ref="${onSheet.ref}"]`, `${PANE} .sheetview-canvas`)).toBe(false);
      await page.mouse.click(onSheet.x, onSheet.y);
      await picked(page, onSheet.ref);
      await lit(page, `${PANE} .sheetview-canvas`);
      await page.waitForFunction(
        ({ pane, ref }) => {
          const e = document.querySelector(`${pane} .picks-front [data-ref="${ref}"]`)!.getBoundingClientRect();
          const h = document.querySelector(`${pane} .sheetview-canvas`)!.getBoundingClientRect();
          const cx = e.left + e.width / 2;
          return cx > h.left + h.width / 3 && cx < h.right - h.width / 3;
        },
        { pane: PANE, ref: onSheet.ref },
        { timeout: 30_000 },
      );
      expect(await centredIn(page, `${PANE} .picks-front [data-ref="${onSheet.ref}"]`, `${PANE} .sheetview-canvas`)).toBe(true);

      // A pad picked on the board centres its pin's part on the schematic, with the pin on screen. A
      // pin's highlight is a point, and a point used to reveal nothing (agni issue 1326). Framing the
      // point alone left a part drawn at the end of a long leg off centre (agni issue 1419).
      const pad = `${PANE} [data-kind="pin"][data-ref="${onSheet.ref}"]`;
      const pin = await page.$eval(pad, (e) => e.getAttribute("data-pin")!);
      await page.mouse.click(...(Object.values(await centreOf(page, `${pad}[data-pin="${pin}"]`)) as [number, number]));
      await picked(page, `${onSheet.ref}.${pin}`);
      const pinOnSheet = `#svg-view [data-kind="pin"][data-ref="${onSheet.ref}"][data-pin="${pin}"]`;
      await page.waitForFunction(({ sel, ref }) => {
        const p = document.querySelector(sel)!.getBoundingClientRect();
        const h = document.querySelector("#svg-view")!.getBoundingClientRect();
        const px = p.left + p.width / 2, py = p.top + p.height / 2;
        if (px < h.left || px > h.right || py < h.top || py > h.bottom) return false;
        // The body nearest the pin is the unit that draws it.
        const bodies = [...document.querySelectorAll(`#svg-view [data-kind="component"][data-ref="${ref}"]`)].map((e) => e.getBoundingClientRect());
        const centre = (r: DOMRect) => ({ x: r.left + r.width / 2, y: r.top + r.height / 2 });
        const body = bodies.map(centre).sort((a, b) => Math.hypot(a.x - px, a.y - py) - Math.hypot(b.x - px, b.y - py))[0];
        return !!body && body.x > h.left + h.width / 3 && body.x < h.right - h.width / 3 && body.y > h.top + h.height / 3 && body.y < h.bottom - h.height / 3;
      }, { sel: pinOnSheet, ref: onSheet.ref }, { timeout: 30_000 });
    });
  }, 120_000);

  it("switches its sheet when its own does not draw the selection", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "kicad/hier_numeric_root.kicad_sch");
      await openBeside(page, "Hier numeric root");
      const root = await page.$eval(`${PANE} .sheetview-sheet`, (s) => (s as HTMLSelectElement).value);
      await page.locator(".sheet-tile", { hasText: /^IO$/ }).click();
      await page.waitForFunction(() => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === "IO", null, { timeout: 30_000 });
      await page.waitForTimeout(500);
      const ref = await page.$eval('#svg-view [data-kind="component"][data-ref]', (e) => e.getAttribute("data-ref")!);
      await page.mouse.click(...Object.values(await centreOf(page, `#svg-view [data-kind="component"][data-ref="${ref}"]`)) as [number, number]);
      await page.waitForFunction(
        ({ pane, root }) => (document.querySelector(`${pane} .sheetview-sheet`) as HTMLSelectElement).value !== root,
        { pane: PANE, root },
        { timeout: 30_000 },
      );
      await lit(page, `${PANE} .sheetview-canvas`);
    });
  }, 120_000);

  it("keeps following once it floats, and stays put when its link is off", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut/designs/gateway");
      await openBeside(page, "Board");
      await page.locator(`${PANE} .sheetview-float`).click();
      await page.waitForFunction((pane) => document.querySelector(pane)?.closest(".dv-render-overlay-float") !== null, PANE, { timeout: 30_000 });
      const first = await firstClear(page, "#svg-view");
      await page.mouse.click(first.x, first.y);
      await picked(page, first.ref);
      await lit(page, `${PANE} .sheetview-canvas`);

      await page.selectOption(`${PANE} .sheetview-link`, "off");
      const before = await page.$eval(`${PANE} .sheetview-canvas > div`, (d) => (d as HTMLElement).style.transform);
      const second = await firstClear(page, "#svg-view", [first.ref]);
      await page.mouse.click(second.x, second.y);
      await picked(page, second.ref);
      await page.waitForTimeout(1500);
      expect(await page.$eval(`${PANE} .sheetview-canvas > div`, (d) => (d as HTMLElement).style.transform)).toBe(before);
    });
  }, 120_000);

  // A board view keeps the board when the board does not place the part (agni issue 1481). Every part
  // of the gateway is on its board, so the page's LocateSubjects answer for one part leaves the board
  // out, as the engine answers for a DNP part or one renumbered after layout.
  const withoutBoard = async (page: Page, ref: string) => {
    await page.route("**/LocateSubjects", async (route) => {
      const body = bodyOf(route.request(), LocateSubjectsRequestSchema);
      if (!body.includes(`"${ref}"`)) return route.continue();
      const resp = await route.fetch();
      const answer = fromBinary(LocateSubjectsResponseSchema, new Uint8Array(await resp.body()));
      for (const s of answer.subjects) s.sheetIds = s.sheetIds.filter((id) => id !== "board");
      await route.fulfill({ response: resp, body: Buffer.from(toBinary(LocateSubjectsResponseSchema, answer)) });
    });
  };

  it("stays on the board when the board does not place the part, and says so", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut/designs/gateway");
      await openBeside(page, "Board");
      const absent = await firstClear(page, "#svg-view");
      await withoutBoard(page, absent.ref);
      await page.mouse.click(absent.x, absent.y);
      await picked(page, absent.ref);
      await page.waitForFunction(
        ({ pane, want }) => document.querySelector(`${pane} .sheetview-absent`)?.textContent === want,
        { pane: PANE, want: `${absent.ref} is not on the board` },
        { timeout: 30_000 },
      );
      expect(await page.$eval(`${PANE} .sheetview-sheet`, (s) => (s as HTMLSelectElement).value)).toBe("board");
      expect(await page.$$eval(`${PANE} .highlight-overlay svg *`, (els) => els.length)).toBe(0);

      // A part the board places still centres there, and the note goes.
      const placed = await firstClear(page, "#svg-view", [absent.ref]);
      await page.mouse.click(placed.x, placed.y);
      await picked(page, placed.ref);
      await lit(page, `${PANE} .sheetview-canvas`);
      expect(await page.$eval(`${PANE} .sheetview-sheet`, (s) => (s as HTMLSelectElement).value)).toBe("board");
      expect(await page.$$eval(`${PANE} .sheetview-absent`, (els) => els.length)).toBe(0);
    });
  }, 120_000);

  it("keeps the board on the canvas when a part picked beside it is not on the board", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut/designs/gateway");
      const schematic = (await page.locator(".sheet-tile.active .sheet-tile-name").textContent()) ?? "";
      await openBeside(page, schematic);
      await page.locator(".sheet-tile", { hasText: /^Board$/ }).click();
      await page.waitForFunction(() => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent === "Board", null, { timeout: 30_000 });
      await page.waitForSelector("#svg-view .picks-front", { timeout: 30_000 });
      const absent = await firstClear(page, `${PANE} .sheetview-canvas`);
      await withoutBoard(page, absent.ref);
      await page.mouse.click(absent.x, absent.y);
      await picked(page, absent.ref);
      await page.waitForFunction((want) => document.querySelector("#board-absent-note")?.textContent === want, `${absent.ref} is not on the board`, { timeout: 30_000 });
      expect(await page.locator(".sheet-tile.active .sheet-tile-name").textContent()).toBe("Board");
    });
  }, 120_000);

  // A walk lights the board beside the schematic, and a walk row moves the board view (agni issue 1480).
  it("lights a walk on the board beside the canvas, and a walk row moves the board there", async () => {
    await withPage(browser, async (page) => {
      const boardSpecs: string[] = [];
      await page.route("**/HighlightSheet", (route) => {
        const body = bodyOf(route.request(), HighlightSheetRequestSchema);
        if (body.includes('"sheet":"board"')) boardSpecs.push(body);
        return route.continue();
      });
      await page.goto(`${base()}/designs/tut/designs/gateway/view?mode=svg&walk=R1&walk_kind=small-parts&walk_hops=2`, { waitUntil: "networkidle" });
      await page.waitForSelector(".walk-net-name", { timeout: 30_000 });
      const net = (await page.locator(".walk-net").first().getAttribute("data-walk-net"))!;
      await openBeside(page, "Board");
      await lit(page, `${PANE} .sheetview-canvas`);
      await expect.poll(() => boardSpecs.some((b) => b.includes(`"${net}"`)), { timeout: 30_000 }).toBe(true);

      // A part row selects the part, and the board view centres its footprint.
      const part = page.locator(".walk-net .walk-part").first();
      const ref = (await part.textContent())!.trim().split(/[\s.]/)[0];
      await part.click();
      await picked(page, ref);
      await page.waitForFunction(
        ({ pane, ref }) => {
          const e = document.querySelector(`${pane} .picks-front [data-ref="${ref}"]`)?.getBoundingClientRect();
          const h = document.querySelector(`${pane} .sheetview-canvas`)!.getBoundingClientRect();
          if (!e) return false;
          const cx = e.left + e.width / 2;
          return cx > h.left + h.width / 3 && cx < h.right - h.width / 3;
        },
        { pane: PANE, ref },
        { timeout: 30_000 },
      );

      // A net row lights that net on the board, over the walk.
      boardSpecs.length = 0;
      const other = page.locator(".walk-net").nth(1);
      const otherNet = (await other.getAttribute("data-walk-net"))!;
      await other.locator(".walk-net-name").click();
      await picked(page, otherNet);
      await expect.poll(() => boardSpecs.some((b) => b.includes(`"${otherNet}"`) && b.includes(`"${net}"`)), { timeout: 30_000 }).toBe(true);
    });
  }, 120_000);

  // An idle SVG board shows an image of itself, so it adds nothing to the canvas's pan (agni issue
  // 1482). A pick, a highlight and a hidden layer still work on it.
  it("shows an image of an idle board, and a pick, a highlight and a hidden layer still work", async () => {
    await withPage(browser, async (page) => {
      const parkedSrc = () => page.$eval(`${PANE} img.svg-parked`, (i) => (i as HTMLImageElement).src).catch(() => "");
      const away = async () => {
        const b = (await page.locator("#svg-view").boundingBox())!;
        await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2);
      };
      const parks = () =>
        page.waitForFunction(
          (pane) => {
            const img = document.querySelector(`${pane} img.svg-parked`) as HTMLImageElement | null;
            const svg = document.querySelector(`${pane} img.svg-parked + svg`) as SVGElement | null;
            return !!img && img.complete && img.naturalWidth > 0 && svg?.style.display === "none";
          },
          PANE,
          { timeout: 30_000 },
        );
      await openViewer(page, "tut/designs/gateway");
      await openBeside(page, "Board");
      await away();
      await parks();
      const first = await parkedSrc();

      // A pick on the schematic lights the part on the parked board.
      const part = await firstClear(page, "#svg-view");
      await page.mouse.click(part.x, part.y);
      await picked(page, part.ref);
      await lit(page, `${PANE} .sheetview-canvas`);
      await parks();
      expect(await parkedSrc()).toBe(first);

      // The pointer over the board brings the drawing back, and a click there picks.
      const box = (await page.locator(`${PANE} .sheetview-canvas`).boundingBox())!;
      await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
      await page.waitForFunction((pane) => !document.querySelector(`${pane} img.svg-parked`), PANE, { timeout: 30_000 });
      const pad = `${PANE} [data-kind="pin"][data-ref="${part.ref}"]`;
      const pin = await page.$eval(pad, (e) => e.getAttribute("data-pin")!);
      await page.mouse.click(...(Object.values(await centreOf(page, `${pad}[data-pin="${pin}"]`)) as [number, number]));
      await picked(page, `${part.ref}.${pin}`);

      // A hidden layer makes a new image once the board is idle again.
      await page.locator(`${PANE} [data-action="layers"]`).click();
      await page.locator(`${PANE} .board-layer-row`, { has: page.locator('input[aria-label="F.Cu"]') }).click();
      await page.locator(`${PANE} [data-action="layers"]`).click();
      await away();
      await parks();
      expect(await parkedSrc()).not.toBe(first);
    });
  }, 120_000);
});

// A view draws the board in WebGL when its reader picks WebGL (agni issue 1368). Only a browser has
// WebGL2, and only a browser shows that the drawing lights and centres a part picked on the canvas,
// that a click on it picks that part, and that turning the board over keeps the part under the click.
describe("a sheet view in WebGL", () => {
  const GL = `${PANE} .sheetview-gl`;
  // litInView is what the view's WebGL drawing lit (canvas.ts, data-lit).
  const litInView = (page: Page) => page.$eval(GL, (c) => JSON.parse((c as HTMLElement).dataset.lit ?? "[]") as string[]);
  // clickCentre clicks the middle of the view's drawing.
  const clickCentre = async (page: Page) => {
    const b = (await page.locator(GL).boundingBox())!;
    await page.mouse.click(Math.round(b.x + b.width / 2), Math.round(b.y + b.height / 2));
  };
  const pickedPart = (page: Page) => page.$eval(TITLE, (t) => t.textContent ?? "");

  it("lights and centres a part picked on the canvas, and a click there picks it, from above and below", async () => {
    await withPage(browser, async (page) => {
      await openViewer(page, "tut/designs/gateway");
      await openBeside(page, "Board");
      await page.check(`${PANE} .sheetview-webgl`);
      await page.waitForSelector(`${PANE}[data-renderer="webgl"]`, { timeout: 30_000 });

      const part = await firstClear(page, "#svg-view");
      await page.mouse.click(part.x, part.y);
      await picked(page, part.ref);
      await expect.poll(() => litInView(page), { timeout: 30_000 }).toContain(`component:${part.ref}`);

      // The view centred the part, so a click in its middle lands on the part or one of its pads.
      await page.waitForTimeout(300);
      await clickCentre(page);
      await expect.poll(() => pickedPart(page), { timeout: 30_000 }).toMatch(new RegExp(`^${part.ref}(\\.|$)`));

      // From below the board turns about the view's middle, so the part is still under it.
      await page.locator(`${PANE} [data-action="flip-h"]`).click();
      await page.waitForSelector(`${PANE}[data-side="back"]`, { timeout: 10_000 });
      await page.waitForTimeout(300);
      await clickCentre(page);
      await expect.poll(() => pickedPart(page), { timeout: 30_000 }).toMatch(new RegExp(`^${part.ref}(\\.|$)`));
    });
  }, 120_000);
});
