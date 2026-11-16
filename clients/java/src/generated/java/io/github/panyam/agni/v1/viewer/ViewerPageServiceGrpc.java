package io.github.panyam.agni.v1.viewer;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * This file defines ViewerPageService, the contract a LIVE viewer page answers (agni issues 1150,
 * 1152).
 * A server or the browser engine answers every other service in agni. The open viewer page answers
 * this one itself, so a host can move the page without reloading it. The host can open a sheet,
 * focus a verdict, or apply a link's location to the page already showing the design. A link
 * (ViewerLocation in the URL) is the cold form of the same question, and ApplyLocation is its hot
 * form. So a link and a call have the same effect.
 * The service is transport-neutral. The page dispatches a method name and a protojson body
 * (web/src/pageservice.ts), and each transport only carries that pair. One transport is postMessage
 * from a parent page (web/src/embed.ts, the `agni-viewer/v1` envelope). The other is a design's
 * relay room, where each method is an agent's tool (web/src/viewerroom.ts, agni issue 1154). No
 * server implements it.
 * Every response includes the page's location after the call. So a caller learns where the page
 * ended up (which sheet a verdict opened) without asking again.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class ViewerPageServiceGrpc {

  private ViewerPageServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.viewer.ViewerPageService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.GetLocationRequest,
      io.github.panyam.agni.v1.viewer.GetLocationResponse> getGetLocationMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetLocation",
      requestType = io.github.panyam.agni.v1.viewer.GetLocationRequest.class,
      responseType = io.github.panyam.agni.v1.viewer.GetLocationResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.GetLocationRequest,
      io.github.panyam.agni.v1.viewer.GetLocationResponse> getGetLocationMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.GetLocationRequest, io.github.panyam.agni.v1.viewer.GetLocationResponse> getGetLocationMethod;
    if ((getGetLocationMethod = ViewerPageServiceGrpc.getGetLocationMethod) == null) {
      synchronized (ViewerPageServiceGrpc.class) {
        if ((getGetLocationMethod = ViewerPageServiceGrpc.getGetLocationMethod) == null) {
          ViewerPageServiceGrpc.getGetLocationMethod = getGetLocationMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.viewer.GetLocationRequest, io.github.panyam.agni.v1.viewer.GetLocationResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetLocation"))
              .setSafe(true)
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.GetLocationRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.GetLocationResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ViewerPageServiceMethodDescriptorSupplier("GetLocation"))
              .build();
        }
      }
    }
    return getGetLocationMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.ApplyLocationRequest,
      io.github.panyam.agni.v1.viewer.ApplyLocationResponse> getApplyLocationMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ApplyLocation",
      requestType = io.github.panyam.agni.v1.viewer.ApplyLocationRequest.class,
      responseType = io.github.panyam.agni.v1.viewer.ApplyLocationResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.ApplyLocationRequest,
      io.github.panyam.agni.v1.viewer.ApplyLocationResponse> getApplyLocationMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.ApplyLocationRequest, io.github.panyam.agni.v1.viewer.ApplyLocationResponse> getApplyLocationMethod;
    if ((getApplyLocationMethod = ViewerPageServiceGrpc.getApplyLocationMethod) == null) {
      synchronized (ViewerPageServiceGrpc.class) {
        if ((getApplyLocationMethod = ViewerPageServiceGrpc.getApplyLocationMethod) == null) {
          ViewerPageServiceGrpc.getApplyLocationMethod = getApplyLocationMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.viewer.ApplyLocationRequest, io.github.panyam.agni.v1.viewer.ApplyLocationResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ApplyLocation"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.ApplyLocationRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.ApplyLocationResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ViewerPageServiceMethodDescriptorSupplier("ApplyLocation"))
              .build();
        }
      }
    }
    return getApplyLocationMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.LocateVerdictRequest,
      io.github.panyam.agni.v1.viewer.LocateVerdictResponse> getLocateVerdictMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "LocateVerdict",
      requestType = io.github.panyam.agni.v1.viewer.LocateVerdictRequest.class,
      responseType = io.github.panyam.agni.v1.viewer.LocateVerdictResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.LocateVerdictRequest,
      io.github.panyam.agni.v1.viewer.LocateVerdictResponse> getLocateVerdictMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.LocateVerdictRequest, io.github.panyam.agni.v1.viewer.LocateVerdictResponse> getLocateVerdictMethod;
    if ((getLocateVerdictMethod = ViewerPageServiceGrpc.getLocateVerdictMethod) == null) {
      synchronized (ViewerPageServiceGrpc.class) {
        if ((getLocateVerdictMethod = ViewerPageServiceGrpc.getLocateVerdictMethod) == null) {
          ViewerPageServiceGrpc.getLocateVerdictMethod = getLocateVerdictMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.viewer.LocateVerdictRequest, io.github.panyam.agni.v1.viewer.LocateVerdictResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "LocateVerdict"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.LocateVerdictRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.LocateVerdictResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ViewerPageServiceMethodDescriptorSupplier("LocateVerdict"))
              .build();
        }
      }
    }
    return getLocateVerdictMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.SelectFindingRequest,
      io.github.panyam.agni.v1.viewer.SelectFindingResponse> getSelectFindingMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "SelectFinding",
      requestType = io.github.panyam.agni.v1.viewer.SelectFindingRequest.class,
      responseType = io.github.panyam.agni.v1.viewer.SelectFindingResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.SelectFindingRequest,
      io.github.panyam.agni.v1.viewer.SelectFindingResponse> getSelectFindingMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.SelectFindingRequest, io.github.panyam.agni.v1.viewer.SelectFindingResponse> getSelectFindingMethod;
    if ((getSelectFindingMethod = ViewerPageServiceGrpc.getSelectFindingMethod) == null) {
      synchronized (ViewerPageServiceGrpc.class) {
        if ((getSelectFindingMethod = ViewerPageServiceGrpc.getSelectFindingMethod) == null) {
          ViewerPageServiceGrpc.getSelectFindingMethod = getSelectFindingMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.viewer.SelectFindingRequest, io.github.panyam.agni.v1.viewer.SelectFindingResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "SelectFinding"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.SelectFindingRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.SelectFindingResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ViewerPageServiceMethodDescriptorSupplier("SelectFinding"))
              .build();
        }
      }
    }
    return getSelectFindingMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.LocateEntityRequest,
      io.github.panyam.agni.v1.viewer.LocateEntityResponse> getLocateEntityMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "LocateEntity",
      requestType = io.github.panyam.agni.v1.viewer.LocateEntityRequest.class,
      responseType = io.github.panyam.agni.v1.viewer.LocateEntityResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.LocateEntityRequest,
      io.github.panyam.agni.v1.viewer.LocateEntityResponse> getLocateEntityMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.LocateEntityRequest, io.github.panyam.agni.v1.viewer.LocateEntityResponse> getLocateEntityMethod;
    if ((getLocateEntityMethod = ViewerPageServiceGrpc.getLocateEntityMethod) == null) {
      synchronized (ViewerPageServiceGrpc.class) {
        if ((getLocateEntityMethod = ViewerPageServiceGrpc.getLocateEntityMethod) == null) {
          ViewerPageServiceGrpc.getLocateEntityMethod = getLocateEntityMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.viewer.LocateEntityRequest, io.github.panyam.agni.v1.viewer.LocateEntityResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "LocateEntity"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.LocateEntityRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.LocateEntityResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ViewerPageServiceMethodDescriptorSupplier("LocateEntity"))
              .build();
        }
      }
    }
    return getLocateEntityMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.ShowSheetRequest,
      io.github.panyam.agni.v1.viewer.ShowSheetResponse> getShowSheetMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ShowSheet",
      requestType = io.github.panyam.agni.v1.viewer.ShowSheetRequest.class,
      responseType = io.github.panyam.agni.v1.viewer.ShowSheetResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.ShowSheetRequest,
      io.github.panyam.agni.v1.viewer.ShowSheetResponse> getShowSheetMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.ShowSheetRequest, io.github.panyam.agni.v1.viewer.ShowSheetResponse> getShowSheetMethod;
    if ((getShowSheetMethod = ViewerPageServiceGrpc.getShowSheetMethod) == null) {
      synchronized (ViewerPageServiceGrpc.class) {
        if ((getShowSheetMethod = ViewerPageServiceGrpc.getShowSheetMethod) == null) {
          ViewerPageServiceGrpc.getShowSheetMethod = getShowSheetMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.viewer.ShowSheetRequest, io.github.panyam.agni.v1.viewer.ShowSheetResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ShowSheet"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.ShowSheetRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.ShowSheetResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ViewerPageServiceMethodDescriptorSupplier("ShowSheet"))
              .build();
        }
      }
    }
    return getShowSheetMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.OpenSheetViewRequest,
      io.github.panyam.agni.v1.viewer.OpenSheetViewResponse> getOpenSheetViewMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "OpenSheetView",
      requestType = io.github.panyam.agni.v1.viewer.OpenSheetViewRequest.class,
      responseType = io.github.panyam.agni.v1.viewer.OpenSheetViewResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.OpenSheetViewRequest,
      io.github.panyam.agni.v1.viewer.OpenSheetViewResponse> getOpenSheetViewMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.OpenSheetViewRequest, io.github.panyam.agni.v1.viewer.OpenSheetViewResponse> getOpenSheetViewMethod;
    if ((getOpenSheetViewMethod = ViewerPageServiceGrpc.getOpenSheetViewMethod) == null) {
      synchronized (ViewerPageServiceGrpc.class) {
        if ((getOpenSheetViewMethod = ViewerPageServiceGrpc.getOpenSheetViewMethod) == null) {
          ViewerPageServiceGrpc.getOpenSheetViewMethod = getOpenSheetViewMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.viewer.OpenSheetViewRequest, io.github.panyam.agni.v1.viewer.OpenSheetViewResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "OpenSheetView"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.OpenSheetViewRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.OpenSheetViewResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ViewerPageServiceMethodDescriptorSupplier("OpenSheetView"))
              .build();
        }
      }
    }
    return getOpenSheetViewMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.CloseSheetViewRequest,
      io.github.panyam.agni.v1.viewer.CloseSheetViewResponse> getCloseSheetViewMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CloseSheetView",
      requestType = io.github.panyam.agni.v1.viewer.CloseSheetViewRequest.class,
      responseType = io.github.panyam.agni.v1.viewer.CloseSheetViewResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.CloseSheetViewRequest,
      io.github.panyam.agni.v1.viewer.CloseSheetViewResponse> getCloseSheetViewMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.viewer.CloseSheetViewRequest, io.github.panyam.agni.v1.viewer.CloseSheetViewResponse> getCloseSheetViewMethod;
    if ((getCloseSheetViewMethod = ViewerPageServiceGrpc.getCloseSheetViewMethod) == null) {
      synchronized (ViewerPageServiceGrpc.class) {
        if ((getCloseSheetViewMethod = ViewerPageServiceGrpc.getCloseSheetViewMethod) == null) {
          ViewerPageServiceGrpc.getCloseSheetViewMethod = getCloseSheetViewMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.viewer.CloseSheetViewRequest, io.github.panyam.agni.v1.viewer.CloseSheetViewResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CloseSheetView"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.CloseSheetViewRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.viewer.CloseSheetViewResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ViewerPageServiceMethodDescriptorSupplier("CloseSheetView"))
              .build();
        }
      }
    }
    return getCloseSheetViewMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static ViewerPageServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ViewerPageServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ViewerPageServiceStub>() {
        @java.lang.Override
        public ViewerPageServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ViewerPageServiceStub(channel, callOptions);
        }
      };
    return ViewerPageServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static ViewerPageServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ViewerPageServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ViewerPageServiceBlockingV2Stub>() {
        @java.lang.Override
        public ViewerPageServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ViewerPageServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return ViewerPageServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static ViewerPageServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ViewerPageServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ViewerPageServiceBlockingStub>() {
        @java.lang.Override
        public ViewerPageServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ViewerPageServiceBlockingStub(channel, callOptions);
        }
      };
    return ViewerPageServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static ViewerPageServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ViewerPageServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ViewerPageServiceFutureStub>() {
        @java.lang.Override
        public ViewerPageServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ViewerPageServiceFutureStub(channel, callOptions);
        }
      };
    return ViewerPageServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * This file defines ViewerPageService, the contract a LIVE viewer page answers (agni issues 1150,
   * 1152).
   * A server or the browser engine answers every other service in agni. The open viewer page answers
   * this one itself, so a host can move the page without reloading it. The host can open a sheet,
   * focus a verdict, or apply a link's location to the page already showing the design. A link
   * (ViewerLocation in the URL) is the cold form of the same question, and ApplyLocation is its hot
   * form. So a link and a call have the same effect.
   * The service is transport-neutral. The page dispatches a method name and a protojson body
   * (web/src/pageservice.ts), and each transport only carries that pair. One transport is postMessage
   * from a parent page (web/src/embed.ts, the `agni-viewer/v1` envelope). The other is a design's
   * relay room, where each method is an agent's tool (web/src/viewerroom.ts, agni issue 1154). No
   * server implements it.
   * Every response includes the page's location after the call. So a caller learns where the page
   * ended up (which sheet a verdict opened) without asking again.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * GetLocation returns where the page is now. It changes nothing. NO_SIDE_EFFECTS records that, so
     * an agent holding only read access may call it (agni issue 1154).
     * </pre>
     */
    default void getLocation(io.github.panyam.agni.v1.viewer.GetLocationRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.GetLocationResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetLocationMethod(), responseObserver);
    }

    /**
     * <pre>
     * ApplyLocation moves the open page to a location, as following a link to it would. A location
     * naming a different design, or a different flatten, opens that design. That is the cold path. A
     * location naming the open design, or no design, applies to the page as it stands.
     * </pre>
     */
    default void applyLocation(io.github.panyam.agni.v1.viewer.ApplyLocationRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.ApplyLocationResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getApplyLocationMethod(), responseObserver);
    }

    /**
     * <pre>
     * LocateVerdict focuses one verdict and draws its proof on a sheet it is drawn on.
     * </pre>
     */
    default void locateVerdict(io.github.panyam.agni.v1.viewer.LocateVerdictRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.LocateVerdictResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getLocateVerdictMethod(), responseObserver);
    }

    /**
     * <pre>
     * SelectFinding focuses one finding of the last run, as clicking its row does.
     * </pre>
     */
    default void selectFinding(io.github.panyam.agni.v1.viewer.SelectFindingRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.SelectFindingResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getSelectFindingMethod(), responseObserver);
    }

    /**
     * <pre>
     * LocateEntity highlights a net, part or pin, as clicking a query result cell does.
     * </pre>
     */
    default void locateEntity(io.github.panyam.agni.v1.viewer.LocateEntityRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.LocateEntityResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getLocateEntityMethod(), responseObserver);
    }

    /**
     * <pre>
     * ShowSheet opens one sheet of the open design, on the canvas or in an open sheet view.
     * </pre>
     */
    default void showSheet(io.github.panyam.agni.v1.viewer.ShowSheetRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.ShowSheetResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getShowSheetMethod(), responseObserver);
    }

    /**
     * <pre>
     * OpenSheetView opens a sheet view beside the canvas on one sheet of the open design, as the
     * Sheets panel's "open beside" does (agni issues 1326, 1370).
     * </pre>
     */
    default void openSheetView(io.github.panyam.agni.v1.viewer.OpenSheetViewRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.OpenSheetViewResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getOpenSheetViewMethod(), responseObserver);
    }

    /**
     * <pre>
     * CloseSheetView closes one sheet view.
     * </pre>
     */
    default void closeSheetView(io.github.panyam.agni.v1.viewer.CloseSheetViewRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.CloseSheetViewResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCloseSheetViewMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service ViewerPageService.
   * <pre>
   * This file defines ViewerPageService, the contract a LIVE viewer page answers (agni issues 1150,
   * 1152).
   * A server or the browser engine answers every other service in agni. The open viewer page answers
   * this one itself, so a host can move the page without reloading it. The host can open a sheet,
   * focus a verdict, or apply a link's location to the page already showing the design. A link
   * (ViewerLocation in the URL) is the cold form of the same question, and ApplyLocation is its hot
   * form. So a link and a call have the same effect.
   * The service is transport-neutral. The page dispatches a method name and a protojson body
   * (web/src/pageservice.ts), and each transport only carries that pair. One transport is postMessage
   * from a parent page (web/src/embed.ts, the `agni-viewer/v1` envelope). The other is a design's
   * relay room, where each method is an agent's tool (web/src/viewerroom.ts, agni issue 1154). No
   * server implements it.
   * Every response includes the page's location after the call. So a caller learns where the page
   * ended up (which sheet a verdict opened) without asking again.
   * </pre>
   */
  public static abstract class ViewerPageServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return ViewerPageServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service ViewerPageService.
   * <pre>
   * This file defines ViewerPageService, the contract a LIVE viewer page answers (agni issues 1150,
   * 1152).
   * A server or the browser engine answers every other service in agni. The open viewer page answers
   * this one itself, so a host can move the page without reloading it. The host can open a sheet,
   * focus a verdict, or apply a link's location to the page already showing the design. A link
   * (ViewerLocation in the URL) is the cold form of the same question, and ApplyLocation is its hot
   * form. So a link and a call have the same effect.
   * The service is transport-neutral. The page dispatches a method name and a protojson body
   * (web/src/pageservice.ts), and each transport only carries that pair. One transport is postMessage
   * from a parent page (web/src/embed.ts, the `agni-viewer/v1` envelope). The other is a design's
   * relay room, where each method is an agent's tool (web/src/viewerroom.ts, agni issue 1154). No
   * server implements it.
   * Every response includes the page's location after the call. So a caller learns where the page
   * ended up (which sheet a verdict opened) without asking again.
   * </pre>
   */
  public static final class ViewerPageServiceStub
      extends io.grpc.stub.AbstractAsyncStub<ViewerPageServiceStub> {
    private ViewerPageServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ViewerPageServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ViewerPageServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetLocation returns where the page is now. It changes nothing. NO_SIDE_EFFECTS records that, so
     * an agent holding only read access may call it (agni issue 1154).
     * </pre>
     */
    public void getLocation(io.github.panyam.agni.v1.viewer.GetLocationRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.GetLocationResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetLocationMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ApplyLocation moves the open page to a location, as following a link to it would. A location
     * naming a different design, or a different flatten, opens that design. That is the cold path. A
     * location naming the open design, or no design, applies to the page as it stands.
     * </pre>
     */
    public void applyLocation(io.github.panyam.agni.v1.viewer.ApplyLocationRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.ApplyLocationResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getApplyLocationMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * LocateVerdict focuses one verdict and draws its proof on a sheet it is drawn on.
     * </pre>
     */
    public void locateVerdict(io.github.panyam.agni.v1.viewer.LocateVerdictRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.LocateVerdictResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getLocateVerdictMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * SelectFinding focuses one finding of the last run, as clicking its row does.
     * </pre>
     */
    public void selectFinding(io.github.panyam.agni.v1.viewer.SelectFindingRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.SelectFindingResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getSelectFindingMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * LocateEntity highlights a net, part or pin, as clicking a query result cell does.
     * </pre>
     */
    public void locateEntity(io.github.panyam.agni.v1.viewer.LocateEntityRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.LocateEntityResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getLocateEntityMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ShowSheet opens one sheet of the open design, on the canvas or in an open sheet view.
     * </pre>
     */
    public void showSheet(io.github.panyam.agni.v1.viewer.ShowSheetRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.ShowSheetResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getShowSheetMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * OpenSheetView opens a sheet view beside the canvas on one sheet of the open design, as the
     * Sheets panel's "open beside" does (agni issues 1326, 1370).
     * </pre>
     */
    public void openSheetView(io.github.panyam.agni.v1.viewer.OpenSheetViewRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.OpenSheetViewResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getOpenSheetViewMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * CloseSheetView closes one sheet view.
     * </pre>
     */
    public void closeSheetView(io.github.panyam.agni.v1.viewer.CloseSheetViewRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.CloseSheetViewResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCloseSheetViewMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service ViewerPageService.
   * <pre>
   * This file defines ViewerPageService, the contract a LIVE viewer page answers (agni issues 1150,
   * 1152).
   * A server or the browser engine answers every other service in agni. The open viewer page answers
   * this one itself, so a host can move the page without reloading it. The host can open a sheet,
   * focus a verdict, or apply a link's location to the page already showing the design. A link
   * (ViewerLocation in the URL) is the cold form of the same question, and ApplyLocation is its hot
   * form. So a link and a call have the same effect.
   * The service is transport-neutral. The page dispatches a method name and a protojson body
   * (web/src/pageservice.ts), and each transport only carries that pair. One transport is postMessage
   * from a parent page (web/src/embed.ts, the `agni-viewer/v1` envelope). The other is a design's
   * relay room, where each method is an agent's tool (web/src/viewerroom.ts, agni issue 1154). No
   * server implements it.
   * Every response includes the page's location after the call. So a caller learns where the page
   * ended up (which sheet a verdict opened) without asking again.
   * </pre>
   */
  public static final class ViewerPageServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<ViewerPageServiceBlockingV2Stub> {
    private ViewerPageServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ViewerPageServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ViewerPageServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * GetLocation returns where the page is now. It changes nothing. NO_SIDE_EFFECTS records that, so
     * an agent holding only read access may call it (agni issue 1154).
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.GetLocationResponse getLocation(io.github.panyam.agni.v1.viewer.GetLocationRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetLocationMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ApplyLocation moves the open page to a location, as following a link to it would. A location
     * naming a different design, or a different flatten, opens that design. That is the cold path. A
     * location naming the open design, or no design, applies to the page as it stands.
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.ApplyLocationResponse applyLocation(io.github.panyam.agni.v1.viewer.ApplyLocationRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getApplyLocationMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * LocateVerdict focuses one verdict and draws its proof on a sheet it is drawn on.
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.LocateVerdictResponse locateVerdict(io.github.panyam.agni.v1.viewer.LocateVerdictRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getLocateVerdictMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * SelectFinding focuses one finding of the last run, as clicking its row does.
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.SelectFindingResponse selectFinding(io.github.panyam.agni.v1.viewer.SelectFindingRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getSelectFindingMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * LocateEntity highlights a net, part or pin, as clicking a query result cell does.
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.LocateEntityResponse locateEntity(io.github.panyam.agni.v1.viewer.LocateEntityRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getLocateEntityMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ShowSheet opens one sheet of the open design, on the canvas or in an open sheet view.
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.ShowSheetResponse showSheet(io.github.panyam.agni.v1.viewer.ShowSheetRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getShowSheetMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * OpenSheetView opens a sheet view beside the canvas on one sheet of the open design, as the
     * Sheets panel's "open beside" does (agni issues 1326, 1370).
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.OpenSheetViewResponse openSheetView(io.github.panyam.agni.v1.viewer.OpenSheetViewRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getOpenSheetViewMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * CloseSheetView closes one sheet view.
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.CloseSheetViewResponse closeSheetView(io.github.panyam.agni.v1.viewer.CloseSheetViewRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getCloseSheetViewMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service ViewerPageService.
   * <pre>
   * This file defines ViewerPageService, the contract a LIVE viewer page answers (agni issues 1150,
   * 1152).
   * A server or the browser engine answers every other service in agni. The open viewer page answers
   * this one itself, so a host can move the page without reloading it. The host can open a sheet,
   * focus a verdict, or apply a link's location to the page already showing the design. A link
   * (ViewerLocation in the URL) is the cold form of the same question, and ApplyLocation is its hot
   * form. So a link and a call have the same effect.
   * The service is transport-neutral. The page dispatches a method name and a protojson body
   * (web/src/pageservice.ts), and each transport only carries that pair. One transport is postMessage
   * from a parent page (web/src/embed.ts, the `agni-viewer/v1` envelope). The other is a design's
   * relay room, where each method is an agent's tool (web/src/viewerroom.ts, agni issue 1154). No
   * server implements it.
   * Every response includes the page's location after the call. So a caller learns where the page
   * ended up (which sheet a verdict opened) without asking again.
   * </pre>
   */
  public static final class ViewerPageServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<ViewerPageServiceBlockingStub> {
    private ViewerPageServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ViewerPageServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ViewerPageServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetLocation returns where the page is now. It changes nothing. NO_SIDE_EFFECTS records that, so
     * an agent holding only read access may call it (agni issue 1154).
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.GetLocationResponse getLocation(io.github.panyam.agni.v1.viewer.GetLocationRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetLocationMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ApplyLocation moves the open page to a location, as following a link to it would. A location
     * naming a different design, or a different flatten, opens that design. That is the cold path. A
     * location naming the open design, or no design, applies to the page as it stands.
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.ApplyLocationResponse applyLocation(io.github.panyam.agni.v1.viewer.ApplyLocationRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getApplyLocationMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * LocateVerdict focuses one verdict and draws its proof on a sheet it is drawn on.
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.LocateVerdictResponse locateVerdict(io.github.panyam.agni.v1.viewer.LocateVerdictRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getLocateVerdictMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * SelectFinding focuses one finding of the last run, as clicking its row does.
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.SelectFindingResponse selectFinding(io.github.panyam.agni.v1.viewer.SelectFindingRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getSelectFindingMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * LocateEntity highlights a net, part or pin, as clicking a query result cell does.
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.LocateEntityResponse locateEntity(io.github.panyam.agni.v1.viewer.LocateEntityRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getLocateEntityMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ShowSheet opens one sheet of the open design, on the canvas or in an open sheet view.
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.ShowSheetResponse showSheet(io.github.panyam.agni.v1.viewer.ShowSheetRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getShowSheetMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * OpenSheetView opens a sheet view beside the canvas on one sheet of the open design, as the
     * Sheets panel's "open beside" does (agni issues 1326, 1370).
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.OpenSheetViewResponse openSheetView(io.github.panyam.agni.v1.viewer.OpenSheetViewRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getOpenSheetViewMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * CloseSheetView closes one sheet view.
     * </pre>
     */
    public io.github.panyam.agni.v1.viewer.CloseSheetViewResponse closeSheetView(io.github.panyam.agni.v1.viewer.CloseSheetViewRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCloseSheetViewMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service ViewerPageService.
   * <pre>
   * This file defines ViewerPageService, the contract a LIVE viewer page answers (agni issues 1150,
   * 1152).
   * A server or the browser engine answers every other service in agni. The open viewer page answers
   * this one itself, so a host can move the page without reloading it. The host can open a sheet,
   * focus a verdict, or apply a link's location to the page already showing the design. A link
   * (ViewerLocation in the URL) is the cold form of the same question, and ApplyLocation is its hot
   * form. So a link and a call have the same effect.
   * The service is transport-neutral. The page dispatches a method name and a protojson body
   * (web/src/pageservice.ts), and each transport only carries that pair. One transport is postMessage
   * from a parent page (web/src/embed.ts, the `agni-viewer/v1` envelope). The other is a design's
   * relay room, where each method is an agent's tool (web/src/viewerroom.ts, agni issue 1154). No
   * server implements it.
   * Every response includes the page's location after the call. So a caller learns where the page
   * ended up (which sheet a verdict opened) without asking again.
   * </pre>
   */
  public static final class ViewerPageServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<ViewerPageServiceFutureStub> {
    private ViewerPageServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ViewerPageServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ViewerPageServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetLocation returns where the page is now. It changes nothing. NO_SIDE_EFFECTS records that, so
     * an agent holding only read access may call it (agni issue 1154).
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.viewer.GetLocationResponse> getLocation(
        io.github.panyam.agni.v1.viewer.GetLocationRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetLocationMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ApplyLocation moves the open page to a location, as following a link to it would. A location
     * naming a different design, or a different flatten, opens that design. That is the cold path. A
     * location naming the open design, or no design, applies to the page as it stands.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.viewer.ApplyLocationResponse> applyLocation(
        io.github.panyam.agni.v1.viewer.ApplyLocationRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getApplyLocationMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * LocateVerdict focuses one verdict and draws its proof on a sheet it is drawn on.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.viewer.LocateVerdictResponse> locateVerdict(
        io.github.panyam.agni.v1.viewer.LocateVerdictRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getLocateVerdictMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * SelectFinding focuses one finding of the last run, as clicking its row does.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.viewer.SelectFindingResponse> selectFinding(
        io.github.panyam.agni.v1.viewer.SelectFindingRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getSelectFindingMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * LocateEntity highlights a net, part or pin, as clicking a query result cell does.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.viewer.LocateEntityResponse> locateEntity(
        io.github.panyam.agni.v1.viewer.LocateEntityRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getLocateEntityMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ShowSheet opens one sheet of the open design, on the canvas or in an open sheet view.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.viewer.ShowSheetResponse> showSheet(
        io.github.panyam.agni.v1.viewer.ShowSheetRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getShowSheetMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * OpenSheetView opens a sheet view beside the canvas on one sheet of the open design, as the
     * Sheets panel's "open beside" does (agni issues 1326, 1370).
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.viewer.OpenSheetViewResponse> openSheetView(
        io.github.panyam.agni.v1.viewer.OpenSheetViewRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getOpenSheetViewMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * CloseSheetView closes one sheet view.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.viewer.CloseSheetViewResponse> closeSheetView(
        io.github.panyam.agni.v1.viewer.CloseSheetViewRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCloseSheetViewMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_GET_LOCATION = 0;
  private static final int METHODID_APPLY_LOCATION = 1;
  private static final int METHODID_LOCATE_VERDICT = 2;
  private static final int METHODID_SELECT_FINDING = 3;
  private static final int METHODID_LOCATE_ENTITY = 4;
  private static final int METHODID_SHOW_SHEET = 5;
  private static final int METHODID_OPEN_SHEET_VIEW = 6;
  private static final int METHODID_CLOSE_SHEET_VIEW = 7;

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
        case METHODID_GET_LOCATION:
          serviceImpl.getLocation((io.github.panyam.agni.v1.viewer.GetLocationRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.GetLocationResponse>) responseObserver);
          break;
        case METHODID_APPLY_LOCATION:
          serviceImpl.applyLocation((io.github.panyam.agni.v1.viewer.ApplyLocationRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.ApplyLocationResponse>) responseObserver);
          break;
        case METHODID_LOCATE_VERDICT:
          serviceImpl.locateVerdict((io.github.panyam.agni.v1.viewer.LocateVerdictRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.LocateVerdictResponse>) responseObserver);
          break;
        case METHODID_SELECT_FINDING:
          serviceImpl.selectFinding((io.github.panyam.agni.v1.viewer.SelectFindingRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.SelectFindingResponse>) responseObserver);
          break;
        case METHODID_LOCATE_ENTITY:
          serviceImpl.locateEntity((io.github.panyam.agni.v1.viewer.LocateEntityRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.LocateEntityResponse>) responseObserver);
          break;
        case METHODID_SHOW_SHEET:
          serviceImpl.showSheet((io.github.panyam.agni.v1.viewer.ShowSheetRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.ShowSheetResponse>) responseObserver);
          break;
        case METHODID_OPEN_SHEET_VIEW:
          serviceImpl.openSheetView((io.github.panyam.agni.v1.viewer.OpenSheetViewRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.OpenSheetViewResponse>) responseObserver);
          break;
        case METHODID_CLOSE_SHEET_VIEW:
          serviceImpl.closeSheetView((io.github.panyam.agni.v1.viewer.CloseSheetViewRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.viewer.CloseSheetViewResponse>) responseObserver);
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
          getGetLocationMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.viewer.GetLocationRequest,
              io.github.panyam.agni.v1.viewer.GetLocationResponse>(
                service, METHODID_GET_LOCATION)))
        .addMethod(
          getApplyLocationMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.viewer.ApplyLocationRequest,
              io.github.panyam.agni.v1.viewer.ApplyLocationResponse>(
                service, METHODID_APPLY_LOCATION)))
        .addMethod(
          getLocateVerdictMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.viewer.LocateVerdictRequest,
              io.github.panyam.agni.v1.viewer.LocateVerdictResponse>(
                service, METHODID_LOCATE_VERDICT)))
        .addMethod(
          getSelectFindingMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.viewer.SelectFindingRequest,
              io.github.panyam.agni.v1.viewer.SelectFindingResponse>(
                service, METHODID_SELECT_FINDING)))
        .addMethod(
          getLocateEntityMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.viewer.LocateEntityRequest,
              io.github.panyam.agni.v1.viewer.LocateEntityResponse>(
                service, METHODID_LOCATE_ENTITY)))
        .addMethod(
          getShowSheetMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.viewer.ShowSheetRequest,
              io.github.panyam.agni.v1.viewer.ShowSheetResponse>(
                service, METHODID_SHOW_SHEET)))
        .addMethod(
          getOpenSheetViewMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.viewer.OpenSheetViewRequest,
              io.github.panyam.agni.v1.viewer.OpenSheetViewResponse>(
                service, METHODID_OPEN_SHEET_VIEW)))
        .addMethod(
          getCloseSheetViewMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.viewer.CloseSheetViewRequest,
              io.github.panyam.agni.v1.viewer.CloseSheetViewResponse>(
                service, METHODID_CLOSE_SHEET_VIEW)))
        .build();
  }

  private static abstract class ViewerPageServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    ViewerPageServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.viewer.PageProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("ViewerPageService");
    }
  }

  private static final class ViewerPageServiceFileDescriptorSupplier
      extends ViewerPageServiceBaseDescriptorSupplier {
    ViewerPageServiceFileDescriptorSupplier() {}
  }

  private static final class ViewerPageServiceMethodDescriptorSupplier
      extends ViewerPageServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    ViewerPageServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (ViewerPageServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new ViewerPageServiceFileDescriptorSupplier())
              .addMethod(getGetLocationMethod())
              .addMethod(getApplyLocationMethod())
              .addMethod(getLocateVerdictMethod())
              .addMethod(getSelectFindingMethod())
              .addMethod(getLocateEntityMethod())
              .addMethod(getShowSheetMethod())
              .addMethod(getOpenSheetViewMethod())
              .addMethod(getCloseSheetViewMethod())
              .build();
        }
      }
    }
    return result;
  }
}
