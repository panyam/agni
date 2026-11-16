// wirefixtures_test replays clients/wirefixtures/fixtures.json through the C++ client against a real
// agni serve, and compares each answer with the recording (agni issues 1633 and 1638). A response
// compares as a message, leaving out the fields a fixture lists under ignore. An error compares its
// code and its details, and not its message, which is prose.
//
//   wirefixtures_test --agni <binary> --mount probe=<abs path>/examples/cross-probe-project \
//     --fixtures <abs path>/clients/wirefixtures/fixtures.json
//
// It starts the server itself on a free port, from an empty working directory with its own HOME, so
// the server serves the API alone and reads no one's agni.yaml. It kills the server when it is done.
#include <arpa/inet.h>
#include <fcntl.h>
#include <netinet/in.h>
#include <signal.h>
#include <sys/socket.h>
#include <sys/wait.h>
#include <unistd.h>

#include <chrono>
#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <functional>
#include <iostream>
#include <map>
#include <memory>
#include <sstream>
#include <string>
#include <vector>

#include <google/protobuf/descriptor.h>
#include <google/protobuf/message.h>
#include <google/protobuf/util/json_util.h>
#include <google/protobuf/util/message_differencer.h>

#include "agni/client.h"
#include "agni/v1/webapi/checks.grpc.pb.h"
#include "agni/v1/webapi/design.grpc.pb.h"
#include "agni/v1/webapi/query.grpc.pb.h"
#include "agni/v1/webapi/server.grpc.pb.h"
#include "agni/v1/webapi/snapshot.grpc.pb.h"
#include "wirefixtures.pb.h"

namespace fs = std::filesystem;
namespace gp = google::protobuf;
namespace webapi = agni::v1::webapi;

namespace {

[[noreturn]] void Fatal(const std::string& msg) {
  std::cerr << "wirefixtures_test: " << msg << "\n";
  std::exit(1);
}

// Server is one agni serve this test started. It is killed when the Server goes out of scope.
class Server {
 public:
  Server(const std::string& agni, const std::string& mount) {
    work_ = fs::temp_directory_path() / ("agni-cpp-wire-" + std::to_string(getpid()));
    fs::remove_all(work_);
    fs::create_directories(work_ / "home");
    log_ = work_ / "serve.log";
    addr_ = "127.0.0.1:" + std::to_string(FreePort());
    pid_ = fork();
    if (pid_ < 0) Fatal("fork failed");
    if (pid_ == 0) {
      int fd = open(log_.c_str(), O_WRONLY | O_CREAT | O_TRUNC, 0600);
      if (fd >= 0) {
        dup2(fd, 1);
        dup2(fd, 2);
      }
      if (chdir(work_.c_str()) != 0) _exit(126);
      setenv("HOME", (work_ / "home").c_str(), 1);
      setenv("XDG_CONFIG_HOME", (work_ / "home").c_str(), 1);
      execl(agni.c_str(), agni.c_str(), "serve", "--addr", addr_.c_str(), "--mount", mount.c_str(),
            static_cast<char*>(nullptr));
      _exit(127);
    }
  }
  ~Server() { Stop(); }

  void Stop() {
    if (pid_ > 0) {
      kill(pid_, SIGKILL);
      waitpid(pid_, nullptr, 0);
      pid_ = -1;
      fs::remove_all(work_);
    }
  }

  const std::string& addr() const { return addr_; }

  bool Running() {
    int status = 0;
    return pid_ > 0 && waitpid(pid_, &status, WNOHANG) == 0;
  }

  std::string Log() const {
    std::ifstream in(log_);
    std::stringstream s;
    s << in.rdbuf();
    return s.str();
  }

 private:
  static int FreePort() {
    int s = socket(AF_INET, SOCK_STREAM, 0);
    sockaddr_in a{};
    a.sin_family = AF_INET;
    a.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    socklen_t n = sizeof a;
    if (s < 0 || bind(s, reinterpret_cast<sockaddr*>(&a), n) != 0 ||
        getsockname(s, reinterpret_cast<sockaddr*>(&a), &n) != 0) {
      Fatal("no free port");
    }
    close(s);
    return ntohs(a.sin_port);
  }

  fs::path work_, log_;
  std::string addr_;
  pid_t pid_ = -1;
};

std::string ToJson(const gp::Message& m) {
  std::string out;
  auto st = gp::util::MessageToJsonString(m, &out);
  if (!st.ok()) Fatal("printing " + std::string(m.GetTypeName()) + ": " + std::string(st.ToString()));
  return out;
}

// Parse reads a fixture's Struct into a message of the rpc's own type. Unknown fields are refused,
// so a fixture naming a field these protos lack fails here rather than comparing as unset.
void Parse(const gp::Struct& from, gp::Message* to, const std::string& what) {
  auto st = gp::util::JsonStringToMessage(ToJson(from), to);
  if (!st.ok()) Fatal(what + ": " + std::string(st.ToString()));
}

// ClearPath clears a field named by its proto name, dotted through singular messages, as the Go
// replayer's clearPath does.
void ClearPath(gp::Message* m, const std::string& path) {
  auto dot = path.find('.');
  const gp::FieldDescriptor* f = m->GetDescriptor()->FindFieldByName(path.substr(0, dot));
  if (f == nullptr) Fatal("ignore names " + path + ", which " + std::string(m->GetTypeName()) + " has no field for");
  const gp::Reflection* r = m->GetReflection();
  if (dot == std::string::npos) {
    r->ClearField(m, f);
  } else if (f->cpp_type() == gp::FieldDescriptor::CPPTYPE_MESSAGE && !f->is_repeated() && r->HasField(*m, f)) {
    ClearPath(r->MutableMessage(m, f), path.substr(dot + 1));
  }
}

// Diff is how two messages of one type differ, or "".
std::string Diff(const gp::Message& want, const gp::Message& got) {
  std::string report;
  gp::util::MessageDifferencer d;
  d.ReportDifferencesToString(&report);
  return d.Compare(want, got) ? "" : report;
}

// Outcome is what one call answered.
struct Outcome {
  grpc::Status status;
  std::unique_ptr<gp::Message> response;
};

using Call = std::function<Outcome(const agni::Client&, const gp::Struct& request, const std::string& name)>;

// Rpc calls one rpc through the client's typed stub for its service.
template <class Service, class Req, class Resp>
Call Rpc(grpc::Status (Service::Stub::*method)(grpc::ClientContext*, const Req&, Resp*)) {
  return [method](const agni::Client& client, const gp::Struct& request, const std::string& name) {
    Req req;
    Parse(request, &req, name + " request");
    auto stub = client.Stub<Service>();
    grpc::ClientContext ctx;
    ctx.set_deadline(std::chrono::system_clock::now() + std::chrono::seconds(120));
    auto resp = std::make_unique<Resp>();
    Outcome o;
    o.status = ((*stub).*method)(&ctx, req, resp.get());
    o.response = std::move(resp);
    return o;
  };
}

// The rpcs the fixtures call, by service/method. A fixture naming another fails and says to add it.
const std::map<std::string, Call>& Calls() {
  static const auto* calls = new std::map<std::string, Call>{
      {"agni.v1.webapi.ServerService/GetServerInfo", Rpc<webapi::ServerService>(&webapi::ServerService::Stub::GetServerInfo)},
      {"agni.v1.webapi.SnapshotService/Ingest", Rpc<webapi::SnapshotService>(&webapi::SnapshotService::Stub::Ingest)},
      {"agni.v1.webapi.SnapshotService/DeleteSnapshot", Rpc<webapi::SnapshotService>(&webapi::SnapshotService::Stub::DeleteSnapshot)},
      {"agni.v1.webapi.DesignService/GetDesign", Rpc<webapi::DesignService>(&webapi::DesignService::Stub::GetDesign)},
      {"agni.v1.webapi.CheckService/CheckDesign", Rpc<webapi::CheckService>(&webapi::CheckService::Stub::CheckDesign)},
      {"agni.v1.webapi.QueryService/RunQuery", Rpc<webapi::QueryService>(&webapi::QueryService::Stub::RunQuery)},
  };
  return *calls;
}

// Replay makes one fixture's call and says how its answer differs from the recording, or "".
std::string Replay(const agni::Client& client, const agni::clienttest::Fixture& f) {
  auto call = Calls().find(f.service() + "/" + f.method());
  if (call == Calls().end()) {
    return "the replayer has no call for " + f.service() + "/" + f.method() + "; add it to Calls()";
  }
  Outcome got = call->second(client, f.request(), f.name());
  if (f.has_error()) {
    if (got.status.ok()) return "answered, want the error " + f.error().code();
    std::string code = agni::CodeName(got.status.error_code());
    if (code != f.error().code()) return "failed with " + code + " (" + got.status.error_message() + "), want " + f.error().code();
    auto details = agni::ErrorDetails(got.status);
    if (details.size() != static_cast<size_t>(f.error().details_size())) {
      return "carries " + std::to_string(details.size()) + " details, want " + std::to_string(f.error().details_size());
    }
    for (int i = 0; i < f.error().details_size(); ++i) {
      const auto& want = f.error().details(i);
      const auto& any = details[i];
      std::string type = any.type_url().substr(any.type_url().rfind('/') + 1);
      if (type != want.type()) return "detail " + std::to_string(i) + " is " + type + ", want " + want.type();
      const gp::Descriptor* d = gp::DescriptorPool::generated_pool()->FindMessageTypeByName(want.type());
      if (d == nullptr) return "detail " + std::to_string(i) + " is " + want.type() + ", which these protos lack";
      const gp::Message* proto = gp::MessageFactory::generated_factory()->GetPrototype(d);
      std::unique_ptr<gp::Message> g(proto->New()), w(proto->New());
      if (!any.UnpackTo(g.get())) return "detail " + std::to_string(i) + " does not unpack as " + want.type();
      Parse(want.value(), w.get(), f.name() + " detail");
      if (auto diff = Diff(*w, *g); !diff.empty()) return "detail " + std::to_string(i) + " differs:\n" + diff;
    }
    return "";
  }
  if (!got.status.ok()) {
    return "failed with " + agni::CodeName(got.status.error_code()) + " (" + got.status.error_message() + "), want an answer";
  }
  std::unique_ptr<gp::Message> want(got.response->New());
  Parse(f.response(), want.get(), f.name() + " response");
  for (const auto& p : f.ignore()) {
    ClearPath(want.get(), p);
    ClearPath(got.response.get(), p);
  }
  if (auto diff = Diff(*want, *got.response); !diff.empty()) return "answered differently:\n" + diff;
  return "";
}

std::string Flag(int argc, char** argv, const std::string& name) {
  for (int i = 1; i + 1 < argc; ++i) {
    if (argv[i] == "--" + name) return argv[i + 1];
  }
  Fatal("missing --" + name);
}

}  // namespace

int main(int argc, char** argv) {
  std::string agni = Flag(argc, argv, "agni"), mount = Flag(argc, argv, "mount"), path = Flag(argc, argv, "fixtures");

  std::ifstream in(path);
  if (!in) Fatal("cannot read " + path + " (make wire-fixtures records it)");
  std::stringstream raw;
  raw << in.rdbuf();
  agni::clienttest::FixtureFile file;
  if (auto st = gp::util::JsonStringToMessage(raw.str(), &file); !st.ok()) Fatal(path + ": " + std::string(st.ToString()));

  // The server goes up before any gRPC channel exists, so the fork copies no gRPC threads.
  Server server(agni, mount);
  // The token goes on every replayed call. The server does not check it yet (agni issue 1627), so
  // this proves only that a call carrying one still answers as recorded.
  agni::Client client(server.addr(), "wire-fixtures");
  auto deadline = std::chrono::system_clock::now() + std::chrono::seconds(60);
  while (!client.channel()->WaitForConnected(std::chrono::system_clock::now() + std::chrono::milliseconds(500))) {
    if (!server.Running() || std::chrono::system_clock::now() > deadline) {
      Fatal("agni serve did not answer at " + server.addr() + ":\n" + server.Log());
    }
  }
  if (auto st = client.Connect(); !st.ok()) Fatal("Connect: " + st.error_message());
  if (!client.HasFeature("grpc")) Fatal("the server does not list the grpc feature");

  int replayed = 0, answers = 0, errors = 0, failed = 0;
  for (const auto& f : file.fixtures()) {
    std::string msg = Replay(client, f);
    ++replayed;
    f.has_error() ? ++errors : ++answers;
    if (!msg.empty()) {
      ++failed;
      std::cerr << "FAIL " << f.name() << ": " << msg << "\n";
    } else {
      std::cout << "ok   " << f.name() << "\n";
    }
  }
  server.Stop();

  // A file that lost its fixtures, or its errors, would pass a loop over nothing.
  if (replayed != file.fixtures_size() || answers == 0 || errors == 0) {
    Fatal("replayed " + std::to_string(replayed) + " of " + std::to_string(file.fixtures_size()) + " fixtures, " +
          std::to_string(answers) + " answers and " + std::to_string(errors) + " errors; want both kinds");
  }
  std::cout << "replayed " << replayed << " fixtures, " << answers << " answers and " << errors << " errors, "
            << failed << " differ\n";
  return failed == 0 ? 0 : 1;
}
