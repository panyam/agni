// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { ACCEPTED, ENVELOPE } from "./embed.js";

// The script every saved HTML report carries (core/report/viewerlink.js), run in a page holding one
// report link, against a stand-in for the viewer window it finds by name.
const SCRIPT = readFileSync(join(process.cwd(), "..", "core", "report", "viewerlink.js"), "utf8");
const VIEWER = "http://127.0.0.1:8080/designs/kicad/d.kicad_sch/view";
const LINK = VIEWER + "?verdict=x&rule=r";

type Posted = { agni: string; id: string; method: string };

function viewerWindow() {
  const posted: Posted[] = [];
  return { posted, location: { href: VIEWER }, postMessage: (m: Posted) => posted.push(m), focus: () => {} };
}

function fromViewer(w: object, data: unknown) {
  const ev = new MessageEvent("message", { data });
  Object.defineProperty(ev, "source", { value: w });
  window.dispatchEvent(ev);
}

describe("a report link beside an open viewer", () => {
  beforeAll(() => {
    document.body.innerHTML = `<a target="agni-viewer" href="${LINK}">R2</a>`;
    new Function(SCRIPT)();
  });
  afterEach(() => vi.useRealTimers());

  function click(w: ReturnType<typeof viewerWindow>) {
    vi.spyOn(window, "open").mockReturnValue(w as unknown as Window);
    document.querySelector("a")!.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, button: 0 }));
    expect(w.posted).toHaveLength(1);
    expect(w.posted[0]).toMatchObject({ agni: ENVELOPE, method: "ApplyLocation" });
    return w.posted[0].id;
  }

  it("loads the link in the viewer's window when nothing there answers", () => {
    vi.useFakeTimers();
    const w = viewerWindow();
    click(w);
    vi.advanceTimersByTime(2500);
    expect(w.location.href).toBe(LINK);
  });

  // A viewer resolving a verdict link runs its rule before it answers, which on a slow machine takes
  // longer than the first wait, so the viewer says it has the request and the link is not loaded over
  // it (agni issue 1271's CI).
  it("waits for a viewer that has taken the request, however long its answer takes", () => {
    vi.useFakeTimers();
    const w = viewerWindow();
    const id = click(w);
    fromViewer(w, { agni: ACCEPTED, id });
    vi.advanceTimersByTime(10_000);
    expect(w.location.href).toBe(VIEWER);
    fromViewer(w, { agni: ENVELOPE, id, ok: true, body: {} });
    vi.advanceTimersByTime(120_000);
    expect(w.location.href).toBe(VIEWER);
  });

  it("loads the link when the viewer answers that it could not apply it", () => {
    vi.useFakeTimers();
    const w = viewerWindow();
    const id = click(w);
    fromViewer(w, { agni: ACCEPTED, id });
    fromViewer(w, { agni: ENVELOPE, id, ok: false, error: { code: "not_found", message: "no design" } });
    expect(w.location.href).toBe(LINK);
  });
});
