// The Connect client for the datasheet service's API (agni.v1.dsapi). The workbench talks only to
// agnids, which serves this page, so the client is rooted at the origin that served it.
import { createClient, type Client } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { DatasheetService } from "./gen/agni/v1/dsapi/datasheet_pb.js";

// datasheetClient returns a typed client for DatasheetService: a datasheet's doc-IR, its shared
// PartSpec draft, the per-author annotations, and the folder tree.
export function datasheetClient(baseUrl = "/"): Client<typeof DatasheetService> {
  return createClient(DatasheetService, createConnectTransport({ baseUrl }));
}
