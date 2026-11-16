package io.github.panyam.agni.v1.param;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * PartSpecService is the READ side of a published PartSpec corpus, and part of the engine's contract
 * beside PartSpec itself (agni issue 749). It answers which spec is CURRENT for a part number and
 * nothing else. Drafts, publishing and history belong to whoever keeps the corpus (the datasheet
 * service's own API) and never appear here, so a reader can only ever see a published, validated
 * spec.
 * The engine consumes it through param.ParamProvider. A design asks about a few hundred part numbers
 * at once, so the lookup is batched. Every answer includes the corpus generation. A caching reader
 * compares generations to learn that what it holds is stale.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class PartSpecServiceGrpc {

  private PartSpecServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.param.PartSpecService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.param.BatchGetPartSpecsRequest,
      io.github.panyam.agni.v1.param.BatchGetPartSpecsResponse> getBatchGetPartSpecsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "BatchGetPartSpecs",
      requestType = io.github.panyam.agni.v1.param.BatchGetPartSpecsRequest.class,
      responseType = io.github.panyam.agni.v1.param.BatchGetPartSpecsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.param.BatchGetPartSpecsRequest,
      io.github.panyam.agni.v1.param.BatchGetPartSpecsResponse> getBatchGetPartSpecsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.param.BatchGetPartSpecsRequest, io.github.panyam.agni.v1.param.BatchGetPartSpecsResponse> getBatchGetPartSpecsMethod;
    if ((getBatchGetPartSpecsMethod = PartSpecServiceGrpc.getBatchGetPartSpecsMethod) == null) {
      synchronized (PartSpecServiceGrpc.class) {
        if ((getBatchGetPartSpecsMethod = PartSpecServiceGrpc.getBatchGetPartSpecsMethod) == null) {
          PartSpecServiceGrpc.getBatchGetPartSpecsMethod = getBatchGetPartSpecsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.param.BatchGetPartSpecsRequest, io.github.panyam.agni.v1.param.BatchGetPartSpecsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "BatchGetPartSpecs"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.param.BatchGetPartSpecsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.param.BatchGetPartSpecsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new PartSpecServiceMethodDescriptorSupplier("BatchGetPartSpecs"))
              .build();
        }
      }
    }
    return getBatchGetPartSpecsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.param.GetGenerationRequest,
      io.github.panyam.agni.v1.param.GetGenerationResponse> getGetGenerationMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetGeneration",
      requestType = io.github.panyam.agni.v1.param.GetGenerationRequest.class,
      responseType = io.github.panyam.agni.v1.param.GetGenerationResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.param.GetGenerationRequest,
      io.github.panyam.agni.v1.param.GetGenerationResponse> getGetGenerationMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.param.GetGenerationRequest, io.github.panyam.agni.v1.param.GetGenerationResponse> getGetGenerationMethod;
    if ((getGetGenerationMethod = PartSpecServiceGrpc.getGetGenerationMethod) == null) {
      synchronized (PartSpecServiceGrpc.class) {
        if ((getGetGenerationMethod = PartSpecServiceGrpc.getGetGenerationMethod) == null) {
          PartSpecServiceGrpc.getGetGenerationMethod = getGetGenerationMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.param.GetGenerationRequest, io.github.panyam.agni.v1.param.GetGenerationResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetGeneration"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.param.GetGenerationRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.param.GetGenerationResponse.getDefaultInstance()))
              .setSchemaDescriptor(new PartSpecServiceMethodDescriptorSupplier("GetGeneration"))
              .build();
        }
      }
    }
    return getGetGenerationMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static PartSpecServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PartSpecServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PartSpecServiceStub>() {
        @java.lang.Override
        public PartSpecServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PartSpecServiceStub(channel, callOptions);
        }
      };
    return PartSpecServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static PartSpecServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PartSpecServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PartSpecServiceBlockingV2Stub>() {
        @java.lang.Override
        public PartSpecServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PartSpecServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return PartSpecServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static PartSpecServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PartSpecServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PartSpecServiceBlockingStub>() {
        @java.lang.Override
        public PartSpecServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PartSpecServiceBlockingStub(channel, callOptions);
        }
      };
    return PartSpecServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static PartSpecServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PartSpecServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PartSpecServiceFutureStub>() {
        @java.lang.Override
        public PartSpecServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PartSpecServiceFutureStub(channel, callOptions);
        }
      };
    return PartSpecServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * PartSpecService is the READ side of a published PartSpec corpus, and part of the engine's contract
   * beside PartSpec itself (agni issue 749). It answers which spec is CURRENT for a part number and
   * nothing else. Drafts, publishing and history belong to whoever keeps the corpus (the datasheet
   * service's own API) and never appear here, so a reader can only ever see a published, validated
   * spec.
   * The engine consumes it through param.ParamProvider. A design asks about a few hundred part numbers
   * at once, so the lookup is batched. Every answer includes the corpus generation. A caching reader
   * compares generations to learn that what it holds is stale.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * BatchGetPartSpecs returns the current spec for each part number that has one. A part number
     * with no published spec is simply absent from the answer. That is not an error, because most
     * parts on a board are never seeded. Matching is case-insensitive and never fuzzy, as
     * param.ParamSet.Lookup matches.
     * </pre>
     */
    default void batchGetPartSpecs(io.github.panyam.agni.v1.param.BatchGetPartSpecsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.param.BatchGetPartSpecsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getBatchGetPartSpecsMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetGeneration returns the corpus generation alone, so a cache can check it is current without
     * fetching any spec.
     * </pre>
     */
    default void getGeneration(io.github.panyam.agni.v1.param.GetGenerationRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.param.GetGenerationResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetGenerationMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service PartSpecService.
   * <pre>
   * PartSpecService is the READ side of a published PartSpec corpus, and part of the engine's contract
   * beside PartSpec itself (agni issue 749). It answers which spec is CURRENT for a part number and
   * nothing else. Drafts, publishing and history belong to whoever keeps the corpus (the datasheet
   * service's own API) and never appear here, so a reader can only ever see a published, validated
   * spec.
   * The engine consumes it through param.ParamProvider. A design asks about a few hundred part numbers
   * at once, so the lookup is batched. Every answer includes the corpus generation. A caching reader
   * compares generations to learn that what it holds is stale.
   * </pre>
   */
  public static abstract class PartSpecServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return PartSpecServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service PartSpecService.
   * <pre>
   * PartSpecService is the READ side of a published PartSpec corpus, and part of the engine's contract
   * beside PartSpec itself (agni issue 749). It answers which spec is CURRENT for a part number and
   * nothing else. Drafts, publishing and history belong to whoever keeps the corpus (the datasheet
   * service's own API) and never appear here, so a reader can only ever see a published, validated
   * spec.
   * The engine consumes it through param.ParamProvider. A design asks about a few hundred part numbers
   * at once, so the lookup is batched. Every answer includes the corpus generation. A caching reader
   * compares generations to learn that what it holds is stale.
   * </pre>
   */
  public static final class PartSpecServiceStub
      extends io.grpc.stub.AbstractAsyncStub<PartSpecServiceStub> {
    private PartSpecServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PartSpecServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PartSpecServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * BatchGetPartSpecs returns the current spec for each part number that has one. A part number
     * with no published spec is simply absent from the answer. That is not an error, because most
     * parts on a board are never seeded. Matching is case-insensitive and never fuzzy, as
     * param.ParamSet.Lookup matches.
     * </pre>
     */
    public void batchGetPartSpecs(io.github.panyam.agni.v1.param.BatchGetPartSpecsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.param.BatchGetPartSpecsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getBatchGetPartSpecsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetGeneration returns the corpus generation alone, so a cache can check it is current without
     * fetching any spec.
     * </pre>
     */
    public void getGeneration(io.github.panyam.agni.v1.param.GetGenerationRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.param.GetGenerationResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetGenerationMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service PartSpecService.
   * <pre>
   * PartSpecService is the READ side of a published PartSpec corpus, and part of the engine's contract
   * beside PartSpec itself (agni issue 749). It answers which spec is CURRENT for a part number and
   * nothing else. Drafts, publishing and history belong to whoever keeps the corpus (the datasheet
   * service's own API) and never appear here, so a reader can only ever see a published, validated
   * spec.
   * The engine consumes it through param.ParamProvider. A design asks about a few hundred part numbers
   * at once, so the lookup is batched. Every answer includes the corpus generation. A caching reader
   * compares generations to learn that what it holds is stale.
   * </pre>
   */
  public static final class PartSpecServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<PartSpecServiceBlockingV2Stub> {
    private PartSpecServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PartSpecServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PartSpecServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * BatchGetPartSpecs returns the current spec for each part number that has one. A part number
     * with no published spec is simply absent from the answer. That is not an error, because most
     * parts on a board are never seeded. Matching is case-insensitive and never fuzzy, as
     * param.ParamSet.Lookup matches.
     * </pre>
     */
    public io.github.panyam.agni.v1.param.BatchGetPartSpecsResponse batchGetPartSpecs(io.github.panyam.agni.v1.param.BatchGetPartSpecsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getBatchGetPartSpecsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetGeneration returns the corpus generation alone, so a cache can check it is current without
     * fetching any spec.
     * </pre>
     */
    public io.github.panyam.agni.v1.param.GetGenerationResponse getGeneration(io.github.panyam.agni.v1.param.GetGenerationRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetGenerationMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service PartSpecService.
   * <pre>
   * PartSpecService is the READ side of a published PartSpec corpus, and part of the engine's contract
   * beside PartSpec itself (agni issue 749). It answers which spec is CURRENT for a part number and
   * nothing else. Drafts, publishing and history belong to whoever keeps the corpus (the datasheet
   * service's own API) and never appear here, so a reader can only ever see a published, validated
   * spec.
   * The engine consumes it through param.ParamProvider. A design asks about a few hundred part numbers
   * at once, so the lookup is batched. Every answer includes the corpus generation. A caching reader
   * compares generations to learn that what it holds is stale.
   * </pre>
   */
  public static final class PartSpecServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<PartSpecServiceBlockingStub> {
    private PartSpecServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PartSpecServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PartSpecServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * BatchGetPartSpecs returns the current spec for each part number that has one. A part number
     * with no published spec is simply absent from the answer. That is not an error, because most
     * parts on a board are never seeded. Matching is case-insensitive and never fuzzy, as
     * param.ParamSet.Lookup matches.
     * </pre>
     */
    public io.github.panyam.agni.v1.param.BatchGetPartSpecsResponse batchGetPartSpecs(io.github.panyam.agni.v1.param.BatchGetPartSpecsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getBatchGetPartSpecsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetGeneration returns the corpus generation alone, so a cache can check it is current without
     * fetching any spec.
     * </pre>
     */
    public io.github.panyam.agni.v1.param.GetGenerationResponse getGeneration(io.github.panyam.agni.v1.param.GetGenerationRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetGenerationMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service PartSpecService.
   * <pre>
   * PartSpecService is the READ side of a published PartSpec corpus, and part of the engine's contract
   * beside PartSpec itself (agni issue 749). It answers which spec is CURRENT for a part number and
   * nothing else. Drafts, publishing and history belong to whoever keeps the corpus (the datasheet
   * service's own API) and never appear here, so a reader can only ever see a published, validated
   * spec.
   * The engine consumes it through param.ParamProvider. A design asks about a few hundred part numbers
   * at once, so the lookup is batched. Every answer includes the corpus generation. A caching reader
   * compares generations to learn that what it holds is stale.
   * </pre>
   */
  public static final class PartSpecServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<PartSpecServiceFutureStub> {
    private PartSpecServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PartSpecServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PartSpecServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * BatchGetPartSpecs returns the current spec for each part number that has one. A part number
     * with no published spec is simply absent from the answer. That is not an error, because most
     * parts on a board are never seeded. Matching is case-insensitive and never fuzzy, as
     * param.ParamSet.Lookup matches.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.param.BatchGetPartSpecsResponse> batchGetPartSpecs(
        io.github.panyam.agni.v1.param.BatchGetPartSpecsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getBatchGetPartSpecsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetGeneration returns the corpus generation alone, so a cache can check it is current without
     * fetching any spec.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.param.GetGenerationResponse> getGeneration(
        io.github.panyam.agni.v1.param.GetGenerationRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetGenerationMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_BATCH_GET_PART_SPECS = 0;
  private static final int METHODID_GET_GENERATION = 1;

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
        case METHODID_BATCH_GET_PART_SPECS:
          serviceImpl.batchGetPartSpecs((io.github.panyam.agni.v1.param.BatchGetPartSpecsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.param.BatchGetPartSpecsResponse>) responseObserver);
          break;
        case METHODID_GET_GENERATION:
          serviceImpl.getGeneration((io.github.panyam.agni.v1.param.GetGenerationRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.param.GetGenerationResponse>) responseObserver);
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
          getBatchGetPartSpecsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.param.BatchGetPartSpecsRequest,
              io.github.panyam.agni.v1.param.BatchGetPartSpecsResponse>(
                service, METHODID_BATCH_GET_PART_SPECS)))
        .addMethod(
          getGetGenerationMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.param.GetGenerationRequest,
              io.github.panyam.agni.v1.param.GetGenerationResponse>(
                service, METHODID_GET_GENERATION)))
        .build();
  }

  private static abstract class PartSpecServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    PartSpecServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.param.ServiceProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("PartSpecService");
    }
  }

  private static final class PartSpecServiceFileDescriptorSupplier
      extends PartSpecServiceBaseDescriptorSupplier {
    PartSpecServiceFileDescriptorSupplier() {}
  }

  private static final class PartSpecServiceMethodDescriptorSupplier
      extends PartSpecServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    PartSpecServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (PartSpecServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new PartSpecServiceFileDescriptorSupplier())
              .addMethod(getBatchGetPartSpecsMethod())
              .addMethod(getGetGenerationMethod())
              .build();
        }
      }
    }
    return result;
  }
}
