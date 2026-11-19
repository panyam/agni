# agni (C++ client)

A C++ client for a shared `agni serve`, over gRPC (agni issue 1638). It generates every message and
stub from `protos/agni/v1` when it builds. A thin layer above them does what
[Writing an agni client](../../docsite/content/build/client-contract.md) asks of every client.

```cpp
#include "agni/client.h"
#include "agni/v1/webapi/design_view.grpc.pb.h"
#include "agni/v1/webapi/snapshot.grpc.pb.h"

namespace webapi = agni::v1::webapi;

agni::Client client("http://127.0.0.1:8080", token);  // token may be empty
grpc::Status st = client.Connect();                    // refuses a server that is not agni.v1

webapi::IngestRequest ingest;
ingest.mutable_snapshot()->set_uri("mount://probe/designs/jetson-cut");
webapi::Snapshot snap;
grpc::ClientContext ctx;
st = client.Stub<webapi::SnapshotService>()->Ingest(&ctx, ingest, &snap);
```

`clients/examples/cpp/main.cc` asks a design three questions and deletes its snapshot.

## What the layer adds

- `agni::Client` dials a server URL such as `http://127.0.0.1:8080`, or a bare `host:port`. A
  token goes on every call as `authorization: Bearer <token>`, and a server started with
  `--auth tokens` refuses a call without a good one (agni issue 1627). An `https://` URL is refused
  until this client can trust a CA (agni issue 1765).
- `Connect` calls `ServerService.GetServerInfo`, refuses a server whose `api` is not `agni.v1`, and
  keeps the answer. `HasFeature("snapshot_store")` tests the answer for a feature by name.
- `Stub<Service>()` returns any agni service's stub on the client's channel. Each call takes its own
  `grpc::ClientContext`, as gRPC's C++ library always does.
- A failed call returns a `grpc::Status`. `CodeName` names its code as Connect does, such as
  `not_found`. `TextPositionOf`, `SnapshotUnavailableOf` and `BlobsMissingOf` read the details the
  server attached, and `ErrorDetails` returns all of them.

The client keeps no snapshot state. A server with `snapshot_recovery` ingests an evicted snapshot
again by itself.

## Adding it to a CMake project

The library needs protobuf and gRPC's C++ library, with `protoc` and `grpc_cpp_plugin`, and CMake
3.21 or later. Homebrew's `grpc` formula brings them all. On Ubuntu, install `libgrpc++-dev`,
`libprotobuf-dev`, `protobuf-compiler` and `protobuf-compiler-grpc`.

From a checkout of agni, add the directory and link the target:

```cmake
add_subdirectory(path/to/agni/clients/cpp agni_client)
target_link_libraries(my_tool PRIVATE agni::client)
```

Without a checkout, fetch the repo at the release your server runs. v0.7.0 has no
`clients/cpp`, so use `main` until a later release ships it.

```cmake
include(FetchContent)
FetchContent_Declare(agni
  GIT_REPOSITORY https://github.com/panyam/agni.git
  GIT_TAG main  # a release tag, once one carries clients/cpp
  SOURCE_SUBDIR clients/cpp)
FetchContent_MakeAvailable(agni)
target_link_libraries(my_tool PRIVATE agni::client)
```

`AGNI_PROTO_DIR` points the build at another `protos/` directory.
[Wire compatibility](../../docsite/content/build/compatibility.md) says what a client built from one
release's protos can rely on from later servers.

## Why nothing generated is committed

Generated C++ compiles only against the protobuf runtime of the `protoc` that wrote it. Code from
protoc 36 does not build against Ubuntu's protobuf 3.21, and the reverse fails too. So the build runs
`protoc` and `grpc_cpp_plugin` over the protos with the toolchain it links against.
`make proto-check` therefore covers no C++. The wire-fixture replayer holds the client to the server
instead.

## Tests

```sh
cmake -S clients/cpp -B clients/cpp/build
cmake --build clients/cpp/build
ctest --test-dir clients/cpp/build --output-on-failure
```

- `client` checks the token, the refusal of another package, the URL rules and detail decoding,
  against a stand-in server in the test process.
- `wire_fixtures` builds `agni` with `go build`, starts `agni serve --mount probe=examples/cross-probe-project`
  on a free port, and replays `clients/wirefixtures/fixtures.json` through the client. Each answer must
  match the recording, apart from each fixture's `ignore` fields. An error must match in code and
  details. Pass `-DAGNI_BINARY=<path>` to serve a binary you already built. The test starts a
  process with `fork`, so it runs on Linux and macOS only.
