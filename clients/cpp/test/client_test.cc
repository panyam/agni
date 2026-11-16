// client_test holds the client's own layer to docsite/content/build/client-contract.md, against a
// stand-in ServerService in this process: the bearer token on each call, the refusal of a server
// that speaks another package, the URL a client is given, and the decoding of error details.
#include <iostream>
#include <memory>
#include <stdexcept>
#include <string>

#include <grpcpp/grpcpp.h>

#include "agni/client.h"
#include "agni/v1/webapi/server.grpc.pb.h"

namespace webapi = agni::v1::webapi;

namespace {

int failures = 0;

void Expect(bool ok, const std::string& what) {
  std::cout << (ok ? "ok   " : "FAIL ") << what << "\n";
  if (!ok) ++failures;
}

// StandIn answers GetServerInfo with the api it is told to, and keeps the authorization metadata
// of the last call.
class StandIn final : public webapi::ServerService::Service {
 public:
  std::string api = "agni.v1";
  std::string authorization;

  grpc::Status GetServerInfo(grpc::ServerContext* ctx, const webapi::GetServerInfoRequest*,
                             webapi::ServerInfo* out) override {
    auto it = ctx->client_metadata().find("authorization");
    authorization = it == ctx->client_metadata().end() ? "" : std::string(it->second.data(), it->second.size());
    out->set_version("v0.0.0-test");
    out->set_api(api);
    out->add_features("grpc");
    return grpc::Status::OK;
  }
};

bool Throws(const std::string& server) {
  try {
    (void)agni::GrpcTarget(server);
  } catch (const std::invalid_argument&) {
    return true;
  }
  return false;
}

}  // namespace

int main() {
  StandIn service;
  int port = 0;
  grpc::ServerBuilder builder;
  builder.AddListeningPort("127.0.0.1:0", grpc::InsecureServerCredentials(), &port);
  builder.RegisterService(&service);
  std::unique_ptr<grpc::Server> server = builder.BuildAndStart();
  if (!server || port == 0) {
    std::cerr << "client_test: the stand-in server did not start\n";
    return 1;
  }
  std::string url = "http://127.0.0.1:" + std::to_string(port);

  agni::Client with_token(url, "s3cret");
  webapi::ServerInfo info;
  grpc::Status st = with_token.Connect(&info);
  Expect(st.ok(), "Connect answers from an agni.v1 server: " + st.error_message());
  Expect(info.api() == "agni.v1" && with_token.server_info().version() == "v0.0.0-test", "Connect keeps the answer");
  Expect(with_token.HasFeature("grpc") && !with_token.HasFeature("snapshot_store"), "HasFeature tests by name");
  Expect(service.authorization == "Bearer s3cret", "the token goes as authorization: Bearer, got \"" + service.authorization + "\"");

  agni::Client without_token(url);
  Expect(without_token.Connect().ok() && service.authorization.empty(), "no token sends no authorization metadata");

  service.api = "agni.v2";
  agni::Client refused(url);
  st = refused.Connect();
  Expect(st.error_code() == grpc::StatusCode::FAILED_PRECONDITION, "Connect refuses agni.v2 with failed_precondition");
  Expect(st.error_message().find("agni.v2") != std::string::npos && st.error_message().find("agni.v1") != std::string::npos,
         "the refusal names both packages: " + st.error_message());
  Expect(refused.server_info().api().empty(), "a refused server's answer is not kept");

  Expect(agni::GrpcTarget("http://127.0.0.1:8080") == "127.0.0.1:8080", "an http URL dials host:port");
  Expect(agni::GrpcTarget("http://localhost:8080/") == "localhost:8080", "a trailing slash is dropped");
  Expect(agni::GrpcTarget("127.0.0.1:8080") == "127.0.0.1:8080", "a bare host:port passes through");
  Expect(Throws("https://agni.example:443"), "https is refused until agni serve has TLS");
  Expect(Throws("ftp://host:1") && Throws("http://host:1/api"), "another scheme or a path is refused");

  Expect(agni::ErrorDetails(grpc::Status(grpc::StatusCode::NOT_FOUND, "x")).empty(), "a status with no details has none");
  Expect(agni::ErrorDetails(grpc::Status(grpc::StatusCode::NOT_FOUND, "x", "\xff\xff\xff")).empty(),
         "details that do not parse read as none");
  // A google.rpc.Status written by hand: code 3, message "bad", and one TextPosition detail.
  webapi::TextPosition pos;
  pos.set_line(2);
  pos.set_column(7);
  google::protobuf::Any any;
  Expect(any.PackFrom(pos), "a TextPosition packs into an Any");
  std::string packed = any.SerializeAsString();
  std::string rpc_status = std::string("\x08\x03\x12\x03" "bad\x1a", 8) + static_cast<char>(packed.size()) + packed;
  grpc::Status bad(grpc::StatusCode::INVALID_ARGUMENT, "bad", rpc_status);
  auto at = agni::TextPositionOf(bad);
  Expect(at && at->line() == 2 && at->column() == 7, "TextPositionOf reads the detail out of google.rpc.Status");
  Expect(!agni::SnapshotUnavailableOf(bad) && !agni::BlobsMissingOf(bad), "a detail of another type is not found");

  Expect(agni::CodeName(grpc::StatusCode::FAILED_PRECONDITION) == "failed_precondition", "CodeName names codes as Connect does");

  server->Shutdown();
  std::cout << (failures == 0 ? "all passed\n" : std::to_string(failures) + " failed\n");
  return failures == 0 ? 0 : 1;
}
