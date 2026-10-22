import { ENVELOPE, type PageReply } from "./embed.js";

// embedclient.ts is the parent's half of the postMessage transport (agni issue 1152), as a `fetch`
// a Connect transport can send through, so a parent written in TypeScript drives a framed viewer with
// the generated client:
//
//   const transport = createConnectTransport({ baseUrl: "https://viewer.invalid", fetch: frameFetch(frame.contentWindow, viewerOrigin, window) });
//   const page = createClient(ViewerPageService, transport);
//   await page.locateVerdict({ id: "demo-pull-up:(net:SDA)" });
//
// The baseUrl is never fetched. Only unary calls in Connect's JSON form are carried, which is every
// rpc ViewerPageService has; a parent in another language posts the envelope embed.ts describes.

// FrameTarget is the framed viewer's window, as the parent sees it.
export interface FrameTarget {
  postMessage(msg: unknown, targetOrigin: string): void;
}

// ReplySource is the parent's own window, where the viewer's replies arrive.
export interface ReplySource {
  addEventListener(type: "message", fn: (ev: MessageEvent) => void): void;
  removeEventListener(type: "message", fn: (ev: MessageEvent) => void): void;
}

let nextId = 0;

// frameFetch sends each request to `target` at `origin` and resolves with the viewer's reply as an
// HTTP response: 200 with the response body, or a Connect error body the client turns back into a
// ConnectError with the page's code. A reply from any other window or origin is ignored.
export function frameFetch(target: FrameTarget, origin: string, self: ReplySource): typeof fetch {
  return async (input, init) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    const method = new URL(url).pathname.replace(/^\//, "");
    const text = await bodyText(init?.body);
    const id = `agni-${++nextId}`;
    const reply = await new Promise<PageReply>((resolve) => {
      const onMessage = (ev: MessageEvent): void => {
        const d = ev.data as PageReply | undefined;
        if (ev.source !== (target as unknown) || ev.origin !== origin || d?.agni !== ENVELOPE || d.id !== id) return;
        self.removeEventListener("message", onMessage);
        resolve(d);
      };
      self.addEventListener("message", onMessage);
      target.postMessage({ agni: ENVELOPE, id, method, body: JSON.parse(text) }, origin);
    });
    const headers = { "content-type": "application/json" };
    if (reply.ok) return new Response(JSON.stringify(reply.body), { status: 200, headers });
    return new Response(JSON.stringify(reply.error), { status: 400, headers });
  };
}

// bodyText reads a request body as Connect's JSON codec sends it, bytes or a string.
async function bodyText(body: BodyInit | null | undefined): Promise<string> {
  if (body == null) return "{}";
  if (typeof body === "string") return body;
  return new Response(body).text();
}
