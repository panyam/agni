package io.github.panyam.agni.client;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.google.protobuf.Any;
import com.google.protobuf.Descriptors;
import com.google.protobuf.DynamicMessage;
import com.google.protobuf.Message;
import com.google.protobuf.util.JsonFormat;
import io.github.panyam.agni.v1.webapi.BlobsMissing;
import io.github.panyam.agni.v1.webapi.SnapshotUnavailable;
import io.github.panyam.agni.v1.webapi.TextPosition;
import io.grpc.CallOptions;
import io.grpc.MethodDescriptor;
import io.grpc.ServiceDescriptor;
import io.grpc.StatusRuntimeException;
import io.grpc.protobuf.ProtoServiceDescriptorSupplier;
import io.grpc.stub.ClientCalls;
import java.io.IOException;
import java.net.ServerSocket;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.TimeUnit;
import java.util.function.Function;
import java.util.stream.Stream;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

/**
 * Replays clients/wirefixtures/fixtures.json through this client against a real {@code agni serve}
 * (agni issues 1632 and 1633). The server mounts examples/cross-probe-project as {@code probe}, and
 * runs from an empty working directory so it serves the API alone. Each call's answer is compared
 * with the recording, leaving out the fields the fixture lists under {@code ignore}. A failed call
 * compares its code and details, never its message. cmd/agni/wirefixtures_test.go is the Go
 * replayer this one follows.
 *
 * <p>AGNI_BIN names a built agni to serve. Without it the test runs {@code go build ./cmd/agni}
 * from the repo root.
 */
class WireFixturesTest {
  private static final Duration START_TIMEOUT = Duration.ofSeconds(90);

  private static Path repoRoot;
  private static Path work;
  private static Process server;
  private static Path serverLog;
  private static AgniClient client;

  @BeforeAll
  static void startServer() throws Exception {
    repoRoot = Path.of(System.getProperty("agni.repoRoot", "../..")).toAbsolutePath().normalize();
    work = Files.createTempDirectory("agni-java-wire-");
    Path agni = agniBinary();
    Path serveDir = Files.createDirectory(work.resolve("serve"));
    serverLog = work.resolve("serve.log");
    int port = freePort();
    Path project = repoRoot.resolve("examples/cross-probe-project");
    server =
        new ProcessBuilder(
                agni.toString(),
                "serve",
                "--addr",
                "127.0.0.1:" + port,
                "--mount",
                "probe=" + project)
            .directory(serveDir.toFile())
            .redirectErrorStream(true)
            .redirectOutput(serverLog.toFile())
            .start();
    String url = "http://127.0.0.1:" + port;
    awaitHealthy(url);
    client = AgniClient.create(url, "wire-fixtures");
  }

  @AfterAll
  static void stopServer() throws Exception {
    if (client != null) {
      client.close();
    }
    if (server != null) {
      server.destroy();
      if (!server.waitFor(10, TimeUnit.SECONDS)) {
        server.destroyForcibly().waitFor(10, TimeUnit.SECONDS);
      }
    }
    if (work != null) {
      try (Stream<Path> s = Files.walk(work)) {
        s.sorted(Comparator.reverseOrder()).forEach(p -> p.toFile().delete());
      }
    }
  }

  @Test
  void replaysEveryFixture() throws Exception {
    Path file = repoRoot.resolve("clients/wirefixtures/fixtures.json");
    JsonArray fixtures =
        JsonParser.parseString(Files.readString(file)).getAsJsonObject().getAsJsonArray("fixtures");
    // The loop below compares nothing on an empty file, so an empty or misread file fails here.
    assertTrue(fixtures.size() > 0, file + " holds no fixtures");

    List<String> problems = new ArrayList<>();
    int answers = 0;
    int errors = 0;
    for (JsonElement e : fixtures) {
      JsonObject f = e.getAsJsonObject();
      String problem = replay(f);
      if (problem != null) {
        problems.add(f.get("name").getAsString() + ": " + problem);
      }
      if (f.has("error")) {
        errors++;
      } else {
        answers++;
      }
    }
    assertEquals(fixtures.size(), answers + errors);
    // The fixtures hold calls that answer and calls that fail, so both comparisons run.
    assertTrue(answers > 0, "no fixture records an answer");
    assertTrue(errors > 0, "no fixture records an error");
    assertTrue(problems.isEmpty(), String.join("\n", problems));
  }

  /** Makes one fixture's call and says how its outcome differs from the recording, or null. */
  @SuppressWarnings("unchecked")
  private static String replay(JsonObject f) throws Exception {
    String service = f.get("service").getAsString();
    String method = f.get("method").getAsString();
    MethodDescriptor<Message, Message> md =
        (MethodDescriptor<Message, Message>) method(service, method);
    Message request = parse(f.get("request"), prototype(md.getRequestMarshaller()).newBuilderForType());
    Message got;
    try {
      got =
          ClientCalls.blockingUnaryCall(
              client.channel(), md, CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.MINUTES), request);
    } catch (StatusRuntimeException err) {
      if (!f.has("error")) {
        return "failed with " + AgniErrors.code(err).orElse("?") + ", want an answer: " + err;
      }
      return errorDiff(f.getAsJsonObject("error"), err);
    }
    if (f.has("error")) {
      return "answered, want the error " + f.getAsJsonObject("error").get("code").getAsString();
    }
    List<String> ignore = new ArrayList<>();
    if (f.has("ignore")) {
      f.getAsJsonArray("ignore").forEach(p -> ignore.add(p.getAsString()));
    }
    Message.Builder want = got.newBuilderForType();
    parse(f.get("response"), want);
    Message.Builder have = got.toBuilder();
    for (String p : ignore) {
      clearPath(want, p);
      clearPath(have, p);
    }
    if (!have.build().equals(want.build())) {
      return "answered " + printer().print(got) + ", want " + f.get("response");
    }
    return null;
  }

  /** Compares a failed call's code and details with a recorded error. */
  private static String errorDiff(JsonObject want, StatusRuntimeException err) throws Exception {
    String code = AgniErrors.code(err).orElse("?");
    if (!code.equals(want.get("code").getAsString())) {
      return "failed with " + code + ", want " + want.get("code").getAsString();
    }
    List<Any> details = AgniErrors.details(err);
    JsonArray wantDetails = want.has("details") ? want.getAsJsonArray("details") : new JsonArray();
    if (details.size() != wantDetails.size()) {
      return "carries " + details.size() + " details, want " + wantDetails.size();
    }
    for (int i = 0; i < details.size(); i++) {
      JsonObject wd = wantDetails.get(i).getAsJsonObject();
      String type = wd.get("type").getAsString();
      Any d = details.get(i);
      String gotType = d.getTypeUrl().substring(d.getTypeUrl().lastIndexOf('/') + 1);
      if (!gotType.equals(type)) {
        return "detail " + i + " is " + gotType + ", want " + type;
      }
      Descriptors.Descriptor desc = MESSAGES.get(type);
      if (desc == null) {
        return "detail " + i + " is " + type + ", and this client does not know that type";
      }
      Message w = parse(wd.get("value"), DynamicMessage.newBuilder(desc));
      Message g = DynamicMessage.parseFrom(desc, d.getValue());
      if (!g.equals(w)) {
        return "detail " + i + " is " + type + " " + printer().print(g) + ", want " + wd.get("value");
      }
      // The typed helper a caller reaches for decodes the same detail.
      Function<Throwable, Optional<? extends Message>> helper = HELPERS.get(type);
      if (helper != null) {
        Optional<? extends Message> typed = helper.apply(err);
        if (typed.isEmpty() || !typed.get().toByteString().equals(d.getValue())) {
          return "AgniErrors does not decode detail " + i + ", " + type;
        }
      }
    }
    return null;
  }

  private static final Map<String, Function<Throwable, Optional<? extends Message>>> HELPERS =
      Map.of(
          TextPosition.getDescriptor().getFullName(), AgniErrors::textPosition,
          SnapshotUnavailable.getDescriptor().getFullName(), AgniErrors::snapshotUnavailable,
          BlobsMissing.getDescriptor().getFullName(), AgniErrors::blobsMissing);

  /** Clears a field named by its proto name, dotted through singular messages. */
  private static void clearPath(Message.Builder b, String path) {
    int dot = path.indexOf('.');
    String head = dot < 0 ? path : path.substring(0, dot);
    Descriptors.FieldDescriptor fd = b.getDescriptorForType().findFieldByName(head);
    if (fd == null) {
      return;
    }
    if (dot < 0) {
      b.clearField(fd);
      return;
    }
    if (fd.getJavaType() == Descriptors.FieldDescriptor.JavaType.MESSAGE
        && !fd.isRepeated()
        && b.hasField(fd)) {
      clearPath(b.getFieldBuilder(fd), path.substring(dot + 1));
    }
  }

  /** The rpc a fixture names, found through the generated {@code <Service>Grpc} class. */
  private static MethodDescriptor<?, ?> method(String service, String method) throws Exception {
    ServiceDescriptor sd = serviceDescriptor(service);
    String full = MethodDescriptor.generateFullMethodName(service, method);
    for (MethodDescriptor<?, ?> m : sd.getMethods()) {
      if (m.getFullMethodName().equals(full)) {
        return m;
      }
    }
    throw new AssertionError(service + " has no rpc " + method);
  }

  private static ServiceDescriptor serviceDescriptor(String service) throws Exception {
    Class<?> grpc = Class.forName("io.github.panyam." + service + "Grpc");
    ServiceDescriptor sd = (ServiceDescriptor) grpc.getMethod("getServiceDescriptor").invoke(null);
    // Index the messages this service's file reaches, so an error detail's type can be decoded.
    ProtoServiceDescriptorSupplier sup = (ProtoServiceDescriptorSupplier) sd.getSchemaDescriptor();
    walk(sup.getFileDescriptor(), MESSAGES, FILES_SEEN);
    return sd;
  }

  private static Message prototype(MethodDescriptor.Marshaller<?> m) {
    return (Message) ((MethodDescriptor.PrototypeMarshaller<?>) m).getMessagePrototype();
  }

  /** Every message type the services the fixtures call reach, by full name. */
  private static final Map<String, Descriptors.Descriptor> MESSAGES = new HashMap<>();

  private static final Map<String, Boolean> FILES_SEEN = new HashMap<>();

  private static void walk(
      Descriptors.FileDescriptor fd,
      Map<String, Descriptors.Descriptor> index,
      Map<String, Boolean> seen) {
    if (seen.putIfAbsent(fd.getName(), true) != null) {
      return;
    }
    for (Descriptors.Descriptor d : fd.getMessageTypes()) {
      addMessage(d, index);
    }
    for (Descriptors.FileDescriptor dep : fd.getDependencies()) {
      walk(dep, index, seen);
    }
  }

  private static void addMessage(Descriptors.Descriptor d, Map<String, Descriptors.Descriptor> index) {
    index.put(d.getFullName(), d);
    for (Descriptors.Descriptor n : d.getNestedTypes()) {
      addMessage(n, index);
    }
  }

  private static Message parse(JsonElement json, Message.Builder b) throws Exception {
    JsonFormat.parser().merge(json == null ? "{}" : json.toString(), b);
    return b.build();
  }

  private static JsonFormat.Printer printer() {
    return JsonFormat.printer().omittingInsignificantWhitespace();
  }

  /** A built agni: AGNI_BIN, or one `go build` writes from the repo root. */
  private static Path agniBinary() throws IOException, InterruptedException {
    String env = System.getenv("AGNI_BIN");
    if (env != null && !env.isEmpty()) {
      return Path.of(env).toAbsolutePath();
    }
    Path out = work.resolve("agni");
    Path log = work.resolve("build.log");
    Process build =
        new ProcessBuilder("go", "build", "-o", out.toString(), "./cmd/agni")
            .directory(repoRoot.toFile())
            .redirectErrorStream(true)
            .redirectOutput(log.toFile())
            .start();
    if (!build.waitFor(10, TimeUnit.MINUTES) || build.exitValue() != 0) {
      fail("go build ./cmd/agni failed:\n" + Files.readString(log, StandardCharsets.UTF_8));
    }
    return out;
  }

  private static int freePort() throws IOException {
    try (ServerSocket s = new ServerSocket(0)) {
      return s.getLocalPort();
    }
  }

  private static void awaitHealthy(String url) throws Exception {
    HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2)).build();
    HttpRequest req = HttpRequest.newBuilder(URI.create(url + "/healthz")).GET().build();
    long deadline = System.nanoTime() + START_TIMEOUT.toNanos();
    while (System.nanoTime() < deadline) {
      if (!server.isAlive()) {
        fail("agni serve exited with " + server.exitValue() + ":\n" + Files.readString(serverLog));
      }
      try {
        if (http.send(req, HttpResponse.BodyHandlers.discarding()).statusCode() == 200) {
          return;
        }
      } catch (IOException notYet) {
        // The server is still starting.
      }
      Thread.sleep(200);
    }
    fail("agni serve did not answer at " + url + ":\n" + Files.readString(serverLog));
  }
}
