// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { Code, ConnectError } from "@connectrpc/connect";
import type { JoinToolRoomOptions, PageTool, ToolRoom } from "@panyam/mcpkit-rooms";
import { ViewerPageService } from "./gen/agni/v1/viewer/page_pb.js";
import type { JoinViewerRoomResponse } from "./gen/agni/v1/webapi/viewerroom_pb.js";
import type { PageDispatch } from "./pageservice.js";
import { AGENTS_KEY, ViewerRoomTab, type RoomClient } from "./viewerroom.js";
import { toolSchema } from "./toolschema.js";

// FakeRoom records what the tab does with its room.
class FakeRoom {
  tools = new Map<string, PageTool>();
  label = "";
  allowed: boolean[] = [];
  focused = 0;
  left = false;
  emits: { name: string; data: unknown }[] = [];
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
  emit(name: string, data?: unknown) {
    this.emits.push({ name, data });
  }
  ask(text: string) {
    this.emit("room.ask", { text });
  }
  selectionChanged(selection: unknown) {
    this.emit("page.selection_changed", { selection });
  }
}

function harness(opts: { stored?: string; joinFails?: boolean; dispatch?: PageDispatch } = {}) {
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
  const win = { location: { href: "http://127.0.0.1:8080/designs/m/a/view" } as Location, addEventListener: (t: string, f: () => void) => void (winListeners[t] = f) };
  const host = document.createElement("div");
  host.hidden = true;
  const dispatch = opts.dispatch ?? vi.fn(async (method: string, body: unknown) => ({ method, body }) as never);
  const tab = new ViewerRoomTab({
    client,
    dispatch,
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
      "viewer_apply_location", "viewer_get_location", "viewer_locate_entity", "viewer_locate_verdict", "viewer_select_finding", "viewer_show_sheet",
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

  // The Ask box (agni issue 1173): the reader's question goes to the agents the tab agreed to, with
  // where the tab is and what is selected, so the agent can answer without asking back.
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
    h.tab.select(focus);
    h.rooms[0].emits.length = 0;
    expect(await h.tab.ask("why is SDA flagged?")).toBe(true);
    expect(h.dispatch).toHaveBeenCalledWith("GetLocation", {});
    expect(h.rooms[0].emits).toEqual([
      { name: "room.ask", data: { text: "why is SDA flagged?", location: { method: "GetLocation", body: {} }, selection: focus } },
    ]);
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
    expect(h.rooms[0].emits).toEqual([{ name: "page.selection_changed", data: { selection: { level: "sheet", sheetId: "s2" } } }]);
  });
});
