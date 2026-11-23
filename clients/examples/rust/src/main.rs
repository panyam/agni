//! The Rust example for `make exercise-clients` (agni issue 1631). It does what clients/examples/go
//! does, through the Rust client in clients/rust. It checks the server, ingests a design, asks it
//! three questions, and prints one line of JSON summing up the answers.
//! docsite/content/build/client-contract.md names the line's fields.
//!
//! ```sh
//! clients/examples/rust/run -server http://127.0.0.1:8080 -design mount://probe/boards/jetson-cut
//! ```

use agni_client::Client;
use agni_client::agni::v1::geom::BoardGeometry;
use agni_client::agni::v1::webapi::{
    CheckDesignRequest, DeleteSnapshotRequest, GetBoardGeometryRequest, GetSummaryRequest,
    IngestRequest, RunQueryRequest, Snapshot,
};
use prost::Message;

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
    board_layers: usize,
    board_parts: usize,
    board_tracks: usize,
}

/// The query every example asks, so their answers compare. The check runs every rule.
const RESISTORS_OF: &str = r#"component.class(?c, "resistor") => ?c"#;

/// Small, so the cut's 51 KB board comes in several pages and every example pages.
const BOARD_PAGE_BYTES: i64 = 16 << 10;

/// Reads every page of the snapshot's board and merges them, as
/// docsite/content/build/board-geometry.md says a client does. prost merges from bytes, and a
/// merge appends repeated fields, so each page is merged in its encoded form.
async fn board_geometry(client: &Client, snapshot: &str) -> Result<BoardGeometry, String> {
    let mut board = BoardGeometry::default();
    let mut page_token = String::new();
    loop {
        let page = client
            .design_views()
            .get_board_geometry(GetBoardGeometryRequest {
                snapshot: snapshot.to_string(),
                max_bytes: BOARD_PAGE_BYTES,
                page_token,
            })
            .await
            .map_err(|s| format!("calling GetBoardGeometry: {s}"))?
            .into_inner();
        if let Some(part) = page.board {
            board
                .merge(part.encode_to_vec().as_slice())
                .map_err(|e| format!("merging a board page: {e}"))?;
        }
        if page.next_page_token.is_empty() {
            return Ok(board);
        }
        page_token = page.next_page_token;
    }
}

#[tokio::main]
async fn main() {
    let (mut server, mut design) = (
        "http://127.0.0.1:8080".to_string(),
        "mount://probe/boards/jetson-cut".to_string(),
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
    // A shared server asks every call for a token (agni issue 1627), passed in AGNI_TOKEN, and its
    // certificate may come from a team's own CA (agni issue 1765), passed in AGNI_CA.
    let token = std::env::var("AGNI_TOKEN").ok().filter(|t| !t.is_empty());
    let ca = match std::env::var("AGNI_CA").ok().filter(|c| !c.is_empty()) {
        Some(path) => Some(std::fs::read(&path).map_err(|e| format!("AGNI_CA {path}: {e}"))?),
        None => None,
    };
    let client = Client::connect_with_ca(server, token.as_deref(), ca.as_deref())
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
        .design_views()
        .get_summary(GetSummaryRequest {
            snapshot: snapshot.to_string(),
            ..Default::default()
        })
        .await
        .map_err(|s| format!("calling GetSummary: {s}"))?
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
    let board = board_geometry(client, snapshot).await?;
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
        board_layers: board.layers.len(),
        board_parts: board.placements.len(),
        board_tracks: board.nets.iter().map(|n| n.segments.len()).sum(),
    })
}
