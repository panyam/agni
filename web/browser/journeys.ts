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

// Example is one changed subject the compare journey clicks, and what to say about it.
export interface Example {
  subject: string;
  note: string; // follows the subject in the caption, such as "a recovery and reset header"
}

// Compare is a later revision of a design, and one change of each kind in it, for the compare
// journey (agni issues 1559 and 1560). The Compare picker opens on the folder holding the open
// design, so dir is the folder to expand there and file the revision's file in it. from and to name
// the two revisions in captions. A kind with no example is a step the journey skips on this design.
export interface Compare {
  dir: string;
  file: string;
  from: string;
  to: string;
  added?: Example;
  removed?: Example;
  changed?: Example;
  newNet?: Example;
  deletedNet?: Example;
  renamed?: Example & { old: string };
  hard?: Example;
}

// Walk is a part to walk outwards from, for the walk journey. stops is a net the first hop reaches
// and stops at, and rail the rail the start's own net is pulled to. A design without one skips that
// step.
export interface Walk {
  start: string;
  sheet: string;
  about: string; // follows the start in a caption, such as "a pull-up on the I2C data line"
  first: string; // a net the first hop reaches
  stops?: string;
  wider: string; // a net the second hop reaches
  widerNote: string; // follows "the walk" in a caption
  rail?: string;
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
  walk?: Walk;
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
    walk: {
      start: "R119",
      sheet: "USB Debug, PD",
      about: "an LED's series resistor",
      first: "/USB Debug, PD/FTDI_3V3",
      wider: "Net-(D21-A)",
      widerNote: "reaches a second LED through R120",
    },
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
    compare: {
      dir: "gateway",
      file: "gateway-rev-c.edn",
      from: "the gateway",
      to: "rev C",
      added: { subject: "R4", note: "a pull-up on I2C_SCL" },
      newNet: { subject: "PMIC_ENABLE", note: "is the PMIC's enable line in rev C" },
      deletedNet: { subject: "PMIC_EN", note: "was the PMIC's enable line before" },
      renamed: { subject: "CLK_IN", old: "XTAL_IN", note: "" },
      hard: { subject: "I2C_SCL", note: "gained the pull-up R4" },
    },
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
    // Rev 1.1.3, Antmicro's current revision, fetched by `make samples-revisions` and copied in by
    // `make -C examples/jetson-project revision` (agni issues 1559 and 1560).
    compare: {
      dir: "jetson-1.1.3",
      file: "jetson-agx-thor-baseboard.kicad_sch",
      from: "rev 1.1.0",
      to: "rev 1.1.3",
      added: { subject: "J21", note: "a new recovery and reset header" },
      removed: { subject: "R27", note: "from the Power sheet" },
      changed: { subject: "D53", note: "was a zener diode and is now a green LED" },
      newNet: { subject: "/DCDC/FAN_DCDC_SW", note: "is the switch node of a new fan regulator" },
      deletedNet: { subject: "/Peripherals/FAN_PWM", note: "carried the fan's PWM before it moved to the DCDC sheet" },
      renamed: { subject: "/SoM_Power/VIN_PWR_ON", old: "/DCDC/VIN_PWR_ON", note: "" },
      hard: { subject: "/Power/3V3_LDO", note: "is now fed by U79 instead of U15" },
    },
    walk: {
      start: "R361",
      sheet: "SoM_IO",
      about: "a pull-up on the I2C connector's data line",
      first: "/Peripherals/I2C_{CONN}.SDA",
      stops: "+3V3",
      wider: "/SoM_IO/I2C5_SDA_1V8",
      widerNote: "crosses the level shifter U80 to the module's 1.8 V side",
      rail: "+3V3",
    },
  },
};

// Pace is how a journey moves. The gate plays at test pace, all zeros. A recording waits so a viewer
// can follow, and moves the cursor to a target before it clicks.
export interface Pace {
  caption: number; // how long a caption shows before its step, in ms
  settle: number; // how long a step's result stays on screen afterwards, in ms
  typeDelay: number; // ms between typed keys
  moveSteps: number; // mouse moves on the way to a click
  perWord: number; // ms a caption stays up for each of its words, beyond the caption time
}

export const testPace: Pace = { caption: 0, settle: 0, typeDelay: 0, moveSteps: 1, perWord: 0 };
export const viewerPace: Pace = { caption: 1800, settle: 1600, typeDelay: 140, moveSteps: 40, perWord: 260 };

// readingTime is how long a caption stays up before its step, at least the pace's caption time and
// longer for a caption with more to read.
export const readingTime = (pace: Pace, text: string): number =>
  pace.caption > 0 ? Math.max(pace.caption, text.split(/\s+/).length * pace.perWord) : 0;

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
  // caption is what the video shows before the step, the same on every design or written for one.
  caption: string | ((d: Design) => string);
  // skip, when it says so, leaves the step out on this design, as for a kind of change it lacks.
  skip?: (d: Design) => boolean;
  // run acts, waits for the viewer to answer, and throws when the answer is wrong.
  run: (r: Reader, d: Design) => Promise<void>;
}

// captionOf is the text a step shows on a design.
export const captionOf = (s: Step, d: Design): string => (typeof s.caption === "string" ? s.caption : s.caption(d));

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

// spanCentred waits for the box around every element el matches to come to the middle third of a
// host. A net drawn in pieces, as auto-layout draws one, is revealed by the box around all of them.
async function spanCentred(page: Page, el: string, host: string, what: string): Promise<void> {
  await page
    .waitForFunction(
      ({ el, host }) => {
        const rs = [...document.querySelectorAll(el)].map((e) => e.getBoundingClientRect());
        const h = document.querySelector(host)?.getBoundingClientRect();
        if (rs.length === 0 || !h) return false;
        const left = Math.min(...rs.map((r) => r.left)), right = Math.max(...rs.map((r) => r.right));
        const top = Math.min(...rs.map((r) => r.top)), bottom = Math.max(...rs.map((r) => r.bottom));
        const cx = (left + right) / 2, cy = (top + bottom) / 2;
        return cx > h.left + h.width / 3 && cx < h.right - h.width / 3 && cy > h.top + h.height / 3 && cy < h.bottom - h.height / 3;
      },
      { el, host },
      { timeout: 30_000 },
    )
    .catch(() => {
      throw new Error(`${what} did not come to the middle of the view`);
    });
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
    title: "What changed between two revisions",
    gate: "gateway",
    steps: [
      {
        caption: (d) => `Open ${revision(d).from} of ${d.label}`,
        run: async (r, d) => {
          await r.page.goto(`${r.base}/designs/${d.mount}/${d.path}/view?mode=svg`, { waitUntil: "networkidle" });
          await r.page.waitForSelector(`${SHEET} svg`, { timeout: 60_000 });
        },
      },
      {
        caption: (d) =>
          `Compare it with ${revision(d).to}. agni pairs parts by their reference and nets by their connections, so it compares the circuits, not the drawings`,
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
        caption: "The legend counts each kind of change, and the list beside the two drawings groups them",
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
      changeStep("added", "b", "component", (c, e) => `Added: a part only ${c.to} has. ${e.subject} is ${e.note}`),
      changeStep("removed", "a", "component", (c, e) => `Removed: a part only ${c.from} has, so it shows on the old drawing. ${e.subject} is gone ${e.note}`),
      changeStep("changed", "b", "component", (_c, e) => `Changed: the same reference with different properties. ${e.subject} ${e.note}`),
      changeStep("newNet", "b", "net", (c, e) => `A new net: connections only ${c.to} makes. ${e.subject} ${e.note}`),
      changeStep("deletedNet", "a", "net", (c, e) => `A deleted net: connections ${c.to} no longer makes. ${e.subject} ${e.note}`),
      changeStep(
        "renamed",
        "b",
        "net",
        (_c, e) =>
          `Renamed: the same pins under a new name. ${(e as Example & { old: string }).old} became ${e.subject}, and agni matched them by their connections, so it's a rename rather than a delete and an add`,
      ),
      changeStep("hard", "b", "net", (_c, e) => `A hard change: the net keeps its name and its connections change. ${e.subject} ${e.note}`),
      {
        caption: "A soft change keeps every connection and changes only a net's class or attributes. This revision has none, so the legend leaves it out",
        run: async (r) => {
          if ((await r.page.locator(".diff-legend .diff-chip", { hasText: /soft/ }).count()) > 0) throw new Error("the legend counts soft changes the caption says aren't there");
        },
      },
      {
        caption: "Close the comparison, and the list tabs back beside the drawing",
        run: async (r) => {
          await clickOn(r, r.page.locator(".diff-close"));
          await r.page.waitForFunction(() => {
            const hole = document.querySelector('[data-dock-panel="canvas"]');
            return !!hole && hole.getBoundingClientRect().width > 0 && getComputedStyle(hole).visibility !== "hidden";
          }, null, { timeout: 30_000 });
        },
      },
    ],
  },
  {
    name: "walk-a-signal",
    title: "Follow a signal through the circuit",
    steps: [
      {
        caption: "Open the schematic",
        run: async (r, d) => {
          await r.page.goto(`${r.base}/designs/${d.mount}/${d.path}/view?mode=svg`, { waitUntil: "networkidle" });
          await r.page.waitForSelector(`${SHEET} svg`, { timeout: 60_000 });
        },
      },
      {
        caption: (d) => `Find ${walkOf(d).start}, ${walkOf(d).about}`,
        run: async (r, d) => {
          const w = walkOf(d);
          await r.press("Control+f");
          await r.page.waitForSelector("input.query-term", { state: "visible", timeout: 10_000 });
          await r.type(w.start);
          await r.press("Enter");
          const name = r.page.locator(".query-row .query-locate", { hasText: new RegExp(`^${w.start}$`) }).first();
          await name.waitFor({ timeout: 30_000 });
          await clickOn(r, name);
          await onSheet(r.page, w.sheet);
          await centred(r.page, `${SHEET} [data-kind="component"][data-ref="${w.start}"]`, SHEET, `${w.start} on the schematic`);
        },
      },
      {
        caption: "Walk outwards from it through small parts. The pins it leaves by light up, and so does each net the walk reaches",
        run: async (r, d) => {
          const w = walkOf(d);
          await clickOn(r, r.page.locator('[data-component="properties"] [data-walk-kind="small-parts"]'));
          await walked(r.page, w.start, 1);
          await r.page.locator(`.walk-net[data-walk-net="${w.first}"]`).waitFor({ state: "attached", timeout: 30_000 });
        },
      },
      {
        caption: (d) => `The walk stops at ${walkOf(d).stops}. A rail reaches nearly every part, so the Trace tab says it ends there and leaves it unlit`,
        skip: (d) => !walkOf(d).stops,
        run: async (r, d) => {
          const row = r.page.locator(`.walk-net[data-walk-net="${walkOf(d).stops}"]`);
          const said = (await row.locator(".walk-ended").textContent({ timeout: 30_000 })) ?? "";
          if (!said.includes("ends here")) throw new Error(`the Trace tab does not say the walk ends at ${walkOf(d).stops}`);
        },
      },
      {
        caption: (d) => `Go one hop wider: the walk ${walkOf(d).widerNote}`,
        run: async (r, d) => {
          const w = walkOf(d);
          await clickOn(r, r.page.locator(".walk-wider"));
          await walked(r.page, w.start, 2);
          await r.page.locator(`.walk-net[data-walk-net="${w.wider}"][data-hop="2"]`).waitFor({ state: "attached", timeout: 30_000 });
        },
      },
      {
        caption: (d) => `Ask which rail the line is pulled to. The route runs through ${walkOf(d).start} to ${walkOf(d).rail}`,
        skip: (d) => !walkOf(d).rail,
        run: async (r, d) => {
          const w = walkOf(d);
          await clickOn(r, r.page.locator('[data-action="walk-goal"][data-goal="rail"]'));
          await r.page.waitForFunction(() => !document.querySelector(".walk-loading") && document.querySelector(".walk-goal[data-goal-outcome]"), null, { timeout: 30_000 });
          const outcome = await r.page.locator(".walk-goal").getAttribute("data-goal-outcome");
          if (outcome !== "routed") throw new Error(`the rail goal answered ${outcome}, not routed`);
          const route = (await r.page.locator(".walk-route-line").first().textContent()) ?? "";
          if (!route.includes(w.start) || !route.includes(w.rail!)) throw new Error(`the route is ${route}, not through ${w.start} to ${w.rail}`);
        },
      },
    ],
  },
];

// walkOf returns the design's walk, which the walk journey needs.
function walkOf(d: Design): Walk {
  if (!d.walk) throw new Error(`${d.label} has no part to walk from`);
  return d.walk;
}

// walked waits for the Trace tab to show a finished walk from start of the given hops.
async function walked(page: Page, start: string, hops: number): Promise<void> {
  const title = `Walk from ${start}, small-parts, ${hops} hop${hops === 1 ? "" : "s"}`;
  await page
    .waitForFunction((t) => !document.querySelector(".walk-loading") && (document.querySelector(".walk-title")?.textContent ?? "").replace(/\s+/g, " ").trim() === t, title, { timeout: 60_000 })
    .catch(async () => {
      throw new Error(`the Trace tab shows "${(await page.locator(".walk-title").textContent().catch(() => "")) ?? ""}", not "${title}"`);
    });
}

// changeStep is the compare journey's step for one kind of change. It clicks the design's example of
// that kind in the Changes list, and checks that the drawing on the side that holds it, the old one
// for a removal or a deleted net, centres it, and that the row stays in view. A design with no
// example of the kind skips the step.
function changeStep(
  kind: "added" | "removed" | "changed" | "newNet" | "deletedNet" | "renamed" | "hard",
  side: "a" | "b",
  what: "component" | "net",
  say: (c: Compare, e: Example) => string,
): Step {
  const example = (d: Design): Example | undefined => d.compare?.[kind];
  return {
    caption: (d) => say(revision(d), example(d)!),
    skip: (d) => !example(d),
    run: async (r, d) => {
      const e = example(d)!;
      await clickOn(r, change(r.page, e.subject));
      await stillInList(r.page, e.subject);
      const where = `${e.subject} in the ${side === "a" ? "old" : "new"} revision`;
      if (what === "component") await centred(r.page, `#diff-svg-${side} [data-kind="component"][data-ref="${e.subject}"]`, `#diff-svg-${side}`, where);
      else await spanCentred(r.page, `#diff-svg-${side} [data-kind="net"][data-net="${e.subject}"]`, `#diff-svg-${side}`, where);
    },
  };
}

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
