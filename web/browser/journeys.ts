// The journeys `make demo-video` records (agni issue 1545, mission #1442). A journey is one thing a
// person does in the viewer, told as steps in the order they'd do them. Each step has a caption,
// which the video shows before the step happens, and an action, which acts as a reader would and
// checks what the viewer did. demo-video.spec.ts plays every journey in the gate at test pace, on the
// cut of Jetson unless the journey names another design, so a journey that breaks fails CI and names
// its step. With DEMO_OUT set it plays one journey on the whole board, paced for a viewer, and
// records it.
//
// Add a journey here when the viewer gives a reader something new to see. The mission issue lists
// the backlog.

import type { Page } from "playwright-core";
import { PANE, TITLE, centreOf, centredIn } from "./picks.js";

// Compare is a later revision of a design, and what changed in it, for the compare journey (agni
// issue 1559). The Compare picker opens on the folder holding the open design, so dir is the folder
// to expand there and file the revision's file in it.
export interface Compare {
  dir: string;
  file: string;
  // part is a part the revision adds, and net a net it adds, drawn on one sheet so a click goes there.
  part: string;
  net: string;
}

// Design is a board a journey plays on, with the parts its steps name.
export interface Design {
  mount: string;
  path: string;
  label: string;
  // find is a part to find by name, and findSheet the sheet that draws it.
  find: string;
  findSheet: string;
  // testPoint is a one-pad part to click on the board, on another sheet than find.
  testPoint: string;
  testPointSheet: string;
  compare?: Compare;
}

export const designs: Record<string, Design> = {
  // The gate's cut of Jetson, committed in examples/cross-probe-project.
  cut: {
    mount: "probe",
    path: "designs/jetson-cut",
    label: "a cut of Jetson's baseboard",
    find: "R119",
    findSheet: "USB Debug, PD",
    testPoint: "TP76",
    testPointSheet: "Power",
  },
  // The tutorial's gateway board, whose rev C the gate compares it with, since the cut has no
  // revision.
  gateway: {
    mount: "tut",
    path: "designs/gateway",
    label: "the tutorial's gateway board",
    find: "R1",
    findSheet: "gateway",
    testPoint: "TP1",
    testPointSheet: "gateway",
    compare: { dir: "gateway", file: "gateway-rev-c.edn", part: "R4", net: "PMIC_ENABLE" },
  },
  // Antmicro's open-source baseboard for the Jetson AGX Thor module, under Apache-2.0, fetched by
  // `make samples-oracle` and copied in by `make -C examples/jetson-project board`.
  jetson: {
    mount: "jetson",
    path: "designs/jetson",
    label: "Antmicro's Jetson AGX Thor baseboard",
    find: "R361",
    findSheet: "SoM_IO",
    testPoint: "TP76",
    testPointSheet: "Power",
    // Rev 1.1.2, fetched by `make samples-revisions` and copied in by `make -C
    // examples/jetson-project revision` (agni issue 1559). J21 is its new recovery and reset header,
    // and FAN_DCDC_SW the switch node of its new fan regulator.
    compare: { dir: "jetson-1.1.2", file: "jetson-agx-thor-baseboard.kicad_sch", part: "J21", net: "/DCDC/FAN_DCDC_SW" },
  },
};

// Pace is how a journey moves. The gate plays at test pace, all zeros. A recording waits so a viewer
// can follow, and moves the cursor to a target before it clicks.
export interface Pace {
  caption: number; // how long a caption shows before its step, in ms
  settle: number; // how long a step's result stays on screen afterwards, in ms
  typeDelay: number; // ms between typed keys
  moveSteps: number; // mouse moves on the way to a click
}

export const testPace: Pace = { caption: 0, settle: 0, typeDelay: 0, moveSteps: 1 };
export const viewerPace: Pace = { caption: 1800, settle: 1600, typeDelay: 140, moveSteps: 40 };

// Reader is what a step acts through, at the journey's pace.
export interface Reader {
  page: Page;
  base: string; // the server's origin, such as http://127.0.0.1:52238
  pace: Pace;
  click(x: number, y: number): Promise<void>;
  type(text: string): Promise<void>;
  press(key: string): Promise<void>;
}

export interface Step {
  caption: string;
  // run acts, waits for the viewer to answer, and throws when the answer is wrong.
  run: (r: Reader, d: Design) => Promise<void>;
}

export interface Journey {
  name: string;
  title: string;
  // gate names the design the gate plays this journey on, the cut unless it says otherwise.
  gate?: string;
  steps: Step[];
}

const SHEET = "#svg-view";
const BOARD = `${PANE} .sheetview-canvas`;
const activeSheet = (page: Page) => page.locator(".sheet-tile.active .sheet-tile-name").textContent().then((t) => (t ?? "").trim());

async function onSheet(page: Page, name: string): Promise<void> {
  await page
    .waitForFunction((n) => document.querySelector(".sheet-tile.active .sheet-tile-name")?.textContent?.trim() === n, name, { timeout: 30_000 })
    .catch(async () => {
      throw new Error(`the canvas shows ${await activeSheet(page)}, not ${name}`);
    });
}

// centred waits for an element to come to the middle third of a host, where a reveal puts it.
async function centred(page: Page, el: string, host: string, what: string): Promise<void> {
  await page
    .waitForFunction(
      ({ el, host }) => {
        const e = document.querySelector(el)?.getBoundingClientRect();
        const h = document.querySelector(host)?.getBoundingClientRect();
        if (!e || !h) return false;
        const cx = e.left + e.width / 2, cy = e.top + e.height / 2;
        return cx > h.left + h.width / 3 && cx < h.right - h.width / 3 && cy > h.top + h.height / 3 && cy < h.bottom - h.height / 3;
      },
      { el, host },
      { timeout: 30_000 },
    )
    .catch(() => {
      throw new Error(`${what} did not come to the middle of the view`);
    });
  if (!(await centredIn(page, el, host))) throw new Error(`${what} is off the middle of the view`);
}

// zoomedIn waits until the canvas shows its sheet at least `factor` times its fitted width.
async function zoomedIn(page: Page, factor: number): Promise<void> {
  await page
    .waitForFunction(
      (factor) => {
        const host = document.querySelector("#svg-view")!.getBoundingClientRect();
        const doc = document.querySelector("#svg-view svg")!.getBoundingClientRect();
        return doc.width / host.width >= factor;
      },
      factor,
      { timeout: 30_000 },
    )
    .catch(() => {
      throw new Error(`the canvas did not zoom in`);
    });
}

const footprintOf = (ref: string): string => `${BOARD} .picks-front [data-ref="${ref}"], ${BOARD} .picks-back [data-ref="${ref}"]`;

export const journeys: Journey[] = [
  {
    name: "find-a-part",
    title: "Find a part on the board",
    steps: [
      {
        caption: "Open the schematic",
        run: async (r, d) => {
          await r.page.goto(`${r.base}/designs/${d.mount}/${d.path}/view?mode=svg`, { waitUntil: "networkidle" });
          await r.page.waitForSelector(`${SHEET} svg`, { timeout: 60_000 });
        },
      },
      {
        caption: "Open the board beside it from the Sheets drawer",
        run: async (r) => {
          await r.press("Control+g");
          const row = r.page.locator(".sheet-tile-row", { has: r.page.locator(".sheet-tile-name", { hasText: /^Board$/ }) });
          const b = await row.locator(".sheet-tile-beside").boundingBox({ timeout: 30_000 });
          if (!b) throw new Error("the Sheets panel lists no Board");
          await r.click(b.x + b.width / 2, b.y + b.height / 2);
          await r.page.waitForSelector(`${PANE} .sheetview-canvas svg`, { timeout: 30_000 });
        },
      },
      {
        caption: "Find a resistor by name",
        run: async (r, d) => {
          await r.press("Control+f");
          await r.page.waitForSelector("input.query-term", { state: "visible", timeout: 10_000 });
          await r.type(d.find);
          await r.press("Enter");
          const row = `.query-row .query-locate`;
          await r.page.locator(row, { hasText: new RegExp(`^${d.find}$`) }).first().waitFor({ timeout: 30_000 });
        },
      },
      {
        caption: "Click it: the schematic opens its sheet and zooms in, and the board centres on it",
        run: async (r, d) => {
          const name = r.page.locator(".query-row .query-locate", { hasText: new RegExp(`^${d.find}$`) }).first();
          const b = await name.boundingBox();
          if (!b) throw new Error(`${d.find} is not in the answer`);
          await r.click(b.x + b.width / 2, b.y + b.height / 2);
          await onSheet(r.page, d.findSheet);
          await zoomedIn(r.page, 2);
          await centred(r.page, `${SHEET} [data-kind="component"][data-ref="${d.find}"]`, SHEET, `${d.find} on the schematic`);
          await centred(r.page, footprintOf(d.find), BOARD, `${d.find}'s footprint on the board`);
        },
      },
      {
        caption: "Click a test point on the board: the schematic follows to its sheet",
        run: async (r, d) => {
          // An idle SVG board view parks as an image (SvgView.parkWhenIdle), and a pointer over it
          // wakes it, so the reader's cursor goes to the board first.
          const host = await r.page.locator(BOARD).first().boundingBox();
          if (!host) throw new Error("the board view is not on screen");
          await r.page.mouse.move(host.x + host.width / 2, host.y + host.height / 2, { steps: r.pace.moveSteps });
          await r.page.waitForSelector(`${BOARD} svg:not(.svg-parked)`, { state: "visible", timeout: 10_000 });
          // The board view is zoomed in on the last part, so the reader zooms it out until the test
          // point is in view, as they would to look for it.
          for (let i = 0; i < 15; i++) {
            const h = await r.page.locator(BOARD).first().boundingBox();
            const t = await r.page.locator(footprintOf(d.testPoint)).first().boundingBox();
            if (!h || !t) throw new Error(`${d.testPoint} is not drawn on the board`);
            const cx = t.x + t.width / 2, cy = t.y + t.height / 2;
            const inside = cx > h.x + 20 && cx < h.x + h.width - 20 && cy > h.y + 20 && cy < h.y + h.height - 20;
            if (inside && Math.min(t.width, t.height) >= 8) break;
            await r.page.mouse.move(h.x + h.width / 2, h.y + h.height / 2);
            await r.page.mouse.wheel(0, inside ? -300 : 300);
            await r.page.waitForTimeout(r.pace.moveSteps > 1 ? 350 : 150);
          }
          const at = await centreOf(r.page, footprintOf(d.testPoint));
          await r.click(at.x, at.y);
          await r.page.waitForFunction(
            ({ sel, ref }) => {
              const t = document.querySelector(sel)?.textContent ?? "";
              return t === ref || t.startsWith(`${ref}.`);
            },
            { sel: TITLE, ref: d.testPoint },
            { timeout: 30_000 },
          );
          await onSheet(r.page, d.testPointSheet);
          await centred(r.page, `${SHEET} [data-kind="component"][data-ref="${d.testPoint}"]`, SHEET, `${d.testPoint} on the schematic`);
        },
      },
    ],
  },
  {
    name: "compare-revisions",
    title: "Compare two revisions of a board",
    gate: "gateway",
    steps: [
      {
        caption: "Open the schematic",
        run: async (r, d) => {
          await r.page.goto(`${r.base}/designs/${d.mount}/${d.path}/view?mode=svg`, { waitUntil: "networkidle" });
          await r.page.waitForSelector(`${SHEET} svg`, { timeout: 60_000 });
        },
      },
      {
        caption: "Compare it with the next revision",
        run: async (r, d) => {
          const c = revision(d);
          await clickOn(r, r.page.locator("#compare button"));
          const tree = r.page.locator("#compare-tree");
          const dir = tree.locator("li", { has: r.page.locator("> button.node.dir", { hasText: new RegExp(`${escape(c.dir)}$`) }) }).first();
          await clickOn(r, dir.locator("> button.node.dir"));
          await clickOn(r, dir.locator("button.node:not(.dir)", { hasText: c.file }).first());
          await r.page.waitForSelector(".diff-legend .diff-chip", { timeout: 60_000 });
          const b = (await r.page.locator(".diff-file").last().textContent()) ?? "";
          if (!b.endsWith(`${c.dir}/${c.file}`)) throw new Error(`side B is ${b}, not ${c.dir}/${c.file}`);
          if ((await r.page.locator(".diff-error").count()) > 0) throw new Error(`the comparison failed: ${await r.page.locator(".diff-error").textContent()}`);
        },
      },
      {
        caption: "The list of changes sits beside the two drawings",
        run: async (r) => {
          await r.page.locator(".finding-group-head").first().waitFor({ timeout: 30_000 });
          for (const id of ["diff", "changes"]) {
            const shown = await r.page.evaluate((id) => {
              const hole = document.querySelector(`[data-dock-panel="${id}"]`);
              return !!hole && hole.getBoundingClientRect().width > 0 && getComputedStyle(hole).visibility !== "hidden";
            }, id);
            if (!shown) throw new Error(`the ${id} panel is not on screen`);
          }
        },
      },
      {
        caption: "Click a part the new revision adds: the drawings go to its sheet",
        run: async (r, d) => {
          const c = revision(d);
          await clickOn(r, change(r.page, c.part));
          await stillInList(r.page, c.part);
          await centred(r.page, `#diff-svg-b [data-kind="component"][data-ref="${c.part}"]`, "#diff-svg-b", `${c.part} in the new revision`);
        },
      },
      {
        caption: "Click a net it adds: the drawings go to where it runs",
        run: async (r, d) => {
          const c = revision(d);
          await clickOn(r, change(r.page, c.net));
          await stillInList(r.page, c.net);
          await centred(r.page, `#diff-svg-b [data-kind="net"][data-net="${c.net}"]`, "#diff-svg-b", `${c.net} in the new revision`);
        },
      },
    ],
  },
];

// revision returns the design's later revision, which the compare journey needs.
function revision(d: Design): Compare {
  if (!d.compare) throw new Error(`${d.label} has no revision to compare with`);
  return d.compare;
}

const escape = (s: string): string => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

// change is the Changes list's row for one changed subject.
const change = (page: Page, subject: string) =>
  page.locator("button.finding.chg", { has: page.locator(".finding-subject", { hasText: new RegExp(`^${escape(subject)}$`) }) }).first();

// stillInList checks that the row just clicked is still in view in the Changes list, which once
// jumped back to its top on every click (agni issue 1559).
async function stillInList(page: Page, subject: string): Promise<void> {
  const inView = await change(page, subject).evaluate((row) => {
    const list = row.closest('[data-dock-panel="changes"]')!.getBoundingClientRect();
    const r = row.getBoundingClientRect();
    return r.top >= list.top && r.bottom <= list.bottom && row.classList.contains("selected");
  });
  if (!inView) throw new Error(`the Changes list moved ${subject}'s row out of view`);
}

// clickOn scrolls an element into view and clicks its middle at the reader's pace.
async function clickOn(r: Reader, el: ReturnType<Page["locator"]>): Promise<void> {
  await el.scrollIntoViewIfNeeded({ timeout: 30_000 });
  const b = await el.boundingBox();
  if (!b) throw new Error("the element to click is not on screen");
  await r.click(b.x + b.width / 2, b.y + b.height / 2);
}

export const journeyNames = (): string[] => journeys.map((j) => j.name);
