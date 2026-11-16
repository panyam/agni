# agni-client for Rust

A Rust client for a shared `agni serve`, over gRPC (agni issue 1631, mission #1625). Messages come
from prost and service clients from tonic, generated from `protos/agni/v1` into `src/gen`. A thin
layer above them connects, checks the server, sends a bearer token and reads error details, as
[Writing an agni client](../../docsite/content/build/client-contract.md) asks of every client.

The crate is not on crates.io yet. Depend on it by path or by git:

```toml
[dependencies]
agni-client = { git = "https://github.com/panyam/agni" }
tokio = { version = "1", features = ["macros", "rt-multi-thread"] }
```

## Use

```rust
use agni_client::agni::v1::webapi::{GetDesignRequest, IngestRequest, RunQueryRequest, Snapshot};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Calls GetServerInfo, and refuses a server whose api is not agni.v1.
    let client = agni_client::Client::connect("http://127.0.0.1:8080", Some("my-token")).await?;
    if !client.has_feature("snapshot_recovery") {
        eprintln!("this server forgets snapshots it evicts");
    }

    let ingest = IngestRequest {
        snapshot: Some(Snapshot { uri: "mount://probe/designs/jetson-cut".into(), ..Default::default() }),
        ..Default::default()
    };
    let snap = client.snapshots().ingest(ingest).await?.into_inner();

    let design = client
        .designs()
        .get_design(GetDesignRequest { snapshot: snap.name.clone(), ..Default::default() })
        .await?
        .into_inner();
    println!("{} components, {} nets", design.component_count, design.net_count);

    let query = RunQueryRequest { snapshot: snap.name.clone(), query: "component.class(?c".into(), ..Default::default() };
    if let Err(status) = client.queries().run_query(query).await {
        if let Some(at) = agni_client::text_position(&status) {
            eprintln!("the query stops parsing at line {}, column {}\n{}", at.line, at.column, at.caret);
        }
    }
    Ok(())
}
```

`Client` has accessors for the services most tools call: `server()`, `snapshots()`, `designs()`,
`checks()` and `queries()`. Every other generated client takes `client.transport()`, as in
`DiffServiceClient::new(client.transport())`, and so carries the token too.

The server accepts the token and does not check it yet. Agni issue 1627 adds the check. For TLS,
build a `tonic::transport::Channel` with the settings and pass it to `Client::with_channel`.

## Errors

A failed call returns a `tonic::Status`. `agni_client::details` decodes the details it carries, and
`text_position`, `snapshot_unavailable` and `blobs_missing` each find one kind.
`agni_client::code_name` spells a code as Connect does, such as `invalid_argument`, so a log line
reads the same from every client.

The client keeps no snapshot state. A server with the `snapshot_recovery` feature ingests an evicted
snapshot again when a question names it.

## Protojson

Every generated message implements `serde::Serialize` and `serde::Deserialize` in protojson's form,
through pbjson. `serde_json::to_string(&design)` prints the message as protojson, the form
`clients/wirefixtures/fixtures.json` records, and `serde_json::from_str` reads it back.

## Generation

`make proto-rust` writes `src/gen` from `buf.gen.rust.yaml`, whose buf remote plugins are pinned by
version. The crates in `Cargo.toml` match those versions. `make proto-check` generates again into a
temp tree and fails when the committed code differs. Never edit `src/gen` by hand.

## Tests

```sh
cd clients/rust && cargo test --workspace
```

`tests/wire_fixtures.rs` builds agni with `go build`, or runs the binary `AGNI_BIN` names. It serves
`examples/cross-probe-project` as the mount `probe` and replays `clients/wirefixtures/fixtures.json`
through this client. The exercise example in `clients/examples/rust` belongs to the same Cargo
workspace, and `make exercise-clients` runs it.
