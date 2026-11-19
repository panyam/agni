# agni (Java client)

A Java client for a shared `agni serve`, over gRPC (agni issue 1632). It holds the messages and
gRPC stubs generated from `protos/agni/v1`, and a thin layer that connects, checks the server and
reads error details. It needs Java 21. Nothing is published to Maven Central yet. Until then, build
it from a checkout.

```sh
cd clients/java
./gradlew build          # compiles, and runs the wire-fixture replayer against a real agni serve
./gradlew jar            # build/libs/agni-client-0.0.0-SNAPSHOT.jar
```

The group is `io.github.panyam.agni`. The generated classes live under `io.github.panyam.agni.v1`,
so `agni.v1.webapi.ServerInfo` is `io.github.panyam.agni.v1.webapi.ServerInfo`. The client's own
classes live in `io.github.panyam.agni.client`.

```java
import io.github.panyam.agni.client.AgniClient;
import io.github.panyam.agni.client.AgniErrors;
import io.github.panyam.agni.v1.webapi.*;

try (AgniClient client = AgniClient.create("http://127.0.0.1:8080", token)) {
  ServerInfo info = client.connect();
  Snapshot snap = client.snapshots().ingest(IngestRequest.newBuilder()
      .setSnapshot(Snapshot.newBuilder().setUri("mount://probe/designs/jetson-cut"))
      .build());
  try {
    RunQueryResponse rows = client.queries().runQuery(RunQueryRequest.newBuilder()
        .setSnapshot(snap.getName())
        .setQuery("component.class(?c, \"resistor\") => ?c")
        .build());
  } catch (io.grpc.StatusRuntimeException e) {
    AgniErrors.textPosition(e).ifPresent(p -> System.err.println(p.getCaret()));
    throw e;
  } finally {
    client.snapshots().deleteSnapshot(
        DeleteSnapshotRequest.newBuilder().setName(snap.getName()).build());
  }
}
```

- `AgniClient.create(address, token)` opens one channel. An `http://host:port` or bare
  `host:port` address uses cleartext HTTP/2, and `https://host:port` uses TLS. A non-empty token
  goes out as `authorization: Bearer <token>` metadata on every call. A server started with `--auth
  tokens` refuses a call without a good one (agni issue 1627). `https://` trusts the system's CAs,
  and `AgniClient.create(address, token, caFile)` trusts the PEM certificates in `caFile` instead,
  such as a team's own CA (agni issue 1765).
- `connect()` calls `ServerService.GetServerInfo`. It throws `IncompatibleServerException` when the
  server's `api` is not `agni.v1`. Test `ServerInfo.getFeaturesList()` for a feature by name, such
  as `snapshot_recovery`, rather than comparing versions.
- `server()`, `snapshots()`, `designViews()`, `checks()` and `queries()` return blocking stubs. Any other
  service takes `channel()`, as in `DiffServiceGrpc.newBlockingStub(client.channel())`, and so does
  an async or future stub.
- `AgniErrors` reads a failed call. `code(e)` names the code as Connect and the wire fixtures do,
  such as `not_found`. `details(e)` lists the packed details. `textPosition(e)`,
  `snapshotUnavailable(e)` and `blobsMissing(e)` unpack the three details the server sends.

The client keeps no snapshot state. A server with `snapshot_recovery` ingests an evicted snapshot
again when a question names it.

## Generated code

`src/generated/java` holds the generated code, and `buf.gen.java.yaml` pins the two buf remote
plugins that write it. `make proto-java` regenerates it, and `make proto-check` fails when the
committed tree differs from a fresh one. Never edit it by hand. protobuf-java refuses generated code
newer than itself, so `build.gradle.kts` pins protobuf-java and grpc-java to the plugins' versions.
Move them together.

## Tests

`WireFixturesTest` builds agni with `go build ./cmd/agni` from the repo root, or uses `AGNI_BIN` when
it is set. It starts `agni serve --mount probe=examples/cross-probe-project` on a free port, from an
empty directory. It replays `clients/wirefixtures/fixtures.json` in order through this client and
compares each answer with the recording, leaving out each fixture's `ignore` fields. A failed call
compares its code and details, never its message.

`clients/examples/java` calls a server through this client for `make exercise-clients`, and
`docsite/content/build/client-contract.md` says what every language's client does.
