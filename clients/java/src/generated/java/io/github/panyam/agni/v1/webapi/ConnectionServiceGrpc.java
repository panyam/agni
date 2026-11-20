package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * This file defines ConnectionService, which keeps the sign-ins a server reads private sources
 * through (agni issue 1724). A mount names a connection, and the server reads the mount's source
 * with that connection's token. Several mounts can share one, such as one GitHub App installation
 * over many repos. The records come from goapplib storage's connections package.
 * A server connection never holds a secret. Its key_source and token_source name where the secret
 * is, "env:NAME" or "file:/abs/path", and the server reads it from there when it needs a token. So
 * no secret is written to --store or sent back over this API.
 * A person's connection belongs to the caller who made it (agni issue 1728). It holds that
 * person's own GitHub token, sealed in --store under the server's --connections-key, and nobody
 * else's request can use it. A design linked from a private repo opens only for a caller whose own
 * connection can read the repo.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class ConnectionServiceGrpc {

  private ConnectionServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.ConnectionService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateConnectionRequest,
      io.github.panyam.agni.v1.webapi.Connection> getCreateConnectionMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CreateConnection",
      requestType = io.github.panyam.agni.v1.webapi.CreateConnectionRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Connection.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateConnectionRequest,
      io.github.panyam.agni.v1.webapi.Connection> getCreateConnectionMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateConnectionRequest, io.github.panyam.agni.v1.webapi.Connection> getCreateConnectionMethod;
    if ((getCreateConnectionMethod = ConnectionServiceGrpc.getCreateConnectionMethod) == null) {
      synchronized (ConnectionServiceGrpc.class) {
        if ((getCreateConnectionMethod = ConnectionServiceGrpc.getCreateConnectionMethod) == null) {
          ConnectionServiceGrpc.getCreateConnectionMethod = getCreateConnectionMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.CreateConnectionRequest, io.github.panyam.agni.v1.webapi.Connection>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CreateConnection"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.CreateConnectionRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Connection.getDefaultInstance()))
              .setSchemaDescriptor(new ConnectionServiceMethodDescriptorSupplier("CreateConnection"))
              .build();
        }
      }
    }
    return getCreateConnectionMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetConnectionRequest,
      io.github.panyam.agni.v1.webapi.Connection> getGetConnectionMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetConnection",
      requestType = io.github.panyam.agni.v1.webapi.GetConnectionRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Connection.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetConnectionRequest,
      io.github.panyam.agni.v1.webapi.Connection> getGetConnectionMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetConnectionRequest, io.github.panyam.agni.v1.webapi.Connection> getGetConnectionMethod;
    if ((getGetConnectionMethod = ConnectionServiceGrpc.getGetConnectionMethod) == null) {
      synchronized (ConnectionServiceGrpc.class) {
        if ((getGetConnectionMethod = ConnectionServiceGrpc.getGetConnectionMethod) == null) {
          ConnectionServiceGrpc.getGetConnectionMethod = getGetConnectionMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetConnectionRequest, io.github.panyam.agni.v1.webapi.Connection>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetConnection"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetConnectionRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Connection.getDefaultInstance()))
              .setSchemaDescriptor(new ConnectionServiceMethodDescriptorSupplier("GetConnection"))
              .build();
        }
      }
    }
    return getGetConnectionMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListConnectionsRequest,
      io.github.panyam.agni.v1.webapi.ListConnectionsResponse> getListConnectionsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListConnections",
      requestType = io.github.panyam.agni.v1.webapi.ListConnectionsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListConnectionsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListConnectionsRequest,
      io.github.panyam.agni.v1.webapi.ListConnectionsResponse> getListConnectionsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListConnectionsRequest, io.github.panyam.agni.v1.webapi.ListConnectionsResponse> getListConnectionsMethod;
    if ((getListConnectionsMethod = ConnectionServiceGrpc.getListConnectionsMethod) == null) {
      synchronized (ConnectionServiceGrpc.class) {
        if ((getListConnectionsMethod = ConnectionServiceGrpc.getListConnectionsMethod) == null) {
          ConnectionServiceGrpc.getListConnectionsMethod = getListConnectionsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListConnectionsRequest, io.github.panyam.agni.v1.webapi.ListConnectionsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListConnections"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListConnectionsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListConnectionsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ConnectionServiceMethodDescriptorSupplier("ListConnections"))
              .build();
        }
      }
    }
    return getListConnectionsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteConnectionRequest,
      com.google.protobuf.Empty> getDeleteConnectionMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "DeleteConnection",
      requestType = io.github.panyam.agni.v1.webapi.DeleteConnectionRequest.class,
      responseType = com.google.protobuf.Empty.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteConnectionRequest,
      com.google.protobuf.Empty> getDeleteConnectionMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteConnectionRequest, com.google.protobuf.Empty> getDeleteConnectionMethod;
    if ((getDeleteConnectionMethod = ConnectionServiceGrpc.getDeleteConnectionMethod) == null) {
      synchronized (ConnectionServiceGrpc.class) {
        if ((getDeleteConnectionMethod = ConnectionServiceGrpc.getDeleteConnectionMethod) == null) {
          ConnectionServiceGrpc.getDeleteConnectionMethod = getDeleteConnectionMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.DeleteConnectionRequest, com.google.protobuf.Empty>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "DeleteConnection"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.DeleteConnectionRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.google.protobuf.Empty.getDefaultInstance()))
              .setSchemaDescriptor(new ConnectionServiceMethodDescriptorSupplier("DeleteConnection"))
              .build();
        }
      }
    }
    return getDeleteConnectionMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static ConnectionServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ConnectionServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ConnectionServiceStub>() {
        @java.lang.Override
        public ConnectionServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ConnectionServiceStub(channel, callOptions);
        }
      };
    return ConnectionServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static ConnectionServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ConnectionServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ConnectionServiceBlockingV2Stub>() {
        @java.lang.Override
        public ConnectionServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ConnectionServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return ConnectionServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static ConnectionServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ConnectionServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ConnectionServiceBlockingStub>() {
        @java.lang.Override
        public ConnectionServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ConnectionServiceBlockingStub(channel, callOptions);
        }
      };
    return ConnectionServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static ConnectionServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ConnectionServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ConnectionServiceFutureStub>() {
        @java.lang.Override
        public ConnectionServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ConnectionServiceFutureStub(channel, callOptions);
        }
      };
    return ConnectionServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * This file defines ConnectionService, which keeps the sign-ins a server reads private sources
   * through (agni issue 1724). A mount names a connection, and the server reads the mount's source
   * with that connection's token. Several mounts can share one, such as one GitHub App installation
   * over many repos. The records come from goapplib storage's connections package.
   * A server connection never holds a secret. Its key_source and token_source name where the secret
   * is, "env:NAME" or "file:/abs/path", and the server reads it from there when it needs a token. So
   * no secret is written to --store or sent back over this API.
   * A person's connection belongs to the caller who made it (agni issue 1728). It holds that
   * person's own GitHub token, sealed in --store under the server's --connections-key, and nobody
   * else's request can use it. A design linked from a private repo opens only for a caller whose own
   * connection can read the repo.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * CreateConnection keeps a new connection. An id a connection already holds is ALREADY_EXISTS. A
     * secret field holding anything but a reference, or a kind's field left empty, is
     * INVALID_ARGUMENT. A server started without --store answers FAILED_PRECONDITION. A person's
     * connection needs a signed-in caller, and a server started with --connections-key.
     * </pre>
     */
    default void createConnection(io.github.panyam.agni.v1.webapi.CreateConnectionRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Connection> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCreateConnectionMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetConnection returns one connection by resource name. Another person's connection is
     * NOT_FOUND.
     * </pre>
     */
    default void getConnection(io.github.panyam.agni.v1.webapi.GetConnectionRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Connection> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetConnectionMethod(), responseObserver);
    }

    /**
     * <pre>
     * ListConnections returns the server's connections and the caller's own, ordered by name.
     * </pre>
     */
    default void listConnections(io.github.panyam.agni.v1.webapi.ListConnectionsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListConnectionsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListConnectionsMethod(), responseObserver);
    }

    /**
     * <pre>
     * DeleteConnection forgets a connection. One a mount still reads through is FAILED_PRECONDITION,
     * naming the mounts, until they are deleted. Deleting a person's connection deletes their sealed
     * token too. Another person's connection is NOT_FOUND.
     * </pre>
     */
    default void deleteConnection(io.github.panyam.agni.v1.webapi.DeleteConnectionRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeleteConnectionMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service ConnectionService.
   * <pre>
   * This file defines ConnectionService, which keeps the sign-ins a server reads private sources
   * through (agni issue 1724). A mount names a connection, and the server reads the mount's source
   * with that connection's token. Several mounts can share one, such as one GitHub App installation
   * over many repos. The records come from goapplib storage's connections package.
   * A server connection never holds a secret. Its key_source and token_source name where the secret
   * is, "env:NAME" or "file:/abs/path", and the server reads it from there when it needs a token. So
   * no secret is written to --store or sent back over this API.
   * A person's connection belongs to the caller who made it (agni issue 1728). It holds that
   * person's own GitHub token, sealed in --store under the server's --connections-key, and nobody
   * else's request can use it. A design linked from a private repo opens only for a caller whose own
   * connection can read the repo.
   * </pre>
   */
  public static abstract class ConnectionServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return ConnectionServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service ConnectionService.
   * <pre>
   * This file defines ConnectionService, which keeps the sign-ins a server reads private sources
   * through (agni issue 1724). A mount names a connection, and the server reads the mount's source
   * with that connection's token. Several mounts can share one, such as one GitHub App installation
   * over many repos. The records come from goapplib storage's connections package.
   * A server connection never holds a secret. Its key_source and token_source name where the secret
   * is, "env:NAME" or "file:/abs/path", and the server reads it from there when it needs a token. So
   * no secret is written to --store or sent back over this API.
   * A person's connection belongs to the caller who made it (agni issue 1728). It holds that
   * person's own GitHub token, sealed in --store under the server's --connections-key, and nobody
   * else's request can use it. A design linked from a private repo opens only for a caller whose own
   * connection can read the repo.
   * </pre>
   */
  public static final class ConnectionServiceStub
      extends io.grpc.stub.AbstractAsyncStub<ConnectionServiceStub> {
    private ConnectionServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ConnectionServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ConnectionServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * CreateConnection keeps a new connection. An id a connection already holds is ALREADY_EXISTS. A
     * secret field holding anything but a reference, or a kind's field left empty, is
     * INVALID_ARGUMENT. A server started without --store answers FAILED_PRECONDITION. A person's
     * connection needs a signed-in caller, and a server started with --connections-key.
     * </pre>
     */
    public void createConnection(io.github.panyam.agni.v1.webapi.CreateConnectionRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Connection> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCreateConnectionMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetConnection returns one connection by resource name. Another person's connection is
     * NOT_FOUND.
     * </pre>
     */
    public void getConnection(io.github.panyam.agni.v1.webapi.GetConnectionRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Connection> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetConnectionMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ListConnections returns the server's connections and the caller's own, ordered by name.
     * </pre>
     */
    public void listConnections(io.github.panyam.agni.v1.webapi.ListConnectionsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListConnectionsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListConnectionsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * DeleteConnection forgets a connection. One a mount still reads through is FAILED_PRECONDITION,
     * naming the mounts, until they are deleted. Deleting a person's connection deletes their sealed
     * token too. Another person's connection is NOT_FOUND.
     * </pre>
     */
    public void deleteConnection(io.github.panyam.agni.v1.webapi.DeleteConnectionRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeleteConnectionMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service ConnectionService.
   * <pre>
   * This file defines ConnectionService, which keeps the sign-ins a server reads private sources
   * through (agni issue 1724). A mount names a connection, and the server reads the mount's source
   * with that connection's token. Several mounts can share one, such as one GitHub App installation
   * over many repos. The records come from goapplib storage's connections package.
   * A server connection never holds a secret. Its key_source and token_source name where the secret
   * is, "env:NAME" or "file:/abs/path", and the server reads it from there when it needs a token. So
   * no secret is written to --store or sent back over this API.
   * A person's connection belongs to the caller who made it (agni issue 1728). It holds that
   * person's own GitHub token, sealed in --store under the server's --connections-key, and nobody
   * else's request can use it. A design linked from a private repo opens only for a caller whose own
   * connection can read the repo.
   * </pre>
   */
  public static final class ConnectionServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<ConnectionServiceBlockingV2Stub> {
    private ConnectionServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ConnectionServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ConnectionServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * CreateConnection keeps a new connection. An id a connection already holds is ALREADY_EXISTS. A
     * secret field holding anything but a reference, or a kind's field left empty, is
     * INVALID_ARGUMENT. A server started without --store answers FAILED_PRECONDITION. A person's
     * connection needs a signed-in caller, and a server started with --connections-key.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Connection createConnection(io.github.panyam.agni.v1.webapi.CreateConnectionRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getCreateConnectionMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetConnection returns one connection by resource name. Another person's connection is
     * NOT_FOUND.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Connection getConnection(io.github.panyam.agni.v1.webapi.GetConnectionRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetConnectionMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListConnections returns the server's connections and the caller's own, ordered by name.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListConnectionsResponse listConnections(io.github.panyam.agni.v1.webapi.ListConnectionsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListConnectionsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteConnection forgets a connection. One a mount still reads through is FAILED_PRECONDITION,
     * naming the mounts, until they are deleted. Deleting a person's connection deletes their sealed
     * token too. Another person's connection is NOT_FOUND.
     * </pre>
     */
    public com.google.protobuf.Empty deleteConnection(io.github.panyam.agni.v1.webapi.DeleteConnectionRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getDeleteConnectionMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service ConnectionService.
   * <pre>
   * This file defines ConnectionService, which keeps the sign-ins a server reads private sources
   * through (agni issue 1724). A mount names a connection, and the server reads the mount's source
   * with that connection's token. Several mounts can share one, such as one GitHub App installation
   * over many repos. The records come from goapplib storage's connections package.
   * A server connection never holds a secret. Its key_source and token_source name where the secret
   * is, "env:NAME" or "file:/abs/path", and the server reads it from there when it needs a token. So
   * no secret is written to --store or sent back over this API.
   * A person's connection belongs to the caller who made it (agni issue 1728). It holds that
   * person's own GitHub token, sealed in --store under the server's --connections-key, and nobody
   * else's request can use it. A design linked from a private repo opens only for a caller whose own
   * connection can read the repo.
   * </pre>
   */
  public static final class ConnectionServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<ConnectionServiceBlockingStub> {
    private ConnectionServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ConnectionServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ConnectionServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * CreateConnection keeps a new connection. An id a connection already holds is ALREADY_EXISTS. A
     * secret field holding anything but a reference, or a kind's field left empty, is
     * INVALID_ARGUMENT. A server started without --store answers FAILED_PRECONDITION. A person's
     * connection needs a signed-in caller, and a server started with --connections-key.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Connection createConnection(io.github.panyam.agni.v1.webapi.CreateConnectionRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCreateConnectionMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetConnection returns one connection by resource name. Another person's connection is
     * NOT_FOUND.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Connection getConnection(io.github.panyam.agni.v1.webapi.GetConnectionRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetConnectionMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListConnections returns the server's connections and the caller's own, ordered by name.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListConnectionsResponse listConnections(io.github.panyam.agni.v1.webapi.ListConnectionsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListConnectionsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteConnection forgets a connection. One a mount still reads through is FAILED_PRECONDITION,
     * naming the mounts, until they are deleted. Deleting a person's connection deletes their sealed
     * token too. Another person's connection is NOT_FOUND.
     * </pre>
     */
    public com.google.protobuf.Empty deleteConnection(io.github.panyam.agni.v1.webapi.DeleteConnectionRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeleteConnectionMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service ConnectionService.
   * <pre>
   * This file defines ConnectionService, which keeps the sign-ins a server reads private sources
   * through (agni issue 1724). A mount names a connection, and the server reads the mount's source
   * with that connection's token. Several mounts can share one, such as one GitHub App installation
   * over many repos. The records come from goapplib storage's connections package.
   * A server connection never holds a secret. Its key_source and token_source name where the secret
   * is, "env:NAME" or "file:/abs/path", and the server reads it from there when it needs a token. So
   * no secret is written to --store or sent back over this API.
   * A person's connection belongs to the caller who made it (agni issue 1728). It holds that
   * person's own GitHub token, sealed in --store under the server's --connections-key, and nobody
   * else's request can use it. A design linked from a private repo opens only for a caller whose own
   * connection can read the repo.
   * </pre>
   */
  public static final class ConnectionServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<ConnectionServiceFutureStub> {
    private ConnectionServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ConnectionServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ConnectionServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * CreateConnection keeps a new connection. An id a connection already holds is ALREADY_EXISTS. A
     * secret field holding anything but a reference, or a kind's field left empty, is
     * INVALID_ARGUMENT. A server started without --store answers FAILED_PRECONDITION. A person's
     * connection needs a signed-in caller, and a server started with --connections-key.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Connection> createConnection(
        io.github.panyam.agni.v1.webapi.CreateConnectionRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCreateConnectionMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetConnection returns one connection by resource name. Another person's connection is
     * NOT_FOUND.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Connection> getConnection(
        io.github.panyam.agni.v1.webapi.GetConnectionRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetConnectionMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ListConnections returns the server's connections and the caller's own, ordered by name.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListConnectionsResponse> listConnections(
        io.github.panyam.agni.v1.webapi.ListConnectionsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListConnectionsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * DeleteConnection forgets a connection. One a mount still reads through is FAILED_PRECONDITION,
     * naming the mounts, until they are deleted. Deleting a person's connection deletes their sealed
     * token too. Another person's connection is NOT_FOUND.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.google.protobuf.Empty> deleteConnection(
        io.github.panyam.agni.v1.webapi.DeleteConnectionRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeleteConnectionMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_CREATE_CONNECTION = 0;
  private static final int METHODID_GET_CONNECTION = 1;
  private static final int METHODID_LIST_CONNECTIONS = 2;
  private static final int METHODID_DELETE_CONNECTION = 3;

  private static final class MethodHandlers<Req, Resp> implements
      io.grpc.stub.ServerCalls.UnaryMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ServerStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ClientStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.BidiStreamingMethod<Req, Resp> {
    private final AsyncService serviceImpl;
    private final int methodId;

    MethodHandlers(AsyncService serviceImpl, int methodId) {
      this.serviceImpl = serviceImpl;
      this.methodId = methodId;
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public void invoke(Req request, io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        case METHODID_CREATE_CONNECTION:
          serviceImpl.createConnection((io.github.panyam.agni.v1.webapi.CreateConnectionRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Connection>) responseObserver);
          break;
        case METHODID_GET_CONNECTION:
          serviceImpl.getConnection((io.github.panyam.agni.v1.webapi.GetConnectionRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Connection>) responseObserver);
          break;
        case METHODID_LIST_CONNECTIONS:
          serviceImpl.listConnections((io.github.panyam.agni.v1.webapi.ListConnectionsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListConnectionsResponse>) responseObserver);
          break;
        case METHODID_DELETE_CONNECTION:
          serviceImpl.deleteConnection((io.github.panyam.agni.v1.webapi.DeleteConnectionRequest) request,
              (io.grpc.stub.StreamObserver<com.google.protobuf.Empty>) responseObserver);
          break;
        default:
          throw new AssertionError();
      }
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public io.grpc.stub.StreamObserver<Req> invoke(
        io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        default:
          throw new AssertionError();
      }
    }
  }

  public static final io.grpc.ServerServiceDefinition bindService(AsyncService service) {
    return io.grpc.ServerServiceDefinition.builder(getServiceDescriptor())
        .addMethod(
          getCreateConnectionMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.CreateConnectionRequest,
              io.github.panyam.agni.v1.webapi.Connection>(
                service, METHODID_CREATE_CONNECTION)))
        .addMethod(
          getGetConnectionMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetConnectionRequest,
              io.github.panyam.agni.v1.webapi.Connection>(
                service, METHODID_GET_CONNECTION)))
        .addMethod(
          getListConnectionsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListConnectionsRequest,
              io.github.panyam.agni.v1.webapi.ListConnectionsResponse>(
                service, METHODID_LIST_CONNECTIONS)))
        .addMethod(
          getDeleteConnectionMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.DeleteConnectionRequest,
              com.google.protobuf.Empty>(
                service, METHODID_DELETE_CONNECTION)))
        .build();
  }

  private static abstract class ConnectionServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    ConnectionServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.ConnectionsProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("ConnectionService");
    }
  }

  private static final class ConnectionServiceFileDescriptorSupplier
      extends ConnectionServiceBaseDescriptorSupplier {
    ConnectionServiceFileDescriptorSupplier() {}
  }

  private static final class ConnectionServiceMethodDescriptorSupplier
      extends ConnectionServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    ConnectionServiceMethodDescriptorSupplier(java.lang.String methodName) {
      this.methodName = methodName;
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.MethodDescriptor getMethodDescriptor() {
      return getServiceDescriptor().findMethodByName(methodName);
    }
  }

  private static volatile io.grpc.ServiceDescriptor serviceDescriptor;

  public static io.grpc.ServiceDescriptor getServiceDescriptor() {
    io.grpc.ServiceDescriptor result = serviceDescriptor;
    if (result == null) {
      synchronized (ConnectionServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new ConnectionServiceFileDescriptorSupplier())
              .addMethod(getCreateConnectionMethod())
              .addMethod(getGetConnectionMethod())
              .addMethod(getListConnectionsMethod())
              .addMethod(getDeleteConnectionMethod())
              .build();
        }
      }
    }
    return result;
  }
}
