import { Code, ConnectError } from "@connectrpc/connect";
import type { JsonValue } from "@bufbuild/protobuf";
import { joinToolRoom, type JoinToolRoomOptions, type ToolRoom } from "@panyam/mcpkit-rooms";
import { ViewerPageService } from "./gen/agni/v1/viewer/page_pb.js";
import type { JoinViewerRoomRequest, JoinViewerRoomResponse } from "./gen/agni/v1/webapi/viewerroom_pb.js";
import type { PageDispatch } from "./pageservice.js";
import type { ViewerLocation } from "./router.js";
import { readOnly, toolName, toolSchema } from "./toolschema.js";

// viewerroom.ts lets an agent drive this tab (agni issue 1154). Under `agni serve --mcp
// --viewer-rooms` the tab joins its design's relay room and offers each ViewerPageService method
// there as a tool (viewer_show_sheet, viewer_locate_verdict, ...), answered by the same dispatcher
// postMessage uses (pageservice.ts). The server's room bridge serves those tools to agents over
// /mcp, each call naming a design, and sends a call to a tab only once its reader turns on "Let
// agents drive this tab". The switch is per tab and off by default, and survives a reload of the tab.

// RoomClient is the one ViewerRoomService call the tab makes, which the server answers.
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

// ViewerRoomTab is this tab's membership in its design's room, following the page from design to
// design.
export class ViewerRoomTab {
  private room: ToolRoom | undefined;
  private design = "";
  // queue runs joins and leaves one at a time, in the order the page moved.
  private queue: Promise<void> = Promise.resolve();
  private readonly checkbox: HTMLInputElement | undefined;
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
    // The bridge sends a call naming no tab to the one its reader looked at last.
    this.win.addEventListener("focus", () => this.room?.focus());
    this.doc.addEventListener("visibilitychange", () => {
      if (this.doc.visibilityState === "visible") this.room?.focus();
    });
  }

  // agentsAllowed is the switch as the reader left it.
  agentsAllowed(): boolean {
    return this.storage?.getItem(AGENTS_KEY) === "1";
  }

  // follow puts the tab in the room of the design loc shows, leaving the room it was in. A location
  // naming no design leaves. It resolves once the tab is in the room, or has failed to join, which it
  // logs rather than throws, since a page with no room still works.
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
    this.showSwitch(false);
    if (!design || design !== this.design) return;
    try {
      const res = await this.deps.client.joinViewerRoom({ design });
      const room = await this.join({
        relay: new URL(res.relayUrl, this.win.location.href).href,
        room: res.room,
        token: res.token,
        username: "agni viewer",
      });
      if (design !== this.design) {
        room.leave();
        return;
      }
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
    });
    label.append(box, " Let agents drive this tab");
    host.append(label);
    return box;
  }

  private showSwitch(on: boolean): void {
    if (this.deps.host) this.deps.host.hidden = !on;
    if (this.checkbox) this.checkbox.checked = this.agentsAllowed();
  }
}

function safeSessionStorage(): Storage | undefined {
  try {
    return globalThis.sessionStorage;
  } catch {
    return undefined;
  }
}
