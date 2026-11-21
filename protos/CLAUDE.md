# protos

This file covers the proto toolchain and how to regenerate code after a proto change.

- The toolchain is Go 1.27.4 and `buf` 1.61.
- **Both protoc plugins are pinned as `tool` directives in `go.mod` and invoked via `go tool`.** Their versions are data, so only `buf` itself has to be on your PATH.
- **After ANY proto change run `make proto` (Go), `make proto-web` (the TypeScript client, which the viewer imports, and the datasheets workbench), `make proto-py` (Python), `make proto-java` (Java) AND `make proto-rust` (Rust).** `make proto-check` fails the gate if any of them is stale. The C++ client generates at build time and has nothing to regenerate.
- `make proto` and `make proto-check` set `GOTOOLCHAIN` to the Go version in `go.mod` for their Go generation. `protoc-gen-go` formats comments with the Go running it, so a newer Go rewrites comments in untouched generated files (agni 994). A `buf generate` typed by hand gets no such pin.
- The Python half uses buf's REMOTE plugins pinned by version, so it needs network. `pyproject.toml`'s `protobuf>=` floor moves with the plugin version.
- The Rust half (`clients/rust/buf.gen.rust.yaml`) uses REMOTE plugins, and `clients/rust/Cargo.toml` pins prost, tonic and pbjson to the versions they write for.
- The Java half (`clients/java/buf.gen.java.yaml`) uses REMOTE plugins too. `clients/java/build.gradle.kts` pins protobuf-java and grpc-java to the plugins' versions, so move them together.
- **The protos are TWO buf modules in one workspace**, declared in the root `buf.yaml`. One is the engine's `protos/`. The other is the datasheet producer's `datasheet/protos/` (`doc`, `derive`, `candidate`, `dsapi`).
- The datasheet module generates into `datasheet/gen/go` and imports `param.proto` and `workspace.proto` through the workspace. `make proto` runs both modules.
- The datasheet half is generated from the repo ROOT. Its `go tool` plugins resolve against the go.mod of the directory buf runs in, and only the root go.mod pins them.
- The TS step reads the whole workspace, and the workbench still builds from `web/`. The Python client reads `protos/` alone, since it is the engine's client.
- **Removing a field before agni has public clients** skips the deprecation release. Scope a `breaking.ignore_only` in the root `buf.yaml` to the rule IDs and the one file, reserve the number and name, and take the ignore out at the next release tag. `proto-check` passes `--exclude-imports`, or the datasheet module re-checks the engine's imported protos where the ignore does not reach. `docsite/content/build/compatibility.md` has the rule (agni 1660).
