package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * ReviewService manages review RUNS as resources (WS9-053). A run used to be a call. You sent a
 * checklist and a design, got outcomes back, and nothing remained. A review does not work that
 * way. A team acts on a checklist verdict over weeks. It compares verdicts between revisions, and
 * gates a release on one. Every one of those uses needs the run to still exist tomorrow.
 * So a client creates, fetches, lists, and deletes runs. A run produces a self-contained
 * agni.v1.checks.CheckResults document, the same one `agni review --results-out` writes. The
 * document holds everything needed to render the run again, including the checklist snapshot. So
 * a review from last quarter renders against the checklist it actually scored, and not against
 * whatever that file says today.
 * This service is AIP-shaped on purpose, while the rest of this API is verb-shaped (see
 * CONSTRAINTS C23). The difference is not one of style. Every other rpc here is a pure function of
 * the snapshot it names, so `GetSummary(snapshot)` names its whole input. A review run has a lifetime
 * independent of its inputs, so it needs an identity of its own.
 * The overlay knobs (--profile-path / --intent-path / --params) are still serve-startup config.
 * The service composes them once into the catalog it holds. So a create request sends the
 * checklist, names the design's snapshot, and sets the ratified-data floor.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class ReviewServiceGrpc {

  private ReviewServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.ReviewService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateReviewRequest,
      io.github.panyam.agni.v1.webapi.Review> getCreateReviewMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CreateReview",
      requestType = io.github.panyam.agni.v1.webapi.CreateReviewRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Review.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateReviewRequest,
      io.github.panyam.agni.v1.webapi.Review> getCreateReviewMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateReviewRequest, io.github.panyam.agni.v1.webapi.Review> getCreateReviewMethod;
    if ((getCreateReviewMethod = ReviewServiceGrpc.getCreateReviewMethod) == null) {
      synchronized (ReviewServiceGrpc.class) {
        if ((getCreateReviewMethod = ReviewServiceGrpc.getCreateReviewMethod) == null) {
          ReviewServiceGrpc.getCreateReviewMethod = getCreateReviewMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.CreateReviewRequest, io.github.panyam.agni.v1.webapi.Review>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CreateReview"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.CreateReviewRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Review.getDefaultInstance()))
              .setSchemaDescriptor(new ReviewServiceMethodDescriptorSupplier("CreateReview"))
              .build();
        }
      }
    }
    return getCreateReviewMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetReviewRequest,
      io.github.panyam.agni.v1.webapi.Review> getGetReviewMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetReview",
      requestType = io.github.panyam.agni.v1.webapi.GetReviewRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Review.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetReviewRequest,
      io.github.panyam.agni.v1.webapi.Review> getGetReviewMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetReviewRequest, io.github.panyam.agni.v1.webapi.Review> getGetReviewMethod;
    if ((getGetReviewMethod = ReviewServiceGrpc.getGetReviewMethod) == null) {
      synchronized (ReviewServiceGrpc.class) {
        if ((getGetReviewMethod = ReviewServiceGrpc.getGetReviewMethod) == null) {
          ReviewServiceGrpc.getGetReviewMethod = getGetReviewMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetReviewRequest, io.github.panyam.agni.v1.webapi.Review>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetReview"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetReviewRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Review.getDefaultInstance()))
              .setSchemaDescriptor(new ReviewServiceMethodDescriptorSupplier("GetReview"))
              .build();
        }
      }
    }
    return getGetReviewMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListReviewsRequest,
      io.github.panyam.agni.v1.webapi.ListReviewsResponse> getListReviewsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListReviews",
      requestType = io.github.panyam.agni.v1.webapi.ListReviewsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListReviewsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListReviewsRequest,
      io.github.panyam.agni.v1.webapi.ListReviewsResponse> getListReviewsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListReviewsRequest, io.github.panyam.agni.v1.webapi.ListReviewsResponse> getListReviewsMethod;
    if ((getListReviewsMethod = ReviewServiceGrpc.getListReviewsMethod) == null) {
      synchronized (ReviewServiceGrpc.class) {
        if ((getListReviewsMethod = ReviewServiceGrpc.getListReviewsMethod) == null) {
          ReviewServiceGrpc.getListReviewsMethod = getListReviewsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListReviewsRequest, io.github.panyam.agni.v1.webapi.ListReviewsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListReviews"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListReviewsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListReviewsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ReviewServiceMethodDescriptorSupplier("ListReviews"))
              .build();
        }
      }
    }
    return getListReviewsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteReviewRequest,
      com.google.protobuf.Empty> getDeleteReviewMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "DeleteReview",
      requestType = io.github.panyam.agni.v1.webapi.DeleteReviewRequest.class,
      responseType = com.google.protobuf.Empty.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteReviewRequest,
      com.google.protobuf.Empty> getDeleteReviewMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteReviewRequest, com.google.protobuf.Empty> getDeleteReviewMethod;
    if ((getDeleteReviewMethod = ReviewServiceGrpc.getDeleteReviewMethod) == null) {
      synchronized (ReviewServiceGrpc.class) {
        if ((getDeleteReviewMethod = ReviewServiceGrpc.getDeleteReviewMethod) == null) {
          ReviewServiceGrpc.getDeleteReviewMethod = getDeleteReviewMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.DeleteReviewRequest, com.google.protobuf.Empty>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "DeleteReview"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.DeleteReviewRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.google.protobuf.Empty.getDefaultInstance()))
              .setSchemaDescriptor(new ReviewServiceMethodDescriptorSupplier("DeleteReview"))
              .build();
        }
      }
    }
    return getDeleteReviewMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetReviewManifestRequest,
      io.github.panyam.agni.v1.webapi.GetReviewManifestResponse> getGetReviewManifestMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetReviewManifest",
      requestType = io.github.panyam.agni.v1.webapi.GetReviewManifestRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.GetReviewManifestResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetReviewManifestRequest,
      io.github.panyam.agni.v1.webapi.GetReviewManifestResponse> getGetReviewManifestMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetReviewManifestRequest, io.github.panyam.agni.v1.webapi.GetReviewManifestResponse> getGetReviewManifestMethod;
    if ((getGetReviewManifestMethod = ReviewServiceGrpc.getGetReviewManifestMethod) == null) {
      synchronized (ReviewServiceGrpc.class) {
        if ((getGetReviewManifestMethod = ReviewServiceGrpc.getGetReviewManifestMethod) == null) {
          ReviewServiceGrpc.getGetReviewManifestMethod = getGetReviewManifestMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetReviewManifestRequest, io.github.panyam.agni.v1.webapi.GetReviewManifestResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetReviewManifest"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetReviewManifestRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetReviewManifestResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ReviewServiceMethodDescriptorSupplier("GetReviewManifest"))
              .build();
        }
      }
    }
    return getGetReviewManifestMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListChecklistsRequest,
      io.github.panyam.agni.v1.webapi.ListChecklistsResponse> getListChecklistsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListChecklists",
      requestType = io.github.panyam.agni.v1.webapi.ListChecklistsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListChecklistsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListChecklistsRequest,
      io.github.panyam.agni.v1.webapi.ListChecklistsResponse> getListChecklistsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListChecklistsRequest, io.github.panyam.agni.v1.webapi.ListChecklistsResponse> getListChecklistsMethod;
    if ((getListChecklistsMethod = ReviewServiceGrpc.getListChecklistsMethod) == null) {
      synchronized (ReviewServiceGrpc.class) {
        if ((getListChecklistsMethod = ReviewServiceGrpc.getListChecklistsMethod) == null) {
          ReviewServiceGrpc.getListChecklistsMethod = getListChecklistsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListChecklistsRequest, io.github.panyam.agni.v1.webapi.ListChecklistsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListChecklists"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListChecklistsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListChecklistsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ReviewServiceMethodDescriptorSupplier("ListChecklists"))
              .build();
        }
      }
    }
    return getListChecklistsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RenderReviewReportRequest,
      io.github.panyam.agni.v1.webapi.RenderReviewReportResponse> getRenderReviewReportMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "RenderReviewReport",
      requestType = io.github.panyam.agni.v1.webapi.RenderReviewReportRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.RenderReviewReportResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RenderReviewReportRequest,
      io.github.panyam.agni.v1.webapi.RenderReviewReportResponse> getRenderReviewReportMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RenderReviewReportRequest, io.github.panyam.agni.v1.webapi.RenderReviewReportResponse> getRenderReviewReportMethod;
    if ((getRenderReviewReportMethod = ReviewServiceGrpc.getRenderReviewReportMethod) == null) {
      synchronized (ReviewServiceGrpc.class) {
        if ((getRenderReviewReportMethod = ReviewServiceGrpc.getRenderReviewReportMethod) == null) {
          ReviewServiceGrpc.getRenderReviewReportMethod = getRenderReviewReportMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.RenderReviewReportRequest, io.github.panyam.agni.v1.webapi.RenderReviewReportResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "RenderReviewReport"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.RenderReviewReportRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.RenderReviewReportResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ReviewServiceMethodDescriptorSupplier("RenderReviewReport"))
              .build();
        }
      }
    }
    return getRenderReviewReportMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static ReviewServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ReviewServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ReviewServiceStub>() {
        @java.lang.Override
        public ReviewServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ReviewServiceStub(channel, callOptions);
        }
      };
    return ReviewServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static ReviewServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ReviewServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ReviewServiceBlockingV2Stub>() {
        @java.lang.Override
        public ReviewServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ReviewServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return ReviewServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static ReviewServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ReviewServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ReviewServiceBlockingStub>() {
        @java.lang.Override
        public ReviewServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ReviewServiceBlockingStub(channel, callOptions);
        }
      };
    return ReviewServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static ReviewServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ReviewServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ReviewServiceFutureStub>() {
        @java.lang.Override
        public ReviewServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ReviewServiceFutureStub(channel, callOptions);
        }
      };
    return ReviewServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * ReviewService manages review RUNS as resources (WS9-053). A run used to be a call. You sent a
   * checklist and a design, got outcomes back, and nothing remained. A review does not work that
   * way. A team acts on a checklist verdict over weeks. It compares verdicts between revisions, and
   * gates a release on one. Every one of those uses needs the run to still exist tomorrow.
   * So a client creates, fetches, lists, and deletes runs. A run produces a self-contained
   * agni.v1.checks.CheckResults document, the same one `agni review --results-out` writes. The
   * document holds everything needed to render the run again, including the checklist snapshot. So
   * a review from last quarter renders against the checklist it actually scored, and not against
   * whatever that file says today.
   * This service is AIP-shaped on purpose, while the rest of this API is verb-shaped (see
   * CONSTRAINTS C23). The difference is not one of style. Every other rpc here is a pure function of
   * the snapshot it names, so `GetSummary(snapshot)` names its whole input. A review run has a lifetime
   * independent of its inputs, so it needs an identity of its own.
   * The overlay knobs (--profile-path / --intent-path / --params) are still serve-startup config.
   * The service composes them once into the catalog it holds. So a create request sends the
   * checklist, names the design's snapshot, and sets the ratified-data floor.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * CreateReview runs the checklist against the design, stores the result, and returns the stored
     * Review. The run is all-or-nothing. An invalid or absent manifest is an error, and so is a
     * snapshot whose design cannot be read. So a partial read never
     * reports items clean without checking them. Nothing is stored when the run fails.
     * It is synchronous. A review is a check sweep per item, and it finishes within the time a
     * request can wait. So this returns the Review itself rather than an Operation. If that stops
     * being true, the answer is AIP-151, not a client-side poll invented here.
     * One review is about ONE design. A project rollup is several Reviews. A document holds a single
     * DesignRef on purpose, because averaging several designs into one document would misrepresent
     * every one of them.
     * </pre>
     */
    default void createReview(io.github.panyam.agni.v1.webapi.CreateReviewRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Review> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCreateReviewMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetReview returns a stored run by name. The document is self-contained, so this needs neither the
     * design nor the checklist to still exist.
     * </pre>
     */
    default void getReview(io.github.panyam.agni.v1.webapi.GetReviewRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Review> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetReviewMethod(), responseObserver);
    }

    /**
     * <pre>
     * ListReviews returns stored runs, newest first. Results are paginated. `filter` narrows them to
     * one design, so a client can ask the question it actually has ("how has THIS board been
     * trending").
     * </pre>
     */
    default void listReviews(io.github.panyam.agni.v1.webapi.ListReviewsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListReviewsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListReviewsMethod(), responseObserver);
    }

    /**
     * <pre>
     * DeleteReview removes a stored run. Deleting a run that is already absent is an error, not a
     * silent success. A client asking to delete something that is not there has a stale view, and
     * should be told so.
     * </pre>
     */
    default void deleteReview(io.github.panyam.agni.v1.webapi.DeleteReviewRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeleteReviewMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetReviewManifest resolves a stored checklist into a value for CreateReview. It parses AND
     * validates. So a client learns once, here, that its manifest is malformed, and not on every run.
     * A caller that already holds a manifest skips this call. The CLI is one, since it reads the YAML
     * the user named.
     * </pre>
     */
    default void getReviewManifest(io.github.panyam.agni.v1.webapi.GetReviewManifestRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetReviewManifestResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetReviewManifestMethod(), responseObserver);
    }

    /**
     * <pre>
     * ListChecklists returns the checklists a design's project declares, inherited ones included
     * (agni issue 859). They come in the order the project writes them, so the first is the
     * project's default. Every surface asks this rpc which checklists a design has. `agni review
     * --checklist &lt;name&gt;`, `agni checklists` and the viewer's picker all read it. A design in no
     * project answers an empty project and no checklists. A project declaring none answers its name
     * and no checklists. The two answers differ because the fixes differ. The first sends a manifest,
     * and the second adds a checklists section. A descriptor that exists and does not parse is an
     * error. The checklists are values, so a client runs one by sending it to CreateReview.
     * </pre>
     */
    default void listChecklists(io.github.panyam.agni.v1.webapi.ListChecklistsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListChecklistsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListChecklistsMethod(), responseObserver);
    }

    /**
     * <pre>
     * RenderReviewReport takes a review the caller already holds and writes it as a checklist page
     * (agni issue 1030). `agni review --format html` writes the same page. The page lists the
     * manifest's areas and items in the manifest's order, each with its outcome and every finding
     * behind it. It runs nothing and reads no store. So a run that was never stored (validate_only)
     * renders as a stored one does. The CLI's page and the viewer's saved page both come from this
     * one renderer.
     * </pre>
     */
    default void renderReviewReport(io.github.panyam.agni.v1.webapi.RenderReviewReportRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RenderReviewReportResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRenderReviewReportMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service ReviewService.
   * <pre>
   * ReviewService manages review RUNS as resources (WS9-053). A run used to be a call. You sent a
   * checklist and a design, got outcomes back, and nothing remained. A review does not work that
   * way. A team acts on a checklist verdict over weeks. It compares verdicts between revisions, and
   * gates a release on one. Every one of those uses needs the run to still exist tomorrow.
   * So a client creates, fetches, lists, and deletes runs. A run produces a self-contained
   * agni.v1.checks.CheckResults document, the same one `agni review --results-out` writes. The
   * document holds everything needed to render the run again, including the checklist snapshot. So
   * a review from last quarter renders against the checklist it actually scored, and not against
   * whatever that file says today.
   * This service is AIP-shaped on purpose, while the rest of this API is verb-shaped (see
   * CONSTRAINTS C23). The difference is not one of style. Every other rpc here is a pure function of
   * the snapshot it names, so `GetSummary(snapshot)` names its whole input. A review run has a lifetime
   * independent of its inputs, so it needs an identity of its own.
   * The overlay knobs (--profile-path / --intent-path / --params) are still serve-startup config.
   * The service composes them once into the catalog it holds. So a create request sends the
   * checklist, names the design's snapshot, and sets the ratified-data floor.
   * </pre>
   */
  public static abstract class ReviewServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return ReviewServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service ReviewService.
   * <pre>
   * ReviewService manages review RUNS as resources (WS9-053). A run used to be a call. You sent a
   * checklist and a design, got outcomes back, and nothing remained. A review does not work that
   * way. A team acts on a checklist verdict over weeks. It compares verdicts between revisions, and
   * gates a release on one. Every one of those uses needs the run to still exist tomorrow.
   * So a client creates, fetches, lists, and deletes runs. A run produces a self-contained
   * agni.v1.checks.CheckResults document, the same one `agni review --results-out` writes. The
   * document holds everything needed to render the run again, including the checklist snapshot. So
   * a review from last quarter renders against the checklist it actually scored, and not against
   * whatever that file says today.
   * This service is AIP-shaped on purpose, while the rest of this API is verb-shaped (see
   * CONSTRAINTS C23). The difference is not one of style. Every other rpc here is a pure function of
   * the snapshot it names, so `GetSummary(snapshot)` names its whole input. A review run has a lifetime
   * independent of its inputs, so it needs an identity of its own.
   * The overlay knobs (--profile-path / --intent-path / --params) are still serve-startup config.
   * The service composes them once into the catalog it holds. So a create request sends the
   * checklist, names the design's snapshot, and sets the ratified-data floor.
   * </pre>
   */
  public static final class ReviewServiceStub
      extends io.grpc.stub.AbstractAsyncStub<ReviewServiceStub> {
    private ReviewServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ReviewServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ReviewServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * CreateReview runs the checklist against the design, stores the result, and returns the stored
     * Review. The run is all-or-nothing. An invalid or absent manifest is an error, and so is a
     * snapshot whose design cannot be read. So a partial read never
     * reports items clean without checking them. Nothing is stored when the run fails.
     * It is synchronous. A review is a check sweep per item, and it finishes within the time a
     * request can wait. So this returns the Review itself rather than an Operation. If that stops
     * being true, the answer is AIP-151, not a client-side poll invented here.
     * One review is about ONE design. A project rollup is several Reviews. A document holds a single
     * DesignRef on purpose, because averaging several designs into one document would misrepresent
     * every one of them.
     * </pre>
     */
    public void createReview(io.github.panyam.agni.v1.webapi.CreateReviewRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Review> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCreateReviewMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetReview returns a stored run by name. The document is self-contained, so this needs neither the
     * design nor the checklist to still exist.
     * </pre>
     */
    public void getReview(io.github.panyam.agni.v1.webapi.GetReviewRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Review> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetReviewMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ListReviews returns stored runs, newest first. Results are paginated. `filter` narrows them to
     * one design, so a client can ask the question it actually has ("how has THIS board been
     * trending").
     * </pre>
     */
    public void listReviews(io.github.panyam.agni.v1.webapi.ListReviewsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListReviewsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListReviewsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * DeleteReview removes a stored run. Deleting a run that is already absent is an error, not a
     * silent success. A client asking to delete something that is not there has a stale view, and
     * should be told so.
     * </pre>
     */
    public void deleteReview(io.github.panyam.agni.v1.webapi.DeleteReviewRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeleteReviewMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetReviewManifest resolves a stored checklist into a value for CreateReview. It parses AND
     * validates. So a client learns once, here, that its manifest is malformed, and not on every run.
     * A caller that already holds a manifest skips this call. The CLI is one, since it reads the YAML
     * the user named.
     * </pre>
     */
    public void getReviewManifest(io.github.panyam.agni.v1.webapi.GetReviewManifestRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetReviewManifestResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetReviewManifestMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ListChecklists returns the checklists a design's project declares, inherited ones included
     * (agni issue 859). They come in the order the project writes them, so the first is the
     * project's default. Every surface asks this rpc which checklists a design has. `agni review
     * --checklist &lt;name&gt;`, `agni checklists` and the viewer's picker all read it. A design in no
     * project answers an empty project and no checklists. A project declaring none answers its name
     * and no checklists. The two answers differ because the fixes differ. The first sends a manifest,
     * and the second adds a checklists section. A descriptor that exists and does not parse is an
     * error. The checklists are values, so a client runs one by sending it to CreateReview.
     * </pre>
     */
    public void listChecklists(io.github.panyam.agni.v1.webapi.ListChecklistsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListChecklistsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListChecklistsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * RenderReviewReport takes a review the caller already holds and writes it as a checklist page
     * (agni issue 1030). `agni review --format html` writes the same page. The page lists the
     * manifest's areas and items in the manifest's order, each with its outcome and every finding
     * behind it. It runs nothing and reads no store. So a run that was never stored (validate_only)
     * renders as a stored one does. The CLI's page and the viewer's saved page both come from this
     * one renderer.
     * </pre>
     */
    public void renderReviewReport(io.github.panyam.agni.v1.webapi.RenderReviewReportRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RenderReviewReportResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRenderReviewReportMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service ReviewService.
   * <pre>
   * ReviewService manages review RUNS as resources (WS9-053). A run used to be a call. You sent a
   * checklist and a design, got outcomes back, and nothing remained. A review does not work that
   * way. A team acts on a checklist verdict over weeks. It compares verdicts between revisions, and
   * gates a release on one. Every one of those uses needs the run to still exist tomorrow.
   * So a client creates, fetches, lists, and deletes runs. A run produces a self-contained
   * agni.v1.checks.CheckResults document, the same one `agni review --results-out` writes. The
   * document holds everything needed to render the run again, including the checklist snapshot. So
   * a review from last quarter renders against the checklist it actually scored, and not against
   * whatever that file says today.
   * This service is AIP-shaped on purpose, while the rest of this API is verb-shaped (see
   * CONSTRAINTS C23). The difference is not one of style. Every other rpc here is a pure function of
   * the snapshot it names, so `GetSummary(snapshot)` names its whole input. A review run has a lifetime
   * independent of its inputs, so it needs an identity of its own.
   * The overlay knobs (--profile-path / --intent-path / --params) are still serve-startup config.
   * The service composes them once into the catalog it holds. So a create request sends the
   * checklist, names the design's snapshot, and sets the ratified-data floor.
   * </pre>
   */
  public static final class ReviewServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<ReviewServiceBlockingV2Stub> {
    private ReviewServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ReviewServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ReviewServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * CreateReview runs the checklist against the design, stores the result, and returns the stored
     * Review. The run is all-or-nothing. An invalid or absent manifest is an error, and so is a
     * snapshot whose design cannot be read. So a partial read never
     * reports items clean without checking them. Nothing is stored when the run fails.
     * It is synchronous. A review is a check sweep per item, and it finishes within the time a
     * request can wait. So this returns the Review itself rather than an Operation. If that stops
     * being true, the answer is AIP-151, not a client-side poll invented here.
     * One review is about ONE design. A project rollup is several Reviews. A document holds a single
     * DesignRef on purpose, because averaging several designs into one document would misrepresent
     * every one of them.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Review createReview(io.github.panyam.agni.v1.webapi.CreateReviewRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getCreateReviewMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetReview returns a stored run by name. The document is self-contained, so this needs neither the
     * design nor the checklist to still exist.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Review getReview(io.github.panyam.agni.v1.webapi.GetReviewRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetReviewMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListReviews returns stored runs, newest first. Results are paginated. `filter` narrows them to
     * one design, so a client can ask the question it actually has ("how has THIS board been
     * trending").
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListReviewsResponse listReviews(io.github.panyam.agni.v1.webapi.ListReviewsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListReviewsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteReview removes a stored run. Deleting a run that is already absent is an error, not a
     * silent success. A client asking to delete something that is not there has a stale view, and
     * should be told so.
     * </pre>
     */
    public com.google.protobuf.Empty deleteReview(io.github.panyam.agni.v1.webapi.DeleteReviewRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getDeleteReviewMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetReviewManifest resolves a stored checklist into a value for CreateReview. It parses AND
     * validates. So a client learns once, here, that its manifest is malformed, and not on every run.
     * A caller that already holds a manifest skips this call. The CLI is one, since it reads the YAML
     * the user named.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetReviewManifestResponse getReviewManifest(io.github.panyam.agni.v1.webapi.GetReviewManifestRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetReviewManifestMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListChecklists returns the checklists a design's project declares, inherited ones included
     * (agni issue 859). They come in the order the project writes them, so the first is the
     * project's default. Every surface asks this rpc which checklists a design has. `agni review
     * --checklist &lt;name&gt;`, `agni checklists` and the viewer's picker all read it. A design in no
     * project answers an empty project and no checklists. A project declaring none answers its name
     * and no checklists. The two answers differ because the fixes differ. The first sends a manifest,
     * and the second adds a checklists section. A descriptor that exists and does not parse is an
     * error. The checklists are values, so a client runs one by sending it to CreateReview.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListChecklistsResponse listChecklists(io.github.panyam.agni.v1.webapi.ListChecklistsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListChecklistsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * RenderReviewReport takes a review the caller already holds and writes it as a checklist page
     * (agni issue 1030). `agni review --format html` writes the same page. The page lists the
     * manifest's areas and items in the manifest's order, each with its outcome and every finding
     * behind it. It runs nothing and reads no store. So a run that was never stored (validate_only)
     * renders as a stored one does. The CLI's page and the viewer's saved page both come from this
     * one renderer.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.RenderReviewReportResponse renderReviewReport(io.github.panyam.agni.v1.webapi.RenderReviewReportRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getRenderReviewReportMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service ReviewService.
   * <pre>
   * ReviewService manages review RUNS as resources (WS9-053). A run used to be a call. You sent a
   * checklist and a design, got outcomes back, and nothing remained. A review does not work that
   * way. A team acts on a checklist verdict over weeks. It compares verdicts between revisions, and
   * gates a release on one. Every one of those uses needs the run to still exist tomorrow.
   * So a client creates, fetches, lists, and deletes runs. A run produces a self-contained
   * agni.v1.checks.CheckResults document, the same one `agni review --results-out` writes. The
   * document holds everything needed to render the run again, including the checklist snapshot. So
   * a review from last quarter renders against the checklist it actually scored, and not against
   * whatever that file says today.
   * This service is AIP-shaped on purpose, while the rest of this API is verb-shaped (see
   * CONSTRAINTS C23). The difference is not one of style. Every other rpc here is a pure function of
   * the snapshot it names, so `GetSummary(snapshot)` names its whole input. A review run has a lifetime
   * independent of its inputs, so it needs an identity of its own.
   * The overlay knobs (--profile-path / --intent-path / --params) are still serve-startup config.
   * The service composes them once into the catalog it holds. So a create request sends the
   * checklist, names the design's snapshot, and sets the ratified-data floor.
   * </pre>
   */
  public static final class ReviewServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<ReviewServiceBlockingStub> {
    private ReviewServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ReviewServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ReviewServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * CreateReview runs the checklist against the design, stores the result, and returns the stored
     * Review. The run is all-or-nothing. An invalid or absent manifest is an error, and so is a
     * snapshot whose design cannot be read. So a partial read never
     * reports items clean without checking them. Nothing is stored when the run fails.
     * It is synchronous. A review is a check sweep per item, and it finishes within the time a
     * request can wait. So this returns the Review itself rather than an Operation. If that stops
     * being true, the answer is AIP-151, not a client-side poll invented here.
     * One review is about ONE design. A project rollup is several Reviews. A document holds a single
     * DesignRef on purpose, because averaging several designs into one document would misrepresent
     * every one of them.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Review createReview(io.github.panyam.agni.v1.webapi.CreateReviewRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCreateReviewMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetReview returns a stored run by name. The document is self-contained, so this needs neither the
     * design nor the checklist to still exist.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Review getReview(io.github.panyam.agni.v1.webapi.GetReviewRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetReviewMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListReviews returns stored runs, newest first. Results are paginated. `filter` narrows them to
     * one design, so a client can ask the question it actually has ("how has THIS board been
     * trending").
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListReviewsResponse listReviews(io.github.panyam.agni.v1.webapi.ListReviewsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListReviewsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteReview removes a stored run. Deleting a run that is already absent is an error, not a
     * silent success. A client asking to delete something that is not there has a stale view, and
     * should be told so.
     * </pre>
     */
    public com.google.protobuf.Empty deleteReview(io.github.panyam.agni.v1.webapi.DeleteReviewRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeleteReviewMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetReviewManifest resolves a stored checklist into a value for CreateReview. It parses AND
     * validates. So a client learns once, here, that its manifest is malformed, and not on every run.
     * A caller that already holds a manifest skips this call. The CLI is one, since it reads the YAML
     * the user named.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetReviewManifestResponse getReviewManifest(io.github.panyam.agni.v1.webapi.GetReviewManifestRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetReviewManifestMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListChecklists returns the checklists a design's project declares, inherited ones included
     * (agni issue 859). They come in the order the project writes them, so the first is the
     * project's default. Every surface asks this rpc which checklists a design has. `agni review
     * --checklist &lt;name&gt;`, `agni checklists` and the viewer's picker all read it. A design in no
     * project answers an empty project and no checklists. A project declaring none answers its name
     * and no checklists. The two answers differ because the fixes differ. The first sends a manifest,
     * and the second adds a checklists section. A descriptor that exists and does not parse is an
     * error. The checklists are values, so a client runs one by sending it to CreateReview.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListChecklistsResponse listChecklists(io.github.panyam.agni.v1.webapi.ListChecklistsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListChecklistsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * RenderReviewReport takes a review the caller already holds and writes it as a checklist page
     * (agni issue 1030). `agni review --format html` writes the same page. The page lists the
     * manifest's areas and items in the manifest's order, each with its outcome and every finding
     * behind it. It runs nothing and reads no store. So a run that was never stored (validate_only)
     * renders as a stored one does. The CLI's page and the viewer's saved page both come from this
     * one renderer.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.RenderReviewReportResponse renderReviewReport(io.github.panyam.agni.v1.webapi.RenderReviewReportRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRenderReviewReportMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service ReviewService.
   * <pre>
   * ReviewService manages review RUNS as resources (WS9-053). A run used to be a call. You sent a
   * checklist and a design, got outcomes back, and nothing remained. A review does not work that
   * way. A team acts on a checklist verdict over weeks. It compares verdicts between revisions, and
   * gates a release on one. Every one of those uses needs the run to still exist tomorrow.
   * So a client creates, fetches, lists, and deletes runs. A run produces a self-contained
   * agni.v1.checks.CheckResults document, the same one `agni review --results-out` writes. The
   * document holds everything needed to render the run again, including the checklist snapshot. So
   * a review from last quarter renders against the checklist it actually scored, and not against
   * whatever that file says today.
   * This service is AIP-shaped on purpose, while the rest of this API is verb-shaped (see
   * CONSTRAINTS C23). The difference is not one of style. Every other rpc here is a pure function of
   * the snapshot it names, so `GetSummary(snapshot)` names its whole input. A review run has a lifetime
   * independent of its inputs, so it needs an identity of its own.
   * The overlay knobs (--profile-path / --intent-path / --params) are still serve-startup config.
   * The service composes them once into the catalog it holds. So a create request sends the
   * checklist, names the design's snapshot, and sets the ratified-data floor.
   * </pre>
   */
  public static final class ReviewServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<ReviewServiceFutureStub> {
    private ReviewServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ReviewServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ReviewServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * CreateReview runs the checklist against the design, stores the result, and returns the stored
     * Review. The run is all-or-nothing. An invalid or absent manifest is an error, and so is a
     * snapshot whose design cannot be read. So a partial read never
     * reports items clean without checking them. Nothing is stored when the run fails.
     * It is synchronous. A review is a check sweep per item, and it finishes within the time a
     * request can wait. So this returns the Review itself rather than an Operation. If that stops
     * being true, the answer is AIP-151, not a client-side poll invented here.
     * One review is about ONE design. A project rollup is several Reviews. A document holds a single
     * DesignRef on purpose, because averaging several designs into one document would misrepresent
     * every one of them.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Review> createReview(
        io.github.panyam.agni.v1.webapi.CreateReviewRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCreateReviewMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetReview returns a stored run by name. The document is self-contained, so this needs neither the
     * design nor the checklist to still exist.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Review> getReview(
        io.github.panyam.agni.v1.webapi.GetReviewRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetReviewMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ListReviews returns stored runs, newest first. Results are paginated. `filter` narrows them to
     * one design, so a client can ask the question it actually has ("how has THIS board been
     * trending").
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListReviewsResponse> listReviews(
        io.github.panyam.agni.v1.webapi.ListReviewsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListReviewsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * DeleteReview removes a stored run. Deleting a run that is already absent is an error, not a
     * silent success. A client asking to delete something that is not there has a stale view, and
     * should be told so.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.google.protobuf.Empty> deleteReview(
        io.github.panyam.agni.v1.webapi.DeleteReviewRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeleteReviewMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetReviewManifest resolves a stored checklist into a value for CreateReview. It parses AND
     * validates. So a client learns once, here, that its manifest is malformed, and not on every run.
     * A caller that already holds a manifest skips this call. The CLI is one, since it reads the YAML
     * the user named.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.GetReviewManifestResponse> getReviewManifest(
        io.github.panyam.agni.v1.webapi.GetReviewManifestRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetReviewManifestMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ListChecklists returns the checklists a design's project declares, inherited ones included
     * (agni issue 859). They come in the order the project writes them, so the first is the
     * project's default. Every surface asks this rpc which checklists a design has. `agni review
     * --checklist &lt;name&gt;`, `agni checklists` and the viewer's picker all read it. A design in no
     * project answers an empty project and no checklists. A project declaring none answers its name
     * and no checklists. The two answers differ because the fixes differ. The first sends a manifest,
     * and the second adds a checklists section. A descriptor that exists and does not parse is an
     * error. The checklists are values, so a client runs one by sending it to CreateReview.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListChecklistsResponse> listChecklists(
        io.github.panyam.agni.v1.webapi.ListChecklistsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListChecklistsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * RenderReviewReport takes a review the caller already holds and writes it as a checklist page
     * (agni issue 1030). `agni review --format html` writes the same page. The page lists the
     * manifest's areas and items in the manifest's order, each with its outcome and every finding
     * behind it. It runs nothing and reads no store. So a run that was never stored (validate_only)
     * renders as a stored one does. The CLI's page and the viewer's saved page both come from this
     * one renderer.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.RenderReviewReportResponse> renderReviewReport(
        io.github.panyam.agni.v1.webapi.RenderReviewReportRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRenderReviewReportMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_CREATE_REVIEW = 0;
  private static final int METHODID_GET_REVIEW = 1;
  private static final int METHODID_LIST_REVIEWS = 2;
  private static final int METHODID_DELETE_REVIEW = 3;
  private static final int METHODID_GET_REVIEW_MANIFEST = 4;
  private static final int METHODID_LIST_CHECKLISTS = 5;
  private static final int METHODID_RENDER_REVIEW_REPORT = 6;

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
        case METHODID_CREATE_REVIEW:
          serviceImpl.createReview((io.github.panyam.agni.v1.webapi.CreateReviewRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Review>) responseObserver);
          break;
        case METHODID_GET_REVIEW:
          serviceImpl.getReview((io.github.panyam.agni.v1.webapi.GetReviewRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Review>) responseObserver);
          break;
        case METHODID_LIST_REVIEWS:
          serviceImpl.listReviews((io.github.panyam.agni.v1.webapi.ListReviewsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListReviewsResponse>) responseObserver);
          break;
        case METHODID_DELETE_REVIEW:
          serviceImpl.deleteReview((io.github.panyam.agni.v1.webapi.DeleteReviewRequest) request,
              (io.grpc.stub.StreamObserver<com.google.protobuf.Empty>) responseObserver);
          break;
        case METHODID_GET_REVIEW_MANIFEST:
          serviceImpl.getReviewManifest((io.github.panyam.agni.v1.webapi.GetReviewManifestRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetReviewManifestResponse>) responseObserver);
          break;
        case METHODID_LIST_CHECKLISTS:
          serviceImpl.listChecklists((io.github.panyam.agni.v1.webapi.ListChecklistsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListChecklistsResponse>) responseObserver);
          break;
        case METHODID_RENDER_REVIEW_REPORT:
          serviceImpl.renderReviewReport((io.github.panyam.agni.v1.webapi.RenderReviewReportRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RenderReviewReportResponse>) responseObserver);
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
          getCreateReviewMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.CreateReviewRequest,
              io.github.panyam.agni.v1.webapi.Review>(
                service, METHODID_CREATE_REVIEW)))
        .addMethod(
          getGetReviewMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetReviewRequest,
              io.github.panyam.agni.v1.webapi.Review>(
                service, METHODID_GET_REVIEW)))
        .addMethod(
          getListReviewsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListReviewsRequest,
              io.github.panyam.agni.v1.webapi.ListReviewsResponse>(
                service, METHODID_LIST_REVIEWS)))
        .addMethod(
          getDeleteReviewMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.DeleteReviewRequest,
              com.google.protobuf.Empty>(
                service, METHODID_DELETE_REVIEW)))
        .addMethod(
          getGetReviewManifestMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetReviewManifestRequest,
              io.github.panyam.agni.v1.webapi.GetReviewManifestResponse>(
                service, METHODID_GET_REVIEW_MANIFEST)))
        .addMethod(
          getListChecklistsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListChecklistsRequest,
              io.github.panyam.agni.v1.webapi.ListChecklistsResponse>(
                service, METHODID_LIST_CHECKLISTS)))
        .addMethod(
          getRenderReviewReportMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.RenderReviewReportRequest,
              io.github.panyam.agni.v1.webapi.RenderReviewReportResponse>(
                service, METHODID_RENDER_REVIEW_REPORT)))
        .build();
  }

  private static abstract class ReviewServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    ReviewServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.ReviewProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("ReviewService");
    }
  }

  private static final class ReviewServiceFileDescriptorSupplier
      extends ReviewServiceBaseDescriptorSupplier {
    ReviewServiceFileDescriptorSupplier() {}
  }

  private static final class ReviewServiceMethodDescriptorSupplier
      extends ReviewServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    ReviewServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (ReviewServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new ReviewServiceFileDescriptorSupplier())
              .addMethod(getCreateReviewMethod())
              .addMethod(getGetReviewMethod())
              .addMethod(getListReviewsMethod())
              .addMethod(getDeleteReviewMethod())
              .addMethod(getGetReviewManifestMethod())
              .addMethod(getListChecklistsMethod())
              .addMethod(getRenderReviewReportMethod())
              .build();
        }
      }
    }
    return result;
  }
}
