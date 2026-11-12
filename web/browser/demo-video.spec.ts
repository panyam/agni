// The demo journeys (agni issue 1545, mission #1442). In the gate, every journey in journeys.ts plays
// at test pace and checks each step, on the cut of Jetson unless the journey names another design, so
// a journey that breaks fails here and names the step it broke at. `make demo-video JOURNEY=<name>` sets DEMO_OUT and plays one journey on
// Jetson's whole board, paced for a viewer, and records it as <name>.mp4 at 1080p.
//
// The recording shows a title card, then a caption before each step and a cursor that moves to what
// it clicks. Both are drawn into the recording page by an init script, so the viewer itself carries no
// demo code. Playwright records from the moment the page exists, so the video is cut to start at the
// title card.

import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import type { Browser, Page } from "playwright-core";
import { launch } from "./browser.js";
import { type Design, type Journey, type Pace, type Reader, designs, journeyNames, journeys, testPace, viewerPace } from "./journeys.js";

const base = (): string => inject("baseUrl");
const out = process.env.DEMO_OUT ?? "";
const only = process.env.JOURNEY ?? "";
const recordOn = process.env.DEMO_DESIGN ?? "jetson";

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

// play runs a journey's steps in order. A step that throws stops the journey with its number and
// caption, so a broken journey names where it broke.
async function play(page: Page, j: Journey, d: Design, pace: Pace, caption: (text: string) => Promise<void>): Promise<void> {
  const r = reader(page, pace);
  for (const [i, s] of j.steps.entries()) {
    await caption(s.caption);
    if (pace.caption > 0) await page.waitForTimeout(pace.caption);
    try {
      await s.run(r, d);
    } catch (e) {
      throw new Error(`${j.name} broke at step ${i + 1}, "${s.caption}": ${(e instanceof Error ? e.message : String(e)).split("\n")[0]}`);
    }
    // A step that loaded a page lost the caption with the old document, so it is shown again.
    await caption(s.caption);
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
    document.documentElement.append(bar, dot);
    document.addEventListener("mousemove", (e) => style(dot, { left: e.clientX + "px", top: e.clientY + "px" }), true);
    document.addEventListener("mousedown", () => style(dot, { transform: "scale(0.7)" }), true);
    document.addEventListener("mouseup", () => style(dot, { transform: "scale(1)" }), true);
    window.__demoCaption = (text) => { bar.textContent = text; bar.style.opacity = text ? "1" : "0"; };
  };
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", add); else add();
})();
`;

function titleCard(title: string, about: string): string {
  return `<!doctype html><html><body style="margin:0;height:100vh;display:flex;flex-direction:column;align-items:center;justify-content:center;background:#141821;color:#fff;font-family:-apple-system,'Segoe UI',Helvetica,Arial,sans-serif">
<div style="font-size:64px;font-weight:600">${title}</div><div style="font-size:30px;margin-top:20px;opacity:0.75">agni, on ${about}</div></body></html>`;
}

describe.runIf(out !== "")("a demo recording", () => {
  it(`records ${only || "a journey"} at 1080p`, async () => {
    const j = journeys.find((x) => x.name === only);
    if (!j) throw new Error(`JOURNEY=${only} names no journey; the journeys are ${journeyNames().join(", ")}`);
    const d = designs[recordOn];
    if (!d) throw new Error(`DEMO_DESIGN=${recordOn} names no design; use ${Object.keys(designs).join(", ")}`);
    mkdirSync(out, { recursive: true });
    const size = { width: 1920, height: 1080 };
    const began = Date.now();
    // A recording shows the view glide to each subject, as a reader without reduced motion sees it.
    const context = await browser.newContext({ viewport: size, recordVideo: { dir: out, size }, reducedMotion: "no-preference" });
    await context.addInitScript(OVERLAY);
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(String(e)));
    let cut = 0;
    try {
      await page.setContent(titleCard(j.title, d.label));
      cut = (Date.now() - began) / 1000;
      await page.waitForTimeout(2500);
      await play(page, j, d, viewerPace, async (text) => {
        await page.evaluate((t) => (window as unknown as { __demoCaption?: (t: string) => void }).__demoCaption?.(t), text);
      });
      await page.evaluate(() => (window as unknown as { __demoCaption?: (t: string) => void }).__demoCaption?.(""));
      await page.waitForTimeout(1200);
    } finally {
      await context.close();
    }
    const webm = await page.video()!.path();
    const mp4 = join(out, `${j.name}.mp4`);
    execFileSync("ffmpeg", ["-y", "-loglevel", "error", "-ss", cut.toFixed(2), "-i", webm, "-c:v", "libx264", "-preset", "slow", "-crf", "18", "-pix_fmt", "yuv420p", "-movflags", "+faststart", mp4]);
    const probed = execFileSync("ffprobe", ["-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=p=0", mp4]).toString().trim();
    expect(probed).toBe("1920,1080");
    expect(errors).toEqual([]);
    console.log(`wrote ${mp4}`);
  }, 600_000);
});
