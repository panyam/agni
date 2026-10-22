import { create, fromJson, toJson, type DescMethod, type JsonValue, type MessageInitShape, type MessageShape } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { ViewerLocationSchema, ViewerPageService, type ViewerLocation as WireLocation } from "./gen/agni/v1/viewer/page_pb.js";
import type { ViewerPageAPI } from "./pageapi.js";
import type { ViewerLocation } from "./router.js";
import type { RenderMode } from "./viewer.js";

// pageservice.ts answers ViewerPageService (protos/agni/v1/viewer/page.proto) over a ViewerPageAPI,
// for any transport that can carry a method name and a protojson body (agni issue 1152). postMessage
// carries them today (embed.ts); a server-held session and an MCP app host are the same pair
// arriving another way (agni issues 1154, 1155), so neither needs a handler of its own.

// PageDispatch answers one call. method is the rpc's name, bare ("LocateVerdict") or qualified as a
// Connect path names it ("agni.v1.viewer.ViewerPageService/LocateVerdict", with or without a leading
// slash), and body is the request in protojson. It resolves to the response in protojson and
// rejects with a ConnectError: unimplemented for a method the service lacks, invalid_argument for a
// body that does not parse or a request the page cannot act on.
export type PageDispatch = (method: string, body: JsonValue | undefined) => Promise<JsonValue>;

type Methods = typeof ViewerPageService.method;
// PageImpl is one handler per rpc, so the service growing a method is a compile error here until it
// is answered.
type PageImpl = {
  [K in keyof Methods]: (req: MessageShape<Methods[K]["input"]>) => Promise<MessageInitShape<Methods[K]["output"]>>;
};

const MODES: readonly RenderMode[] = ["svg", "webgl", "native"];

// pageDispatcher builds the dispatch for one page.
export function pageDispatcher(api: ViewerPageAPI): PageDispatch {
  const here = () => locationToWire(api.location());
  const impl: PageImpl = {
    getLocation: async () => ({ location: here() }),
    applyLocation: async (req) => {
      const { location, verdictResolved } = await api.applyLocation(locationFromWire(req.location));
      return { location: locationToWire(location), verdictResolved };
    },
    locateVerdict: async (req) => ({ resolved: await api.locateVerdict(req.id, { rule: req.rule, subjects: req.subjects }), location: here() }),
    selectFinding: async (req) => ({ selected: await api.selectFinding(req.subject, { sheet: req.sheet || undefined, netId: req.netId }), location: here() }),
    locateEntity: async (req) => {
      await api.locateEntity(req.kind, req.subject, { pin: req.pin, sheet: req.sheet || undefined });
      return { location: here() };
    },
    showSheet: async (req) => {
      await api.showSheet(req.sheet);
      return { location: here() };
    },
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

// locationFromWire reads a requested location, keeping only what it sets: an empty string, an empty
// list, a zero and an unset optional are left out, which ViewerPageAPI.applyLocation reads as "leave
// it as it is". A mode the viewer has no renderer for is refused rather than ignored.
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
