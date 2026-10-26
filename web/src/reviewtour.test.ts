// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { EventBus } from "@panyam/tsappkit";
import { reviewTourSteps, startReviewTour, visible, type TourHandle } from "./reviewtour.js";
import { viewerEmitter, viewerEventNames } from "./viewerevents.js";

const host = { show: () => {}, selectArea: () => {} };

// box gives an element a fixed client box, since jsdom has no layout.
function box(el: Element, x: number, y: number, width: number, height: number): void {
  el.getBoundingClientRect = () => ({ x, y, width, height, left: x, top: y, right: x + width, bottom: y + height, toJSON: () => ({}) }) as DOMRect;
}

afterEach(() => {
  document.body.replaceChildren();
});

// A step can only advance on an event the viewer reports, so each one the tour waits on has to be
// emitted somewhere in the app. Without this a renamed or dropped emit leaves a step that never
// advances, which only the browser walk would show.
describe("every review tour step is tourable", () => {
  const src = readdirSync(__dirname)
    .filter((f) => /\.(ts|tsx)$/.test(f) && !/\.test\./.test(f) && f !== "viewerevents.ts" && f !== "reviewtour.ts")
    .map((f) => readFileSync(join(__dirname, f), "utf8"))
    .join("\n");
  const emitted = (name: string) => new RegExp(`(emit|events)\\??\\.?\\(\\s*"${name.replace(".", "\\.")}"`).test(src);

  it("finds the emits it looks for", () => {
    // Positive control: a pattern matching nothing would pass every step below.
    expect(emitted("checks.ran")).toBe(true);
    expect(emitted("no.such.event")).toBe(false);
  });

  for (const step of reviewTourSteps(host)) {
    const on = step.advanceOn;
    it(`${step.id} advances on an event the viewer emits`, () => {
      expect(on && "event" in on, `${step.id} should advance on an app event, not a DOM one`).toBe(true);
      expect(emitted((on as { event: string }).event), `nothing in web/src emits ${(on as { event: string }).event}`).toBe(true);
    });
  }

  it("emits every event it declares", () => {
    expect(viewerEventNames.filter((n) => !emitted(n))).toEqual([]);
  });
});

describe("visible", () => {
  it("clips an element to its scrolling ancestors and drops one scrolled away", () => {
    const list = document.createElement("div");
    list.style.overflowY = "auto";
    const shown = document.createElement("div");
    const half = document.createElement("div");
    const gone = document.createElement("div");
    list.append(shown, half, gone);
    document.body.append(list);
    box(list, 0, 0, 100, 100);
    box(shown, 0, 10, 100, 20);
    box(half, 0, 90, 100, 20);
    box(gone, 0, 150, 100, 20);
    expect(visible([shown, half, gone])).toEqual([
      { x: 0, y: 10, width: 100, height: 20 },
      { x: 0, y: 90, width: 100, height: 10 },
    ]);
  });
});

describe("the ask step", () => {
  it("offers the Query panel's Ask button and Properties' Ask rows as separate regions", () => {
    const query = document.createElement("button");
    query.className = "query-ask";
    const rows = [0, 1].map(() => {
      const b = document.createElement("button");
      b.className = "pr-ask-go";
      return b;
    });
    document.body.append(query, ...rows);
    box(query, 900, 860, 150, 24);
    box(rows[0], 10, 900, 180, 30);
    box(rows[1], 10, 932, 180, 30);
    const ask = reviewTourSteps(host).find((s) => s.id === "ask")!;
    const rects = [...(ask.target!() as Iterable<{ x: number }>)];
    expect(rects.map((r) => r.x).sort((a, b) => a - b)).toEqual([10, 10, 900]);
  });
});

describe("startReviewTour, paced by its driver", () => {
  const start = (): { bus: EventBus; handle: TourHandle } => {
    const bus = new EventBus();
    const handle = startReviewTour(bus, host, "host");
    return { bus, handle };
  };
  const root = () => document.querySelector<HTMLElement>(".tsappkit-tour");

  it("holds each step until released, and advances on the viewer's event", () => {
    const { bus, handle } = start();
    expect(handle.step).toBe("run-checklist");
    expect(root()?.style.visibility).toBe("hidden");
    handle.release();
    expect(root()?.style.visibility).toBe("");

    const emit = viewerEmitter(bus);
    emit("checklist.ran", { checklist: "bring-up", ok: false });
    expect(handle.step, "a failed run does not finish the step").toBe("run-checklist");
    emit("checklist.ran", { checklist: "bring-up", ok: true });
    expect(handle.step).toBe("run-checks");
    expect(root()?.style.visibility, "the next step waits for its release").toBe("hidden");
    expect(handle.events.map((e) => e.type)).toEqual(["checklist.ran", "checklist.ran"]);
  });

  it("records the reader's first press of a released step only", () => {
    const { handle } = start();
    document.body.dispatchEvent(new Event("pointerdown", { bubbles: true }));
    expect(handle.pressed["run-checklist"], "a held step takes no press").toBeUndefined();
    handle.release();
    document.body.dispatchEvent(new Event("pointerdown", { bubbles: true }));
    const first = handle.pressed["run-checklist"];
    expect(first).toBeGreaterThan(0);
    document.body.dispatchEvent(new Event("pointerdown", { bubbles: true }));
    expect(handle.pressed["run-checklist"]).toBe(first);
  });

  // The steps before the answer, finished in order, so a test can start at click-cell.
  const toClickCell = (bus: EventBus): void => {
    const emit = viewerEmitter(bus);
    emit("checklist.ran", { checklist: "bring-up", ok: true });
    emit("checks.ran", { findings: 3, cancelled: false });
    emit("finding.selected", { rule: "r", subject: "SDA" });
    emit("queryset.opened", { title: "Netlist audit", queries: 9 });
  };

  it("advances past the answer on a part", () => {
    const { bus, handle } = start();
    toClickCell(bus);
    expect(handle.step).toBe("click-cell");
    viewerEmitter(bus)("cell.located", { kind: "component", subject: "U3" });
    expect(handle.step).toBe("ask");
    expect(handle.ended).toBe("");
  });

  it("ends the tour when the reader clicks a net where the step asks for a part", () => {
    const { bus, handle } = start();
    toClickCell(bus);
    viewerEmitter(bus)("cell.located", { kind: "net", subject: "GND" });
    expect(handle.ended).toBe("offPath");
    expect(handle.step, "the tour stopped on the step the reader left").toBe("click-cell");
    expect(root(), "the overlay is gone, so the viewer carries on as usual").toBeNull();
  });

  it("ends when the reader presses Esc", () => {
    const { handle } = start();
    handle.release();
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    expect(handle.ended).toBe("cancelled");
    expect(root()).toBeNull();
  });
});
