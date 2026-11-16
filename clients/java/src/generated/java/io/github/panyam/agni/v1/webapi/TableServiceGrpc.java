package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * TableService projects an answer into tables. The caller already holds the answer and sends it in
 * the request. The service reads no design and keeps no state.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class TableServiceGrpc {

  private TableServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.TableService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.TabulateRequest,
      io.github.panyam.agni.v1.webapi.TabulateResponse> getTabulateMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Tabulate",
      requestType = io.github.panyam.agni.v1.webapi.TabulateRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.TabulateResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.TabulateRequest,
      io.github.panyam.agni.v1.webapi.TabulateResponse> getTabulateMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.TabulateRequest, io.github.panyam.agni.v1.webapi.TabulateResponse> getTabulateMethod;
    if ((getTabulateMethod = TableServiceGrpc.getTabulateMethod) == null) {
      synchronized (TableServiceGrpc.class) {
        if ((getTabulateMethod = TableServiceGrpc.getTabulateMethod) == null) {
          TableServiceGrpc.getTabulateMethod = getTabulateMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.TabulateRequest, io.github.panyam.agni.v1.webapi.TabulateResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Tabulate"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.TabulateRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.TabulateResponse.getDefaultInstance()))
              .setSchemaDescriptor(new TableServiceMethodDescriptorSupplier("Tabulate"))
              .build();
        }
      }
    }
    return getTabulateMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.EncodeTablesRequest,
      io.github.panyam.agni.v1.webapi.EncodeTablesResponse> getEncodeMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Encode",
      requestType = io.github.panyam.agni.v1.webapi.EncodeTablesRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.EncodeTablesResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.EncodeTablesRequest,
      io.github.panyam.agni.v1.webapi.EncodeTablesResponse> getEncodeMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.EncodeTablesRequest, io.github.panyam.agni.v1.webapi.EncodeTablesResponse> getEncodeMethod;
    if ((getEncodeMethod = TableServiceGrpc.getEncodeMethod) == null) {
      synchronized (TableServiceGrpc.class) {
        if ((getEncodeMethod = TableServiceGrpc.getEncodeMethod) == null) {
          TableServiceGrpc.getEncodeMethod = getEncodeMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.EncodeTablesRequest, io.github.panyam.agni.v1.webapi.EncodeTablesResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Encode"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.EncodeTablesRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.EncodeTablesResponse.getDefaultInstance()))
              .setSchemaDescriptor(new TableServiceMethodDescriptorSupplier("Encode"))
              .build();
        }
      }
    }
    return getEncodeMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static TableServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<TableServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<TableServiceStub>() {
        @java.lang.Override
        public TableServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new TableServiceStub(channel, callOptions);
        }
      };
    return TableServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static TableServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<TableServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<TableServiceBlockingV2Stub>() {
        @java.lang.Override
        public TableServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new TableServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return TableServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static TableServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<TableServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<TableServiceBlockingStub>() {
        @java.lang.Override
        public TableServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new TableServiceBlockingStub(channel, callOptions);
        }
      };
    return TableServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static TableServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<TableServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<TableServiceFutureStub>() {
        @java.lang.Override
        public TableServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new TableServiceFutureStub(channel, callOptions);
        }
      };
    return TableServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * TableService projects an answer into tables. The caller already holds the answer and sends it in
   * the request. The service reads no design and keeps no state.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * Tabulate returns the answer's tables. A check run gives its findings. When the response includes
     * verdicts, it also gives the verdicts and a count of them per rule. A diff gives one table, a row
     * per change. A review gives its items and its summary. A query answer gives one table, and a query
     * set gives one per answered query. A set with a failed query is an invalid argument naming that
     * query, since a missing table would read as a question that matched nothing.
     * </pre>
     */
    default void tabulate(io.github.panyam.agni.v1.webapi.TabulateRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.TabulateResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getTabulateMethod(), responseObserver);
    }

    /**
     * <pre>
     * Encode writes tables as files. It serves a caller that has no encoder of its own and wants the
     * bytes `--format csv` writes, such as the viewer saving a table (agni issue 956). The CLI uses the
     * same encoder, so a cell that a spreadsheet would read as a formula is guarded the same way
     * everywhere. Encode writes the tables as sent, in their order, and reads no design.
     * </pre>
     */
    default void encode(io.github.panyam.agni.v1.webapi.EncodeTablesRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.EncodeTablesResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getEncodeMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service TableService.
   * <pre>
   * TableService projects an answer into tables. The caller already holds the answer and sends it in
   * the request. The service reads no design and keeps no state.
   * </pre>
   */
  public static abstract class TableServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return TableServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service TableService.
   * <pre>
   * TableService projects an answer into tables. The caller already holds the answer and sends it in
   * the request. The service reads no design and keeps no state.
   * </pre>
   */
  public static final class TableServiceStub
      extends io.grpc.stub.AbstractAsyncStub<TableServiceStub> {
    private TableServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected TableServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new TableServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * Tabulate returns the answer's tables. A check run gives its findings. When the response includes
     * verdicts, it also gives the verdicts and a count of them per rule. A diff gives one table, a row
     * per change. A review gives its items and its summary. A query answer gives one table, and a query
     * set gives one per answered query. A set with a failed query is an invalid argument naming that
     * query, since a missing table would read as a question that matched nothing.
     * </pre>
     */
    public void tabulate(io.github.panyam.agni.v1.webapi.TabulateRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.TabulateResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getTabulateMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * Encode writes tables as files. It serves a caller that has no encoder of its own and wants the
     * bytes `--format csv` writes, such as the viewer saving a table (agni issue 956). The CLI uses the
     * same encoder, so a cell that a spreadsheet would read as a formula is guarded the same way
     * everywhere. Encode writes the tables as sent, in their order, and reads no design.
     * </pre>
     */
    public void encode(io.github.panyam.agni.v1.webapi.EncodeTablesRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.EncodeTablesResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getEncodeMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service TableService.
   * <pre>
   * TableService projects an answer into tables. The caller already holds the answer and sends it in
   * the request. The service reads no design and keeps no state.
   * </pre>
   */
  public static final class TableServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<TableServiceBlockingV2Stub> {
    private TableServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected TableServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new TableServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * Tabulate returns the answer's tables. A check run gives its findings. When the response includes
     * verdicts, it also gives the verdicts and a count of them per rule. A diff gives one table, a row
     * per change. A review gives its items and its summary. A query answer gives one table, and a query
     * set gives one per answered query. A set with a failed query is an invalid argument naming that
     * query, since a missing table would read as a question that matched nothing.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.TabulateResponse tabulate(io.github.panyam.agni.v1.webapi.TabulateRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getTabulateMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Encode writes tables as files. It serves a caller that has no encoder of its own and wants the
     * bytes `--format csv` writes, such as the viewer saving a table (agni issue 956). The CLI uses the
     * same encoder, so a cell that a spreadsheet would read as a formula is guarded the same way
     * everywhere. Encode writes the tables as sent, in their order, and reads no design.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.EncodeTablesResponse encode(io.github.panyam.agni.v1.webapi.EncodeTablesRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getEncodeMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service TableService.
   * <pre>
   * TableService projects an answer into tables. The caller already holds the answer and sends it in
   * the request. The service reads no design and keeps no state.
   * </pre>
   */
  public static final class TableServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<TableServiceBlockingStub> {
    private TableServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected TableServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new TableServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * Tabulate returns the answer's tables. A check run gives its findings. When the response includes
     * verdicts, it also gives the verdicts and a count of them per rule. A diff gives one table, a row
     * per change. A review gives its items and its summary. A query answer gives one table, and a query
     * set gives one per answered query. A set with a failed query is an invalid argument naming that
     * query, since a missing table would read as a question that matched nothing.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.TabulateResponse tabulate(io.github.panyam.agni.v1.webapi.TabulateRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getTabulateMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Encode writes tables as files. It serves a caller that has no encoder of its own and wants the
     * bytes `--format csv` writes, such as the viewer saving a table (agni issue 956). The CLI uses the
     * same encoder, so a cell that a spreadsheet would read as a formula is guarded the same way
     * everywhere. Encode writes the tables as sent, in their order, and reads no design.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.EncodeTablesResponse encode(io.github.panyam.agni.v1.webapi.EncodeTablesRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getEncodeMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service TableService.
   * <pre>
   * TableService projects an answer into tables. The caller already holds the answer and sends it in
   * the request. The service reads no design and keeps no state.
   * </pre>
   */
  public static final class TableServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<TableServiceFutureStub> {
    private TableServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected TableServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new TableServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * Tabulate returns the answer's tables. A check run gives its findings. When the response includes
     * verdicts, it also gives the verdicts and a count of them per rule. A diff gives one table, a row
     * per change. A review gives its items and its summary. A query answer gives one table, and a query
     * set gives one per answered query. A set with a failed query is an invalid argument naming that
     * query, since a missing table would read as a question that matched nothing.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.TabulateResponse> tabulate(
        io.github.panyam.agni.v1.webapi.TabulateRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getTabulateMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * Encode writes tables as files. It serves a caller that has no encoder of its own and wants the
     * bytes `--format csv` writes, such as the viewer saving a table (agni issue 956). The CLI uses the
     * same encoder, so a cell that a spreadsheet would read as a formula is guarded the same way
     * everywhere. Encode writes the tables as sent, in their order, and reads no design.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.EncodeTablesResponse> encode(
        io.github.panyam.agni.v1.webapi.EncodeTablesRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getEncodeMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_TABULATE = 0;
  private static final int METHODID_ENCODE = 1;

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
        case METHODID_TABULATE:
          serviceImpl.tabulate((io.github.panyam.agni.v1.webapi.TabulateRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.TabulateResponse>) responseObserver);
          break;
        case METHODID_ENCODE:
          serviceImpl.encode((io.github.panyam.agni.v1.webapi.EncodeTablesRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.EncodeTablesResponse>) responseObserver);
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
          getTabulateMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.TabulateRequest,
              io.github.panyam.agni.v1.webapi.TabulateResponse>(
                service, METHODID_TABULATE)))
        .addMethod(
          getEncodeMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.EncodeTablesRequest,
              io.github.panyam.agni.v1.webapi.EncodeTablesResponse>(
                service, METHODID_ENCODE)))
        .build();
  }

  private static abstract class TableServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    TableServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.TablesProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("TableService");
    }
  }

  private static final class TableServiceFileDescriptorSupplier
      extends TableServiceBaseDescriptorSupplier {
    TableServiceFileDescriptorSupplier() {}
  }

  private static final class TableServiceMethodDescriptorSupplier
      extends TableServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    TableServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (TableServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new TableServiceFileDescriptorSupplier())
              .addMethod(getTabulateMethod())
              .addMethod(getEncodeMethod())
              .build();
        }
      }
    }
    return result;
  }
}
