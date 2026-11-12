// @vitest-environment jsdom
import { describe, it, expect, beforeAll, vi } from "vitest";
import { readFileSync } from "node:fs";
import { loadRecents } from "@agni/web-shared/recents.js";
import { join } from "node:path";

// The composition root, under test at last (agni issue 136).
//
// Every other web test constructs its subject directly, with collaborators supplied. Nothing
// constructed `main.ts`. It resolves each island hole by id, builds each client, wires each
// callback, and assembles the presenter, and none of that ran under test. That is not an oversight
// in one PR. It is a structural blind spot, and it has now shipped the same bug twice.
//
//   WS9-052 (PR 135) shipped a review panel that was broken in the running app because main.ts never
//   passed the review clients to the presenter. Unit tests, presenter tests and typecheck all green.
//
//   Issue 175 (PR 182) shipped a project bar that rendered nothing because main.ts never constructed
//   one. `views.projectBar` was undefined, so `views.projectBar?.setState(...)` was a no-op on every
//   push. Again green, again found by opening the page (PR 184 fixed it).
//
// Both are the same shape, correct logic and correct tests wired to nothing. Every test here therefore
// asserts the WIRING, and does it against the REAL page and the REAL entry point, because a
// hand-built DOM would be one more artifact that agrees with the code while the shipped page does
// not.
//
// This is the cheap rung of #136 and not the whole ticket. jsdom does not render, so
// nothing below asserts a pixel, a highlight, or an SVG. It does assert that the app boots, mounts
// what the page declares, and reaches every service on a real deep-link open. The flows that
// need a real browser are still open on the issue.

// pageBody is the real ViewerPage template's Body block. The point is to read the SHIPPED template.
// The holes and their ids are the contract between the server-rendered page and main.ts. A test
// carrying its own copy of the markup would keep passing after the page changed underneath it.
//
// The block is HTML with a few server-filled fields (the engine, its size limit, the base path and
// the host). So slicing it out needs no template engine, as long as each field is filled with what
// `agni serve` renders by default. A field this does not know throws. Leaving `{{ ... }}` in an
// attribute that the bundle reads would test a page no server ever sends.
const servedFields: Record<string, string> = {
  "{{ .Engine }}": "server",
  "{{ .WasmMaxBytes }}": "134217728",
  "{{ .Base }}": "/",
  "{{ .Host }}": "",
  "{{ .Build }}": "",
  "{{ .EmbedOrigins }}": "",
  "{{ .ViewerRooms }}": "",
};

function pageBody(): string {
  const src = readFileSync(join(process.cwd(), "templates/ViewerPage.html"), "utf8");
  const m = /\{\{ define "Body" \}\}([\s\S]*?)\n\{\{ end \}\}/.exec(src);
  if (!m) throw new Error("ViewerPage.html has no Body block; the composition test cannot run");
  let body = m[1];
  for (const [field, value] of Object.entries(servedFields)) body = body.split(field).join(value);
  const left = /\{\{[^}]*\}\}/.exec(body);
  if (left) throw new Error(`ViewerPage.html's Body has a field this test does not fill: ${left[0]}`);
  return body;
}

function readSrc(name: string): string {
  return readFileSync(join(process.cwd(), "src", name), "utf8");
}

// jsdom lacks two browser APIs the shell reaches for. Both are stubbed rather than worked around,
// because the alternative is not booting the real composition root.
// Neither stub can hide a wiring bug, since an island is mounted or it is not, and an rpc is called
// or it is not.
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}

// The service replies that the restore needs, keyed by rpc method name. They are the thinnest thing
// each caller accepts. This file asserts that a call was MADE, never what came back.
const REPLIES: Record<string, unknown> = {
  ListMounts: { mounts: [{ name: "m" }] },
  ListDir: { entries: [] },
  GetDesign: { layout: "grid", sheets: [{ id: "s1", name: "Top" }], availableLayouts: ["grid"], nativeAvailable: false },
  GetSheet: { content: { case: "svg", value: "<svg/>" } },
  GetSheetSvg: { svg: "<svg/>" },
  ResolveDesign: {
    design: { name: "projects/demo/designs/board", entryUri: "mount://m/d/b.edn" },
    project: { name: "projects/demo", title: "Demo project", config: { conventions: { name: "demo" }, checklists: [] } },
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
    entityQueries: [{ kind: "net", label: "What pins are on it?", query: "component.net(?ref, ?net) => ?ref", teaches: "join", binds: ["net"] }],
  },
  // One row, so the query panel has a row to open (agni issue 794).
  RunQuery: { columns: ["ref"], columnKinds: ["component"], rows: [{ cells: ["R1"] }] },
};

const called: string[] = [];
const bootErrors: unknown[] = [];

beforeAll(async () => {
  (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = ResizeObserverStub;
  // WebGL is genuinely absent here. A browser without a context returns null. The shell handles that
  // and the readout says so, so the boot still completes.
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

  // A DESIGN url, not the bare page. With it, the test exercises the restore loop rather than only
  // the mount. main.ts reads the URL at boot and replays it through presenter.restore, so
  // every client the open path needs gets used the way a deep-link refresh uses it.
  // It arrives as the Load page sends a reader once it has read the design (agni issue 1072).
  window.history.replaceState(null, "", "/designs/m/d/b.edn/view?loaded=1");
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

  // main.ts reports a boot failure in the readout. A person opening the page would have seen
  // an error there, and both prior bugs were eventually found that way.
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

// The hole test above walks `data-component` islands, and a note strip is not one. `undrawnStrip`
// and `staleLinkStrip` take `document.getElementById(...)` and no-op on null, so deleting
// `<div id="undrawn-note">` from the page leaves every test in this file green while the strip is
// silently dead (OUT_OF_SCOPE, PR 490, issue 392 acceptance 3). The same is true of every other id
// that the root resolves. The lookup is the contract, and a missing hole is a no-op, not an error.
//
// So read the ids off main.ts the way the port test reads ViewSink, and resolve each one against the
// booted page. Every id the root looks up today is declared in ViewerPage.html's Body block. Suppose
// the root means to resolve an id somewhere else, or on an element built at runtime. That id would
// fail here, and it should. It is a second contract, and it should be stated out loud rather than
// pass by accident.
describe("every id the composition root looks up is declared by the page", () => {
  it("resolves each getElementById in main.ts against the real template", () => {
    const ids = [...new Set([...readSrc("main.ts").matchAll(/getElementById\("([^"]+)"\)/g)].map((m) => m[1]))];
    expect(ids.length).toBeGreaterThan(20); // main.ts really was parsed, not an empty match

    const absent = ids.filter((id) => !document.getElementById(id));
    expect(absent, "ids main.ts resolves that the page never declares").toEqual([]);
  });
});

// The hole test catches a panel whose island was never wired. It cannot catch a presenter VIEW PORT
// that was never wired, because an unwired port puts nothing on the page to notice the absence of.
// The project bar shipped invisible that way.
//
// ViewSink's own doc states the contract that this enforces, "adding a panel is one field here and
// one line in the composition root". Nothing checked the second half.
//
// Reading the two files as source is unusual for a unit test and is the right shape here, for the
// same reason docsite/nav_test.go checks a new page reached all four of its registration points.
// This test prevents an OMISSION ACROSS FILES, and only something holding both files can see an
// omission. There is no runtime state that distinguishes "deliberately absent" from "forgotten".
//
// The ports are optional in the interface by design, so a host embedding the presenter can leave a
// panel out (docsite/content/architecture/web-client.md). That is a statement about OTHER hosts.
// This app is the reference host and wires all of them, and "optional" stopped the type checker
// from noticing.
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

// This is the client half of the same blind spot, and it bit first. A client that is never passed
// is not a type error, since the parameters are optional. It shows up nowhere in the DOM either. The
// presenter simply no-ops every method that needed it. But a missing client CANNOT reach the
// network, so the rpcs that a real open makes are the observable that catches it.
describe("a deep-link open reaches every service the page needs", () => {
  it("calls each client at least once", () => {
    // One rpc per client constructed in main.ts. If a client stops being passed, its method stops
    // being called, and the panel that depended on it goes quiet the way WS9-052 did.
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

  // End to end through the real root, a stubbed resolution reaches the real bar and renders. This
  // assertion fails if the project bar is ever unwired again, and it does not depend on how the
  // wiring is spelled.
  it("renders the resolved project into the bar", () => {
    expect(document.querySelector(".projbar")?.textContent ?? "").toContain("Demo project");
  });
});

// The landing page's Recent list is written by the viewer, not by the landing page, so the wiring
// that feeds it is invisible from either file alone. recents.ts has its own tests and the landing
// panels have theirs, and both are still green if main.ts never records anything. The deep-link
// boot above is an opening, so the store is an observable of it.
describe("opening a design feeds the landing page's Recent list", () => {
  it("records the opened design", () => {
    const got = loadRecents();
    expect(got.map((r) => `${r.mount}/${r.path}`), "the deep-linked design was not recorded").toContain("m/d/b.edn");
    expect(got[0].kind).toBe("design"); // routed back to the viewer, not the workbench
  });
});

// Run checks, click a finding, watch it locate. This is agni issue 136's "open design → Review →
// run → click a finding → highlight" flow, minus the pixels. Every step here is a real click on the
// real page driving the real composition root, and the observable is the rpc the presenter makes.
//
// It cannot assert whether anything lit up on the canvas, which needs WebGL and a browser. That
// half remains on the successor issue. The wiring is testable here and the rendering is not. So
// testing the wiring here costs nothing, and it covers the failure mode that has actually shipped
// (a panel wired to a client it never received).
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

    // Counted rather than tested for membership, because the deep-link restore already highlights, so
    // "HighlightSheet appears in called" is true before the click and the assertion would pass with
    // the handler unwired. It did, on the first run of this test.
    const before = called.filter((m) => m === "HighlightSheet").length;
    locate.click();
    await vi.waitFor(() => expect(called.filter((m) => m === "HighlightSheet").length).toBeGreaterThan(before), 5000);
  });
});

// Clicking the drawing is the entry point, and everything else waits on it. The click crosses four
// files. The renderer keys the element, SvgView resolves the click, selection.ts writes the query,
// and main.ts hands it to the panel and brings that panel forward. Each of those has its own test.
// This test fails if they are not connected to each other.
describe("clicking the drawing selects what was clicked, and Ask asks about it", () => {
  it("names a click in Properties, leaves the query box alone, and runs the preset from Ask", async () => {
    // The SVG the server would send, keyed the way core/render/svg.go keys it.
    const wire = document.createElementNS("http://www.w3.org/2000/svg", "polyline");
    wire.setAttribute("data-kind", "net");
    wire.setAttribute("data-net", "PMIC_CORE_3V3");
    document.body.appendChild(wire);
    // jsdom implements no layout, so elementFromPoint does not exist at all. A stand-in lets this
    // test drive the real click path. The coordinates are arbitrary, and only the identity of what
    // comes back matters.
    const realElementFromPoint = document.elementFromPoint;
    document.elementFromPoint = ((x: number, y: number) => (x === 400 && y === 300 ? wire : null)) as typeof document.elementFromPoint;

    try {
      const box = () => document.querySelector<HTMLTextAreaElement>('[data-component="query"] textarea');
      const typed = box()?.value ?? "";
      const host = document.getElementById("svg-view");
      expect(host, "the page declares no #svg-view host").toBeTruthy();
      host!.dispatchEvent(new MouseEvent("mousedown", { clientX: 400, clientY: 300, bubbles: true }));
      window.dispatchEvent(new MouseEvent("mouseup", { clientX: 400, clientY: 300, bubbles: true }));

      // Properties names the pick (agni issue 902), and the query box keeps whatever was in it,
      // since a stray click used to overwrite a query the reader had typed.
      await vi.waitFor(() => {
        expect(document.querySelector('[data-component="properties"] .pr-kind')?.textContent).toBe("net");
        expect(document.querySelector('[data-component="properties"] .pr-title')?.textContent).toBe("PMIC_CORE_3V3");
      }, 3000);
      expect(box()?.value ?? "").toBe(typed);

      // Ask runs the served preset, and its binding chip names the net that was clicked (agni issue
      // 793). main.ts wiring the Ask row to the query panel is its own edit to leave out.
      const ask = document.querySelector<HTMLButtonElement>('[data-component="properties"] .pr-ask-go');
      expect(ask, "Properties offers no Ask row for a net").toBeTruthy();
      ask!.click();
      await vi.waitFor(() => {
        expect(box()?.value ?? "").toContain("?net");
        const bound = document.querySelector<HTMLInputElement>('[data-component="query"] .query-binding-value');
        expect(bound?.value ?? "").toBe("PMIC_CORE_3V3");
      }, 3000);
      // The question opened a tab of its own, named for what it asked about and the served label,
      // so the query box the reader started in is still there behind it.
      const front = document.querySelector('[data-component="query"] .query-tab.on .query-tab-go');
      // The title, since the label also carries a running mark while the query is in flight.
      expect(front?.getAttribute("title") ?? "").toBe("PMIC_CORE_3V3: What pins are on it?");
      expect(document.querySelectorAll('[data-component="query"] .query-tab').length).toBe(2);
      // And the query panel names what was asked about. The next hop is offered from there.
      const named = document.querySelector('[data-component="query"] .query-selection-name');
      expect(named?.textContent ?? "").toBe("PMIC_CORE_3V3");
    } finally {
      document.elementFromPoint = realElementFromPoint;
      wire.remove();
    }
  });
});

// Opening a query row asks the question again with witnesses (agni issue 794). The panel's onExplain
// is optional, so main.ts leaving it unwired would leave every row showing citations alone, with
// every unit test green.
describe("opening a query row asks how it holds", () => {
  it("runs the tab's question again when a row is opened", async () => {
    const toggle = await vi.waitFor(() => {
      const b = document.querySelector<HTMLButtonElement>('[data-component="query"] .query-cite-toggle');
      expect(b, "the query panel shows no row to open").not.toBeNull();
      return b!;
    }, 5000);
    const before = called.filter((m) => m === "RunQuery").length;
    toggle.click();
    await vi.waitFor(() => expect(called.filter((m) => m === "RunQuery").length).toBeGreaterThan(before), 5000);
  });
});

// Escape is a PAGE-level binding, so nothing below main.ts can assert it. The presenter's
// clearHighlights has its own unit test. No other test asks whether this page ever hears the key at
// all, and the page did not hear it. The only Escape handler in the client belongs to the
// datasheet workbench (pagegestures, reached from regionview) and never sees this canvas, so the
// gesture a reader reaches for first did nothing (agni issue 348).
//
// The observable is the findings panel's selected row rather than an rpc, because clearing does NOT
// make one. setHighlights([]) short-circuits the overlay fetch instead of requesting an empty one.
describe("Escape clears the highlight (agni issue 348)", () => {
  // Every describe in this file shares ONE booted page, so a finding may already be focused from the
  // locate test above and a blind click would TOGGLE IT OFF. This asserts the state we need rather
  // than assuming it, so it does not depend on what ran before.
  async function ensureSelected(): Promise<void> {
    const locate = await vi.waitFor(() => {
      const b = document.querySelector(".check-locate") as HTMLButtonElement | null;
      expect(b, "no finding row rendered to click").not.toBeNull();
      return b!;
    }, 2000);
    if (!document.querySelector(".check-row.selected")) locate.click();
    await vi.waitFor(() => expect(document.querySelector(".check-row.selected"), "no row ended up selected").not.toBeNull(), 2000);
  }

  it("deselects the focused finding when the key reaches the page", async () => {
    await ensureSelected();
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    await vi.waitFor(() => expect(document.querySelector(".check-row.selected"), "Escape did not reach the page").toBeNull(), 2000);
  }, 10000);

  it("declines the key while the reader is typing, so a text field keeps its own Escape", async () => {
    await ensureSelected();
    const input = document.createElement("input");
    document.body.appendChild(input);
    input.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    // Still selected, because the key belonged to the field, not to the drawing.
    expect(document.querySelector(".check-row.selected")).not.toBeNull();
    input.remove();
  }, 10000);
});
