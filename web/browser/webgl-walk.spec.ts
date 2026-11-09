// The WebGL walk, mission #1329's exercise (agni issue 1330). An engineer opens Jetson's board and
// its densest schematic sheet, pans and zooms, clicks things, drags a box, and follows a finding and
// a trace. The walk does each in SVG and then in WebGL and compares the two.
//
// 1. Timings. Each mode opens each sheet from its Sheets tile and times the click to the
//    SHEET_DRAWN mark (src/drawmark.ts). Then it runs a fixed pan and zoom script, one input event
//    per animation frame, and summarizes the requestAnimationFrame deltas. The clock is the page's
//    performance.now. A last row asks whether WebGL beat SVG on the board. It is judged only headed,
//    because headless Chrome draws WebGL in software (SwiftShader), and there it reads "not
//    available" rather than a verdict either way.
// 2. Clicks. A footprint, a pad and a track on the board, a pin and a bus on the schematic. The walk
//    finds and clicks each in SVG, then clicks the same place in WebGL. "The same place" comes from
//    the labels both renderers draw. Each fits the sheet to the view on its own, so the walk fits a
//    scale and offset between their label anchors (frametimes.ts) and carries the SVG point
//    through it. A zoom replays as the same wheel steps at the mapped point, which keeps that fit
//    valid, since both views zoom by panzoom's one wheelZoomFactor.
// 3. A Shift-drag box, mapped the same way.
// 4. A verdict link and a trace link, opened in each mode. The canvas writes the subjects it was asked
//    to light to data-highlight in every mode and those it lit to data-lit in WebGL (canvas.ts). The
//    walk reads the SVG side off the keyed elements of the drawn sheet, matched as
//    resolveHighlights matches packed keys, and compares the two sets.
//
// A Selection is compared as the Properties panel shows it, its kind, its title and its whole text,
// since a reader sees exactly that. A difference no panel shows is out of reach, and #1436 is
// waiting for a reason to compare Selection values field by field.
//
// Steps that wait on a ticket read "not available" until it lands (exercise-report.ts, waitedOn),
// and a waiting step that passes fails the walk, so the PR that makes it pass removes its wait.
// `make exercise-webgl` passes CLOSED_TICKETS as the other walks do.
//
// The gate walks the cross-probe cut of Jetson (examples/cross-probe-project). `make exercise-webgl`
// walks the whole board (WALK_DESIGN=jetson, examples/jetson-project), headed unless HEADED=0, and
// writes the table to EXERCISE_OUT/webgl.md.

import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import type { Browser, Page } from "playwright-core";
import { mkdirSync, writeFileSync } from "node:fs";
import { loadavg } from "node:os";
import { join } from "node:path";
import { headed, launch, viewportFor } from "./browser.js";
import { exitCode, table, waitedOn, type Row } from "./exercise-report.js";
import { fitFrame, mapPoint, summarize, zoomFrame, type Frame, type FrameSummary, type Point } from "./frametimes.js";
import { wheelZoomFactor } from "@agni/web-shared/panzoom.js";
import { TITLE, clearPoint } from "./picks.js";

const base = (): string => inject("baseUrl");

// Each walk names its densest schematic sheet, which the "densest sheet" step checks, plus a verdict
// whose link lights something on a sheet and a trace through one resistor. The verdict fails on the
// cut and passes on Jetson, and either way its link lights its subjects.
const walks = {
  cut: {
    mount: "probe",
    path: "designs/jetson-cut",
    about: "the cross-probe cut of Jetson, nineteen parts and their copper",
    sheet: "Power",
    // busSheet is a sheet that draws a bus, for the bus click. The cut's Power sheet draws none.
    busSheet: "USB Debug, PD",
    ground: "GND",
    verdict: { id: "single-pin-net:(net:/USB Debug%2C PD/UART3_DEBUG.RX)", rule: "single-pin-net", subject: "net:/USB Debug, PD/UART3_DEBUG.RX" },
    trace: "R119.1,R119.2",
  },
  jetson: {
    mount: "jetson",
    path: "designs/jetson",
    about: "Antmicro's open-source baseboard for the Jetson AGX Thor module",
    sheet: "Expansion connector",
    busSheet: "Expansion connector",
    ground: "GND",
    verdict: { id: "bulk-cap:(net:/Power/USBPD1_HV)", rule: "bulk-cap", subject: "net:/Power/USBPD1_HV" },
    trace: "R119.1,R119.2",
  },
};
type Walk = (typeof walks)[keyof typeof walks];
// WALK_TIMEOUT_MS bounds a walk. Headless SwiftShader on a loaded machine spends minutes on each of
// Jetson's WebGL board steps, and its walk ran past half an hour once the board view beside the
// schematic joined it (agni issue 1435).
const WALK_TIMEOUT_MS: Record<keyof typeof walks, number> = { cut: 1_800_000, jetson: 3_600_000 };
const chosen = process.env.WALK_DESIGN ? [process.env.WALK_DESIGN] : ["cut"];
for (const w of chosen) {
  if (!(w in walks)) throw new Error(`WALK_DESIGN=${w} names no walk; use one of ${Object.keys(walks).join(", ")}`);
}

type Mode = "svg" | "webgl";
const MODES: Mode[] = ["svg", "webgl"];
const SVG = "#svg-view";
const CANVAS = "#view";
const PROPS = '[data-component="properties"]';
const DRAWN = "agni:sheet-drawn";
// The walk trusts a fit over at least MIN_ANCHORS labels whose median distance from their twins is
// at most MAX_RESIDUAL_PX, and otherwise says so rather than clicking somewhere else. On a board the
// anchors agree to a tenth of a pixel. On a schematic each renderer places a justified label a few
// pixels apart (4.5 px median on the cut's Power sheet), and a fit over dozens of them still places
// the frame far closer than that.
const MAX_RESIDUAL_PX = 8;
const MIN_ANCHORS = 10;

// Unwalkable is a failure of the walk's own SVG half, such as no part to click or two views that do
// not line up. It fails the step even when the step waits on a ticket, since that ticket would not
// fix it.
class Unwalkable extends Error {}
// NotJudged leaves a step "not available" whatever it waits on, for a run that cannot judge it.
class NotJudged extends Error {}

// VIDEO names a directory to record the walk into, one .webm per design, for a demo. A recording
// run judges no timing, since the encoder competes with the page for the CPU.
const video = process.env.VIDEO ?? "";

const closed = new Set((process.env.CLOSED_TICKETS ?? "").split(",").filter((t) => t !== "").map((t) => `#${t.replace(/^#/, "")}`));

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 180_000);
afterAll(async () => {
  await browser?.close();
});

// sheetParam is a sheet's id in a view URL. The board's is "board", and a schematic sheet's its path.
const sheetParam = (name: string): string => (name === "Board" ? "board" : `/${name}`);

// open loads a design in one mode on one sheet and waits for it to be drawn.
async function open(page: Page, walk: Walk, mode: Mode, sheet: string, extra = ""): Promise<void> {
  const q = `mode=${mode}&sheet=${encodeURIComponent(sheetParam(sheet))}${extra}`;
  await page.goto(`${base()}/designs/${walk.mount}/${walk.path}/view?${q}`, { waitUntil: "networkidle", timeout: 180_000 });
  await drawnSince(page, mode, 0);
  await idle(page);
}

// idle waits for the page to stop loading. The sheet draws before the design's rules, report and
// parts load, and until they do the busy overlay lies over the drawing and takes every click and
// wheel. On a GPU that is over before a hand reaches the mouse. Under SwiftShader on Jetson it lasted
// seconds, and the walk's first wheel steps went to the overlay (agni issue 1336).
async function idle(page: Page): Promise<void> {
  await page.waitForFunction(() => !document.getElementById("render-busy")?.dataset.busy, null, { timeout: 180_000, polling: 50 });
}

// drawnSince waits for a SHEET_DRAWN mark from `mode` after `since` on the page's clock, and returns
// its time. view names the drawing, the canvas or a sheet view's id, since each marks its own draws.
async function drawnSince(page: Page, mode: Mode, since: number, view = "canvas"): Promise<number> {
  const h = await page.waitForFunction(
    ({ name, mode, since, view }) =>
      performance
        .getEntriesByName(name)
        .map((e) => e as PerformanceMark)
        .find((e) => {
          const d = e.detail as { mode?: string; view?: string };
          return d?.mode === mode && (d.view ?? "canvas") === view && e.startTime > since;
        })?.startTime ?? false,
    { name: DRAWN, mode, since, view },
    { timeout: 180_000, polling: 50 },
  );
  return (await h.jsonValue()) as number;
}

// tile clicks a Sheets tile by its exact name.
async function tile(page: Page, name: string): Promise<void> {
  const tiles = page.locator(".sheet-tile");
  for (let i = 0; i < (await tiles.count()); i++) {
    if ((await tiles.nth(i).locator(".sheet-tile-name").innerText()).trim() === name) {
      await tiles.nth(i).click();
      return;
    }
  }
  throw new Error(`no Sheets tile is named ${name}`);
}

// host is the rect the drawing fills in client pixels, which is the same in both modes since the SVG
// host lies over the canvas.
// within names a sheet view's stage instead, for a step that pans a view beside the canvas.
async function host(page: Page, mode: Mode, within = ""): Promise<{ x: number; y: number; width: number; height: number }> {
  const b = await page.locator(within || (mode === "svg" ? SVG : CANVAS)).boundingBox();
  if (!b || b.width < 50 || b.height < 50) throw new Error(`the ${mode} view has no room to draw in`);
  return b;
}

// nextFrame waits for the page's next animation frame, so the walk sends one input event per frame
// as a hand on a mouse would.
const nextFrame = (page: Page): Promise<unknown> => page.evaluate(() => new Promise((r) => requestAnimationFrame(() => r(null))));

// panZoom runs the fixed script over the middle of the view and returns its frame times. The
// script is 40 wheel steps in, 40 out, and a drag 600 px across and back in 60 moves.
async function panZoom(page: Page, mode: Mode, within = ""): Promise<FrameSummary> {
  const h = await host(page, mode, within);
  const cx = h.x + h.width / 2, cy = h.y + h.height / 2;
  await page.mouse.move(cx, cy);
  await page.evaluate(() => {
    const w = window as unknown as { __frames: number[]; __sampling: boolean };
    w.__frames = [];
    w.__sampling = true;
    let last = performance.now();
    const tick = (t: number) => {
      w.__frames.push(t - last);
      last = t;
      if (w.__sampling) requestAnimationFrame(tick);
    };
    requestAnimationFrame((t) => {
      last = t;
      requestAnimationFrame(tick);
    });
  });
  for (const dy of [-120, 120]) {
    for (let i = 0; i < 40; i++) {
      await page.mouse.wheel(0, dy);
      await nextFrame(page);
    }
  }
  await page.mouse.move(cx - 300, cy);
  await page.mouse.down();
  for (const i of [...Array(30).keys()].map((k) => k + 1).concat([...Array(30).keys()].reverse())) {
    await page.mouse.move(cx - 300 + i * 20, cy);
    await nextFrame(page);
  }
  await page.mouse.up();
  await nextFrame(page);
  const frames = await page.evaluate(() => {
    const w = window as unknown as { __frames: number[]; __sampling: boolean };
    w.__sampling = false;
    return w.__frames;
  });
  return summarize(frames);
}

// anchors reads every label drawn once in a mode, as the client point of its text anchor.
async function anchors(page: Page, mode: Mode): Promise<Map<string, Point>> {
  const entries = await page.evaluate((sel) => {
    const seen = new Map<string, [number, number] | null>();
    const view = document.querySelector(sel.view)!.getBoundingClientRect();
    // The SVG host holds the drawn sheet's <svg> and the highlight overlay's, and only the first is
    // the sheet.
    const root = sel.svg
      ? [...document.querySelectorAll(`${sel.view} svg`)].find((s) => !s.closest(".highlight-overlay"))
      : document.querySelector("#text-overlay");
    for (const t of root?.querySelectorAll<SVGTextElement>("text") ?? []) {
      const s = (t.textContent ?? "").trim();
      const ctm = t.getScreenCTM();
      if (!s || !ctm || t.getBoundingClientRect().width === 0) continue;
      const x = t.x.baseVal.numberOfItems ? t.x.baseVal.getItem(0).value : 0;
      const y = t.y.baseVal.numberOfItems ? t.y.baseVal.getItem(0).value : 0;
      const px = ctm.a * x + ctm.c * y + ctm.e, py = ctm.b * x + ctm.d * y + ctm.f;
      if (px < view.left || px > view.right || py < view.top || py > view.bottom) continue;
      seen.set(s, seen.has(s) ? null : [px, py]);
    }
    return [...seen].filter((e): e is [string, [number, number]] => e[1] !== null);
  }, mode === "svg" ? { view: SVG, svg: true } : { view: CANVAS, svg: false });
  return new Map(entries);
}

// frameBetween fits the frame that carries an SVG client point to the WebGL one showing the same
// place.
function frameBetween(svg: Map<string, Point>, gl: Map<string, Point>): Frame {
  const pairs = [...svg].filter(([k]) => gl.has(k)).map(([k, p]): [Point, Point] => [p, gl.get(k)!]);
  const f = fitFrame(pairs);
  if (!f || pairs.length < MIN_ANCHORS) throw new Unwalkable(`the two modes share ${pairs.length} labels, too few to line their views up`);
  if (f.residual > MAX_RESIDUAL_PX) throw new Unwalkable(`the two modes' views disagree by ${f.residual.toFixed(1)} px after the fit`);
  return f;
}

// refit lines up two zoomed views, or answers null when they share too few labels to.
function refit(svg: Map<string, Point>, gl: Map<string, Point>): Frame | null {
  try {
    return frameBetween(svg, gl);
  } catch {
    return null;
  }
}

interface Shown {
  kind: string;
  title: string;
  text: string;
}

// shown reads the Selection as Properties shows it, waiting up to `ms` for a title to appear.
// Some of the panel's rows, such as the checks a part has, arrive after its title. So shown waits
// for the text to hold still across SETTLE_MS too, or two clicks on one part read as two answers.
async function shown(page: Page, ms: number): Promise<Shown> {
  await page.waitForFunction((sel) => (document.querySelector(sel)?.textContent ?? "") !== "", TITLE, { timeout: ms }).catch(() => {});
  const until = Date.now() + ms;
  let last = await read(page);
  for (;;) {
    await page.waitForTimeout(SETTLE_MS);
    const now = await read(page);
    if (same(now, last) || Date.now() > until) return now;
    last = now;
  }
}

const SETTLE_MS = 300;

async function read(page: Page): Promise<Shown> {
  return page.evaluate((p) => {
    const panel = document.querySelector(p);
    // With nothing selected, the panel describes the design or the sheet instead.
    const kind = panel?.querySelector(".pr-kind")?.textContent ?? "";
    const title = kind === "design" || kind === "sheet" ? "" : (panel?.querySelector(".pr-title")?.textContent ?? "");
    return { kind: title ? (panel?.querySelector(".pr-kind")?.textContent ?? "") : "", title, text: title ? ((panel as HTMLElement | null)?.innerText ?? "") : "" };
  }, PROPS);
}

const said = (s: Shown): string => (s.title ? `${s.kind} ${s.title}` : "nothing");

// apart names what two answers disagree on, the pick or else the first Properties line that differs.
function apart(a: Shown, b: Shown): string {
  if (a.kind === "area" && b.kind === "area" && a.title !== b.title) {
    // Two areas of different sizes differ by members, so name the lines one lists and the other lacks.
    const la = new Set(a.text.split("\n")), lb = new Set(b.text.split("\n"));
    const only = (x: Set<string>, y: Set<string>) => [...x].filter((l) => !y.has(l)).slice(0, 8).map((l) => JSON.stringify(l)).join(", ");
    return `SVG picked ${said(a)}, WebGL ${said(b)}. Only SVG lists ${only(la, lb) || "nothing"}, only WebGL lists ${only(lb, la) || "nothing"}`;
  }
  if (a.kind !== b.kind || a.title !== b.title) return `SVG picked ${said(a)}, WebGL ${said(b)}`;
  const la = a.text.split("\n"), lb = b.text.split("\n");
  const i = la.findIndex((l, j) => l !== lb[j]);
  return `both picked ${said(a)}, and Properties differs at line ${i + 1}, ${JSON.stringify(la[i] ?? "")} in SVG and ${JSON.stringify(lb[i] ?? "")} in WebGL`;
}
const same = (a: Shown, b: Shown): boolean => a.kind === b.kind && a.title === b.title && a.text === b.text;

// A Target is something on a sheet to click, the elements matching `sel` and the Properties kind a
// click on one shows. A line is clicked at a point on its stroke. A `clear` target is a part,
// clicked at a point clear of its own pads (picks.ts, clearPoint). Anything else is clicked at its
// middle. A target `absent` from a design reads "not available" on it rather than failing.
interface Target {
  name: string;
  sheet: string;
  sel: string;
  kind: string;
  line?: boolean;
  clear?: boolean;
  absent?: boolean;
  waitsOn: string[];
}

// candidates lists a target's points in the fitted SVG view, nearest the middle first, each with the
// ref-des of the part it belongs to, if any.
async function candidates(page: Page, t: Target, limit = 6): Promise<{ at: Point; ref: string }[]> {
  return page.evaluate(({ sel, line, view, limit }) => {
    const v = document.querySelector(view)!.getBoundingClientRect();
    const mid = [v.left + v.width / 2, v.top + v.height / 2];
    const pts: { at: [number, number]; ref: string }[] = [];
    for (const el of document.querySelectorAll(sel)) {
      let p: [number, number] | null = null;
      if (line) {
        const geo = (el instanceof SVGGeometryElement ? el : el.querySelector("path, line, polyline")) as SVGGeometryElement | null;
        const ctm = geo?.getScreenCTM();
        if (geo && ctm) {
          const at = geo.getPointAtLength(geo.getTotalLength() / 2);
          p = [ctm.a * at.x + ctm.c * at.y + ctm.e, ctm.b * at.x + ctm.d * at.y + ctm.f];
        }
      } else {
        const r = el.getBoundingClientRect();
        if (r.width > 0 || r.height > 0) p = [r.left + r.width / 2, r.top + r.height / 2];
      }
      if (p && p[0] > v.left + 20 && p[0] < v.right - 20 && p[1] > v.top + 20 && p[1] < v.bottom - 20) pts.push({ at: p, ref: el.getAttribute("data-ref") ?? "" });
    }
    const far = (p: { at: [number, number] }) => Math.hypot(p.at[0] - mid[0], p.at[1] - mid[1]);
    return pts.sort((a, b) => far(a) - far(b)).slice(0, limit);
  }, { sel: t.sel, line: t.line ?? false, view: SVG, limit });
}

// zoomIn zooms the view on a point with `steps` wheel steps, a frame apart, and returns the point it
// zoomed about. A wheel event's client position is a whole pixel, so the walk rounds the point
// itself and knows the pivot exactly (zoomFrame).
async function zoomIn(page: Page, [x, y]: Point, steps: number): Promise<Point> {
  const pivot: Point = [Math.round(x), Math.round(y)];
  await page.mouse.move(pivot[0], pivot[1]);
  for (let i = 0; i < steps; i++) {
    await page.mouse.wheel(0, WHEEL_DELTA);
    await nextFrame(page);
  }
  await page.waitForTimeout(150);
  return pivot;
}

// WHEEL_DELTA is one zoom step, and zoomBy is how far `steps` of them zoom either view.
const WHEEL_DELTA = -400;
const zoomBy = (steps: number): number => wheelZoomFactor(WHEEL_DELTA) ** steps;

// ZOOM is how many wheel steps the walk zooms a click target in by, per sheet, so a small part on a
// whole board has room around it. A fitted cut's sheet needs none. The cut's board takes 4, so a
// SOT-23 has a point a pixel clear of its pads (CLEAR_MARGIN_PX).
const ZOOM = { cut: { Board: 4, sheet: 0 }, jetson: { Board: 6, sheet: 3 } };

// ZOOM_SLACK is how far the scale between the two views may move across the same wheel steps. Each
// renderer fits a sheet on its own, so the scales differ by a few percent at fit. Both zoom by
// panzoom's wheelZoomFactor, so a ratio that moves means one view missed a step, and a click carried
// across would land on a neighbour.
const ZOOM_SLACK = 0.02;

// CLEAR_MARGIN_PX keeps a footprint click that far clear of the part's pads and neighbours, since a
// pad's edge can sit half a pixel apart in the two renderers. On the cut SVG's probe missed Q2's pad
// by 0.4 px and WebGL's landed on it.
const CLEAR_MARGIN_PX = 1;

// clickBoth clicks a target in SVG, then the same place in WebGL, and returns what each showed.
// Both views zoom in on the target by the same wheel steps first, about the same place.
async function clickBoth(page: Page, walk: Walk, name: string, t: Target): Promise<{ svg: Shown; gl: Shown }> {
  const zoom = ZOOM[name as keyof typeof ZOOM][t.sheet === "Board" ? "Board" : "sheet"];
  await open(page, walk, "svg", t.sheet);
  const svgFit = await anchors(page, "svg");
  const found = await candidates(page, t);
  if (found.length === 0) {
    const why = `${t.sheet} draws no ${t.name} the walk can click`;
    throw t.absent ? new Error(why) : new Unwalkable(why);
  }
  const tried: string[] = [];
  for (const { at, ref } of found) {
    await open(page, walk, "svg", t.sheet);
    const svgPivot = await zoomIn(page, at, zoom);
    const svgZoomed = await anchors(page, "svg");
    const click: Point = t.clear ? await clearPoint(page, `${t.sel}[data-ref="${ref}"]`, ref, "component", CLEAR_MARGIN_PX).then((p) => [p.x, p.y] as Point).catch(() => at) : at;
    await page.mouse.click(click[0], click[1]);
    const svg = await shown(page, 10_000);
    if (svg.kind !== t.kind) {
      tried.push(said(svg));
      continue;
    }
    await open(page, walk, "webgl", t.sheet);
    const frame = frameBetween(svgFit, await anchors(page, "webgl"));
    const glPivot = await zoomIn(page, mapPoint(frame, svgPivot), zoom);
    // The views line up to a hundredth of a pixel at fit. A wheel event carries whole pixels, so each
    // zooms about a point up to half a pixel from the other's, and the zoom grows that to several
    // pixels away from the pivot. So the walk lines the zoomed views up again from the labels they
    // show, and where they share too few, carries the fit through the zoom (zoomFrame).
    const refitted = refit(svgZoomed, await anchors(page, "webgl"));
    if (refitted && Math.abs(refitted.kx / frame.kx - 1) > ZOOM_SLACK) throw new Unwalkable(`WebGL zoomed ${(refitted.kx / frame.kx).toFixed(2)} times as far as SVG on ${t.sheet}, so a click would land elsewhere`);
    const there = mapPoint(refitted ?? zoomFrame(frame, svgPivot, glPivot, zoomBy(zoom)), click);
    await page.mouse.click(there[0], there[1]);
    return { svg, gl: await shown(page, 5_000) };
  }
  throw new Unwalkable(`no ${t.name} clicked as a ${t.kind} in SVG (got ${tried.join(", ")})`);
}

// EDGE_PX is how far every keyed element's edge must lie from a box's sides. The walk carries the
// box into WebGL through a fitted frame, which is good to a pixel or so, and an element whose edge
// lies on a side could fall inside in one view and outside in the other.
const EDGE_PX = 3;

// clearBox returns a box around `ref` on the SVG view, with between 20 and 80 px to spare, that no
// keyed element's edge lies within EDGE_PX of.
async function clearBox(page: Page, ref: string): Promise<[Point, Point]> {
  const found = await page.evaluate(({ view, ref, edge }) => {
    const part = [...document.querySelectorAll(`${view} [data-kind="component"][data-ref="${ref}"]`)].map((el) => el.getBoundingClientRect());
    if (part.length === 0) return null;
    const left = Math.min(...part.map((r) => r.left)), top = Math.min(...part.map((r) => r.top));
    const right = Math.max(...part.map((r) => r.right)), bottom = Math.max(...part.map((r) => r.bottom));
    const keyed = [...document.querySelectorAll(`${view} [data-kind]`)].flatMap((el) => (el.tagName.toLowerCase() === "g" ? [...el.children] : [el])).map((el) => el.getBoundingClientRect());
    for (let m = 20; m <= 80; m += 2) {
      const b = { left: left - m, top: top - m, right: right + m, bottom: bottom + m };
      const near = (v: number, side: number) => Math.abs(v - side) <= edge;
      const touches = keyed.some((r) => r.right >= b.left - edge && r.left <= b.right + edge && r.bottom >= b.top - edge && r.top <= b.bottom + edge &&
        (near(r.left, b.left) || near(r.right, b.left) || near(r.left, b.right) || near(r.right, b.right) ||
         near(r.top, b.top) || near(r.bottom, b.top) || near(r.top, b.bottom) || near(r.bottom, b.bottom)));
      if (!touches) return [[b.left, b.top], [b.right, b.bottom]];
    }
    return null;
  }, { view: SVG, ref, edge: EDGE_PX });
  if (!found) throw new Unwalkable(`no box around ${ref} keeps every element ${EDGE_PX} px from its sides`);
  return found as [Point, Point];
}

// lonePins keeps the pin points whose pin target lies EDGE_PX or more from any other pin's. A click
// carried into WebGL through the fitted frame lands a pixel or so from where it landed in SVG, and
// that must not move it onto a neighbour.
async function lonePins(page: Page, pins: { at: Point; ref: string }[]): Promise<{ at: Point; ref: string }[]> {
  const keep = await page.evaluate(({ view, pins, edge }) => {
    const rects = [...document.querySelectorAll(`${view} [data-kind="pin"]`)].map((el) => el.getBoundingClientRect());
    return pins.map(({ at }) => {
      const mine = rects.filter((r) => at[0] >= r.left && at[0] <= r.right && at[1] >= r.top && at[1] <= r.bottom);
      const reach = Math.max(0, ...mine.map((r) => r.width / 2));
      return rects.every((r) => mine.includes(r) || Math.hypot(Math.max(r.left - at[0], 0, at[0] - r.right), Math.max(r.top - at[1], 0, at[1] - r.bottom)) > reach + edge);
    });
  }, { view: SVG, pins, edge: EDGE_PX });
  return pins.filter((_, i) => keep[i]);
}

// sheetShown reads Properties as it describes the sheet on screen, its parts and nets among it. A
// click on bare paper names the sheet, so the walk tries points along the view's edges until one
// does. The panel lists the sheet's contents as the drawing reads them, so it waits for the text to
// hold still.
async function sheetShown(page: Page, mode: Mode): Promise<string> {
  const v = await host(page, mode);
  const isSheet = () => page.evaluate((p) => document.querySelector(p)?.querySelector(".pr-kind")?.textContent === "sheet", PROPS);
  const edges: Point[] = [[0.01, 0.5], [0.99, 0.5], [0.5, 0.99], [0.01, 0.9], [0.99, 0.9], [0.01, 0.2], [0.99, 0.2]];
  for (const [fx, fy] of edges) {
    await page.mouse.click(v.x + v.width * fx, v.y + v.height * fy);
    // The canvas builds its pick index on the first click, which takes a few seconds on a large board.
    const named = await page.waitForFunction((p) => document.querySelector(p)?.querySelector(".pr-kind")?.textContent === "sheet", PROPS, { timeout: 5_000 }).then(() => true, () => false);
    if (named) break;
  }
  if (!(await isSheet())) throw new Unwalkable(`no click by the ${mode} view's edges names the sheet`);
  const text = () => page.evaluate((p) => (document.querySelector(p) as HTMLElement | null)?.innerText ?? "", PROPS);
  let last = await text();
  for (let i = 0; i < 20; i++) {
    await page.waitForTimeout(SETTLE_MS);
    const now = await text();
    if (now === last) return now;
    last = now;
  }
  return last;
}

// litSubjects reads what the shown sheet lit. WebGL writes it to the canvas's data-lit. SVG has no
// such list, so the walk matches the keyed elements of the drawn sheet against the subjects the
// canvas was asked to light (data-highlight), as resolveHighlights matches a packed sheet's keys.
async function litSubjects(page: Page, mode: Mode): Promise<string[]> {
  await waitLit(page, mode);
  return readLit(page, mode);
}

// waitLit waits for a mode to draw some highlight.
async function waitLit(page: Page, mode: Mode): Promise<void> {
  if (mode === "webgl") {
    await page
      .waitForFunction((c) => ((document.querySelector(c) as HTMLElement | null)?.dataset.lit ?? "[]") !== "[]", CANVAS, { timeout: 60_000 })
      .catch(() => {
        throw new Error("WebGL lit nothing within 60 s");
      });
    return;
  }
  await page.waitForFunction((s) => document.querySelectorAll(`${s} .highlight-overlay svg *`).length > 0, SVG, { timeout: 60_000 }).catch(() => {
    throw new Unwalkable("SVG drew no highlight within 60 s");
  });
}

// readLit reads what the shown sheet lit, as litSubjects describes, without waiting.
async function readLit(page: Page, mode: Mode): Promise<string[]> {
  if (mode === "webgl") return page.evaluate((c) => JSON.parse((document.querySelector(c) as HTMLElement).dataset.lit ?? "[]") as string[], CANVAS);
  return page.evaluate(({ svg, canvas }) => {
    type Spec = { components?: string[]; nets?: string[]; netIds?: string[]; busIds?: string[]; pins?: { refDes: string; pin: string }[] };
    const specs = JSON.parse((document.querySelector(canvas) as HTMLElement).dataset.highlight ?? "[]") as Spec[];
    const lit = new Set<string>();
    const drawn = [...document.querySelectorAll(`${svg} svg`)].find((s) => !s.closest(".highlight-overlay"));
    if (!drawn) return [];
    for (const spec of specs) {
      const comps = new Set(spec.components ?? []), nets = new Set(spec.nets ?? []), ids = new Set(spec.netIds ?? []);
      const buses = new Set(spec.busIds ?? []), pins = new Set((spec.pins ?? []).map((p) => `${p.refDes}.${p.pin}`));
      for (const el of drawn.querySelectorAll("[data-kind]")) {
        const ref = el.getAttribute("data-ref") ?? "", net = el.getAttribute("data-net") ?? "", id = el.getAttribute("data-net-id") ?? "";
        const bus = el.getAttribute("data-bus") ?? "", pin = el.getAttribute("data-pin") ?? "";
        if (ref && comps.has(ref)) lit.add(`component:${ref}`);
        else if (id && ids.has(id)) lit.add(`net:${net || id}`);
        else if (net && nets.has(net)) lit.add(`net:${net}`);
        else if (bus && buses.has(bus)) lit.add(`bus:${bus}`);
        else if (ref && pin && pins.has(`${ref}.${pin}`)) lit.add(`pin:${ref}.${pin}`);
      }
    }
    return [...lit].sort();
  }, { svg: SVG, canvas: CANVAS });
}

// compareLit opens a link in each mode and compares the sheet it lands on and what it lit there.
async function compareLit(page: Page, walk: Walk, extra: string): Promise<string> {
  const got: { sheet: string; lit: string[] }[] = [];
  for (const mode of MODES) {
    await page.goto(`${base()}/designs/${walk.mount}/${walk.path}/view?mode=${mode}${extra}`, { waitUntil: "networkidle", timeout: 180_000 });
    await drawnSince(page, mode, 0);
    const lit = await litSubjects(page, mode);
    const sheet = (await page.locator(".sheet-tile.active .sheet-tile-name").innerText().catch(() => "")).trim();
    got.push({ sheet, lit });
  }
  const [s, g] = got;
  if (s.lit.length === 0) throw new Error(`SVG lit nothing on ${s.sheet}`);
  if (s.sheet !== g.sheet) throw new Error(`SVG opened ${s.sheet} and WebGL ${g.sheet}`);
  const onlySvg = s.lit.filter((x) => !g.lit.includes(x)), onlyGl = g.lit.filter((x) => !s.lit.includes(x));
  if (onlySvg.length || onlyGl.length) {
    throw new Error(`on ${s.sheet}, only SVG lit [${onlySvg.join(", ")}] and only WebGL lit [${onlyGl.join(", ")}]`);
  }
  return `both lit the same ${s.lit.length} subject(s) on ${s.sheet}`;
}

const ms = (n: number): string => `${n.toFixed(1)} ms`;

// lightFindings turns on the Highlight menu's "show all findings", as a reader would, and runs every
// check on the open board, which then lights every finding drawn on it (agni issue 356). It returns
// how many findings the run listed, how many of them badge the board, and how many subjects the board
// lit. Every finding the board badges has to light there, since the badge says the board draws its
// subject (LocateSubjects, agni issue 1325).
// lightFindings lights every finding through Highlight → show all findings, and checks that the board
// lights each subject a Board badge names. Two findings on one net badge twice and light once, so
// the check compares subjects rather than counts. Each badge sits beside its finding's locate
// button, which names the subject's kind and spells it as the lit list does.
async function lightFindings(page: Page, mode: Mode): Promise<{ findings: number; badged: number; lit: number }> {
  await page.click(".highlight-btn");
  const all = page.locator(".highlight-show-all");
  if (!(await all.isChecked())) await all.check();
  await page.click(".highlight-btn");
  await page.click(".checks-run");
  await page
    .waitForFunction(() => document.querySelector(".checks-run")?.textContent?.trim() === "Run checks", null, { timeout: 600_000 })
    .catch(() => {
      throw new Unwalkable("Run checks did not finish within 10 minutes");
    });
  const findings = await page.locator(".check-locate").count();
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(1000);
  const badged = await page.evaluate(() => {
    const subjects = new Set<string>();
    for (const badge of document.querySelectorAll('.sheet-badge[title="show sheet Board"]')) {
      const locate = badge.closest(".check-subject")?.querySelector(".check-locate");
      const kind = locate?.getAttribute("title")?.split(" ")[1] ?? "";
      const label = locate?.textContent?.trim() ?? "";
      if (label) subjects.add(`${kind || "component"}:${label}`);
    }
    return [...subjects].sort();
  });
  if (badged.length === 0) throw new Unwalkable(`none of the run's ${findings} findings is drawn on the board`);
  const lit = new Set(await readLit(page, mode));
  const dark = badged.filter((s) => !lit.has(s));
  if (dark.length > 0) {
    throw new Error(`${mode} lit ${badged.length - dark.length} of the ${badged.length} subjects the run's findings badge on the board, not ${dark.slice(0, 5).join(", ")}`);
  }
  return { findings, badged: badged.length, lit: lit.size };
}

for (const name of chosen) {
  const walk: Walk = walks[name as keyof typeof walks];
  const label = `${walk.path.split("/").pop()} (${walk.mount})`;
  describe(`the WebGL walk on ${name}`, () => {
    it("compares SVG and WebGL on one board", async () => {
      const rows: Row[] = [];
      const passed = new Set<string>();
      const out = process.env.EXERCISE_OUT;
      if (out) mkdirSync(out, { recursive: true });
      // A recording fixes the page at 1080p and records at that size. A headed page otherwise takes the
      // window's size, and the video showed it in one corner of a grey frame.
      const recorded = { width: 1920, height: 1080 };
      const context = await browser.newContext({
        viewport: video ? recorded : viewportFor({ width: 1400, height: 1000 }),
        ...(video ? { recordVideo: { dir: video, size: recorded } } : {}),
      });
      const page = await context.newPage();
      const pageErrors: string[] = [];
      page.on("pageerror", (e) => pageErrors.push(String(e)));
      const timings: Record<string, Record<Mode, { draw: number; frames: FrameSummary }>> = {};
      let gpu = "";

      const step = async (step: string, opts: { waitsOn?: string[]; needs?: string[] }, body: () => Promise<string>): Promise<void> => {
        const waitsOn = opts.waitsOn ?? [];
        const missing = (opts.needs ?? []).filter((s) => !passed.has(s));
        if (missing.length > 0) {
          rows.push({ step, design: label, status: "not available", detail: `not reached, after ${missing.join(", ")}` });
          return;
        }
        const t0 = Date.now();
        let ok = false;
        let detail: string;
        try {
          detail = await body();
          ok = true;
          passed.add(step);
        } catch (e) {
          detail = (e instanceof Error ? e.message : String(e)).split("\n")[0];
          if (e instanceof Unwalkable) {
            rows.push({ step, design: label, ms: Date.now() - t0, status: "fail", detail: `the walk could not set the step up: ${detail}` });
            return;
          }
          if (e instanceof NotJudged) {
            rows.push({ step, design: label, ms: Date.now() - t0, status: "not available", detail });
            return;
          }
        }
        const open = waitsOn.filter((t) => !closed.has(t));
        const shut = waitsOn.filter((t) => closed.has(t));
        if (shut.length > 0) detail += ` (${shut.join(", ")} closed)`;
        rows.push({ step, design: label, ms: Date.now() - t0, ...waitedOn(ok, open, detail) });
      };

      try {
        await step("densest sheet", {}, async () => {
          // Opened on the board, so every schematic tile clicked below draws a sheet not yet shown.
          await open(page, walk, "svg", "Board");
          const names = await page.$$eval("#sheet-overview .sheet-tile .sheet-tile-name", (ns) => ns.map((n) => (n.textContent ?? "").trim()));
          const counts: [string, number][] = [];
          for (const n of names.filter((n) => n !== "Board")) {
            const since = await page.evaluate(() => performance.now());
            await tile(page, n);
            await drawnSince(page, "svg", since);
            counts.push([n, await page.evaluate((s) => [...document.querySelectorAll(`${s} svg`)].find((e) => !e.closest(".highlight-overlay"))?.querySelectorAll("*").length ?? 0, SVG)]);
          }
          counts.sort((a, b) => b[1] - a[1]);
          if (counts[0][0] !== walk.sheet) throw new Unwalkable(`${counts[0][0]} draws ${counts[0][1]} SVG elements, more than ${walk.sheet}, so the walk names the wrong sheet`);
          return `${walk.sheet}, ${counts[0][1]} SVG elements, against ${counts[1]?.[1] ?? 0} on ${counts[1]?.[0] ?? "no other sheet"}`;
        });

        for (const sheet of ["Board", walk.sheet]) {
          for (const mode of MODES) {
            await step(`draw ${sheet} (${mode})`, {}, async () => {
              // Open on another sheet, so the timed draw is a tile click to a sheet not yet shown.
              await open(page, walk, mode, sheet === "Board" ? walk.sheet : "Board");
              if (!gpu) gpu = await page.evaluate(() => {
                const gl = document.createElement("canvas").getContext("webgl2");
                const ext = gl?.getExtension("WEBGL_debug_renderer_info");
                return gl && ext ? String(gl.getParameter(ext.UNMASKED_RENDERER_WEBGL)) : "no WebGL2";
              });
              const since = await page.evaluate(() => performance.now());
              await tile(page, sheet);
              const drawn = await drawnSince(page, mode, since);
              await idle(page);
              await page.waitForLoadState("networkidle");
              const frames = await panZoom(page, mode);
              (timings[sheet] ??= {} as Record<Mode, { draw: number; frames: FrameSummary }>)[mode] = { draw: drawn - since, frames };
              return `first draw ${ms(drawn - since)}, frames p50 ${ms(frames.p50)}, p95 ${ms(frames.p95)}, max ${ms(frames.max)} over ${frames.frames}`;
            });
          }
        }

        // The board again with highlights on, since #1331 expected the lag there, as the highlight
        // recolour sets one uniform per highlighted primitive. A reader lights a run's findings with
        // Highlight → show all findings. The walk also lights the ground net, the largest highlight a
        // board draws, through the page API's locateEntity, and times that.
        for (const mode of MODES) {
          await step(`light the findings on the board (${mode})`, { needs: [`draw Board (${mode})`] }, async () => {
            await open(page, walk, mode, "Board");
            const { findings, badged, lit } = await lightFindings(page, mode);
            const frames = await panZoom(page, mode);
            return `the run's ${findings} findings badge ${badged} subjects on the board, and all of them lit, frames p50 ${ms(frames.p50)}, p95 ${ms(frames.p95)}, max ${ms(frames.max)} over ${frames.frames}`;
          });
        }
        const litTimes: Partial<Record<Mode, FrameSummary>> = {};
        for (const mode of MODES) {
          await step(`pan the board with ${walk.ground} lit (${mode})`, { needs: [`draw Board (${mode})`] }, async () => {
            await open(page, walk, mode, "Board", "&devpage=1");
            // The page API is installed once the URL's design has opened, after the first draw.
            await page.waitForFunction(() => "__agniViewerPage" in window, null, { timeout: 60_000 });
            await page.evaluate(
              (net) => (window as unknown as { __agniViewerPage: { locateEntity(k: string, s: string, o: { sheet: string }): Promise<void> } }).__agniViewerPage.locateEntity("net", net, { sheet: "board" }),
              walk.ground,
            );
            const lit = await litSubjects(page, mode);
            if (!lit.includes(`net:${walk.ground}`)) throw new Unwalkable(`${mode} lit [${lit.join(", ")}], not ${walk.ground}`);
            const frames = await panZoom(page, mode);
            litTimes[mode] = frames;
            return `frames p50 ${ms(frames.p50)}, p95 ${ms(frames.p95)}, max ${ms(frames.max)} over ${frames.frames}`;
          });
        }

        // This passed at its first headed run on Jetson (agni issue 1330), so it waits on nothing, and a
        // headed run where WebGL loses is a regression.
        await step("WebGL beats SVG on the board", { needs: ["draw Board (svg)", "draw Board (webgl)"] }, async () => {
          if (!headed) throw new NotJudged("judged only headed, since headless Chrome draws WebGL in software");
          if (video) throw new NotJudged("not judged while the walk records a video, which competes for the CPU");
          const { svg, webgl } = timings.Board;
          const verdict = `p95 ${ms(webgl.frames.p95)} against ${ms(svg.frames.p95)}, first draw ${ms(webgl.draw)} against ${ms(svg.draw)}`;
          // WebGL has to take at most 80% of SVG's p95 frame time to count as beating it.
          if (webgl.frames.p95 > 0.8 * svg.frames.p95) throw new Error(`WebGL ${verdict}`);
          return `WebGL ${verdict}`;
        });

        // This also passed at its first headed run on Jetson, with GND lit, so it waits on nothing.
        await step("WebGL beats SVG on the lit board", { needs: [`pan the board with ${walk.ground} lit (svg)`, `pan the board with ${walk.ground} lit (webgl)`] }, async () => {
          if (!headed) throw new NotJudged("judged only headed, since headless Chrome draws WebGL in software");
          if (video) throw new NotJudged("not judged while the walk records a video, which competes for the CPU");
          const { svg, webgl } = litTimes as Record<Mode, FrameSummary>;
          const verdict = `p95 ${ms(webgl.p95)} against ${ms(svg.p95)}, max ${ms(webgl.max)} against ${ms(svg.max)}`;
          if (webgl.p95 > 0.8 * svg.p95) throw new Error(`WebGL ${verdict}`);
          return `WebGL ${verdict}`;
        });

        const targets: Target[] = [
          { name: "footprint", sheet: "Board", sel: `${SVG} .picks-front [data-kind="component"]`, kind: "component", clear: true, waitsOn: [] },
          { name: "pad", sheet: "Board", sel: `${SVG} [data-kind="pin"]`, kind: "pin", waitsOn: [] },
          { name: "track", sheet: "Board", sel: `${SVG} [data-kind="net"]`, kind: "net", line: true, waitsOn: [] },
          { name: "schematic pin", sheet: walk.sheet, sel: `${SVG} [data-kind="pin"]`, kind: "pin", waitsOn: [] },
          // A bus is picked by its name, which the KiCad reader gives each drawn bus from the label
          // that names it (agni issue 1435).
          { name: "bus", sheet: walk.busSheet, sel: `${SVG} [data-kind="bus"]:not([data-bus=""])`, kind: "bus", line: true, waitsOn: [] },
        ];
        for (const t of targets) {
          await step(`click a ${t.name}`, { waitsOn: t.waitsOn }, async () => {
            const { svg, gl } = await clickBoth(page, walk, name, t);
            if (!same(svg, gl)) throw new Error(apart(svg, gl));
            return `both picked ${said(svg)}`;
          });
        }

        await step("box select", {}, async () => {
          await open(page, walk, "svg", walk.sheet);
          const near = (await candidates(page, { name: "part", sheet: walk.sheet, sel: `${SVG} [data-kind="component"]`, kind: "component", waitsOn: [] }))[0];
          if (!near) throw new Unwalkable(`${walk.sheet} draws no part to box`);
          const corners = await clearBox(page, near.ref);
          const svgFit = await anchors(page, "svg");
          await open(page, walk, "webgl", walk.sheet);
          const frame = frameBetween(svgFit, await anchors(page, "webgl"));
          const got: Shown[] = [];
          for (const mode of MODES) {
            await open(page, walk, mode, walk.sheet);
            const [a, b] = mode === "svg" ? corners : corners.map((c) => mapPoint(frame, c));
            await page.keyboard.down("Shift");
            await page.mouse.move(a[0], a[1]);
            await page.mouse.down();
            for (let i = 1; i <= 10; i++) await page.mouse.move(a[0] + ((b[0] - a[0]) * i) / 10, a[1] + ((b[1] - a[1]) * i) / 10);
            await page.mouse.up();
            await page.keyboard.up("Shift");
            got.push(await shown(page, mode === "svg" ? 10_000 : 5_000));
          }
          const [svg, gl] = got;
          if (svg.kind !== "area") throw new Unwalkable(`a Shift-drag in SVG selected ${said(svg)}, not an area`);
          if (!same(svg, gl)) throw new Error(apart(svg, gl));
          return `both selected ${said(svg)}`;
        });

        // Two Shift-clicks on pins of two parts make an area of the two pins. Both views zoom in on the
        // sheet's middle by the same wheel steps first, as the click steps do, since a fitted dense
        // sheet leaves no pin clear of its neighbours.
        await step("Shift-click two pins", {}, async () => {
          const zoom = ZOOM[name as keyof typeof ZOOM].sheet;
          await open(page, walk, "svg", walk.sheet);
          const svgFit = await anchors(page, "svg");
          const v = await host(page, "svg");
          const mid: Point = [v.x + v.width / 2, v.y + v.height / 2];
          const svgPivot = await zoomIn(page, mid, zoom);
          const pins = await lonePins(page, await candidates(page, { name: "pin", sheet: walk.sheet, sel: `${SVG} [data-kind="pin"]`, kind: "pin", waitsOn: [] }, 200));
          const two = pins.filter((p, i) => pins.findIndex((q) => q.ref === p.ref) === i).slice(0, 2);
          if (two.length < 2) throw new Unwalkable(`${walk.sheet} draws no two parts' pins near its middle with no other pin within ${EDGE_PX} px`);
          const svgZoomed = await anchors(page, "svg");
          const shiftClick = async (points: Point[]): Promise<Shown> => {
            await page.keyboard.down("Shift");
            for (const [x, y] of points) await page.mouse.click(x, y);
            await page.keyboard.up("Shift");
            return shown(page, 10_000);
          };
          const svg = await shiftClick(two.map((p) => p.at));
          await open(page, walk, "webgl", walk.sheet);
          const frame = frameBetween(svgFit, await anchors(page, "webgl"));
          const glPivot = await zoomIn(page, mapPoint(frame, svgPivot), zoom);
          const fitted = refit(svgZoomed, await anchors(page, "webgl"));
          if (fitted && Math.abs(fitted.kx / frame.kx - 1) > ZOOM_SLACK) throw new Unwalkable(`WebGL zoomed ${(fitted.kx / frame.kx).toFixed(2)} times as far as SVG on ${walk.sheet}`);
          const zoomed = fitted ?? zoomFrame(frame, svgPivot, glPivot, zoomBy(zoom));
          const gl = await shiftClick(two.map((p) => mapPoint(zoomed, p.at)));
          if (svg.kind !== "area") throw new Unwalkable(`two Shift-clicks in SVG selected ${said(svg)}, not an area`);
          if (!same(svg, gl)) throw new Error(apart(svg, gl));
          return `both selected ${said(svg)}`;
        });

        // Properties lists what the sheet on screen draws, read off whichever renderer drew it (agni
        // issue 1333).
        for (const sheet of [walk.sheet, "Board"]) {
          await step(`what ${sheet} draws`, {}, async () => {
            const got: string[] = [];
            for (const mode of MODES) {
              await open(page, walk, mode, sheet);
              got.push(await sheetShown(page, mode));
            }
            const [svg, gl] = got;
            if (svg !== gl) {
              const la = svg.split("\n"), lb = gl.split("\n");
              const i = la.findIndex((l, j) => l !== lb[j]);
              throw new Error(`Properties differs at line ${i + 1}, ${JSON.stringify(la[i] ?? "")} in SVG and ${JSON.stringify(lb[i] ?? "")} in WebGL`);
            }
            return `both list ${svg.split("\n").length} lines`;
          });
        }

        // The hidden SVG host keeps the last sheet it drew. A reader can now switch to SVG and back
        // in one page, so Properties must read whichever renderer drew the sheet on screen, not the
        // SVG host's old sheet (agni issue 1333, reachable since 1336).
        await step("what a sheet draws after SVG and back", {}, async () => {
          await open(page, walk, "svg", walk.sheet);
          const svg = await sheetShown(page, "svg");
          await open(page, walk, "webgl", "Board");
          let since = await page.evaluate(() => performance.now());
          await page.locator(".mode-btn", { hasText: "SVG" }).click();
          await drawnSince(page, "svg", since);
          await idle(page);
          since = await page.evaluate(() => performance.now());
          await page.locator(".mode-btn", { hasText: "WebGL" }).click();
          await drawnSince(page, "webgl", since);
          await idle(page);
          since = await page.evaluate(() => performance.now());
          await tile(page, walk.sheet);
          await drawnSince(page, "webgl", since);
          await idle(page);
          const gl = await sheetShown(page, "webgl");
          if (svg !== gl) {
            const la = svg.split("\n"), lb = gl.split("\n");
            const i = la.findIndex((l, j) => l !== lb[j]);
            throw new Error(`Properties differs at line ${i + 1}, ${JSON.stringify(la[i] ?? "")} in SVG and ${JSON.stringify(lb[i] ?? "")} in WebGL after the switch`);
          }
          return `both list ${svg.split("\n").length} lines`;
        });

        await step("a finding's highlight", {}, async () => {
          const v = walk.verdict;
          return compareLit(page, walk, `&verdict=${encodeURIComponent(v.id)}&rule=${encodeURIComponent(v.rule)}&subject=${encodeURIComponent(v.subject)}`);
        });

        await step("a trace's highlight", {}, async () => compareLit(page, walk, `&trace=${encodeURIComponent(walk.trace)}`));

        // A board beside its schematic draws in a sheet view, in the renderer the view was opened in
        // (agni issue 1368). These steps run last, since a view docked beside the canvas narrows it
        // in the layout the page saves, and the clicks above find their targets at the canvas's size. The walk opens the canvas on the densest schematic sheet and a view of the
        // board beside it through the page API, then times the view's first draw and pans it.
        const besideTimes: Partial<Record<Mode, FrameSummary>> = {};
        for (const mode of MODES) {
          await step(`pan a board view beside ${walk.sheet} (${mode})`, {}, async () => {
            await open(page, walk, "svg", walk.sheet, "&devpage=1");
            await page.waitForFunction(() => "__agniViewerPage" in window, null, { timeout: 60_000 });
            const since = await page.evaluate(() => performance.now());
            const view = await page.evaluate(
              (m) => (window as unknown as { __agniViewerPage: { openSheetView(s: string, r: string): Promise<{ id: string; renderer: string }> } }).__agniViewerPage.openSheetView("board", m),
              mode,
            );
            if (view.renderer !== mode) throw new Unwalkable(`the view drew in ${view.renderer}, not ${mode}`);
            const drawn = await drawnSince(page, mode, since, view.id);
            await page.waitForLoadState("networkidle");
            const frames = await panZoom(page, mode, `[data-sheetview="${view.id}"] .sheetview-stage`);
            besideTimes[mode] = frames;
            await page.evaluate((id) => (window as unknown as { __agniViewerPage: { closeSheetView(v: string): boolean } }).__agniViewerPage.closeSheetView(id), view.id);
            return `first draw ${ms(drawn - since)}, frames p50 ${ms(frames.p50)}, p95 ${ms(frames.p95)}, max ${ms(frames.max)} over ${frames.frames}`;
          });
        }
        await step("a WebGL board view beats an SVG one", { needs: [`pan a board view beside ${walk.sheet} (svg)`, `pan a board view beside ${walk.sheet} (webgl)`] }, async () => {
          if (!headed) throw new NotJudged("judged only headed, since headless Chrome draws WebGL in software");
          if (video) throw new NotJudged("not judged while the walk records a video, which competes for the CPU");
          const svg = besideTimes.svg!, webgl = besideTimes.webgl!;
          const verdict = `p95 ${ms(webgl.p95)} against ${ms(svg.p95)}`;
          if (webgl.p95 > 0.8 * svg.p95) throw new Error(`WebGL ${verdict}`);
          return `WebGL ${verdict}`;
        });

        await step("no page errors", {}, async () => {
          if (pageErrors.length) throw new Error(pageErrors.join("; "));
          return "none";
        });
      } finally {
        const machine =
          `${headed ? "Headed" : "Headless"} Chromium${video ? ", recording a video" : ""}, renderer ${gpu || "unknown"}, load average ${loadavg().map((l) => l.toFixed(2)).join(" ")}. ` +
          "Times are on the page's performance.now. A first draw runs from a Sheets tile click to the frame after the sheet first shows, and frame times are requestAnimationFrame deltas over 80 wheel steps and a 600 px drag, one input per frame.";
        const report = `## WebGL walk, ${walk.about}\n\n${machine}\n\n${table(rows)}\n`;
        process.stdout.write(`\n${report}\n`);
        if (out) writeFileSync(join(out, "webgl.md"), report);
        const recording = page.video();
        await page.close();
        await context.close();
        if (recording) {
          const file = join(video, `webgl-walk-${name}.webm`);
          await recording.saveAs(file);
          await recording.delete();
          process.stdout.write(`video: ${file}\n`);
        }
      }
      const broken = rows.filter((r) => r.status === "fail").map((r) => `${r.step}: ${r.detail}`);
      expect(exitCode(rows), broken.join("\n")).toBe(0);
    }, WALK_TIMEOUT_MS[name as keyof typeof walks] ?? 1_800_000);
  });
}
