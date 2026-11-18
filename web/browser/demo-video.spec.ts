// The demo journeys (agni issue 1545, mission #1442). In the gate, every journey in journeys.ts plays
// at test pace and checks each step, on the cut of Jetson unless the journey names another design, so
// a journey that breaks fails here and names the step it broke at.
//
// `make try JOURNEY=<name>` sets TRY and plays one journey at a pace a viewer can follow, with a
// caption before each step and a cursor that moves to what it clicks (agni issues 1545 and 1600).
// Both are drawn into the page by an init script, so the viewer itself carries no demo code.
// - Headed, the default, the browser stays open for the reviewer when the journey ends, or stops
//   before the step UNTIL names, until they close its window. STEP=1 puts a Next button on the
//   caption, and each step waits for it. RECORD=1 records the window, the reviewer's own clicks
//   included, as DEMO_OUT/<name>-try.mp4 once it closes, and HANDOVER=0 closes it when the journey
//   ends.
// - HEADED=0 records the journey as DEMO_OUT/<name>.mp4 at 1080p after a title card, for the docsite
//   and release notes. Playwright records from the moment the page exists, so the video is cut to
//   start at the title card.

import { execFileSync } from "node:child_process";
import { mkdirSync, rmSync } from "node:fs";
import { join } from "node:path";
import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import type { Browser, Page } from "playwright-core";
import { headed, launch } from "./browser.js";
import { type Design, type Journey, type Pace, type Reader, captionOf, designs, journeyNames, journeys, readingTime, testPace, viewerPace } from "./journeys.js";
import { playedSteps, stepsBefore } from "./trysteps.js";

const base = (): string => inject("baseUrl");
const out = process.env.DEMO_OUT || "/tmp/agni-demo";
const only = process.env.JOURNEY ?? "";
const recordOn = process.env.DEMO_DESIGN ?? "jetson";
const trying = process.env.TRY === "1";

let browser: Browser;
beforeAll(async () => {
  browser = await launch();
}, 180_000);
afterAll(async () => {
  await browser?.close();
});

function reader(page: Page, pace: Pace): Reader {
  const wait = (ms: number) => (ms > 0 ? page.waitForTimeout(ms) : Promise.resolve());
  return {
    page,
    base: base(),
    pace,
    async click(x, y) {
      await page.mouse.move(x, y, { steps: pace.moveSteps });
      await wait(pace.typeDelay * 2);
      await page.mouse.click(x, y);
    },
    async type(text) {
      await page.keyboard.type(text, { delay: pace.typeDelay });
    },
    async press(key) {
      await wait(pace.typeDelay * 2);
      await page.keyboard.press(key);
    },
  };
}

// Stops says where a tried journey stops and waits. stop is how many of the played steps run, and
// next, when set, waits for the reviewer before each one.
interface Stops {
  stop?: number;
  next?: () => Promise<void>;
}

// play runs a journey's steps in order. A step that throws stops the journey with its number and
// caption, so a broken journey names where it broke.
async function play(page: Page, j: Journey, d: Design, pace: Pace, caption: (text: string) => Promise<void>, stops: Stops = {}): Promise<void> {
  const r = reader(page, pace);
  let played = 0;
  for (const [i, s] of j.steps.entries()) {
    if (s.skip?.(d)) continue;
    if (stops.stop !== undefined && played >= stops.stop) return;
    played++;
    const text = captionOf(s, d);
    await caption(text);
    await stops.next?.();
    const read = readingTime(pace, text);
    if (read > 0) await page.waitForTimeout(read);
    try {
      await s.run(r, d);
    } catch (e) {
      throw new Error(`${j.name} broke at step ${i + 1}, "${text}": ${(e instanceof Error ? e.message : String(e)).split("\n")[0]}`);
    }
    // A step that loaded a page lost the caption with the old document, so it is shown again.
    await caption(text);
    if (pace.settle > 0) await page.waitForTimeout(pace.settle);
  }
}

describe("the demo journeys, at test pace on the gate's designs", () => {
  // The mission asks that a broken journey fail with the step it broke at, so a video never goes
  // stale without anyone noticing.
  it("names the step a broken journey stopped at", async () => {
    const broken: Journey = {
      name: "broken",
      title: "Broken",
      steps: [
        { caption: "First", run: async () => {} },
        { caption: "Second", run: async () => { throw new Error("R0 is not in the answer"); } },
        { caption: "Third", run: async () => {} },
      ],
    };
    const shown: string[] = [];
    await expect(play({} as Page, broken, designs.cut, testPace, async (t) => void shown.push(t))).rejects.toThrow(
      'broken broke at step 2, "Second": R0 is not in the answer',
    );
    expect(shown).not.toContain("Third");
  });

  // make try stops before the step UNTIL names, and with STEP=1 waits for Next before each step.
  it("stops before the step it is told to, and waits before each step it plays", async () => {
    const ran: string[] = [];
    const steps = ["First", "Second", "Third"].map((caption) => ({ caption, run: async () => void ran.push(caption) }));
    const j: Journey = { name: "three", title: "Three", steps };
    let waited = 0;
    await play({} as Page, j, designs.cut, testPace, async () => {}, { stop: 2, next: async () => void waited++ });
    expect(ran).toEqual(["First", "Second"]);
    expect(waited).toBe(2);
    ran.length = 0;
    await play({} as Page, j, designs.cut, testPace, async () => {});
    expect(ran).toEqual(["First", "Second", "Third"]);
  });

  for (const j of journeys) {
    it(`plays ${j.name}`, async () => {
      const page = await browser.newPage({ viewport: { width: 1400, height: 1000 } });
      const errors: string[] = [];
      page.on("pageerror", (e) => errors.push(String(e)));
      try {
        const d = designs[j.gate ?? "cut"];
        if (!d) throw new Error(`${j.name} names no design ${j.gate}`);
        await play(page, j, d, testPace, async () => {});
      } finally {
        await page.close();
      }
      expect(errors).toEqual([]);
    }, 180_000);
  }
});

// OVERLAY draws the caption bar and the cursor into every document the recording page loads. Neither
// takes a pointer event, so a click lands where it would without them.
const OVERLAY = `
(() => {
  const style = (el, css) => Object.assign(el.style, css);
  const add = () => {
    if (document.getElementById("demo-caption")) return;
    const bar = document.createElement("div");
    bar.id = "demo-caption";
    style(bar, { position: "fixed", left: "50%", bottom: "48px", transform: "translateX(-50%)", maxWidth: "80%",
      padding: "14px 28px", borderRadius: "10px", background: "rgba(20,24,33,0.88)", color: "#fff",
      font: "500 28px/1.3 -apple-system, 'Segoe UI', Helvetica, Arial, sans-serif", textAlign: "center",
      zIndex: "2147483647", pointerEvents: "none", opacity: "0", transition: "opacity 0.35s ease" });
    const dot = document.createElement("div");
    dot.id = "demo-cursor";
    style(dot, { position: "fixed", left: "-40px", top: "-40px", width: "22px", height: "22px", marginLeft: "-11px",
      marginTop: "-11px", borderRadius: "50%", background: "rgba(255,196,0,0.55)", border: "2px solid rgba(20,24,33,0.85)",
      zIndex: "2147483647", pointerEvents: "none", transition: "transform 0.12s ease" });
    const next = document.createElement("button");
    next.id = "demo-next";
    next.textContent = "Next";
    style(next, { display: "none", marginLeft: "20px", padding: "6px 18px", borderRadius: "6px", border: "none",
      background: "#ffc400", color: "#141821", font: "600 22px/1.2 inherit", cursor: "pointer", pointerEvents: "auto",
      verticalAlign: "middle" });
    document.documentElement.append(bar, dot);
    document.addEventListener("mousemove", (e) => style(dot, { left: e.clientX + "px", top: e.clientY + "px" }), true);
    document.addEventListener("mousedown", () => style(dot, { transform: "scale(0.7)" }), true);
    document.addEventListener("mouseup", () => style(dot, { transform: "scale(1)" }), true);
    window.__demoCaption = (text) => { bar.textContent = text; bar.append(next); bar.style.opacity = text ? "1" : "0"; };
    // __demoNext shows the Next button and resolves when it is clicked, for make try's STEP=1.
    window.__demoNext = () => new Promise((done) => {
      next.style.display = "inline-block";
      next.onclick = () => { next.style.display = "none"; done(); };
    });
  };
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", add); else add();
})();
`;

function titleCard(title: string, about: string): string {
  return `<!doctype html><html><body style="margin:0;height:100vh;display:flex;flex-direction:column;align-items:center;justify-content:center;background:#141821;color:#fff;font-family:-apple-system,'Segoe UI',Helvetica,Arial,sans-serif">
<div style="font-size:64px;font-weight:600">${title}</div><div style="font-size:30px;margin-top:20px;opacity:0.75">agni, on ${about}</div></body></html>`;
}

// toMp4 turns a page's webm into an mp4 a chat or a release note can play, cut to start cutAt seconds
// in, and answers its size as ffprobe reads it, such as "1920,1080".
function toMp4(webm: string, mp4: string, cutAt: number): string {
  execFileSync("ffmpeg", ["-y", "-loglevel", "error", "-ss", cutAt.toFixed(2), "-i", webm, "-c:v", "libx264", "-preset", "slow", "-crf", "18", "-pix_fmt", "yuv420p", "-movflags", "+faststart", mp4]);
  return execFileSync("ffprobe", ["-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=p=0", mp4]).toString().trim();
}

// SVG_FIRST makes a page open in SVG, as the gate and the recording do in headless Chrome. The
// journeys check the board's SVG elements, and a visible Chrome on a hardware GPU would open the board
// in WebGL. The viewer chooses WebGL only when a context asked with failIfMajorPerformanceCaveat
// succeeds (web/src/rendercaps.ts), and nothing else asks that way. So refusing that one request
// changes the default and leaves the WebGL box working on the real GPU once the browser is handed over.
const SVG_FIRST = `
(() => {
  const getContext = HTMLCanvasElement.prototype.getContext;
  HTMLCanvasElement.prototype.getContext = function (kind, opts) {
    if (opts && opts.failIfMajorPerformanceCaveat) return null;
    return getContext.call(this, kind, opts);
  };
})();
`;

// A day, as the test timeout. A reviewer closes the window when they are done, and the run ends then.
const reviewerTime = 24 * 60 * 60 * 1000;

// The size of a recording made headless, for the docsite, the README and release notes.
const published = { width: 1920, height: 1080 };

describe.runIf(trying)("a journey to try or record", () => {
  it(`tries ${only || "a journey"}`, async () => {
    const j = journeys.find((x) => x.name === only);
    if (!j) throw new Error(`JOURNEY=${only} names no journey; the journeys are ${journeyNames().join(", ")}`);
    const d = designs[recordOn];
    if (!d) throw new Error(`DEMO_DESIGN=${recordOn} names no design; use ${Object.keys(designs).join(", ")}`);
    const steps = playedSteps(j, d);
    // Written straight to stdout, since vitest holds a test's console output until the test ends, and
    // this one ends when the reviewer closes the window.
    const say = (text: string) => void process.stdout.write(`${text}\n`);
    say(`${j.name} on ${d.label}, where UNTIL=<number> stops before a step:\n${steps.map((p) => `  ${p.n}. ${p.caption}`).join("\n")}`);
    // An unknown UNTIL fails here, before the browser opens a page.
    const stop = stepsBefore(j, d, process.env.UNTIL ?? "");
    // Headless there is no one to hand over to, so the run records the journey at the published size,
    // after a title card, and checks the page logged no error. Headed it records only on RECORD=1, at
    // the window's own size, which a first window measures. x264 wants each side even.
    const record = !headed || process.env.RECORD === "1";
    const handOver = headed && process.env.HANDOVER !== "0";
    let size: { width: number; height: number } | undefined = headed ? undefined : published;
    if (headed && record) {
      const probe = await browser.newContext({ viewport: null });
      const p = await probe.newPage();
      size = await p.evaluate(() => ({ width: innerWidth - (innerWidth % 2), height: innerHeight - (innerHeight % 2) }));
      await probe.close();
    }
    if (record) mkdirSync(out, { recursive: true });
    const began = Date.now();
    // The view glides to each subject, as a reader without reduced motion sees it.
    const context = await browser.newContext({
      viewport: headed ? null : published,
      reducedMotion: "no-preference",
      ...(record && size ? { recordVideo: { dir: out, size } } : {}),
    });
    await context.addInitScript(OVERLAY);
    await context.addInitScript(SVG_FIRST);
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(String(e)));
    const caption = (text: string) => page.evaluate((t) => (window as unknown as { __demoCaption?: (t: string) => void }).__demoCaption?.(t), text);
    const next =
      process.env.STEP === "1"
        ? () => page.evaluate(() => (window as unknown as { __demoNext?: () => Promise<void> }).__demoNext?.())
        : undefined;
    const closed = new Promise<void>((done) => {
      page.once("close", () => done());
      browser.once("disconnected", () => done());
    });
    // keep closes the window, then writes the recording from the cut on. It runs after a broken step
    // too, so the video shows where the journey broke.
    let cut = 0;
    const keep = async () => {
      await context.close().catch(() => {});
      if (!record) return;
      const mp4 = join(out, headed ? `${j.name}-try.mp4` : `${j.name}.mp4`);
      try {
        const webm = await page.video()!.path();
        const probed = toMp4(webm, mp4, cut);
        rmSync(webm, { force: true });
        if (!headed) expect(probed).toBe(`${published.width},${published.height}`);
        say(`wrote ${mp4} (${probed.replace(",", " by ")})`);
      } catch (e) {
        if (!headed) throw e;
        // Quitting Chrome, rather than closing its window, can stop the recording before it is written.
        say(`could not write ${mp4}: ${(e instanceof Error ? e.message : String(e)).split("\n")[0]}. Close the window rather than quitting Chrome.`);
      }
    };
    try {
      // A first visit reads the design on the Load page, out of the picture, so the journey opens the
      // viewer as a returning reader does.
      await page.goto(`${base()}/files/${d.mount}/${d.path}/view?mode=svg`, { waitUntil: "networkidle", timeout: 180_000 });
      await page.waitForSelector("#svg-view svg", { timeout: 180_000 });
      if (headed) {
        cut = (Date.now() - began) / 1000;
        await caption(`${j.title}, on ${d.label}`);
      } else {
        await page.setContent(titleCard(j.title, d.label));
        cut = (Date.now() - began) / 1000;
      }
      await page.waitForTimeout(2500);
      await play(page, j, d, viewerPace, caption, { stop, next });
    } catch (e) {
      const gone = page.isClosed();
      await keep();
      // A reviewer who closes the window part way through has finished trying, which is no failure.
      if (gone && headed) return;
      throw e;
    }
    const left = steps[stop];
    if (!handOver) {
      say(left ? `stopped before step ${left.n}, "${left.caption}"` : "played every step");
      await caption("");
      await page.waitForTimeout(1200);
      await keep();
      expect(errors).toEqual([]);
      return;
    }
    await caption(
      left
        ? `Stopped before step ${left.n} (UNTIL=${process.env.UNTIL}). The journey is paused, so the next step is yours: ${left.caption}. Close the window when you are done.`
        : "That was the whole journey. The viewer is yours. Close the window when you are done.",
    );
    say(
      (left ? `stopped before step ${left.n}, "${left.caption}"; ` : "played every step; ") +
        (record ? "recording until you close the window" : "close the window when you are done"),
    );
    await closed;
    await keep();
  }, reviewerTime);
});
