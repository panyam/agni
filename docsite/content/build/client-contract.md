---
title: "Writing an agni client"
description: "What a client in any language does against a shared agni serve: connect, check the server, ask about a snapshot, read errors, and prove it with the wire fixtures."
---
A team builds a tool in TypeScript, Rust, Java or C++ and points it at a shared `agni serve`. The
code generated from `protos/agni/v1` gives it every message and every rpc. This page says what the
thin layer above that code does, so every language's client behaves alike (agni issues 1625 and
1633). [Wire compatibility](../compatibility/) says what a client can rely on across releases.

## Connecting

`agni serve` answers three protocols on one port. Connect, over HTTP/1.1 or HTTP/2, in JSON or
binary protobuf. gRPC, over HTTP/2, cleartext or TLS. And gRPC-Web. A language with a mature gRPC
library, such as Rust with tonic, Java with grpc-java or C++ with gRPC's own, generates stubs and
uses gRPC. TypeScript uses Connect, as the viewer does. A language with neither can post protojson to
`/<service>/<method>` with `Content-Type: application/json`, which needs no library at all.

A shared server will ask who is calling. The client sends a bearer token in the `Authorization`
header, or the gRPC metadata of that name, and the server refuses a call without one (agni issue 1627).

## Checking the server first

A client calls `ServerService.GetServerInfo` before anything else. It refuses a server whose `api` is
not `agni.v1`, and says so plainly. It tests for each feature it needs by name, such as `grpc`, rather
than comparing versions.

## Asking about a design

A client asks its questions about a snapshot, a design the server ingested once and holds under a
name.

1. `SnapshotService.Ingest` with the design's `mount://` URI returns the snapshot. A client may name
   it itself with `snapshot_id`, and sending the same ingest again returns the same snapshot.
2. Every analysis rpc then names it: `GetDesign`, `CheckDesign`, `RunQuery` and the rest.
3. `DeleteSnapshot` when done.

The client keeps nothing else. A server with `snapshot_recovery` ingests a snapshot it evicted again
when a question names it, and with `snapshot_store` it does so after a restart too. A snapshot whose
files changed is refused as stale (`failed_precondition`), unless it was ingested with `refresh`. Then
the server reads it again, and the answer's `Agni-Snapshot-Refreshed` header names the tiers that
changed.

## Reading errors

A failed rpc carries a code and a list of details, the same over Connect and gRPC. A client turns
the code into its language's error and keeps the details, since they say what to do:

| Detail | Comes with | What it says |
| --- | --- | --- |
| `agni.v1.webapi.TextPosition` | `invalid_argument` from a query or a rule | Where in the text it stopped parsing: a byte `offset`, and a 1-based `line` and `column`. |
| `agni.v1.webapi.SnapshotUnavailable` | `not_found` or `failed_precondition` | Which snapshot, whether it is missing or stale, and the tiers that changed. |
| `agni.v1.webapi.BlobsMissing` | `failed_precondition` from an ingest of uploaded files | Which file contents to upload before ingesting again. |

## Proving it with the wire fixtures

`clients/wirefixtures/fixtures.json` holds calls recorded from a server on the Jetson cut: each one's
service, method, request, and its response or error. A client's tests start `agni serve --mount
probe=examples/cross-probe-project`, replay every call in order, and compare each answer with the
recording. They leave out the fields a fixture lists under `ignore`, and an error's message, which is
prose. The gate replays them over Connect's JSON form and over gRPC (`TestWireFixtures`), so the file
always matches the server. `make wire-fixtures` records them again after an intended change.

The Rust client in `clients/rust` replays them through tonic in `tests/wire_fixtures.rs`. It reads
each fixture's protojson into the generated message with pbjson and compares messages, and it decodes
an error's details from gRPC's `grpc-status-details-bin` trailer.

## The exercise

`make exercise-clients` runs each language's example from `clients/examples/<language>/run` against
one server. Each prints one line of JSON, and the exercise compares it with the CLI's answers on the
same design:

```
{"version":"v0.7.0","api":"agni.v1","features":["grpc","snapshot_recovery","snapshot_store"],"components":19,"nets":17,"sheets":4,"findings":19,"query_rows":4}
```

The fields are the server's version, package and features, then the design's component, net and
sheet counts from `GetDesign`, the findings from `CheckDesign` with every rule, and the rows of
`component.class(?c, "resistor") => ?c`. `clients/examples/go` is the reference.
