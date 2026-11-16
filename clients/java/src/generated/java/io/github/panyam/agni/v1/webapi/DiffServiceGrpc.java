package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * DiffService computes the semantic diff between two designs' netlist IR (diff.Designs,
 * docsite/content/architecture/semantic-diff.md). It matches components by ref_des and nets by
 * name, and it recovers renames by connection signature. The diff core has no presentation code.
 * This service owns the wire form, and `agni diff --format json` emits the same form, so the CLI
 * and the viewer parse one message (WS9-004).
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class DiffServiceGrpc {

  private DiffServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.DiffService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DiffDesignsRequest,
      io.github.panyam.agni.v1.webapi.DiffDesignsResponse> getDiffDesignsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "DiffDesigns",
      requestType = io.github.panyam.agni.v1.webapi.DiffDesignsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.DiffDesignsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DiffDesignsRequest,
      io.github.panyam.agni.v1.webapi.DiffDesignsResponse> getDiffDesignsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DiffDesignsRequest, io.github.panyam.agni.v1.webapi.DiffDesignsResponse> getDiffDesignsMethod;
    if ((getDiffDesignsMethod = DiffServiceGrpc.getDiffDesignsMethod) == null) {
      synchronized (DiffServiceGrpc.class) {
        if ((getDiffDesignsMethod = DiffServiceGrpc.getDiffDesignsMethod) == null) {
          DiffServiceGrpc.getDiffDesignsMethod = getDiffDesignsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.DiffDesignsRequest, io.github.panyam.agni.v1.webapi.DiffDesignsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "DiffDesigns"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.DiffDesignsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.DiffDesignsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DiffServiceMethodDescriptorSupplier("DiffDesigns"))
              .build();
        }
      }
    }
    return getDiffDesignsMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static DiffServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DiffServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DiffServiceStub>() {
        @java.lang.Override
        public DiffServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DiffServiceStub(channel, callOptions);
        }
      };
    return DiffServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static DiffServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DiffServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DiffServiceBlockingV2Stub>() {
        @java.lang.Override
        public DiffServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DiffServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return DiffServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static DiffServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DiffServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DiffServiceBlockingStub>() {
        @java.lang.Override
        public DiffServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DiffServiceBlockingStub(channel, callOptions);
        }
      };
    return DiffServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static DiffServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DiffServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DiffServiceFutureStub>() {
        @java.lang.Override
        public DiffServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DiffServiceFutureStub(channel, callOptions);
        }
      };
    return DiffServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * DiffService computes the semantic diff between two designs' netlist IR (diff.Designs,
   * docsite/content/architecture/semantic-diff.md). It matches components by ref_des and nets by
   * name, and it recovers renames by connection signature. The diff core has no presentation code.
   * This service owns the wire form, and `agni diff --format json` emits the same form, so the CLI
   * and the viewer parse one message (WS9-004).
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * DiffDesigns loads both snapshots' netlist IR. It returns the classified report and the
     * highlight maps, and the viewer joins those maps to geometry. If either side fails to load
     * (unknown mount, escaping path, no netlist reader), the call fails. There is no partial diff.
     * </pre>
     */
    default void diffDesigns(io.github.panyam.agni.v1.webapi.DiffDesignsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.DiffDesignsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDiffDesignsMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service DiffService.
   * <pre>
   * DiffService computes the semantic diff between two designs' netlist IR (diff.Designs,
   * docsite/content/architecture/semantic-diff.md). It matches components by ref_des and nets by
   * name, and it recovers renames by connection signature. The diff core has no presentation code.
   * This service owns the wire form, and `agni diff --format json` emits the same form, so the CLI
   * and the viewer parse one message (WS9-004).
   * </pre>
   */
  public static abstract class DiffServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return DiffServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service DiffService.
   * <pre>
   * DiffService computes the semantic diff between two designs' netlist IR (diff.Designs,
   * docsite/content/architecture/semantic-diff.md). It matches components by ref_des and nets by
   * name, and it recovers renames by connection signature. The diff core has no presentation code.
   * This service owns the wire form, and `agni diff --format json` emits the same form, so the CLI
   * and the viewer parse one message (WS9-004).
   * </pre>
   */
  public static final class DiffServiceStub
      extends io.grpc.stub.AbstractAsyncStub<DiffServiceStub> {
    private DiffServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DiffServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DiffServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * DiffDesigns loads both snapshots' netlist IR. It returns the classified report and the
     * highlight maps, and the viewer joins those maps to geometry. If either side fails to load
     * (unknown mount, escaping path, no netlist reader), the call fails. There is no partial diff.
     * </pre>
     */
    public void diffDesigns(io.github.panyam.agni.v1.webapi.DiffDesignsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.DiffDesignsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDiffDesignsMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service DiffService.
   * <pre>
   * DiffService computes the semantic diff between two designs' netlist IR (diff.Designs,
   * docsite/content/architecture/semantic-diff.md). It matches components by ref_des and nets by
   * name, and it recovers renames by connection signature. The diff core has no presentation code.
   * This service owns the wire form, and `agni diff --format json` emits the same form, so the CLI
   * and the viewer parse one message (WS9-004).
   * </pre>
   */
  public static final class DiffServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<DiffServiceBlockingV2Stub> {
    private DiffServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DiffServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DiffServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * DiffDesigns loads both snapshots' netlist IR. It returns the classified report and the
     * highlight maps, and the viewer joins those maps to geometry. If either side fails to load
     * (unknown mount, escaping path, no netlist reader), the call fails. There is no partial diff.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.DiffDesignsResponse diffDesigns(io.github.panyam.agni.v1.webapi.DiffDesignsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getDiffDesignsMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service DiffService.
   * <pre>
   * DiffService computes the semantic diff between two designs' netlist IR (diff.Designs,
   * docsite/content/architecture/semantic-diff.md). It matches components by ref_des and nets by
   * name, and it recovers renames by connection signature. The diff core has no presentation code.
   * This service owns the wire form, and `agni diff --format json` emits the same form, so the CLI
   * and the viewer parse one message (WS9-004).
   * </pre>
   */
  public static final class DiffServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<DiffServiceBlockingStub> {
    private DiffServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DiffServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DiffServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * DiffDesigns loads both snapshots' netlist IR. It returns the classified report and the
     * highlight maps, and the viewer joins those maps to geometry. If either side fails to load
     * (unknown mount, escaping path, no netlist reader), the call fails. There is no partial diff.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.DiffDesignsResponse diffDesigns(io.github.panyam.agni.v1.webapi.DiffDesignsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDiffDesignsMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service DiffService.
   * <pre>
   * DiffService computes the semantic diff between two designs' netlist IR (diff.Designs,
   * docsite/content/architecture/semantic-diff.md). It matches components by ref_des and nets by
   * name, and it recovers renames by connection signature. The diff core has no presentation code.
   * This service owns the wire form, and `agni diff --format json` emits the same form, so the CLI
   * and the viewer parse one message (WS9-004).
   * </pre>
   */
  public static final class DiffServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<DiffServiceFutureStub> {
    private DiffServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DiffServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DiffServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * DiffDesigns loads both snapshots' netlist IR. It returns the classified report and the
     * highlight maps, and the viewer joins those maps to geometry. If either side fails to load
     * (unknown mount, escaping path, no netlist reader), the call fails. There is no partial diff.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.DiffDesignsResponse> diffDesigns(
        io.github.panyam.agni.v1.webapi.DiffDesignsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDiffDesignsMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_DIFF_DESIGNS = 0;

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
        case METHODID_DIFF_DESIGNS:
          serviceImpl.diffDesigns((io.github.panyam.agni.v1.webapi.DiffDesignsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.DiffDesignsResponse>) responseObserver);
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
          getDiffDesignsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.DiffDesignsRequest,
              io.github.panyam.agni.v1.webapi.DiffDesignsResponse>(
                service, METHODID_DIFF_DESIGNS)))
        .build();
  }

  private static abstract class DiffServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    DiffServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.DiffProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("DiffService");
    }
  }

  private static final class DiffServiceFileDescriptorSupplier
      extends DiffServiceBaseDescriptorSupplier {
    DiffServiceFileDescriptorSupplier() {}
  }

  private static final class DiffServiceMethodDescriptorSupplier
      extends DiffServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    DiffServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (DiffServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new DiffServiceFileDescriptorSupplier())
              .addMethod(getDiffDesignsMethod())
              .build();
        }
      }
    }
    return result;
  }
}
