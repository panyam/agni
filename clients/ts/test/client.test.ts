// The client's own behaviour against an in-process router, with no server. It checks the bearer token
// on each call, the refusal of a server that speaks another package, and the readers for error details.

import { create } from "@bufbuild/protobuf";
import { Code, ConnectError, createRouterTransport } from "@connectrpc/connect";
import { describe, expect, test } from "vitest";
import { AgniClient, IncompatibleServerError, blobsMissing, interceptorsFor, snapshotUnavailable, textPosition } from "../src/index.js";
import { DesignViewService } from "../src/gen/agni/v1/webapi/design_view_pb.js";
import { TextPositionSchema } from "../src/gen/agni/v1/webapi/query_pb.js";
import { ServerService } from "../src/gen/agni/v1/webapi/server_pb.js";
import {
  BlobsMissingSchema,
  SnapshotUnavailableSchema,
  SnapshotUnavailable_Reason,
} from "../src/gen/agni/v1/webapi/snapshot_pb.js";

// serverSaying is a client whose server answers GetServerInfo with info, and records the
// Authorization header each call carried.
function serverSaying(info: { api: string; features?: string[] }, token?: string) {
  const seen: (string | null)[] = [];
  const transport = createRouterTransport(
    ({ service }) => {
      service(ServerService, {
        getServerInfo(_req, ctx) {
          seen.push(ctx.requestHeader.get("Authorization"));
          return { version: "v0.0.0", api: info.api, features: info.features ?? [] };
        },
      });
    },
    { transport: { interceptors: interceptorsFor({ token }) } },
  );
  return { client: new AgniClient(transport), seen };
}

describe("checkServer", () => {
  test("accepts agni.v1 and returns what the server said", async () => {
    const { client } = serverSaying({ api: "agni.v1", features: ["grpc"] });
    const info = await client.checkServer(["grpc"]);
    expect(info.api).toBe("agni.v1");
    expect(info.features).toEqual(["grpc"]);
  });

  test("refuses a server that speaks another package, and says which", async () => {
    const { client } = serverSaying({ api: "agni.v2" });
    const err = await client.checkServer().catch((e: unknown) => e);
    expect(err).toBeInstanceOf(IncompatibleServerError);
    expect((err as Error).message).toBe('the server speaks "agni.v2", and this client was built for agni.v1');
  });

  test("refuses a server that lacks a feature the caller needs", async () => {
    const { client } = serverSaying({ api: "agni.v1", features: ["snapshot_recovery"] });
    await expect(client.checkServer(["grpc"])).rejects.toThrow("the server lacks the feature grpc; it has snapshot_recovery");
  });
});

describe("the bearer token", () => {
  test("goes on every call as Authorization: Bearer", async () => {
    const { client, seen } = serverSaying({ api: "agni.v1" }, "s3cret");
    await client.checkServer();
    await client.checkServer();
    expect(seen).toEqual(["Bearer s3cret", "Bearer s3cret"]);
  });

  test("is left off when none is given", async () => {
    const { client, seen } = serverSaying({ api: "agni.v1" });
    await client.checkServer();
    expect(seen).toEqual([null]);
  });
});

describe("error details", () => {
  // failing answers GetSummary with err, so the detail crosses the router's wire encoding.
  async function failing(err: ConnectError): Promise<unknown> {
    const transport = createRouterTransport(({ service }) => {
      service(DesignViewService, {
        getSummary() {
          throw err;
        },
      });
    });
    return new AgniClient(transport).designView.getSummary({ snapshot: "snapshots/x" }).catch((e: unknown) => e);
  }

  test("each reader finds its own detail and no other", async () => {
    const pos = await failing(
      new ConnectError("parse", Code.InvalidArgument, undefined, [
        { desc: TextPositionSchema, value: create(TextPositionSchema, { offset: 4, line: 1, column: 5 }) },
      ]),
    );
    expect(textPosition(pos)?.column).toBe(5);
    expect(snapshotUnavailable(pos)).toBeUndefined();
    expect(blobsMissing(pos)).toBeUndefined();

    const gone = await failing(
      new ConnectError("gone", Code.NotFound, undefined, [
        {
          desc: SnapshotUnavailableSchema,
          value: create(SnapshotUnavailableSchema, { name: "snapshots/x", reason: SnapshotUnavailable_Reason.MISSING }),
        },
      ]),
    );
    expect(snapshotUnavailable(gone)?.reason).toBe(SnapshotUnavailable_Reason.MISSING);
    expect(textPosition(gone)).toBeUndefined();

    const blobs = await failing(
      new ConnectError("upload", Code.FailedPrecondition, undefined, [
        { desc: BlobsMissingSchema, value: create(BlobsMissingSchema, { sha256: ["sha256:ab"] }) },
      ]),
    );
    expect(blobsMissing(blobs)?.sha256).toEqual(["sha256:ab"]);
  });

  test("an error that is not a ConnectError has no details", () => {
    expect(textPosition(new Error("plain"))).toBeUndefined();
  });
});
