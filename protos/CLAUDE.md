# protos

This file covers the proto toolchain and how to regenerate code after a proto change.

- The toolchain is Go 1.26.9 and `buf` 1.61.
- **Both protoc plugins are pinned as `tool` directives in `go.mod` and invoked via `go tool`.** Their versions are data, so only `buf` itself has to be on your PATH.
- **After ANY proto change run `make proto` (Go), `make proto-web` (TS) AND `make proto-py` (Python).** `make proto-check` fails the gate if any of them is stale.
- `make proto` and `make proto-check` set `GOTOOLCHAIN` to the Go version in `go.mod` for their Go generation. `protoc-gen-go` formats comments with the Go running it, so a newer Go rewrites comments in untouched generated files (agni 994). A `buf generate` typed by hand gets no such pin.
- The Python half uses buf's REMOTE plugins pinned by version, so it needs network. `pyproject.toml`'s `protobuf>=` floor moves with the plugin version.
- **The protos are TWO buf modules in one workspace**, declared in the root `buf.yaml`. One is the engine's `protos/`. The other is the datasheet producer's `datasheet/protos/` (`doc`, `derive`, `candidate`, `dsapi`).
- The datasheet module generates into `datasheet/gen/go` and imports `param.proto` and `workspace.proto` through the workspace. `make proto` runs both modules.
- The datasheet half is generated from the repo ROOT. Its `go tool` plugins resolve against the go.mod of the directory buf runs in, and only the root go.mod pins them.
- The TS step reads the whole workspace, and the workbench still builds from `web/`. The Python client reads `protos/` alone, since it is the engine's client.
