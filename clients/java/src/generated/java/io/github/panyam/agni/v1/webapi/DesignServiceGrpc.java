package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * This file defines DesignService, which keeps the designs a server holds (agni issue 1694). A
 * design is a set of files with a `design.yaml` among them, named "designs/{design}". It ties to no
 * project, and a project lists it by name. DesignViewService (design_view.proto) answers questions
 * about a snapshot of a design instead.
 * A server holds designs in two kinds of place. The folder named by `agni serve --store-dir` keeps
 * the designs callers create, and they can be replaced and deleted. Mounted folders declare designs
 * too, and those are read-only. A design id is unique across both.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class DesignServiceGrpc {

  private DesignServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.DesignService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateDesignRequest,
      io.github.panyam.agni.v1.webapi.Design> getCreateDesignMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CreateDesign",
      requestType = io.github.panyam.agni.v1.webapi.CreateDesignRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Design.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateDesignRequest,
      io.github.panyam.agni.v1.webapi.Design> getCreateDesignMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateDesignRequest, io.github.panyam.agni.v1.webapi.Design> getCreateDesignMethod;
    if ((getCreateDesignMethod = DesignServiceGrpc.getCreateDesignMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getCreateDesignMethod = DesignServiceGrpc.getCreateDesignMethod) == null) {
          DesignServiceGrpc.getCreateDesignMethod = getCreateDesignMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.CreateDesignRequest, io.github.panyam.agni.v1.webapi.Design>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CreateDesign"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.CreateDesignRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Design.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("CreateDesign"))
              .build();
        }
      }
    }
    return getCreateDesignMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetDesignRequest,
      io.github.panyam.agni.v1.webapi.Design> getGetDesignMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetDesign",
      requestType = io.github.panyam.agni.v1.webapi.GetDesignRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Design.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetDesignRequest,
      io.github.panyam.agni.v1.webapi.Design> getGetDesignMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetDesignRequest, io.github.panyam.agni.v1.webapi.Design> getGetDesignMethod;
    if ((getGetDesignMethod = DesignServiceGrpc.getGetDesignMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getGetDesignMethod = DesignServiceGrpc.getGetDesignMethod) == null) {
          DesignServiceGrpc.getGetDesignMethod = getGetDesignMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetDesignRequest, io.github.panyam.agni.v1.webapi.Design>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetDesign"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetDesignRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Design.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("GetDesign"))
              .build();
        }
      }
    }
    return getGetDesignMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListDesignsRequest,
      io.github.panyam.agni.v1.webapi.ListDesignsResponse> getListDesignsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListDesigns",
      requestType = io.github.panyam.agni.v1.webapi.ListDesignsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListDesignsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListDesignsRequest,
      io.github.panyam.agni.v1.webapi.ListDesignsResponse> getListDesignsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListDesignsRequest, io.github.panyam.agni.v1.webapi.ListDesignsResponse> getListDesignsMethod;
    if ((getListDesignsMethod = DesignServiceGrpc.getListDesignsMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getListDesignsMethod = DesignServiceGrpc.getListDesignsMethod) == null) {
          DesignServiceGrpc.getListDesignsMethod = getListDesignsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListDesignsRequest, io.github.panyam.agni.v1.webapi.ListDesignsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListDesigns"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListDesignsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListDesignsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("ListDesigns"))
              .build();
        }
      }
    }
    return getListDesignsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdateDesignRequest,
      io.github.panyam.agni.v1.webapi.Design> getUpdateDesignMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "UpdateDesign",
      requestType = io.github.panyam.agni.v1.webapi.UpdateDesignRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Design.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdateDesignRequest,
      io.github.panyam.agni.v1.webapi.Design> getUpdateDesignMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdateDesignRequest, io.github.panyam.agni.v1.webapi.Design> getUpdateDesignMethod;
    if ((getUpdateDesignMethod = DesignServiceGrpc.getUpdateDesignMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getUpdateDesignMethod = DesignServiceGrpc.getUpdateDesignMethod) == null) {
          DesignServiceGrpc.getUpdateDesignMethod = getUpdateDesignMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.UpdateDesignRequest, io.github.panyam.agni.v1.webapi.Design>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "UpdateDesign"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.UpdateDesignRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Design.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("UpdateDesign"))
              .build();
        }
      }
    }
    return getUpdateDesignMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteDesignRequest,
      com.google.protobuf.Empty> getDeleteDesignMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "DeleteDesign",
      requestType = io.github.panyam.agni.v1.webapi.DeleteDesignRequest.class,
      responseType = com.google.protobuf.Empty.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteDesignRequest,
      com.google.protobuf.Empty> getDeleteDesignMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteDesignRequest, com.google.protobuf.Empty> getDeleteDesignMethod;
    if ((getDeleteDesignMethod = DesignServiceGrpc.getDeleteDesignMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getDeleteDesignMethod = DesignServiceGrpc.getDeleteDesignMethod) == null) {
          DesignServiceGrpc.getDeleteDesignMethod = getDeleteDesignMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.DeleteDesignRequest, com.google.protobuf.Empty>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "DeleteDesign"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.DeleteDesignRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.google.protobuf.Empty.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("DeleteDesign"))
              .build();
        }
      }
    }
    return getDeleteDesignMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static DesignServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DesignServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DesignServiceStub>() {
        @java.lang.Override
        public DesignServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DesignServiceStub(channel, callOptions);
        }
      };
    return DesignServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static DesignServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DesignServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DesignServiceBlockingV2Stub>() {
        @java.lang.Override
        public DesignServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DesignServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return DesignServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static DesignServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DesignServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DesignServiceBlockingStub>() {
        @java.lang.Override
        public DesignServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DesignServiceBlockingStub(channel, callOptions);
        }
      };
    return DesignServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static DesignServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DesignServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DesignServiceFutureStub>() {
        @java.lang.Override
        public DesignServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DesignServiceFutureStub(channel, callOptions);
        }
      };
    return DesignServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * This file defines DesignService, which keeps the designs a server holds (agni issue 1694). A
   * design is a set of files with a `design.yaml` among them, named "designs/{design}". It ties to no
   * project, and a project lists it by name. DesignViewService (design_view.proto) answers questions
   * about a snapshot of a design instead.
   * A server holds designs in two kinds of place. The folder named by `agni serve --store-dir` keeps
   * the designs callers create, and they can be replaced and deleted. Mounted folders declare designs
   * too, and those are read-only. A design id is unique across both.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * CreateDesign stores a new design from its files. The files are a manifest of paths and hashes.
     * Contents the server lacks are FAILED_PRECONDITION with a BlobsMissing detail naming their hashes,
     * and the caller sends them with SnapshotService.UploadBlobs and asks again. The `design.yaml`
     * among the files sets the title and series. A `name` in it naming a different id is
     * INVALID_ARGUMENT, and so is an `extends`, since a stored design stands alone. An id any design
     * already holds is ALREADY_EXISTS. A server started without --store-dir answers
     * FAILED_PRECONDITION.
     * </pre>
     */
    default void createDesign(io.github.panyam.agni.v1.webapi.CreateDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Design> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCreateDesignMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetDesign returns one design by resource name. An id two mounted folders both declare is
     * FAILED_PRECONDITION naming both, since answering with either could describe the wrong board.
     * </pre>
     */
    default void getDesign(io.github.panyam.agni.v1.webapi.GetDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Design> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetDesignMethod(), responseObserver);
    }

    /**
     * <pre>
     * ListDesigns returns the designs this server holds, ordered by resource name.
     * </pre>
     */
    default void listDesigns(io.github.panyam.agni.v1.webapi.ListDesignsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListDesignsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListDesignsMethod(), responseObserver);
    }

    /**
     * <pre>
     * UpdateDesign replaces a stored design's whole file set, on the terms CreateDesign states. A
     * design declared in a mounted folder is read-only, and a write to it is FAILED_PRECONDITION.
     * </pre>
     */
    default void updateDesign(io.github.panyam.agni.v1.webapi.UpdateDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Design> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getUpdateDesignMethod(), responseObserver);
    }

    /**
     * <pre>
     * DeleteDesign removes a stored design and its files. A project listing it keeps the name.
     * </pre>
     */
    default void deleteDesign(io.github.panyam.agni.v1.webapi.DeleteDesignRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeleteDesignMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service DesignService.
   * <pre>
   * This file defines DesignService, which keeps the designs a server holds (agni issue 1694). A
   * design is a set of files with a `design.yaml` among them, named "designs/{design}". It ties to no
   * project, and a project lists it by name. DesignViewService (design_view.proto) answers questions
   * about a snapshot of a design instead.
   * A server holds designs in two kinds of place. The folder named by `agni serve --store-dir` keeps
   * the designs callers create, and they can be replaced and deleted. Mounted folders declare designs
   * too, and those are read-only. A design id is unique across both.
   * </pre>
   */
  public static abstract class DesignServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return DesignServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service DesignService.
   * <pre>
   * This file defines DesignService, which keeps the designs a server holds (agni issue 1694). A
   * design is a set of files with a `design.yaml` among them, named "designs/{design}". It ties to no
   * project, and a project lists it by name. DesignViewService (design_view.proto) answers questions
   * about a snapshot of a design instead.
   * A server holds designs in two kinds of place. The folder named by `agni serve --store-dir` keeps
   * the designs callers create, and they can be replaced and deleted. Mounted folders declare designs
   * too, and those are read-only. A design id is unique across both.
   * </pre>
   */
  public static final class DesignServiceStub
      extends io.grpc.stub.AbstractAsyncStub<DesignServiceStub> {
    private DesignServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DesignServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DesignServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * CreateDesign stores a new design from its files. The files are a manifest of paths and hashes.
     * Contents the server lacks are FAILED_PRECONDITION with a BlobsMissing detail naming their hashes,
     * and the caller sends them with SnapshotService.UploadBlobs and asks again. The `design.yaml`
     * among the files sets the title and series. A `name` in it naming a different id is
     * INVALID_ARGUMENT, and so is an `extends`, since a stored design stands alone. An id any design
     * already holds is ALREADY_EXISTS. A server started without --store-dir answers
     * FAILED_PRECONDITION.
     * </pre>
     */
    public void createDesign(io.github.panyam.agni.v1.webapi.CreateDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Design> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCreateDesignMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetDesign returns one design by resource name. An id two mounted folders both declare is
     * FAILED_PRECONDITION naming both, since answering with either could describe the wrong board.
     * </pre>
     */
    public void getDesign(io.github.panyam.agni.v1.webapi.GetDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Design> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetDesignMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ListDesigns returns the designs this server holds, ordered by resource name.
     * </pre>
     */
    public void listDesigns(io.github.panyam.agni.v1.webapi.ListDesignsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListDesignsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListDesignsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * UpdateDesign replaces a stored design's whole file set, on the terms CreateDesign states. A
     * design declared in a mounted folder is read-only, and a write to it is FAILED_PRECONDITION.
     * </pre>
     */
    public void updateDesign(io.github.panyam.agni.v1.webapi.UpdateDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Design> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getUpdateDesignMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * DeleteDesign removes a stored design and its files. A project listing it keeps the name.
     * </pre>
     */
    public void deleteDesign(io.github.panyam.agni.v1.webapi.DeleteDesignRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeleteDesignMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service DesignService.
   * <pre>
   * This file defines DesignService, which keeps the designs a server holds (agni issue 1694). A
   * design is a set of files with a `design.yaml` among them, named "designs/{design}". It ties to no
   * project, and a project lists it by name. DesignViewService (design_view.proto) answers questions
   * about a snapshot of a design instead.
   * A server holds designs in two kinds of place. The folder named by `agni serve --store-dir` keeps
   * the designs callers create, and they can be replaced and deleted. Mounted folders declare designs
   * too, and those are read-only. A design id is unique across both.
   * </pre>
   */
  public static final class DesignServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<DesignServiceBlockingV2Stub> {
    private DesignServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DesignServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DesignServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * CreateDesign stores a new design from its files. The files are a manifest of paths and hashes.
     * Contents the server lacks are FAILED_PRECONDITION with a BlobsMissing detail naming their hashes,
     * and the caller sends them with SnapshotService.UploadBlobs and asks again. The `design.yaml`
     * among the files sets the title and series. A `name` in it naming a different id is
     * INVALID_ARGUMENT, and so is an `extends`, since a stored design stands alone. An id any design
     * already holds is ALREADY_EXISTS. A server started without --store-dir answers
     * FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Design createDesign(io.github.panyam.agni.v1.webapi.CreateDesignRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getCreateDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetDesign returns one design by resource name. An id two mounted folders both declare is
     * FAILED_PRECONDITION naming both, since answering with either could describe the wrong board.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Design getDesign(io.github.panyam.agni.v1.webapi.GetDesignRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListDesigns returns the designs this server holds, ordered by resource name.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListDesignsResponse listDesigns(io.github.panyam.agni.v1.webapi.ListDesignsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListDesignsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * UpdateDesign replaces a stored design's whole file set, on the terms CreateDesign states. A
     * design declared in a mounted folder is read-only, and a write to it is FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Design updateDesign(io.github.panyam.agni.v1.webapi.UpdateDesignRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getUpdateDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteDesign removes a stored design and its files. A project listing it keeps the name.
     * </pre>
     */
    public com.google.protobuf.Empty deleteDesign(io.github.panyam.agni.v1.webapi.DeleteDesignRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getDeleteDesignMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service DesignService.
   * <pre>
   * This file defines DesignService, which keeps the designs a server holds (agni issue 1694). A
   * design is a set of files with a `design.yaml` among them, named "designs/{design}". It ties to no
   * project, and a project lists it by name. DesignViewService (design_view.proto) answers questions
   * about a snapshot of a design instead.
   * A server holds designs in two kinds of place. The folder named by `agni serve --store-dir` keeps
   * the designs callers create, and they can be replaced and deleted. Mounted folders declare designs
   * too, and those are read-only. A design id is unique across both.
   * </pre>
   */
  public static final class DesignServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<DesignServiceBlockingStub> {
    private DesignServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DesignServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DesignServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * CreateDesign stores a new design from its files. The files are a manifest of paths and hashes.
     * Contents the server lacks are FAILED_PRECONDITION with a BlobsMissing detail naming their hashes,
     * and the caller sends them with SnapshotService.UploadBlobs and asks again. The `design.yaml`
     * among the files sets the title and series. A `name` in it naming a different id is
     * INVALID_ARGUMENT, and so is an `extends`, since a stored design stands alone. An id any design
     * already holds is ALREADY_EXISTS. A server started without --store-dir answers
     * FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Design createDesign(io.github.panyam.agni.v1.webapi.CreateDesignRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCreateDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetDesign returns one design by resource name. An id two mounted folders both declare is
     * FAILED_PRECONDITION naming both, since answering with either could describe the wrong board.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Design getDesign(io.github.panyam.agni.v1.webapi.GetDesignRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListDesigns returns the designs this server holds, ordered by resource name.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListDesignsResponse listDesigns(io.github.panyam.agni.v1.webapi.ListDesignsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListDesignsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * UpdateDesign replaces a stored design's whole file set, on the terms CreateDesign states. A
     * design declared in a mounted folder is read-only, and a write to it is FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Design updateDesign(io.github.panyam.agni.v1.webapi.UpdateDesignRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getUpdateDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteDesign removes a stored design and its files. A project listing it keeps the name.
     * </pre>
     */
    public com.google.protobuf.Empty deleteDesign(io.github.panyam.agni.v1.webapi.DeleteDesignRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeleteDesignMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service DesignService.
   * <pre>
   * This file defines DesignService, which keeps the designs a server holds (agni issue 1694). A
   * design is a set of files with a `design.yaml` among them, named "designs/{design}". It ties to no
   * project, and a project lists it by name. DesignViewService (design_view.proto) answers questions
   * about a snapshot of a design instead.
   * A server holds designs in two kinds of place. The folder named by `agni serve --store-dir` keeps
   * the designs callers create, and they can be replaced and deleted. Mounted folders declare designs
   * too, and those are read-only. A design id is unique across both.
   * </pre>
   */
  public static final class DesignServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<DesignServiceFutureStub> {
    private DesignServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DesignServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DesignServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * CreateDesign stores a new design from its files. The files are a manifest of paths and hashes.
     * Contents the server lacks are FAILED_PRECONDITION with a BlobsMissing detail naming their hashes,
     * and the caller sends them with SnapshotService.UploadBlobs and asks again. The `design.yaml`
     * among the files sets the title and series. A `name` in it naming a different id is
     * INVALID_ARGUMENT, and so is an `extends`, since a stored design stands alone. An id any design
     * already holds is ALREADY_EXISTS. A server started without --store-dir answers
     * FAILED_PRECONDITION.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Design> createDesign(
        io.github.panyam.agni.v1.webapi.CreateDesignRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCreateDesignMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetDesign returns one design by resource name. An id two mounted folders both declare is
     * FAILED_PRECONDITION naming both, since answering with either could describe the wrong board.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Design> getDesign(
        io.github.panyam.agni.v1.webapi.GetDesignRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetDesignMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ListDesigns returns the designs this server holds, ordered by resource name.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListDesignsResponse> listDesigns(
        io.github.panyam.agni.v1.webapi.ListDesignsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListDesignsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * UpdateDesign replaces a stored design's whole file set, on the terms CreateDesign states. A
     * design declared in a mounted folder is read-only, and a write to it is FAILED_PRECONDITION.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Design> updateDesign(
        io.github.panyam.agni.v1.webapi.UpdateDesignRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getUpdateDesignMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * DeleteDesign removes a stored design and its files. A project listing it keeps the name.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.google.protobuf.Empty> deleteDesign(
        io.github.panyam.agni.v1.webapi.DeleteDesignRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeleteDesignMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_CREATE_DESIGN = 0;
  private static final int METHODID_GET_DESIGN = 1;
  private static final int METHODID_LIST_DESIGNS = 2;
  private static final int METHODID_UPDATE_DESIGN = 3;
  private static final int METHODID_DELETE_DESIGN = 4;

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
        case METHODID_CREATE_DESIGN:
          serviceImpl.createDesign((io.github.panyam.agni.v1.webapi.CreateDesignRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Design>) responseObserver);
          break;
        case METHODID_GET_DESIGN:
          serviceImpl.getDesign((io.github.panyam.agni.v1.webapi.GetDesignRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Design>) responseObserver);
          break;
        case METHODID_LIST_DESIGNS:
          serviceImpl.listDesigns((io.github.panyam.agni.v1.webapi.ListDesignsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListDesignsResponse>) responseObserver);
          break;
        case METHODID_UPDATE_DESIGN:
          serviceImpl.updateDesign((io.github.panyam.agni.v1.webapi.UpdateDesignRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Design>) responseObserver);
          break;
        case METHODID_DELETE_DESIGN:
          serviceImpl.deleteDesign((io.github.panyam.agni.v1.webapi.DeleteDesignRequest) request,
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
          getCreateDesignMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.CreateDesignRequest,
              io.github.panyam.agni.v1.webapi.Design>(
                service, METHODID_CREATE_DESIGN)))
        .addMethod(
          getGetDesignMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetDesignRequest,
              io.github.panyam.agni.v1.webapi.Design>(
                service, METHODID_GET_DESIGN)))
        .addMethod(
          getListDesignsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListDesignsRequest,
              io.github.panyam.agni.v1.webapi.ListDesignsResponse>(
                service, METHODID_LIST_DESIGNS)))
        .addMethod(
          getUpdateDesignMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.UpdateDesignRequest,
              io.github.panyam.agni.v1.webapi.Design>(
                service, METHODID_UPDATE_DESIGN)))
        .addMethod(
          getDeleteDesignMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.DeleteDesignRequest,
              com.google.protobuf.Empty>(
                service, METHODID_DELETE_DESIGN)))
        .build();
  }

  private static abstract class DesignServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    DesignServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.DesignsProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("DesignService");
    }
  }

  private static final class DesignServiceFileDescriptorSupplier
      extends DesignServiceBaseDescriptorSupplier {
    DesignServiceFileDescriptorSupplier() {}
  }

  private static final class DesignServiceMethodDescriptorSupplier
      extends DesignServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    DesignServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (DesignServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new DesignServiceFileDescriptorSupplier())
              .addMethod(getCreateDesignMethod())
              .addMethod(getGetDesignMethod())
              .addMethod(getListDesignsMethod())
              .addMethod(getUpdateDesignMethod())
              .addMethod(getDeleteDesignMethod())
              .build();
        }
      }
    }
    return result;
  }
}
