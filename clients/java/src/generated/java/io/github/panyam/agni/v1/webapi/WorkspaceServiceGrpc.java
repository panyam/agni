package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * WorkspaceService exposes the server's read-only folders ("mounts") and their directory contents.
 * The server is started with the mounts. Mounts are the security boundary. A client names an
 * artifact by a `mount://&lt;mount&gt;/&lt;path&gt;` URI. The URI's authority is a mount, never an absolute
 * host path.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class WorkspaceServiceGrpc {

  private WorkspaceServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.WorkspaceService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListMountsRequest,
      io.github.panyam.agni.v1.webapi.ListMountsResponse> getListMountsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListMounts",
      requestType = io.github.panyam.agni.v1.webapi.ListMountsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListMountsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListMountsRequest,
      io.github.panyam.agni.v1.webapi.ListMountsResponse> getListMountsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListMountsRequest, io.github.panyam.agni.v1.webapi.ListMountsResponse> getListMountsMethod;
    if ((getListMountsMethod = WorkspaceServiceGrpc.getListMountsMethod) == null) {
      synchronized (WorkspaceServiceGrpc.class) {
        if ((getListMountsMethod = WorkspaceServiceGrpc.getListMountsMethod) == null) {
          WorkspaceServiceGrpc.getListMountsMethod = getListMountsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListMountsRequest, io.github.panyam.agni.v1.webapi.ListMountsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListMounts"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListMountsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListMountsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new WorkspaceServiceMethodDescriptorSupplier("ListMounts"))
              .build();
        }
      }
    }
    return getListMountsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetMountRequest,
      io.github.panyam.agni.v1.webapi.Mount> getGetMountMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetMount",
      requestType = io.github.panyam.agni.v1.webapi.GetMountRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Mount.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetMountRequest,
      io.github.panyam.agni.v1.webapi.Mount> getGetMountMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetMountRequest, io.github.panyam.agni.v1.webapi.Mount> getGetMountMethod;
    if ((getGetMountMethod = WorkspaceServiceGrpc.getGetMountMethod) == null) {
      synchronized (WorkspaceServiceGrpc.class) {
        if ((getGetMountMethod = WorkspaceServiceGrpc.getGetMountMethod) == null) {
          WorkspaceServiceGrpc.getGetMountMethod = getGetMountMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetMountRequest, io.github.panyam.agni.v1.webapi.Mount>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetMount"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetMountRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Mount.getDefaultInstance()))
              .setSchemaDescriptor(new WorkspaceServiceMethodDescriptorSupplier("GetMount"))
              .build();
        }
      }
    }
    return getGetMountMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListDirRequest,
      io.github.panyam.agni.v1.webapi.ListDirResponse> getListDirMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListDir",
      requestType = io.github.panyam.agni.v1.webapi.ListDirRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListDirResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListDirRequest,
      io.github.panyam.agni.v1.webapi.ListDirResponse> getListDirMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListDirRequest, io.github.panyam.agni.v1.webapi.ListDirResponse> getListDirMethod;
    if ((getListDirMethod = WorkspaceServiceGrpc.getListDirMethod) == null) {
      synchronized (WorkspaceServiceGrpc.class) {
        if ((getListDirMethod = WorkspaceServiceGrpc.getListDirMethod) == null) {
          WorkspaceServiceGrpc.getListDirMethod = getListDirMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListDirRequest, io.github.panyam.agni.v1.webapi.ListDirResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListDir"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListDirRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListDirResponse.getDefaultInstance()))
              .setSchemaDescriptor(new WorkspaceServiceMethodDescriptorSupplier("ListDir"))
              .build();
        }
      }
    }
    return getListDirMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListDesignFilesRequest,
      io.github.panyam.agni.v1.webapi.ListDesignFilesResponse> getListDesignFilesMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListDesignFiles",
      requestType = io.github.panyam.agni.v1.webapi.ListDesignFilesRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListDesignFilesResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListDesignFilesRequest,
      io.github.panyam.agni.v1.webapi.ListDesignFilesResponse> getListDesignFilesMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListDesignFilesRequest, io.github.panyam.agni.v1.webapi.ListDesignFilesResponse> getListDesignFilesMethod;
    if ((getListDesignFilesMethod = WorkspaceServiceGrpc.getListDesignFilesMethod) == null) {
      synchronized (WorkspaceServiceGrpc.class) {
        if ((getListDesignFilesMethod = WorkspaceServiceGrpc.getListDesignFilesMethod) == null) {
          WorkspaceServiceGrpc.getListDesignFilesMethod = getListDesignFilesMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListDesignFilesRequest, io.github.panyam.agni.v1.webapi.ListDesignFilesResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListDesignFiles"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListDesignFilesRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListDesignFilesResponse.getDefaultInstance()))
              .setSchemaDescriptor(new WorkspaceServiceMethodDescriptorSupplier("ListDesignFiles"))
              .build();
        }
      }
    }
    return getListDesignFilesMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ProposeDesignsRequest,
      io.github.panyam.agni.v1.webapi.ProposeDesignsResponse> getProposeDesignsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ProposeDesigns",
      requestType = io.github.panyam.agni.v1.webapi.ProposeDesignsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ProposeDesignsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ProposeDesignsRequest,
      io.github.panyam.agni.v1.webapi.ProposeDesignsResponse> getProposeDesignsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ProposeDesignsRequest, io.github.panyam.agni.v1.webapi.ProposeDesignsResponse> getProposeDesignsMethod;
    if ((getProposeDesignsMethod = WorkspaceServiceGrpc.getProposeDesignsMethod) == null) {
      synchronized (WorkspaceServiceGrpc.class) {
        if ((getProposeDesignsMethod = WorkspaceServiceGrpc.getProposeDesignsMethod) == null) {
          WorkspaceServiceGrpc.getProposeDesignsMethod = getProposeDesignsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ProposeDesignsRequest, io.github.panyam.agni.v1.webapi.ProposeDesignsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ProposeDesigns"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ProposeDesignsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ProposeDesignsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new WorkspaceServiceMethodDescriptorSupplier("ProposeDesigns"))
              .build();
        }
      }
    }
    return getProposeDesignsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeclareCompanionRequest,
      io.github.panyam.agni.v1.webapi.DeclareCompanionResponse> getDeclareCompanionMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "DeclareCompanion",
      requestType = io.github.panyam.agni.v1.webapi.DeclareCompanionRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.DeclareCompanionResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeclareCompanionRequest,
      io.github.panyam.agni.v1.webapi.DeclareCompanionResponse> getDeclareCompanionMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeclareCompanionRequest, io.github.panyam.agni.v1.webapi.DeclareCompanionResponse> getDeclareCompanionMethod;
    if ((getDeclareCompanionMethod = WorkspaceServiceGrpc.getDeclareCompanionMethod) == null) {
      synchronized (WorkspaceServiceGrpc.class) {
        if ((getDeclareCompanionMethod = WorkspaceServiceGrpc.getDeclareCompanionMethod) == null) {
          WorkspaceServiceGrpc.getDeclareCompanionMethod = getDeclareCompanionMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.DeclareCompanionRequest, io.github.panyam.agni.v1.webapi.DeclareCompanionResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "DeclareCompanion"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.DeclareCompanionRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.DeclareCompanionResponse.getDefaultInstance()))
              .setSchemaDescriptor(new WorkspaceServiceMethodDescriptorSupplier("DeclareCompanion"))
              .build();
        }
      }
    }
    return getDeclareCompanionMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static WorkspaceServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<WorkspaceServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<WorkspaceServiceStub>() {
        @java.lang.Override
        public WorkspaceServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new WorkspaceServiceStub(channel, callOptions);
        }
      };
    return WorkspaceServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static WorkspaceServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<WorkspaceServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<WorkspaceServiceBlockingV2Stub>() {
        @java.lang.Override
        public WorkspaceServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new WorkspaceServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return WorkspaceServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static WorkspaceServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<WorkspaceServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<WorkspaceServiceBlockingStub>() {
        @java.lang.Override
        public WorkspaceServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new WorkspaceServiceBlockingStub(channel, callOptions);
        }
      };
    return WorkspaceServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static WorkspaceServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<WorkspaceServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<WorkspaceServiceFutureStub>() {
        @java.lang.Override
        public WorkspaceServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new WorkspaceServiceFutureStub(channel, callOptions);
        }
      };
    return WorkspaceServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * WorkspaceService exposes the server's read-only folders ("mounts") and their directory contents.
   * The server is started with the mounts. Mounts are the security boundary. A client names an
   * artifact by a `mount://&lt;mount&gt;/&lt;path&gt;` URI. The URI's authority is a mount, never an absolute
   * host path.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * ListMounts returns the mounts the server is serving, in the order they were configured. It
     * never fails for a running server. Set opens to leave out each mount that holds nothing the
     * caller can open.
     * </pre>
     */
    default void listMounts(io.github.panyam.agni.v1.webapi.ListMountsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListMountsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListMountsMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetMount returns one mount by its name, the handle ListMounts gives (agni issue 1734). A page
     * that opens on a mount's address reads that mount alone. A name the server does not serve is
     * NOT_FOUND.
     * </pre>
     */
    default void getMount(io.github.panyam.agni.v1.webapi.GetMountRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Mount> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetMountMethod(), responseObserver);
    }

    /**
     * <pre>
     * ListDir lists one directory level inside a mount, with its subdirectories and its files. Each
     * file is labeled with the reader that would open it. The client addresses a location by an
     * artifact URI. The URI's authority is a mount, never an absolute host path, and the server
     * resolves it inside that mount. It is lazy, one level per call, so large trees remain
     * responsive. Set opens to leave out each subdirectory that holds nothing the caller can open,
     * at any depth.
     * </pre>
     */
    default void listDir(io.github.panyam.agni.v1.webapi.ListDirRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListDirResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListDirMethod(), responseObserver);
    }

    /**
     * <pre>
     * ListDesignFiles lists every file a design's analysis reads (agni issue 853). A client may
     * analyse the design somewhere else, and the in-browser engine is the first such client. It can
     * then fetch exactly those files and nothing more. The list covers the design's folder, its
     * project's descriptor, and every config directory the project names. The client fetches each
     * file by its path under the server's read-only `/raw/&lt;mount&gt;/&lt;path&gt;` route. A set larger than
     * the server's bound (256 MB) is refused with its size. A board that big belongs on the server's
     * own engine.
     * </pre>
     */
    default void listDesignFiles(io.github.panyam.agni.v1.webapi.ListDesignFilesRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListDesignFilesResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListDesignFilesMethod(), responseObserver);
    }

    /**
     * <pre>
     * ProposeDesigns takes a folder that declares no design, and groups its files into the designs
     * they make (agni issue 854). Imagine a page handed a drop of files. It must show what it will
     * read before it reads it. A folder already holding a `design.yaml` is reported as declared, and
     * the server does not guess again. Each proposal includes the `design.yaml` that would declare
     * it. Every file that no proposal reads is listed with the reason, so nothing is dropped silently.
     * </pre>
     */
    default void proposeDesigns(io.github.panyam.agni.v1.webapi.ProposeDesignsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ProposeDesignsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getProposeDesignsMethod(), responseObserver);
    }

    /**
     * <pre>
     * DeclareCompanion answers the design.yaml that declares a file as one of a design's companions,
     * such as a board a page attached with Snapshot.board_uri (agni issue 1367). It writes nothing.
     * A host whose mount a page can write, such as the browser's drop mount, saves the text, and on
     * any other the reader commits it. An existing descriptor keeps its comments and other keys, and
     * a new one names the design's entry and extends a project.yaml above it.
     * </pre>
     */
    default void declareCompanion(io.github.panyam.agni.v1.webapi.DeclareCompanionRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.DeclareCompanionResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeclareCompanionMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service WorkspaceService.
   * <pre>
   * WorkspaceService exposes the server's read-only folders ("mounts") and their directory contents.
   * The server is started with the mounts. Mounts are the security boundary. A client names an
   * artifact by a `mount://&lt;mount&gt;/&lt;path&gt;` URI. The URI's authority is a mount, never an absolute
   * host path.
   * </pre>
   */
  public static abstract class WorkspaceServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return WorkspaceServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service WorkspaceService.
   * <pre>
   * WorkspaceService exposes the server's read-only folders ("mounts") and their directory contents.
   * The server is started with the mounts. Mounts are the security boundary. A client names an
   * artifact by a `mount://&lt;mount&gt;/&lt;path&gt;` URI. The URI's authority is a mount, never an absolute
   * host path.
   * </pre>
   */
  public static final class WorkspaceServiceStub
      extends io.grpc.stub.AbstractAsyncStub<WorkspaceServiceStub> {
    private WorkspaceServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected WorkspaceServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new WorkspaceServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * ListMounts returns the mounts the server is serving, in the order they were configured. It
     * never fails for a running server. Set opens to leave out each mount that holds nothing the
     * caller can open.
     * </pre>
     */
    public void listMounts(io.github.panyam.agni.v1.webapi.ListMountsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListMountsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListMountsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetMount returns one mount by its name, the handle ListMounts gives (agni issue 1734). A page
     * that opens on a mount's address reads that mount alone. A name the server does not serve is
     * NOT_FOUND.
     * </pre>
     */
    public void getMount(io.github.panyam.agni.v1.webapi.GetMountRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Mount> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetMountMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ListDir lists one directory level inside a mount, with its subdirectories and its files. Each
     * file is labeled with the reader that would open it. The client addresses a location by an
     * artifact URI. The URI's authority is a mount, never an absolute host path, and the server
     * resolves it inside that mount. It is lazy, one level per call, so large trees remain
     * responsive. Set opens to leave out each subdirectory that holds nothing the caller can open,
     * at any depth.
     * </pre>
     */
    public void listDir(io.github.panyam.agni.v1.webapi.ListDirRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListDirResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListDirMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ListDesignFiles lists every file a design's analysis reads (agni issue 853). A client may
     * analyse the design somewhere else, and the in-browser engine is the first such client. It can
     * then fetch exactly those files and nothing more. The list covers the design's folder, its
     * project's descriptor, and every config directory the project names. The client fetches each
     * file by its path under the server's read-only `/raw/&lt;mount&gt;/&lt;path&gt;` route. A set larger than
     * the server's bound (256 MB) is refused with its size. A board that big belongs on the server's
     * own engine.
     * </pre>
     */
    public void listDesignFiles(io.github.panyam.agni.v1.webapi.ListDesignFilesRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListDesignFilesResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListDesignFilesMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ProposeDesigns takes a folder that declares no design, and groups its files into the designs
     * they make (agni issue 854). Imagine a page handed a drop of files. It must show what it will
     * read before it reads it. A folder already holding a `design.yaml` is reported as declared, and
     * the server does not guess again. Each proposal includes the `design.yaml` that would declare
     * it. Every file that no proposal reads is listed with the reason, so nothing is dropped silently.
     * </pre>
     */
    public void proposeDesigns(io.github.panyam.agni.v1.webapi.ProposeDesignsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ProposeDesignsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getProposeDesignsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * DeclareCompanion answers the design.yaml that declares a file as one of a design's companions,
     * such as a board a page attached with Snapshot.board_uri (agni issue 1367). It writes nothing.
     * A host whose mount a page can write, such as the browser's drop mount, saves the text, and on
     * any other the reader commits it. An existing descriptor keeps its comments and other keys, and
     * a new one names the design's entry and extends a project.yaml above it.
     * </pre>
     */
    public void declareCompanion(io.github.panyam.agni.v1.webapi.DeclareCompanionRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.DeclareCompanionResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeclareCompanionMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service WorkspaceService.
   * <pre>
   * WorkspaceService exposes the server's read-only folders ("mounts") and their directory contents.
   * The server is started with the mounts. Mounts are the security boundary. A client names an
   * artifact by a `mount://&lt;mount&gt;/&lt;path&gt;` URI. The URI's authority is a mount, never an absolute
   * host path.
   * </pre>
   */
  public static final class WorkspaceServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<WorkspaceServiceBlockingV2Stub> {
    private WorkspaceServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected WorkspaceServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new WorkspaceServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * ListMounts returns the mounts the server is serving, in the order they were configured. It
     * never fails for a running server. Set opens to leave out each mount that holds nothing the
     * caller can open.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListMountsResponse listMounts(io.github.panyam.agni.v1.webapi.ListMountsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListMountsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetMount returns one mount by its name, the handle ListMounts gives (agni issue 1734). A page
     * that opens on a mount's address reads that mount alone. A name the server does not serve is
     * NOT_FOUND.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Mount getMount(io.github.panyam.agni.v1.webapi.GetMountRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetMountMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListDir lists one directory level inside a mount, with its subdirectories and its files. Each
     * file is labeled with the reader that would open it. The client addresses a location by an
     * artifact URI. The URI's authority is a mount, never an absolute host path, and the server
     * resolves it inside that mount. It is lazy, one level per call, so large trees remain
     * responsive. Set opens to leave out each subdirectory that holds nothing the caller can open,
     * at any depth.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListDirResponse listDir(io.github.panyam.agni.v1.webapi.ListDirRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListDirMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListDesignFiles lists every file a design's analysis reads (agni issue 853). A client may
     * analyse the design somewhere else, and the in-browser engine is the first such client. It can
     * then fetch exactly those files and nothing more. The list covers the design's folder, its
     * project's descriptor, and every config directory the project names. The client fetches each
     * file by its path under the server's read-only `/raw/&lt;mount&gt;/&lt;path&gt;` route. A set larger than
     * the server's bound (256 MB) is refused with its size. A board that big belongs on the server's
     * own engine.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListDesignFilesResponse listDesignFiles(io.github.panyam.agni.v1.webapi.ListDesignFilesRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListDesignFilesMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ProposeDesigns takes a folder that declares no design, and groups its files into the designs
     * they make (agni issue 854). Imagine a page handed a drop of files. It must show what it will
     * read before it reads it. A folder already holding a `design.yaml` is reported as declared, and
     * the server does not guess again. Each proposal includes the `design.yaml` that would declare
     * it. Every file that no proposal reads is listed with the reason, so nothing is dropped silently.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ProposeDesignsResponse proposeDesigns(io.github.panyam.agni.v1.webapi.ProposeDesignsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getProposeDesignsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeclareCompanion answers the design.yaml that declares a file as one of a design's companions,
     * such as a board a page attached with Snapshot.board_uri (agni issue 1367). It writes nothing.
     * A host whose mount a page can write, such as the browser's drop mount, saves the text, and on
     * any other the reader commits it. An existing descriptor keeps its comments and other keys, and
     * a new one names the design's entry and extends a project.yaml above it.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.DeclareCompanionResponse declareCompanion(io.github.panyam.agni.v1.webapi.DeclareCompanionRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getDeclareCompanionMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service WorkspaceService.
   * <pre>
   * WorkspaceService exposes the server's read-only folders ("mounts") and their directory contents.
   * The server is started with the mounts. Mounts are the security boundary. A client names an
   * artifact by a `mount://&lt;mount&gt;/&lt;path&gt;` URI. The URI's authority is a mount, never an absolute
   * host path.
   * </pre>
   */
  public static final class WorkspaceServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<WorkspaceServiceBlockingStub> {
    private WorkspaceServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected WorkspaceServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new WorkspaceServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * ListMounts returns the mounts the server is serving, in the order they were configured. It
     * never fails for a running server. Set opens to leave out each mount that holds nothing the
     * caller can open.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListMountsResponse listMounts(io.github.panyam.agni.v1.webapi.ListMountsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListMountsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetMount returns one mount by its name, the handle ListMounts gives (agni issue 1734). A page
     * that opens on a mount's address reads that mount alone. A name the server does not serve is
     * NOT_FOUND.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Mount getMount(io.github.panyam.agni.v1.webapi.GetMountRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetMountMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListDir lists one directory level inside a mount, with its subdirectories and its files. Each
     * file is labeled with the reader that would open it. The client addresses a location by an
     * artifact URI. The URI's authority is a mount, never an absolute host path, and the server
     * resolves it inside that mount. It is lazy, one level per call, so large trees remain
     * responsive. Set opens to leave out each subdirectory that holds nothing the caller can open,
     * at any depth.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListDirResponse listDir(io.github.panyam.agni.v1.webapi.ListDirRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListDirMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListDesignFiles lists every file a design's analysis reads (agni issue 853). A client may
     * analyse the design somewhere else, and the in-browser engine is the first such client. It can
     * then fetch exactly those files and nothing more. The list covers the design's folder, its
     * project's descriptor, and every config directory the project names. The client fetches each
     * file by its path under the server's read-only `/raw/&lt;mount&gt;/&lt;path&gt;` route. A set larger than
     * the server's bound (256 MB) is refused with its size. A board that big belongs on the server's
     * own engine.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListDesignFilesResponse listDesignFiles(io.github.panyam.agni.v1.webapi.ListDesignFilesRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListDesignFilesMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ProposeDesigns takes a folder that declares no design, and groups its files into the designs
     * they make (agni issue 854). Imagine a page handed a drop of files. It must show what it will
     * read before it reads it. A folder already holding a `design.yaml` is reported as declared, and
     * the server does not guess again. Each proposal includes the `design.yaml` that would declare
     * it. Every file that no proposal reads is listed with the reason, so nothing is dropped silently.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ProposeDesignsResponse proposeDesigns(io.github.panyam.agni.v1.webapi.ProposeDesignsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getProposeDesignsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeclareCompanion answers the design.yaml that declares a file as one of a design's companions,
     * such as a board a page attached with Snapshot.board_uri (agni issue 1367). It writes nothing.
     * A host whose mount a page can write, such as the browser's drop mount, saves the text, and on
     * any other the reader commits it. An existing descriptor keeps its comments and other keys, and
     * a new one names the design's entry and extends a project.yaml above it.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.DeclareCompanionResponse declareCompanion(io.github.panyam.agni.v1.webapi.DeclareCompanionRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeclareCompanionMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service WorkspaceService.
   * <pre>
   * WorkspaceService exposes the server's read-only folders ("mounts") and their directory contents.
   * The server is started with the mounts. Mounts are the security boundary. A client names an
   * artifact by a `mount://&lt;mount&gt;/&lt;path&gt;` URI. The URI's authority is a mount, never an absolute
   * host path.
   * </pre>
   */
  public static final class WorkspaceServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<WorkspaceServiceFutureStub> {
    private WorkspaceServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected WorkspaceServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new WorkspaceServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * ListMounts returns the mounts the server is serving, in the order they were configured. It
     * never fails for a running server. Set opens to leave out each mount that holds nothing the
     * caller can open.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListMountsResponse> listMounts(
        io.github.panyam.agni.v1.webapi.ListMountsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListMountsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetMount returns one mount by its name, the handle ListMounts gives (agni issue 1734). A page
     * that opens on a mount's address reads that mount alone. A name the server does not serve is
     * NOT_FOUND.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Mount> getMount(
        io.github.panyam.agni.v1.webapi.GetMountRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetMountMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ListDir lists one directory level inside a mount, with its subdirectories and its files. Each
     * file is labeled with the reader that would open it. The client addresses a location by an
     * artifact URI. The URI's authority is a mount, never an absolute host path, and the server
     * resolves it inside that mount. It is lazy, one level per call, so large trees remain
     * responsive. Set opens to leave out each subdirectory that holds nothing the caller can open,
     * at any depth.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListDirResponse> listDir(
        io.github.panyam.agni.v1.webapi.ListDirRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListDirMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ListDesignFiles lists every file a design's analysis reads (agni issue 853). A client may
     * analyse the design somewhere else, and the in-browser engine is the first such client. It can
     * then fetch exactly those files and nothing more. The list covers the design's folder, its
     * project's descriptor, and every config directory the project names. The client fetches each
     * file by its path under the server's read-only `/raw/&lt;mount&gt;/&lt;path&gt;` route. A set larger than
     * the server's bound (256 MB) is refused with its size. A board that big belongs on the server's
     * own engine.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListDesignFilesResponse> listDesignFiles(
        io.github.panyam.agni.v1.webapi.ListDesignFilesRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListDesignFilesMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ProposeDesigns takes a folder that declares no design, and groups its files into the designs
     * they make (agni issue 854). Imagine a page handed a drop of files. It must show what it will
     * read before it reads it. A folder already holding a `design.yaml` is reported as declared, and
     * the server does not guess again. Each proposal includes the `design.yaml` that would declare
     * it. Every file that no proposal reads is listed with the reason, so nothing is dropped silently.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ProposeDesignsResponse> proposeDesigns(
        io.github.panyam.agni.v1.webapi.ProposeDesignsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getProposeDesignsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * DeclareCompanion answers the design.yaml that declares a file as one of a design's companions,
     * such as a board a page attached with Snapshot.board_uri (agni issue 1367). It writes nothing.
     * A host whose mount a page can write, such as the browser's drop mount, saves the text, and on
     * any other the reader commits it. An existing descriptor keeps its comments and other keys, and
     * a new one names the design's entry and extends a project.yaml above it.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.DeclareCompanionResponse> declareCompanion(
        io.github.panyam.agni.v1.webapi.DeclareCompanionRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeclareCompanionMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_LIST_MOUNTS = 0;
  private static final int METHODID_GET_MOUNT = 1;
  private static final int METHODID_LIST_DIR = 2;
  private static final int METHODID_LIST_DESIGN_FILES = 3;
  private static final int METHODID_PROPOSE_DESIGNS = 4;
  private static final int METHODID_DECLARE_COMPANION = 5;

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
        case METHODID_LIST_MOUNTS:
          serviceImpl.listMounts((io.github.panyam.agni.v1.webapi.ListMountsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListMountsResponse>) responseObserver);
          break;
        case METHODID_GET_MOUNT:
          serviceImpl.getMount((io.github.panyam.agni.v1.webapi.GetMountRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Mount>) responseObserver);
          break;
        case METHODID_LIST_DIR:
          serviceImpl.listDir((io.github.panyam.agni.v1.webapi.ListDirRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListDirResponse>) responseObserver);
          break;
        case METHODID_LIST_DESIGN_FILES:
          serviceImpl.listDesignFiles((io.github.panyam.agni.v1.webapi.ListDesignFilesRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListDesignFilesResponse>) responseObserver);
          break;
        case METHODID_PROPOSE_DESIGNS:
          serviceImpl.proposeDesigns((io.github.panyam.agni.v1.webapi.ProposeDesignsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ProposeDesignsResponse>) responseObserver);
          break;
        case METHODID_DECLARE_COMPANION:
          serviceImpl.declareCompanion((io.github.panyam.agni.v1.webapi.DeclareCompanionRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.DeclareCompanionResponse>) responseObserver);
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
          getListMountsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListMountsRequest,
              io.github.panyam.agni.v1.webapi.ListMountsResponse>(
                service, METHODID_LIST_MOUNTS)))
        .addMethod(
          getGetMountMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetMountRequest,
              io.github.panyam.agni.v1.webapi.Mount>(
                service, METHODID_GET_MOUNT)))
        .addMethod(
          getListDirMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListDirRequest,
              io.github.panyam.agni.v1.webapi.ListDirResponse>(
                service, METHODID_LIST_DIR)))
        .addMethod(
          getListDesignFilesMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListDesignFilesRequest,
              io.github.panyam.agni.v1.webapi.ListDesignFilesResponse>(
                service, METHODID_LIST_DESIGN_FILES)))
        .addMethod(
          getProposeDesignsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ProposeDesignsRequest,
              io.github.panyam.agni.v1.webapi.ProposeDesignsResponse>(
                service, METHODID_PROPOSE_DESIGNS)))
        .addMethod(
          getDeclareCompanionMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.DeclareCompanionRequest,
              io.github.panyam.agni.v1.webapi.DeclareCompanionResponse>(
                service, METHODID_DECLARE_COMPANION)))
        .build();
  }

  private static abstract class WorkspaceServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    WorkspaceServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.WorkspaceProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("WorkspaceService");
    }
  }

  private static final class WorkspaceServiceFileDescriptorSupplier
      extends WorkspaceServiceBaseDescriptorSupplier {
    WorkspaceServiceFileDescriptorSupplier() {}
  }

  private static final class WorkspaceServiceMethodDescriptorSupplier
      extends WorkspaceServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    WorkspaceServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (WorkspaceServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new WorkspaceServiceFileDescriptorSupplier())
              .addMethod(getListMountsMethod())
              .addMethod(getGetMountMethod())
              .addMethod(getListDirMethod())
              .addMethod(getListDesignFilesMethod())
              .addMethod(getProposeDesignsMethod())
              .addMethod(getDeclareCompanionMethod())
              .build();
        }
      }
    }
    return result;
  }
}
