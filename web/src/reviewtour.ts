// The review tour: a reader's first review of a design, taken in the viewer itself, over tsappkit's
// Tour (panyam/goapplib#128). Each step dims the page, leaves the controls the step needs reachable,
// and advances on the viewer's own event for the action (viewerevents.ts), never on a click it
// infers. `?tour=review` starts it. The browser-review walk takes the same tour (web/browser), so the
// gate holds the steps to the viewer they describe.

import { Tour, type TourEventSource, type TourRect, type TourStep } from "@panyam/tsappkit";
import { type ViewerEvents, viewerEventNames } from "./viewerevents.js";

// ReviewTourHost is what the tour needs from the page beyond its DOM: a dock panel brought forward
// for each step, and an area selected for the area step's Show me, which a click cannot make.
export interface ReviewTourHost {
  show: (panel: string) => void;
  selectArea: () => void;
}

const one = (sel: string): HTMLElement | null => document.querySelector<HTMLElement>(sel);
const every = (sel: string): HTMLElement[] => [...document.querySelectorAll<HTMLElement>(sel)];
const click = (sel: string) => () => one(sel)?.click();

// A step whose reader may pick any row of a list targets the list rather than each row. The tour
// re-reads its targets every frame, and on a large design a hole per row cost the page seconds while
// it located the row the reader clicked.
//
// visible is the part of each element its scrolling ancestors show. A finding scrolled out of the
// Checks list still has a box, over whatever panel sits there, and a hole cut there would hand that
// panel's clicks through, so a hidden element contributes nothing.
export function visible(els: Iterable<Element>): TourRect[] {
  const out: TourRect[] = [];
  for (const el of els) {
    let { left, top, right, bottom } = el.getBoundingClientRect();
    for (let p = el.parentElement; p && right > left && bottom > top; p = p.parentElement) {
      const o = getComputedStyle(p);
      if (o.overflowX === "visible" && o.overflowY === "visible") continue;
      const r = p.getBoundingClientRect();
      left = Math.max(left, r.left);
      top = Math.max(top, r.top);
      right = Math.min(right, r.right);
      bottom = Math.min(bottom, r.bottom);
    }
    if (right > left && bottom > top) out.push({ x: left, y: top, width: right - left, height: bottom - top });
  }
  return out;
}

// panels names the dock panel each step works in.
const panels: Record<string, string> = {
  "run-checklist": "review",
  "run-checks": "checks",
  "click-finding": "checks",
  "open-set": "query",
  "click-cell": "query",
  ask: "query",
  "select-area": "canvas",
  "ask-area": "properties",
  "save-report": "review",
};

// reveals names, for a step whose control can sit below its panel's fold, the element to scroll into
// view when the step goes live. The dimmed page takes no wheel, so a control off screen leaves the
// reader no way to reach it.
const askConnect = () => {
  const asks = every(".pr-ask-go");
  return asks.find((b) => /connect to/.test(b.textContent ?? "")) ?? asks[0] ?? null;
};
const reveals: Record<string, () => HTMLElement | null> = {
  "run-checklist": () => one("button.rv-run"),
  "click-finding": () => one(".check-locate"),
  "click-cell": () => one(".query-row"),
  "ask-area": askConnect,
  "save-report": () => one(".rv-save"),
};

// reveal brings a step's panel forward and its control into view, a frame later so a panel the dock
// just activated has laid out.
function reveal(host: ReviewTourHost, step: string): void {
  host.show(panels[step] ?? "");
  requestAnimationFrame(() => reveals[step]?.()?.scrollIntoView({ block: "nearest", inline: "nearest" }));
}

export function reviewTourSteps(host: ReviewTourHost): TourStep<ViewerEvents>[] {
  return [
    {
      id: "run-checklist",
      title: "Run a checklist",
      text:
        "Pick one of the project's checklists in the Review panel and press Run review. Each item is a question " +
        "bound to a rule, a query or a manual check, and the engine scores it: pass, fail, provisional, or " +
        "not automated.",
      target: () => visible(every("select.rv-checklist, button.rv-run")),
      advanceOn: { event: "checklist.ran", where: (d) => d.ok },
      doItForMe: click("button.rv-run"),
    },
    {
      id: "run-checks",
      title: "Run the checks",
      text:
        "Press Run checks to run every rule in the catalog over the design. The findings list fills, and " +
        "nothing lights on the drawing until you choose a finding to look at.",
      target: () => visible(every(".checks-run")),
      advanceOn: { event: "checks.ran", where: (d) => !d.cancelled },
      doItForMe: click(".checks-run"),
    },
    {
      id: "click-finding",
      title: "Look at a finding",
      text: "Click any finding. The viewer locates its subjects (nets, parts, pins) on the sheet and lights them.",
      target: () => visible(every(".checks-table")),
      advanceOn: { event: "finding.selected" },
      doItForMe: click(".check-locate"),
    },
    {
      id: "open-set",
      title: "Open a query set",
      text:
        "Open the Sets menu and choose a set, one the project keeps or a file of your own. Each query in it " +
        "opens in a tab of its own, all answered over one read of the design.",
      target: () => visible(every(".query-sets-open, .query-sets-menu")),
      advanceOn: { event: "queryset.opened" },
      doItForMe: () => {
        if (!one(".query-sets-menu")) one(".query-sets-open")?.click();
        (one(".query-set-item") ?? one(".query-set-file"))?.click();
      },
    },
    {
      id: "click-cell",
      title: "Click into an answer",
      text:
        "Click a part in the answer. A query answer carries the entity behind each cell, so the click " +
        "lights it on the drawing, and a part drawn on another sheet offers a chip that goes there.",
      target: () => visible(every(".query-results")),
      advanceOn: { event: "cell.located" },
      doItForMe: click(".query-row .query-locate"),
    },
    {
      id: "ask",
      title: "Ask a follow-up",
      text: "Ask follows up on the part you picked with a served question, which opens in a tab of its own.",
      target: () => visible(every(".query-ask:not([disabled])")),
      advanceOn: { event: "query.asked" },
      doItForMe: click(".query-ask:not([disabled])"),
    },
    {
      id: "select-area",
      title: "Select a block",
      text:
        "Hold Shift and drag a rectangle around a block of the sheet. Properties then describes the parts, " +
        "pins and wires inside as one area, with the nets that cross its edge as its interface.",
      target: () => visible(every("#svg-view")),
      advanceOn: { event: "area.selected", where: (d) => d.components > 0 },
      doItForMe: host.selectArea,
    },
    {
      id: "ask-area",
      title: "Ask about the block",
      text: "Ask what the block connects to. The question binds every part in the area at once.",
      target: () => {
        const ask = askConnect();
        return ask ? visible([ask]) : null;
      },
      advanceOn: { event: "query.asked" },
      doItForMe: () => askConnect()?.click(),
    },
    {
      id: "save-report",
      title: "Save the checklist page",
      text:
        "Save the checklist's page. The viewer asks the engine for the page 'agni review --format html' " +
        "writes, links back to this viewer included.",
      target: () => visible(every(".rv-save")),
      advanceOn: { event: "report.saved" },
      doItForMe: click(".rv-save"),
    },
  ];
}

// TourHandle is what `?tour=review` leaves on the page for a driver to follow, the browser-review
// walk among them. With pace set to "host" the page shows a step only when the driver releases it,
// so the walk can check one step's result before the reader starts the next; a step the walk has
// not released yet is hidden and takes no input.
export interface TourHandle {
  step: string;
  index: number;
  // ended is "" while the tour runs, then the reason it ended.
  ended: string;
  // pressed is when the reader first pressed a pointer during each step on show, by step id, so a
  // driver times a step from the reader's action rather than from when it was offered.
  pressed: Record<string, number>;
  // events is every viewer event since the tour started, so a driver can read which finding, cell
  // or checklist the reader chose.
  events: { type: string; data: unknown }[];
  release: () => void;
}

export function startReviewTour(bus: TourEventSource, host: ReviewTourHost, pace: "self" | "host"): TourHandle {
  const handle: TourHandle = { step: "", index: -1, ended: "", pressed: {}, events: [], release: () => {} };
  const root = () => one(".tsappkit-tour");
  const shown = () => !!root() && root()!.style.visibility !== "hidden";
  // A step the driver has not released measures nothing, since the tour re-reads every target each
  // frame and a long findings list would cost the page that time for the whole walk.
  const steps = reviewTourSteps(host).map((s) => ({ ...s, target: s.target && (() => (shown() ? s.target!() : null)) }));
  const tour = new Tour<ViewerEvents>(steps, {
    onStep: (step, index) => {
      handle.step = step.id;
      handle.index = index;
      if (pace === "host") root()!.style.visibility = "hidden";
      else reveal(host, step.id);
    },
    onEnd: (reason) => {
      handle.ended = reason;
      document.removeEventListener("pointerdown", pressed, true);
    },
  });
  handle.release = () => {
    const r = root();
    if (r) r.style.visibility = "";
    if (handle.step) reveal(host, handle.step);
  };
  const pressed = () => {
    if (handle.step && shown()) handle.pressed[handle.step] ??= Date.now();
  };
  document.addEventListener("pointerdown", pressed, true);
  for (const type of viewerEventNames) bus.on(type, (data?: unknown) => handle.events.push({ type, data }));
  tour.attach(bus);
  tour.start();
  return handle;
}
