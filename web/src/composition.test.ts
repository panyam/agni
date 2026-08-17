// @vitest-environment jsdom
import { describe, it, expect, beforeAll, vi } from "vitest";
import { readFileSync } from "node:fs";
import { loadRecents } from "./recents.js";
import { join } from "node:path";

// The composition root, under test at last (agni issue 136).
//
// Every other web test constructs its subject directly, with collaborators supplied. The one thing
// nothing constructed is `main.ts`: it resolves each island hole by id, builds each client, wires
// each callback, and assembles the presenter, and none of that ran under test. That is not an
// oversight in one PR, it is a structural blind spot, and it has now shipped the same bug twice.
//
//   WS9-052 (PR 135) shipped a review panel that was broken in the running app because main.ts never
//   passed the review clients to the presenter. Unit tests, presenter tests and typecheck all green.
//
//   Issue 175 (PR 182) shipped a project bar that rendered nothing because main.ts never constructed
//   one. `views.projectBar` was undefined, so `views.projectBar?.setState(...)` was a no-op on every
//   push. Again green, again found by opening the page (PR 184 fixed it).
//
// Both are the same shape: correct logic, correct tests, wired to nothing. Every test here therefore
// asserts the WIRING, and does it against the REAL page and the REAL entry point, because a
// hand-built DOM would be one more artifact that agrees with the code while the shipped page does
// not.
//
// This is the cheap rung of #136 and deliberately not the whole ticket. jsdom does not render, so
// nothing below asserts a pixel, a highlight, or an SVG. What it does assert is that the app boots,
// mounts what the page declares, and reaches every service on a real deep-link open. The flows that
// need a real browser are still open on the issue.

// pageBody is the real ViewerPage template's Body block. Reading the SHIPPED template is the point:
// the holes and their ids are the contract between the server-rendered page and main.ts, and a test
// carrying its own copy of the markup would keep passing after the page changed underneath it.
//
// The block is static HTML (the only `{{ }}` in the file are the define/end wrappers themselves), so
// slicing it out needs no template engine. If that stops being true this throws rather than quietly
// testing a fragment.
function pageBody(): string {
  const src = readFileSync(join(process.cwd(), "templates/ViewerPage.html"), "utf8");
  const m = /\{\{ define "Body" \}\}([\s\S]*?)\n\{\{ end \}\}/.exec(src);
  if (!m) throw new Error("ViewerPage.html has no Body block; the composition test cannot run");
  return m[1];
}

function readSrc(name: string): string {
  return readFileSync(join(process.cwd(), "src", name), "utf8");
}

// jsdom lacks two browser APIs the shell reaches for. Both are stubbed rather than worked around,
// because the alternative is not booting the real composition root, which is the entire exercise.
// Neither stub can hide a wiring bug: an island is mounted or it is not, an rpc is called or it is
// not.
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}

// The service replies the restore needs, keyed by rpc method name. They are the thinnest thing each
// caller accepts; this asserts that a call was MADE, never what came back.
const REPLIES: Record<string, unknown> = {
  ListMounts: { mounts: [{ name: "m" }] },
  ListDir: { entries: [] },
  GetDesign: { layout: "grid", sheets: [{ id: "s1", name: "Top" }], availableLayouts: ["grid"], nativeAvailable: false },
  GetSheet: { content: { case: "svg", value: "<svg/>" } },
  GetSheetSvg: { svg: "<svg/>" },
  ResolveDesign: {
    design: { name: "projects/demo/designs/board", entryUri: "mount://m/d/b.edn" },
    project: { name: "projects/demo", title: "Demo project", conventionsUri: "mount://m/conventions.yaml" },
  },
  ListRules: {
    rules: [{ name: "duplicate-ref-des", severity: "error", summary: "a designator claimed twice", available: true }],
  },
  CheckDesign: {
    findings: [
      {
        rule: "duplicate-ref-des",
        severity: "error",
        subject: { kind: "component", ref: "R1" },
        message: "ref-des claimed by 2 placements",
        sheets: ["s1"],
      },
    ],
  },
  GetLayoutReport: {},
  GetComponentParams: { components: [] },
  GetExpectations: { expectations: [] },
  ListReviews: { reviews: [] },
  ListRelations: {
    relations: [],
    examples: [],
    // Without these a click highlights and asks nothing, so their absence here would make the click
    // test below fail for the right reason.
    entityQueries: [{ kind: "net", query: 'component-on-net(?ref, "{net}") => ?ref', teaches: "join" }],
  },
};

const called: string[] = [];
const bootErrors: unknown[] = [];

beforeAll(async () => {
  (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = ResizeObserverStub;
  // WebGL is genuinely absent here. Returning null is what a browser without a context does, and the
  // shell handles it (the readout says so), which is why the boot still completes.
  HTMLCanvasElement.prototype.getContext = (() => null) as unknown as HTMLCanvasElement["getContext"];
  vi.spyOn(console, "error").mockImplementation((...args: unknown[]) => {
    bootErrors.push(args[0]);
  });
  (globalThis as unknown as { fetch: unknown }).fetch = async (input: unknown) => {
    const url = typeof input === "string" ? input : (input as Request).url;
    const method = url.split("/").pop() ?? "";
    called.push(method);
    return new Response(JSON.stringify(REPLIES[method] ?? {}), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  };

  // A DESIGN url, not the bare page. This is what makes the test exercise the restore loop rather
  // than only the mount: main.ts reads the URL at boot and replays it through presenter.restore, so
  // every client the open path needs gets used the way a deep-link refresh uses it.
  window.history.replaceState(null, "", "/designs/m/d/b.edn/view");
  document.body.innerHTML = pageBody();
  await import("./main.js");

  // The boot chain is async (initializeFromRoot, then the deep-link replay), so wait on the work
  // finishing rather than on a fixed delay.
  await vi.waitFor(() => {
    expect(called).toContain("GetDesign");
    expect(document.querySelector('[data-component="findings"]')?.children.length).toBeGreaterThan(0);
  }, 5000);
});

describe("the composition root boots", () => {
  it("raises nothing on the way up", () => {
    expect(bootErrors).toEqual([]);
  });

  // The readout is where main.ts reports a boot failure. An error there is exactly what a person
  // would have seen on opening the page, which is how both prior bugs were eventually found.
  it("leaves no error in the readout", () => {
    expect(document.getElementById("readout")?.textContent ?? "").not.toContain("error:");
  });
});

// This makes the template the declaration of record. Adding a panel means adding its hole here, and
// from that moment the hole is a claim the composition root has to satisfy.
describe("every island hole the page declares is mounted", () => {
  it("mounts something into each data-component hole", () => {
    const declared = [...pageBody().matchAll(/data-component="([^"]+)"/g)].map((m) => m[1]);
    expect(declared.length).toBeGreaterThan(10); // the page really was parsed, not an empty match

    const empty = declared.filter((name) => {
      const el = document.querySelector(`[data-component="${name}"]`);
      return !el || el.children.length === 0;
    });
    expect(empty, "island holes the composition root never filled").toEqual([]);
  });
});

// The hole test catches a panel whose island was never wired. It cannot catch a presenter VIEW PORT
// that was never wired, because an unwired port puts nothing on the page to notice the absence of.
// That is precisely how the project bar shipped invisible.
//
// ViewSink's own doc states the contract this enforces: "adding a panel is one field here and one
// line in the composition root". Nothing checked the second half.
//
// Reading the two files as source is unusual for a unit test and is the right shape here, for the
// same reason docsite/nav_test.go checks a new page reached all four of its registration points: the
// failure being prevented is an OMISSION ACROSS FILES, and only something holding both files can see
// an omission. There is no runtime state that distinguishes "deliberately absent" from "forgotten".
//
// The ports are optional in the interface by design, so a host embedding the presenter can leave a
// panel out (build/overlay.md). That is a statement about OTHER hosts. This app is the reference host
// and wires all of them, and "optional" is exactly what stopped the type checker from noticing.
describe("every presenter view port is wired by the composition root", () => {
  it("passes a view for each port ViewSink declares", () => {
    const block = /export interface ViewSink \{([\s\S]*?)\n\}/.exec(readSrc("viewer.ts"));
    expect(block, "ViewSink interface not found in viewer.ts").not.toBeNull();
    const ports = [...block![1].matchAll(/^ {2}(\w+)\??:/gm)].map((m) => m[1]);
    expect(ports.length).toBeGreaterThan(8); // the interface really was parsed

    const construction = /new ViewerPresenter\(([\s\S]*?)\n {4}\);/.exec(readSrc("main.ts"));
    expect(construction, "the ViewerPresenter construction was not found in main.ts").not.toBeNull();

    const unwired = ports.filter((p) => !new RegExp(`^\\s*${p}:`, "m").test(construction![1]));
    expect(unwired, "view ports the viewer app declares but never wires").toEqual([]);
  });
});

// The client half of the same blind spot, and the one that bit first. A client that is never passed
// is not a type error (the parameters are optional) and shows up nowhere in the DOM; the presenter
// simply no-ops every method that needed it. What it CANNOT do is reach the network, so the rpcs a
// real open makes are the observable that catches it.
describe("a deep-link open reaches every service the page needs", () => {
  it("calls each client at least once", () => {
    // One rpc per client constructed in main.ts. If a client stops being passed, its method stops
    // being called, and the panel that depended on it goes quiet in exactly the way WS9-052 did.
    const perClient = {
      design: "GetDesign",
      checks: "ListRules",
      project: "ResolveDesign",
      review: "ListReviews",
      workspace: "ListMounts",
      query: "ListRelations",
    };
    const missing = Object.entries(perClient)
      .filter(([, method]) => !called.includes(method))
      .map(([client]) => client);
    expect(missing, "clients the composition root never gave the presenter").toEqual([]);
  });

  // End to end through the real root: a stubbed resolution reaches the real bar and renders. This is
  // the assertion that fails if the project bar is ever unwired again, without depending on how the
  // wiring is spelled.
  it("renders the resolved project into the bar", () => {
    expect(document.querySelector(".projbar")?.textContent ?? "").toContain("Demo project");
  });
});

// The landing page's Recent list is written by the viewer, not by the landing page, so the wiring
// that feeds it is invisible from either file alone: recents.ts has its own tests and the landing
// panels have theirs, and both stay green if main.ts never records anything. The deep-link boot
// above is exactly an opening, so the store is an observable of it.
describe("opening a design feeds the landing page's Recent list", () => {
  it("records the opened design", () => {
    const got = loadRecents();
    expect(got.map((r) => `${r.mount}/${r.path}`), "the deep-linked design was not recorded").toContain("m/d/b.edn");
    expect(got[0].kind).toBe("design"); // routed back to the viewer, not the workbench
  });
});

// Run checks, click a finding, watch it locate. This is agni issue 136's "open design → Review →
// run → click a finding → highlight" flow, minus the pixels: every step here is a real click on the
// real page driving the real composition root, and the observable is the rpc the presenter makes.
//
// What it cannot assert is whether anything lit up on the canvas, which needs WebGL and a browser.
// That half stays on the successor issue, and the split is the point: the wiring is testable here
// and the rendering is not, so testing the wiring here costs nothing and covers the failure mode
// that has actually shipped (a panel wired to a client it never received).
describe("run checks, then locate a finding", () => {
  it("runs the check the panel offers", async () => {
    const run = document.querySelector(".checks-run") as HTMLButtonElement | null;
    expect(run, "the checks panel rendered no Run button").not.toBeNull();
    run!.click();
    await vi.waitFor(() => expect(called).toContain("CheckDesign"), 5000);
  });

  it("highlights the subject when its finding is clicked", async () => {
    const locate = await vi.waitFor(() => {
      const b = document.querySelector(".check-locate") as HTMLButtonElement | null;
      expect(b, "no finding row rendered to click").not.toBeNull();
      return b!;
    }, 5000);
    expect(locate.textContent).toContain("R1");

    // Counted rather than tested for membership: the deep-link restore already highlights, so
    // "HighlightSheet appears in called" is true before the click and the assertion would pass with
    // the handler unwired. It did, on the first run of this test.
    const before = called.filter((m) => m === "HighlightSheet").length;
    locate.click();
    await vi.waitFor(() => expect(called.filter((m) => m === "HighlightSheet").length).toBeGreaterThan(before), 5000);
  });
});

// Clicking the drawing is the entry point everything else waits on, and it crosses four files:
// the renderer keys the element, SvgView resolves the click, selection.ts writes the query, and
// main.ts hands it to the panel and brings that panel forward. Each of those has its own test; this
// is the one that fails if they are not connected to each other.
describe("clicking the drawing asks a question about what was clicked", () => {
  it("turns a click on a keyed element into a query in the panel", async () => {
    // The SVG the server would send, keyed the way core/render/svg.go keys it.
    const wire = document.createElementNS("http://www.w3.org/2000/svg", "polyline");
    wire.setAttribute("data-kind", "net");
    wire.setAttribute("data-net", "PMIC_CORE_3V3");
    document.body.appendChild(wire);
    // jsdom implements no layout, so elementFromPoint does not exist at all. Standing it in is what
    // lets this test drive the real click path; the coordinates are arbitrary and only the identity
    // of what comes back matters.
    const realElementFromPoint = document.elementFromPoint;
    document.elementFromPoint = ((x: number, y: number) => (x === 400 && y === 300 ? wire : null)) as typeof document.elementFromPoint;

    try {
      const host = document.getElementById("svg-view");
      expect(host, "the page declares no #svg-view host").toBeTruthy();
      host!.dispatchEvent(new MouseEvent("mousedown", { clientX: 400, clientY: 300, bubbles: true }));
      window.dispatchEvent(new MouseEvent("mouseup", { clientX: 400, clientY: 300, bubbles: true }));

      // The query panel's textarea holds the generated query, and it names the net that was clicked.
      await vi.waitFor(() => {
        const box = document.querySelector<HTMLTextAreaElement>('[data-component="query"] textarea');
        expect(box?.value ?? "").toContain("PMIC_CORE_3V3");
      }, 3000);
    } finally {
      document.elementFromPoint = realElementFromPoint;
      wire.remove();
    }
  });
});
