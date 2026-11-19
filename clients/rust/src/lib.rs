//! A Rust client for a shared `agni serve`, over gRPC (agni issue 1631).
//!
//! The messages and the per-service gRPC clients are generated from `protos/agni/v1` into `src/gen`
//! and live under [`agni`]. This crate adds the thin layer every client has.
//! `docsite/content/build/client-contract.md` describes that layer.
//!
//! - [`Client::connect`] takes the server's address and an optional bearer token. It calls
//!   `ServerService.GetServerInfo` first and refuses a server whose `api` is not [`API`].
//! - Every call carries the token as `authorization: Bearer <token>` metadata.
//! - [`details`] and its three helpers read the error details a failed call carries.
//!
//! ```no_run
//! # async fn demo() -> Result<(), agni_client::Error> {
//! use agni_client::agni::v1::webapi::{GetSummaryRequest, IngestRequest, Snapshot};
//!
//! let client = agni_client::Client::connect("http://127.0.0.1:8080", None).await?;
//! let ingest = IngestRequest {
//!     snapshot: Some(Snapshot { uri: "mount://probe/designs/jetson-cut".into(), ..Default::default() }),
//!     ..Default::default()
//! };
//! let snap = client.snapshots().ingest(ingest).await?.into_inner();
//! let design = client
//!     .design_views()
//!     .get_summary(GetSummaryRequest { snapshot: snap.name.clone(), ..Default::default() })
//!     .await?
//!     .into_inner();
//! println!("{} components", design.component_count);
//! # Ok(())
//! # }
//! ```
//!
//! The client keeps no snapshot state. A server with `snapshot_recovery` ingests an evicted snapshot
//! again by itself when a question names it.

use std::fmt;

use tonic::metadata::{Ascii, MetadataValue};
use tonic::service::Interceptor;
use tonic::service::interceptor::InterceptedService;
use tonic::transport::{Certificate, Channel, ClientTlsConfig, Endpoint};
use tonic::{Code, Request, Status};

use agni::v1::webapi::check_service_client::CheckServiceClient;
use agni::v1::webapi::connection_service_client::ConnectionServiceClient;
use agni::v1::webapi::design_service_client::DesignServiceClient;
use agni::v1::webapi::design_view_service_client::DesignViewServiceClient;
use agni::v1::webapi::pack_service_client::PackServiceClient;
use agni::v1::webapi::query_service_client::QueryServiceClient;
use agni::v1::webapi::server_service_client::ServerServiceClient;
use agni::v1::webapi::snapshot_service_client::SnapshotServiceClient;
use agni::v1::webapi::{
    BlobsMissing, GetServerInfoRequest, ServerInfo, SnapshotUnavailable, TextPosition,
};

// rustfmt leaves src/gen as buf wrote it, so `make proto-check` compares like with like.
#[path = "gen/mod.rs"]
#[allow(clippy::all, missing_docs)]
#[rustfmt::skip]
mod generated;

/// The messages and gRPC clients generated from `protos/agni/v1`, one module per proto package.
pub use generated::agni;

/// The package this client speaks. [`Client::connect`] refuses a server that answers another one.
pub const API: &str = "agni.v1";

/// The channel every service client of a [`Client`] calls through. It adds the bearer token.
pub type Transport = InterceptedService<Channel, Bearer>;

/// A connection to one `agni serve`, checked with `GetServerInfo`.
///
/// Cloning it is cheap, and the clones share one HTTP/2 connection. Each service accessor returns a
/// fresh generated client over that connection.
#[derive(Clone)]
pub struct Client {
    transport: Transport,
    info: ServerInfo,
}

impl Client {
    /// Connects to the server at `address`, such as `http://127.0.0.1:8080`, and checks it.
    ///
    /// With a token, every call carries `authorization: Bearer <token>`. A server started with
    /// `--auth tokens` refuses a call without a good one (agni issue 1627). An `https://` address
    /// speaks TLS and trusts the system's CAs.
    pub async fn connect(address: impl Into<String>, token: Option<&str>) -> Result<Client, Error> {
        Client::connect_with_ca(address, token, None).await
    }

    /// Connects as [`Client::connect`] does, and for an `https://` address also trusts the PEM CA
    /// certificates in `ca_pem`, such as a team's own CA (agni issue 1765).
    pub async fn connect_with_ca(
        address: impl Into<String>,
        token: Option<&str>,
        ca_pem: Option<&[u8]>,
    ) -> Result<Client, Error> {
        let address = address.into();
        let mut endpoint = Endpoint::from_shared(address.clone())
            .map_err(|e| Error::Address(format!("{address}: {e}")))?;
        if address.starts_with("https://") {
            let mut tls = ClientTlsConfig::new().with_native_roots();
            if let Some(pem) = ca_pem {
                tls = tls.ca_certificate(Certificate::from_pem(pem));
            }
            endpoint = endpoint.tls_config(tls).map_err(Error::Transport)?;
        }
        let channel = endpoint.connect().await.map_err(Error::Transport)?;
        Client::with_channel(channel, token).await
    }

    /// Checks the server behind a channel the caller built, such as one with TLS settings, and
    /// returns a client over it.
    pub async fn with_channel(channel: Channel, token: Option<&str>) -> Result<Client, Error> {
        let bearer = Bearer::new(token)?;
        let transport = InterceptedService::new(channel, bearer);
        let info = ServerServiceClient::new(transport.clone())
            .get_server_info(GetServerInfoRequest {})
            .await
            .map_err(Error::Status)?
            .into_inner();
        check_api(&info)?;
        Ok(Client { transport, info })
    }

    /// What the server said about itself when the client connected.
    pub fn info(&self) -> &ServerInfo {
        &self.info
    }

    /// Whether the server has an optional feature on, such as `grpc` or `snapshot_recovery`. A
    /// client tests a feature by name rather than comparing versions.
    pub fn has_feature(&self, name: &str) -> bool {
        self.info.features.iter().any(|f| f == name)
    }

    /// The channel with the token, for a service this type has no accessor for, as in
    /// `DiffServiceClient::new(client.transport())`.
    pub fn transport(&self) -> Transport {
        self.transport.clone()
    }

    /// `agni.v1.webapi.ServerService`.
    pub fn server(&self) -> ServerServiceClient<Transport> {
        ServerServiceClient::new(self.transport())
    }

    /// `agni.v1.webapi.SnapshotService`.
    pub fn snapshots(&self) -> SnapshotServiceClient<Transport> {
        SnapshotServiceClient::new(self.transport())
    }

    /// `agni.v1.webapi.DesignService`, the designs a server holds (agni issue 1694).
    pub fn designs(&self) -> DesignServiceClient<Transport> {
        DesignServiceClient::new(self.transport())
    }

    /// `agni.v1.webapi.ConnectionService`, the connections a server reads private sources through
    /// (agni issue 1724).
    pub fn connections(&self) -> ConnectionServiceClient<Transport> {
        ConnectionServiceClient::new(self.transport())
    }

    /// `agni.v1.webapi.PackService`, the packs of rules a server keeps (agni issue 1704).
    pub fn packs(&self) -> PackServiceClient<Transport> {
        PackServiceClient::new(self.transport())
    }

    /// `agni.v1.webapi.DesignViewService`.
    pub fn design_views(&self) -> DesignViewServiceClient<Transport> {
        DesignViewServiceClient::new(self.transport())
    }

    /// `agni.v1.webapi.CheckService`.
    pub fn checks(&self) -> CheckServiceClient<Transport> {
        CheckServiceClient::new(self.transport())
    }

    /// `agni.v1.webapi.QueryService`.
    pub fn queries(&self) -> QueryServiceClient<Transport> {
        QueryServiceClient::new(self.transport())
    }
}

/// The contract's first rule. A client refuses a server that speaks another package.
fn check_api(info: &ServerInfo) -> Result<(), Error> {
    if info.api == API {
        Ok(())
    } else {
        Err(Error::Api(info.api.clone()))
    }
}

impl fmt::Debug for Client {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Client")
            .field("info", &self.info)
            .finish_non_exhaustive()
    }
}

/// Adds the bearer token to every call, when there is one.
#[derive(Clone, Default)]
pub struct Bearer(Option<MetadataValue<Ascii>>);

impl Bearer {
    /// A token of `None` sends no `authorization` metadata.
    pub fn new(token: Option<&str>) -> Result<Bearer, Error> {
        match token {
            None => Ok(Bearer(None)),
            Some(t) => format!("Bearer {t}")
                .parse()
                .map(|v| Bearer(Some(v)))
                .map_err(|_| Error::Token),
        }
    }
}

impl Interceptor for Bearer {
    fn call(&mut self, mut req: Request<()>) -> Result<Request<()>, Status> {
        if let Some(v) = &self.0 {
            req.metadata_mut().insert("authorization", v.clone());
        }
        Ok(req)
    }
}

impl fmt::Debug for Bearer {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        // The token is a secret, so a log line says only whether there is one.
        f.write_str(if self.0.is_some() {
            "Bearer(set)"
        } else {
            "Bearer(none)"
        })
    }
}

/// Why [`Client::connect`] failed.
#[derive(Debug)]
pub enum Error {
    /// The address does not parse as a URI.
    Address(String),
    /// The token holds characters a header cannot carry.
    Token,
    /// The connection failed.
    Transport(tonic::transport::Error),
    /// `GetServerInfo` failed. [`details`] reads what the status carries.
    Status(Status),
    /// The server speaks another package than [`API`].
    Api(String),
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::Address(e) => write!(f, "the server address does not parse: {e}"),
            Error::Token => f.write_str("the bearer token holds characters a header cannot carry"),
            Error::Transport(e) => {
                // tonic's own text is "transport error", and the cause sits in its sources.
                write!(f, "connecting to the server: {e}")?;
                let mut cause = std::error::Error::source(e);
                while let Some(c) = cause {
                    write!(f, ": {c}")?;
                    cause = c.source();
                }
                Ok(())
            }
            Error::Status(s) => write!(
                f,
                "calling GetServerInfo: {}: {}",
                code_name(s.code()),
                s.message()
            ),
            Error::Api(api) => write!(
                f,
                "the server speaks {api:?}, and this client was built for {API}"
            ),
        }
    }
}

impl std::error::Error for Error {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            Error::Transport(e) => Some(e),
            Error::Status(s) => Some(s),
            _ => None,
        }
    }
}

impl From<Status> for Error {
    fn from(s: Status) -> Error {
        Error::Status(s)
    }
}

/// One error detail a failed call carries.
#[derive(Clone, Debug, PartialEq)]
pub enum Detail {
    /// Where a query or a rule stopped parsing, with `invalid_argument`.
    TextPosition(TextPosition),
    /// Which snapshot is missing or stale, with `not_found` or `failed_precondition`.
    SnapshotUnavailable(SnapshotUnavailable),
    /// Which file contents to upload before ingesting again, with `failed_precondition`.
    BlobsMissing(BlobsMissing),
    /// A detail of a type this crate does not know, kept as it came.
    Other(pbjson_types::Any),
}

impl Detail {
    /// The detail's full proto name, such as `agni.v1.webapi.TextPosition`.
    pub fn type_name(&self) -> &str {
        match self {
            Detail::TextPosition(_) => "agni.v1.webapi.TextPosition",
            Detail::SnapshotUnavailable(_) => "agni.v1.webapi.SnapshotUnavailable",
            Detail::BlobsMissing(_) => "agni.v1.webapi.BlobsMissing",
            Detail::Other(any) => type_name(&any.type_url),
        }
    }
}

/// `google.rpc.Status`, which gRPC carries in the `grpc-status-details-bin` trailer. tonic exposes
/// its bytes as `Status::details`.
#[derive(Clone, PartialEq, prost::Message)]
struct RpcStatus {
    #[prost(int32, tag = "1")]
    code: i32,
    #[prost(string, tag = "2")]
    message: String,
    #[prost(message, repeated, tag = "3")]
    details: Vec<pbjson_types::Any>,
}

fn type_name(type_url: &str) -> &str {
    type_url.rsplit('/').next().unwrap_or(type_url)
}

/// The details a failed call carries, in the order the server sent them. A status with no details
/// gives an empty list.
pub fn details(status: &Status) -> Result<Vec<Detail>, prost::DecodeError> {
    use prost::Message;
    if status.details().is_empty() {
        return Ok(Vec::new());
    }
    let rpc = RpcStatus::decode(status.details())?;
    rpc.details
        .into_iter()
        .map(|any| {
            Ok(match type_name(&any.type_url) {
                "agni.v1.webapi.TextPosition" => {
                    Detail::TextPosition(TextPosition::decode(any.value)?)
                }
                "agni.v1.webapi.SnapshotUnavailable" => {
                    Detail::SnapshotUnavailable(SnapshotUnavailable::decode(any.value)?)
                }
                "agni.v1.webapi.BlobsMissing" => {
                    Detail::BlobsMissing(BlobsMissing::decode(any.value)?)
                }
                _ => Detail::Other(any),
            })
        })
        .collect()
}

/// The `TextPosition` a failed query or rule carries, if any.
pub fn text_position(status: &Status) -> Option<TextPosition> {
    details(status).ok()?.into_iter().find_map(|d| match d {
        Detail::TextPosition(p) => Some(p),
        _ => None,
    })
}

/// The `SnapshotUnavailable` a call on a missing or stale snapshot carries, if any.
pub fn snapshot_unavailable(status: &Status) -> Option<SnapshotUnavailable> {
    details(status).ok()?.into_iter().find_map(|d| match d {
        Detail::SnapshotUnavailable(s) => Some(s),
        _ => None,
    })
}

/// The `BlobsMissing` an ingest of uploaded files carries, if any.
pub fn blobs_missing(status: &Status) -> Option<BlobsMissing> {
    details(status).ok()?.into_iter().find_map(|d| match d {
        Detail::BlobsMissing(b) => Some(b),
        _ => None,
    })
}

/// A code's name as Connect and the wire fixtures spell it, such as `invalid_argument`. gRPC and
/// Connect share the codes and differ only in spelling.
pub fn code_name(code: Code) -> &'static str {
    match code {
        Code::Ok => "ok",
        Code::Cancelled => "canceled",
        Code::Unknown => "unknown",
        Code::InvalidArgument => "invalid_argument",
        Code::DeadlineExceeded => "deadline_exceeded",
        Code::NotFound => "not_found",
        Code::AlreadyExists => "already_exists",
        Code::PermissionDenied => "permission_denied",
        Code::ResourceExhausted => "resource_exhausted",
        Code::FailedPrecondition => "failed_precondition",
        Code::Aborted => "aborted",
        Code::OutOfRange => "out_of_range",
        Code::Unimplemented => "unimplemented",
        Code::Internal => "internal",
        Code::Unavailable => "unavailable",
        Code::DataLoss => "data_loss",
        Code::Unauthenticated => "unauthenticated",
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use prost::Message;

    #[test]
    fn details_reads_each_known_type_and_keeps_others() {
        let pos = TextPosition {
            offset: 18,
            line: 1,
            column: 19,
            caret: "x\n^".into(),
        };
        let rpc = RpcStatus {
            code: Code::InvalidArgument as i32,
            message: "does not parse".into(),
            details: vec![
                pbjson_types::Any {
                    type_url: "type.googleapis.com/agni.v1.webapi.TextPosition".into(),
                    value: pos.encode_to_vec().into(),
                },
                pbjson_types::Any {
                    type_url: "type.googleapis.com/x.Unknown".into(),
                    value: vec![1u8].into(),
                },
            ],
        };
        let status = Status::with_details(
            Code::InvalidArgument,
            "does not parse",
            rpc.encode_to_vec().into(),
        );
        let got = details(&status).unwrap();
        assert_eq!(got.len(), 2);
        assert_eq!(got[0], Detail::TextPosition(pos.clone()));
        assert_eq!(got[1].type_name(), "x.Unknown");
        assert_eq!(text_position(&status), Some(pos));
        assert_eq!(snapshot_unavailable(&status), None);
        assert!(
            details(&Status::not_found("no details"))
                .unwrap()
                .is_empty()
        );
    }

    #[test]
    fn check_api_refuses_another_package() {
        let info = |api: &str| ServerInfo {
            api: api.into(),
            ..Default::default()
        };
        assert!(check_api(&info("agni.v1")).is_ok());
        let err = check_api(&info("agni.v2")).unwrap_err();
        assert!(matches!(&err, Error::Api(a) if a == "agni.v2"));
        assert_eq!(
            err.to_string(),
            r#"the server speaks "agni.v2", and this client was built for agni.v1"#
        );
        assert!(check_api(&info("")).is_err());
    }

    #[test]
    fn bearer_refuses_a_token_a_header_cannot_carry() {
        assert!(Bearer::new(Some("abc")).is_ok());
        assert!(matches!(Bearer::new(Some("a\nb")), Err(Error::Token)));
        assert_eq!(
            format!("{:?}", Bearer::new(Some("secret")).unwrap()),
            "Bearer(set)"
        );
    }
}
