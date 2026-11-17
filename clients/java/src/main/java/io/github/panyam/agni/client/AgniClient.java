package io.github.panyam.agni.client;

import io.github.panyam.agni.v1.webapi.CheckServiceGrpc;
import io.github.panyam.agni.v1.webapi.DesignViewServiceGrpc;
import io.github.panyam.agni.v1.webapi.GetServerInfoRequest;
import io.github.panyam.agni.v1.webapi.QueryServiceGrpc;
import io.github.panyam.agni.v1.webapi.ServerInfo;
import io.github.panyam.agni.v1.webapi.ServerServiceGrpc;
import io.github.panyam.agni.v1.webapi.SnapshotServiceGrpc;
import io.grpc.CallOptions;
import io.grpc.Channel;
import io.grpc.ChannelCredentials;
import io.grpc.ClientCall;
import io.grpc.ClientInterceptor;
import io.grpc.ClientInterceptors;
import io.grpc.ForwardingClientCall;
import io.grpc.Grpc;
import io.grpc.InsecureChannelCredentials;
import io.grpc.ManagedChannel;
import io.grpc.Metadata;
import io.grpc.MethodDescriptor;
import io.grpc.TlsChannelCredentials;
import java.net.URI;
import java.util.concurrent.TimeUnit;

/**
 * A client of a shared {@code agni serve}, over gRPC (agni issue 1632). It holds one channel to the
 * server and sends the bearer token, when it has one, on every call.
 *
 * <p>Call {@link #connect()} first. It asks the server what it is and refuses a server that speaks
 * another package. Then ask questions through the stubs: {@link #snapshots()} ingests a design,
 * and the other stubs name the snapshot it returns. A service without a method here takes {@link
 * #channel()}, as in {@code DiffServiceGrpc.newBlockingStub(client.channel())}. {@link AgniErrors}
 * reads the details a failed call carries.
 *
 * <p>docsite/content/build/client-contract.md says what every language's client does.
 */
public final class AgniClient implements AutoCloseable {
  /** The proto package this client was generated from, and the only {@code api} it accepts. */
  public static final String API = "agni.v1";

  private static final Metadata.Key<String> AUTHORIZATION =
      Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER);

  private final ManagedChannel managed;
  private final Channel channel;

  private AgniClient(ManagedChannel managed, Channel channel) {
    this.managed = managed;
    this.channel = channel;
  }

  /** A client of the server at {@code address}, with no bearer token. */
  public static AgniClient create(String address) {
    return create(address, null);
  }

  /**
   * A client of the server at {@code address}. The address is {@code http://host:port} or {@code
   * host:port} for cleartext HTTP/2, or {@code https://host:port} for TLS. A non-empty {@code token}
   * goes out as {@code authorization: Bearer <token>} on every call. The server does not check it
   * yet (agni issue 1627).
   */
  public static AgniClient create(String address, String token) {
    Target t = Target.parse(address);
    ChannelCredentials creds =
        t.tls ? TlsChannelCredentials.create() : InsecureChannelCredentials.create();
    ManagedChannel managed = Grpc.newChannelBuilderForAddress(t.host, t.port, creds).build();
    Channel channel = managed;
    if (token != null && !token.isEmpty()) {
      channel = ClientInterceptors.intercept(managed, new Bearer(token));
    }
    return new AgniClient(managed, channel);
  }

  /**
   * Asks the server what it is, and returns its answer when it speaks {@link #API}.
   *
   * @throws IncompatibleServerException when the server speaks another package
   * @throws io.grpc.StatusRuntimeException when the call fails
   */
  public ServerInfo connect() {
    ServerInfo info = server().getServerInfo(GetServerInfoRequest.getDefaultInstance());
    if (!API.equals(info.getApi())) {
      throw new IncompatibleServerException(info);
    }
    return info;
  }

  /** The channel every stub here uses, with the bearer token attached. */
  public Channel channel() {
    return channel;
  }

  public ServerServiceGrpc.ServerServiceBlockingStub server() {
    return ServerServiceGrpc.newBlockingStub(channel);
  }

  public SnapshotServiceGrpc.SnapshotServiceBlockingStub snapshots() {
    return SnapshotServiceGrpc.newBlockingStub(channel);
  }

  public DesignViewServiceGrpc.DesignViewServiceBlockingStub designViews() {
    return DesignViewServiceGrpc.newBlockingStub(channel);
  }

  public CheckServiceGrpc.CheckServiceBlockingStub checks() {
    return CheckServiceGrpc.newBlockingStub(channel);
  }

  public QueryServiceGrpc.QueryServiceBlockingStub queries() {
    return QueryServiceGrpc.newBlockingStub(channel);
  }

  /** Closes the channel, waiting up to five seconds for calls in flight. */
  @Override
  public void close() {
    managed.shutdown();
    try {
      if (!managed.awaitTermination(5, TimeUnit.SECONDS)) {
        managed.shutdownNow();
      }
    } catch (InterruptedException e) {
      managed.shutdownNow();
      Thread.currentThread().interrupt();
    }
  }

  /** The host, port and transport an address names. */
  private record Target(String host, int port, boolean tls) {
    static Target parse(String address) {
      String a = address.trim();
      if (!a.contains("://")) {
        a = "http://" + a;
      }
      URI u = URI.create(a);
      boolean tls;
      switch (u.getScheme()) {
        case "http" -> tls = false;
        case "https" -> tls = true;
        default ->
            throw new IllegalArgumentException(
                "agni: the address " + address + " is not http://host:port or https://host:port");
      }
      if (u.getHost() == null) {
        throw new IllegalArgumentException("agni: the address " + address + " names no host");
      }
      int port = u.getPort() >= 0 ? u.getPort() : (tls ? 443 : 80);
      return new Target(u.getHost(), port, tls);
    }
  }

  /** Bearer adds the authorization metadata to every call. */
  private static final class Bearer implements ClientInterceptor {
    private final String value;

    Bearer(String token) {
      this.value = "Bearer " + token;
    }

    @Override
    public <Q, R> ClientCall<Q, R> interceptCall(
        MethodDescriptor<Q, R> method, CallOptions options, Channel next) {
      return new ForwardingClientCall.SimpleForwardingClientCall<>(next.newCall(method, options)) {
        @Override
        public void start(Listener<R> listener, Metadata headers) {
          headers.put(AUTHORIZATION, value);
          super.start(listener, headers);
        }
      };
    }
  }
}
