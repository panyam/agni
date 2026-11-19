// agni's C++ client (agni issue 1638). It calls a shared `agni serve` over gRPC with the stubs
// generated from protos/agni/v1, and adds what docsite/content/build/client-contract.md asks of every
// client: a bearer token on each call, a check of the server before anything else, and the error
// details a failed call carries.
//
//   agni::Client client("http://127.0.0.1:8080");
//   agni::v1::webapi::ServerInfo info;
//   grpc::Status st = client.Connect(&info);
//   auto snapshots = client.Stub<agni::v1::webapi::SnapshotService>();
//
// Every rpc goes through a stub, and each call takes its own grpc::ClientContext. The client keeps
// no snapshot state. A server with snapshot_recovery ingests an evicted snapshot again by itself.
#ifndef AGNI_CLIENT_H_
#define AGNI_CLIENT_H_

#include <memory>
#include <optional>
#include <string>
#include <string_view>
#include <vector>

#include <google/protobuf/any.pb.h>
#include <grpcpp/grpcpp.h>

#include "agni/v1/webapi/query.pb.h"
#include "agni/v1/webapi/server.grpc.pb.h"
#include "agni/v1/webapi/snapshot.pb.h"

namespace agni {

// The proto package this client was generated from. Connect refuses a server that speaks another.
inline constexpr std::string_view kApi = "agni.v1";

// GrpcTarget turns a server URL, such as http://127.0.0.1:8080, into the host:port gRPC dials. A bare
// host:port passes through. It throws std::invalid_argument for https://, since this client can't
// trust a CA yet (agni issue 1765), and for any other scheme.
std::string GrpcTarget(std::string_view server);

class Client {
 public:
  // server is a URL or host:port. A non-empty token goes on every call as the metadata
  // `authorization: Bearer <token>`. A server started with --auth tokens refuses a call without a
  // good one (agni issue 1627).
  explicit Client(std::string_view server, std::string token = "");

  // Connect calls ServerService.GetServerInfo and keeps the answer. A server whose api is not
  // agni.v1 is refused with FAILED_PRECONDITION and a message naming both packages. The answer is
  // written to info when info is not null.
  grpc::Status Connect(v1::webapi::ServerInfo* info = nullptr);

  // The answer Connect kept, empty before Connect succeeds.
  const v1::webapi::ServerInfo& server_info() const { return info_; }

  // HasFeature says whether the server Connect reached has a feature on, such as "grpc" or
  // "snapshot_store". A client tests for a feature by name rather than comparing versions.
  bool HasFeature(std::string_view feature) const;

  // Stub returns a stub for one of agni's services on this client's channel, such as
  // client.Stub<agni::v1::webapi::DesignViewService>().
  template <class Service>
  std::unique_ptr<typename Service::Stub> Stub() const {
    return Service::NewStub(channel_);
  }

  std::shared_ptr<grpc::Channel> channel() const { return channel_; }
  const std::string& target() const { return target_; }

 private:
  std::string target_;
  std::shared_ptr<grpc::Channel> channel_;
  v1::webapi::ServerInfo info_;
};

// CodeName is a status code as Connect and the wire fixtures name it, such as "invalid_argument".
std::string CodeName(grpc::StatusCode code);

// ErrorDetails decodes the details a failed call carries. gRPC sends them as a serialized
// google.rpc.Status, which grpc::Status::error_details() holds. It returns none for a status with no
// details, and none when the bytes do not parse.
std::vector<google::protobuf::Any> ErrorDetails(const grpc::Status& status);

// FindDetail returns the first detail of type M a failed call carries, or nothing.
template <class M>
std::optional<M> FindDetail(const grpc::Status& status) {
  for (const auto& any : ErrorDetails(status)) {
    M m;
    if (any.Is<M>() && any.UnpackTo(&m)) return m;
  }
  return std::nullopt;
}

// Where a query or a rule stopped parsing, with invalid_argument.
inline std::optional<v1::webapi::TextPosition> TextPositionOf(const grpc::Status& status) {
  return FindDetail<v1::webapi::TextPosition>(status);
}

// Which snapshot cannot answer and why, with not_found or failed_precondition.
inline std::optional<v1::webapi::SnapshotUnavailable> SnapshotUnavailableOf(const grpc::Status& status) {
  return FindDetail<v1::webapi::SnapshotUnavailable>(status);
}

// Which file contents to upload before ingesting again, with failed_precondition.
inline std::optional<v1::webapi::BlobsMissing> BlobsMissingOf(const grpc::Status& status) {
  return FindDetail<v1::webapi::BlobsMissing>(status);
}

}  // namespace agni

#endif  // AGNI_CLIENT_H_
