import { describe, expect, it, vi } from "vitest";
import { Code, ConnectError, createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { ViewerPageService } from "./gen/agni/v1/viewer/page_pb.js";
import { ACCEPTED, codeName, ENVELOPE, embedOrigins, listenForHosts, originAllowed, parseRequest, type HostWindow } from "./embed.js";
import { frameFetch } from "./embedclient.js";
import type { ViewerPageAPI } from "./pageapi.js";
import { pageDispatcher, type PageDispatch } from "./pageservice.js";
import { emptyLocation } from "./router.js";

const VIEWER = "https://viewer.example";
const PORTAL = "https://portal.example";

// fakeWindow is a window that delivers a posted message to its listeners as a MessageEvent from a
// given source and origin, the two fields the transport checks.
function fakeWindow(origin: string) {
  const listeners = new Set<(ev: MessageEvent) => void>();
  const w = {
    location: { origin },
    parent: null as HostWindow["parent"],
    addEventListener: (_t: "message", fn: (ev: MessageEvent) => void) => listeners.add(fn),
    removeEventListener: (_t: "message", fn: (ev: MessageEvent) => void) => listeners.delete(fn),
    // deliver hands `data` to this window's listeners as if `from` had posted it.
    deliver(data: unknown, from: { origin: string; win: unknown }) {
      for (const fn of [...listeners]) fn({ data, origin: from.origin, source: from.win } as MessageEvent);
    },
    listening: () => listeners.size,
  };
  return w;
}

// pair is a parent page framing the viewer. Each window's postMessage delivers to the other, stamped
// with the sender's origin, and only when the sender named the receiver's origin, as a browser does.
function pair(parentOrigin: string) {
  const viewer = fakeWindow(VIEWER);
  const parent = fakeWindow(parentOrigin);
  const posted: { to: string; msg: unknown }[] = [];
  const viewerHandle = {
    postMessage: (msg: unknown, target: string) => {
      if (target === VIEWER) queueMicrotask(() => viewer.deliver(msg, { origin: parentOrigin, win: parentHandle }));
    },
  };
  const parentHandle = {
    postMessage: (msg: unknown, target: string) => {
      posted.push({ to: target, msg });
      if (target === parentOrigin || target === "*") queueMicrotask(() => parent.deliver(msg, { origin: VIEWER, win: viewerHandle }));
    },
  };
  viewer.parent = parentHandle;
  return { viewer, parent, viewerHandle, parentHandle, posted };
}

function fakeApi(): ViewerPageAPI {
  return {
    location: () => ({ ...emptyLocation(), mount: "demo", path: "d.kicad_sch", sheet: "s-a" }),
    applyLocation: vi.fn(async () => ({ location: { ...emptyLocation(), mount: "demo", path: "d.kicad_sch", sheet: "s-a", verdict: "demo-rule:(net:NET_A)" }, verdictResolved: true })),
    locateVerdict: vi.fn(async () => true),
    selectFinding: vi.fn(async () => true),
    locateEntity: vi.fn(async () => {}),
    showSheet: vi.fn(async (id: string) => {
      if (id !== "s-a") throw new Error(`the open design has no sheet "${id}"`);
    }),
  };
}

describe("the postMessage envelope", () => {
  it("reads only an agni-viewer/v1 request with an id and a method", () => {
    expect(parseRequest({ agni: ENVELOPE, id: 1, method: "GetLocation" })).toEqual({ agni: ENVELOPE, id: 1, method: "GetLocation", body: undefined });
    for (const junk of [null, "GetLocation", { agni: "agni-viewer/v2", id: 1, method: "GetLocation" }, { agni: ENVELOPE, method: "GetLocation" }, { agni: ENVELOPE, id: {}, method: "x" }, { agni: ENVELOPE, id: 1 }]) {
      expect(parseRequest(junk), JSON.stringify(junk)).toBeNull();
    }
  });

  it("allows the page's own origin and the named ones, never an opaque one", () => {
    expect(originAllowed(VIEWER, VIEWER, [])).toBe(true);
    expect(originAllowed(PORTAL, VIEWER, [])).toBe(false);
    expect(originAllowed(PORTAL, VIEWER, [PORTAL])).toBe(true);
    expect(originAllowed("null", "null", ["null"])).toBe(false);
    expect(embedOrigins(` ${PORTAL}  http://127.0.0.1:9000 `)).toEqual([PORTAL, "http://127.0.0.1:9000"]);
    expect(embedOrigins(undefined)).toEqual([]);
  });

  it("spells a Connect code as the wire does", () => {
    expect(codeName(Code.InvalidArgument)).toBe("invalid_argument");
    expect(codeName(Code.PermissionDenied)).toBe("permission_denied");
    expect(codeName(Code.NotFound)).toBe("not_found");
  });
});

describe("listenForHosts", () => {
  it("refuses a page from an origin the server did not name, and dispatches nothing", async () => {
    const p = pair(PORTAL);
    const dispatch = vi.fn<PageDispatch>(async () => ({}));
    listenForHosts(p.viewer, dispatch, []);
    const got = new Promise((r) => p.parent.addEventListener("message", (ev) => r(ev.data)));
    p.viewerHandle.postMessage({ agni: ENVELOPE, id: 7, method: "ShowSheet", body: { sheet: "s-a" } }, VIEWER);
    expect(await got).toMatchObject({ id: 7, ok: false, error: { code: "permission_denied" } });
    expect(dispatch).not.toHaveBeenCalled();
  });

  // A report opened from disk has an opaque origin and is named by no server, and loading the link in
  // the viewer's window would do what this does, so the page takes it and answers only ok.
  const LINK = "http://127.0.0.1:8080/designs/kicad/hier_numeric_root.kicad_sch/view?sheet=%2FIO";
  for (const origin of ["null", PORTAL]) {
    it(`takes a link-only ApplyLocation from ${origin}, and answers without the location`, async () => {
      const p = pair(origin);
      const dispatch = vi.fn<PageDispatch>(async () => ({ location: { mount: "kicad", sheet: "/IO" }, verdictResolved: false }));
      listenForHosts(p.viewer, dispatch, []);
      const seen: unknown[] = [];
      const got = new Promise((r) =>
        p.parent.addEventListener("message", (ev) => {
          if ((ev.data as { ready?: boolean }).ready) return;
          seen.push(ev.data);
          if ((ev.data as { agni?: string }).agni === ENVELOPE) r(ev.data);
        }),
      );
      // The fake parent posts to whatever origin is named; a real opaque page posts with "*".
      p.viewer.deliver({ agni: ENVELOPE, id: 3, method: "ApplyLocation", body: { url: LINK } }, { origin, win: p.parentHandle });
      expect(await got).toEqual({ agni: ENVELOPE, id: 3, ok: true, body: {} });
      // Taken before it was answered, so a report waiting on it knows a viewer is working.
      expect(seen[0]).toEqual({ agni: ACCEPTED, id: 3 });
      expect(dispatch).toHaveBeenCalledWith("ApplyLocation", { url: LINK });
    });
  }

  it("refuses anything else from an origin it was not told about, a link with a location included", async () => {
    const p = pair(PORTAL);
    const dispatch = vi.fn<PageDispatch>(async () => ({}));
    listenForHosts(p.viewer, dispatch, []);
    const replies: unknown[] = [];
    p.parent.addEventListener("message", (ev) => !(ev.data as { ready?: boolean }).ready && replies.push(ev.data));
    for (const [id, method, body] of [
      [4, "ApplyLocation", { url: LINK, location: { sheet: "/IO" } }],
      [5, "ApplyLocation", { location: { sheet: "/IO" } }],
      [6, "GetLocation", {}],
    ] as const) {
      p.viewer.deliver({ agni: ENVELOPE, id, method, body }, { origin: PORTAL, win: p.parentHandle });
    }
    await new Promise((r) => setTimeout(r, 0));
    expect(replies).toHaveLength(3);
    for (const r of replies) expect(r).toMatchObject({ ok: false, error: { code: "permission_denied" } });
    expect(dispatch).not.toHaveBeenCalled();
  });

  it("announces itself to the parent at its own origin and each named one", () => {
    const p = pair(PORTAL);
    listenForHosts(p.viewer, vi.fn<PageDispatch>(), [PORTAL]);
    expect(p.posted).toEqual([
      { to: VIEWER, msg: { agni: ENVELOPE, ready: true } },
      { to: PORTAL, msg: { agni: ENVELOPE, ready: true } },
    ]);
  });

  it("answers requests one at a time, in the order they arrive", async () => {
    const p = pair(VIEWER);
    const order: string[] = [];
    let release!: () => void;
    const gate = new Promise<void>((r) => (release = r));
    const dispatch: PageDispatch = async (method) => {
      order.push(`start ${method}`);
      if (method === "ApplyLocation") await gate;
      order.push(`end ${method}`);
      return {};
    };
    listenForHosts(p.viewer, dispatch, []);
    const replies: unknown[] = [];
    const both = new Promise<void>((r) =>
      p.parent.addEventListener("message", (ev) => {
        if ((ev.data as { ready?: boolean }).ready) return; // the page announcing itself, not a reply
        if ((ev.data as { agni?: string }).agni === ACCEPTED) return; // a request taken, not yet answered
        replies.push(ev.data);
        if (replies.length === 2) r();
      }),
    );
    p.viewerHandle.postMessage({ agni: ENVELOPE, id: 1, method: "ApplyLocation" }, VIEWER);
    p.viewerHandle.postMessage({ agni: ENVELOPE, id: 2, method: "GetLocation" }, VIEWER);
    await new Promise((r) => setTimeout(r, 0));
    expect(order).toEqual(["start ApplyLocation"]);
    release();
    await both;
    expect(order).toEqual(["start ApplyLocation", "end ApplyLocation", "start GetLocation", "end GetLocation"]);
  });

  it("stops listening when told to", () => {
    const p = pair(VIEWER);
    const stop = listenForHosts(p.viewer, vi.fn<PageDispatch>(), []);
    expect(p.viewer.listening()).toBe(1);
    stop();
    expect(p.viewer.listening()).toBe(0);
  });
});

// The generated Connect client, sending through frameFetch, reaches the page's dispatcher over the
// envelope and back: one contract, carried by postMessage.
describe("a parent driving the viewer with the generated client", () => {
  function client(api: ViewerPageAPI) {
    const p = pair(PORTAL);
    listenForHosts(p.viewer, pageDispatcher(api), [PORTAL]);
    const transport = createConnectTransport({ baseUrl: "https://viewer.invalid", fetch: frameFetch(p.viewerHandle, VIEWER, p.parent) });
    return createClient(ViewerPageService, transport);
  }

  it("applies a location with a verdict and its hints, and reads the sheet the page landed on", async () => {
    const api = fakeApi();
    const page = client(api);
    const res = await page.applyLocation({ location: { verdict: "demo-rule:(net:NET_A)", rule: "demo-rule", subjects: ["net:NET_A"] } });
    expect(api.applyLocation).toHaveBeenCalledWith({ verdict: "demo-rule:(net:NET_A)", rule: "demo-rule", subjects: ["net:NET_A"] });
    expect(res.verdictResolved).toBe(true);
    expect(res.location?.sheet).toBe("s-a");
  });

  it("gets the page's refusal back as a ConnectError with its code", async () => {
    const page = client(fakeApi());
    const err = await page.showSheet({ sheet: "s-z" }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ConnectError);
    expect((err as ConnectError).code).toBe(Code.InvalidArgument);
    expect((err as ConnectError).rawMessage).toBe('the open design has no sheet "s-z"');
  });
});
