package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * This file defines ViewerRoomService. An open viewer page uses it to join its design's room, so an
 * agent can drive the page (agni issue 1154).
 * `agni serve --mcp --viewer-rooms` embeds a relay at /relay/ and mcpkit's room bridge beside /mcp.
 * A viewer page joins the room of the design it shows. There it offers ViewerPageService's methods
 * as tools (viewer_show_sheet, viewer_locate_verdict, ...). The bridge serves them to agents, and
 * each call names its design. Before joining, the page needs two things from the server. One is
 * the room's name, resolved as every other surface resolves the design (C32). The other is a token
 * for the relay to admit the page. This service returns both. Without --viewer-rooms the service is
 * not served.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class ViewerRoomServiceGrpc {

  private ViewerRoomServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.ViewerRoomService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.JoinViewerRoomRequest,
      io.github.panyam.agni.v1.webapi.JoinViewerRoomResponse> getJoinViewerRoomMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "JoinViewerRoom",
      requestType = io.github.panyam.agni.v1.webapi.JoinViewerRoomRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.JoinViewerRoomResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.JoinViewerRoomRequest,
      io.github.panyam.agni.v1.webapi.JoinViewerRoomResponse> getJoinViewerRoomMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.JoinViewerRoomRequest, io.github.panyam.agni.v1.webapi.JoinViewerRoomResponse> getJoinViewerRoomMethod;
    if ((getJoinViewerRoomMethod = ViewerRoomServiceGrpc.getJoinViewerRoomMethod) == null) {
      synchronized (ViewerRoomServiceGrpc.class) {
        if ((getJoinViewerRoomMethod = ViewerRoomServiceGrpc.getJoinViewerRoomMethod) == null) {
          ViewerRoomServiceGrpc.getJoinViewerRoomMethod = getJoinViewerRoomMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.JoinViewerRoomRequest, io.github.panyam.agni.v1.webapi.JoinViewerRoomResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "JoinViewerRoom"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.JoinViewerRoomRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.JoinViewerRoomResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ViewerRoomServiceMethodDescriptorSupplier("JoinViewerRoom"))
              .build();
        }
      }
    }
    return getJoinViewerRoomMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static ViewerRoomServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ViewerRoomServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ViewerRoomServiceStub>() {
        @java.lang.Override
        public ViewerRoomServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ViewerRoomServiceStub(channel, callOptions);
        }
      };
    return ViewerRoomServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static ViewerRoomServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ViewerRoomServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ViewerRoomServiceBlockingV2Stub>() {
        @java.lang.Override
        public ViewerRoomServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ViewerRoomServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return ViewerRoomServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static ViewerRoomServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ViewerRoomServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ViewerRoomServiceBlockingStub>() {
        @java.lang.Override
        public ViewerRoomServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ViewerRoomServiceBlockingStub(channel, callOptions);
        }
      };
    return ViewerRoomServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static ViewerRoomServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ViewerRoomServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ViewerRoomServiceFutureStub>() {
        @java.lang.Override
        public ViewerRoomServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ViewerRoomServiceFutureStub(channel, callOptions);
        }
      };
    return ViewerRoomServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * This file defines ViewerRoomService. An open viewer page uses it to join its design's room, so an
   * agent can drive the page (agni issue 1154).
   * `agni serve --mcp --viewer-rooms` embeds a relay at /relay/ and mcpkit's room bridge beside /mcp.
   * A viewer page joins the room of the design it shows. There it offers ViewerPageService's methods
   * as tools (viewer_show_sheet, viewer_locate_verdict, ...). The bridge serves them to agents, and
   * each call names its design. Before joining, the page needs two things from the server. One is
   * the room's name, resolved as every other surface resolves the design (C32). The other is a token
   * for the relay to admit the page. This service returns both. Without --viewer-rooms the service is
   * not served.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * JoinViewerRoom names the room for a design and mints a relay token scoped to it. It stores
     * nothing; each call mints a fresh token.
     * </pre>
     */
    default void joinViewerRoom(io.github.panyam.agni.v1.webapi.JoinViewerRoomRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.JoinViewerRoomResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getJoinViewerRoomMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service ViewerRoomService.
   * <pre>
   * This file defines ViewerRoomService. An open viewer page uses it to join its design's room, so an
   * agent can drive the page (agni issue 1154).
   * `agni serve --mcp --viewer-rooms` embeds a relay at /relay/ and mcpkit's room bridge beside /mcp.
   * A viewer page joins the room of the design it shows. There it offers ViewerPageService's methods
   * as tools (viewer_show_sheet, viewer_locate_verdict, ...). The bridge serves them to agents, and
   * each call names its design. Before joining, the page needs two things from the server. One is
   * the room's name, resolved as every other surface resolves the design (C32). The other is a token
   * for the relay to admit the page. This service returns both. Without --viewer-rooms the service is
   * not served.
   * </pre>
   */
  public static abstract class ViewerRoomServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return ViewerRoomServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service ViewerRoomService.
   * <pre>
   * This file defines ViewerRoomService. An open viewer page uses it to join its design's room, so an
   * agent can drive the page (agni issue 1154).
   * `agni serve --mcp --viewer-rooms` embeds a relay at /relay/ and mcpkit's room bridge beside /mcp.
   * A viewer page joins the room of the design it shows. There it offers ViewerPageService's methods
   * as tools (viewer_show_sheet, viewer_locate_verdict, ...). The bridge serves them to agents, and
   * each call names its design. Before joining, the page needs two things from the server. One is
   * the room's name, resolved as every other surface resolves the design (C32). The other is a token
   * for the relay to admit the page. This service returns both. Without --viewer-rooms the service is
   * not served.
   * </pre>
   */
  public static final class ViewerRoomServiceStub
      extends io.grpc.stub.AbstractAsyncStub<ViewerRoomServiceStub> {
    private ViewerRoomServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ViewerRoomServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ViewerRoomServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * JoinViewerRoom names the room for a design and mints a relay token scoped to it. It stores
     * nothing; each call mints a fresh token.
     * </pre>
     */
    public void joinViewerRoom(io.github.panyam.agni.v1.webapi.JoinViewerRoomRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.JoinViewerRoomResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getJoinViewerRoomMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service ViewerRoomService.
   * <pre>
   * This file defines ViewerRoomService. An open viewer page uses it to join its design's room, so an
   * agent can drive the page (agni issue 1154).
   * `agni serve --mcp --viewer-rooms` embeds a relay at /relay/ and mcpkit's room bridge beside /mcp.
   * A viewer page joins the room of the design it shows. There it offers ViewerPageService's methods
   * as tools (viewer_show_sheet, viewer_locate_verdict, ...). The bridge serves them to agents, and
   * each call names its design. Before joining, the page needs two things from the server. One is
   * the room's name, resolved as every other surface resolves the design (C32). The other is a token
   * for the relay to admit the page. This service returns both. Without --viewer-rooms the service is
   * not served.
   * </pre>
   */
  public static final class ViewerRoomServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<ViewerRoomServiceBlockingV2Stub> {
    private ViewerRoomServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ViewerRoomServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ViewerRoomServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * JoinViewerRoom names the room for a design and mints a relay token scoped to it. It stores
     * nothing; each call mints a fresh token.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.JoinViewerRoomResponse joinViewerRoom(io.github.panyam.agni.v1.webapi.JoinViewerRoomRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getJoinViewerRoomMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service ViewerRoomService.
   * <pre>
   * This file defines ViewerRoomService. An open viewer page uses it to join its design's room, so an
   * agent can drive the page (agni issue 1154).
   * `agni serve --mcp --viewer-rooms` embeds a relay at /relay/ and mcpkit's room bridge beside /mcp.
   * A viewer page joins the room of the design it shows. There it offers ViewerPageService's methods
   * as tools (viewer_show_sheet, viewer_locate_verdict, ...). The bridge serves them to agents, and
   * each call names its design. Before joining, the page needs two things from the server. One is
   * the room's name, resolved as every other surface resolves the design (C32). The other is a token
   * for the relay to admit the page. This service returns both. Without --viewer-rooms the service is
   * not served.
   * </pre>
   */
  public static final class ViewerRoomServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<ViewerRoomServiceBlockingStub> {
    private ViewerRoomServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ViewerRoomServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ViewerRoomServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * JoinViewerRoom names the room for a design and mints a relay token scoped to it. It stores
     * nothing; each call mints a fresh token.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.JoinViewerRoomResponse joinViewerRoom(io.github.panyam.agni.v1.webapi.JoinViewerRoomRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getJoinViewerRoomMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service ViewerRoomService.
   * <pre>
   * This file defines ViewerRoomService. An open viewer page uses it to join its design's room, so an
   * agent can drive the page (agni issue 1154).
   * `agni serve --mcp --viewer-rooms` embeds a relay at /relay/ and mcpkit's room bridge beside /mcp.
   * A viewer page joins the room of the design it shows. There it offers ViewerPageService's methods
   * as tools (viewer_show_sheet, viewer_locate_verdict, ...). The bridge serves them to agents, and
   * each call names its design. Before joining, the page needs two things from the server. One is
   * the room's name, resolved as every other surface resolves the design (C32). The other is a token
   * for the relay to admit the page. This service returns both. Without --viewer-rooms the service is
   * not served.
   * </pre>
   */
  public static final class ViewerRoomServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<ViewerRoomServiceFutureStub> {
    private ViewerRoomServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ViewerRoomServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ViewerRoomServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * JoinViewerRoom names the room for a design and mints a relay token scoped to it. It stores
     * nothing; each call mints a fresh token.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.JoinViewerRoomResponse> joinViewerRoom(
        io.github.panyam.agni.v1.webapi.JoinViewerRoomRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getJoinViewerRoomMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_JOIN_VIEWER_ROOM = 0;

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
        case METHODID_JOIN_VIEWER_ROOM:
          serviceImpl.joinViewerRoom((io.github.panyam.agni.v1.webapi.JoinViewerRoomRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.JoinViewerRoomResponse>) responseObserver);
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
          getJoinViewerRoomMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.JoinViewerRoomRequest,
              io.github.panyam.agni.v1.webapi.JoinViewerRoomResponse>(
                service, METHODID_JOIN_VIEWER_ROOM)))
        .build();
  }

  private static abstract class ViewerRoomServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    ViewerRoomServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.ViewerroomProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("ViewerRoomService");
    }
  }

  private static final class ViewerRoomServiceFileDescriptorSupplier
      extends ViewerRoomServiceBaseDescriptorSupplier {
    ViewerRoomServiceFileDescriptorSupplier() {}
  }

  private static final class ViewerRoomServiceMethodDescriptorSupplier
      extends ViewerRoomServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    ViewerRoomServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (ViewerRoomServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new ViewerRoomServiceFileDescriptorSupplier())
              .addMethod(getJoinViewerRoomMethod())
              .build();
        }
      }
    }
    return result;
  }
}
