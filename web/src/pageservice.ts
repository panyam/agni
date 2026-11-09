import { create, fromJson, toJson, type DescMethod, type JsonValue, type MessageInitShape, type MessageShape } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { ViewerLocationSchema, ViewerPageService, type ViewerLocation as WireLocation } from "./gen/agni/v1/viewer/page_pb.js";
import type { ViewerPageAPI } from "./pageapi.js";
import { hasFile, parseUrl, type ViewerLocation } from "./router.js";
import type { RenderMode } from "./viewer.js";

// pageservice.ts answers ViewerPageService (protos/agni/v1/viewer/page.proto) over a ViewerPageAPI,
// for any transport that can carry a method name and a protojson body (agni issue 1152).
// postMessage carries them today (embed.ts). A server-held session and an MCP app host send the
// same pair another way (agni issues 1154, 1155), so neither needs a handler of its own.

// PageDispatch answers one call. method is the rpc's name. It can be bare ("LocateVerdict") or
// qualified as a Connect path ("agni.v1.viewer.ViewerPageService/LocateVerdict"), with or without a
// leading slash. body is the request in protojson. It resolves to the response in protojson. It
// rejects with a ConnectError, unimplemented for a method the service lacks and invalid_argument
// for a body that does not parse or a request the page cannot act on.
export type PageDispatch = (method: string, body: JsonValue | undefined) => Promise<JsonValue>;

type Methods = typeof ViewerPageService.method;
// PageImpl is one handler per rpc, so a new method in the service is a compile error here until it
// has a handler.
type PageImpl = {
  [K in keyof Methods]: (req: MessageShape<Methods[K]["input"]>) => Promise<MessageInitShape<Methods[K]["output"]>>;
};

const MODES: readonly RenderMode[] = ["svg", "webgl", "native"];

// pageDispatcher builds the dispatch for one page.
export function pageDispatcher(api: ViewerPageAPI): PageDispatch {
  const here = () => locationToWire(api.location());
  const impl: PageImpl = {
    getLocation: async () => ({ location: here(), views: api.sheetViews() }),
    applyLocation: async (req) => {
      if (req.url && req.location) throw new ConnectError("ApplyLocation takes a location or a url, not both", Code.InvalidArgument);
      const { location, verdictResolved } = await api.applyLocation(req.url ? locationFromLink(req.url) : locationFromWire(req.location));
      return { location: locationToWire(location), verdictResolved };
    },
    locateVerdict: async (req) => ({ resolved: await api.locateVerdict(req.id, { rule: req.rule, subjects: req.subjects }), location: here() }),
    selectFinding: async (req) => ({ selected: await api.selectFinding(req.subject, { sheet: req.sheet || undefined, netId: req.netId }), location: here() }),
    locateEntity: async (req) => {
      await api.locateEntity(req.kind, req.subject, { pin: req.pin, sheet: req.sheet || undefined });
      return { location: here() };
    },
    showSheet: async (req) => {
      await api.showSheet(req.sheet, req.view || undefined);
      return { location: here(), views: api.sheetViews() };
    },
    openSheetView: async (req) => {
      const view = await api.openSheetView(req.sheet, req.renderer);
      return { view, location: here(), views: api.sheetViews() };
    },
    closeSheetView: async (req) => ({ closed: api.closeSheetView(req.view), location: here(), views: api.sheetViews() }),
  };
  return async (method, body) => {
    const m = findMethod(method);
    if (!m) throw new ConnectError(`ViewerPageService has no method "${method}"`, Code.Unimplemented);
    let req: MessageShape<DescMethod["input"]>;
    try {
      req = fromJson(m.input, body ?? {});
    } catch (e) {
      throw new ConnectError(`${m.name}: ${e instanceof Error ? e.message : String(e)}`, Code.InvalidArgument);
    }
    const handler = impl[m.localName as keyof PageImpl] as (r: typeof req) => Promise<MessageInitShape<DescMethod["output"]>>;
    let res: MessageInitShape<DescMethod["output"]>;
    try {
      res = await handler(req);
    } catch (e) {
      throw e instanceof ConnectError ? e : new ConnectError(e instanceof Error ? e.message : String(e), Code.InvalidArgument);
    }
    return toJson(m.output, create(m.output, res));
  };
}

// isLinkOnly says whether a call is ApplyLocation carrying a url and nothing else. The page takes
// that one request from any origin, since loading the link would do the same (agni issue 1156). A
// body that does not parse is not one.
export function isLinkOnly(method: string, body: JsonValue | undefined): boolean {
  const m = findMethod(method);
  if (m?.name !== "ApplyLocation") return false;
  try {
    const req = fromJson(ViewerPageService.method.applyLocation.input, body ?? {});
    return req.url !== "" && req.location === undefined;
  } catch {
    return false;
  }
}

// locationFromLink reads a viewer link as the location it addresses, keeping only what it sets. It
// uses the same parser that reads the page's own address. The link's host is not compared with the
// page's, since a link minted for localhost names the same design on 127.0.0.1. A link to no design
// is refused.
export function locationFromLink(url: string): Partial<ViewerLocation> {
  let u: URL;
  try {
    u = new URL(url, "http://link.invalid/");
  } catch {
    throw new ConnectError(`"${url}" is not a URL`, Code.InvalidArgument);
  }
  const loc = parseUrl(u.pathname, u.search);
  if (!hasFile(loc)) throw new ConnectError(`"${url}" is not a link to a design in this viewer`, Code.InvalidArgument);
  const out = locationFromWire(locationToWire(loc));
  // A link can only say the two switches are on, so an absent one leaves the page's own alone rather
  // than turning it off.
  if (!loc.symbols) delete out.symbols;
  if (!loc.flatten) delete out.flatten;
  return out;
}

function findMethod(name: string): DescMethod | undefined {
  const short = name.replace(/^\//, "");
  const prefix = `${ViewerPageService.typeName}/`;
  const bare = short.startsWith(prefix) ? short.slice(prefix.length) : short;
  return ViewerPageService.methods.find((m) => m.name === bare);
}

// locationToWire spells a full location as the wire message, every field set.
export function locationToWire(loc: ViewerLocation): WireLocation {
  return create(ViewerLocationSchema, {
    mount: loc.mount,
    path: loc.path,
    sheet: loc.sheet,
    mode: loc.mode,
    layout: loc.layout,
    symbols: loc.symbols,
    flatten: loc.flatten,
    verdict: loc.verdict,
    hash: loc.hash,
    rule: loc.rule,
    subjects: [...loc.subjects],
    trace: loc.trace,
    traceHops: loc.traceHops,
    query: loc.query,
    title: loc.title,
    binds: [...loc.binds],
    set: loc.set,
    q: loc.q,
    against: loc.against,
    renameApprox: loc.renameApprox,
  });
}

// locationFromWire reads a requested location, keeping only what it sets. It leaves out an empty
// string, an empty list, a zero and an unset optional, and ViewerPageAPI.applyLocation reads each
// absence as "leave it as it is". A mode the viewer has no renderer for is refused rather than
// ignored.
export function locationFromWire(w: WireLocation | undefined): Partial<ViewerLocation> {
  if (!w) return {};
  if (w.mode && !MODES.includes(w.mode as RenderMode)) {
    throw new ConnectError(`mode "${w.mode}" is not one of ${MODES.join(", ")}`, Code.InvalidArgument);
  }
  const out: Partial<ViewerLocation> = {};
  const strings = ["mount", "path", "sheet", "layout", "verdict", "hash", "rule", "trace", "query", "title", "set", "q", "against"] as const;
  for (const k of strings) if (w[k]) out[k] = w[k];
  if (w.mode) out.mode = w.mode as RenderMode;
  if (w.symbols !== undefined) out.symbols = w.symbols;
  if (w.flatten !== undefined) out.flatten = w.flatten;
  if (w.subjects.length > 0) out.subjects = [...w.subjects];
  if (w.binds.length > 0) out.binds = [...w.binds];
  if (w.traceHops) out.traceHops = w.traceHops;
  if (w.renameApprox) out.renameApprox = true;
  return out;
}
