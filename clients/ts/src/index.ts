// @panyam/agni-client (agni issue 1630). This entry holds what every transport shares. Import
// "@panyam/agni-client/node" in Node and "@panyam/agni-client/web" in a browser for createAgniClient
// and connect. The messages and services are under "@panyam/agni-client/gen/agni/v1/...".

export { AgniClient, API, IncompatibleServerError, bearer, interceptorsFor, type ClientOptions } from "./client.js";
export { blobsMissing, snapshotUnavailable, textPosition } from "./errors.js";
export { Code, ConnectError } from "@connectrpc/connect";
