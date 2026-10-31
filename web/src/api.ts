// Connect clients for the WS9 web API. The service contracts are generated from proto
// (CONSTRAINTS C2), and this module only wires a browser transport onto them. The view layer
// calls these clients instead of hand-rolling fetch/JSON.
import { LexiconService } from "./gen/agni/v1/webapi/lexicon_pb.js";
import { createClient, type Client, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";
import { DesignService } from "./gen/agni/v1/webapi/design_pb.js";
import { CheckService } from "./gen/agni/v1/webapi/checks_pb.js";
import { DiffService } from "./gen/agni/v1/webapi/diff_pb.js";
import { ProjectService } from "./gen/agni/v1/webapi/project_pb.js";
import { QueryService } from "./gen/agni/v1/webapi/query_pb.js";
import { ReviewService } from "./gen/agni/v1/webapi/review_pb.js";
import { TableService } from "./gen/agni/v1/webapi/tables_pb.js";
import { SnapshotService } from "./gen/agni/v1/webapi/snapshot_pb.js";
import { SnapshotRegistry, recoveringInterceptor } from "./snapshots.js";
import { pageFlattens } from "./router.js";

// engineFetch, when set, answers every client's requests in place of the network, which is how the
// in-browser engine serves the same clients `agni serve` does (agni issue 178).
let engineFetch: typeof globalThis.fetch | undefined;

// useEngineFetch routes every client created after it through f. Call it before the page builds its
// clients.
export function useEngineFetch(f: typeof globalThis.fetch): void {
  engineFetch = f;
}

// timingMode is the page address's ?timing value: "" when absent, "explain" to ask for query plans
// too, anything else for the timings alone (agni issue 914).
export function timingMode(): string {
  try {
    const v = new URLSearchParams(globalThis.location?.search ?? "").get("timing");
    return v === null ? "" : v || "1";
  } catch {
    return "";
  }
}

// timingInterceptor asks each request where its time went, with the Agni-Timing header, and logs the
// breakdown the engine sends back as "agni timing <rpc>: <RequestTiming as JSON>". The browser engine
// answers it as a server does, so the same breakdown shows for a design read in the page.
//
// The logged JSON also carries `page`, the request's start and wall time on the page's clock, so the
// time a request spent waiting outside the engine (queued behind a worker, or in transit) is the
// difference from totalMicros (agni issue 968). A request that throws is logged with its `error`, since
// its time counts as much as an answered one's.
export function timingInterceptor(mode: string): Interceptor {
  return (next) => async (req) => {
    req.header.set("Agni-Timing", mode);
    const start = performance.now();
    const page = () => ({ startMs: start, wallMs: performance.now() - start });
    let res;
    try {
      res = await next(req);
    } catch (e) {
      console.info(`agni timing ${req.method.name}: ${JSON.stringify({ page: page(), error: e instanceof Error ? e.message : String(e) })}`);
      throw e;
    }
    const t = res.header.get("Agni-Timing");
    if (t) console.info(`agni timing ${req.method.name}: ${withPage(t, page())}`);
    return res;
  };
}

// withPage adds the page's clock to the engine's breakdown, or leaves a breakdown that is not a JSON
// object as it came.
function withPage(t: string, page: { startMs: number; wallMs: number }): string {
  try {
    const v = JSON.parse(t);
    if (v && typeof v === "object" && !Array.isArray(v)) return JSON.stringify({ ...v, page });
  } catch {
    // Logged as sent.
  }
  return t;
}

// newTransport builds a Connect transport rooted at baseUrl. It defaults to "/" so the
// app talks to the same origin that served it (the `agni serve` dev server). A question whose
// snapshot the host no longer holds is recovered through the page's registry and asked again
// (agni issue 1061), except on the transport the registry itself ingests through.
export function newTransport(baseUrl = "/", recover = true) {
  const mode = timingMode();
  const interceptors = mode ? [timingInterceptor(mode)] : [];
  if (recover) interceptors.push(recoveringInterceptor(() => registry));
  if (engineFetch) return createConnectTransport({ baseUrl: new URL(baseUrl, location.href).href, fetch: engineFetch, interceptors });
  return createConnectTransport({ baseUrl, interceptors });
}

let registry: SnapshotRegistry | undefined;

// snapshots is the page's one SnapshotRegistry, made on first use over the engine the page chose,
// so every client asks about the same snapshots.
export function snapshots(): SnapshotRegistry {
  registry ??= new SnapshotRegistry(createClient(SnapshotService, newTransport("/", false)), undefined, { flatten: pageFlattens(globalThis.location?.search ?? "") });
  return registry;
}

// resetSnapshots forgets the page's registry, for a test that changes the engine between pages.
export function resetSnapshots(): void {
  registry = undefined;
}

// workspaceClient returns a typed client for WorkspaceService (mounts and their contents).
export function workspaceClient(baseUrl?: string): Client<typeof WorkspaceService> {
  return createClient(WorkspaceService, newTransport(baseUrl));
}

// designClient returns a typed client for DesignService (design summary + packed sheets).
export function designClient(baseUrl?: string): Client<typeof DesignService> {
  return createClient(DesignService, newTransport(baseUrl));
}

// checksClient returns a typed client for CheckService (rule catalog, check runs, the
// severity report, and expectation sidecars; WS9-026).
export function checksClient(baseUrl?: string): Client<typeof CheckService> {
  return createClient(CheckService, newTransport(baseUrl));
}

// lexiconClient returns a typed client for LexiconService (agni issues 1266, 1267).
export function lexiconClient(baseUrl?: string): Client<typeof LexiconService> {
  return createClient(LexiconService, newTransport(baseUrl));
}

// diffClient returns a typed client for DiffService (the semantic diff between two designs
// plus the highlight maps the visual diff joins to geometry, WS9-005).
export function diffClient(baseUrl?: string): Client<typeof DiffService> {
  return createClient(DiffService, newTransport(baseUrl));
}

// projectClient returns a typed client for ProjectService (which project a design belongs to, and
// the config that project supplies).
export function projectClient(baseUrl?: string): Client<typeof ProjectService> {
  return createClient(ProjectService, newTransport(baseUrl));
}

// queryClient returns a typed client for QueryService (ad-hoc datalog queries over a design's
// fact base, the same engine `agni query` runs; WS9-036, WS3-029).
export function queryClient(baseUrl?: string): Client<typeof QueryService> {
  return createClient(QueryService, newTransport(baseUrl));
}

// tableClient returns a typed client for TableService (an answer projected into tables, and tables
// encoded as files; agni issues 898 and 956).
export function tableClient(baseUrl?: string): Client<typeof TableService> {
  return createClient(TableService, newTransport(baseUrl));
}

// reviewClient returns a typed client for ReviewService (review runs as resources, plus
// GetReviewManifest to resolve a stored checklist into the value a create takes; WS9-052).
export function reviewClient(baseUrl?: string): Client<typeof ReviewService> {
  return createClient(ReviewService, newTransport(baseUrl));
}
