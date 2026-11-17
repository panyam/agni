package io.github.panyam.agni.example;

import io.github.panyam.agni.client.AgniClient;
import io.github.panyam.agni.v1.geom.BoardGeometry;
import io.github.panyam.agni.v1.geom.NetCopper;
import io.github.panyam.agni.v1.webapi.CheckDesignRequest;
import io.github.panyam.agni.v1.webapi.CheckDesignResponse;
import io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest;
import io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest;
import io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse;
import io.github.panyam.agni.v1.webapi.GetSummaryResponse;
import io.github.panyam.agni.v1.webapi.GetSummaryRequest;
import io.github.panyam.agni.v1.webapi.IngestRequest;
import io.github.panyam.agni.v1.webapi.RunQueryRequest;
import io.github.panyam.agni.v1.webapi.RunQueryResponse;
import io.github.panyam.agni.v1.webapi.ServerInfo;
import io.github.panyam.agni.v1.webapi.Snapshot;
import io.grpc.StatusRuntimeException;
import java.util.ArrayList;
import java.util.List;
import java.util.function.Supplier;

/**
 * The Java example for {@code make exercise-clients} (agni issue 1632). It does what
 * clients/examples/go does, through the Java client, against a shared {@code agni serve}. It checks
 * the server, ingests a design, asks it three questions, and prints one line of JSON summing up the
 * answers. docsite/content/build/client-contract.md names the line's fields.
 *
 * <pre>clients/examples/java/run -server http://127.0.0.1:8080 -design mount://probe/designs/jetson-cut</pre>
 */
public final class Main {
  /** The query every example asks, so their answers compare. The check runs every rule. */
  private static final String RESISTORS_OF = "component.class(?c, \"resistor\") => ?c";

  /** Small, so the cut's 51 KB board comes in several pages and every example pages. */
  private static final long BOARD_PAGE_BYTES = 16 << 10;

  /**
   * Reads every page of the snapshot's board and merges them, as
   * docsite/content/build/board-geometry.md says a client does. mergeFrom appends repeated fields.
   */
  private static BoardGeometry boardGeometry(AgniClient client, String snapshot) {
    BoardGeometry.Builder board = BoardGeometry.newBuilder();
    String token = "";
    do {
      String pageToken = token;
      GetBoardGeometryResponse page =
          call(
              "GetBoardGeometry",
              () ->
                  client
                      .designViews()
                      .getBoardGeometry(
                          GetBoardGeometryRequest.newBuilder()
                              .setSnapshot(snapshot)
                              .setMaxBytes(BOARD_PAGE_BYTES)
                              .setPageToken(pageToken)
                              .build()));
      board.mergeFrom(page.getBoard());
      token = page.getNextPageToken();
    } while (!token.isEmpty());
    return board.build();
  }

  public static void main(String[] args) {
    String server = "http://127.0.0.1:8080";
    String design = "mount://probe/designs/jetson-cut";
    for (int i = 0; i < args.length; i++) {
      String flag = args[i];
      String a = flag.replaceFirst("^--?", "");
      String value = null;
      int eq = a.indexOf('=');
      if (eq >= 0) {
        value = a.substring(eq + 1);
        a = a.substring(0, eq);
      } else if (i + 1 < args.length) {
        value = args[++i];
      }
      switch (a) {
        case "server" -> server = value;
        case "design" -> design = value;
        default -> usage("unknown flag " + flag);
      }
      if (value == null) {
        usage("flag -" + a + " needs a value");
      }
    }
    try (AgniClient client = AgniClient.create(server)) {
      System.out.println(run(client, design));
    } catch (RuntimeException e) {
      System.err.println(e.getMessage());
      System.exit(1);
    }
  }

  private static String run(AgniClient client, String design) {
    // The contract's first rule: refuse a server that speaks another package. connect() does it.
    ServerInfo info = call("GetServerInfo", client::connect);
    Snapshot snap =
        call(
            "Ingest",
            () ->
                client
                    .snapshots()
                    .ingest(
                        IngestRequest.newBuilder()
                            .setSnapshot(Snapshot.newBuilder().setUri(design))
                            .build()));
    String name = snap.getName();
    try {
      GetSummaryResponse d =
          call(
              "GetSummary",
              () -> client.designViews().getSummary(GetSummaryRequest.newBuilder().setSnapshot(name).build()));
      CheckDesignResponse c =
          call(
              "CheckDesign",
              () -> client.checks().checkDesign(CheckDesignRequest.newBuilder().setSnapshot(name).build()));
      RunQueryResponse q =
          call(
              "RunQuery",
              () ->
                  client
                      .queries()
                      .runQuery(
                          RunQueryRequest.newBuilder().setSnapshot(name).setQuery(RESISTORS_OF).build()));
      BoardGeometry board = boardGeometry(client, name);
      int tracks = 0;
      for (NetCopper n : board.getNetsList()) {
        tracks += n.getSegmentsCount();
      }
      List<String> features = new ArrayList<>(info.getFeaturesList());
      features.sort(null);
      List<String> quoted = new ArrayList<>();
      for (String f : features) {
        quoted.add(quote(f));
      }
      return "{\"version\":"
          + quote(info.getVersion())
          + ",\"api\":"
          + quote(info.getApi())
          + ",\"features\":["
          + String.join(",", quoted)
          + "],\"components\":"
          + d.getComponentCount()
          + ",\"nets\":"
          + d.getNetCount()
          + ",\"sheets\":"
          + d.getSheetsCount()
          + ",\"findings\":"
          + c.getFindingsCount()
          + ",\"query_rows\":"
          + q.getRowsCount()
          + ",\"board_layers\":"
          + board.getLayersCount()
          + ",\"board_parts\":"
          + board.getPlacementsCount()
          + ",\"board_tracks\":"
          + tracks
          + "}";
    } finally {
      client.snapshots().deleteSnapshot(DeleteSnapshotRequest.newBuilder().setName(name).build());
    }
  }

  /** Makes one call, and names the rpc when it fails. */
  private static <T> T call(String rpc, Supplier<T> c) {
    try {
      return c.get();
    } catch (StatusRuntimeException e) {
      throw new IllegalStateException("calling " + rpc + ": " + e.getMessage(), e);
    }
  }

  /** A JSON string literal. */
  private static String quote(String s) {
    StringBuilder b = new StringBuilder("\"");
    for (char ch : s.toCharArray()) {
      switch (ch) {
        case '"' -> b.append("\\\"");
        case '\\' -> b.append("\\\\");
        case '\n' -> b.append("\\n");
        case '\r' -> b.append("\\r");
        case '\t' -> b.append("\\t");
        default -> {
          if (ch < 0x20) {
            b.append(String.format("\\u%04x", (int) ch));
          } else {
            b.append(ch);
          }
        }
      }
    }
    return b.append('"').toString();
  }

  private static void usage(String problem) {
    System.err.println(problem);
    System.err.println("usage: run -server <url> -design <mount:// URI>");
    System.exit(2);
  }
}
