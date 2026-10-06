// Connect clients for the WS9 web API. The service contracts are generated from proto
// (CONSTRAINTS C2), and this module only wires a browser transport onto them. The view layer
// calls these clients instead of hand-rolling fetch/JSON.
import { createClient, type Client, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";
import { DesignService } from "./gen/agni/v1/webapi/design_pb.js";
import { CheckService } from "./gen/agni/v1/webapi/checks_pb.js";
import { DiffService } from "./gen/agni/v1/webapi/diff_pb.js";
import { ProjectService } from "./gen/agni/v1/webapi/project_pb.js";
import { QueryService } from "./gen/agni/v1/webapi/query_pb.js";
import { ReviewService } from "./gen/agni/v1/webapi/review_pb.js";

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
function timingMode(): string {
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
export function timingInterceptor(mode: string): Interceptor {
  return (next) => async (req) => {
    req.header.set("Agni-Timing", mode);
    const res = await next(req);
    const t = res.header.get("Agni-Timing");
    if (t) console.info(`agni timing ${req.method.name}: ${t}`);
    return res;
  };
}

// newTransport builds a Connect transport rooted at baseUrl. It defaults to "/" so the
// app talks to the same origin that served it (the `agni serve` dev server).
export function newTransport(baseUrl = "/") {
  const mode = timingMode();
  const interceptors = mode ? [timingInterceptor(mode)] : [];
  if (engineFetch) return createConnectTransport({ baseUrl: new URL(baseUrl, location.href).href, fetch: engineFetch, interceptors });
  return createConnectTransport({ baseUrl, interceptors });
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

// reviewClient returns a typed client for ReviewService (review runs as resources, plus
// GetReviewManifest to resolve a stored checklist into the value a create takes; WS9-052).
export function reviewClient(baseUrl?: string): Client<typeof ReviewService> {
  return createClient(ReviewService, newTransport(baseUrl));
}
