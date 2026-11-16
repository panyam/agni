---
title: "Wire compatibility"
description: "What a client built against one agni release can rely on in later ones, how the gate holds the protos to it, and how a client checks the server it reached."
---
A team builds a tool against agni v0.7.0's protos, in TypeScript, Rust, Java or C++, and points it
at a shared `agni serve`. A month later the server is upgraded. This page says what the tool can rely
on then, and what the repo does to keep that promise (agni issue 1629).

## What a release may change

The protos under `protos/agni/v1` are the contract. Every rpc's request and response is a message
there, and the CLI's `--format json` prints the same messages (C31).

- **A patch release**, such as v0.7.1, adds nothing a client must handle and removes nothing. A new
  optional field or a new rpc may appear, and a client built earlier ignores it.
- **A minor release**, such as v0.8.0, may add fields, rpcs and services freely. It removes or renames
  a field or rpc only after that one was marked deprecated in an earlier release. Its release notes
  name each removal under Breaking, with what replaces it.
- **The package**, `agni.v1`, does not change within v0. A change that cannot keep a field's number,
  type and name would go into a new package, `agni.v2`, served beside the old one.

Old and new clients meet on the wire, so the rules follow protobuf's. A field keeps its number, its
type and its name. Its name matters because a JSON client reads the name rather than the number.
A removed field's number is reserved, so it is never reused for another meaning.

## How the gate holds the protos to it

`make proto-check` runs `buf breaking` against the latest release tag, with the `FILE` rules in
`buf.yaml`. Suppose a pull request renames `Snapshot.uri`. The gate then fails and names both the
field and its JSON name, and the change has to keep the old field, mark it deprecated, and add the new
one beside it. CI fetches the tags for this, and a checkout without them fails the check and says to
fetch them.

Until agni has public clients, a removal may skip the deprecation release. `buf.yaml` then ignores
that delete rule for that one file, and the ignore comes out once a later release tag exists. The
check passes `--exclude-imports`, so each proto is checked once, in its own module, where its
module's ignores apply. agni 1660 removed `ir.Design.stackup` this way.

## Deprecating a field or an rpc

1. Add the replacement beside the old one, and have the server fill or answer both.
2. Mark the old one `[deprecated = true]` in the proto, and say in its comment what replaces it and
   from which release.
3. Name it in that release's notes.
4. Remove it in a later minor release, no sooner than one minor after the deprecation, with its field
   number reserved.

## How a client checks the server it reached

A client calls `ServerService.GetServerInfo` first. It answers the release the server was built from,
the package it speaks (`agni.v1`), and the optional features it has on:

| Feature | What it means |
| --- | --- |
| `grpc` | The server answers gRPC and gRPC-Web beside Connect, on the same port. |
| `snapshot_recovery` | A question on a snapshot the server evicted is answered after it ingests the snapshot again. |
| `snapshot_store` | The server keeps snapshot definitions across a restart (`--state-dir`), so recovery works after one too. |

A client tests for a feature by name rather than comparing versions. A feature can be off by
configuration, such as `snapshot_store` on a server with no `--state-dir`, without the client
mistaking the server's age. A client that needs a feature the server lacks says so plainly, rather
than failing at its first call.
