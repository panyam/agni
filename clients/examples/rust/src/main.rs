//! The Rust example for `make exercise-clients` (agni issue 1631). It does what clients/examples/go
//! does, through the Rust client in clients/rust. It checks the server, ingests a design, asks it
//! three questions, and prints one line of JSON summing up the answers.
//! docsite/content/build/client-contract.md names the line's fields.
//!
//! ```sh
//! clients/examples/rust/run -server http://127.0.0.1:8080 -design mount://probe/designs/jetson-cut
//! ```

use agni_client::Client;
use agni_client::agni::v1::webapi::{
    CheckDesignRequest, DeleteSnapshotRequest, GetDesignRequest, IngestRequest, RunQueryRequest,
    Snapshot,
};

/// The line every language's example prints. Its JSON names are the contract.
#[derive(serde::Serialize)]
struct Summary {
    version: String,
    api: String,
    features: Vec<String>,
    components: i32,
    nets: i32,
    sheets: usize,
    findings: usize,
    query_rows: usize,
}

/// The query every example asks, so their answers compare. The check runs every rule.
const RESISTORS_OF: &str = r#"component.class(?c, "resistor") => ?c"#;

#[tokio::main]
async fn main() {
    let (mut server, mut design) = (
        "http://127.0.0.1:8080".to_string(),
        "mount://probe/designs/jetson-cut".to_string(),
    );
    let mut args = std::env::args().skip(1);
    while let Some(flag) = args.next() {
        let slot = match flag.trim_start_matches('-') {
            "server" => &mut server,
            "design" => &mut design,
            _ => fail(format!(
                "unknown flag {flag}; this takes -server <url> and -design <uri>"
            )),
        };
        *slot = args
            .next()
            .unwrap_or_else(|| fail(format!("{flag} needs a value")));
    }
    match run(&server, &design).await {
        Ok(s) => println!(
            "{}",
            serde_json::to_string(&s).expect("a struct of plain fields serializes")
        ),
        Err(e) => fail(e),
    }
}

fn fail(msg: impl std::fmt::Display) -> ! {
    eprintln!("{msg}");
    std::process::exit(1)
}

async fn run(server: &str, design: &str) -> Result<Summary, String> {
    // Connecting calls GetServerInfo and refuses a server that speaks another package.
    let client = Client::connect(server, None)
        .await
        .map_err(|e| e.to_string())?;
    let ingest = IngestRequest {
        snapshot: Some(Snapshot {
            uri: design.to_string(),
            ..Default::default()
        }),
        ..Default::default()
    };
    let name = client
        .snapshots()
        .ingest(ingest)
        .await
        .map_err(|s| format!("calling Ingest: {s}"))?
        .into_inner()
        .name;
    let summary = ask(&client, &name).await;
    // Delete the snapshot whether or not the questions succeeded, as the Go example's defer does.
    let _ = client
        .snapshots()
        .delete_snapshot(DeleteSnapshotRequest { name })
        .await;
    summary
}

async fn ask(client: &Client, snapshot: &str) -> Result<Summary, String> {
    let d = client
        .designs()
        .get_design(GetDesignRequest {
            snapshot: snapshot.to_string(),
            ..Default::default()
        })
        .await
        .map_err(|s| format!("calling GetDesign: {s}"))?
        .into_inner();
    let c = client
        .checks()
        .check_design(CheckDesignRequest {
            snapshot: snapshot.to_string(),
            ..Default::default()
        })
        .await
        .map_err(|s| format!("calling CheckDesign: {s}"))?
        .into_inner();
    let q = client
        .queries()
        .run_query(RunQueryRequest {
            snapshot: snapshot.to_string(),
            query: RESISTORS_OF.to_string(),
            ..Default::default()
        })
        .await
        .map_err(|s| format!("calling RunQuery: {s}"))?
        .into_inner();
    let info = client.info();
    let mut features = info.features.clone();
    features.sort();
    Ok(Summary {
        version: info.version.clone(),
        api: info.api.clone(),
        features,
        components: d.component_count,
        nets: d.net_count,
        sheets: d.sheets.len(),
        findings: c.findings.len(),
        query_rows: q.rows.len(),
    })
}
