// Connect clients for the WS9 web API. The service contracts are generated from proto
// (CONSTRAINTS C2); this module only wires a browser transport onto them. The view layer
// calls these clients instead of hand-rolling fetch/JSON.
import { createClient, type Client } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";
import { DesignService } from "./gen/agni/v1/webapi/design_pb.js";
import { CheckService } from "./gen/agni/v1/webapi/checks_pb.js";
import { DiffService } from "./gen/agni/v1/webapi/diff_pb.js";
import { DatasheetService } from "./gen/agni/v1/webapi/datasheet_pb.js";
import { QueryService } from "./gen/agni/v1/webapi/query_pb.js";
import { ReviewService } from "./gen/agni/v1/webapi/review_pb.js";

// newTransport builds a Connect transport rooted at baseUrl. It defaults to "/" so the
// app talks to the same origin that served it (the `agni serve` dev server).
export function newTransport(baseUrl = "/") {
  return createConnectTransport({ baseUrl });
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
// severity report, and expectation sidecars — extracted from DesignService in WS9-026).
export function checksClient(baseUrl?: string): Client<typeof CheckService> {
  return createClient(CheckService, newTransport(baseUrl));
}

// diffClient returns a typed client for DiffService (the semantic diff between two designs
// plus the highlight maps the visual diff joins to geometry, WS9-005).
export function diffClient(baseUrl?: string): Client<typeof DiffService> {
  return createClient(DiffService, newTransport(baseUrl));
}

// datasheetClient returns a typed client for DatasheetService (a datasheet's doc-IR for the
// extraction workbench, WS13-006).
export function datasheetClient(baseUrl?: string): Client<typeof DatasheetService> {
  return createClient(DatasheetService, newTransport(baseUrl));
}

// queryClient returns a typed client for QueryService (ad-hoc datalog queries over a design's
// fact base — the web front-end to the same engine `agni query` runs, WS9-036 / WS3-029).
export function queryClient(baseUrl?: string): Client<typeof QueryService> {
  return createClient(QueryService, newTransport(baseUrl));
}

// reviewClient returns a typed client for ReviewService (WS9-052): review runs as resources, plus
// GetReviewManifest to resolve a stored checklist into the value a create takes.
export function reviewClient(baseUrl?: string): Client<typeof ReviewService> {
  return createClient(ReviewService, newTransport(baseUrl));
}
