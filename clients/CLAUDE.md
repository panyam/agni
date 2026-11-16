# clients

This file covers the language clients and the checks that hold them to the server. The contract every client follows is `docsite/content/build/client-contract.md`.

- **Each language has a client under `clients/<language>` and an example under `clients/examples/<language>/run`.** `make exercise-clients` starts one `agni serve`, runs every `run`, and compares each one's summary line with the CLI's answers on the Jetson cut. A new language adds the two directories and nothing in the harness.
- **`clients/wirefixtures/fixtures.json` holds recorded calls, and every client replays them in its own tests.** `TestWireFixtures` replays them in the gate over Connect and gRPC, so the file always matches the server. `make wire-fixtures` records them again after an intended change, and the diff is the review.
- Generated code is pinned and checked per language. TypeScript, Rust, Java and Python commit theirs, and `make proto-check` fails when one is stale (`protos/CLAUDE.md`). C++ generates at build time, since its generated code compiles only against the protobuf it came from, so its CI replay holds it to the server.
- **Each client's CI job (`ts-client`, `client-rust`, `java-client`, `client-cpp`) runs beside `make testall`, not inside it.** A machine without Rust, a JDK or gRPC C++ still runs the gate.
- Homebrew's rustup keeps `cargo` off PATH. Put `/opt/homebrew/opt/rustup/bin` on PATH before the exercise's Rust row or `cargo test` in `clients/rust`.
- In a fresh worktree, run `pnpm install` before `make proto-check`, since its TypeScript step runs `node_modules/.bin/protoc-gen-es`.
- The fixtures spell error codes as Connect does. A gRPC client maps its code enum to those names, and gRPC's `CANCELLED` becomes `canceled`.
- **The viewer imports its generated code from `@panyam/agni-client`** (agni 1651), so a change under `clients/ts/src` is a viewer change too. The Docker image's web stage copies `clients/ts` for the same reason.
- A fixture for an rpc the Rust or C++ replayer has never called needs that rpc added to its table by hand (`clients/rust/tests/wire_fixtures.rs`, `clients/cpp/test/wirefixtures_test.cc`). Go, TypeScript and Java dispatch by name.
- gitleaks reads a recorded page token as an API key. Allowlist the exact value in `.gitleaks.toml`, never the pattern.
- gRPC sends call credentials only over TLS. The gRPC clients send their bearer token as plain `authorization` metadata, so it also reaches a cleartext server (agni issue 1627).
