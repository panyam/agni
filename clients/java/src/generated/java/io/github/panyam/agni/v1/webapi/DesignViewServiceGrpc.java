package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * DesignViewService turns a snapshot of a design into something the viewer can draw. It reuses the
 * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
 * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
 * other supported format is drawn as an auto-layout of its netlist graph.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class DesignViewServiceGrpc {

  private DesignViewServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.DesignViewService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetSummaryRequest,
      io.github.panyam.agni.v1.webapi.GetSummaryResponse> getGetSummaryMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetSummary",
      requestType = io.github.panyam.agni.v1.webapi.GetSummaryRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.GetSummaryResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetSummaryRequest,
      io.github.panyam.agni.v1.webapi.GetSummaryResponse> getGetSummaryMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetSummaryRequest, io.github.panyam.agni.v1.webapi.GetSummaryResponse> getGetSummaryMethod;
    if ((getGetSummaryMethod = DesignViewServiceGrpc.getGetSummaryMethod) == null) {
      synchronized (DesignViewServiceGrpc.class) {
        if ((getGetSummaryMethod = DesignViewServiceGrpc.getGetSummaryMethod) == null) {
          DesignViewServiceGrpc.getGetSummaryMethod = getGetSummaryMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetSummaryRequest, io.github.panyam.agni.v1.webapi.GetSummaryResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetSummary"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetSummaryRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetSummaryResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignViewServiceMethodDescriptorSupplier("GetSummary"))
              .build();
        }
      }
    }
    return getGetSummaryMethod;
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
    if ((getGetSheetMethod = DesignViewServiceGrpc.getGetSheetMethod) == null) {
      synchronized (DesignViewServiceGrpc.class) {
        if ((getGetSheetMethod = DesignViewServiceGrpc.getGetSheetMethod) == null) {
          DesignViewServiceGrpc.getGetSheetMethod = getGetSheetMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetSheetRequest, io.github.panyam.agni.v1.webapi.GetSheetResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetSheet"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetSheetRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetSheetResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignViewServiceMethodDescriptorSupplier("GetSheet"))
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
    if ((getHighlightSheetMethod = DesignViewServiceGrpc.getHighlightSheetMethod) == null) {
      synchronized (DesignViewServiceGrpc.class) {
        if ((getHighlightSheetMethod = DesignViewServiceGrpc.getHighlightSheetMethod) == null) {
          DesignViewServiceGrpc.getHighlightSheetMethod = getHighlightSheetMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.HighlightSheetRequest, io.github.panyam.agni.v1.webapi.HighlightSheetResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "HighlightSheet"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.HighlightSheetRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.HighlightSheetResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignViewServiceMethodDescriptorSupplier("HighlightSheet"))
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
    if ((getGetLayoutReportMethod = DesignViewServiceGrpc.getGetLayoutReportMethod) == null) {
      synchronized (DesignViewServiceGrpc.class) {
        if ((getGetLayoutReportMethod = DesignViewServiceGrpc.getGetLayoutReportMethod) == null) {
          DesignViewServiceGrpc.getGetLayoutReportMethod = getGetLayoutReportMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetLayoutReportRequest, io.github.panyam.agni.v1.webapi.GetLayoutReportResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetLayoutReport"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetLayoutReportRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetLayoutReportResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignViewServiceMethodDescriptorSupplier("GetLayoutReport"))
              .build();
        }
      }
    }
    return getGetLayoutReportMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest,
      io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse> getGetBoardGeometryMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetBoardGeometry",
      requestType = io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest,
      io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse> getGetBoardGeometryMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest, io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse> getGetBoardGeometryMethod;
    if ((getGetBoardGeometryMethod = DesignViewServiceGrpc.getGetBoardGeometryMethod) == null) {
      synchronized (DesignViewServiceGrpc.class) {
        if ((getGetBoardGeometryMethod = DesignViewServiceGrpc.getGetBoardGeometryMethod) == null) {
          DesignViewServiceGrpc.getGetBoardGeometryMethod = getGetBoardGeometryMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest, io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetBoardGeometry"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignViewServiceMethodDescriptorSupplier("GetBoardGeometry"))
              .build();
        }
      }
    }
    return getGetBoardGeometryMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetBoardMeshesRequest,
      io.github.panyam.agni.v1.webapi.GetBoardMeshesResponse> getGetBoardMeshesMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetBoardMeshes",
      requestType = io.github.panyam.agni.v1.webapi.GetBoardMeshesRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.GetBoardMeshesResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetBoardMeshesRequest,
      io.github.panyam.agni.v1.webapi.GetBoardMeshesResponse> getGetBoardMeshesMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetBoardMeshesRequest, io.github.panyam.agni.v1.webapi.GetBoardMeshesResponse> getGetBoardMeshesMethod;
    if ((getGetBoardMeshesMethod = DesignViewServiceGrpc.getGetBoardMeshesMethod) == null) {
      synchronized (DesignViewServiceGrpc.class) {
        if ((getGetBoardMeshesMethod = DesignViewServiceGrpc.getGetBoardMeshesMethod) == null) {
          DesignViewServiceGrpc.getGetBoardMeshesMethod = getGetBoardMeshesMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetBoardMeshesRequest, io.github.panyam.agni.v1.webapi.GetBoardMeshesResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetBoardMeshes"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetBoardMeshesRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetBoardMeshesResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignViewServiceMethodDescriptorSupplier("GetBoardMeshes"))
              .build();
        }
      }
    }
    return getGetBoardMeshesMethod;
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
    if ((getLocateSubjectsMethod = DesignViewServiceGrpc.getLocateSubjectsMethod) == null) {
      synchronized (DesignViewServiceGrpc.class) {
        if ((getLocateSubjectsMethod = DesignViewServiceGrpc.getLocateSubjectsMethod) == null) {
          DesignViewServiceGrpc.getLocateSubjectsMethod = getLocateSubjectsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.LocateSubjectsRequest, io.github.panyam.agni.v1.webapi.LocateSubjectsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "LocateSubjects"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.LocateSubjectsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.LocateSubjectsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignViewServiceMethodDescriptorSupplier("LocateSubjects"))
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
    if ((getTraceDesignMethod = DesignViewServiceGrpc.getTraceDesignMethod) == null) {
      synchronized (DesignViewServiceGrpc.class) {
        if ((getTraceDesignMethod = DesignViewServiceGrpc.getTraceDesignMethod) == null) {
          DesignViewServiceGrpc.getTraceDesignMethod = getTraceDesignMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.TraceDesignRequest, io.github.panyam.agni.v1.webapi.TraceDesignResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "TraceDesign"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.TraceDesignRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.TraceDesignResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignViewServiceMethodDescriptorSupplier("TraceDesign"))
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
    if ((getReachablePinsMethod = DesignViewServiceGrpc.getReachablePinsMethod) == null) {
      synchronized (DesignViewServiceGrpc.class) {
        if ((getReachablePinsMethod = DesignViewServiceGrpc.getReachablePinsMethod) == null) {
          DesignViewServiceGrpc.getReachablePinsMethod = getReachablePinsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ReachablePinsRequest, io.github.panyam.agni.v1.webapi.ReachablePinsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ReachablePins"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ReachablePinsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ReachablePinsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignViewServiceMethodDescriptorSupplier("ReachablePins"))
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
    if ((getWalkDesignMethod = DesignViewServiceGrpc.getWalkDesignMethod) == null) {
      synchronized (DesignViewServiceGrpc.class) {
        if ((getWalkDesignMethod = DesignViewServiceGrpc.getWalkDesignMethod) == null) {
          DesignViewServiceGrpc.getWalkDesignMethod = getWalkDesignMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.WalkDesignRequest, io.github.panyam.agni.v1.webapi.WalkDesignResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "WalkDesign"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.WalkDesignRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.WalkDesignResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignViewServiceMethodDescriptorSupplier("WalkDesign"))
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
    if ((getListWalkKindsMethod = DesignViewServiceGrpc.getListWalkKindsMethod) == null) {
      synchronized (DesignViewServiceGrpc.class) {
        if ((getListWalkKindsMethod = DesignViewServiceGrpc.getListWalkKindsMethod) == null) {
          DesignViewServiceGrpc.getListWalkKindsMethod = getListWalkKindsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListWalkKindsRequest, io.github.panyam.agni.v1.webapi.ListWalkKindsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListWalkKinds"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListWalkKindsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListWalkKindsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignViewServiceMethodDescriptorSupplier("ListWalkKinds"))
              .build();
        }
      }
    }
    return getListWalkKindsMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static DesignViewServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DesignViewServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DesignViewServiceStub>() {
        @java.lang.Override
        public DesignViewServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DesignViewServiceStub(channel, callOptions);
        }
      };
    return DesignViewServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static DesignViewServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DesignViewServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DesignViewServiceBlockingV2Stub>() {
        @java.lang.Override
        public DesignViewServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DesignViewServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return DesignViewServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static DesignViewServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DesignViewServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DesignViewServiceBlockingStub>() {
        @java.lang.Override
        public DesignViewServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DesignViewServiceBlockingStub(channel, callOptions);
        }
      };
    return DesignViewServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static DesignViewServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DesignViewServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DesignViewServiceFutureStub>() {
        @java.lang.Override
        public DesignViewServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DesignViewServiceFutureStub(channel, callOptions);
        }
      };
    return DesignViewServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * DesignViewService turns a snapshot of a design into something the viewer can draw. It reuses the
   * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
   * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
   * other supported format is drawn as an auto-layout of its netlist graph.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * GetSummary returns a snapshot's summary plus the list of drawable sheets. The sheet
     * navigator reads that list. It resolves the render mode per file, faithful for geometry and
     * an auto-layout otherwise.
     * </pre>
     */
    default void getSummary(io.github.panyam.agni.v1.webapi.GetSummaryRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetSummaryResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetSummaryMethod(), responseObserver);
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
     * GetBoardGeometry returns the design's board as agni.v1.geom.BoardGeometry, a page at a time
     * (agni issue 1634). A front end in another language draws the board from it, and
     * docsite/content/build/board-geometry.md says what each message means. The first page carries
     * the board's units, layers, stack, outline and padstacks. Later pages carry whole placements,
     * nets, zones, texts and graphics, in that order. A client repeats the call with
     * next_page_token until it comes back empty, and merges the pages with its protobuf library's
     * merge, which appends repeated fields. A design with no board answers not_found.
     * </pre>
     */
    default void getBoardGeometry(io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetBoardGeometryMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetBoardMeshes returns the 3D model meshes for the board's parts, a page at a time (agni issue
     * 1652). Each ComponentPlacement's model_instances names its meshes by key. A stack view asks for
     * them only when it draws models, so a check, a review or the 2D board never reads a mesh.
     * The request's max_triangles sets a budget for the whole board, and the server fits the meshes to
     * it. The largest meshes lose detail first, and then the smallest parts draw as blocks. The first
     * page says what the server simplified and what it left as blocks. A part's height never depends on
     * the budget, since the board's heights come from the full models.
     * </pre>
     */
    default void getBoardMeshes(io.github.panyam.agni.v1.webapi.GetBoardMeshesRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetBoardMeshesResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetBoardMeshesMethod(), responseObserver);
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
   * Base class for the server implementation of the service DesignViewService.
   * <pre>
   * DesignViewService turns a snapshot of a design into something the viewer can draw. It reuses the
   * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
   * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
   * other supported format is drawn as an auto-layout of its netlist graph.
   * </pre>
   */
  public static abstract class DesignViewServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return DesignViewServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service DesignViewService.
   * <pre>
   * DesignViewService turns a snapshot of a design into something the viewer can draw. It reuses the
   * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
   * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
   * other supported format is drawn as an auto-layout of its netlist graph.
   * </pre>
   */
  public static final class DesignViewServiceStub
      extends io.grpc.stub.AbstractAsyncStub<DesignViewServiceStub> {
    private DesignViewServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DesignViewServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DesignViewServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetSummary returns a snapshot's summary plus the list of drawable sheets. The sheet
     * navigator reads that list. It resolves the render mode per file, faithful for geometry and
     * an auto-layout otherwise.
     * </pre>
     */
    public void getSummary(io.github.panyam.agni.v1.webapi.GetSummaryRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetSummaryResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetSummaryMethod(), getCallOptions()), request, responseObserver);
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
     * GetBoardGeometry returns the design's board as agni.v1.geom.BoardGeometry, a page at a time
     * (agni issue 1634). A front end in another language draws the board from it, and
     * docsite/content/build/board-geometry.md says what each message means. The first page carries
     * the board's units, layers, stack, outline and padstacks. Later pages carry whole placements,
     * nets, zones, texts and graphics, in that order. A client repeats the call with
     * next_page_token until it comes back empty, and merges the pages with its protobuf library's
     * merge, which appends repeated fields. A design with no board answers not_found.
     * </pre>
     */
    public void getBoardGeometry(io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetBoardGeometryMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetBoardMeshes returns the 3D model meshes for the board's parts, a page at a time (agni issue
     * 1652). Each ComponentPlacement's model_instances names its meshes by key. A stack view asks for
     * them only when it draws models, so a check, a review or the 2D board never reads a mesh.
     * The request's max_triangles sets a budget for the whole board, and the server fits the meshes to
     * it. The largest meshes lose detail first, and then the smallest parts draw as blocks. The first
     * page says what the server simplified and what it left as blocks. A part's height never depends on
     * the budget, since the board's heights come from the full models.
     * </pre>
     */
    public void getBoardMeshes(io.github.panyam.agni.v1.webapi.GetBoardMeshesRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetBoardMeshesResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetBoardMeshesMethod(), getCallOptions()), request, responseObserver);
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
   * A stub to allow clients to do synchronous rpc calls to service DesignViewService.
   * <pre>
   * DesignViewService turns a snapshot of a design into something the viewer can draw. It reuses the
   * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
   * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
   * other supported format is drawn as an auto-layout of its netlist graph.
   * </pre>
   */
  public static final class DesignViewServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<DesignViewServiceBlockingV2Stub> {
    private DesignViewServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DesignViewServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DesignViewServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * GetSummary returns a snapshot's summary plus the list of drawable sheets. The sheet
     * navigator reads that list. It resolves the render mode per file, faithful for geometry and
     * an auto-layout otherwise.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetSummaryResponse getSummary(io.github.panyam.agni.v1.webapi.GetSummaryRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetSummaryMethod(), getCallOptions(), request);
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
     * GetBoardGeometry returns the design's board as agni.v1.geom.BoardGeometry, a page at a time
     * (agni issue 1634). A front end in another language draws the board from it, and
     * docsite/content/build/board-geometry.md says what each message means. The first page carries
     * the board's units, layers, stack, outline and padstacks. Later pages carry whole placements,
     * nets, zones, texts and graphics, in that order. A client repeats the call with
     * next_page_token until it comes back empty, and merges the pages with its protobuf library's
     * merge, which appends repeated fields. A design with no board answers not_found.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse getBoardGeometry(io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetBoardGeometryMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetBoardMeshes returns the 3D model meshes for the board's parts, a page at a time (agni issue
     * 1652). Each ComponentPlacement's model_instances names its meshes by key. A stack view asks for
     * them only when it draws models, so a check, a review or the 2D board never reads a mesh.
     * The request's max_triangles sets a budget for the whole board, and the server fits the meshes to
     * it. The largest meshes lose detail first, and then the smallest parts draw as blocks. The first
     * page says what the server simplified and what it left as blocks. A part's height never depends on
     * the budget, since the board's heights come from the full models.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetBoardMeshesResponse getBoardMeshes(io.github.panyam.agni.v1.webapi.GetBoardMeshesRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetBoardMeshesMethod(), getCallOptions(), request);
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
   * A stub to allow clients to do limited synchronous rpc calls to service DesignViewService.
   * <pre>
   * DesignViewService turns a snapshot of a design into something the viewer can draw. It reuses the
   * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
   * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
   * other supported format is drawn as an auto-layout of its netlist graph.
   * </pre>
   */
  public static final class DesignViewServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<DesignViewServiceBlockingStub> {
    private DesignViewServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DesignViewServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DesignViewServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetSummary returns a snapshot's summary plus the list of drawable sheets. The sheet
     * navigator reads that list. It resolves the render mode per file, faithful for geometry and
     * an auto-layout otherwise.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetSummaryResponse getSummary(io.github.panyam.agni.v1.webapi.GetSummaryRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetSummaryMethod(), getCallOptions(), request);
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
     * GetBoardGeometry returns the design's board as agni.v1.geom.BoardGeometry, a page at a time
     * (agni issue 1634). A front end in another language draws the board from it, and
     * docsite/content/build/board-geometry.md says what each message means. The first page carries
     * the board's units, layers, stack, outline and padstacks. Later pages carry whole placements,
     * nets, zones, texts and graphics, in that order. A client repeats the call with
     * next_page_token until it comes back empty, and merges the pages with its protobuf library's
     * merge, which appends repeated fields. A design with no board answers not_found.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse getBoardGeometry(io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetBoardGeometryMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetBoardMeshes returns the 3D model meshes for the board's parts, a page at a time (agni issue
     * 1652). Each ComponentPlacement's model_instances names its meshes by key. A stack view asks for
     * them only when it draws models, so a check, a review or the 2D board never reads a mesh.
     * The request's max_triangles sets a budget for the whole board, and the server fits the meshes to
     * it. The largest meshes lose detail first, and then the smallest parts draw as blocks. The first
     * page says what the server simplified and what it left as blocks. A part's height never depends on
     * the budget, since the board's heights come from the full models.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.GetBoardMeshesResponse getBoardMeshes(io.github.panyam.agni.v1.webapi.GetBoardMeshesRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetBoardMeshesMethod(), getCallOptions(), request);
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
   * A stub to allow clients to do ListenableFuture-style rpc calls to service DesignViewService.
   * <pre>
   * DesignViewService turns a snapshot of a design into something the viewer can draw. It reuses the
   * engine's readers, auto-layout, and packer. The web tier only chooses which to use per file,
   * and serves the result. A design with schematic geometry (EDIF .eds) renders faithfully. Any
   * other supported format is drawn as an auto-layout of its netlist graph.
   * </pre>
   */
  public static final class DesignViewServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<DesignViewServiceFutureStub> {
    private DesignViewServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DesignViewServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DesignViewServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetSummary returns a snapshot's summary plus the list of drawable sheets. The sheet
     * navigator reads that list. It resolves the render mode per file, faithful for geometry and
     * an auto-layout otherwise.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.GetSummaryResponse> getSummary(
        io.github.panyam.agni.v1.webapi.GetSummaryRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetSummaryMethod(), getCallOptions()), request);
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
     * GetBoardGeometry returns the design's board as agni.v1.geom.BoardGeometry, a page at a time
     * (agni issue 1634). A front end in another language draws the board from it, and
     * docsite/content/build/board-geometry.md says what each message means. The first page carries
     * the board's units, layers, stack, outline and padstacks. Later pages carry whole placements,
     * nets, zones, texts and graphics, in that order. A client repeats the call with
     * next_page_token until it comes back empty, and merges the pages with its protobuf library's
     * merge, which appends repeated fields. A design with no board answers not_found.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse> getBoardGeometry(
        io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetBoardGeometryMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetBoardMeshes returns the 3D model meshes for the board's parts, a page at a time (agni issue
     * 1652). Each ComponentPlacement's model_instances names its meshes by key. A stack view asks for
     * them only when it draws models, so a check, a review or the 2D board never reads a mesh.
     * The request's max_triangles sets a budget for the whole board, and the server fits the meshes to
     * it. The largest meshes lose detail first, and then the smallest parts draw as blocks. The first
     * page says what the server simplified and what it left as blocks. A part's height never depends on
     * the budget, since the board's heights come from the full models.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.GetBoardMeshesResponse> getBoardMeshes(
        io.github.panyam.agni.v1.webapi.GetBoardMeshesRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetBoardMeshesMethod(), getCallOptions()), request);
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

  private static final int METHODID_GET_SUMMARY = 0;
  private static final int METHODID_GET_SHEET = 1;
  private static final int METHODID_HIGHLIGHT_SHEET = 2;
  private static final int METHODID_GET_LAYOUT_REPORT = 3;
  private static final int METHODID_GET_BOARD_GEOMETRY = 4;
  private static final int METHODID_GET_BOARD_MESHES = 5;
  private static final int METHODID_LOCATE_SUBJECTS = 6;
  private static final int METHODID_TRACE_DESIGN = 7;
  private static final int METHODID_REACHABLE_PINS = 8;
  private static final int METHODID_WALK_DESIGN = 9;
  private static final int METHODID_LIST_WALK_KINDS = 10;

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
        case METHODID_GET_SUMMARY:
          serviceImpl.getSummary((io.github.panyam.agni.v1.webapi.GetSummaryRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetSummaryResponse>) responseObserver);
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
        case METHODID_GET_BOARD_GEOMETRY:
          serviceImpl.getBoardGeometry((io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse>) responseObserver);
          break;
        case METHODID_GET_BOARD_MESHES:
          serviceImpl.getBoardMeshes((io.github.panyam.agni.v1.webapi.GetBoardMeshesRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.GetBoardMeshesResponse>) responseObserver);
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
          getGetSummaryMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetSummaryRequest,
              io.github.panyam.agni.v1.webapi.GetSummaryResponse>(
                service, METHODID_GET_SUMMARY)))
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
          getGetBoardGeometryMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetBoardGeometryRequest,
              io.github.panyam.agni.v1.webapi.GetBoardGeometryResponse>(
                service, METHODID_GET_BOARD_GEOMETRY)))
        .addMethod(
          getGetBoardMeshesMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetBoardMeshesRequest,
              io.github.panyam.agni.v1.webapi.GetBoardMeshesResponse>(
                service, METHODID_GET_BOARD_MESHES)))
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

  private static abstract class DesignViewServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    DesignViewServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.DesignViewProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("DesignViewService");
    }
  }

  private static final class DesignViewServiceFileDescriptorSupplier
      extends DesignViewServiceBaseDescriptorSupplier {
    DesignViewServiceFileDescriptorSupplier() {}
  }

  private static final class DesignViewServiceMethodDescriptorSupplier
      extends DesignViewServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    DesignViewServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (DesignViewServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new DesignViewServiceFileDescriptorSupplier())
              .addMethod(getGetSummaryMethod())
              .addMethod(getGetSheetMethod())
              .addMethod(getHighlightSheetMethod())
              .addMethod(getGetLayoutReportMethod())
              .addMethod(getGetBoardGeometryMethod())
              .addMethod(getGetBoardMeshesMethod())
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
