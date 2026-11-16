package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * ServerService says what a host is, so a client can check it before relying on it (agni issue
 * 1629). A client built against one release calls GetServerInfo first, and can say clearly when the
 * server it reached is too old for a feature it needs.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class ServerServiceGrpc {

  private ServerServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.ServerService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetServerInfoRequest,
      io.github.panyam.agni.v1.webapi.ServerInfo> getGetServerInfoMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetServerInfo",
      requestType = io.github.panyam.agni.v1.webapi.GetServerInfoRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ServerInfo.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetServerInfoRequest,
      io.github.panyam.agni.v1.webapi.ServerInfo> getGetServerInfoMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetServerInfoRequest, io.github.panyam.agni.v1.webapi.ServerInfo> getGetServerInfoMethod;
    if ((getGetServerInfoMethod = ServerServiceGrpc.getGetServerInfoMethod) == null) {
      synchronized (ServerServiceGrpc.class) {
        if ((getGetServerInfoMethod = ServerServiceGrpc.getGetServerInfoMethod) == null) {
          ServerServiceGrpc.getGetServerInfoMethod = getGetServerInfoMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetServerInfoRequest, io.github.panyam.agni.v1.webapi.ServerInfo>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetServerInfo"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetServerInfoRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ServerInfo.getDefaultInstance()))
              .setSchemaDescriptor(new ServerServiceMethodDescriptorSupplier("GetServerInfo"))
              .build();
        }
      }
    }
    return getGetServerInfoMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static ServerServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ServerServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ServerServiceStub>() {
        @java.lang.Override
        public ServerServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ServerServiceStub(channel, callOptions);
        }
      };
    return ServerServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static ServerServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ServerServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ServerServiceBlockingV2Stub>() {
        @java.lang.Override
        public ServerServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ServerServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return ServerServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static ServerServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ServerServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ServerServiceBlockingStub>() {
        @java.lang.Override
        public ServerServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ServerServiceBlockingStub(channel, callOptions);
        }
      };
    return ServerServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static ServerServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ServerServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ServerServiceFutureStub>() {
        @java.lang.Override
        public ServerServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ServerServiceFutureStub(channel, callOptions);
        }
      };
    return ServerServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * ServerService says what a host is, so a client can check it before relying on it (agni issue
   * 1629). A client built against one release calls GetServerInfo first, and can say clearly when the
   * server it reached is too old for a feature it needs.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * GetServerInfo answers the release, the proto package and the optional features this host has on.
     * </pre>
     */
    default void getServerInfo(io.github.panyam.agni.v1.webapi.GetServerInfoRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ServerInfo> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetServerInfoMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service ServerService.
   * <pre>
   * ServerService says what a host is, so a client can check it before relying on it (agni issue
   * 1629). A client built against one release calls GetServerInfo first, and can say clearly when the
   * server it reached is too old for a feature it needs.
   * </pre>
   */
  public static abstract class ServerServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return ServerServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service ServerService.
   * <pre>
   * ServerService says what a host is, so a client can check it before relying on it (agni issue
   * 1629). A client built against one release calls GetServerInfo first, and can say clearly when the
   * server it reached is too old for a feature it needs.
   * </pre>
   */
  public static final class ServerServiceStub
      extends io.grpc.stub.AbstractAsyncStub<ServerServiceStub> {
    private ServerServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ServerServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ServerServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetServerInfo answers the release, the proto package and the optional features this host has on.
     * </pre>
     */
    public void getServerInfo(io.github.panyam.agni.v1.webapi.GetServerInfoRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ServerInfo> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetServerInfoMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service ServerService.
   * <pre>
   * ServerService says what a host is, so a client can check it before relying on it (agni issue
   * 1629). A client built against one release calls GetServerInfo first, and can say clearly when the
   * server it reached is too old for a feature it needs.
   * </pre>
   */
  public static final class ServerServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<ServerServiceBlockingV2Stub> {
    private ServerServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ServerServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ServerServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * GetServerInfo answers the release, the proto package and the optional features this host has on.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ServerInfo getServerInfo(io.github.panyam.agni.v1.webapi.GetServerInfoRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetServerInfoMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service ServerService.
   * <pre>
   * ServerService says what a host is, so a client can check it before relying on it (agni issue
   * 1629). A client built against one release calls GetServerInfo first, and can say clearly when the
   * server it reached is too old for a feature it needs.
   * </pre>
   */
  public static final class ServerServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<ServerServiceBlockingStub> {
    private ServerServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ServerServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ServerServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetServerInfo answers the release, the proto package and the optional features this host has on.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ServerInfo getServerInfo(io.github.panyam.agni.v1.webapi.GetServerInfoRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetServerInfoMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service ServerService.
   * <pre>
   * ServerService says what a host is, so a client can check it before relying on it (agni issue
   * 1629). A client built against one release calls GetServerInfo first, and can say clearly when the
   * server it reached is too old for a feature it needs.
   * </pre>
   */
  public static final class ServerServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<ServerServiceFutureStub> {
    private ServerServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ServerServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ServerServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetServerInfo answers the release, the proto package and the optional features this host has on.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ServerInfo> getServerInfo(
        io.github.panyam.agni.v1.webapi.GetServerInfoRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetServerInfoMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_GET_SERVER_INFO = 0;

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
        case METHODID_GET_SERVER_INFO:
          serviceImpl.getServerInfo((io.github.panyam.agni.v1.webapi.GetServerInfoRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ServerInfo>) responseObserver);
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
          getGetServerInfoMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetServerInfoRequest,
              io.github.panyam.agni.v1.webapi.ServerInfo>(
                service, METHODID_GET_SERVER_INFO)))
        .build();
  }

  private static abstract class ServerServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    ServerServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.ServerProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("ServerService");
    }
  }

  private static final class ServerServiceFileDescriptorSupplier
      extends ServerServiceBaseDescriptorSupplier {
    ServerServiceFileDescriptorSupplier() {}
  }

  private static final class ServerServiceMethodDescriptorSupplier
      extends ServerServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    ServerServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (ServerServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new ServerServiceFileDescriptorSupplier())
              .addMethod(getGetServerInfoMethod())
              .build();
        }
      }
    }
    return result;
  }
}
