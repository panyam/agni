package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * LexiconService answers questions about the naming lexicon a snapshot was read with.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class LexiconServiceGrpc {

  private LexiconServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.LexiconService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetLexiconRequest,
      io.github.panyam.agni.v1.webapi.GetLexiconResponse> getGetLexiconMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetLexicon",
      requestType = io.github.panyam.agni.v1.webapi.GetLexiconRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.GetLexiconResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetLexiconRequest,
      io.github.panyam.agni.v1.webapi.GetLexiconResponse> getGetLexiconMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetLexiconRequest, io.github.panyam.agni.v1.webapi.GetLexiconResponse> getGetLexiconMethod;
    if ((getGetLexiconMethod = LexiconServiceGrpc.getGetLexiconMethod) == null) {
      synchronized (LexiconServiceGrpc.class) {
        if ((getGetLexiconMethod = LexiconServiceGrpc.getGetLexiconMethod) == null) {
          LexiconServiceGrpc.getGetLexiconMethod = getGetLexiconMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetLexiconRequest, io.github.panyam.agni.v1.webapi.GetLexiconResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetLexicon"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetLexiconRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetLexiconResponse.getDefaultInstance()))
              .setSchemaDescriptor(new LexiconServiceMethodDescriptorSupplier("GetLexicon"))
              .build();
        }
      }
    }
    return getGetLexiconMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ExplainNetRequest,
      io.github.panyam.agni.v1.webapi.ExplainNetResponse> getExplainNetMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ExplainNet",
      requestType = io.github.panyam.agni.v1.webapi.ExplainNetRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ExplainNetResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ExplainNetRequest,
      io.github.panyam.agni.v1.webapi.ExplainNetResponse> getExplainNetMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ExplainNetRequest, io.github.panyam.agni.v1.webapi.ExplainNetResponse> getExplainNetMethod;
    if ((getExplainNetMethod = LexiconServiceGrpc.getExplainNetMethod) == null) {
      synchronized (LexiconServiceGrpc.class) {
        if ((getExplainNetMethod = LexiconServiceGrpc.getExplainNetMethod) == null) {
          LexiconServiceGrpc.getExplainNetMethod = getExplainNetMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ExplainNetRequest, io.github.panyam.agni.v1.webapi.ExplainNetResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ExplainNet"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ExplainNetRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ExplainNetResponse.getDefaultInstance()))
              .setSchemaDescriptor(new LexiconServiceMethodDescriptorSupplier("ExplainNet"))
              .build();
        }
      }
    }
    return getExplainNetMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CompareLexiconsRequest,
      io.github.panyam.agni.v1.webapi.CompareLexiconsResponse> getCompareLexiconsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CompareLexicons",
      requestType = io.github.panyam.agni.v1.webapi.CompareLexiconsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.CompareLexiconsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CompareLexiconsRequest,
      io.github.panyam.agni.v1.webapi.CompareLexiconsResponse> getCompareLexiconsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CompareLexiconsRequest, io.github.panyam.agni.v1.webapi.CompareLexiconsResponse> getCompareLexiconsMethod;
    if ((getCompareLexiconsMethod = LexiconServiceGrpc.getCompareLexiconsMethod) == null) {
      synchronized (LexiconServiceGrpc.class) {
        if ((getCompareLexiconsMethod = LexiconServiceGrpc.getCompareLexiconsMethod) == null) {
          LexiconServiceGrpc.getCompareLexiconsMethod = getCompareLexiconsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.CompareLexiconsRequest, io.github.panyam.agni.v1.webapi.CompareLexiconsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CompareLexicons"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.CompareLexiconsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.CompareLexiconsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new LexiconServiceMethodDescriptorSupplier("CompareLexicons"))
              .build();
        }
      }
    }
    return getCompareLexiconsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RenderConventionRequest,
      io.github.panyam.agni.v1.webapi.RenderConventionResponse> getRenderConventionMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "RenderConvention",
      requestType = io.github.panyam.agni.v1.webapi.RenderConventionRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.RenderConventionResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RenderConventionRequest,
      io.github.panyam.agni.v1.webapi.RenderConventionResponse> getRenderConventionMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RenderConventionRequest, io.github.panyam.agni.v1.webapi.RenderConventionResponse> getRenderConventionMethod;
    if ((getRenderConventionMethod = LexiconServiceGrpc.getRenderConventionMethod) == null) {
      synchronized (LexiconServiceGrpc.class) {
        if ((getRenderConventionMethod = LexiconServiceGrpc.getRenderConventionMethod) == null) {
          LexiconServiceGrpc.getRenderConventionMethod = getRenderConventionMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.RenderConventionRequest, io.github.panyam.agni.v1.webapi.RenderConventionResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "RenderConvention"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.RenderConventionRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.RenderConventionResponse.getDefaultInstance()))
              .setSchemaDescriptor(new LexiconServiceMethodDescriptorSupplier("RenderConvention"))
              .build();
        }
      }
    }
    return getRenderConventionMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static LexiconServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<LexiconServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<LexiconServiceStub>() {
        @java.lang.Override
        public LexiconServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new LexiconServiceStub(channel, callOptions);
        }
      };
    return LexiconServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static LexiconServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<LexiconServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<LexiconServiceBlockingV2Stub>() {
        @java.lang.Override
        public LexiconServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new LexiconServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return LexiconServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static LexiconServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<LexiconServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<LexiconServiceBlockingStub>() {
        @java.lang.Override
        public LexiconServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new LexiconServiceBlockingStub(channel, callOptions);
        }
      };
    return LexiconServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static LexiconServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<LexiconServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<LexiconServiceFutureStub>() {
        @java.lang.Override
        public LexiconServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new LexiconServiceFutureStub(channel, callOptions);
        }
      };
    return LexiconServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * LexiconService answers questions about the naming lexicon a snapshot was read with.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * GetLexicon returns every pattern of the snapshot's effective lexicon, with where it came from.
     * Given a candidate convention, it lays that over the built-ins instead. So a client editing a
     * convention lists what a read would use, and never merges a convention itself. A pattern that
     * will not compile is INVALID_ARGUMENT.
     * </pre>
     */
    default void getLexicon(io.github.panyam.agni.v1.webapi.GetLexiconRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetLexiconResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetLexiconMethod(), responseObserver);
    }

    /**
     * <pre>
     * ExplainNet returns one net's roles with their reasons. It also returns every pattern of the
     * effective lexicon that the net's name matches, exclusions included. So a name that missed a
     * vocabulary shows what stopped it. A name missing from the design is NOT_FOUND.
     * </pre>
     */
    default void explainNet(io.github.panyam.agni.v1.webapi.ExplainNetRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ExplainNetResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getExplainNetMethod(), responseObserver);
    }

    /**
     * <pre>
     * CompareLexicons reads the snapshot again, with a candidate convention replacing the original
     * one. It runs the rule catalog over both reads. It returns the nets whose roles change, and the
     * findings that appear or disappear.
     * </pre>
     */
    default void compareLexicons(io.github.panyam.agni.v1.webapi.CompareLexiconsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.CompareLexiconsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCompareLexiconsMethod(), responseObserver);
    }

    /**
     * <pre>
     * RenderConvention writes a convention as a project's YAML file. naming.Parse reads that file back
     * to the same message. It asks nothing about a design, so it names no snapshot.
     * </pre>
     */
    default void renderConvention(io.github.panyam.agni.v1.webapi.RenderConventionRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RenderConventionResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRenderConventionMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service LexiconService.
   * <pre>
   * LexiconService answers questions about the naming lexicon a snapshot was read with.
   * </pre>
   */
  public static abstract class LexiconServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return LexiconServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service LexiconService.
   * <pre>
   * LexiconService answers questions about the naming lexicon a snapshot was read with.
   * </pre>
   */
  public static final class LexiconServiceStub
      extends io.grpc.stub.AbstractAsyncStub<LexiconServiceStub> {
    private LexiconServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected LexiconServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new LexiconServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetLexicon returns every pattern of the snapshot's effective lexicon, with where it came from.
     * Given a candidate convention, it lays that over the built-ins instead. So a client editing a
     * convention lists what a read would use, and never merges a convention itself. A pattern that
     * will not compile is INVALID_ARGUMENT.
     * </pre>
     */
    public void getLexicon(io.github.panyam.agni.v1.webapi.GetLexiconRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetLexiconResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetLexiconMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ExplainNet returns one net's roles with their reasons. It also returns every pattern of the
     * effective lexicon that the net's name matches, exclusions included. So a name that missed a
     * vocabulary shows what stopped it. A name missing from the design is NOT_FOUND.
     * </pre>
     */
    public void explainNet(io.github.panyam.agni.v1.webapi.ExplainNetRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ExplainNetResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getExplainNetMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * CompareLexicons reads the snapshot again, with a candidate convention replacing the original
     * one. It runs the rule catalog over both reads. It returns the nets whose roles change, and the
     * findings that appear or disappear.
     * </pre>
     */
    public void compareLexicons(io.github.panyam.agni.v1.webapi.CompareLexiconsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.CompareLexiconsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCompareLexiconsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * RenderConvention writes a convention as a project's YAML file. naming.Parse reads that file back
     * to the same message. It asks nothing about a design, so it names no snapshot.
     * </pre>
     */
    public void renderConvention(io.github.panyam.agni.v1.webapi.RenderConventionRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RenderConventionResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRenderConventionMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service LexiconService.
   * <pre>
   * LexiconService answers questions about the naming lexicon a snapshot was read with.
   * </pre>
   */
  public static final class LexiconServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<LexiconServiceBlockingV2Stub> {
    private LexiconServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected LexiconServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new LexiconServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * GetLexicon returns every pattern of the snapshot's effective lexicon, with where it came from.
     * Given a candidate convention, it lays that over the built-ins instead. So a client editing a
     * convention lists what a read would use, and never merges a convention itself. A pattern that
     * will not compile is INVALID_ARGUMENT.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetLexiconResponse getLexicon(io.github.panyam.agni.v1.webapi.GetLexiconRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetLexiconMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ExplainNet returns one net's roles with their reasons. It also returns every pattern of the
     * effective lexicon that the net's name matches, exclusions included. So a name that missed a
     * vocabulary shows what stopped it. A name missing from the design is NOT_FOUND.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ExplainNetResponse explainNet(io.github.panyam.agni.v1.webapi.ExplainNetRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getExplainNetMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * CompareLexicons reads the snapshot again, with a candidate convention replacing the original
     * one. It runs the rule catalog over both reads. It returns the nets whose roles change, and the
     * findings that appear or disappear.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.CompareLexiconsResponse compareLexicons(io.github.panyam.agni.v1.webapi.CompareLexiconsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getCompareLexiconsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * RenderConvention writes a convention as a project's YAML file. naming.Parse reads that file back
     * to the same message. It asks nothing about a design, so it names no snapshot.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.RenderConventionResponse renderConvention(io.github.panyam.agni.v1.webapi.RenderConventionRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getRenderConventionMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service LexiconService.
   * <pre>
   * LexiconService answers questions about the naming lexicon a snapshot was read with.
   * </pre>
   */
  public static final class LexiconServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<LexiconServiceBlockingStub> {
    private LexiconServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected LexiconServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new LexiconServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetLexicon returns every pattern of the snapshot's effective lexicon, with where it came from.
     * Given a candidate convention, it lays that over the built-ins instead. So a client editing a
     * convention lists what a read would use, and never merges a convention itself. A pattern that
     * will not compile is INVALID_ARGUMENT.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetLexiconResponse getLexicon(io.github.panyam.agni.v1.webapi.GetLexiconRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetLexiconMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ExplainNet returns one net's roles with their reasons. It also returns every pattern of the
     * effective lexicon that the net's name matches, exclusions included. So a name that missed a
     * vocabulary shows what stopped it. A name missing from the design is NOT_FOUND.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ExplainNetResponse explainNet(io.github.panyam.agni.v1.webapi.ExplainNetRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getExplainNetMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * CompareLexicons reads the snapshot again, with a candidate convention replacing the original
     * one. It runs the rule catalog over both reads. It returns the nets whose roles change, and the
     * findings that appear or disappear.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.CompareLexiconsResponse compareLexicons(io.github.panyam.agni.v1.webapi.CompareLexiconsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCompareLexiconsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * RenderConvention writes a convention as a project's YAML file. naming.Parse reads that file back
     * to the same message. It asks nothing about a design, so it names no snapshot.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.RenderConventionResponse renderConvention(io.github.panyam.agni.v1.webapi.RenderConventionRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRenderConventionMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service LexiconService.
   * <pre>
   * LexiconService answers questions about the naming lexicon a snapshot was read with.
   * </pre>
   */
  public static final class LexiconServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<LexiconServiceFutureStub> {
    private LexiconServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected LexiconServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new LexiconServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetLexicon returns every pattern of the snapshot's effective lexicon, with where it came from.
     * Given a candidate convention, it lays that over the built-ins instead. So a client editing a
     * convention lists what a read would use, and never merges a convention itself. A pattern that
     * will not compile is INVALID_ARGUMENT.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.GetLexiconResponse> getLexicon(
        io.github.panyam.agni.v1.webapi.GetLexiconRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetLexiconMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ExplainNet returns one net's roles with their reasons. It also returns every pattern of the
     * effective lexicon that the net's name matches, exclusions included. So a name that missed a
     * vocabulary shows what stopped it. A name missing from the design is NOT_FOUND.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ExplainNetResponse> explainNet(
        io.github.panyam.agni.v1.webapi.ExplainNetRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getExplainNetMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * CompareLexicons reads the snapshot again, with a candidate convention replacing the original
     * one. It runs the rule catalog over both reads. It returns the nets whose roles change, and the
     * findings that appear or disappear.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.CompareLexiconsResponse> compareLexicons(
        io.github.panyam.agni.v1.webapi.CompareLexiconsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCompareLexiconsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * RenderConvention writes a convention as a project's YAML file. naming.Parse reads that file back
     * to the same message. It asks nothing about a design, so it names no snapshot.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.RenderConventionResponse> renderConvention(
        io.github.panyam.agni.v1.webapi.RenderConventionRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRenderConventionMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_GET_LEXICON = 0;
  private static final int METHODID_EXPLAIN_NET = 1;
  private static final int METHODID_COMPARE_LEXICONS = 2;
  private static final int METHODID_RENDER_CONVENTION = 3;

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
        case METHODID_GET_LEXICON:
          serviceImpl.getLexicon((io.github.panyam.agni.v1.webapi.GetLexiconRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetLexiconResponse>) responseObserver);
          break;
        case METHODID_EXPLAIN_NET:
          serviceImpl.explainNet((io.github.panyam.agni.v1.webapi.ExplainNetRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ExplainNetResponse>) responseObserver);
          break;
        case METHODID_COMPARE_LEXICONS:
          serviceImpl.compareLexicons((io.github.panyam.agni.v1.webapi.CompareLexiconsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.CompareLexiconsResponse>) responseObserver);
          break;
        case METHODID_RENDER_CONVENTION:
          serviceImpl.renderConvention((io.github.panyam.agni.v1.webapi.RenderConventionRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RenderConventionResponse>) responseObserver);
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
          getGetLexiconMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetLexiconRequest,
              io.github.panyam.agni.v1.webapi.GetLexiconResponse>(
                service, METHODID_GET_LEXICON)))
        .addMethod(
          getExplainNetMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ExplainNetRequest,
              io.github.panyam.agni.v1.webapi.ExplainNetResponse>(
                service, METHODID_EXPLAIN_NET)))
        .addMethod(
          getCompareLexiconsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.CompareLexiconsRequest,
              io.github.panyam.agni.v1.webapi.CompareLexiconsResponse>(
                service, METHODID_COMPARE_LEXICONS)))
        .addMethod(
          getRenderConventionMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.RenderConventionRequest,
              io.github.panyam.agni.v1.webapi.RenderConventionResponse>(
                service, METHODID_RENDER_CONVENTION)))
        .build();
  }

  private static abstract class LexiconServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    LexiconServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.LexiconProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("LexiconService");
    }
  }

  private static final class LexiconServiceFileDescriptorSupplier
      extends LexiconServiceBaseDescriptorSupplier {
    LexiconServiceFileDescriptorSupplier() {}
  }

  private static final class LexiconServiceMethodDescriptorSupplier
      extends LexiconServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    LexiconServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (LexiconServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new LexiconServiceFileDescriptorSupplier())
              .addMethod(getGetLexiconMethod())
              .addMethod(getExplainNetMethod())
              .addMethod(getCompareLexiconsMethod())
              .addMethod(getRenderConventionMethod())
              .build();
        }
      }
    }
    return result;
  }
}
