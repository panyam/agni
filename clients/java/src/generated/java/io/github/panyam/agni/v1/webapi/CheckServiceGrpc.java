package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * CheckService runs the rule catalog (the check/ library) over a design's netlist IR. It also
 * serves the catalog itself. A geometry-only file with no netlist is an invalid argument.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class CheckServiceGrpc {

  private CheckServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.CheckService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListRulesRequest,
      io.github.panyam.agni.v1.webapi.ListRulesResponse> getListRulesMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListRules",
      requestType = io.github.panyam.agni.v1.webapi.ListRulesRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListRulesResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListRulesRequest,
      io.github.panyam.agni.v1.webapi.ListRulesResponse> getListRulesMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListRulesRequest, io.github.panyam.agni.v1.webapi.ListRulesResponse> getListRulesMethod;
    if ((getListRulesMethod = CheckServiceGrpc.getListRulesMethod) == null) {
      synchronized (CheckServiceGrpc.class) {
        if ((getListRulesMethod = CheckServiceGrpc.getListRulesMethod) == null) {
          CheckServiceGrpc.getListRulesMethod = getListRulesMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListRulesRequest, io.github.panyam.agni.v1.webapi.ListRulesResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListRules"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListRulesRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListRulesResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CheckServiceMethodDescriptorSupplier("ListRules"))
              .build();
        }
      }
    }
    return getListRulesMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CheckDesignRequest,
      io.github.panyam.agni.v1.webapi.CheckDesignResponse> getCheckDesignMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CheckDesign",
      requestType = io.github.panyam.agni.v1.webapi.CheckDesignRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.CheckDesignResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CheckDesignRequest,
      io.github.panyam.agni.v1.webapi.CheckDesignResponse> getCheckDesignMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CheckDesignRequest, io.github.panyam.agni.v1.webapi.CheckDesignResponse> getCheckDesignMethod;
    if ((getCheckDesignMethod = CheckServiceGrpc.getCheckDesignMethod) == null) {
      synchronized (CheckServiceGrpc.class) {
        if ((getCheckDesignMethod = CheckServiceGrpc.getCheckDesignMethod) == null) {
          CheckServiceGrpc.getCheckDesignMethod = getCheckDesignMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.CheckDesignRequest, io.github.panyam.agni.v1.webapi.CheckDesignResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CheckDesign"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.CheckDesignRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.CheckDesignResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CheckServiceMethodDescriptorSupplier("CheckDesign"))
              .build();
        }
      }
    }
    return getCheckDesignMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetExpectationsRequest,
      io.github.panyam.agni.v1.webapi.GetExpectationsResponse> getGetExpectationsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetExpectations",
      requestType = io.github.panyam.agni.v1.webapi.GetExpectationsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.GetExpectationsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetExpectationsRequest,
      io.github.panyam.agni.v1.webapi.GetExpectationsResponse> getGetExpectationsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetExpectationsRequest, io.github.panyam.agni.v1.webapi.GetExpectationsResponse> getGetExpectationsMethod;
    if ((getGetExpectationsMethod = CheckServiceGrpc.getGetExpectationsMethod) == null) {
      synchronized (CheckServiceGrpc.class) {
        if ((getGetExpectationsMethod = CheckServiceGrpc.getGetExpectationsMethod) == null) {
          CheckServiceGrpc.getGetExpectationsMethod = getGetExpectationsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetExpectationsRequest, io.github.panyam.agni.v1.webapi.GetExpectationsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetExpectations"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetExpectationsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetExpectationsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CheckServiceMethodDescriptorSupplier("GetExpectations"))
              .build();
        }
      }
    }
    return getGetExpectationsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetCheckReportRequest,
      io.github.panyam.agni.v1.webapi.GetCheckReportResponse> getGetCheckReportMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetCheckReport",
      requestType = io.github.panyam.agni.v1.webapi.GetCheckReportRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.GetCheckReportResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetCheckReportRequest,
      io.github.panyam.agni.v1.webapi.GetCheckReportResponse> getGetCheckReportMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetCheckReportRequest, io.github.panyam.agni.v1.webapi.GetCheckReportResponse> getGetCheckReportMethod;
    if ((getGetCheckReportMethod = CheckServiceGrpc.getGetCheckReportMethod) == null) {
      synchronized (CheckServiceGrpc.class) {
        if ((getGetCheckReportMethod = CheckServiceGrpc.getGetCheckReportMethod) == null) {
          CheckServiceGrpc.getGetCheckReportMethod = getGetCheckReportMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetCheckReportRequest, io.github.panyam.agni.v1.webapi.GetCheckReportResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetCheckReport"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetCheckReportRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetCheckReportResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CheckServiceMethodDescriptorSupplier("GetCheckReport"))
              .build();
        }
      }
    }
    return getGetCheckReportMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RenderCheckReportRequest,
      io.github.panyam.agni.v1.webapi.RenderCheckReportResponse> getRenderCheckReportMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "RenderCheckReport",
      requestType = io.github.panyam.agni.v1.webapi.RenderCheckReportRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.RenderCheckReportResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RenderCheckReportRequest,
      io.github.panyam.agni.v1.webapi.RenderCheckReportResponse> getRenderCheckReportMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.RenderCheckReportRequest, io.github.panyam.agni.v1.webapi.RenderCheckReportResponse> getRenderCheckReportMethod;
    if ((getRenderCheckReportMethod = CheckServiceGrpc.getRenderCheckReportMethod) == null) {
      synchronized (CheckServiceGrpc.class) {
        if ((getRenderCheckReportMethod = CheckServiceGrpc.getRenderCheckReportMethod) == null) {
          CheckServiceGrpc.getRenderCheckReportMethod = getRenderCheckReportMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.RenderCheckReportRequest, io.github.panyam.agni.v1.webapi.RenderCheckReportResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "RenderCheckReport"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.RenderCheckReportRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.RenderCheckReportResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CheckServiceMethodDescriptorSupplier("RenderCheckReport"))
              .build();
        }
      }
    }
    return getRenderCheckReportMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetInterfaceCoverageRequest,
      io.github.panyam.agni.v1.webapi.GetInterfaceCoverageResponse> getGetInterfaceCoverageMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetInterfaceCoverage",
      requestType = io.github.panyam.agni.v1.webapi.GetInterfaceCoverageRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.GetInterfaceCoverageResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetInterfaceCoverageRequest,
      io.github.panyam.agni.v1.webapi.GetInterfaceCoverageResponse> getGetInterfaceCoverageMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetInterfaceCoverageRequest, io.github.panyam.agni.v1.webapi.GetInterfaceCoverageResponse> getGetInterfaceCoverageMethod;
    if ((getGetInterfaceCoverageMethod = CheckServiceGrpc.getGetInterfaceCoverageMethod) == null) {
      synchronized (CheckServiceGrpc.class) {
        if ((getGetInterfaceCoverageMethod = CheckServiceGrpc.getGetInterfaceCoverageMethod) == null) {
          CheckServiceGrpc.getGetInterfaceCoverageMethod = getGetInterfaceCoverageMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetInterfaceCoverageRequest, io.github.panyam.agni.v1.webapi.GetInterfaceCoverageResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetInterfaceCoverage"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetInterfaceCoverageRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetInterfaceCoverageResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CheckServiceMethodDescriptorSupplier("GetInterfaceCoverage"))
              .build();
        }
      }
    }
    return getGetInterfaceCoverageMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetComponentParamsRequest,
      io.github.panyam.agni.v1.webapi.GetComponentParamsResponse> getGetComponentParamsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetComponentParams",
      requestType = io.github.panyam.agni.v1.webapi.GetComponentParamsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.GetComponentParamsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetComponentParamsRequest,
      io.github.panyam.agni.v1.webapi.GetComponentParamsResponse> getGetComponentParamsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetComponentParamsRequest, io.github.panyam.agni.v1.webapi.GetComponentParamsResponse> getGetComponentParamsMethod;
    if ((getGetComponentParamsMethod = CheckServiceGrpc.getGetComponentParamsMethod) == null) {
      synchronized (CheckServiceGrpc.class) {
        if ((getGetComponentParamsMethod = CheckServiceGrpc.getGetComponentParamsMethod) == null) {
          CheckServiceGrpc.getGetComponentParamsMethod = getGetComponentParamsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetComponentParamsRequest, io.github.panyam.agni.v1.webapi.GetComponentParamsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetComponentParams"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetComponentParamsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetComponentParamsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CheckServiceMethodDescriptorSupplier("GetComponentParams"))
              .build();
        }
      }
    }
    return getGetComponentParamsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetNamingConventionRequest,
      io.github.panyam.agni.v1.webapi.GetNamingConventionResponse> getGetNamingConventionMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetNamingConvention",
      requestType = io.github.panyam.agni.v1.webapi.GetNamingConventionRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.GetNamingConventionResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetNamingConventionRequest,
      io.github.panyam.agni.v1.webapi.GetNamingConventionResponse> getGetNamingConventionMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetNamingConventionRequest, io.github.panyam.agni.v1.webapi.GetNamingConventionResponse> getGetNamingConventionMethod;
    if ((getGetNamingConventionMethod = CheckServiceGrpc.getGetNamingConventionMethod) == null) {
      synchronized (CheckServiceGrpc.class) {
        if ((getGetNamingConventionMethod = CheckServiceGrpc.getGetNamingConventionMethod) == null) {
          CheckServiceGrpc.getGetNamingConventionMethod = getGetNamingConventionMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetNamingConventionRequest, io.github.panyam.agni.v1.webapi.GetNamingConventionResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetNamingConvention"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetNamingConventionRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetNamingConventionResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CheckServiceMethodDescriptorSupplier("GetNamingConvention"))
              .build();
        }
      }
    }
    return getGetNamingConventionMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static CheckServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<CheckServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<CheckServiceStub>() {
        @java.lang.Override
        public CheckServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new CheckServiceStub(channel, callOptions);
        }
      };
    return CheckServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static CheckServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<CheckServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<CheckServiceBlockingV2Stub>() {
        @java.lang.Override
        public CheckServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new CheckServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return CheckServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static CheckServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<CheckServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<CheckServiceBlockingStub>() {
        @java.lang.Override
        public CheckServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new CheckServiceBlockingStub(channel, callOptions);
        }
      };
    return CheckServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static CheckServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<CheckServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<CheckServiceFutureStub>() {
        @java.lang.Override
        public CheckServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new CheckServiceFutureStub(channel, callOptions);
        }
      };
    return CheckServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * CheckService runs the rule catalog (the check/ library) over a design's netlist IR. It also
   * serves the catalog itself. A geometry-only file with no netlist is an invalid argument.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * ListRules returns the rule catalog that a check of the snapshot runs. Each rule gets one
     * RuleInfo, with its tags and whether it runs on the design. The tags are an open classification,
     * and the client groups and filters by them. A rule is available exactly when a whole CheckDesign
     * of the same snapshot would not report it in skipped (agni issue 950). Deciding that reads the
     * netlist and not the board. A CheckDesign call names the subset to actually run.
     * </pre>
     */
    default void listRules(io.github.panyam.agni.v1.webapi.ListRulesRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListRulesResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListRulesMethod(), responseObserver);
    }

    /**
     * <pre>
     * CheckDesign runs the rule checks over a loaded design and returns the findings. Checks
     * run on demand and can be slower than a metadata fetch, so loading a design and validating it
     * are separate calls. request.rules selects the subset to run (empty = all). The viewer joins
     * on each finding's subject (kind + ref) to highlight the offending element, since the subject
     * matches PackedSheet primitive keys.
     * </pre>
     */
    default void checkDesign(io.github.panyam.agni.v1.webapi.CheckDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.CheckDesignResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCheckDesignMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetExpectations returns a design's expected findings from its `&lt;design&gt;.expect.yaml` sidecar
     * (WS6-006), if one sits beside the file. Expectations are static per design. They do not depend
     * on the rule subset a CheckDesign run uses, so they are their own resource. The viewer fetches
     * them once and reconciles them client-side against the CheckDesign findings. It shows each
     * expected rule as matched, missing (not yet implemented) or pending. A design with no sidecar
     * returns an empty list rather than an error, so the panel degrades gracefully.
     * </pre>
     */
    default void getExpectations(io.github.panyam.agni.v1.webapi.GetExpectationsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetExpectationsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetExpectationsMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetCheckReport runs the same checks as CheckDesign and returns the severity-organized report
     * (WS3-022). Sections come worst-severity first. Findings are grouped by rule, and each group
     * includes the catalog summary. The report is its own message rather than a client-side pivot
     * of CheckDesign. So the CLI's markdown/report formats and the web report panel render one
     * canonical shape.
     * </pre>
     */
    default void getCheckReport(io.github.panyam.agni.v1.webapi.GetCheckReportRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetCheckReportResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetCheckReportMethod(), responseObserver);
    }

    /**
     * <pre>
     * RenderCheckReport writes a check run as a file to keep (agni issue 127). The caller already
     * holds the run. The file is either the CheckResults document from `agni check --results-out` or
     * the verdict report from `agni check --verdicts --format html`. It runs no rules. The engine
     * renders the bytes. So a report saved from the viewer and one written by the CLI are the same
     * file apart from their timestamps, and no second renderer exists to drift from this one.
     * </pre>
     */
    default void renderCheckReport(io.github.panyam.agni.v1.webapi.RenderCheckReportRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RenderCheckReportResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRenderCheckReportMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetInterfaceCoverage projects the interface-profile mechanism (WS3-034) into a per-interface
     * coverage matrix (WS9-041). A profile is DETECTED when its in-use confidence gate holds, so two
     * of its signals are present or a host declares it. For every detected profile, the matrix lists
     * each required signal with its matched net and its state (present, missing, dangling or
     * pullup_missing). It reuses the datalog that the profile rules compile to, so the panel and the
     * findings cannot disagree. A design with no detected interface returns an empty list rather than
     * an error. It is silent by construction, like the rules themselves.
     * </pre>
     */
    default void getInterfaceCoverage(io.github.panyam.agni.v1.webapi.GetInterfaceCoverageRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetInterfaceCoverageResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetInterfaceCoverageMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetComponentParams shows the datasheet join (WS10-003) read-only, per design. It lists every
     * component whose MPN resolves to a seeded PartSpec, with that spec's parameters. It is the browse
     * surface for "what does the engine know about my parts" (WS9-035). The datasheet rules read the
     * same join, and this rpc exposes it for a UI tree rather than querying it again. An empty list
     * comes back, rather than an error, for a design with no joined specs or a serve started without
     * --params. So the panel degrades gracefully.
     * </pre>
     */
    default void getComponentParams(io.github.panyam.agni.v1.webapi.GetComponentParamsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetComponentParamsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetComponentParamsMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetNamingConvention resolves a stored convention config into a VALUE for an OverlayConfig
     * (WS9-128). It parses and validates, so a client learns once, here, that its config is malformed,
     * rather than on every run that sends it.
     * It exists for the same reason as GetReviewManifest. OverlayConfig deliberately holds values
     * rather than paths (C22), so a service composing one needs no filesystem. A browser, though,
     * holds a ref and has no filesystem either, and something has to bridge the two. As its own rpc,
     * the bridge keeps the read named in the contract instead of hidden inside a check run. A caller
     * that already holds a convention never calls it. The CLI is one, since it reads the YAML the user
     * named.
     * </pre>
     */
    default void getNamingConvention(io.github.panyam.agni.v1.webapi.GetNamingConventionRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetNamingConventionResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetNamingConventionMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service CheckService.
   * <pre>
   * CheckService runs the rule catalog (the check/ library) over a design's netlist IR. It also
   * serves the catalog itself. A geometry-only file with no netlist is an invalid argument.
   * </pre>
   */
  public static abstract class CheckServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return CheckServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service CheckService.
   * <pre>
   * CheckService runs the rule catalog (the check/ library) over a design's netlist IR. It also
   * serves the catalog itself. A geometry-only file with no netlist is an invalid argument.
   * </pre>
   */
  public static final class CheckServiceStub
      extends io.grpc.stub.AbstractAsyncStub<CheckServiceStub> {
    private CheckServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected CheckServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new CheckServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * ListRules returns the rule catalog that a check of the snapshot runs. Each rule gets one
     * RuleInfo, with its tags and whether it runs on the design. The tags are an open classification,
     * and the client groups and filters by them. A rule is available exactly when a whole CheckDesign
     * of the same snapshot would not report it in skipped (agni issue 950). Deciding that reads the
     * netlist and not the board. A CheckDesign call names the subset to actually run.
     * </pre>
     */
    public void listRules(io.github.panyam.agni.v1.webapi.ListRulesRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListRulesResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListRulesMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * CheckDesign runs the rule checks over a loaded design and returns the findings. Checks
     * run on demand and can be slower than a metadata fetch, so loading a design and validating it
     * are separate calls. request.rules selects the subset to run (empty = all). The viewer joins
     * on each finding's subject (kind + ref) to highlight the offending element, since the subject
     * matches PackedSheet primitive keys.
     * </pre>
     */
    public void checkDesign(io.github.panyam.agni.v1.webapi.CheckDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.CheckDesignResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCheckDesignMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetExpectations returns a design's expected findings from its `&lt;design&gt;.expect.yaml` sidecar
     * (WS6-006), if one sits beside the file. Expectations are static per design. They do not depend
     * on the rule subset a CheckDesign run uses, so they are their own resource. The viewer fetches
     * them once and reconciles them client-side against the CheckDesign findings. It shows each
     * expected rule as matched, missing (not yet implemented) or pending. A design with no sidecar
     * returns an empty list rather than an error, so the panel degrades gracefully.
     * </pre>
     */
    public void getExpectations(io.github.panyam.agni.v1.webapi.GetExpectationsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetExpectationsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetExpectationsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetCheckReport runs the same checks as CheckDesign and returns the severity-organized report
     * (WS3-022). Sections come worst-severity first. Findings are grouped by rule, and each group
     * includes the catalog summary. The report is its own message rather than a client-side pivot
     * of CheckDesign. So the CLI's markdown/report formats and the web report panel render one
     * canonical shape.
     * </pre>
     */
    public void getCheckReport(io.github.panyam.agni.v1.webapi.GetCheckReportRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetCheckReportResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetCheckReportMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * RenderCheckReport writes a check run as a file to keep (agni issue 127). The caller already
     * holds the run. The file is either the CheckResults document from `agni check --results-out` or
     * the verdict report from `agni check --verdicts --format html`. It runs no rules. The engine
     * renders the bytes. So a report saved from the viewer and one written by the CLI are the same
     * file apart from their timestamps, and no second renderer exists to drift from this one.
     * </pre>
     */
    public void renderCheckReport(io.github.panyam.agni.v1.webapi.RenderCheckReportRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RenderCheckReportResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRenderCheckReportMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetInterfaceCoverage projects the interface-profile mechanism (WS3-034) into a per-interface
     * coverage matrix (WS9-041). A profile is DETECTED when its in-use confidence gate holds, so two
     * of its signals are present or a host declares it. For every detected profile, the matrix lists
     * each required signal with its matched net and its state (present, missing, dangling or
     * pullup_missing). It reuses the datalog that the profile rules compile to, so the panel and the
     * findings cannot disagree. A design with no detected interface returns an empty list rather than
     * an error. It is silent by construction, like the rules themselves.
     * </pre>
     */
    public void getInterfaceCoverage(io.github.panyam.agni.v1.webapi.GetInterfaceCoverageRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetInterfaceCoverageResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetInterfaceCoverageMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetComponentParams shows the datasheet join (WS10-003) read-only, per design. It lists every
     * component whose MPN resolves to a seeded PartSpec, with that spec's parameters. It is the browse
     * surface for "what does the engine know about my parts" (WS9-035). The datasheet rules read the
     * same join, and this rpc exposes it for a UI tree rather than querying it again. An empty list
     * comes back, rather than an error, for a design with no joined specs or a serve started without
     * --params. So the panel degrades gracefully.
     * </pre>
     */
    public void getComponentParams(io.github.panyam.agni.v1.webapi.GetComponentParamsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetComponentParamsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetComponentParamsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetNamingConvention resolves a stored convention config into a VALUE for an OverlayConfig
     * (WS9-128). It parses and validates, so a client learns once, here, that its config is malformed,
     * rather than on every run that sends it.
     * It exists for the same reason as GetReviewManifest. OverlayConfig deliberately holds values
     * rather than paths (C22), so a service composing one needs no filesystem. A browser, though,
     * holds a ref and has no filesystem either, and something has to bridge the two. As its own rpc,
     * the bridge keeps the read named in the contract instead of hidden inside a check run. A caller
     * that already holds a convention never calls it. The CLI is one, since it reads the YAML the user
     * named.
     * </pre>
     */
    public void getNamingConvention(io.github.panyam.agni.v1.webapi.GetNamingConventionRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetNamingConventionResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetNamingConventionMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service CheckService.
   * <pre>
   * CheckService runs the rule catalog (the check/ library) over a design's netlist IR. It also
   * serves the catalog itself. A geometry-only file with no netlist is an invalid argument.
   * </pre>
   */
  public static final class CheckServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<CheckServiceBlockingV2Stub> {
    private CheckServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected CheckServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new CheckServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * ListRules returns the rule catalog that a check of the snapshot runs. Each rule gets one
     * RuleInfo, with its tags and whether it runs on the design. The tags are an open classification,
     * and the client groups and filters by them. A rule is available exactly when a whole CheckDesign
     * of the same snapshot would not report it in skipped (agni issue 950). Deciding that reads the
     * netlist and not the board. A CheckDesign call names the subset to actually run.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListRulesResponse listRules(io.github.panyam.agni.v1.webapi.ListRulesRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListRulesMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * CheckDesign runs the rule checks over a loaded design and returns the findings. Checks
     * run on demand and can be slower than a metadata fetch, so loading a design and validating it
     * are separate calls. request.rules selects the subset to run (empty = all). The viewer joins
     * on each finding's subject (kind + ref) to highlight the offending element, since the subject
     * matches PackedSheet primitive keys.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.CheckDesignResponse checkDesign(io.github.panyam.agni.v1.webapi.CheckDesignRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getCheckDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetExpectations returns a design's expected findings from its `&lt;design&gt;.expect.yaml` sidecar
     * (WS6-006), if one sits beside the file. Expectations are static per design. They do not depend
     * on the rule subset a CheckDesign run uses, so they are their own resource. The viewer fetches
     * them once and reconciles them client-side against the CheckDesign findings. It shows each
     * expected rule as matched, missing (not yet implemented) or pending. A design with no sidecar
     * returns an empty list rather than an error, so the panel degrades gracefully.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetExpectationsResponse getExpectations(io.github.panyam.agni.v1.webapi.GetExpectationsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetExpectationsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetCheckReport runs the same checks as CheckDesign and returns the severity-organized report
     * (WS3-022). Sections come worst-severity first. Findings are grouped by rule, and each group
     * includes the catalog summary. The report is its own message rather than a client-side pivot
     * of CheckDesign. So the CLI's markdown/report formats and the web report panel render one
     * canonical shape.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetCheckReportResponse getCheckReport(io.github.panyam.agni.v1.webapi.GetCheckReportRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetCheckReportMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * RenderCheckReport writes a check run as a file to keep (agni issue 127). The caller already
     * holds the run. The file is either the CheckResults document from `agni check --results-out` or
     * the verdict report from `agni check --verdicts --format html`. It runs no rules. The engine
     * renders the bytes. So a report saved from the viewer and one written by the CLI are the same
     * file apart from their timestamps, and no second renderer exists to drift from this one.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.RenderCheckReportResponse renderCheckReport(io.github.panyam.agni.v1.webapi.RenderCheckReportRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getRenderCheckReportMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetInterfaceCoverage projects the interface-profile mechanism (WS3-034) into a per-interface
     * coverage matrix (WS9-041). A profile is DETECTED when its in-use confidence gate holds, so two
     * of its signals are present or a host declares it. For every detected profile, the matrix lists
     * each required signal with its matched net and its state (present, missing, dangling or
     * pullup_missing). It reuses the datalog that the profile rules compile to, so the panel and the
     * findings cannot disagree. A design with no detected interface returns an empty list rather than
     * an error. It is silent by construction, like the rules themselves.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetInterfaceCoverageResponse getInterfaceCoverage(io.github.panyam.agni.v1.webapi.GetInterfaceCoverageRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetInterfaceCoverageMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetComponentParams shows the datasheet join (WS10-003) read-only, per design. It lists every
     * component whose MPN resolves to a seeded PartSpec, with that spec's parameters. It is the browse
     * surface for "what does the engine know about my parts" (WS9-035). The datasheet rules read the
     * same join, and this rpc exposes it for a UI tree rather than querying it again. An empty list
     * comes back, rather than an error, for a design with no joined specs or a serve started without
     * --params. So the panel degrades gracefully.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetComponentParamsResponse getComponentParams(io.github.panyam.agni.v1.webapi.GetComponentParamsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetComponentParamsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetNamingConvention resolves a stored convention config into a VALUE for an OverlayConfig
     * (WS9-128). It parses and validates, so a client learns once, here, that its config is malformed,
     * rather than on every run that sends it.
     * It exists for the same reason as GetReviewManifest. OverlayConfig deliberately holds values
     * rather than paths (C22), so a service composing one needs no filesystem. A browser, though,
     * holds a ref and has no filesystem either, and something has to bridge the two. As its own rpc,
     * the bridge keeps the read named in the contract instead of hidden inside a check run. A caller
     * that already holds a convention never calls it. The CLI is one, since it reads the YAML the user
     * named.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetNamingConventionResponse getNamingConvention(io.github.panyam.agni.v1.webapi.GetNamingConventionRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetNamingConventionMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service CheckService.
   * <pre>
   * CheckService runs the rule catalog (the check/ library) over a design's netlist IR. It also
   * serves the catalog itself. A geometry-only file with no netlist is an invalid argument.
   * </pre>
   */
  public static final class CheckServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<CheckServiceBlockingStub> {
    private CheckServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected CheckServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new CheckServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * ListRules returns the rule catalog that a check of the snapshot runs. Each rule gets one
     * RuleInfo, with its tags and whether it runs on the design. The tags are an open classification,
     * and the client groups and filters by them. A rule is available exactly when a whole CheckDesign
     * of the same snapshot would not report it in skipped (agni issue 950). Deciding that reads the
     * netlist and not the board. A CheckDesign call names the subset to actually run.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListRulesResponse listRules(io.github.panyam.agni.v1.webapi.ListRulesRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListRulesMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * CheckDesign runs the rule checks over a loaded design and returns the findings. Checks
     * run on demand and can be slower than a metadata fetch, so loading a design and validating it
     * are separate calls. request.rules selects the subset to run (empty = all). The viewer joins
     * on each finding's subject (kind + ref) to highlight the offending element, since the subject
     * matches PackedSheet primitive keys.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.CheckDesignResponse checkDesign(io.github.panyam.agni.v1.webapi.CheckDesignRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCheckDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetExpectations returns a design's expected findings from its `&lt;design&gt;.expect.yaml` sidecar
     * (WS6-006), if one sits beside the file. Expectations are static per design. They do not depend
     * on the rule subset a CheckDesign run uses, so they are their own resource. The viewer fetches
     * them once and reconciles them client-side against the CheckDesign findings. It shows each
     * expected rule as matched, missing (not yet implemented) or pending. A design with no sidecar
     * returns an empty list rather than an error, so the panel degrades gracefully.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetExpectationsResponse getExpectations(io.github.panyam.agni.v1.webapi.GetExpectationsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetExpectationsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetCheckReport runs the same checks as CheckDesign and returns the severity-organized report
     * (WS3-022). Sections come worst-severity first. Findings are grouped by rule, and each group
     * includes the catalog summary. The report is its own message rather than a client-side pivot
     * of CheckDesign. So the CLI's markdown/report formats and the web report panel render one
     * canonical shape.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetCheckReportResponse getCheckReport(io.github.panyam.agni.v1.webapi.GetCheckReportRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetCheckReportMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * RenderCheckReport writes a check run as a file to keep (agni issue 127). The caller already
     * holds the run. The file is either the CheckResults document from `agni check --results-out` or
     * the verdict report from `agni check --verdicts --format html`. It runs no rules. The engine
     * renders the bytes. So a report saved from the viewer and one written by the CLI are the same
     * file apart from their timestamps, and no second renderer exists to drift from this one.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.RenderCheckReportResponse renderCheckReport(io.github.panyam.agni.v1.webapi.RenderCheckReportRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRenderCheckReportMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetInterfaceCoverage projects the interface-profile mechanism (WS3-034) into a per-interface
     * coverage matrix (WS9-041). A profile is DETECTED when its in-use confidence gate holds, so two
     * of its signals are present or a host declares it. For every detected profile, the matrix lists
     * each required signal with its matched net and its state (present, missing, dangling or
     * pullup_missing). It reuses the datalog that the profile rules compile to, so the panel and the
     * findings cannot disagree. A design with no detected interface returns an empty list rather than
     * an error. It is silent by construction, like the rules themselves.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetInterfaceCoverageResponse getInterfaceCoverage(io.github.panyam.agni.v1.webapi.GetInterfaceCoverageRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetInterfaceCoverageMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetComponentParams shows the datasheet join (WS10-003) read-only, per design. It lists every
     * component whose MPN resolves to a seeded PartSpec, with that spec's parameters. It is the browse
     * surface for "what does the engine know about my parts" (WS9-035). The datasheet rules read the
     * same join, and this rpc exposes it for a UI tree rather than querying it again. An empty list
     * comes back, rather than an error, for a design with no joined specs or a serve started without
     * --params. So the panel degrades gracefully.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetComponentParamsResponse getComponentParams(io.github.panyam.agni.v1.webapi.GetComponentParamsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetComponentParamsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetNamingConvention resolves a stored convention config into a VALUE for an OverlayConfig
     * (WS9-128). It parses and validates, so a client learns once, here, that its config is malformed,
     * rather than on every run that sends it.
     * It exists for the same reason as GetReviewManifest. OverlayConfig deliberately holds values
     * rather than paths (C22), so a service composing one needs no filesystem. A browser, though,
     * holds a ref and has no filesystem either, and something has to bridge the two. As its own rpc,
     * the bridge keeps the read named in the contract instead of hidden inside a check run. A caller
     * that already holds a convention never calls it. The CLI is one, since it reads the YAML the user
     * named.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetNamingConventionResponse getNamingConvention(io.github.panyam.agni.v1.webapi.GetNamingConventionRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetNamingConventionMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service CheckService.
   * <pre>
   * CheckService runs the rule catalog (the check/ library) over a design's netlist IR. It also
   * serves the catalog itself. A geometry-only file with no netlist is an invalid argument.
   * </pre>
   */
  public static final class CheckServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<CheckServiceFutureStub> {
    private CheckServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected CheckServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new CheckServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * ListRules returns the rule catalog that a check of the snapshot runs. Each rule gets one
     * RuleInfo, with its tags and whether it runs on the design. The tags are an open classification,
     * and the client groups and filters by them. A rule is available exactly when a whole CheckDesign
     * of the same snapshot would not report it in skipped (agni issue 950). Deciding that reads the
     * netlist and not the board. A CheckDesign call names the subset to actually run.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListRulesResponse> listRules(
        io.github.panyam.agni.v1.webapi.ListRulesRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListRulesMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * CheckDesign runs the rule checks over a loaded design and returns the findings. Checks
     * run on demand and can be slower than a metadata fetch, so loading a design and validating it
     * are separate calls. request.rules selects the subset to run (empty = all). The viewer joins
     * on each finding's subject (kind + ref) to highlight the offending element, since the subject
     * matches PackedSheet primitive keys.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.CheckDesignResponse> checkDesign(
        io.github.panyam.agni.v1.webapi.CheckDesignRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCheckDesignMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetExpectations returns a design's expected findings from its `&lt;design&gt;.expect.yaml` sidecar
     * (WS6-006), if one sits beside the file. Expectations are static per design. They do not depend
     * on the rule subset a CheckDesign run uses, so they are their own resource. The viewer fetches
     * them once and reconciles them client-side against the CheckDesign findings. It shows each
     * expected rule as matched, missing (not yet implemented) or pending. A design with no sidecar
     * returns an empty list rather than an error, so the panel degrades gracefully.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.GetExpectationsResponse> getExpectations(
        io.github.panyam.agni.v1.webapi.GetExpectationsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetExpectationsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetCheckReport runs the same checks as CheckDesign and returns the severity-organized report
     * (WS3-022). Sections come worst-severity first. Findings are grouped by rule, and each group
     * includes the catalog summary. The report is its own message rather than a client-side pivot
     * of CheckDesign. So the CLI's markdown/report formats and the web report panel render one
     * canonical shape.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.GetCheckReportResponse> getCheckReport(
        io.github.panyam.agni.v1.webapi.GetCheckReportRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetCheckReportMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * RenderCheckReport writes a check run as a file to keep (agni issue 127). The caller already
     * holds the run. The file is either the CheckResults document from `agni check --results-out` or
     * the verdict report from `agni check --verdicts --format html`. It runs no rules. The engine
     * renders the bytes. So a report saved from the viewer and one written by the CLI are the same
     * file apart from their timestamps, and no second renderer exists to drift from this one.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.RenderCheckReportResponse> renderCheckReport(
        io.github.panyam.agni.v1.webapi.RenderCheckReportRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRenderCheckReportMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetInterfaceCoverage projects the interface-profile mechanism (WS3-034) into a per-interface
     * coverage matrix (WS9-041). A profile is DETECTED when its in-use confidence gate holds, so two
     * of its signals are present or a host declares it. For every detected profile, the matrix lists
     * each required signal with its matched net and its state (present, missing, dangling or
     * pullup_missing). It reuses the datalog that the profile rules compile to, so the panel and the
     * findings cannot disagree. A design with no detected interface returns an empty list rather than
     * an error. It is silent by construction, like the rules themselves.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.GetInterfaceCoverageResponse> getInterfaceCoverage(
        io.github.panyam.agni.v1.webapi.GetInterfaceCoverageRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetInterfaceCoverageMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetComponentParams shows the datasheet join (WS10-003) read-only, per design. It lists every
     * component whose MPN resolves to a seeded PartSpec, with that spec's parameters. It is the browse
     * surface for "what does the engine know about my parts" (WS9-035). The datasheet rules read the
     * same join, and this rpc exposes it for a UI tree rather than querying it again. An empty list
     * comes back, rather than an error, for a design with no joined specs or a serve started without
     * --params. So the panel degrades gracefully.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.GetComponentParamsResponse> getComponentParams(
        io.github.panyam.agni.v1.webapi.GetComponentParamsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetComponentParamsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetNamingConvention resolves a stored convention config into a VALUE for an OverlayConfig
     * (WS9-128). It parses and validates, so a client learns once, here, that its config is malformed,
     * rather than on every run that sends it.
     * It exists for the same reason as GetReviewManifest. OverlayConfig deliberately holds values
     * rather than paths (C22), so a service composing one needs no filesystem. A browser, though,
     * holds a ref and has no filesystem either, and something has to bridge the two. As its own rpc,
     * the bridge keeps the read named in the contract instead of hidden inside a check run. A caller
     * that already holds a convention never calls it. The CLI is one, since it reads the YAML the user
     * named.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.GetNamingConventionResponse> getNamingConvention(
        io.github.panyam.agni.v1.webapi.GetNamingConventionRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetNamingConventionMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_LIST_RULES = 0;
  private static final int METHODID_CHECK_DESIGN = 1;
  private static final int METHODID_GET_EXPECTATIONS = 2;
  private static final int METHODID_GET_CHECK_REPORT = 3;
  private static final int METHODID_RENDER_CHECK_REPORT = 4;
  private static final int METHODID_GET_INTERFACE_COVERAGE = 5;
  private static final int METHODID_GET_COMPONENT_PARAMS = 6;
  private static final int METHODID_GET_NAMING_CONVENTION = 7;

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
        case METHODID_LIST_RULES:
          serviceImpl.listRules((io.github.panyam.agni.v1.webapi.ListRulesRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListRulesResponse>) responseObserver);
          break;
        case METHODID_CHECK_DESIGN:
          serviceImpl.checkDesign((io.github.panyam.agni.v1.webapi.CheckDesignRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.CheckDesignResponse>) responseObserver);
          break;
        case METHODID_GET_EXPECTATIONS:
          serviceImpl.getExpectations((io.github.panyam.agni.v1.webapi.GetExpectationsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetExpectationsResponse>) responseObserver);
          break;
        case METHODID_GET_CHECK_REPORT:
          serviceImpl.getCheckReport((io.github.panyam.agni.v1.webapi.GetCheckReportRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetCheckReportResponse>) responseObserver);
          break;
        case METHODID_RENDER_CHECK_REPORT:
          serviceImpl.renderCheckReport((io.github.panyam.agni.v1.webapi.RenderCheckReportRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.RenderCheckReportResponse>) responseObserver);
          break;
        case METHODID_GET_INTERFACE_COVERAGE:
          serviceImpl.getInterfaceCoverage((io.github.panyam.agni.v1.webapi.GetInterfaceCoverageRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetInterfaceCoverageResponse>) responseObserver);
          break;
        case METHODID_GET_COMPONENT_PARAMS:
          serviceImpl.getComponentParams((io.github.panyam.agni.v1.webapi.GetComponentParamsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetComponentParamsResponse>) responseObserver);
          break;
        case METHODID_GET_NAMING_CONVENTION:
          serviceImpl.getNamingConvention((io.github.panyam.agni.v1.webapi.GetNamingConventionRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetNamingConventionResponse>) responseObserver);
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
          getListRulesMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListRulesRequest,
              io.github.panyam.agni.v1.webapi.ListRulesResponse>(
                service, METHODID_LIST_RULES)))
        .addMethod(
          getCheckDesignMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.CheckDesignRequest,
              io.github.panyam.agni.v1.webapi.CheckDesignResponse>(
                service, METHODID_CHECK_DESIGN)))
        .addMethod(
          getGetExpectationsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetExpectationsRequest,
              io.github.panyam.agni.v1.webapi.GetExpectationsResponse>(
                service, METHODID_GET_EXPECTATIONS)))
        .addMethod(
          getGetCheckReportMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetCheckReportRequest,
              io.github.panyam.agni.v1.webapi.GetCheckReportResponse>(
                service, METHODID_GET_CHECK_REPORT)))
        .addMethod(
          getRenderCheckReportMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.RenderCheckReportRequest,
              io.github.panyam.agni.v1.webapi.RenderCheckReportResponse>(
                service, METHODID_RENDER_CHECK_REPORT)))
        .addMethod(
          getGetInterfaceCoverageMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetInterfaceCoverageRequest,
              io.github.panyam.agni.v1.webapi.GetInterfaceCoverageResponse>(
                service, METHODID_GET_INTERFACE_COVERAGE)))
        .addMethod(
          getGetComponentParamsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetComponentParamsRequest,
              io.github.panyam.agni.v1.webapi.GetComponentParamsResponse>(
                service, METHODID_GET_COMPONENT_PARAMS)))
        .addMethod(
          getGetNamingConventionMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetNamingConventionRequest,
              io.github.panyam.agni.v1.webapi.GetNamingConventionResponse>(
                service, METHODID_GET_NAMING_CONVENTION)))
        .build();
  }

  private static abstract class CheckServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    CheckServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.ChecksProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("CheckService");
    }
  }

  private static final class CheckServiceFileDescriptorSupplier
      extends CheckServiceBaseDescriptorSupplier {
    CheckServiceFileDescriptorSupplier() {}
  }

  private static final class CheckServiceMethodDescriptorSupplier
      extends CheckServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    CheckServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (CheckServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new CheckServiceFileDescriptorSupplier())
              .addMethod(getListRulesMethod())
              .addMethod(getCheckDesignMethod())
              .addMethod(getGetExpectationsMethod())
              .addMethod(getGetCheckReportMethod())
              .addMethod(getRenderCheckReportMethod())
              .addMethod(getGetInterfaceCoverageMethod())
              .addMethod(getGetComponentParamsMethod())
              .addMethod(getGetNamingConventionMethod())
              .build();
        }
      }
    }
    return result;
  }
}
