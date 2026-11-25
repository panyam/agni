// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { Code, ConnectError } from "@connectrpc/connect";
import type { JoinToolRoomOptions, PageTool, PairedByCode, Principal, ToolRefusal, ToolRoom } from "@panyam/mcpkit-rooms";
import { ViewerPageService } from "@panyam/agni-client/gen/agni/v1/viewer/page_pb";
import type { JoinViewerRoomResponse } from "@panyam/agni-client/gen/agni/v1/webapi/viewerroom_pb";
import { pageDispatcher, type PageAnswers, type PageDispatch } from "./pageservice.js";
import { AGENTS_KEY, MAX_SHOWN, ViewerRoomTab, type RoomClient } from "./viewerroom.js";
import type { ViewerPageAPI } from "./pageapi.js";
import { emptyLocation } from "./router.js";
import { toolSchema } from "./toolschema.js";

// FakeRoom records what the tab does with its room.
class FakeRoom {
  tools = new Map<string, PageTool>();
  label = "";
  allowed: boolean[] = [];
  focused = 0;
  left = false;
  emits: { name: string; data: unknown }[] = [];
  refuse: ((r: ToolRefusal) => void) | undefined;
  constructor(readonly opts: JoinToolRoomOptions) {}
  registerTool(t: PageTool) {
    this.tools.set(t.name, t);
    return { unregister: () => this.tools.delete(t.name) };
  }
  setLabel(l: string) {
    this.label = l;
  }
  setAgentsAllowed(a: boolean) {
    this.allowed.push(a);
  }
  focus() {
    this.focused++;
  }
  leave() {
    this.left = true;
  }
  onToolRefused(fn: (r: ToolRefusal) => void) {
    this.refuse = fn;
  }
  events: string[] = [];
  registerEvent(d: { name: string }) {
    this.events.push(d.name);
    return { unregister: () => {} };
  }
  emit(name: string, data?: unknown) {
    this.emits.push({ name, data });
  }
  ask(text: string) {
    this.emit("room.ask", { text });
  }
  selectionChanged(selection: unknown) {
    this.emit("page.selection_changed", { selection });
  }
  offers: { ttlMs?: number }[] = [];
  paired: ((p: PairedByCode) => void) | undefined;
  grantList: Principal[] = [];
  offerPairingCode(opts: { ttlMs?: number } = {}) {
    this.offers.push(opts);
    return `CODE${this.offers.length}`;
  }
  onPaired(fn: (p: PairedByCode) => void) {
    this.paired = fn;
  }
  // pair is an agent entering the code: the page grants it and says so, as the rooms client does.
  pair(p: Principal, code: string) {
    this.grantList.push(p);
    this.paired!({ ...p, code, sameSubject: false });
  }
  grants() {
    return this.grantList.map((g) => ({ ...g }));
  }
  revoke(who: Principal) {
    this.grantList = this.grantList.filter((g) => !(g.subject === who.subject && (!who.client || g.client === who.client)));
  }
}

function harness(opts: { stored?: string; joinFails?: boolean; dispatch?: PageDispatch; dispatcher?: (a: PageAnswers) => PageDispatch } = {}) {
  const rooms: FakeRoom[] = [];
  const asked: string[] = [];
  const client: RoomClient = {
    joinViewerRoom: async (req) => {
      asked.push(req.design ?? "");
      if (opts.joinFails) throw new ConnectError("no such mount", Code.InvalidArgument);
      return { room: `design:${req.design}.sch`, relayUrl: "/relay/", token: `tok-${req.design}`, bridgeSubjects: ["agni-bridge"] } as JoinViewerRoomResponse;
    },
  };
  const store = new Map<string, string>(opts.stored ? [[AGENTS_KEY, opts.stored]] : []);
  const storage = { getItem: (k: string) => store.get(k) ?? null, setItem: (k: string, v: string) => void store.set(k, v) };
  const winListeners: Record<string, () => void> = {};
  const win = { location: { href: "http://127.0.0.1:8080/mounts/m/a/view" } as Location, addEventListener: (t: string, f: () => void) => void (winListeners[t] = f) };
  const host = document.createElement("div");
  host.hidden = true;
  const dispatch = opts.dispatch ?? vi.fn(async (method: string, body: unknown) => ({ method, body }) as never);
  const tab = new ViewerRoomTab({
    client,
    dispatcher: opts.dispatcher ?? (() => dispatch),
    host,
    storage,
    win: win as never,
    join: async (o) => {
      const r = new FakeRoom(o);
      rooms.push(r);
      return r as unknown as ToolRoom;
    },
  });
  return { tab, rooms, asked, store, host, dispatch, winListeners };
}

// answering builds a harness whose tab answers agents with the real dispatcher, over a page that
// records what it was asked to highlight.
function answering() {
  const located: unknown[][] = [];
  const api = {
    location: () => ({ ...emptyLocation(), mount: "m", path: "a", sheet: "s1" }),
    locateEntity: async (...args: unknown[]) => void located.push(args),
  } as unknown as ViewerPageAPI;
  const h = harness({ dispatcher: (answers) => pageDispatcher(api, answers) });
  const call = (tool: string, args: Record<string, unknown>) => h.rooms[0].tools.get(tool)!.execute({ design: "m/a", ...args }, {});
  const box = () => h.host.querySelector<HTMLElement>(".agents-answers")!;
  return { ...h, located, call, box };
}

describe("an agent's answers on the tab", () => {
  it("shows an answer as text, with a button per entity that highlights it", async () => {
    const h = answering();
    await h.tab.follow({ mount: "m", path: "a" });
    expect(h.box().hidden).toBe(true);
    const out = await h.call("viewer_show_answer", { text: "<b>SDA</b> has no pull-up", entities: [{ kind: "net", subject: "SDA" }, { kind: "pin", subject: "U1", pin: "3" }] });
    expect(out).toMatchObject({ answer: "answer-1" });
    expect(h.box().hidden).toBe(false);
    const item = h.box().querySelector<HTMLElement>('[data-answer="answer-1"]')!;
    expect(item.querySelector(".agents-words")!.textContent).toBe("<b>SDA</b> has no pull-up");
    expect(item.querySelector("b")).toBeNull();
    const entities = [...item.querySelectorAll<HTMLButtonElement>("button.agents-entity")];
    expect(entities.map((b) => b.textContent)).toEqual(["SDA", "U1.3"]);
    entities[1].click();
    await vi.waitFor(() => expect(h.located).toEqual([["pin", "U1", { pin: "3", sheet: undefined }]]));
  });

  it("declares the choice event, and sends the reader's choice on a proposal once", async () => {
    const h = answering();
    await h.tab.follow({ mount: "m", path: "a" });
    expect(h.rooms[0].events).toEqual(["page.proposal_answered"]);
    const out = await h.call("viewer_propose", { action: "open sheet IO/1", entities: [{ kind: "net", subject: "SDA" }] });
    expect(out).toMatchObject({ proposal: "proposal-1" });
    const card = h.box().querySelector<HTMLElement>('[data-proposal="proposal-1"]')!;
    card.querySelector<HTMLButtonElement>("button.agents-accept")!.click();
    expect(h.rooms[0].emits).toEqual([{ name: "page.proposal_answered", data: { proposal: "proposal-1", choice: "accepted", action: "open sheet IO/1" } }]);
    expect(card.dataset.choice).toBe("accepted");
    expect(card.querySelector("button.agents-accept, button.agents-dismiss")).toBeNull();
    expect(card.querySelector(".agents-choices")!.textContent).toBe("Accepted");
  });

  it("keeps the newest answers on screen and clears them when the tab moves to another design's room", async () => {
    const h = answering();
    await h.tab.follow({ mount: "m", path: "a" });
    for (let i = 1; i <= MAX_SHOWN + 2; i++) await h.call("viewer_show_answer", { text: `answer ${i}` });
    await h.call("viewer_propose", { action: "go" });
    const words = [...h.box().querySelectorAll(".agents-words")].map((p) => p.textContent);
    expect(words).toHaveLength(MAX_SHOWN);
    expect(words[0]).toBe("go");
    expect(words[1]).toBe(`answer ${MAX_SHOWN + 2}`);
    await h.tab.follow({ mount: "m", path: "b" });
    expect(h.box().childElementCount).toBe(0);
    expect(h.box().hidden).toBe(true);
  });
});

describe("ViewerRoomTab", () => {
  it("joins its design's room with the server's token and offers every page method as a tool", async () => {
    const h = harness();
    await h.tab.follow({ mount: "m", path: "a" });
    expect(h.asked).toEqual(["m/a"]);
    expect(h.rooms).toHaveLength(1);
    const r = h.rooms[0];
    expect(r.opts).toMatchObject({ relay: "http://127.0.0.1:8080/relay/", room: "design:m/a.sch", token: "tok-m/a" });
    // The tab answers the server's bridges alone, so another page holding a token for the room cannot
    // call it directly (agni issue 1173).
    expect(r.opts.trustBridges).toEqual(["agni-bridge"]);
    expect([...r.tools.keys()].sort()).toEqual([
      "viewer_apply_location", "viewer_close_sheet_view", "viewer_get_location", "viewer_locate_entity", "viewer_locate_verdict",
      "viewer_open_sheet_view", "viewer_propose", "viewer_select_finding", "viewer_show_answer", "viewer_show_sheet",
    ]);
    expect(r.tools.get("viewer_show_sheet")?.inputSchema).toEqual(toolSchema(ViewerPageService.method.showSheet));
    expect(r.tools.get("viewer_get_location")?.annotations?.readOnlyHint).toBe(true);
    expect(r.tools.get("viewer_show_sheet")?.annotations?.readOnlyHint).toBe(false);
    expect(r.label).toBe("m/a");
    expect(h.host.hidden).toBe(false);
  });

  it("dispatches a call without the design, which only names the room", async () => {
    const h = harness();
    await h.tab.follow({ mount: "m", path: "a" });
    const out = await h.rooms[0].tools.get("viewer_show_sheet")!.execute({ design: "m/a", sheet: "s2" }, {});
    expect(h.dispatch).toHaveBeenCalledWith("ShowSheet", { sheet: "s2" });
    expect(out).toEqual({ method: "ShowSheet", body: { sheet: "s2" } });
  });

  it("reports a page error with its code", async () => {
    const h = harness({ dispatch: async () => Promise.reject(new ConnectError("sheet s9 is not in this design", Code.InvalidArgument)) });
    await h.tab.follow({ mount: "m", path: "a" });
    await expect(h.rooms[0].tools.get("viewer_show_sheet")!.execute({ design: "m/a", sheet: "s9" }, {})).rejects.toThrow("InvalidArgument: sheet s9 is not in this design");
  });

  it("keeps agents out until the reader turns the switch on, and remembers it for the tab", async () => {
    const h = harness();
    await h.tab.follow({ mount: "m", path: "a" });
    expect(h.rooms[0].allowed).toEqual([false]);
    const box = h.host.querySelector("input")!;
    box.checked = true;
    box.dispatchEvent(new Event("change"));
    expect(h.rooms[0].allowed).toEqual([false, true]);
    expect(h.store.get(AGENTS_KEY)).toBe("1");

    const again = harness({ stored: "1" });
    await again.tab.follow({ mount: "m", path: "a" });
    expect(again.rooms[0].allowed).toEqual([true]);
    expect(again.host.querySelector("input")!.checked).toBe(true);
  });

  it("moves to the new design's room when the page opens another design", async () => {
    const h = harness({ stored: "1" });
    await h.tab.follow({ mount: "m", path: "a" });
    await h.tab.follow({ mount: "m", path: "a" });
    expect(h.rooms).toHaveLength(1);
    await h.tab.follow({ mount: "m", path: "b" });
    expect(h.rooms).toHaveLength(2);
    expect(h.rooms[0].left).toBe(true);
    expect(h.rooms[1].opts.room).toBe("design:m/b.sch");
    expect(h.rooms[1].allowed).toEqual([true]);
    await h.tab.leave();
    expect(h.rooms[1].left).toBe(true);
    expect(h.host.hidden).toBe(true);
  });

  it("tells the bridge the reader is looking at the tab when the window takes focus", async () => {
    const h = harness();
    await h.tab.follow({ mount: "m", path: "a" });
    const before = h.rooms[0].focused;
    h.winListeners.focus();
    expect(h.rooms[0].focused).toBe(before + 1);
  });

  it("stays usable and hides the switch when the server will not name a room", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const h = harness({ joinFails: true });
    await h.tab.follow({ mount: "m", path: "a" });
    expect(h.rooms).toHaveLength(0);
    expect(h.host.hidden).toBe(true);
    expect(warn).toHaveBeenCalledWith(expect.stringContaining("no such mount"));
    warn.mockRestore();
  });

  // The Ask box sends the reader's question to the agents the tab agreed to (agni issue 1173). The
  // question includes where the tab is and what is selected, so the agent can answer without asking
  // back.
  it("sends the reader's question with the tab's location and selection, only once the switch is on", async () => {
    const h = harness();
    await h.tab.follow({ mount: "m", path: "a" });
    const form = h.host.querySelector<HTMLFormElement>("form.agents-ask")!;
    expect(form, "the tab renders an ask box").toBeTruthy();
    expect(form.hidden, "the ask box hides while agents are kept out").toBe(true);
    expect(await h.tab.ask("why is SDA flagged?")).toBe(false);
    expect(h.rooms[0].emits).toEqual([]);

    const box = h.host.querySelector<HTMLInputElement>("input[type=checkbox]")!;
    box.checked = true;
    box.dispatchEvent(new Event("change"));
    expect(form.hidden).toBe(false);
    const focus = { level: "entity", sel: { kind: "net", net: "SDA" }, sheetId: "s1" } as const;
    h.tab.select(focus, "sheetview-2");
    h.rooms[0].emits.length = 0;
    expect(await h.tab.ask("why is SDA flagged?")).toBe(true);
    expect(h.dispatch).toHaveBeenCalledWith("GetLocation", {});
    expect(h.rooms[0].emits).toEqual([
      { name: "room.ask", data: { text: "why is SDA flagged?", location: { method: "GetLocation", body: {} }, selection: { ...focus, view: "sheetview-2" } } },
    ]);
  });

  it("names a tool the bridge refused beside the switch, until the tab moves rooms", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const h = harness({ stored: "1" });
    await h.tab.follow({ mount: "m", path: "a" });
    const note = h.host.querySelector<HTMLElement>(".agents-refused")!;
    expect(note.hidden).toBe(true);
    h.rooms[0].refuse!({ name: "viewer_show_sheet", reason: "schema differs from the declaration" });
    expect(note.hidden).toBe(false);
    expect(note.textContent).toBe("Agents cannot call viewer_show_sheet on this tab");
    expect(note.title).toBe("viewer_show_sheet: schema differs from the declaration");
    expect(warn).toHaveBeenCalledWith(expect.stringContaining("viewer_show_sheet"));
    await h.tab.follow({ mount: "m", path: "b" });
    expect(note.hidden).toBe(true);
    expect(note.textContent).toBe("");
    warn.mockRestore();
  });

  it("asks from the box and clears it, and sends nothing blank", async () => {
    const h = harness({ stored: "1" });
    await h.tab.follow({ mount: "m", path: "a" });
    const form = h.host.querySelector<HTMLFormElement>("form.agents-ask")!;
    const input = form.querySelector<HTMLInputElement>("input[type=text]")!;
    input.value = "   ";
    form.dispatchEvent(new Event("submit", { cancelable: true }));
    await Promise.resolve();
    expect(h.rooms[0].emits.filter((e) => e.name === "room.ask")).toEqual([]);
    input.value = "what drives RESET?";
    form.dispatchEvent(new Event("submit", { cancelable: true }));
    await vi.waitFor(() => expect(h.rooms[0].emits.filter((e) => e.name === "room.ask")).toHaveLength(1));
    expect(input.value).toBe("");
  });

  it("tells agents what the reader selected, only while the switch is on", async () => {
    const h = harness();
    await h.tab.follow({ mount: "m", path: "a" });
    h.tab.select({ level: "entity", sel: { kind: "component", ref: "U7" } });
    expect(h.rooms[0].emits).toEqual([]);
    const box = h.host.querySelector<HTMLInputElement>("input[type=checkbox]")!;
    box.checked = true;
    box.dispatchEvent(new Event("change"));
    h.tab.select({ level: "sheet", sheetId: "s2" });
    expect(h.rooms[0].emits).toEqual([{ name: "page.selection_changed", data: { selection: { level: "sheet", sheetId: "s2", view: "canvas" } } }]);
  });

  // An agent hears which view a pick came from (agni issue 1370), so a pick on the board view beside
  // the schematic reads as one.
  it("names the sheet view a selection was made in", async () => {
    const h = harness({ stored: "1" });
    await h.tab.follow({ mount: "m", path: "a" });
    h.tab.select({ level: "entity", sel: { kind: "component", ref: "U7" }, sheetId: "board" }, "sheetview-3");
    expect(h.rooms[0].emits).toEqual([
      { name: "page.selection_changed", data: { selection: { level: "entity", sel: { kind: "component", ref: "U7" }, sheetId: "board", view: "sheetview-3" } } },
    ]);
  });
  it("offers a pairing code on a click, and shows it for the reader to give an agent", async () => {
    const h = harness();
    await h.tab.follow({ mount: "m", path: "a" });
    const code = h.host.querySelector<HTMLElement>("[data-pair-code]")!;
    expect(code.hidden).toBe(true);
    h.host.querySelector<HTMLButtonElement>("button.agents-pair")!.click();
    expect(h.rooms[0].offers).toEqual([{ ttlMs: 10 * 60_000 }]);
    expect(code.hidden).toBe(false);
    expect(code.dataset.pairCode).toBe("CODE1");
    expect(code.textContent).toContain("CODE1");
    expect(code.textContent).toContain("pair");
  });

  it("lists an agent that paired by the code, and revokes it from its row", async () => {
    const h = harness();
    await h.tab.follow({ mount: "m", path: "a" });
    h.host.querySelector<HTMLButtonElement>("button.agents-pair")!.click();
    const rows = () => [...h.host.querySelectorAll<HTMLElement>(".agents-grants li")];
    expect(rows()).toHaveLength(0);
    h.rooms[0].pair({ subject: "local:ada", client: "ada-agent" }, "CODE1");
    expect(h.host.querySelector<HTMLElement>("[data-pair-code]")!.hidden).toBe(true);
    expect(rows().map((r) => r.querySelector(".agents-grant-name")!.textContent)).toEqual(["local:ada (ada-agent)"]);
    rows()[0].querySelector<HTMLButtonElement>("button.agents-revoke")!.click();
    expect(h.rooms[0].grantList).toEqual([]);
    expect(rows()).toHaveLength(0);
  });

  it("forgets the code and the paired agents when the tab moves to another design's room", async () => {
    const h = harness();
    await h.tab.follow({ mount: "m", path: "a" });
    h.host.querySelector<HTMLButtonElement>("button.agents-pair")!.click();
    h.rooms[0].pair({ subject: "local:ada" }, "CODE1");
    h.host.querySelector<HTMLButtonElement>("button.agents-pair")!.click();
    await h.tab.follow({ mount: "m", path: "b" });
    expect(h.host.querySelector<HTMLElement>("[data-pair-code]")!.hidden).toBe(true);
    expect(h.host.querySelectorAll(".agents-grants li")).toHaveLength(0);
  });
});
