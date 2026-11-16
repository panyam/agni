#include "agni/client.h"

#include <algorithm>
#include <stdexcept>
#include <utility>

#include <google/protobuf/unknown_field_set.h>
#include <grpcpp/support/client_interceptor.h>

namespace agni {
namespace {

using grpc::experimental::ClientInterceptorFactoryInterface;
using grpc::experimental::ClientRpcInfo;
using grpc::experimental::InterceptionHookPoints;
using grpc::experimental::Interceptor;
using grpc::experimental::InterceptorBatchMethods;

// BearerToken adds the authorization metadata to every call. A gRPC call credential would do the
// same, but gRPC sends call credentials only over TLS, and agni serve speaks cleartext for now.
class BearerToken : public Interceptor {
 public:
  explicit BearerToken(const std::string* value) : value_(value) {}
  void Intercept(InterceptorBatchMethods* methods) override {
    if (methods->QueryInterceptionHookPoint(InterceptionHookPoints::PRE_SEND_INITIAL_METADATA)) {
      methods->GetSendInitialMetadata()->insert({"authorization", *value_});
    }
    methods->Proceed();
  }

 private:
  const std::string* value_;
};

class BearerTokenFactory : public ClientInterceptorFactoryInterface {
 public:
  explicit BearerTokenFactory(std::string token) : value_("Bearer " + std::move(token)) {}
  Interceptor* CreateClientInterceptor(ClientRpcInfo*) override { return new BearerToken(&value_); }

 private:
  // The channel owns this factory and outlives every interceptor it makes.
  const std::string value_;
};

}  // namespace

std::string GrpcTarget(std::string_view server) {
  std::string_view s = server;
  if (auto at = s.find("://"); at != std::string_view::npos) {
    std::string_view scheme = s.substr(0, at);
    if (scheme == "https") {
      throw std::invalid_argument("agni serve has no TLS yet (agni issue 1627), so " + std::string(server) +
                                  " cannot be reached; use http://");
    }
    if (scheme != "http") {
      throw std::invalid_argument("the server " + std::string(server) + " is not http:// or host:port");
    }
    s.remove_prefix(at + 3);
  }
  while (!s.empty() && s.back() == '/') s.remove_suffix(1);
  if (s.empty() || s.find('/') != std::string_view::npos) {
    throw std::invalid_argument("the server " + std::string(server) + " is not a URL with only a host and port");
  }
  return std::string(s);
}

Client::Client(std::string_view server, std::string token) : target_(GrpcTarget(server)) {
  auto creds = grpc::InsecureChannelCredentials();
  if (token.empty()) {
    channel_ = grpc::CreateChannel(target_, creds);
    return;
  }
  std::vector<std::unique_ptr<ClientInterceptorFactoryInterface>> interceptors;
  interceptors.push_back(std::make_unique<BearerTokenFactory>(std::move(token)));
  channel_ = grpc::experimental::CreateCustomChannelWithInterceptors(target_, creds, grpc::ChannelArguments(),
                                                                     std::move(interceptors));
}

grpc::Status Client::Connect(v1::webapi::ServerInfo* info) {
  v1::webapi::ServerInfo got;
  grpc::ClientContext ctx;
  grpc::Status st = Stub<v1::webapi::ServerService>()->GetServerInfo(&ctx, v1::webapi::GetServerInfoRequest(), &got);
  if (!st.ok()) return st;
  if (got.api() != kApi) {
    return grpc::Status(grpc::StatusCode::FAILED_PRECONDITION,
                        "the server at " + target_ + " speaks \"" + got.api() + "\", and this client was built for " +
                            std::string(kApi));
  }
  info_ = got;
  if (info != nullptr) *info = std::move(got);
  return grpc::Status::OK;
}

bool Client::HasFeature(std::string_view feature) const {
  const auto& f = info_.features();
  return std::find(f.begin(), f.end(), feature) != f.end();
}

std::string CodeName(grpc::StatusCode code) {
  switch (code) {
    case grpc::StatusCode::OK: return "ok";
    case grpc::StatusCode::CANCELLED: return "canceled";
    case grpc::StatusCode::UNKNOWN: return "unknown";
    case grpc::StatusCode::INVALID_ARGUMENT: return "invalid_argument";
    case grpc::StatusCode::DEADLINE_EXCEEDED: return "deadline_exceeded";
    case grpc::StatusCode::NOT_FOUND: return "not_found";
    case grpc::StatusCode::ALREADY_EXISTS: return "already_exists";
    case grpc::StatusCode::PERMISSION_DENIED: return "permission_denied";
    case grpc::StatusCode::RESOURCE_EXHAUSTED: return "resource_exhausted";
    case grpc::StatusCode::FAILED_PRECONDITION: return "failed_precondition";
    case grpc::StatusCode::ABORTED: return "aborted";
    case grpc::StatusCode::OUT_OF_RANGE: return "out_of_range";
    case grpc::StatusCode::UNIMPLEMENTED: return "unimplemented";
    case grpc::StatusCode::INTERNAL: return "internal";
    case grpc::StatusCode::UNAVAILABLE: return "unavailable";
    case grpc::StatusCode::DATA_LOSS: return "data_loss";
    case grpc::StatusCode::UNAUTHENTICATED: return "unauthenticated";
    default: return "code_" + std::to_string(static_cast<int>(code));
  }
}

std::vector<google::protobuf::Any> ErrorDetails(const grpc::Status& status) {
  // google.rpc.Status is code = 1, message = 2 and repeated Any details = 3. Reading field 3 as an
  // unknown field spares every user of this library a copy of googleapis' status.proto.
  std::vector<google::protobuf::Any> out;
  google::protobuf::UnknownFieldSet fields;
  if (status.error_details().empty() || !fields.ParseFromString(status.error_details())) return out;
  for (int i = 0; i < fields.field_count(); ++i) {
    const auto& f = fields.field(i);
    if (f.number() != 3 || f.type() != google::protobuf::UnknownField::TYPE_LENGTH_DELIMITED) continue;
    google::protobuf::Any any;
    if (!any.ParseFromString(f.length_delimited())) return {};
    out.push_back(std::move(any));
  }
  return out;
}

}  // namespace agni
