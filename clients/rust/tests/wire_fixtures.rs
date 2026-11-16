//! Replays clients/wirefixtures/fixtures.json through the Rust client against a real `agni serve`
//! (agni issue 1631). The Go gate replays the same file over Connect and gRPC (cmd/agni's
//! TestWireFixtures), so the file always matches the server, and this test proves the Rust client
//! sends and reads that wire.
//!
//! It builds agni with `go build` from the repo root, or runs the binary `AGNI_BIN` names. It serves
//! examples/cross-probe-project as the mount `probe`, from an empty temp directory so the server
//! serves the API alone.

use std::fmt::Debug;
use std::net::TcpListener;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use agni_client::agni::v1::webapi::check_service_client::CheckServiceClient;
use agni_client::agni::v1::webapi::design_service_client::DesignServiceClient;
use agni_client::agni::v1::webapi::query_service_client::QueryServiceClient;
use agni_client::agni::v1::webapi::server_service_client::ServerServiceClient;
use agni_client::agni::v1::webapi::snapshot_service_client::SnapshotServiceClient;
use agni_client::agni::v1::webapi::{
    BlobsMissing, CheckDesignRequest, CheckDesignResponse, DeleteSnapshotRequest, GetDesignRequest,
    GetDesignResponse, GetServerInfoRequest, IngestRequest, RunQueryRequest, RunQueryResponse,
    ServerInfo, Snapshot, SnapshotUnavailable, TextPosition,
};
use agni_client::{Client, Detail};
use serde::Serialize;
use serde::de::DeserializeOwned;
use serde_json::Value;

fn repo_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .expect("the repo root")
}

/// A temp directory, removed on drop.
struct TempDir(PathBuf);

impl TempDir {
    fn new(prefix: &str) -> TempDir {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let dir = std::env::temp_dir().join(format!("{prefix}-{}-{nanos}", std::process::id()));
        std::fs::create_dir_all(&dir).expect("a temp directory");
        TempDir(dir)
    }
}

impl Drop for TempDir {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

/// The `agni serve` this test started. Dropping it kills the server.
struct Server {
    child: Child,
    url: String,
    log: PathBuf,
}

impl Drop for Server {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

fn agni_binary(bin_dir: &Path) -> PathBuf {
    if let Some(bin) = std::env::var_os("AGNI_BIN") {
        return PathBuf::from(bin);
    }
    let out = bin_dir.join("agni");
    let status = Command::new("go")
        .args(["build", "-o"])
        .arg(&out)
        .arg("./cmd/agni")
        .current_dir(repo_root())
        .status()
        .expect("go on PATH, to build agni (or set AGNI_BIN)");
    assert!(status.success(), "go build ./cmd/agni failed");
    out
}

fn serve(agni: &Path, work: &Path) -> Server {
    let port = TcpListener::bind("127.0.0.1:0")
        .unwrap()
        .local_addr()
        .unwrap()
        .port();
    let addr = format!("127.0.0.1:{port}");
    let mount = format!(
        "probe={}",
        repo_root().join("examples/cross-probe-project").display()
    );
    let log = work.join("serve.log");
    let out = std::fs::File::create(&log).unwrap();
    let child = Command::new(agni)
        .args(["serve", "--addr", &addr, "--mount", &mount])
        .current_dir(work)
        .stdin(Stdio::null())
        .stdout(out.try_clone().unwrap())
        .stderr(out)
        .spawn()
        .expect("agni serve starts");
    Server {
        child,
        url: format!("http://{addr}"),
        log,
    }
}

/// Connects once the server answers, through the client's own connect step. The token goes out as
/// `authorization` metadata, which this server accepts and does not check yet (agni issue 1627).
async fn connect(server: &Server) -> Client {
    let deadline = Instant::now() + Duration::from_secs(60);
    loop {
        match Client::connect(server.url.clone(), Some("wire-fixtures")).await {
            Ok(c) => return c,
            Err(e) if Instant::now() > deadline => panic!(
                "agni serve did not answer at {}: {e}\n{}",
                server.url,
                std::fs::read_to_string(&server.log).unwrap_or_default()
            ),
            Err(_) => tokio::time::sleep(Duration::from_millis(200)).await,
        }
    }
}

#[derive(serde::Deserialize)]
struct Fixtures {
    fixtures: Vec<Fixture>,
}

#[derive(serde::Deserialize)]
struct Fixture {
    name: String,
    service: String,
    method: String,
    request: Value,
    response: Option<Value>,
    error: Option<FixtureError>,
    #[serde(default)]
    ignore: Vec<String>,
}

#[derive(serde::Deserialize)]
struct FixtureError {
    code: String,
    #[serde(default)]
    details: Vec<FixtureDetail>,
}

#[derive(serde::Deserialize)]
struct FixtureDetail {
    #[serde(rename = "type")]
    type_name: String,
    value: Value,
}

/// What a replay compared, so the test can say it compared something of each kind.
#[derive(Default)]
struct Tally {
    answers: usize,
    errors: usize,
    details: usize,
}

/// Replays one fixture's call through the client's service clients, looked up by the fixture's
/// service and method. A fixture this table lacks fails the test, since a call it cannot replay is a
/// call it cannot vouch for.
macro_rules! replay {
    ($f:expr, $client:expr, $tally:expr, $( $svc:literal $method:literal => $stub:ident :: $call:ident ($req:ty) -> $resp:ty ),* $(,)?) => {
        match ($f.service.as_str(), $f.method.as_str()) {
            $( ($svc, $method) => match serde_json::from_value::<$req>($f.request.clone()) {
                Ok(req) => {
                    let got = $stub::new($client.transport()).$call(req).await;
                    judge::<$resp>(got.map(|r| r.into_inner()), $f, $tally)
                }
                Err(e) => Err(format!("its request does not read as {}: {e}", stringify!($req))),
            }, )*
            (s, m) => Err(format!("this test has no replay for {s}/{m}; add a row to its table")),
        }
    };
}

#[tokio::test(flavor = "multi_thread")]
async fn wire_fixtures() {
    let raw = std::fs::read_to_string(repo_root().join("clients/wirefixtures/fixtures.json"))
        .expect("clients/wirefixtures/fixtures.json (make wire-fixtures records it)");
    let file: Fixtures = serde_json::from_str(&raw).expect("the fixtures parse");
    assert!(!file.fixtures.is_empty(), "fixtures.json holds no fixtures");

    let bin = TempDir::new("agni-rust-bin");
    let agni = agni_binary(&bin.0);
    let work = TempDir::new("agni-rust-wire");
    let server = serve(&agni, &work.0);
    let client = connect(&server).await;

    let mut tally = Tally::default();
    let mut failures = Vec::new();
    for f in &file.fixtures {
        let res = replay!(f, client, &mut tally,
            "agni.v1.webapi.ServerService" "GetServerInfo" => ServerServiceClient::get_server_info(GetServerInfoRequest) -> ServerInfo,
            "agni.v1.webapi.SnapshotService" "Ingest" => SnapshotServiceClient::ingest(IngestRequest) -> Snapshot,
            "agni.v1.webapi.SnapshotService" "DeleteSnapshot" => SnapshotServiceClient::delete_snapshot(DeleteSnapshotRequest) -> pbjson_types::Empty,
            "agni.v1.webapi.DesignService" "GetDesign" => DesignServiceClient::get_design(GetDesignRequest) -> GetDesignResponse,
            "agni.v1.webapi.CheckService" "CheckDesign" => CheckServiceClient::check_design(CheckDesignRequest) -> CheckDesignResponse,
            "agni.v1.webapi.QueryService" "RunQuery" => QueryServiceClient::run_query(RunQueryRequest) -> RunQueryResponse,
        );
        if let Err(e) = res {
            failures.push(format!("{}: {e}", f.name));
        }
    }
    drop(client);
    drop(server);

    assert!(
        failures.is_empty(),
        "{} of {} fixtures differ:\n{}",
        failures.len(),
        file.fixtures.len(),
        failures.join("\n")
    );
    // A count guard, so an empty or mis-read file cannot pass by comparing nothing.
    assert_eq!(
        tally.answers + tally.errors,
        file.fixtures.len(),
        "every fixture was compared"
    );
    assert!(
        tally.answers > 0 && tally.errors > 0 && tally.details > 0,
        "compared answers, errors and details"
    );
}

/// Compares a call's outcome with its fixture. An answer compares as a message, leaving out the
/// fixture's `ignore` fields. An error compares its code and its details, and not its message,
/// which is prose.
fn judge<M>(got: Result<M, tonic::Status>, f: &Fixture, tally: &mut Tally) -> Result<(), String>
where
    M: Serialize + DeserializeOwned + PartialEq + Debug,
{
    match (got, &f.response, &f.error) {
        (Ok(msg), Some(want), None) => {
            tally.answers += 1;
            same_message(&msg, want, &f.ignore)
        }
        (Err(status), None, Some(want)) => {
            tally.errors += 1;
            let code = agni_client::code_name(status.code());
            if code != want.code {
                return Err(format!("failed with {code}, want {}", want.code));
            }
            let details = agni_client::details(&status)
                .map_err(|e| format!("its details do not decode: {e}"))?;
            if details.len() != want.details.len() {
                return Err(format!(
                    "carries {} details, want {}",
                    details.len(),
                    want.details.len()
                ));
            }
            for (i, (d, w)) in details.iter().zip(&want.details).enumerate() {
                tally.details += 1;
                if d.type_name() != w.type_name {
                    return Err(format!(
                        "detail {i} is {}, want {}",
                        d.type_name(),
                        w.type_name
                    ));
                }
                let same = match d {
                    Detail::TextPosition(m) => same_message::<TextPosition>(m, &w.value, &[]),
                    Detail::SnapshotUnavailable(m) => {
                        same_message::<SnapshotUnavailable>(m, &w.value, &[])
                    }
                    Detail::BlobsMissing(m) => same_message::<BlobsMissing>(m, &w.value, &[]),
                    Detail::Other(_) => Err("is a type this client does not decode".to_string()),
                };
                same.map_err(|e| format!("detail {i} {}: {e}", w.type_name))?;
            }
            Ok(())
        }
        (Ok(_), None, Some(want)) => Err(format!("answered, want the error {}", want.code)),
        (Err(status), Some(_), None) => Err(format!(
            "failed with {}: {}, want an answer",
            agni_client::code_name(status.code()),
            status.message()
        )),
        _ => Err("the fixture holds neither one response nor one error".to_string()),
    }
}

/// Compares a message with a fixture's protojson as messages of one type. Both go through the
/// message type, so field order, default values and the two JSON spellings of a name do not matter.
fn same_message<M>(got: &M, want: &Value, ignore: &[String]) -> Result<(), String>
where
    M: Serialize + DeserializeOwned + PartialEq + Debug,
{
    let mut got_json = serde_json::to_value(got).map_err(|e| e.to_string())?;
    let mut want_json = want.clone();
    for path in ignore {
        clear_path(&mut got_json, path);
        clear_path(&mut want_json, path);
    }
    let got_msg: M = serde_json::from_value(got_json.clone()).map_err(|e| e.to_string())?;
    let want_msg: M = serde_json::from_value(want_json.clone())
        .map_err(|e| format!("the fixture does not read: {e}"))?;
    if got_msg == want_msg {
        Ok(())
    } else {
        Err(format!("answered {got_json}, want {want_json}"))
    }
}

/// Clears a field named by its proto name, dotted through singular messages, in a message's JSON.
/// protojson writes the camelCase name and reads either, so both spellings go.
fn clear_path(v: &mut Value, path: &str) {
    let Some(obj) = v.as_object_mut() else { return };
    let (head, rest) = match path.split_once('.') {
        Some((h, r)) => (h, Some(r)),
        None => (path, None),
    };
    for key in [head.to_string(), camel(head)] {
        match rest {
            None => {
                obj.remove(&key);
            }
            Some(r) => {
                if let Some(inner) = obj.get_mut(&key) {
                    clear_path(inner, r);
                }
            }
        }
    }
}

fn camel(name: &str) -> String {
    let mut out = String::new();
    let mut upper = false;
    for c in name.chars() {
        if c == '_' {
            upper = true;
        } else if upper {
            out.extend(c.to_uppercase());
            upper = false;
        } else {
            out.push(c);
        }
    }
    out
}
