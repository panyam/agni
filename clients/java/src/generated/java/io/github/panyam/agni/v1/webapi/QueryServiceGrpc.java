package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * QueryService evaluates a declarative-datalog query over a design's fact relations and returns
 * the answer rows with provenance. The relations are the WS3-004 fact base, and the rules assert
 * over the same ones. A geometry-only file with no netlist is an invalid argument. A malformed
 * query is also an invalid argument, and the caller shows it inline.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class QueryServiceGrpc {

  private QueryServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.QueryService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RunQueryRequest,
      io.github.panyam.agni.v1.webapi.RunQueryResponse> getRunQueryMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "RunQuery",
      requestType = io.github.panyam.agni.v1.webapi.RunQueryRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.RunQueryResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RunQueryRequest,
      io.github.panyam.agni.v1.webapi.RunQueryResponse> getRunQueryMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RunQueryRequest, io.github.panyam.agni.v1.webapi.RunQueryResponse> getRunQueryMethod;
    if ((getRunQueryMethod = QueryServiceGrpc.getRunQueryMethod) == null) {
      synchronized (QueryServiceGrpc.class) {
        if ((getRunQueryMethod = QueryServiceGrpc.getRunQueryMethod) == null) {
          QueryServiceGrpc.getRunQueryMethod = getRunQueryMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.RunQueryRequest, io.github.panyam.agni.v1.webapi.RunQueryResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "RunQuery"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.RunQueryRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.RunQueryResponse.getDefaultInstance()))
              .setSchemaDescriptor(new QueryServiceMethodDescriptorSupplier("RunQuery"))
              .build();
        }
      }
    }
    return getRunQueryMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RunQueriesRequest,
      io.github.panyam.agni.v1.webapi.RunQueriesResponse> getRunQueriesMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "RunQueries",
      requestType = io.github.panyam.agni.v1.webapi.RunQueriesRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.RunQueriesResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RunQueriesRequest,
      io.github.panyam.agni.v1.webapi.RunQueriesResponse> getRunQueriesMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RunQueriesRequest, io.github.panyam.agni.v1.webapi.RunQueriesResponse> getRunQueriesMethod;
    if ((getRunQueriesMethod = QueryServiceGrpc.getRunQueriesMethod) == null) {
      synchronized (QueryServiceGrpc.class) {
        if ((getRunQueriesMethod = QueryServiceGrpc.getRunQueriesMethod) == null) {
          QueryServiceGrpc.getRunQueriesMethod = getRunQueriesMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.RunQueriesRequest, io.github.panyam.agni.v1.webapi.RunQueriesResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "RunQueries"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.RunQueriesRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.RunQueriesResponse.getDefaultInstance()))
              .setSchemaDescriptor(new QueryServiceMethodDescriptorSupplier("RunQueries"))
              .build();
        }
      }
    }
    return getRunQueriesMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListRelationsRequest,
      io.github.panyam.agni.v1.webapi.ListRelationsResponse> getListRelationsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListRelations",
      requestType = io.github.panyam.agni.v1.webapi.ListRelationsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListRelationsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListRelationsRequest,
      io.github.panyam.agni.v1.webapi.ListRelationsResponse> getListRelationsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListRelationsRequest, io.github.panyam.agni.v1.webapi.ListRelationsResponse> getListRelationsMethod;
    if ((getListRelationsMethod = QueryServiceGrpc.getListRelationsMethod) == null) {
      synchronized (QueryServiceGrpc.class) {
        if ((getListRelationsMethod = QueryServiceGrpc.getListRelationsMethod) == null) {
          QueryServiceGrpc.getListRelationsMethod = getListRelationsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListRelationsRequest, io.github.panyam.agni.v1.webapi.ListRelationsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListRelations"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListRelationsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListRelationsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new QueryServiceMethodDescriptorSupplier("ListRelations"))
              .build();
        }
      }
    }
    return getListRelationsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListQuerySetsRequest,
      io.github.panyam.agni.v1.webapi.ListQuerySetsResponse> getListQuerySetsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListQuerySets",
      requestType = io.github.panyam.agni.v1.webapi.ListQuerySetsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListQuerySetsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListQuerySetsRequest,
      io.github.panyam.agni.v1.webapi.ListQuerySetsResponse> getListQuerySetsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListQuerySetsRequest, io.github.panyam.agni.v1.webapi.ListQuerySetsResponse> getListQuerySetsMethod;
    if ((getListQuerySetsMethod = QueryServiceGrpc.getListQuerySetsMethod) == null) {
      synchronized (QueryServiceGrpc.class) {
        if ((getListQuerySetsMethod = QueryServiceGrpc.getListQuerySetsMethod) == null) {
          QueryServiceGrpc.getListQuerySetsMethod = getListQuerySetsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListQuerySetsRequest, io.github.panyam.agni.v1.webapi.ListQuerySetsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListQuerySets"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListQuerySetsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListQuerySetsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new QueryServiceMethodDescriptorSupplier("ListQuerySets"))
              .build();
        }
      }
    }
    return getListQuerySetsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ParseQuerySetRequest,
      io.github.panyam.agni.v1.webapi.ParseQuerySetResponse> getParseQuerySetMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ParseQuerySet",
      requestType = io.github.panyam.agni.v1.webapi.ParseQuerySetRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ParseQuerySetResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ParseQuerySetRequest,
      io.github.panyam.agni.v1.webapi.ParseQuerySetResponse> getParseQuerySetMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ParseQuerySetRequest, io.github.panyam.agni.v1.webapi.ParseQuerySetResponse> getParseQuerySetMethod;
    if ((getParseQuerySetMethod = QueryServiceGrpc.getParseQuerySetMethod) == null) {
      synchronized (QueryServiceGrpc.class) {
        if ((getParseQuerySetMethod = QueryServiceGrpc.getParseQuerySetMethod) == null) {
          QueryServiceGrpc.getParseQuerySetMethod = getParseQuerySetMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ParseQuerySetRequest, io.github.panyam.agni.v1.webapi.ParseQuerySetResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ParseQuerySet"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ParseQuerySetRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ParseQuerySetResponse.getDefaultInstance()))
              .setSchemaDescriptor(new QueryServiceMethodDescriptorSupplier("ParseQuerySet"))
              .build();
        }
      }
    }
    return getParseQuerySetMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static QueryServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<QueryServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<QueryServiceStub>() {
        @java.lang.Override
        public QueryServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new QueryServiceStub(channel, callOptions);
        }
      };
    return QueryServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static QueryServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<QueryServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<QueryServiceBlockingV2Stub>() {
        @java.lang.Override
        public QueryServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new QueryServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return QueryServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static QueryServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<QueryServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<QueryServiceBlockingStub>() {
        @java.lang.Override
        public QueryServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new QueryServiceBlockingStub(channel, callOptions);
        }
      };
    return QueryServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static QueryServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<QueryServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<QueryServiceFutureStub>() {
        @java.lang.Override
        public QueryServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new QueryServiceFutureStub(channel, callOptions);
        }
      };
    return QueryServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * QueryService evaluates a declarative-datalog query over a design's fact relations and returns
   * the answer rows with provenance. The relations are the WS3-004 fact base, and the rules assert
   * over the same ones. A geometry-only file with no netlist is an invalid argument. A malformed
   * query is also an invalid argument, and the caller shows it inline.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * RunQuery parses and evaluates the query text over the loaded design. It returns the projected
     * columns and the answer rows, and each row has the provenance of the facts that produced it.
     * The relation vocabulary matches what the CLI documents (net.max_voltage, component.mpn,
     * component.net, reaches). The `param` (datasheet) relations read the server's corpus
     * (`--params` or `--params-url`), with the project's own `params/` layered over it per MPN.
     * Datasheet data is deployment-bound (C16), so with no corpus a query over `param` returns no
     * rows rather than an error.
     * </pre>
     */
    default void runQuery(io.github.panyam.agni.v1.webapi.RunQueryRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RunQueryResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRunQueryMethod(), responseObserver);
    }

    /**
     * <pre>
     * RunQueries answers a named list of queries over ONE read of the design (agni issue 729). An
     * audit is usually a workbook rather than a question. Asking each table through RunQuery would
     * read and project the design once per table. Every result has exactly the shape RunQuery would
     * return for that query alone. When a query fails, its error is reported against its name, and
     * the other queries still answer. Two cases fail the whole call. One is a set that is unusable
     * as a whole, with no queries, a repeated name, or a preamble holding a goal. The other is a
     * design that cannot be read.
     * </pre>
     */
    default void runQueries(io.github.panyam.agni.v1.webapi.RunQueriesRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RunQueriesResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRunQueriesMethod(), responseObserver);
    }

    /**
     * <pre>
     * ListRelations returns the queryable relation catalog (WS9-037). The catalog holds the built-in
     * relations, the shipped library's derived relations, the predicates, and any relations an
     * overlay registers. Each entry has its argument labels, a one-line summary, and a kind for
     * grouping. The catalog is static per service build and does not depend on a loaded design. So
     * the client fetches it once to fill the panel's relation picker.
     * With a path set, it answers one place in the namespace tree instead (agni issue 751). That
     * place is a module and its members, or one member with its signature. A derived member also
     * gets its definition. `agni query --relations &lt;path&gt;` prints the same answer.
     * </pre>
     */
    default void listRelations(io.github.panyam.agni.v1.webapi.ListRelationsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListRelationsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListRelationsMethod(), responseObserver);
    }

    /**
     * <pre>
     * ListQuerySets returns the query sets a design's project keeps (agni issue 745), inherited ones
     * first. Each set is parsed and named by its file. A viewer can open a set as one tab per query
     * and run it through RunQueries. A design in no project has no sets, and that is not an error.
     * </pre>
     */
    default void listQuerySets(io.github.panyam.agni.v1.webapi.ListQuerySetsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListQuerySetsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListQuerySetsMethod(), responseObserver);
    }

    /**
     * <pre>
     * ParseQuerySet reads a query set file's text into a QuerySet, which RunQueries takes. It serves
     * a caller that holds the text and has no parser, such as a browser after the reader picks a
     * file. It checks the same things `agni query --set` checks: an unknown key, no queries, a
     * repeated name, and a preamble that holds a goal. Under C22, a client gets a config value it
     * does not hold from that value's own rpc. So RunQueries itself never takes YAML.
     * </pre>
     */
    default void parseQuerySet(io.github.panyam.agni.v1.webapi.ParseQuerySetRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ParseQuerySetResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getParseQuerySetMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service QueryService.
   * <pre>
   * QueryService evaluates a declarative-datalog query over a design's fact relations and returns
   * the answer rows with provenance. The relations are the WS3-004 fact base, and the rules assert
   * over the same ones. A geometry-only file with no netlist is an invalid argument. A malformed
   * query is also an invalid argument, and the caller shows it inline.
   * </pre>
   */
  public static abstract class QueryServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return QueryServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service QueryService.
   * <pre>
   * QueryService evaluates a declarative-datalog query over a design's fact relations and returns
   * the answer rows with provenance. The relations are the WS3-004 fact base, and the rules assert
   * over the same ones. A geometry-only file with no netlist is an invalid argument. A malformed
   * query is also an invalid argument, and the caller shows it inline.
   * </pre>
   */
  public static final class QueryServiceStub
      extends io.grpc.stub.AbstractAsyncStub<QueryServiceStub> {
    private QueryServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected QueryServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new QueryServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * RunQuery parses and evaluates the query text over the loaded design. It returns the projected
     * columns and the answer rows, and each row has the provenance of the facts that produced it.
     * The relation vocabulary matches what the CLI documents (net.max_voltage, component.mpn,
     * component.net, reaches). The `param` (datasheet) relations read the server's corpus
     * (`--params` or `--params-url`), with the project's own `params/` layered over it per MPN.
     * Datasheet data is deployment-bound (C16), so with no corpus a query over `param` returns no
     * rows rather than an error.
     * </pre>
     */
    public void runQuery(io.github.panyam.agni.v1.webapi.RunQueryRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RunQueryResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRunQueryMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * RunQueries answers a named list of queries over ONE read of the design (agni issue 729). An
     * audit is usually a workbook rather than a question. Asking each table through RunQuery would
     * read and project the design once per table. Every result has exactly the shape RunQuery would
     * return for that query alone. When a query fails, its error is reported against its name, and
     * the other queries still answer. Two cases fail the whole call. One is a set that is unusable
     * as a whole, with no queries, a repeated name, or a preamble holding a goal. The other is a
     * design that cannot be read.
     * </pre>
     */
    public void runQueries(io.github.panyam.agni.v1.webapi.RunQueriesRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RunQueriesResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRunQueriesMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ListRelations returns the queryable relation catalog (WS9-037). The catalog holds the built-in
     * relations, the shipped library's derived relations, the predicates, and any relations an
     * overlay registers. Each entry has its argument labels, a one-line summary, and a kind for
     * grouping. The catalog is static per service build and does not depend on a loaded design. So
     * the client fetches it once to fill the panel's relation picker.
     * With a path set, it answers one place in the namespace tree instead (agni issue 751). That
     * place is a module and its members, or one member with its signature. A derived member also
     * gets its definition. `agni query --relations &lt;path&gt;` prints the same answer.
     * </pre>
     */
    public void listRelations(io.github.panyam.agni.v1.webapi.ListRelationsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListRelationsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListRelationsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ListQuerySets returns the query sets a design's project keeps (agni issue 745), inherited ones
     * first. Each set is parsed and named by its file. A viewer can open a set as one tab per query
     * and run it through RunQueries. A design in no project has no sets, and that is not an error.
     * </pre>
     */
    public void listQuerySets(io.github.panyam.agni.v1.webapi.ListQuerySetsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListQuerySetsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListQuerySetsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ParseQuerySet reads a query set file's text into a QuerySet, which RunQueries takes. It serves
     * a caller that holds the text and has no parser, such as a browser after the reader picks a
     * file. It checks the same things `agni query --set` checks: an unknown key, no queries, a
     * repeated name, and a preamble that holds a goal. Under C22, a client gets a config value it
     * does not hold from that value's own rpc. So RunQueries itself never takes YAML.
     * </pre>
     */
    public void parseQuerySet(io.github.panyam.agni.v1.webapi.ParseQuerySetRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ParseQuerySetResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getParseQuerySetMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service QueryService.
   * <pre>
   * QueryService evaluates a declarative-datalog query over a design's fact relations and returns
   * the answer rows with provenance. The relations are the WS3-004 fact base, and the rules assert
   * over the same ones. A geometry-only file with no netlist is an invalid argument. A malformed
   * query is also an invalid argument, and the caller shows it inline.
   * </pre>
   */
  public static final class QueryServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<QueryServiceBlockingV2Stub> {
    private QueryServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected QueryServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new QueryServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * RunQuery parses and evaluates the query text over the loaded design. It returns the projected
     * columns and the answer rows, and each row has the provenance of the facts that produced it.
     * The relation vocabulary matches what the CLI documents (net.max_voltage, component.mpn,
     * component.net, reaches). The `param` (datasheet) relations read the server's corpus
     * (`--params` or `--params-url`), with the project's own `params/` layered over it per MPN.
     * Datasheet data is deployment-bound (C16), so with no corpus a query over `param` returns no
     * rows rather than an error.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.RunQueryResponse runQuery(io.github.panyam.agni.v1.webapi.RunQueryRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getRunQueryMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * RunQueries answers a named list of queries over ONE read of the design (agni issue 729). An
     * audit is usually a workbook rather than a question. Asking each table through RunQuery would
     * read and project the design once per table. Every result has exactly the shape RunQuery would
     * return for that query alone. When a query fails, its error is reported against its name, and
     * the other queries still answer. Two cases fail the whole call. One is a set that is unusable
     * as a whole, with no queries, a repeated name, or a preamble holding a goal. The other is a
     * design that cannot be read.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.RunQueriesResponse runQueries(io.github.panyam.agni.v1.webapi.RunQueriesRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getRunQueriesMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListRelations returns the queryable relation catalog (WS9-037). The catalog holds the built-in
     * relations, the shipped library's derived relations, the predicates, and any relations an
     * overlay registers. Each entry has its argument labels, a one-line summary, and a kind for
     * grouping. The catalog is static per service build and does not depend on a loaded design. So
     * the client fetches it once to fill the panel's relation picker.
     * With a path set, it answers one place in the namespace tree instead (agni issue 751). That
     * place is a module and its members, or one member with its signature. A derived member also
     * gets its definition. `agni query --relations &lt;path&gt;` prints the same answer.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListRelationsResponse listRelations(io.github.panyam.agni.v1.webapi.ListRelationsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListRelationsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListQuerySets returns the query sets a design's project keeps (agni issue 745), inherited ones
     * first. Each set is parsed and named by its file. A viewer can open a set as one tab per query
     * and run it through RunQueries. A design in no project has no sets, and that is not an error.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListQuerySetsResponse listQuerySets(io.github.panyam.agni.v1.webapi.ListQuerySetsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListQuerySetsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ParseQuerySet reads a query set file's text into a QuerySet, which RunQueries takes. It serves
     * a caller that holds the text and has no parser, such as a browser after the reader picks a
     * file. It checks the same things `agni query --set` checks: an unknown key, no queries, a
     * repeated name, and a preamble that holds a goal. Under C22, a client gets a config value it
     * does not hold from that value's own rpc. So RunQueries itself never takes YAML.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ParseQuerySetResponse parseQuerySet(io.github.panyam.agni.v1.webapi.ParseQuerySetRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getParseQuerySetMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service QueryService.
   * <pre>
   * QueryService evaluates a declarative-datalog query over a design's fact relations and returns
   * the answer rows with provenance. The relations are the WS3-004 fact base, and the rules assert
   * over the same ones. A geometry-only file with no netlist is an invalid argument. A malformed
   * query is also an invalid argument, and the caller shows it inline.
   * </pre>
   */
  public static final class QueryServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<QueryServiceBlockingStub> {
    private QueryServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected QueryServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new QueryServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * RunQuery parses and evaluates the query text over the loaded design. It returns the projected
     * columns and the answer rows, and each row has the provenance of the facts that produced it.
     * The relation vocabulary matches what the CLI documents (net.max_voltage, component.mpn,
     * component.net, reaches). The `param` (datasheet) relations read the server's corpus
     * (`--params` or `--params-url`), with the project's own `params/` layered over it per MPN.
     * Datasheet data is deployment-bound (C16), so with no corpus a query over `param` returns no
     * rows rather than an error.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.RunQueryResponse runQuery(io.github.panyam.agni.v1.webapi.RunQueryRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRunQueryMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * RunQueries answers a named list of queries over ONE read of the design (agni issue 729). An
     * audit is usually a workbook rather than a question. Asking each table through RunQuery would
     * read and project the design once per table. Every result has exactly the shape RunQuery would
     * return for that query alone. When a query fails, its error is reported against its name, and
     * the other queries still answer. Two cases fail the whole call. One is a set that is unusable
     * as a whole, with no queries, a repeated name, or a preamble holding a goal. The other is a
     * design that cannot be read.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.RunQueriesResponse runQueries(io.github.panyam.agni.v1.webapi.RunQueriesRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRunQueriesMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListRelations returns the queryable relation catalog (WS9-037). The catalog holds the built-in
     * relations, the shipped library's derived relations, the predicates, and any relations an
     * overlay registers. Each entry has its argument labels, a one-line summary, and a kind for
     * grouping. The catalog is static per service build and does not depend on a loaded design. So
     * the client fetches it once to fill the panel's relation picker.
     * With a path set, it answers one place in the namespace tree instead (agni issue 751). That
     * place is a module and its members, or one member with its signature. A derived member also
     * gets its definition. `agni query --relations &lt;path&gt;` prints the same answer.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListRelationsResponse listRelations(io.github.panyam.agni.v1.webapi.ListRelationsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListRelationsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListQuerySets returns the query sets a design's project keeps (agni issue 745), inherited ones
     * first. Each set is parsed and named by its file. A viewer can open a set as one tab per query
     * and run it through RunQueries. A design in no project has no sets, and that is not an error.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListQuerySetsResponse listQuerySets(io.github.panyam.agni.v1.webapi.ListQuerySetsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListQuerySetsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ParseQuerySet reads a query set file's text into a QuerySet, which RunQueries takes. It serves
     * a caller that holds the text and has no parser, such as a browser after the reader picks a
     * file. It checks the same things `agni query --set` checks: an unknown key, no queries, a
     * repeated name, and a preamble that holds a goal. Under C22, a client gets a config value it
     * does not hold from that value's own rpc. So RunQueries itself never takes YAML.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ParseQuerySetResponse parseQuerySet(io.github.panyam.agni.v1.webapi.ParseQuerySetRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getParseQuerySetMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service QueryService.
   * <pre>
   * QueryService evaluates a declarative-datalog query over a design's fact relations and returns
   * the answer rows with provenance. The relations are the WS3-004 fact base, and the rules assert
   * over the same ones. A geometry-only file with no netlist is an invalid argument. A malformed
   * query is also an invalid argument, and the caller shows it inline.
   * </pre>
   */
  public static final class QueryServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<QueryServiceFutureStub> {
    private QueryServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected QueryServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new QueryServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * RunQuery parses and evaluates the query text over the loaded design. It returns the projected
     * columns and the answer rows, and each row has the provenance of the facts that produced it.
     * The relation vocabulary matches what the CLI documents (net.max_voltage, component.mpn,
     * component.net, reaches). The `param` (datasheet) relations read the server's corpus
     * (`--params` or `--params-url`), with the project's own `params/` layered over it per MPN.
     * Datasheet data is deployment-bound (C16), so with no corpus a query over `param` returns no
     * rows rather than an error.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.RunQueryResponse> runQuery(
        io.github.panyam.agni.v1.webapi.RunQueryRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRunQueryMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * RunQueries answers a named list of queries over ONE read of the design (agni issue 729). An
     * audit is usually a workbook rather than a question. Asking each table through RunQuery would
     * read and project the design once per table. Every result has exactly the shape RunQuery would
     * return for that query alone. When a query fails, its error is reported against its name, and
     * the other queries still answer. Two cases fail the whole call. One is a set that is unusable
     * as a whole, with no queries, a repeated name, or a preamble holding a goal. The other is a
     * design that cannot be read.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.RunQueriesResponse> runQueries(
        io.github.panyam.agni.v1.webapi.RunQueriesRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRunQueriesMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ListRelations returns the queryable relation catalog (WS9-037). The catalog holds the built-in
     * relations, the shipped library's derived relations, the predicates, and any relations an
     * overlay registers. Each entry has its argument labels, a one-line summary, and a kind for
     * grouping. The catalog is static per service build and does not depend on a loaded design. So
     * the client fetches it once to fill the panel's relation picker.
     * With a path set, it answers one place in the namespace tree instead (agni issue 751). That
     * place is a module and its members, or one member with its signature. A derived member also
     * gets its definition. `agni query --relations &lt;path&gt;` prints the same answer.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListRelationsResponse> listRelations(
        io.github.panyam.agni.v1.webapi.ListRelationsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListRelationsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ListQuerySets returns the query sets a design's project keeps (agni issue 745), inherited ones
     * first. Each set is parsed and named by its file. A viewer can open a set as one tab per query
     * and run it through RunQueries. A design in no project has no sets, and that is not an error.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListQuerySetsResponse> listQuerySets(
        io.github.panyam.agni.v1.webapi.ListQuerySetsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListQuerySetsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ParseQuerySet reads a query set file's text into a QuerySet, which RunQueries takes. It serves
     * a caller that holds the text and has no parser, such as a browser after the reader picks a
     * file. It checks the same things `agni query --set` checks: an unknown key, no queries, a
     * repeated name, and a preamble that holds a goal. Under C22, a client gets a config value it
     * does not hold from that value's own rpc. So RunQueries itself never takes YAML.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ParseQuerySetResponse> parseQuerySet(
        io.github.panyam.agni.v1.webapi.ParseQuerySetRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getParseQuerySetMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_RUN_QUERY = 0;
  private static final int METHODID_RUN_QUERIES = 1;
  private static final int METHODID_LIST_RELATIONS = 2;
  private static final int METHODID_LIST_QUERY_SETS = 3;
  private static final int METHODID_PARSE_QUERY_SET = 4;

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
        case METHODID_RUN_QUERY:
          serviceImpl.runQuery((io.github.panyam.agni.v1.webapi.RunQueryRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RunQueryResponse>) responseObserver);
          break;
        case METHODID_RUN_QUERIES:
          serviceImpl.runQueries((io.github.panyam.agni.v1.webapi.RunQueriesRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RunQueriesResponse>) responseObserver);
          break;
        case METHODID_LIST_RELATIONS:
          serviceImpl.listRelations((io.github.panyam.agni.v1.webapi.ListRelationsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListRelationsResponse>) responseObserver);
          break;
        case METHODID_LIST_QUERY_SETS:
          serviceImpl.listQuerySets((io.github.panyam.agni.v1.webapi.ListQuerySetsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListQuerySetsResponse>) responseObserver);
          break;
        case METHODID_PARSE_QUERY_SET:
          serviceImpl.parseQuerySet((io.github.panyam.agni.v1.webapi.ParseQuerySetRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ParseQuerySetResponse>) responseObserver);
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
          getRunQueryMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.RunQueryRequest,
              io.github.panyam.agni.v1.webapi.RunQueryResponse>(
                service, METHODID_RUN_QUERY)))
        .addMethod(
          getRunQueriesMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.RunQueriesRequest,
              io.github.panyam.agni.v1.webapi.RunQueriesResponse>(
                service, METHODID_RUN_QUERIES)))
        .addMethod(
          getListRelationsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListRelationsRequest,
              io.github.panyam.agni.v1.webapi.ListRelationsResponse>(
                service, METHODID_LIST_RELATIONS)))
        .addMethod(
          getListQuerySetsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListQuerySetsRequest,
              io.github.panyam.agni.v1.webapi.ListQuerySetsResponse>(
                service, METHODID_LIST_QUERY_SETS)))
        .addMethod(
          getParseQuerySetMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ParseQuerySetRequest,
              io.github.panyam.agni.v1.webapi.ParseQuerySetResponse>(
                service, METHODID_PARSE_QUERY_SET)))
        .build();
  }

  private static abstract class QueryServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    QueryServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.QueryProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("QueryService");
    }
  }

  private static final class QueryServiceFileDescriptorSupplier
      extends QueryServiceBaseDescriptorSupplier {
    QueryServiceFileDescriptorSupplier() {}
  }

  private static final class QueryServiceMethodDescriptorSupplier
      extends QueryServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    QueryServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (QueryServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new QueryServiceFileDescriptorSupplier())
              .addMethod(getRunQueryMethod())
              .addMethod(getRunQueriesMethod())
              .addMethod(getListRelationsMethod())
              .addMethod(getListQuerySetsMethod())
              .addMethod(getParseQuerySetMethod())
              .build();
        }
      }
    }
    return result;
  }
}
