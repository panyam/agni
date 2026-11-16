package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * DesignService loads a design file into something the viewer can draw. It reuses the
 * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
 * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
 * other supported format is drawn as an auto-layout of its netlist graph.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class DesignServiceGrpc {

  private DesignServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.DesignService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetDesignRequest,
      io.github.panyam.agni.v1.webapi.GetDesignResponse> getGetDesignMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetDesign",
      requestType = io.github.panyam.agni.v1.webapi.GetDesignRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.GetDesignResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetDesignRequest,
      io.github.panyam.agni.v1.webapi.GetDesignResponse> getGetDesignMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetDesignRequest, io.github.panyam.agni.v1.webapi.GetDesignResponse> getGetDesignMethod;
    if ((getGetDesignMethod = DesignServiceGrpc.getGetDesignMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getGetDesignMethod = DesignServiceGrpc.getGetDesignMethod) == null) {
          DesignServiceGrpc.getGetDesignMethod = getGetDesignMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetDesignRequest, io.github.panyam.agni.v1.webapi.GetDesignResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetDesign"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetDesignRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetDesignResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("GetDesign"))
              .build();
        }
      }
    }
    return getGetDesignMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetSheetRequest,
      io.github.panyam.agni.v1.webapi.GetSheetResponse> getGetSheetMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetSheet",
      requestType = io.github.panyam.agni.v1.webapi.GetSheetRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.GetSheetResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetSheetRequest,
      io.github.panyam.agni.v1.webapi.GetSheetResponse> getGetSheetMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetSheetRequest, io.github.panyam.agni.v1.webapi.GetSheetResponse> getGetSheetMethod;
    if ((getGetSheetMethod = DesignServiceGrpc.getGetSheetMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getGetSheetMethod = DesignServiceGrpc.getGetSheetMethod) == null) {
          DesignServiceGrpc.getGetSheetMethod = getGetSheetMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetSheetRequest, io.github.panyam.agni.v1.webapi.GetSheetResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetSheet"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetSheetRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetSheetResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("GetSheet"))
              .build();
        }
      }
    }
    return getGetSheetMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.HighlightSheetRequest,
      io.github.panyam.agni.v1.webapi.HighlightSheetResponse> getHighlightSheetMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "HighlightSheet",
      requestType = io.github.panyam.agni.v1.webapi.HighlightSheetRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.HighlightSheetResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.HighlightSheetRequest,
      io.github.panyam.agni.v1.webapi.HighlightSheetResponse> getHighlightSheetMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.HighlightSheetRequest, io.github.panyam.agni.v1.webapi.HighlightSheetResponse> getHighlightSheetMethod;
    if ((getHighlightSheetMethod = DesignServiceGrpc.getHighlightSheetMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getHighlightSheetMethod = DesignServiceGrpc.getHighlightSheetMethod) == null) {
          DesignServiceGrpc.getHighlightSheetMethod = getHighlightSheetMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.HighlightSheetRequest, io.github.panyam.agni.v1.webapi.HighlightSheetResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "HighlightSheet"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.HighlightSheetRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.HighlightSheetResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("HighlightSheet"))
              .build();
        }
      }
    }
    return getHighlightSheetMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetLayoutReportRequest,
      io.github.panyam.agni.v1.webapi.GetLayoutReportResponse> getGetLayoutReportMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetLayoutReport",
      requestType = io.github.panyam.agni.v1.webapi.GetLayoutReportRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.GetLayoutReportResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetLayoutReportRequest,
      io.github.panyam.agni.v1.webapi.GetLayoutReportResponse> getGetLayoutReportMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetLayoutReportRequest, io.github.panyam.agni.v1.webapi.GetLayoutReportResponse> getGetLayoutReportMethod;
    if ((getGetLayoutReportMethod = DesignServiceGrpc.getGetLayoutReportMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getGetLayoutReportMethod = DesignServiceGrpc.getGetLayoutReportMethod) == null) {
          DesignServiceGrpc.getGetLayoutReportMethod = getGetLayoutReportMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetLayoutReportRequest, io.github.panyam.agni.v1.webapi.GetLayoutReportResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetLayoutReport"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetLayoutReportRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetLayoutReportResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("GetLayoutReport"))
              .build();
        }
      }
    }
    return getGetLayoutReportMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.LocateSubjectsRequest,
      io.github.panyam.agni.v1.webapi.LocateSubjectsResponse> getLocateSubjectsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "LocateSubjects",
      requestType = io.github.panyam.agni.v1.webapi.LocateSubjectsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.LocateSubjectsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.LocateSubjectsRequest,
      io.github.panyam.agni.v1.webapi.LocateSubjectsResponse> getLocateSubjectsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.LocateSubjectsRequest, io.github.panyam.agni.v1.webapi.LocateSubjectsResponse> getLocateSubjectsMethod;
    if ((getLocateSubjectsMethod = DesignServiceGrpc.getLocateSubjectsMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getLocateSubjectsMethod = DesignServiceGrpc.getLocateSubjectsMethod) == null) {
          DesignServiceGrpc.getLocateSubjectsMethod = getLocateSubjectsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.LocateSubjectsRequest, io.github.panyam.agni.v1.webapi.LocateSubjectsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "LocateSubjects"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.LocateSubjectsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.LocateSubjectsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("LocateSubjects"))
              .build();
        }
      }
    }
    return getLocateSubjectsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.TraceDesignRequest,
      io.github.panyam.agni.v1.webapi.TraceDesignResponse> getTraceDesignMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "TraceDesign",
      requestType = io.github.panyam.agni.v1.webapi.TraceDesignRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.TraceDesignResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.TraceDesignRequest,
      io.github.panyam.agni.v1.webapi.TraceDesignResponse> getTraceDesignMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.TraceDesignRequest, io.github.panyam.agni.v1.webapi.TraceDesignResponse> getTraceDesignMethod;
    if ((getTraceDesignMethod = DesignServiceGrpc.getTraceDesignMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getTraceDesignMethod = DesignServiceGrpc.getTraceDesignMethod) == null) {
          DesignServiceGrpc.getTraceDesignMethod = getTraceDesignMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.TraceDesignRequest, io.github.panyam.agni.v1.webapi.TraceDesignResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "TraceDesign"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.TraceDesignRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.TraceDesignResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("TraceDesign"))
              .build();
        }
      }
    }
    return getTraceDesignMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ReachablePinsRequest,
      io.github.panyam.agni.v1.webapi.ReachablePinsResponse> getReachablePinsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ReachablePins",
      requestType = io.github.panyam.agni.v1.webapi.ReachablePinsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ReachablePinsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ReachablePinsRequest,
      io.github.panyam.agni.v1.webapi.ReachablePinsResponse> getReachablePinsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ReachablePinsRequest, io.github.panyam.agni.v1.webapi.ReachablePinsResponse> getReachablePinsMethod;
    if ((getReachablePinsMethod = DesignServiceGrpc.getReachablePinsMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getReachablePinsMethod = DesignServiceGrpc.getReachablePinsMethod) == null) {
          DesignServiceGrpc.getReachablePinsMethod = getReachablePinsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ReachablePinsRequest, io.github.panyam.agni.v1.webapi.ReachablePinsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ReachablePins"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ReachablePinsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ReachablePinsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("ReachablePins"))
              .build();
        }
      }
    }
    return getReachablePinsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.WalkDesignRequest,
      io.github.panyam.agni.v1.webapi.WalkDesignResponse> getWalkDesignMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "WalkDesign",
      requestType = io.github.panyam.agni.v1.webapi.WalkDesignRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.WalkDesignResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.WalkDesignRequest,
      io.github.panyam.agni.v1.webapi.WalkDesignResponse> getWalkDesignMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.WalkDesignRequest, io.github.panyam.agni.v1.webapi.WalkDesignResponse> getWalkDesignMethod;
    if ((getWalkDesignMethod = DesignServiceGrpc.getWalkDesignMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getWalkDesignMethod = DesignServiceGrpc.getWalkDesignMethod) == null) {
          DesignServiceGrpc.getWalkDesignMethod = getWalkDesignMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.WalkDesignRequest, io.github.panyam.agni.v1.webapi.WalkDesignResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "WalkDesign"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.WalkDesignRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.WalkDesignResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("WalkDesign"))
              .build();
        }
      }
    }
    return getWalkDesignMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListWalkKindsRequest,
      io.github.panyam.agni.v1.webapi.ListWalkKindsResponse> getListWalkKindsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListWalkKinds",
      requestType = io.github.panyam.agni.v1.webapi.ListWalkKindsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListWalkKindsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListWalkKindsRequest,
      io.github.panyam.agni.v1.webapi.ListWalkKindsResponse> getListWalkKindsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListWalkKindsRequest, io.github.panyam.agni.v1.webapi.ListWalkKindsResponse> getListWalkKindsMethod;
    if ((getListWalkKindsMethod = DesignServiceGrpc.getListWalkKindsMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getListWalkKindsMethod = DesignServiceGrpc.getListWalkKindsMethod) == null) {
          DesignServiceGrpc.getListWalkKindsMethod = getListWalkKindsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListWalkKindsRequest, io.github.panyam.agni.v1.webapi.ListWalkKindsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListWalkKinds"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListWalkKindsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListWalkKindsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("ListWalkKinds"))
              .build();
        }
      }
    }
    return getListWalkKindsMethod;
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
   * DesignService loads a design file into something the viewer can draw. It reuses the
   * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
   * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
   * other supported format is drawn as an auto-layout of its netlist graph.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * GetDesign loads a file and returns its summary plus the list of drawable sheets. The sheet
     * navigator reads that list. It resolves the render mode per file, faithful for geometry and
     * an auto-layout otherwise.
     * </pre>
     */
    default void getDesign(io.github.panyam.agni.v1.webapi.GetDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetDesignResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetDesignMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetSheet returns one rendered sheet. The `format` selects the renderer. PACKED is the
     * tier-2 geometry for the WebGL viewer (C8). SVG is the render.SheetSVG verification
     * backend. NATIVE is the format's own tool as a golden reference, and is not yet
     * implemented. `sheet` selects by id, name, or 0-based index, as the CLI does.
     * </pre>
     */
    default void getSheet(io.github.panyam.agni.v1.webapi.GetSheetRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetSheetResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetSheetMethod(), responseObserver);
    }

    /**
     * <pre>
     * HighlightSheet resolves highlight specs against one rendered sheet. A spec says "these
     * components, nets, and pins, in this color/alpha". It returns a drawable overlay layer,
     * separate from the base render. PACKED yields primitive-index groups. They join the GetSheet
     * PackedSheet by index, and the WebGL viewer redraws just those primitives. SVG yields a
     * transparent overlay document with the same viewBox as the GetSheet SVG. The client stacks
     * or composites it above the sheet. NATIVE has no overlay and is rejected. The request's
     * sheet/layout/symbols must match the GetSheet call that produced the base render. Then the
     * two layers describe the same geometry.
     * </pre>
     */
    default void highlightSheet(io.github.panyam.agni.v1.webapi.HighlightSheetRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.HighlightSheetResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getHighlightSheetMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetLayoutReport explains how an auto-layout maps each component to a drawn node. It gives the
     * device class, and says whether the node is a glyph, the generic box, the design's own symbol,
     * or an unresolved fallback. It uses the same symbol source as the render, so it describes what
     * the viewer shows. It points at --symbol-path when provided symbols do not resolve.
     * </pre>
     */
    default void getLayoutReport(io.github.panyam.agni.v1.webapi.GetLayoutReportRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetLayoutReportResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetLayoutReportMethod(), responseObserver);
    }

    /**
     * <pre>
     * LocateSubjects says which sheets draw each named part, pin, net or bus (agni issue 1325). A
     * viewer opens one of them for a selection that came from another pane, a finding or a link. It
     * answers from the snapshot's schematic, and from its board only when the request asks, since
     * reading a large board costs seconds (agni issue 943).
     * </pre>
     */
    default void locateSubjects(io.github.panyam.agni.v1.webapi.LocateSubjectsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.LocateSubjectsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getLocateSubjectsMethod(), responseObserver);
    }

    /**
     * <pre>
     * TraceDesign walks from one pin to another through the series parts between them, and returns
     * the route. The route lists each part crossed with its own pin on both sides, each net passed
     * through, and the other parts on those nets. `agni trace` prints the same answer. So a route read
     * in a terminal and a route drawn on the canvas cannot disagree.
     * It is verb-shaped (C23). A route does not exist after the call, so it is derived rather than a
     * resource. The wire keeps the three outcomes apart, as every other surface does. Imagine an
     * endpoint that names nothing the design has. That is a failed QUESTION. A client that rendered
     * it as "not connected" would send its reader to look at the board, when the mistake is in what
     * they typed.
     * </pre>
     */
    default void traceDesign(io.github.panyam.agni.v1.webapi.TraceDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.TraceDesignResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getTraceDesignMethod(), responseObserver);
    }

    /**
     * <pre>
     * ReachablePins lists the pins a trace from one pin routes to within a radius, nearest first, so
     * the Trace panel offers only targets a trace can reach (agni issue 1541). It walks as TraceDesign
     * does. Every listed pin routes at that radius, and every pin that routes and sits on no rail is
     * listed. A reached rail or ground is named with its pin count instead of pin by pin. `agni trace
     * --from U1.3 --reachable` prints the same answer. A start that names nothing the design has is an
     * answer with `reason` set, as on TraceDesign.
     * </pre>
     */
    default void reachablePins(io.github.panyam.agni.v1.webapi.ReachablePinsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ReachablePinsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getReachablePinsMethod(), responseObserver);
    }

    /**
     * <pre>
     * WalkDesign walks outwards from one part or pin, hop by hop, and answers the nets it reached, the
     * part it crossed into each, and the parts it left at the boundary with the reason (agni issue
     * 1339). Hop 1 is the start's own nets. A ground or a global net is reached and never walked
     * through, and so is a rail except on the series walk, which passes through one as trace does.
     * The answer says which branches ended there. The walk kind decides which parts are crossed. `agni walk` prints the same answer.
     * Like TraceDesign it is verb-shaped (C23), and a start that names nothing the design has is an
     * answer with `reason` set rather than an empty walk.
     * </pre>
     */
    default void walkDesign(io.github.panyam.agni.v1.webapi.WalkDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.WalkDesignResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getWalkDesignMethod(), responseObserver);
    }

    /**
     * <pre>
     * ListWalkKinds lists the walk kinds WalkDesign takes, in the order a viewer offers them. It reads
     * no design, so it names no snapshot.
     * </pre>
     */
    default void listWalkKinds(io.github.panyam.agni.v1.webapi.ListWalkKindsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListWalkKindsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListWalkKindsMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service DesignService.
   * <pre>
   * DesignService loads a design file into something the viewer can draw. It reuses the
   * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
   * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
   * other supported format is drawn as an auto-layout of its netlist graph.
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
   * DesignService loads a design file into something the viewer can draw. It reuses the
   * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
   * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
   * other supported format is drawn as an auto-layout of its netlist graph.
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
     * GetDesign loads a file and returns its summary plus the list of drawable sheets. The sheet
     * navigator reads that list. It resolves the render mode per file, faithful for geometry and
     * an auto-layout otherwise.
     * </pre>
     */
    public void getDesign(io.github.panyam.agni.v1.webapi.GetDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetDesignResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetDesignMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetSheet returns one rendered sheet. The `format` selects the renderer. PACKED is the
     * tier-2 geometry for the WebGL viewer (C8). SVG is the render.SheetSVG verification
     * backend. NATIVE is the format's own tool as a golden reference, and is not yet
     * implemented. `sheet` selects by id, name, or 0-based index, as the CLI does.
     * </pre>
     */
    public void getSheet(io.github.panyam.agni.v1.webapi.GetSheetRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetSheetResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetSheetMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * HighlightSheet resolves highlight specs against one rendered sheet. A spec says "these
     * components, nets, and pins, in this color/alpha". It returns a drawable overlay layer,
     * separate from the base render. PACKED yields primitive-index groups. They join the GetSheet
     * PackedSheet by index, and the WebGL viewer redraws just those primitives. SVG yields a
     * transparent overlay document with the same viewBox as the GetSheet SVG. The client stacks
     * or composites it above the sheet. NATIVE has no overlay and is rejected. The request's
     * sheet/layout/symbols must match the GetSheet call that produced the base render. Then the
     * two layers describe the same geometry.
     * </pre>
     */
    public void highlightSheet(io.github.panyam.agni.v1.webapi.HighlightSheetRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.HighlightSheetResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getHighlightSheetMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetLayoutReport explains how an auto-layout maps each component to a drawn node. It gives the
     * device class, and says whether the node is a glyph, the generic box, the design's own symbol,
     * or an unresolved fallback. It uses the same symbol source as the render, so it describes what
     * the viewer shows. It points at --symbol-path when provided symbols do not resolve.
     * </pre>
     */
    public void getLayoutReport(io.github.panyam.agni.v1.webapi.GetLayoutReportRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetLayoutReportResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetLayoutReportMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * LocateSubjects says which sheets draw each named part, pin, net or bus (agni issue 1325). A
     * viewer opens one of them for a selection that came from another pane, a finding or a link. It
     * answers from the snapshot's schematic, and from its board only when the request asks, since
     * reading a large board costs seconds (agni issue 943).
     * </pre>
     */
    public void locateSubjects(io.github.panyam.agni.v1.webapi.LocateSubjectsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.LocateSubjectsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getLocateSubjectsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * TraceDesign walks from one pin to another through the series parts between them, and returns
     * the route. The route lists each part crossed with its own pin on both sides, each net passed
     * through, and the other parts on those nets. `agni trace` prints the same answer. So a route read
     * in a terminal and a route drawn on the canvas cannot disagree.
     * It is verb-shaped (C23). A route does not exist after the call, so it is derived rather than a
     * resource. The wire keeps the three outcomes apart, as every other surface does. Imagine an
     * endpoint that names nothing the design has. That is a failed QUESTION. A client that rendered
     * it as "not connected" would send its reader to look at the board, when the mistake is in what
     * they typed.
     * </pre>
     */
    public void traceDesign(io.github.panyam.agni.v1.webapi.TraceDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.TraceDesignResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getTraceDesignMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ReachablePins lists the pins a trace from one pin routes to within a radius, nearest first, so
     * the Trace panel offers only targets a trace can reach (agni issue 1541). It walks as TraceDesign
     * does. Every listed pin routes at that radius, and every pin that routes and sits on no rail is
     * listed. A reached rail or ground is named with its pin count instead of pin by pin. `agni trace
     * --from U1.3 --reachable` prints the same answer. A start that names nothing the design has is an
     * answer with `reason` set, as on TraceDesign.
     * </pre>
     */
    public void reachablePins(io.github.panyam.agni.v1.webapi.ReachablePinsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ReachablePinsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getReachablePinsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * WalkDesign walks outwards from one part or pin, hop by hop, and answers the nets it reached, the
     * part it crossed into each, and the parts it left at the boundary with the reason (agni issue
     * 1339). Hop 1 is the start's own nets. A ground or a global net is reached and never walked
     * through, and so is a rail except on the series walk, which passes through one as trace does.
     * The answer says which branches ended there. The walk kind decides which parts are crossed. `agni walk` prints the same answer.
     * Like TraceDesign it is verb-shaped (C23), and a start that names nothing the design has is an
     * answer with `reason` set rather than an empty walk.
     * </pre>
     */
    public void walkDesign(io.github.panyam.agni.v1.webapi.WalkDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.WalkDesignResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getWalkDesignMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ListWalkKinds lists the walk kinds WalkDesign takes, in the order a viewer offers them. It reads
     * no design, so it names no snapshot.
     * </pre>
     */
    public void listWalkKinds(io.github.panyam.agni.v1.webapi.ListWalkKindsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListWalkKindsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListWalkKindsMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service DesignService.
   * <pre>
   * DesignService loads a design file into something the viewer can draw. It reuses the
   * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
   * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
   * other supported format is drawn as an auto-layout of its netlist graph.
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
     * GetDesign loads a file and returns its summary plus the list of drawable sheets. The sheet
     * navigator reads that list. It resolves the render mode per file, faithful for geometry and
     * an auto-layout otherwise.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetDesignResponse getDesign(io.github.panyam.agni.v1.webapi.GetDesignRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetSheet returns one rendered sheet. The `format` selects the renderer. PACKED is the
     * tier-2 geometry for the WebGL viewer (C8). SVG is the render.SheetSVG verification
     * backend. NATIVE is the format's own tool as a golden reference, and is not yet
     * implemented. `sheet` selects by id, name, or 0-based index, as the CLI does.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetSheetResponse getSheet(io.github.panyam.agni.v1.webapi.GetSheetRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetSheetMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * HighlightSheet resolves highlight specs against one rendered sheet. A spec says "these
     * components, nets, and pins, in this color/alpha". It returns a drawable overlay layer,
     * separate from the base render. PACKED yields primitive-index groups. They join the GetSheet
     * PackedSheet by index, and the WebGL viewer redraws just those primitives. SVG yields a
     * transparent overlay document with the same viewBox as the GetSheet SVG. The client stacks
     * or composites it above the sheet. NATIVE has no overlay and is rejected. The request's
     * sheet/layout/symbols must match the GetSheet call that produced the base render. Then the
     * two layers describe the same geometry.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.HighlightSheetResponse highlightSheet(io.github.panyam.agni.v1.webapi.HighlightSheetRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getHighlightSheetMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetLayoutReport explains how an auto-layout maps each component to a drawn node. It gives the
     * device class, and says whether the node is a glyph, the generic box, the design's own symbol,
     * or an unresolved fallback. It uses the same symbol source as the render, so it describes what
     * the viewer shows. It points at --symbol-path when provided symbols do not resolve.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetLayoutReportResponse getLayoutReport(io.github.panyam.agni.v1.webapi.GetLayoutReportRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetLayoutReportMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * LocateSubjects says which sheets draw each named part, pin, net or bus (agni issue 1325). A
     * viewer opens one of them for a selection that came from another pane, a finding or a link. It
     * answers from the snapshot's schematic, and from its board only when the request asks, since
     * reading a large board costs seconds (agni issue 943).
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.LocateSubjectsResponse locateSubjects(io.github.panyam.agni.v1.webapi.LocateSubjectsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getLocateSubjectsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * TraceDesign walks from one pin to another through the series parts between them, and returns
     * the route. The route lists each part crossed with its own pin on both sides, each net passed
     * through, and the other parts on those nets. `agni trace` prints the same answer. So a route read
     * in a terminal and a route drawn on the canvas cannot disagree.
     * It is verb-shaped (C23). A route does not exist after the call, so it is derived rather than a
     * resource. The wire keeps the three outcomes apart, as every other surface does. Imagine an
     * endpoint that names nothing the design has. That is a failed QUESTION. A client that rendered
     * it as "not connected" would send its reader to look at the board, when the mistake is in what
     * they typed.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.TraceDesignResponse traceDesign(io.github.panyam.agni.v1.webapi.TraceDesignRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getTraceDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ReachablePins lists the pins a trace from one pin routes to within a radius, nearest first, so
     * the Trace panel offers only targets a trace can reach (agni issue 1541). It walks as TraceDesign
     * does. Every listed pin routes at that radius, and every pin that routes and sits on no rail is
     * listed. A reached rail or ground is named with its pin count instead of pin by pin. `agni trace
     * --from U1.3 --reachable` prints the same answer. A start that names nothing the design has is an
     * answer with `reason` set, as on TraceDesign.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ReachablePinsResponse reachablePins(io.github.panyam.agni.v1.webapi.ReachablePinsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getReachablePinsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * WalkDesign walks outwards from one part or pin, hop by hop, and answers the nets it reached, the
     * part it crossed into each, and the parts it left at the boundary with the reason (agni issue
     * 1339). Hop 1 is the start's own nets. A ground or a global net is reached and never walked
     * through, and so is a rail except on the series walk, which passes through one as trace does.
     * The answer says which branches ended there. The walk kind decides which parts are crossed. `agni walk` prints the same answer.
     * Like TraceDesign it is verb-shaped (C23), and a start that names nothing the design has is an
     * answer with `reason` set rather than an empty walk.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.WalkDesignResponse walkDesign(io.github.panyam.agni.v1.webapi.WalkDesignRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getWalkDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListWalkKinds lists the walk kinds WalkDesign takes, in the order a viewer offers them. It reads
     * no design, so it names no snapshot.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListWalkKindsResponse listWalkKinds(io.github.panyam.agni.v1.webapi.ListWalkKindsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListWalkKindsMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service DesignService.
   * <pre>
   * DesignService loads a design file into something the viewer can draw. It reuses the
   * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
   * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
   * other supported format is drawn as an auto-layout of its netlist graph.
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
     * GetDesign loads a file and returns its summary plus the list of drawable sheets. The sheet
     * navigator reads that list. It resolves the render mode per file, faithful for geometry and
     * an auto-layout otherwise.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetDesignResponse getDesign(io.github.panyam.agni.v1.webapi.GetDesignRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetSheet returns one rendered sheet. The `format` selects the renderer. PACKED is the
     * tier-2 geometry for the WebGL viewer (C8). SVG is the render.SheetSVG verification
     * backend. NATIVE is the format's own tool as a golden reference, and is not yet
     * implemented. `sheet` selects by id, name, or 0-based index, as the CLI does.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetSheetResponse getSheet(io.github.panyam.agni.v1.webapi.GetSheetRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetSheetMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * HighlightSheet resolves highlight specs against one rendered sheet. A spec says "these
     * components, nets, and pins, in this color/alpha". It returns a drawable overlay layer,
     * separate from the base render. PACKED yields primitive-index groups. They join the GetSheet
     * PackedSheet by index, and the WebGL viewer redraws just those primitives. SVG yields a
     * transparent overlay document with the same viewBox as the GetSheet SVG. The client stacks
     * or composites it above the sheet. NATIVE has no overlay and is rejected. The request's
     * sheet/layout/symbols must match the GetSheet call that produced the base render. Then the
     * two layers describe the same geometry.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.HighlightSheetResponse highlightSheet(io.github.panyam.agni.v1.webapi.HighlightSheetRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getHighlightSheetMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetLayoutReport explains how an auto-layout maps each component to a drawn node. It gives the
     * device class, and says whether the node is a glyph, the generic box, the design's own symbol,
     * or an unresolved fallback. It uses the same symbol source as the render, so it describes what
     * the viewer shows. It points at --symbol-path when provided symbols do not resolve.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetLayoutReportResponse getLayoutReport(io.github.panyam.agni.v1.webapi.GetLayoutReportRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetLayoutReportMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * LocateSubjects says which sheets draw each named part, pin, net or bus (agni issue 1325). A
     * viewer opens one of them for a selection that came from another pane, a finding or a link. It
     * answers from the snapshot's schematic, and from its board only when the request asks, since
     * reading a large board costs seconds (agni issue 943).
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.LocateSubjectsResponse locateSubjects(io.github.panyam.agni.v1.webapi.LocateSubjectsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getLocateSubjectsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * TraceDesign walks from one pin to another through the series parts between them, and returns
     * the route. The route lists each part crossed with its own pin on both sides, each net passed
     * through, and the other parts on those nets. `agni trace` prints the same answer. So a route read
     * in a terminal and a route drawn on the canvas cannot disagree.
     * It is verb-shaped (C23). A route does not exist after the call, so it is derived rather than a
     * resource. The wire keeps the three outcomes apart, as every other surface does. Imagine an
     * endpoint that names nothing the design has. That is a failed QUESTION. A client that rendered
     * it as "not connected" would send its reader to look at the board, when the mistake is in what
     * they typed.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.TraceDesignResponse traceDesign(io.github.panyam.agni.v1.webapi.TraceDesignRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getTraceDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ReachablePins lists the pins a trace from one pin routes to within a radius, nearest first, so
     * the Trace panel offers only targets a trace can reach (agni issue 1541). It walks as TraceDesign
     * does. Every listed pin routes at that radius, and every pin that routes and sits on no rail is
     * listed. A reached rail or ground is named with its pin count instead of pin by pin. `agni trace
     * --from U1.3 --reachable` prints the same answer. A start that names nothing the design has is an
     * answer with `reason` set, as on TraceDesign.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ReachablePinsResponse reachablePins(io.github.panyam.agni.v1.webapi.ReachablePinsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getReachablePinsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * WalkDesign walks outwards from one part or pin, hop by hop, and answers the nets it reached, the
     * part it crossed into each, and the parts it left at the boundary with the reason (agni issue
     * 1339). Hop 1 is the start's own nets. A ground or a global net is reached and never walked
     * through, and so is a rail except on the series walk, which passes through one as trace does.
     * The answer says which branches ended there. The walk kind decides which parts are crossed. `agni walk` prints the same answer.
     * Like TraceDesign it is verb-shaped (C23), and a start that names nothing the design has is an
     * answer with `reason` set rather than an empty walk.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.WalkDesignResponse walkDesign(io.github.panyam.agni.v1.webapi.WalkDesignRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getWalkDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListWalkKinds lists the walk kinds WalkDesign takes, in the order a viewer offers them. It reads
     * no design, so it names no snapshot.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListWalkKindsResponse listWalkKinds(io.github.panyam.agni.v1.webapi.ListWalkKindsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListWalkKindsMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service DesignService.
   * <pre>
   * DesignService loads a design file into something the viewer can draw. It reuses the
   * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
   * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
   * other supported format is drawn as an auto-layout of its netlist graph.
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
     * GetDesign loads a file and returns its summary plus the list of drawable sheets. The sheet
     * navigator reads that list. It resolves the render mode per file, faithful for geometry and
     * an auto-layout otherwise.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.GetDesignResponse> getDesign(
        io.github.panyam.agni.v1.webapi.GetDesignRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetDesignMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetSheet returns one rendered sheet. The `format` selects the renderer. PACKED is the
     * tier-2 geometry for the WebGL viewer (C8). SVG is the render.SheetSVG verification
     * backend. NATIVE is the format's own tool as a golden reference, and is not yet
     * implemented. `sheet` selects by id, name, or 0-based index, as the CLI does.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.GetSheetResponse> getSheet(
        io.github.panyam.agni.v1.webapi.GetSheetRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetSheetMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * HighlightSheet resolves highlight specs against one rendered sheet. A spec says "these
     * components, nets, and pins, in this color/alpha". It returns a drawable overlay layer,
     * separate from the base render. PACKED yields primitive-index groups. They join the GetSheet
     * PackedSheet by index, and the WebGL viewer redraws just those primitives. SVG yields a
     * transparent overlay document with the same viewBox as the GetSheet SVG. The client stacks
     * or composites it above the sheet. NATIVE has no overlay and is rejected. The request's
     * sheet/layout/symbols must match the GetSheet call that produced the base render. Then the
     * two layers describe the same geometry.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.HighlightSheetResponse> highlightSheet(
        io.github.panyam.agni.v1.webapi.HighlightSheetRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getHighlightSheetMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetLayoutReport explains how an auto-layout maps each component to a drawn node. It gives the
     * device class, and says whether the node is a glyph, the generic box, the design's own symbol,
     * or an unresolved fallback. It uses the same symbol source as the render, so it describes what
     * the viewer shows. It points at --symbol-path when provided symbols do not resolve.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.GetLayoutReportResponse> getLayoutReport(
        io.github.panyam.agni.v1.webapi.GetLayoutReportRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetLayoutReportMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * LocateSubjects says which sheets draw each named part, pin, net or bus (agni issue 1325). A
     * viewer opens one of them for a selection that came from another pane, a finding or a link. It
     * answers from the snapshot's schematic, and from its board only when the request asks, since
     * reading a large board costs seconds (agni issue 943).
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.LocateSubjectsResponse> locateSubjects(
        io.github.panyam.agni.v1.webapi.LocateSubjectsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getLocateSubjectsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * TraceDesign walks from one pin to another through the series parts between them, and returns
     * the route. The route lists each part crossed with its own pin on both sides, each net passed
     * through, and the other parts on those nets. `agni trace` prints the same answer. So a route read
     * in a terminal and a route drawn on the canvas cannot disagree.
     * It is verb-shaped (C23). A route does not exist after the call, so it is derived rather than a
     * resource. The wire keeps the three outcomes apart, as every other surface does. Imagine an
     * endpoint that names nothing the design has. That is a failed QUESTION. A client that rendered
     * it as "not connected" would send its reader to look at the board, when the mistake is in what
     * they typed.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.TraceDesignResponse> traceDesign(
        io.github.panyam.agni.v1.webapi.TraceDesignRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getTraceDesignMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ReachablePins lists the pins a trace from one pin routes to within a radius, nearest first, so
     * the Trace panel offers only targets a trace can reach (agni issue 1541). It walks as TraceDesign
     * does. Every listed pin routes at that radius, and every pin that routes and sits on no rail is
     * listed. A reached rail or ground is named with its pin count instead of pin by pin. `agni trace
     * --from U1.3 --reachable` prints the same answer. A start that names nothing the design has is an
     * answer with `reason` set, as on TraceDesign.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ReachablePinsResponse> reachablePins(
        io.github.panyam.agni.v1.webapi.ReachablePinsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getReachablePinsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * WalkDesign walks outwards from one part or pin, hop by hop, and answers the nets it reached, the
     * part it crossed into each, and the parts it left at the boundary with the reason (agni issue
     * 1339). Hop 1 is the start's own nets. A ground or a global net is reached and never walked
     * through, and so is a rail except on the series walk, which passes through one as trace does.
     * The answer says which branches ended there. The walk kind decides which parts are crossed. `agni walk` prints the same answer.
     * Like TraceDesign it is verb-shaped (C23), and a start that names nothing the design has is an
     * answer with `reason` set rather than an empty walk.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.WalkDesignResponse> walkDesign(
        io.github.panyam.agni.v1.webapi.WalkDesignRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getWalkDesignMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ListWalkKinds lists the walk kinds WalkDesign takes, in the order a viewer offers them. It reads
     * no design, so it names no snapshot.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListWalkKindsResponse> listWalkKinds(
        io.github.panyam.agni.v1.webapi.ListWalkKindsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListWalkKindsMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_GET_DESIGN = 0;
  private static final int METHODID_GET_SHEET = 1;
  private static final int METHODID_HIGHLIGHT_SHEET = 2;
  private static final int METHODID_GET_LAYOUT_REPORT = 3;
  private static final int METHODID_LOCATE_SUBJECTS = 4;
  private static final int METHODID_TRACE_DESIGN = 5;
  private static final int METHODID_REACHABLE_PINS = 6;
  private static final int METHODID_WALK_DESIGN = 7;
  private static final int METHODID_LIST_WALK_KINDS = 8;

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
        case METHODID_GET_DESIGN:
          serviceImpl.getDesign((io.github.panyam.agni.v1.webapi.GetDesignRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetDesignResponse>) responseObserver);
          break;
        case METHODID_GET_SHEET:
          serviceImpl.getSheet((io.github.panyam.agni.v1.webapi.GetSheetRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetSheetResponse>) responseObserver);
          break;
        case METHODID_HIGHLIGHT_SHEET:
          serviceImpl.highlightSheet((io.github.panyam.agni.v1.webapi.HighlightSheetRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.HighlightSheetResponse>) responseObserver);
          break;
        case METHODID_GET_LAYOUT_REPORT:
          serviceImpl.getLayoutReport((io.github.panyam.agni.v1.webapi.GetLayoutReportRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetLayoutReportResponse>) responseObserver);
          break;
        case METHODID_LOCATE_SUBJECTS:
          serviceImpl.locateSubjects((io.github.panyam.agni.v1.webapi.LocateSubjectsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.LocateSubjectsResponse>) responseObserver);
          break;
        case METHODID_TRACE_DESIGN:
          serviceImpl.traceDesign((io.github.panyam.agni.v1.webapi.TraceDesignRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.TraceDesignResponse>) responseObserver);
          break;
        case METHODID_REACHABLE_PINS:
          serviceImpl.reachablePins((io.github.panyam.agni.v1.webapi.ReachablePinsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ReachablePinsResponse>) responseObserver);
          break;
        case METHODID_WALK_DESIGN:
          serviceImpl.walkDesign((io.github.panyam.agni.v1.webapi.WalkDesignRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.WalkDesignResponse>) responseObserver);
          break;
        case METHODID_LIST_WALK_KINDS:
          serviceImpl.listWalkKinds((io.github.panyam.agni.v1.webapi.ListWalkKindsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListWalkKindsResponse>) responseObserver);
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
          getGetDesignMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetDesignRequest,
              io.github.panyam.agni.v1.webapi.GetDesignResponse>(
                service, METHODID_GET_DESIGN)))
        .addMethod(
          getGetSheetMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetSheetRequest,
              io.github.panyam.agni.v1.webapi.GetSheetResponse>(
                service, METHODID_GET_SHEET)))
        .addMethod(
          getHighlightSheetMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.HighlightSheetRequest,
              io.github.panyam.agni.v1.webapi.HighlightSheetResponse>(
                service, METHODID_HIGHLIGHT_SHEET)))
        .addMethod(
          getGetLayoutReportMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetLayoutReportRequest,
              io.github.panyam.agni.v1.webapi.GetLayoutReportResponse>(
                service, METHODID_GET_LAYOUT_REPORT)))
        .addMethod(
          getLocateSubjectsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.LocateSubjectsRequest,
              io.github.panyam.agni.v1.webapi.LocateSubjectsResponse>(
                service, METHODID_LOCATE_SUBJECTS)))
        .addMethod(
          getTraceDesignMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.TraceDesignRequest,
              io.github.panyam.agni.v1.webapi.TraceDesignResponse>(
                service, METHODID_TRACE_DESIGN)))
        .addMethod(
          getReachablePinsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ReachablePinsRequest,
              io.github.panyam.agni.v1.webapi.ReachablePinsResponse>(
                service, METHODID_REACHABLE_PINS)))
        .addMethod(
          getWalkDesignMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.WalkDesignRequest,
              io.github.panyam.agni.v1.webapi.WalkDesignResponse>(
                service, METHODID_WALK_DESIGN)))
        .addMethod(
          getListWalkKindsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListWalkKindsRequest,
              io.github.panyam.agni.v1.webapi.ListWalkKindsResponse>(
                service, METHODID_LIST_WALK_KINDS)))
        .build();
  }

  private static abstract class DesignServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    DesignServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.DesignProto.getDescriptor();
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
              .addMethod(getGetDesignMethod())
              .addMethod(getGetSheetMethod())
              .addMethod(getHighlightSheetMethod())
              .addMethod(getGetLayoutReportMethod())
              .addMethod(getLocateSubjectsMethod())
              .addMethod(getTraceDesignMethod())
              .addMethod(getReachablePinsMethod())
              .addMethod(getWalkDesignMethod())
              .addMethod(getListWalkKindsMethod())
              .build();
        }
      }
    }
    return result;
  }
}
