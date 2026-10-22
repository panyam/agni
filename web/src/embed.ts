import type { JsonValue } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import type { PageDispatch } from "./pageservice.js";

// embed.ts is the postMessage transport for ViewerPageService (agni issue 1152): a parent page that
// frames the viewer drives it without reloading it. A request is
//
//   { agni: "agni-viewer/v1", id, method: "LocateVerdict", body: { id: "demo-pull-up:(net:SDA)" } }
//
// and the page answers the window that sent it, at that window's origin, with the same id:
//
//   { agni: "agni-viewer/v1", id, ok: true, body: { resolved: true, location: { ... } } }
//   { agni: "agni-viewer/v1", id, ok: false, error: { code: "invalid_argument", message: "..." } }
//
// body is the rpc's request and response in protojson, so the envelope is the whole protocol and a
// parent in any language can speak it. embedclient.ts wraps it as a Connect transport for a parent
// written in TypeScript.
//
// Once listening the page posts { agni, ready: true } to its parent, so a parent need not guess when
// the viewer has booted. Requests are answered one at a time, in the order they arrive.

export const ENVELOPE = "agni-viewer/v1";

// PageRequest is one call from a host.
export interface PageRequest {
  agni: typeof ENVELOPE;
  id: string | number;
  method: string;
  body?: JsonValue;
}

// PageReply answers one PageRequest. code is a Connect code's wire name ("not_found").
export type PageReply =
  | { agni: typeof ENVELOPE; id: string | number; ok: true; body: JsonValue }
  | { agni: typeof ENVELOPE; id: string | number; ok: false; error: { code: string; message: string } };

// parseRequest reads a message as a PageRequest, null for anything that is not one, since a page
// receives messages from extensions and other frames that are not meant for it.
export function parseRequest(data: unknown): PageRequest | null {
  if (typeof data !== "object" || data === null) return null;
  const d = data as Record<string, unknown>;
  if (d.agni !== ENVELOPE || typeof d.method !== "string") return null;
  if (typeof d.id !== "string" && typeof d.id !== "number") return null;
  return { agni: ENVELOPE, id: d.id, method: d.method, body: d.body as JsonValue | undefined };
}

// originAllowed says whether a page at `origin` may drive this one: its own origin always, and
// otherwise only an origin the server named (`agni serve --embed-origin`). An opaque origin ("null",
// a sandboxed frame or a file) never matches.
export function originAllowed(origin: string, self: string, allowed: readonly string[]): boolean {
  if (origin === "null" || origin === "") return false;
  return origin === self || allowed.includes(origin);
}

// embedOrigins reads the origins the server allows, space-separated on the shell's .app element.
export function embedOrigins(attr: string | undefined): string[] {
  return (attr ?? "").split(/\s+/).filter((o) => o !== "");
}

// HostWindow is the slice of a window the transport uses, so a test can stand two fakes in for the
// page and its parent.
export interface HostWindow {
  addEventListener(type: "message", fn: (ev: MessageEvent) => void): void;
  removeEventListener(type: "message", fn: (ev: MessageEvent) => void): void;
  location: { origin: string };
  parent: { postMessage(msg: unknown, targetOrigin: string): void } | null;
}

// listenForHosts answers PageRequests posted to `win` from an allowed origin, and announces itself to
// the parent frame. A request from any other origin is answered permission_denied, so a host that is
// not allowed learns why rather than waiting. It returns a function that stops listening.
export function listenForHosts(win: HostWindow, dispatch: PageDispatch, allowed: readonly string[]): () => void {
  const self = win.location.origin;
  let queue: Promise<unknown> = Promise.resolve();
  const onMessage = (ev: MessageEvent): void => {
    const req = parseRequest(ev.data);
    const source = ev.source as { postMessage(msg: unknown, targetOrigin: string): void } | null;
    if (!req || !source) return;
    const reply = (r: PageReply) => source.postMessage(r, ev.origin);
    if (!originAllowed(ev.origin, self, allowed)) {
      reply({ agni: ENVELOPE, id: req.id, ok: false, error: { code: "permission_denied", message: `this viewer does not take requests from ${ev.origin}; start it with --embed-origin ${ev.origin}` } });
      return;
    }
    queue = queue.then(async () => {
      try {
        reply({ agni: ENVELOPE, id: req.id, ok: true, body: await dispatch(req.method, req.body) });
      } catch (e) {
        const err = ConnectError.from(e);
        reply({ agni: ENVELOPE, id: req.id, ok: false, error: { code: codeName(err.code), message: err.rawMessage } });
      }
    });
  };
  win.addEventListener("message", onMessage);
  const parent = win.parent;
  if (parent && parent !== (win as unknown)) {
    for (const origin of new Set([self, ...allowed])) parent.postMessage({ agni: ENVELOPE, ready: true }, origin);
  }
  return () => win.removeEventListener("message", onMessage);
}

// codeName is a Connect code's wire spelling, "invalid_argument" for Code.InvalidArgument.
export function codeName(code: Code): string {
  return Code[code].replace(/([a-z])([A-Z])/g, "$1_$2").toLowerCase();
}
