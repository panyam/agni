package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * This file defines PackService, which keeps the packs a server holds (agni issue 1704). A pack is
 * the rules a design is judged by: a naming convention, checklists, interface profiles, datasheet
 * parameters and shared intent. It holds what a `project.yaml` holds, as an AnalysisConfig, and it
 * ties to no design. A snapshot names the packs it is read under, and they layer over the design's own
 * config in the order it names them.
 * A pack is a resource (CONSTRAINTS C23). It outlives the call that made it, and a report records
 * which packs it applied and each one's hash, so a later read can say a pack changed since.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class PackServiceGrpc {

  private PackServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.PackService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreatePackRequest,
      io.github.panyam.agni.v1.webapi.Pack> getCreatePackMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CreatePack",
      requestType = io.github.panyam.agni.v1.webapi.CreatePackRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Pack.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreatePackRequest,
      io.github.panyam.agni.v1.webapi.Pack> getCreatePackMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreatePackRequest, io.github.panyam.agni.v1.webapi.Pack> getCreatePackMethod;
    if ((getCreatePackMethod = PackServiceGrpc.getCreatePackMethod) == null) {
      synchronized (PackServiceGrpc.class) {
        if ((getCreatePackMethod = PackServiceGrpc.getCreatePackMethod) == null) {
          PackServiceGrpc.getCreatePackMethod = getCreatePackMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.CreatePackRequest, io.github.panyam.agni.v1.webapi.Pack>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CreatePack"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.CreatePackRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Pack.getDefaultInstance()))
              .setSchemaDescriptor(new PackServiceMethodDescriptorSupplier("CreatePack"))
              .build();
        }
      }
    }
    return getCreatePackMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetPackRequest,
      io.github.panyam.agni.v1.webapi.Pack> getGetPackMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetPack",
      requestType = io.github.panyam.agni.v1.webapi.GetPackRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Pack.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetPackRequest,
      io.github.panyam.agni.v1.webapi.Pack> getGetPackMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetPackRequest, io.github.panyam.agni.v1.webapi.Pack> getGetPackMethod;
    if ((getGetPackMethod = PackServiceGrpc.getGetPackMethod) == null) {
      synchronized (PackServiceGrpc.class) {
        if ((getGetPackMethod = PackServiceGrpc.getGetPackMethod) == null) {
          PackServiceGrpc.getGetPackMethod = getGetPackMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetPackRequest, io.github.panyam.agni.v1.webapi.Pack>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetPack"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetPackRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Pack.getDefaultInstance()))
              .setSchemaDescriptor(new PackServiceMethodDescriptorSupplier("GetPack"))
              .build();
        }
      }
    }
    return getGetPackMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListPacksRequest,
      io.github.panyam.agni.v1.webapi.ListPacksResponse> getListPacksMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListPacks",
      requestType = io.github.panyam.agni.v1.webapi.ListPacksRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListPacksResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListPacksRequest,
      io.github.panyam.agni.v1.webapi.ListPacksResponse> getListPacksMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListPacksRequest, io.github.panyam.agni.v1.webapi.ListPacksResponse> getListPacksMethod;
    if ((getListPacksMethod = PackServiceGrpc.getListPacksMethod) == null) {
      synchronized (PackServiceGrpc.class) {
        if ((getListPacksMethod = PackServiceGrpc.getListPacksMethod) == null) {
          PackServiceGrpc.getListPacksMethod = getListPacksMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListPacksRequest, io.github.panyam.agni.v1.webapi.ListPacksResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListPacks"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListPacksRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListPacksResponse.getDefaultInstance()))
              .setSchemaDescriptor(new PackServiceMethodDescriptorSupplier("ListPacks"))
              .build();
        }
      }
    }
    return getListPacksMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdatePackRequest,
      io.github.panyam.agni.v1.webapi.Pack> getUpdatePackMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "UpdatePack",
      requestType = io.github.panyam.agni.v1.webapi.UpdatePackRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Pack.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdatePackRequest,
      io.github.panyam.agni.v1.webapi.Pack> getUpdatePackMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdatePackRequest, io.github.panyam.agni.v1.webapi.Pack> getUpdatePackMethod;
    if ((getUpdatePackMethod = PackServiceGrpc.getUpdatePackMethod) == null) {
      synchronized (PackServiceGrpc.class) {
        if ((getUpdatePackMethod = PackServiceGrpc.getUpdatePackMethod) == null) {
          PackServiceGrpc.getUpdatePackMethod = getUpdatePackMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.UpdatePackRequest, io.github.panyam.agni.v1.webapi.Pack>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "UpdatePack"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.UpdatePackRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Pack.getDefaultInstance()))
              .setSchemaDescriptor(new PackServiceMethodDescriptorSupplier("UpdatePack"))
              .build();
        }
      }
    }
    return getUpdatePackMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeletePackRequest,
      com.google.protobuf.Empty> getDeletePackMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "DeletePack",
      requestType = io.github.panyam.agni.v1.webapi.DeletePackRequest.class,
      responseType = com.google.protobuf.Empty.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeletePackRequest,
      com.google.protobuf.Empty> getDeletePackMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeletePackRequest, com.google.protobuf.Empty> getDeletePackMethod;
    if ((getDeletePackMethod = PackServiceGrpc.getDeletePackMethod) == null) {
      synchronized (PackServiceGrpc.class) {
        if ((getDeletePackMethod = PackServiceGrpc.getDeletePackMethod) == null) {
          PackServiceGrpc.getDeletePackMethod = getDeletePackMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.DeletePackRequest, com.google.protobuf.Empty>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "DeletePack"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.DeletePackRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.google.protobuf.Empty.getDefaultInstance()))
              .setSchemaDescriptor(new PackServiceMethodDescriptorSupplier("DeletePack"))
              .build();
        }
      }
    }
    return getDeletePackMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static PackServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PackServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PackServiceStub>() {
        @java.lang.Override
        public PackServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PackServiceStub(channel, callOptions);
        }
      };
    return PackServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static PackServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PackServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PackServiceBlockingV2Stub>() {
        @java.lang.Override
        public PackServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PackServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return PackServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static PackServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PackServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PackServiceBlockingStub>() {
        @java.lang.Override
        public PackServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PackServiceBlockingStub(channel, callOptions);
        }
      };
    return PackServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static PackServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PackServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PackServiceFutureStub>() {
        @java.lang.Override
        public PackServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PackServiceFutureStub(channel, callOptions);
        }
      };
    return PackServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * This file defines PackService, which keeps the packs a server holds (agni issue 1704). A pack is
   * the rules a design is judged by: a naming convention, checklists, interface profiles, datasheet
   * parameters and shared intent. It holds what a `project.yaml` holds, as an AnalysisConfig, and it
   * ties to no design. A snapshot names the packs it is read under, and they layer over the design's own
   * config in the order it names them.
   * A pack is a resource (CONSTRAINTS C23). It outlives the call that made it, and a report records
   * which packs it applied and each one's hash, so a later read can say a pack changed since.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * CreatePack keeps a new pack. An id a pack already holds is ALREADY_EXISTS. A server started
     * without --store answers FAILED_PRECONDITION. A pack with files needs --uploads too, and contents
     * the server lacks are FAILED_PRECONDITION with a BlobsMissing detail, as UpdateArtifact reports
     * them.
     * </pre>
     */
    default void createPack(io.github.panyam.agni.v1.webapi.CreatePackRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Pack> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCreatePackMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetPack returns one pack by resource name.
     * </pre>
     */
    default void getPack(io.github.panyam.agni.v1.webapi.GetPackRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Pack> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetPackMethod(), responseObserver);
    }

    /**
     * <pre>
     * ListPacks returns the packs this server holds, ordered by name.
     * </pre>
     */
    default void listPacks(io.github.panyam.agni.v1.webapi.ListPacksRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListPacksResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListPacksMethod(), responseObserver);
    }

    /**
     * <pre>
     * UpdatePack changes a pack's title, config or files, the fields update_mask names. A pack's hash
     * moves whenever its config or its files change.
     * </pre>
     */
    default void updatePack(io.github.panyam.agni.v1.webapi.UpdatePackRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Pack> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getUpdatePackMethod(), responseObserver);
    }

    /**
     * <pre>
     * DeletePack removes a pack. A snapshot that names it then fails to read, naming the pack.
     * </pre>
     */
    default void deletePack(io.github.panyam.agni.v1.webapi.DeletePackRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeletePackMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service PackService.
   * <pre>
   * This file defines PackService, which keeps the packs a server holds (agni issue 1704). A pack is
   * the rules a design is judged by: a naming convention, checklists, interface profiles, datasheet
   * parameters and shared intent. It holds what a `project.yaml` holds, as an AnalysisConfig, and it
   * ties to no design. A snapshot names the packs it is read under, and they layer over the design's own
   * config in the order it names them.
   * A pack is a resource (CONSTRAINTS C23). It outlives the call that made it, and a report records
   * which packs it applied and each one's hash, so a later read can say a pack changed since.
   * </pre>
   */
  public static abstract class PackServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return PackServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service PackService.
   * <pre>
   * This file defines PackService, which keeps the packs a server holds (agni issue 1704). A pack is
   * the rules a design is judged by: a naming convention, checklists, interface profiles, datasheet
   * parameters and shared intent. It holds what a `project.yaml` holds, as an AnalysisConfig, and it
   * ties to no design. A snapshot names the packs it is read under, and they layer over the design's own
   * config in the order it names them.
   * A pack is a resource (CONSTRAINTS C23). It outlives the call that made it, and a report records
   * which packs it applied and each one's hash, so a later read can say a pack changed since.
   * </pre>
   */
  public static final class PackServiceStub
      extends io.grpc.stub.AbstractAsyncStub<PackServiceStub> {
    private PackServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PackServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PackServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * CreatePack keeps a new pack. An id a pack already holds is ALREADY_EXISTS. A server started
     * without --store answers FAILED_PRECONDITION. A pack with files needs --uploads too, and contents
     * the server lacks are FAILED_PRECONDITION with a BlobsMissing detail, as UpdateArtifact reports
     * them.
     * </pre>
     */
    public void createPack(io.github.panyam.agni.v1.webapi.CreatePackRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Pack> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCreatePackMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetPack returns one pack by resource name.
     * </pre>
     */
    public void getPack(io.github.panyam.agni.v1.webapi.GetPackRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Pack> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetPackMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ListPacks returns the packs this server holds, ordered by name.
     * </pre>
     */
    public void listPacks(io.github.panyam.agni.v1.webapi.ListPacksRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListPacksResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListPacksMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * UpdatePack changes a pack's title, config or files, the fields update_mask names. A pack's hash
     * moves whenever its config or its files change.
     * </pre>
     */
    public void updatePack(io.github.panyam.agni.v1.webapi.UpdatePackRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Pack> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getUpdatePackMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * DeletePack removes a pack. A snapshot that names it then fails to read, naming the pack.
     * </pre>
     */
    public void deletePack(io.github.panyam.agni.v1.webapi.DeletePackRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeletePackMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service PackService.
   * <pre>
   * This file defines PackService, which keeps the packs a server holds (agni issue 1704). A pack is
   * the rules a design is judged by: a naming convention, checklists, interface profiles, datasheet
   * parameters and shared intent. It holds what a `project.yaml` holds, as an AnalysisConfig, and it
   * ties to no design. A snapshot names the packs it is read under, and they layer over the design's own
   * config in the order it names them.
   * A pack is a resource (CONSTRAINTS C23). It outlives the call that made it, and a report records
   * which packs it applied and each one's hash, so a later read can say a pack changed since.
   * </pre>
   */
  public static final class PackServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<PackServiceBlockingV2Stub> {
    private PackServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PackServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PackServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * CreatePack keeps a new pack. An id a pack already holds is ALREADY_EXISTS. A server started
     * without --store answers FAILED_PRECONDITION. A pack with files needs --uploads too, and contents
     * the server lacks are FAILED_PRECONDITION with a BlobsMissing detail, as UpdateArtifact reports
     * them.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Pack createPack(io.github.panyam.agni.v1.webapi.CreatePackRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getCreatePackMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetPack returns one pack by resource name.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Pack getPack(io.github.panyam.agni.v1.webapi.GetPackRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetPackMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListPacks returns the packs this server holds, ordered by name.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListPacksResponse listPacks(io.github.panyam.agni.v1.webapi.ListPacksRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListPacksMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * UpdatePack changes a pack's title, config or files, the fields update_mask names. A pack's hash
     * moves whenever its config or its files change.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Pack updatePack(io.github.panyam.agni.v1.webapi.UpdatePackRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getUpdatePackMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeletePack removes a pack. A snapshot that names it then fails to read, naming the pack.
     * </pre>
     */
    public com.google.protobuf.Empty deletePack(io.github.panyam.agni.v1.webapi.DeletePackRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getDeletePackMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service PackService.
   * <pre>
   * This file defines PackService, which keeps the packs a server holds (agni issue 1704). A pack is
   * the rules a design is judged by: a naming convention, checklists, interface profiles, datasheet
   * parameters and shared intent. It holds what a `project.yaml` holds, as an AnalysisConfig, and it
   * ties to no design. A snapshot names the packs it is read under, and they layer over the design's own
   * config in the order it names them.
   * A pack is a resource (CONSTRAINTS C23). It outlives the call that made it, and a report records
   * which packs it applied and each one's hash, so a later read can say a pack changed since.
   * </pre>
   */
  public static final class PackServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<PackServiceBlockingStub> {
    private PackServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PackServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PackServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * CreatePack keeps a new pack. An id a pack already holds is ALREADY_EXISTS. A server started
     * without --store answers FAILED_PRECONDITION. A pack with files needs --uploads too, and contents
     * the server lacks are FAILED_PRECONDITION with a BlobsMissing detail, as UpdateArtifact reports
     * them.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Pack createPack(io.github.panyam.agni.v1.webapi.CreatePackRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCreatePackMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetPack returns one pack by resource name.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Pack getPack(io.github.panyam.agni.v1.webapi.GetPackRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetPackMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListPacks returns the packs this server holds, ordered by name.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListPacksResponse listPacks(io.github.panyam.agni.v1.webapi.ListPacksRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListPacksMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * UpdatePack changes a pack's title, config or files, the fields update_mask names. A pack's hash
     * moves whenever its config or its files change.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Pack updatePack(io.github.panyam.agni.v1.webapi.UpdatePackRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getUpdatePackMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeletePack removes a pack. A snapshot that names it then fails to read, naming the pack.
     * </pre>
     */
    public com.google.protobuf.Empty deletePack(io.github.panyam.agni.v1.webapi.DeletePackRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeletePackMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service PackService.
   * <pre>
   * This file defines PackService, which keeps the packs a server holds (agni issue 1704). A pack is
   * the rules a design is judged by: a naming convention, checklists, interface profiles, datasheet
   * parameters and shared intent. It holds what a `project.yaml` holds, as an AnalysisConfig, and it
   * ties to no design. A snapshot names the packs it is read under, and they layer over the design's own
   * config in the order it names them.
   * A pack is a resource (CONSTRAINTS C23). It outlives the call that made it, and a report records
   * which packs it applied and each one's hash, so a later read can say a pack changed since.
   * </pre>
   */
  public static final class PackServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<PackServiceFutureStub> {
    private PackServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PackServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PackServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * CreatePack keeps a new pack. An id a pack already holds is ALREADY_EXISTS. A server started
     * without --store answers FAILED_PRECONDITION. A pack with files needs --uploads too, and contents
     * the server lacks are FAILED_PRECONDITION with a BlobsMissing detail, as UpdateArtifact reports
     * them.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Pack> createPack(
        io.github.panyam.agni.v1.webapi.CreatePackRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCreatePackMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetPack returns one pack by resource name.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Pack> getPack(
        io.github.panyam.agni.v1.webapi.GetPackRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetPackMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ListPacks returns the packs this server holds, ordered by name.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListPacksResponse> listPacks(
        io.github.panyam.agni.v1.webapi.ListPacksRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListPacksMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * UpdatePack changes a pack's title, config or files, the fields update_mask names. A pack's hash
     * moves whenever its config or its files change.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Pack> updatePack(
        io.github.panyam.agni.v1.webapi.UpdatePackRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getUpdatePackMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * DeletePack removes a pack. A snapshot that names it then fails to read, naming the pack.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.google.protobuf.Empty> deletePack(
        io.github.panyam.agni.v1.webapi.DeletePackRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeletePackMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_CREATE_PACK = 0;
  private static final int METHODID_GET_PACK = 1;
  private static final int METHODID_LIST_PACKS = 2;
  private static final int METHODID_UPDATE_PACK = 3;
  private static final int METHODID_DELETE_PACK = 4;

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
        case METHODID_CREATE_PACK:
          serviceImpl.createPack((io.github.panyam.agni.v1.webapi.CreatePackRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Pack>) responseObserver);
          break;
        case METHODID_GET_PACK:
          serviceImpl.getPack((io.github.panyam.agni.v1.webapi.GetPackRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Pack>) responseObserver);
          break;
        case METHODID_LIST_PACKS:
          serviceImpl.listPacks((io.github.panyam.agni.v1.webapi.ListPacksRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListPacksResponse>) responseObserver);
          break;
        case METHODID_UPDATE_PACK:
          serviceImpl.updatePack((io.github.panyam.agni.v1.webapi.UpdatePackRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Pack>) responseObserver);
          break;
        case METHODID_DELETE_PACK:
          serviceImpl.deletePack((io.github.panyam.agni.v1.webapi.DeletePackRequest) request,
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
          getCreatePackMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.CreatePackRequest,
              io.github.panyam.agni.v1.webapi.Pack>(
                service, METHODID_CREATE_PACK)))
        .addMethod(
          getGetPackMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetPackRequest,
              io.github.panyam.agni.v1.webapi.Pack>(
                service, METHODID_GET_PACK)))
        .addMethod(
          getListPacksMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListPacksRequest,
              io.github.panyam.agni.v1.webapi.ListPacksResponse>(
                service, METHODID_LIST_PACKS)))
        .addMethod(
          getUpdatePackMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.UpdatePackRequest,
              io.github.panyam.agni.v1.webapi.Pack>(
                service, METHODID_UPDATE_PACK)))
        .addMethod(
          getDeletePackMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.DeletePackRequest,
              com.google.protobuf.Empty>(
                service, METHODID_DELETE_PACK)))
        .build();
  }

  private static abstract class PackServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    PackServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.PacksProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("PackService");
    }
  }

  private static final class PackServiceFileDescriptorSupplier
      extends PackServiceBaseDescriptorSupplier {
    PackServiceFileDescriptorSupplier() {}
  }

  private static final class PackServiceMethodDescriptorSupplier
      extends PackServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    PackServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (PackServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new PackServiceFileDescriptorSupplier())
              .addMethod(getCreatePackMethod())
              .addMethod(getGetPackMethod())
              .addMethod(getListPacksMethod())
              .addMethod(getUpdatePackMethod())
              .addMethod(getDeletePackMethod())
              .build();
        }
      }
    }
    return result;
  }
}
