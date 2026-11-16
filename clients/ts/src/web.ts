// The client in a browser (agni issue 1630), over connect-web's fetch transports. A browser cannot
// speak gRPC itself, so this speaks Connect by default and gRPC-Web on request.

import { createConnectTransport, createGrpcWebTransport } from "@connectrpc/connect-web";
import type { Transport } from "@connectrpc/connect";
import { AgniClient, interceptorsFor, type ClientOptions } from "./client.js";

export interface WebClientOptions extends ClientOptions {
  /** protocol is "connect" (the default) or "grpc-web". */
  protocol?: "connect" | "grpc-web";
  /** binary sends Connect's binary form rather than JSON, which is connect-web's default. */
  binary?: boolean;
  /** fetch replaces the global fetch, for a test or a page that wraps its requests. */
  fetch?: typeof globalThis.fetch;
}

/** createAgniClient returns a client for the server at baseUrl, such as "https://agni.example". */
export function createAgniClient(baseUrl: string, opts: WebClientOptions = {}): AgniClient {
  return new AgniClient(webTransport(baseUrl, opts));
}

/**
 * connect returns a client for the server at baseUrl once checkServer has accepted it. It throws
 * IncompatibleServerError for a server that speaks another package or lacks a feature in needs.
 */
export async function connect(baseUrl: string, opts: WebClientOptions = {}, needs: string[] = []): Promise<AgniClient> {
  const client = createAgniClient(baseUrl, opts);
  await client.checkServer(needs);
  return client;
}

function webTransport(baseUrl: string, opts: WebClientOptions): Transport {
  const interceptors = interceptorsFor(opts);
  if (opts.protocol === "grpc-web") {
    return createGrpcWebTransport({ baseUrl, interceptors, fetch: opts.fetch });
  }
  return createConnectTransport({ baseUrl, interceptors, useBinaryFormat: opts.binary, fetch: opts.fetch });
}

export * from "./index.js";
