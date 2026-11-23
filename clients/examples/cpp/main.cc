// The C++ client's example for `make exercise-clients` (agni issue 1638). It does what
// clients/examples/go does, through agni's C++ client, against a shared `agni serve`: it checks the
// server, ingests a design, asks it three questions, and prints one line of JSON summing up the
// answers. docsite/content/build/client-contract.md names the line's fields.
//
//   clients/examples/cpp/run -server http://127.0.0.1:8080 -design mount://probe/boards/jetson-cut
#include <algorithm>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <iostream>
#include <stdexcept>
#include <string>
#include <vector>

#include "agni/client.h"
#include "agni/v1/webapi/checks.grpc.pb.h"
#include "agni/v1/webapi/design_view.grpc.pb.h"
#include "agni/v1/webapi/query.grpc.pb.h"
#include "agni/v1/webapi/snapshot.grpc.pb.h"

namespace webapi = agni::v1::webapi;

namespace {

// The query every example asks, so their answers compare. The check runs every rule.
constexpr char kResistorsOf[] = R"(component.class(?c, "resistor") => ?c)";

// Small, so the cut's 51 KB board comes in several pages and every example pages.
constexpr int64_t kBoardPageBytes = 16 << 10;

std::string Quote(const std::string& s) {
  std::string out = "\"";
  for (unsigned char c : s) {
    if (c == '"' || c == '\\') {
      out += '\\';
      out += static_cast<char>(c);
    } else if (c < 0x20) {
      char buf[8];
      std::snprintf(buf, sizeof buf, "\\u%04x", c);
      out += buf;
    } else {
      out += static_cast<char>(c);
    }
  }
  return out + "\"";
}

[[noreturn]] void Fail(const std::string& call, const grpc::Status& st) {
  std::cerr << "calling " << call << ": " << agni::CodeName(st.error_code()) << ": " << st.error_message() << "\n";
  std::exit(1);
}

}  // namespace

int main(int argc, char** argv) {
  std::string server = "http://127.0.0.1:8080", design = "mount://probe/boards/jetson-cut";
  for (int i = 1; i + 1 < argc; i += 2) {
    std::string flag = argv[i];
    if (flag == "-server" || flag == "--server") {
      server = argv[i + 1];
    } else if (flag == "-design" || flag == "--design") {
      design = argv[i + 1];
    } else {
      std::cerr << "usage: " << argv[0] << " -server <url> -design <mount:// URI>\n";
      return 2;
    }
  }

  try {
    (void)agni::GrpcTarget(server);
  } catch (const std::invalid_argument& e) {
    std::cerr << e.what() << "\n";
    return 2;
  }
  // A shared server asks every call for a token (agni issue 1627), passed in AGNI_TOKEN, and its
  // certificate may come from a team's own CA (agni issue 1765), passed in AGNI_CA.
  const char* token = std::getenv("AGNI_TOKEN");
  const char* ca = std::getenv("AGNI_CA");
  agni::Client client(server, token ? token : "", ca ? ca : "");
  webapi::ServerInfo info;
  // The contract's first rule: Connect refuses a server that speaks another package.
  if (auto st = client.Connect(&info); !st.ok()) Fail("GetServerInfo", st);

  auto snapshots = client.Stub<webapi::SnapshotService>();
  webapi::IngestRequest ingest;
  ingest.mutable_snapshot()->set_uri(design);
  webapi::Snapshot snap;
  {
    grpc::ClientContext ctx;
    if (auto st = snapshots->Ingest(&ctx, ingest, &snap); !st.ok()) Fail("Ingest", st);
  }
  const std::string name = snap.name();

  webapi::GetSummaryResponse d;
  webapi::CheckDesignResponse c;
  webapi::RunQueryResponse q;
  agni::v1::geom::BoardGeometry board;
  grpc::Status st;
  const char* failed = nullptr;
  {
    webapi::GetSummaryRequest req;
    req.set_snapshot(name);
    grpc::ClientContext ctx;
    st = client.Stub<webapi::DesignViewService>()->GetSummary(&ctx, req, &d);
    if (!st.ok()) failed = "GetSummary";
  }
  if (!failed) {
    webapi::CheckDesignRequest req;
    req.set_snapshot(name);
    grpc::ClientContext ctx;
    st = client.Stub<webapi::CheckService>()->CheckDesign(&ctx, req, &c);
    if (!st.ok()) failed = "CheckDesign";
  }
  if (!failed) {
    webapi::RunQueryRequest req;
    req.set_snapshot(name);
    req.set_query(kResistorsOf);
    grpc::ClientContext ctx;
    st = client.Stub<webapi::QueryService>()->RunQuery(&ctx, req, &q);
    if (!st.ok()) failed = "RunQuery";
  }
  // Read every page of the board and merge them, as docsite/content/build/board-geometry.md says a
  // client does. MergeFrom appends repeated fields.
  for (std::string token; !failed;) {
    webapi::GetBoardGeometryRequest req;
    req.set_snapshot(name);
    req.set_max_bytes(kBoardPageBytes);
    req.set_page_token(token);
    webapi::GetBoardGeometryResponse page;
    grpc::ClientContext ctx;
    st = client.Stub<webapi::DesignViewService>()->GetBoardGeometry(&ctx, req, &page);
    if (!st.ok()) {
      failed = "GetBoardGeometry";
      break;
    }
    board.MergeFrom(page.board());
    token = page.next_page_token();
    if (token.empty()) break;
  }
  int tracks = 0;
  for (const auto& n : board.nets()) tracks += n.segments_size();
  {
    webapi::DeleteSnapshotRequest req;
    req.set_name(name);
    google::protobuf::Empty empty;
    grpc::ClientContext ctx;
    (void)snapshots->DeleteSnapshot(&ctx, req, &empty);
  }
  if (failed) Fail(failed, st);

  std::vector<std::string> features(info.features().begin(), info.features().end());
  std::sort(features.begin(), features.end());
  std::string joined;
  for (const auto& f : features) joined += (joined.empty() ? "" : ",") + Quote(f);
  std::cout << "{\"version\":" << Quote(info.version()) << ",\"api\":" << Quote(info.api()) << ",\"features\":["
            << joined << "],\"components\":" << d.component_count() << ",\"nets\":" << d.net_count()
            << ",\"sheets\":" << d.sheets_size() << ",\"findings\":" << c.findings_size()
            << ",\"query_rows\":" << q.rows_size() << ",\"board_layers\":" << board.layers_size()
            << ",\"board_parts\":" << board.placements_size() << ",\"board_tracks\":" << tracks << "}"
            << std::endl;
  return 0;
}
