// The client in Node (agni issue 1630), over connect-node's transports. It speaks Connect by default,
// and gRPC or gRPC-Web on request, since `agni serve` answers all three on one port.

import { createConnectTransport, createGrpcTransport, createGrpcWebTransport } from "@connectrpc/connect-node";
import type { Transport } from "@connectrpc/connect";
import { AgniClient, interceptorsFor, type ClientOptions } from "./client.js";

export interface NodeClientOptions extends ClientOptions {
  /** protocol is "connect" (the default), "grpc" or "grpc-web". */
  protocol?: "connect" | "grpc" | "grpc-web";
  /** httpVersion is "1.1" (the default) or "2" for Connect and gRPC-Web. gRPC always uses HTTP/2. */
  httpVersion?: "1.1" | "2";
  /** json sends Connect's JSON form rather than binary protobuf. */
  json?: boolean;
  /**
   * ca is PEM CA certificates to trust for an https:// server, in place of Node's own, such as a
   * team's own CA (agni issue 1765). `NODE_EXTRA_CA_CERTS` adds one to Node's instead.
   */
  ca?: string | Buffer;
}

/** createAgniClient returns a client for the server at baseUrl, such as "http://127.0.0.1:8080". */
export function createAgniClient(baseUrl: string, opts: NodeClientOptions = {}): AgniClient {
  return new AgniClient(nodeTransport(baseUrl, opts));
}

/**
 * connect returns a client for the server at baseUrl once checkServer has accepted it. It throws
 * IncompatibleServerError for a server that speaks another package or lacks a feature in needs.
 */
export async function connect(baseUrl: string, opts: NodeClientOptions = {}, needs: string[] = []): Promise<AgniClient> {
  const client = createAgniClient(baseUrl, opts);
  await client.checkServer(needs);
  return client;
}

function nodeTransport(baseUrl: string, opts: NodeClientOptions): Transport {
  const interceptors = interceptorsFor(opts);
  const httpVersion = opts.httpVersion ?? "1.1";
  const nodeOptions = opts.ca ? { ca: opts.ca } : undefined;
  switch (opts.protocol ?? "connect") {
    case "grpc":
      return createGrpcTransport({ baseUrl, interceptors, nodeOptions });
    case "grpc-web":
      return createGrpcWebTransport({ baseUrl, httpVersion, interceptors, nodeOptions });
    case "connect":
      return createConnectTransport({ baseUrl, httpVersion, interceptors, nodeOptions, useBinaryFormat: !opts.json });
  }
}

export * from "./index.js";
