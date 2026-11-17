// AgniClient for every transport (agni issue 1630). It holds one Connect client per service on one
// transport, and checks the server before a caller asks it anything. node.ts and web.ts build the
// transport. docsite/content/build/client-contract.md says what each client in each language does.

import { createClient, type Client, type Interceptor, type Transport } from "@connectrpc/connect";
import { PartSpecService } from "./gen/agni/v1/param/service_pb.js";
import { ViewerPageService } from "./gen/agni/v1/viewer/page_pb.js";
import { CheckService } from "./gen/agni/v1/webapi/checks_pb.js";
import { DesignViewService } from "./gen/agni/v1/webapi/design_view_pb.js";
import { DiffService } from "./gen/agni/v1/webapi/diff_pb.js";
import { LexiconService } from "./gen/agni/v1/webapi/lexicon_pb.js";
import { ProjectService } from "./gen/agni/v1/webapi/project_pb.js";
import { QueryService } from "./gen/agni/v1/webapi/query_pb.js";
import { ReviewService } from "./gen/agni/v1/webapi/review_pb.js";
import { ServerService, type ServerInfo } from "./gen/agni/v1/webapi/server_pb.js";
import { SnapshotService } from "./gen/agni/v1/webapi/snapshot_pb.js";
import { TableService } from "./gen/agni/v1/webapi/tables_pb.js";
import { ViewerRoomService } from "./gen/agni/v1/webapi/viewerroom_pb.js";
import { WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";

/** API is the proto package this client was built for. A server that speaks another is refused. */
export const API = "agni.v1";

/** ClientOptions are what every transport takes. */
export interface ClientOptions {
  /**
   * token is sent on every call as `Authorization: Bearer <token>`. A shared server will refuse a
   * call without one (agni issue 1627).
   */
  token?: string;
  /** interceptors run on every call, after the one that sends the token. */
  interceptors?: Interceptor[];
}

/**
 * IncompatibleServerError says the server cannot serve this client: it speaks another package, or
 * it lacks a feature the caller named. info is what the server said about itself.
 */
export class IncompatibleServerError extends Error {
  constructor(
    message: string,
    readonly info: ServerInfo,
  ) {
    super(message);
    this.name = "IncompatibleServerError";
  }
}

/**
 * AgniClient calls one agni server. Each field is that service's Connect client, named after the
 * service without its `Service` suffix, so `DesignViewService.GetSummary` is `client.designView.getSummary`.
 * A method takes the request as a plain object and returns the response message.
 */
export class AgniClient {
  readonly server: Client<typeof ServerService>;
  readonly snapshot: Client<typeof SnapshotService>;
  readonly designView: Client<typeof DesignViewService>;
  readonly check: Client<typeof CheckService>;
  readonly query: Client<typeof QueryService>;
  readonly diff: Client<typeof DiffService>;
  readonly lexicon: Client<typeof LexiconService>;
  readonly project: Client<typeof ProjectService>;
  readonly review: Client<typeof ReviewService>;
  readonly table: Client<typeof TableService>;
  readonly workspace: Client<typeof WorkspaceService>;
  readonly partSpec: Client<typeof PartSpecService>;
  readonly viewerPage: Client<typeof ViewerPageService>;
  readonly viewerRoom: Client<typeof ViewerRoomService>;

  constructor(readonly transport: Transport) {
    this.server = createClient(ServerService, transport);
    this.snapshot = createClient(SnapshotService, transport);
    this.designView = createClient(DesignViewService, transport);
    this.check = createClient(CheckService, transport);
    this.query = createClient(QueryService, transport);
    this.diff = createClient(DiffService, transport);
    this.lexicon = createClient(LexiconService, transport);
    this.project = createClient(ProjectService, transport);
    this.review = createClient(ReviewService, transport);
    this.table = createClient(TableService, transport);
    this.workspace = createClient(WorkspaceService, transport);
    this.partSpec = createClient(PartSpecService, transport);
    this.viewerPage = createClient(ViewerPageService, transport);
    this.viewerRoom = createClient(ViewerRoomService, transport);
  }

  /**
   * checkServer calls ServerService.GetServerInfo and returns its answer. It throws
   * IncompatibleServerError when the server speaks a package other than agni.v1, or lacks a feature
   * named in needs, such as "grpc". A client tests features by name rather than comparing versions
   * (docsite/content/build/compatibility.md).
   */
  async checkServer(needs: string[] = []): Promise<ServerInfo> {
    const info = await this.server.getServerInfo({});
    if (info.api !== API) {
      throw new IncompatibleServerError(
        `the server speaks ${JSON.stringify(info.api)}, and this client was built for ${API}`,
        info,
      );
    }
    const missing = needs.filter((f) => !info.features.includes(f));
    if (missing.length > 0) {
      throw new IncompatibleServerError(
        `the server lacks the feature ${missing.join(", ")}; it has ${info.features.join(", ") || "none"}`,
        info,
      );
    }
    return info;
  }
}

/** bearer sends a token on every call as `Authorization: Bearer <token>`. */
export function bearer(token: string): Interceptor {
  return (next) => (req) => {
    req.header.set("Authorization", `Bearer ${token}`);
    return next(req);
  };
}

/** interceptorsFor puts the token's interceptor first, then the caller's own. */
export function interceptorsFor(opts: ClientOptions): Interceptor[] {
  return [...(opts.token ? [bearer(opts.token)] : []), ...(opts.interceptors ?? [])];
}
