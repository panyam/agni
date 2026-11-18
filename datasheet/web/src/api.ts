// The Connect client for the datasheet service's API (agni.v1.dsapi). The workbench talks only to
// agnids, which serves this page, so the client is rooted at the origin that served it.
import { createClient, type Client } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { DatasheetService } from "./gen/agni/v1/dsapi/datasheet_pb.js";
import { WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";

// datasheetClient returns a typed client for DatasheetService. The service serves a datasheet's
// doc-IR, its shared PartSpec draft, and the per-author annotations.
export function datasheetClient(baseUrl = "/"): Client<typeof DatasheetService> {
  return createClient(DatasheetService, createConnectTransport({ baseUrl }));
}

// workspaceClient returns a typed client for the engine's WorkspaceService, which agnids serves over
// its mounts (agni issue 1734). The folder tree lists through it, as the viewer's does.
export function workspaceClient(baseUrl = "/"): Client<typeof WorkspaceService> {
  return createClient(WorkspaceService, createConnectTransport({ baseUrl }));
}
