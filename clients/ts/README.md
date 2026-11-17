# @panyam/agni-client

A TypeScript client for a shared `agni serve`, for Node and browsers (agni issue 1630). It speaks
Connect over the protobuf-es code generated from `protos/agni/v1`. The viewer's generator writes
that code. The package is private for now and not on npm. A later ticket publishes it.

```ts
import { createAgniClient, snapshotUnavailable, textPosition } from "@panyam/agni-client/node";
// In a browser, import from "@panyam/agni-client/web" instead.

const agni = createAgniClient("http://127.0.0.1:8080", { token: process.env.AGNI_TOKEN });
const info = await agni.checkServer(); // refuses a server that does not speak agni.v1

const snap = await agni.snapshot.ingest({ snapshot: { uri: "mount://probe/designs/jetson-cut" } });
const design = await agni.designView.getSummary({ snapshot: snap.name });
const rows = await agni.query.runQuery({ snapshot: snap.name, query: 'component.class(?c, "resistor") => ?c' });
await agni.snapshot.deleteSnapshot({ name: snap.name });
```

`AgniClient` holds one Connect client per service. Each field takes the service's name without
`Service`, so `DesignViewService.GetSummary` is `agni.designView.getSummary`. A method takes its request as a
plain object and returns the response message. The messages, enums and services are under
`@panyam/agni-client/gen/agni/v1/...`.

## Connecting

`createAgniClient(baseUrl, options)` builds the client. `connect(baseUrl, options, needs)` builds it
and calls `checkServer(needs)` before it returns.

- `token` goes on every call as `Authorization: Bearer <token>`. The server does not check it yet.
  Agni issue 1627 makes a shared server refuse a call without one.
- In Node, `protocol` picks `"connect"` (the default), `"grpc"` or `"grpc-web"`, and `httpVersion`
  picks `"1.1"` (the default) or `"2"`. `json: true` sends Connect's JSON form.
- In a browser, `protocol` picks `"connect"` or `"grpc-web"`, and `binary: true` sends binary
  protobuf. `fetch` replaces the global one.
- `interceptors` adds Connect interceptors after the token's.

`checkServer(needs)` calls `ServerService.GetServerInfo` and returns its answer. It throws
`IncompatibleServerError` when the server speaks a package other than `agni.v1`, or lacks a feature
named in `needs`, such as `"grpc"`.

The client keeps nothing between calls. A server with `snapshot_recovery` ingests an evicted snapshot
again when a question names it, so the client never ingests one again itself.

## Errors

A failed call throws a `ConnectError`. Its `code` is the gRPC code. Each of three readers returns one
type of detail from the error, or `undefined` when the error has none of that type.

- `textPosition(err)` says where a query or a rule stopped parsing, with `invalid_argument`.
- `snapshotUnavailable(err)` says which snapshot is missing (`not_found`) or stale
  (`failed_precondition`), and which tiers changed.
- `blobsMissing(err)` names the file contents to upload before an ingest of uploaded files.

```ts
try {
  await agni.query.runQuery({ snapshot: snap.name, query: "component.class(?c" });
} catch (err) {
  const at = textPosition(err);
  if (at) console.error(`line ${at.line}, column ${at.column}\n${at.caret}`);
}
```

## Working on it

- `pnpm install` at the repo root installs it with the rest of the pnpm workspace.
- `make proto-web` regenerates `src/gen` with `buf.gen.ts.yaml`, and `make proto-check` fails when it
  is stale. The viewer imports this code too, under `@panyam/agni-client/gen/...` (agni issue 1651),
  so a change here is a change to the viewer, and `make web-test` covers it.
- `make ts-client-test` typechecks the client and its example, compiles the client to `dist/`, and
  runs the tests. `test/wirefixtures.test.ts` starts `agni serve` on `examples/cross-probe-project`,
  replays `clients/wirefixtures/fixtures.json` over Connect and over gRPC, and compares each answer
  with the recording. Under `pnpm test` alone it builds agni with `go build`, unless `AGNI_BIN`
  names a binary.
- `make exercise-clients` runs the example in `clients/examples/ts`.

[Writing an agni client](../../docsite/content/build/client-contract.md) says what every client does,
and [Wire compatibility](../../docsite/content/build/compatibility.md) says what it can rely on across
releases.
