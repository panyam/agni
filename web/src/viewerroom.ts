import { Code, ConnectError } from "@connectrpc/connect";
import type { JsonValue } from "@bufbuild/protobuf";
import { EVENT_ASK, joinToolRoom, type JoinToolRoomOptions, type Principal, type ToolRefusal, type ToolRoom } from "@panyam/mcpkit-rooms";
import { ViewerPageService } from "@panyam/agni-client/gen/agni/v1/viewer/page_pb";
import type { JoinViewerRoomRequest, JoinViewerRoomResponse } from "@panyam/agni-client/gen/agni/v1/webapi/viewerroom_pb";
import type { PageDispatch } from "./pageservice.js";
import type { Focus } from "./properties.js";
import type { ViewerLocation } from "./router.js";
import { readOnly, toolName, toolSchema } from "./toolschema.js";

// viewerroom.ts lets an agent drive this tab (agni issue 1154). Under `agni serve --mcp
// --viewer-rooms` the tab joins its design's relay room and offers each ViewerPageService method
// there as a tool (viewer_show_sheet, viewer_locate_verdict, ...). The dispatcher that answers
// postMessage (pageservice.ts) answers these tools too. The server's room bridge serves those tools
// to agents over /mcp, each call naming a design. It sends a call to a tab only once its reader
// turns on "Let agents drive this tab". The switch is per tab and off by default, and survives a
// reload of the tab.
//
// The other direction goes through the same agreement (agni issue 1173). With the switch on, the tab
// tells the room what the reader selected, and the Ask box beside the switch sends the reader's
// question with the tab's location and selection. The bridge passes both to the agents that the tab
// agreed to. Those agents hear them through next_request or the room's events resource.
//
// Another person's agent gets in by pairing (agni issue 1903). The reader clicks "Pair an agent", the
// tab shows a code, and the agent enters it with the bridge's pair tool. Each paired agent gets a row
// with a Revoke button. The tab holds its pairings in memory, so a reload or a move to another
// design ends them.

// RoomClient holds the one ViewerRoomService call that the tab makes to the server.
export interface RoomClient {
  joinViewerRoom(req: Partial<JoinViewerRoomRequest>): Promise<JoinViewerRoomResponse>;
}

export interface ViewerRoomDeps {
  client: RoomClient;
  dispatch: PageDispatch;
  // host is where the switch renders, hidden until the tab is in a room.
  host: HTMLElement | null;
  join?: (opts: JoinToolRoomOptions) => Promise<ToolRoom>;
  storage?: Pick<Storage, "getItem" | "setItem">;
  win?: Pick<Window, "addEventListener" | "location">;
  doc?: Pick<Document, "addEventListener" | "hasFocus" | "visibilityState" | "createElement">;
}

// AGENTS_KEY holds the switch in sessionStorage, which is per tab, so a reload keeps it and a new tab
// starts with it off.
export const AGENTS_KEY = "agni.viewerRooms.agentsAllowed";

// CANVAS_VIEW is the view a selection names when no sheet view made it, a pick on the canvas or a
// row or link anywhere else on the page. A sheet view names itself by its id, "sheetview-<n>", the
// same id the page contract's OpenSheetView returns (agni issue 1370).
export const CANVAS_VIEW = "canvas";

// PAIR_CODE_TTL_MS is how long a pairing code works, the most a bridge honours.
export const PAIR_CODE_TTL_MS = 10 * 60_000;

// RoomSelection is a selection as agents hear it, with the view it was made in.
export type RoomSelection = Focus & { view: string };

// ViewerRoomTab is this tab's membership in its design's room, following the page from design to
// design.
export class ViewerRoomTab {
  private room: ToolRoom | undefined;
  private design = "";
  // queue runs joins and leaves one at a time, in the order the page moved.
  private queue: Promise<void> = Promise.resolve();
  private readonly checkbox: HTMLInputElement | undefined;
  private readonly askForm: HTMLFormElement | undefined;
  // refusedNote names the tools a bridge refused to offer agents, hidden while it refused none.
  private readonly refusedNote: HTMLElement | undefined;
  private refused = new Map<string, string>();
  // pairing holds the pair button, the code it offered and the agents paired with the tab.
  private readonly pairing: { code: HTMLElement; grants: HTMLUListElement } | undefined;
  private codeTimer: ReturnType<typeof setTimeout> | undefined;
  // focus is what the reader last selected. An ask sends it along.
  private focus: RoomSelection | undefined;
  private readonly join: (opts: JoinToolRoomOptions) => Promise<ToolRoom>;
  private readonly storage: Pick<Storage, "getItem" | "setItem"> | undefined;
  private readonly win: Pick<Window, "addEventListener" | "location">;
  private readonly doc: Pick<Document, "addEventListener" | "hasFocus" | "visibilityState" | "createElement">;

  constructor(private readonly deps: ViewerRoomDeps) {
    this.join = deps.join ?? joinToolRoom;
    this.storage = deps.storage ?? safeSessionStorage();
    this.win = deps.win ?? window;
    this.doc = deps.doc ?? document;
    this.checkbox = this.renderSwitch();
    this.askForm = this.renderAsk();
    this.refusedNote = this.renderRefused();
    this.pairing = this.renderPairing();
    // When a call names no tab, the bridge sends it to the tab its reader looked at last.
    this.win.addEventListener("focus", () => this.room?.focus());
    this.doc.addEventListener("visibilitychange", () => {
      if (this.doc.visibilityState === "visible") this.room?.focus();
    });
  }

  // agentsAllowed is the switch as the reader left it.
  agentsAllowed(): boolean {
    return this.storage?.getItem(AGENTS_KEY) === "1";
  }

  // ask sends the reader's question to the agents that the tab agreed to, with the tab's location
  // and selection. It returns false when nothing was sent, because the tab is in no room or its
  // reader keeps agents out.
  async ask(text: string): Promise<boolean> {
    const room = this.room;
    if (!room || !this.agentsAllowed() || !text.trim()) return false;
    let location: unknown = null;
    try {
      location = await this.deps.dispatch("GetLocation", {});
    } catch {
      // An ask still goes without a location. The agent can read the location with
      // viewer_get_location.
    }
    room.emit(EVENT_ASK, { text: text.trim(), location, selection: this.focus ?? null });
    return true;
  }

  // select records what the reader selected and the view it was made in and, while agents may drive
  // the tab, tells them.
  select(focus: Focus, view: string = CANVAS_VIEW): void {
    const sel: RoomSelection = { ...focus, view };
    this.focus = sel;
    if (!this.room || !this.agentsAllowed()) return;
    try {
      this.room.selectionChanged(sel);
    } catch (e) {
      // An area over the event size limit is not worth an error on the page.
      console.warn(`agni: the selection was not sent to agents: ${e instanceof Error ? e.message : String(e)}`);
    }
  }

  // follow moves the tab into the room for the design that loc shows, leaving the room it was in. A
  // location naming no design leaves. The promise resolves once the tab is in the room or has failed
  // to join. A failure to join is logged rather than thrown, since a page with no room still works.
  follow(loc: Pick<ViewerLocation, "mount" | "path">): Promise<void> {
    const design = loc.mount && loc.path ? `${loc.mount}/${loc.path}` : "";
    if (design === this.design) return this.queue;
    this.design = design;
    this.queue = this.queue.then(() => this.moveTo(design));
    return this.queue;
  }

  // leave takes the tab out of its room.
  leave(): Promise<void> {
    return this.follow({ mount: "", path: "" });
  }

  private async moveTo(design: string): Promise<void> {
    this.room?.leave();
    this.room = undefined;
    this.refused.clear();
    this.showRefused();
    this.showCode("");
    this.showGrants();
    this.showSwitch(false);
    if (!design || design !== this.design) return;
    try {
      const res = await this.deps.client.joinViewerRoom({ design });
      const room = await this.join({
        relay: new URL(res.relayUrl, this.win.location.href).href,
        room: res.room,
        token: res.token,
        username: "agni viewer",
        // Answer the server's bridges alone, so a page holding a token for this room cannot call the
        // tab directly, past the bridge's check of the switch (agni issue 1173).
        trustBridges: res.bridgeSubjects,
      });
      if (design !== this.design) {
        room.leave();
        return;
      }
      // A bridge refuses a tool whose schema differs from the one it declares. That happens when the
      // page and the server come from different builds, and the twin fixture keeps it from shipping
      // (agni issue 1173).
      room.onToolRefused((r) => this.toolRefused(r));
      for (const m of ViewerPageService.methods) {
        room.registerTool({
          name: toolName(m),
          inputSchema: toolSchema(m),
          annotations: { readOnlyHint: readOnly(m) },
          execute: async (args) => {
            // The bridge passes the design through to name the room; the request has no such field.
            const { design: _design, ...body } = args;
            try {
              return await this.deps.dispatch(m.name, body as JsonValue);
            } catch (e) {
              throw e instanceof ConnectError ? new Error(`${Code[e.code]}: ${e.rawMessage}`) : e;
            }
          },
        });
      }
      room.onPaired(() => {
        this.showCode("");
        this.showGrants();
      });
      room.setLabel(design);
      room.setAgentsAllowed(this.agentsAllowed());
      if (this.doc.hasFocus()) room.focus();
      this.room = room;
      this.showSwitch(true);
    } catch (e) {
      console.warn(`agni: this tab did not join its design's room, so agents cannot drive it: ${e instanceof Error ? e.message : String(e)}`);
    }
  }

  private renderSwitch(): HTMLInputElement | undefined {
    const host = this.deps.host;
    if (!host) return undefined;
    const label = this.doc.createElement("label");
    label.className = "agents-switch-label";
    label.title = "An agent connected to this server's /mcp can move this tab: open a sheet, focus a verdict, highlight a net. Off by default, and for this tab only.";
    const box = this.doc.createElement("input");
    box.type = "checkbox";
    box.checked = this.agentsAllowed();
    box.addEventListener("change", () => {
      this.storage?.setItem(AGENTS_KEY, box.checked ? "1" : "0");
      this.room?.setAgentsAllowed(box.checked);
      if (this.askForm) this.askForm.hidden = !box.checked;
    });
    label.append(box, " Let agents drive this tab");
    host.append(label);
    return box;
  }

  // renderAsk draws the Ask box beside the switch. The box shows only while agents may drive the tab,
  // since agents that the tab has not agreed to hear nothing it sends.
  private renderAsk(): HTMLFormElement | undefined {
    const host = this.deps.host;
    if (!host) return undefined;
    const form = this.doc.createElement("form");
    form.className = "agents-ask";
    form.hidden = !this.agentsAllowed();
    const input = this.doc.createElement("input");
    input.type = "text";
    input.placeholder = "Ask your agent about this tab";
    input.title = "Sent with this tab's sheet and selection to the agents this tab lets in. An agent hears it as an event, or by calling next_request.";
    const send = this.doc.createElement("button");
    send.type = "submit";
    send.textContent = "Ask";
    form.append(input, send);
    form.addEventListener("submit", (ev) => {
      ev.preventDefault();
      const text = input.value;
      if (!text.trim()) return;
      void this.ask(text).then((sent) => {
        if (sent) input.value = "";
      });
    });
    host.append(form);
    return form;
  }

  // renderRefused draws the note beside the switch. The note names the tools that agents cannot call
  // on this tab.
  private renderRefused(): HTMLElement | undefined {
    const host = this.deps.host;
    if (!host) return undefined;
    const note = this.doc.createElement("span");
    note.className = "agents-refused";
    note.hidden = true;
    host.append(note);
    return note;
  }

  // renderPairing draws the pair button, the code it offers, and the list of agents paired with the
  // tab. They show whatever the switch says, since the switch lets in the reader's own agents and
  // pairing lets in another person's.
  private renderPairing(): { code: HTMLElement; grants: HTMLUListElement } | undefined {
    const host = this.deps.host;
    if (!host) return undefined;
    const box = this.doc.createElement("span");
    box.className = "agents-pairing";
    const pair = this.doc.createElement("button");
    pair.type = "button";
    pair.className = "agents-pair";
    pair.textContent = "Pair an agent";
    pair.title = "Shows a code for another person's agent. It enters the code with the pair tool and can then drive this tab until you revoke it.";
    pair.addEventListener("click", () => {
      if (this.room) this.showCode(this.room.offerPairingCode({ ttlMs: PAIR_CODE_TTL_MS }));
    });
    const code = this.doc.createElement("span");
    code.className = "agents-pair-code";
    code.hidden = true;
    code.dataset.pairCode = "";
    const grants = this.doc.createElement("ul");
    grants.className = "agents-grants";
    box.append(pair, code, grants);
    host.append(box);
    return { code, grants };
  }

  // showCode shows the code the tab offered, or hides it when code is empty. A shown code hides
  // itself once it stops working.
  private showCode(code: string): void {
    if (this.codeTimer !== undefined) clearTimeout(this.codeTimer);
    this.codeTimer = undefined;
    const el = this.pairing?.code;
    if (!el) return;
    el.hidden = !code;
    if (code) {
      const shown = this.doc.createElement("code");
      shown.textContent = code;
      el.replaceChildren("Give the agent code ", shown, ". It enters it with pair, once, within 10 minutes.");
    } else {
      el.replaceChildren();
    }
    el.dataset.pairCode = code;
    if (code) this.codeTimer = setTimeout(() => this.showCode(""), PAIR_CODE_TTL_MS);
  }

  // showGrants lists the agents paired with the tab, each with a button that revokes it. A revoked
  // agent's next call is refused.
  private showGrants(): void {
    const list = this.pairing?.grants;
    if (!list) return;
    const room = this.room;
    list.replaceChildren(
      ...(room?.grants() ?? []).map((g) => {
        const row = this.doc.createElement("li");
        const name = this.doc.createElement("span");
        name.className = "agents-grant-name";
        name.textContent = grantName(g);
        const revoke = this.doc.createElement("button");
        revoke.type = "button";
        revoke.className = "agents-revoke";
        revoke.textContent = "Revoke";
        revoke.addEventListener("click", () => {
          room?.revoke(g);
          this.showGrants();
        });
        row.append(name, " ", revoke);
        return row;
      }),
    );
  }

  private toolRefused(r: ToolRefusal): void {
    console.warn(`agni: the server will not offer agents ${r.name}: ${r.reason}`);
    this.refused.set(r.name, r.reason);
    this.showRefused();
  }

  private showRefused(): void {
    const note = this.refusedNote;
    if (!note) return;
    const names = [...this.refused.keys()].sort();
    note.hidden = names.length === 0;
    note.textContent = names.length ? `Agents cannot call ${names.join(", ")} on this tab` : "";
    note.title = [...this.refused].map(([n, why]) => `${n}: ${why}`).join("\n");
  }

  private showSwitch(on: boolean): void {
    if (this.deps.host) this.deps.host.hidden = !on;
    if (this.checkbox) this.checkbox.checked = this.agentsAllowed();
    if (this.askForm) this.askForm.hidden = !this.agentsAllowed();
  }
}

// grantName names a paired agent by the subject its token carries, and its client when it gave one.
function grantName(g: Principal): string {
  return g.client ? `${g.subject} (${g.client})` : g.subject;
}

function safeSessionStorage(): Storage | undefined {
  try {
    return globalThis.sessionStorage;
  } catch {
    return undefined;
  }
}
