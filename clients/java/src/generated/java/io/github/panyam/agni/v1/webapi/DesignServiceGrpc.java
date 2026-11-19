package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * This file defines DesignService, which keeps the designs a server holds (agni issues 1694, 1696).
 * A design is named "designs/{design}". It holds artifacts, each one source of the design such as a
 * schematic with its sheets, a board, or a BOM. It ties to no project, and a project lists it by
 * name. DesignViewService (design_view.proto) answers questions about a snapshot of a design instead.
 * A server holds designs in two kinds of place. The store named by `agni serve --store` keeps the
 * designs callers create as records, and `--uploads` keeps their files. Those designs can be changed
 * and deleted. Mounted folders declare designs too, and those are read-only. A design id is unique
 * across both. Nothing on this service names a folder, so each store decides how it keeps a design.
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

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetArtifactRequest,
      io.github.panyam.agni.v1.webapi.Artifact> getGetArtifactMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetArtifact",
      requestType = io.github.panyam.agni.v1.webapi.GetArtifactRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Artifact.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetArtifactRequest,
      io.github.panyam.agni.v1.webapi.Artifact> getGetArtifactMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetArtifactRequest, io.github.panyam.agni.v1.webapi.Artifact> getGetArtifactMethod;
    if ((getGetArtifactMethod = DesignServiceGrpc.getGetArtifactMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getGetArtifactMethod = DesignServiceGrpc.getGetArtifactMethod) == null) {
          DesignServiceGrpc.getGetArtifactMethod = getGetArtifactMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetArtifactRequest, io.github.panyam.agni.v1.webapi.Artifact>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetArtifact"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetArtifactRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Artifact.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("GetArtifact"))
              .build();
        }
      }
    }
    return getGetArtifactMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListArtifactsRequest,
      io.github.panyam.agni.v1.webapi.ListArtifactsResponse> getListArtifactsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListArtifacts",
      requestType = io.github.panyam.agni.v1.webapi.ListArtifactsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListArtifactsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListArtifactsRequest,
      io.github.panyam.agni.v1.webapi.ListArtifactsResponse> getListArtifactsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListArtifactsRequest, io.github.panyam.agni.v1.webapi.ListArtifactsResponse> getListArtifactsMethod;
    if ((getListArtifactsMethod = DesignServiceGrpc.getListArtifactsMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getListArtifactsMethod = DesignServiceGrpc.getListArtifactsMethod) == null) {
          DesignServiceGrpc.getListArtifactsMethod = getListArtifactsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListArtifactsRequest, io.github.panyam.agni.v1.webapi.ListArtifactsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListArtifacts"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListArtifactsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListArtifactsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("ListArtifacts"))
              .build();
        }
      }
    }
    return getListArtifactsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdateArtifactRequest,
      io.github.panyam.agni.v1.webapi.Artifact> getUpdateArtifactMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "UpdateArtifact",
      requestType = io.github.panyam.agni.v1.webapi.UpdateArtifactRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Artifact.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdateArtifactRequest,
      io.github.panyam.agni.v1.webapi.Artifact> getUpdateArtifactMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdateArtifactRequest, io.github.panyam.agni.v1.webapi.Artifact> getUpdateArtifactMethod;
    if ((getUpdateArtifactMethod = DesignServiceGrpc.getUpdateArtifactMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getUpdateArtifactMethod = DesignServiceGrpc.getUpdateArtifactMethod) == null) {
          DesignServiceGrpc.getUpdateArtifactMethod = getUpdateArtifactMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.UpdateArtifactRequest, io.github.panyam.agni.v1.webapi.Artifact>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "UpdateArtifact"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.UpdateArtifactRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Artifact.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("UpdateArtifact"))
              .build();
        }
      }
    }
    return getUpdateArtifactMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsRequest,
      io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsResponse> getBatchUpdateArtifactsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "BatchUpdateArtifacts",
      requestType = io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsRequest,
      io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsResponse> getBatchUpdateArtifactsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsRequest, io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsResponse> getBatchUpdateArtifactsMethod;
    if ((getBatchUpdateArtifactsMethod = DesignServiceGrpc.getBatchUpdateArtifactsMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getBatchUpdateArtifactsMethod = DesignServiceGrpc.getBatchUpdateArtifactsMethod) == null) {
          DesignServiceGrpc.getBatchUpdateArtifactsMethod = getBatchUpdateArtifactsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsRequest, io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "BatchUpdateArtifacts"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("BatchUpdateArtifacts"))
              .build();
        }
      }
    }
    return getBatchUpdateArtifactsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ImportDesignFilesRequest,
      io.github.panyam.agni.v1.webapi.ImportDesignFilesResponse> getImportDesignFilesMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ImportDesignFiles",
      requestType = io.github.panyam.agni.v1.webapi.ImportDesignFilesRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ImportDesignFilesResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ImportDesignFilesRequest,
      io.github.panyam.agni.v1.webapi.ImportDesignFilesResponse> getImportDesignFilesMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ImportDesignFilesRequest, io.github.panyam.agni.v1.webapi.ImportDesignFilesResponse> getImportDesignFilesMethod;
    if ((getImportDesignFilesMethod = DesignServiceGrpc.getImportDesignFilesMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getImportDesignFilesMethod = DesignServiceGrpc.getImportDesignFilesMethod) == null) {
          DesignServiceGrpc.getImportDesignFilesMethod = getImportDesignFilesMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ImportDesignFilesRequest, io.github.panyam.agni.v1.webapi.ImportDesignFilesResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ImportDesignFiles"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ImportDesignFilesRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ImportDesignFilesResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("ImportDesignFiles"))
              .build();
        }
      }
    }
    return getImportDesignFilesMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.AddDesignsRequest,
      io.github.panyam.agni.v1.webapi.AddDesignsResponse> getAddDesignsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "AddDesigns",
      requestType = io.github.panyam.agni.v1.webapi.AddDesignsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.AddDesignsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.AddDesignsRequest,
      io.github.panyam.agni.v1.webapi.AddDesignsResponse> getAddDesignsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.AddDesignsRequest, io.github.panyam.agni.v1.webapi.AddDesignsResponse> getAddDesignsMethod;
    if ((getAddDesignsMethod = DesignServiceGrpc.getAddDesignsMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getAddDesignsMethod = DesignServiceGrpc.getAddDesignsMethod) == null) {
          DesignServiceGrpc.getAddDesignsMethod = getAddDesignsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.AddDesignsRequest, io.github.panyam.agni.v1.webapi.AddDesignsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "AddDesigns"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.AddDesignsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.AddDesignsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("AddDesigns"))
              .build();
        }
      }
    }
    return getAddDesignsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteArtifactRequest,
      com.google.protobuf.Empty> getDeleteArtifactMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "DeleteArtifact",
      requestType = io.github.panyam.agni.v1.webapi.DeleteArtifactRequest.class,
      responseType = com.google.protobuf.Empty.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteArtifactRequest,
      com.google.protobuf.Empty> getDeleteArtifactMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteArtifactRequest, com.google.protobuf.Empty> getDeleteArtifactMethod;
    if ((getDeleteArtifactMethod = DesignServiceGrpc.getDeleteArtifactMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getDeleteArtifactMethod = DesignServiceGrpc.getDeleteArtifactMethod) == null) {
          DesignServiceGrpc.getDeleteArtifactMethod = getDeleteArtifactMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.DeleteArtifactRequest, com.google.protobuf.Empty>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "DeleteArtifact"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.DeleteArtifactRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.google.protobuf.Empty.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("DeleteArtifact"))
              .build();
        }
      }
    }
    return getDeleteArtifactMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateReportRequest,
      io.github.panyam.agni.v1.webapi.Report> getCreateReportMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CreateReport",
      requestType = io.github.panyam.agni.v1.webapi.CreateReportRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Report.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateReportRequest,
      io.github.panyam.agni.v1.webapi.Report> getCreateReportMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateReportRequest, io.github.panyam.agni.v1.webapi.Report> getCreateReportMethod;
    if ((getCreateReportMethod = DesignServiceGrpc.getCreateReportMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getCreateReportMethod = DesignServiceGrpc.getCreateReportMethod) == null) {
          DesignServiceGrpc.getCreateReportMethod = getCreateReportMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.CreateReportRequest, io.github.panyam.agni.v1.webapi.Report>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CreateReport"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.CreateReportRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Report.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("CreateReport"))
              .build();
        }
      }
    }
    return getCreateReportMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetReportRequest,
      io.github.panyam.agni.v1.webapi.Report> getGetReportMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetReport",
      requestType = io.github.panyam.agni.v1.webapi.GetReportRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Report.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetReportRequest,
      io.github.panyam.agni.v1.webapi.Report> getGetReportMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetReportRequest, io.github.panyam.agni.v1.webapi.Report> getGetReportMethod;
    if ((getGetReportMethod = DesignServiceGrpc.getGetReportMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getGetReportMethod = DesignServiceGrpc.getGetReportMethod) == null) {
          DesignServiceGrpc.getGetReportMethod = getGetReportMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetReportRequest, io.github.panyam.agni.v1.webapi.Report>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetReport"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetReportRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Report.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("GetReport"))
              .build();
        }
      }
    }
    return getGetReportMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListReportsRequest,
      io.github.panyam.agni.v1.webapi.ListReportsResponse> getListReportsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListReports",
      requestType = io.github.panyam.agni.v1.webapi.ListReportsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListReportsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListReportsRequest,
      io.github.panyam.agni.v1.webapi.ListReportsResponse> getListReportsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListReportsRequest, io.github.panyam.agni.v1.webapi.ListReportsResponse> getListReportsMethod;
    if ((getListReportsMethod = DesignServiceGrpc.getListReportsMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getListReportsMethod = DesignServiceGrpc.getListReportsMethod) == null) {
          DesignServiceGrpc.getListReportsMethod = getListReportsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListReportsRequest, io.github.panyam.agni.v1.webapi.ListReportsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListReports"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListReportsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListReportsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("ListReports"))
              .build();
        }
      }
    }
    return getListReportsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteReportRequest,
      com.google.protobuf.Empty> getDeleteReportMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "DeleteReport",
      requestType = io.github.panyam.agni.v1.webapi.DeleteReportRequest.class,
      responseType = com.google.protobuf.Empty.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteReportRequest,
      com.google.protobuf.Empty> getDeleteReportMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteReportRequest, com.google.protobuf.Empty> getDeleteReportMethod;
    if ((getDeleteReportMethod = DesignServiceGrpc.getDeleteReportMethod) == null) {
      synchronized (DesignServiceGrpc.class) {
        if ((getDeleteReportMethod = DesignServiceGrpc.getDeleteReportMethod) == null) {
          DesignServiceGrpc.getDeleteReportMethod = getDeleteReportMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.DeleteReportRequest, com.google.protobuf.Empty>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "DeleteReport"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.DeleteReportRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.google.protobuf.Empty.getDefaultInstance()))
              .setSchemaDescriptor(new DesignServiceMethodDescriptorSupplier("DeleteReport"))
              .build();
        }
      }
    }
    return getDeleteReportMethod;
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
   * This file defines DesignService, which keeps the designs a server holds (agni issues 1694, 1696).
   * A design is named "designs/{design}". It holds artifacts, each one source of the design such as a
   * schematic with its sheets, a board, or a BOM. It ties to no project, and a project lists it by
   * name. DesignViewService (design_view.proto) answers questions about a snapshot of a design instead.
   * A server holds designs in two kinds of place. The store named by `agni serve --store` keeps the
   * designs callers create as records, and `--uploads` keeps their files. Those designs can be changed
   * and deleted. Mounted folders declare designs too, and those are read-only. A design id is unique
   * across both. Nothing on this service names a folder, so each store decides how it keeps a design.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * CreateDesign makes a new design with no artifacts. Its title, series, flatten_hierarchy and
     * intent come from `design`. An id any design already holds is ALREADY_EXISTS. A server started without
     * --store answers FAILED_PRECONDITION, and one without --uploads refuses an artifact's files the
     * same way.
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
     * UpdateDesign changes a stored design's title, series, flatten_hierarchy or intent, the fields
     * update_mask names. A design declared in a mounted folder is read-only, and a write to it is
     * FAILED_PRECONDITION.
     * </pre>
     */
    default void updateDesign(io.github.panyam.agni.v1.webapi.UpdateDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Design> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getUpdateDesignMethod(), responseObserver);
    }

    /**
     * <pre>
     * DeleteDesign removes a stored design and its artifacts. A project listing it keeps the name.
     * </pre>
     */
    default void deleteDesign(io.github.panyam.agni.v1.webapi.DeleteDesignRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeleteDesignMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetArtifact returns one artifact of a stored design.
     * </pre>
     */
    default void getArtifact(io.github.panyam.agni.v1.webapi.GetArtifactRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Artifact> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetArtifactMethod(), responseObserver);
    }

    /**
     * <pre>
     * ListArtifacts returns a design's artifacts, ordered by name. A mounted design has none, since its
     * folder names its files.
     * </pre>
     */
    default void listArtifacts(io.github.panyam.agni.v1.webapi.ListArtifactsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListArtifactsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListArtifactsMethod(), responseObserver);
    }

    /**
     * <pre>
     * UpdateArtifact replaces an artifact's whole file set in one write, so a reader sees the old files
     * or the new ones. With allow_missing it creates the artifact. Contents the server lacks are
     * FAILED_PRECONDITION with a BlobsMissing detail naming their hashes, and the caller sends them with
     * SnapshotService.UploadBlobs and asks again.
     * The server reads the files to find the artifact's type and entry. Files that make two designs
     * are INVALID_ARGUMENT. A write leaving the design with two netlists, two schematics and no netlist,
     * or two boards is FAILED_PRECONDITION, since the server could not tell which one analysis reads.
     * </pre>
     */
    default void updateArtifact(io.github.panyam.agni.v1.webapi.UpdateArtifactRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Artifact> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getUpdateArtifactMethod(), responseObserver);
    }

    /**
     * <pre>
     * BatchUpdateArtifacts applies several UpdateArtifact requests to one design in one write, so the
     * design moves from its old artifacts to its new ones with no state in between. Any request failing
     * fails the batch, and nothing is written.
     * </pre>
     */
    default void batchUpdateArtifacts(io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getBatchUpdateArtifactsMethod(), responseObserver);
    }

    /**
     * <pre>
     * ImportDesignFiles puts a folder's files into a design as its artifacts, grouped the way
     * ProposeDesigns groups a dropped folder (agni issue 1719). The file analysis reads, with every file
     * it reads, becomes one artifact, and each view of it, such as the board, becomes another. Each is
     * named for its type: netlist, schematic, board. All of them are written in one write, as
     * BatchUpdateArtifacts writes, creating or replacing each. A `design.yaml` or `project.yaml` among
     * the files is left out, since the server writes a design's descriptor itself. Files that make no
     * design, or more than one, are INVALID_ARGUMENT naming what they make. Contents the server lacks
     * are FAILED_PRECONDITION with a BlobsMissing detail, as UpdateArtifact reports them.
     * </pre>
     */
    default void importDesignFiles(io.github.panyam.agni.v1.webapi.ImportDesignFilesRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ImportDesignFilesResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getImportDesignFilesMethod(), responseObserver);
    }

    /**
     * <pre>
     * AddDesigns makes designs from boards in a mounted folder (agni issue 1736). The server proposes
     * the folder's designs as WorkspaceService.ProposeDesigns does, and a person picks which to add.
     * Each added design is a stored one: the server copies its files by hash, groups them into
     * artifacts as ImportDesignFiles does, and records each artifact's source in the mount. Its title,
     * series and intent come from the proposal and the request, so the server writes nothing into the
     * mount. A proposal the request does not name is not added. With validate_only the server answers
     * the designs it would make and writes nothing. A folder outside every mount is NOT_FOUND, an id a
     * design already holds is ALREADY_EXISTS, and a server with no --store or --uploads is
     * FAILED_PRECONDITION.
     * </pre>
     */
    default void addDesigns(io.github.panyam.agni.v1.webapi.AddDesignsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.AddDesignsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getAddDesignsMethod(), responseObserver);
    }

    /**
     * <pre>
     * DeleteArtifact removes one artifact. Removing the design's only netlist or schematic leaves the
     * design unable to be read until another is put.
     * </pre>
     */
    default void deleteArtifact(io.github.panyam.agni.v1.webapi.DeleteArtifactRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeleteArtifactMethod(), responseObserver);
    }

    /**
     * <pre>
     * CreateReport checks a stored design against packs and keeps the run as
     * "designs/{design}/reports/{report}" (agni issue 1704). The packs apply in order over the design's
     * own intent: the default packs of the project the request names, then the packs the report names.
     * The run is a full check of the catalog the packs compose, as `agni check` with no --rule runs. A
     * report on a mounted design is FAILED_PRECONDITION, since only a stored design has the content
     * hash a report records. A server without --store answers FAILED_PRECONDITION.
     * </pre>
     */
    default void createReport(io.github.panyam.agni.v1.webapi.CreateReportRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Report> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCreateReportMethod(), responseObserver);
    }

    /**
     * <pre>
     * GetReport returns one report, and says whether the design or a pack changed since it ran.
     * </pre>
     */
    default void getReport(io.github.panyam.agni.v1.webapi.GetReportRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Report> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetReportMethod(), responseObserver);
    }

    /**
     * <pre>
     * ListReports returns a design's reports, newest first, each saying what changed since it ran.
     * </pre>
     */
    default void listReports(io.github.panyam.agni.v1.webapi.ListReportsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListReportsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListReportsMethod(), responseObserver);
    }

    /**
     * <pre>
     * DeleteReport removes one report. DeleteDesign removes a design's reports with it.
     * </pre>
     */
    default void deleteReport(io.github.panyam.agni.v1.webapi.DeleteReportRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeleteReportMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service DesignService.
   * <pre>
   * This file defines DesignService, which keeps the designs a server holds (agni issues 1694, 1696).
   * A design is named "designs/{design}". It holds artifacts, each one source of the design such as a
   * schematic with its sheets, a board, or a BOM. It ties to no project, and a project lists it by
   * name. DesignViewService (design_view.proto) answers questions about a snapshot of a design instead.
   * A server holds designs in two kinds of place. The store named by `agni serve --store` keeps the
   * designs callers create as records, and `--uploads` keeps their files. Those designs can be changed
   * and deleted. Mounted folders declare designs too, and those are read-only. A design id is unique
   * across both. Nothing on this service names a folder, so each store decides how it keeps a design.
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
   * This file defines DesignService, which keeps the designs a server holds (agni issues 1694, 1696).
   * A design is named "designs/{design}". It holds artifacts, each one source of the design such as a
   * schematic with its sheets, a board, or a BOM. It ties to no project, and a project lists it by
   * name. DesignViewService (design_view.proto) answers questions about a snapshot of a design instead.
   * A server holds designs in two kinds of place. The store named by `agni serve --store` keeps the
   * designs callers create as records, and `--uploads` keeps their files. Those designs can be changed
   * and deleted. Mounted folders declare designs too, and those are read-only. A design id is unique
   * across both. Nothing on this service names a folder, so each store decides how it keeps a design.
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
     * CreateDesign makes a new design with no artifacts. Its title, series, flatten_hierarchy and
     * intent come from `design`. An id any design already holds is ALREADY_EXISTS. A server started without
     * --store answers FAILED_PRECONDITION, and one without --uploads refuses an artifact's files the
     * same way.
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
     * UpdateDesign changes a stored design's title, series, flatten_hierarchy or intent, the fields
     * update_mask names. A design declared in a mounted folder is read-only, and a write to it is
     * FAILED_PRECONDITION.
     * </pre>
     */
    public void updateDesign(io.github.panyam.agni.v1.webapi.UpdateDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Design> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getUpdateDesignMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * DeleteDesign removes a stored design and its artifacts. A project listing it keeps the name.
     * </pre>
     */
    public void deleteDesign(io.github.panyam.agni.v1.webapi.DeleteDesignRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeleteDesignMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetArtifact returns one artifact of a stored design.
     * </pre>
     */
    public void getArtifact(io.github.panyam.agni.v1.webapi.GetArtifactRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Artifact> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetArtifactMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ListArtifacts returns a design's artifacts, ordered by name. A mounted design has none, since its
     * folder names its files.
     * </pre>
     */
    public void listArtifacts(io.github.panyam.agni.v1.webapi.ListArtifactsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListArtifactsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListArtifactsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * UpdateArtifact replaces an artifact's whole file set in one write, so a reader sees the old files
     * or the new ones. With allow_missing it creates the artifact. Contents the server lacks are
     * FAILED_PRECONDITION with a BlobsMissing detail naming their hashes, and the caller sends them with
     * SnapshotService.UploadBlobs and asks again.
     * The server reads the files to find the artifact's type and entry. Files that make two designs
     * are INVALID_ARGUMENT. A write leaving the design with two netlists, two schematics and no netlist,
     * or two boards is FAILED_PRECONDITION, since the server could not tell which one analysis reads.
     * </pre>
     */
    public void updateArtifact(io.github.panyam.agni.v1.webapi.UpdateArtifactRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Artifact> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getUpdateArtifactMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * BatchUpdateArtifacts applies several UpdateArtifact requests to one design in one write, so the
     * design moves from its old artifacts to its new ones with no state in between. Any request failing
     * fails the batch, and nothing is written.
     * </pre>
     */
    public void batchUpdateArtifacts(io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getBatchUpdateArtifactsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ImportDesignFiles puts a folder's files into a design as its artifacts, grouped the way
     * ProposeDesigns groups a dropped folder (agni issue 1719). The file analysis reads, with every file
     * it reads, becomes one artifact, and each view of it, such as the board, becomes another. Each is
     * named for its type: netlist, schematic, board. All of them are written in one write, as
     * BatchUpdateArtifacts writes, creating or replacing each. A `design.yaml` or `project.yaml` among
     * the files is left out, since the server writes a design's descriptor itself. Files that make no
     * design, or more than one, are INVALID_ARGUMENT naming what they make. Contents the server lacks
     * are FAILED_PRECONDITION with a BlobsMissing detail, as UpdateArtifact reports them.
     * </pre>
     */
    public void importDesignFiles(io.github.panyam.agni.v1.webapi.ImportDesignFilesRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ImportDesignFilesResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getImportDesignFilesMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * AddDesigns makes designs from boards in a mounted folder (agni issue 1736). The server proposes
     * the folder's designs as WorkspaceService.ProposeDesigns does, and a person picks which to add.
     * Each added design is a stored one: the server copies its files by hash, groups them into
     * artifacts as ImportDesignFiles does, and records each artifact's source in the mount. Its title,
     * series and intent come from the proposal and the request, so the server writes nothing into the
     * mount. A proposal the request does not name is not added. With validate_only the server answers
     * the designs it would make and writes nothing. A folder outside every mount is NOT_FOUND, an id a
     * design already holds is ALREADY_EXISTS, and a server with no --store or --uploads is
     * FAILED_PRECONDITION.
     * </pre>
     */
    public void addDesigns(io.github.panyam.agni.v1.webapi.AddDesignsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.AddDesignsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getAddDesignsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * DeleteArtifact removes one artifact. Removing the design's only netlist or schematic leaves the
     * design unable to be read until another is put.
     * </pre>
     */
    public void deleteArtifact(io.github.panyam.agni.v1.webapi.DeleteArtifactRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeleteArtifactMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * CreateReport checks a stored design against packs and keeps the run as
     * "designs/{design}/reports/{report}" (agni issue 1704). The packs apply in order over the design's
     * own intent: the default packs of the project the request names, then the packs the report names.
     * The run is a full check of the catalog the packs compose, as `agni check` with no --rule runs. A
     * report on a mounted design is FAILED_PRECONDITION, since only a stored design has the content
     * hash a report records. A server without --store answers FAILED_PRECONDITION.
     * </pre>
     */
    public void createReport(io.github.panyam.agni.v1.webapi.CreateReportRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Report> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCreateReportMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * GetReport returns one report, and says whether the design or a pack changed since it ran.
     * </pre>
     */
    public void getReport(io.github.panyam.agni.v1.webapi.GetReportRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Report> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetReportMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ListReports returns a design's reports, newest first, each saying what changed since it ran.
     * </pre>
     */
    public void listReports(io.github.panyam.agni.v1.webapi.ListReportsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListReportsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListReportsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * DeleteReport removes one report. DeleteDesign removes a design's reports with it.
     * </pre>
     */
    public void deleteReport(io.github.panyam.agni.v1.webapi.DeleteReportRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeleteReportMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service DesignService.
   * <pre>
   * This file defines DesignService, which keeps the designs a server holds (agni issues 1694, 1696).
   * A design is named "designs/{design}". It holds artifacts, each one source of the design such as a
   * schematic with its sheets, a board, or a BOM. It ties to no project, and a project lists it by
   * name. DesignViewService (design_view.proto) answers questions about a snapshot of a design instead.
   * A server holds designs in two kinds of place. The store named by `agni serve --store` keeps the
   * designs callers create as records, and `--uploads` keeps their files. Those designs can be changed
   * and deleted. Mounted folders declare designs too, and those are read-only. A design id is unique
   * across both. Nothing on this service names a folder, so each store decides how it keeps a design.
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
     * CreateDesign makes a new design with no artifacts. Its title, series, flatten_hierarchy and
     * intent come from `design`. An id any design already holds is ALREADY_EXISTS. A server started without
     * --store answers FAILED_PRECONDITION, and one without --uploads refuses an artifact's files the
     * same way.
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
     * UpdateDesign changes a stored design's title, series, flatten_hierarchy or intent, the fields
     * update_mask names. A design declared in a mounted folder is read-only, and a write to it is
     * FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Design updateDesign(io.github.panyam.agni.v1.webapi.UpdateDesignRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getUpdateDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteDesign removes a stored design and its artifacts. A project listing it keeps the name.
     * </pre>
     */
    public com.google.protobuf.Empty deleteDesign(io.github.panyam.agni.v1.webapi.DeleteDesignRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getDeleteDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetArtifact returns one artifact of a stored design.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Artifact getArtifact(io.github.panyam.agni.v1.webapi.GetArtifactRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetArtifactMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListArtifacts returns a design's artifacts, ordered by name. A mounted design has none, since its
     * folder names its files.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListArtifactsResponse listArtifacts(io.github.panyam.agni.v1.webapi.ListArtifactsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListArtifactsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * UpdateArtifact replaces an artifact's whole file set in one write, so a reader sees the old files
     * or the new ones. With allow_missing it creates the artifact. Contents the server lacks are
     * FAILED_PRECONDITION with a BlobsMissing detail naming their hashes, and the caller sends them with
     * SnapshotService.UploadBlobs and asks again.
     * The server reads the files to find the artifact's type and entry. Files that make two designs
     * are INVALID_ARGUMENT. A write leaving the design with two netlists, two schematics and no netlist,
     * or two boards is FAILED_PRECONDITION, since the server could not tell which one analysis reads.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Artifact updateArtifact(io.github.panyam.agni.v1.webapi.UpdateArtifactRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getUpdateArtifactMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * BatchUpdateArtifacts applies several UpdateArtifact requests to one design in one write, so the
     * design moves from its old artifacts to its new ones with no state in between. Any request failing
     * fails the batch, and nothing is written.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsResponse batchUpdateArtifacts(io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getBatchUpdateArtifactsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ImportDesignFiles puts a folder's files into a design as its artifacts, grouped the way
     * ProposeDesigns groups a dropped folder (agni issue 1719). The file analysis reads, with every file
     * it reads, becomes one artifact, and each view of it, such as the board, becomes another. Each is
     * named for its type: netlist, schematic, board. All of them are written in one write, as
     * BatchUpdateArtifacts writes, creating or replacing each. A `design.yaml` or `project.yaml` among
     * the files is left out, since the server writes a design's descriptor itself. Files that make no
     * design, or more than one, are INVALID_ARGUMENT naming what they make. Contents the server lacks
     * are FAILED_PRECONDITION with a BlobsMissing detail, as UpdateArtifact reports them.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ImportDesignFilesResponse importDesignFiles(io.github.panyam.agni.v1.webapi.ImportDesignFilesRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getImportDesignFilesMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * AddDesigns makes designs from boards in a mounted folder (agni issue 1736). The server proposes
     * the folder's designs as WorkspaceService.ProposeDesigns does, and a person picks which to add.
     * Each added design is a stored one: the server copies its files by hash, groups them into
     * artifacts as ImportDesignFiles does, and records each artifact's source in the mount. Its title,
     * series and intent come from the proposal and the request, so the server writes nothing into the
     * mount. A proposal the request does not name is not added. With validate_only the server answers
     * the designs it would make and writes nothing. A folder outside every mount is NOT_FOUND, an id a
     * design already holds is ALREADY_EXISTS, and a server with no --store or --uploads is
     * FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.AddDesignsResponse addDesigns(io.github.panyam.agni.v1.webapi.AddDesignsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getAddDesignsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteArtifact removes one artifact. Removing the design's only netlist or schematic leaves the
     * design unable to be read until another is put.
     * </pre>
     */
    public com.google.protobuf.Empty deleteArtifact(io.github.panyam.agni.v1.webapi.DeleteArtifactRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getDeleteArtifactMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * CreateReport checks a stored design against packs and keeps the run as
     * "designs/{design}/reports/{report}" (agni issue 1704). The packs apply in order over the design's
     * own intent: the default packs of the project the request names, then the packs the report names.
     * The run is a full check of the catalog the packs compose, as `agni check` with no --rule runs. A
     * report on a mounted design is FAILED_PRECONDITION, since only a stored design has the content
     * hash a report records. A server without --store answers FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Report createReport(io.github.panyam.agni.v1.webapi.CreateReportRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getCreateReportMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetReport returns one report, and says whether the design or a pack changed since it ran.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Report getReport(io.github.panyam.agni.v1.webapi.GetReportRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetReportMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListReports returns a design's reports, newest first, each saying what changed since it ran.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListReportsResponse listReports(io.github.panyam.agni.v1.webapi.ListReportsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListReportsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteReport removes one report. DeleteDesign removes a design's reports with it.
     * </pre>
     */
    public com.google.protobuf.Empty deleteReport(io.github.panyam.agni.v1.webapi.DeleteReportRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getDeleteReportMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service DesignService.
   * <pre>
   * This file defines DesignService, which keeps the designs a server holds (agni issues 1694, 1696).
   * A design is named "designs/{design}". It holds artifacts, each one source of the design such as a
   * schematic with its sheets, a board, or a BOM. It ties to no project, and a project lists it by
   * name. DesignViewService (design_view.proto) answers questions about a snapshot of a design instead.
   * A server holds designs in two kinds of place. The store named by `agni serve --store` keeps the
   * designs callers create as records, and `--uploads` keeps their files. Those designs can be changed
   * and deleted. Mounted folders declare designs too, and those are read-only. A design id is unique
   * across both. Nothing on this service names a folder, so each store decides how it keeps a design.
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
     * CreateDesign makes a new design with no artifacts. Its title, series, flatten_hierarchy and
     * intent come from `design`. An id any design already holds is ALREADY_EXISTS. A server started without
     * --store answers FAILED_PRECONDITION, and one without --uploads refuses an artifact's files the
     * same way.
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
     * UpdateDesign changes a stored design's title, series, flatten_hierarchy or intent, the fields
     * update_mask names. A design declared in a mounted folder is read-only, and a write to it is
     * FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Design updateDesign(io.github.panyam.agni.v1.webapi.UpdateDesignRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getUpdateDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteDesign removes a stored design and its artifacts. A project listing it keeps the name.
     * </pre>
     */
    public com.google.protobuf.Empty deleteDesign(io.github.panyam.agni.v1.webapi.DeleteDesignRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeleteDesignMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetArtifact returns one artifact of a stored design.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Artifact getArtifact(io.github.panyam.agni.v1.webapi.GetArtifactRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetArtifactMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListArtifacts returns a design's artifacts, ordered by name. A mounted design has none, since its
     * folder names its files.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListArtifactsResponse listArtifacts(io.github.panyam.agni.v1.webapi.ListArtifactsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListArtifactsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * UpdateArtifact replaces an artifact's whole file set in one write, so a reader sees the old files
     * or the new ones. With allow_missing it creates the artifact. Contents the server lacks are
     * FAILED_PRECONDITION with a BlobsMissing detail naming their hashes, and the caller sends them with
     * SnapshotService.UploadBlobs and asks again.
     * The server reads the files to find the artifact's type and entry. Files that make two designs
     * are INVALID_ARGUMENT. A write leaving the design with two netlists, two schematics and no netlist,
     * or two boards is FAILED_PRECONDITION, since the server could not tell which one analysis reads.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Artifact updateArtifact(io.github.panyam.agni.v1.webapi.UpdateArtifactRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getUpdateArtifactMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * BatchUpdateArtifacts applies several UpdateArtifact requests to one design in one write, so the
     * design moves from its old artifacts to its new ones with no state in between. Any request failing
     * fails the batch, and nothing is written.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsResponse batchUpdateArtifacts(io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getBatchUpdateArtifactsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ImportDesignFiles puts a folder's files into a design as its artifacts, grouped the way
     * ProposeDesigns groups a dropped folder (agni issue 1719). The file analysis reads, with every file
     * it reads, becomes one artifact, and each view of it, such as the board, becomes another. Each is
     * named for its type: netlist, schematic, board. All of them are written in one write, as
     * BatchUpdateArtifacts writes, creating or replacing each. A `design.yaml` or `project.yaml` among
     * the files is left out, since the server writes a design's descriptor itself. Files that make no
     * design, or more than one, are INVALID_ARGUMENT naming what they make. Contents the server lacks
     * are FAILED_PRECONDITION with a BlobsMissing detail, as UpdateArtifact reports them.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ImportDesignFilesResponse importDesignFiles(io.github.panyam.agni.v1.webapi.ImportDesignFilesRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getImportDesignFilesMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * AddDesigns makes designs from boards in a mounted folder (agni issue 1736). The server proposes
     * the folder's designs as WorkspaceService.ProposeDesigns does, and a person picks which to add.
     * Each added design is a stored one: the server copies its files by hash, groups them into
     * artifacts as ImportDesignFiles does, and records each artifact's source in the mount. Its title,
     * series and intent come from the proposal and the request, so the server writes nothing into the
     * mount. A proposal the request does not name is not added. With validate_only the server answers
     * the designs it would make and writes nothing. A folder outside every mount is NOT_FOUND, an id a
     * design already holds is ALREADY_EXISTS, and a server with no --store or --uploads is
     * FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.AddDesignsResponse addDesigns(io.github.panyam.agni.v1.webapi.AddDesignsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getAddDesignsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteArtifact removes one artifact. Removing the design's only netlist or schematic leaves the
     * design unable to be read until another is put.
     * </pre>
     */
    public com.google.protobuf.Empty deleteArtifact(io.github.panyam.agni.v1.webapi.DeleteArtifactRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeleteArtifactMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * CreateReport checks a stored design against packs and keeps the run as
     * "designs/{design}/reports/{report}" (agni issue 1704). The packs apply in order over the design's
     * own intent: the default packs of the project the request names, then the packs the report names.
     * The run is a full check of the catalog the packs compose, as `agni check` with no --rule runs. A
     * report on a mounted design is FAILED_PRECONDITION, since only a stored design has the content
     * hash a report records. A server without --store answers FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Report createReport(io.github.panyam.agni.v1.webapi.CreateReportRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCreateReportMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * GetReport returns one report, and says whether the design or a pack changed since it ran.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Report getReport(io.github.panyam.agni.v1.webapi.GetReportRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetReportMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListReports returns a design's reports, newest first, each saying what changed since it ran.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListReportsResponse listReports(io.github.panyam.agni.v1.webapi.ListReportsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListReportsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteReport removes one report. DeleteDesign removes a design's reports with it.
     * </pre>
     */
    public com.google.protobuf.Empty deleteReport(io.github.panyam.agni.v1.webapi.DeleteReportRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeleteReportMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service DesignService.
   * <pre>
   * This file defines DesignService, which keeps the designs a server holds (agni issues 1694, 1696).
   * A design is named "designs/{design}". It holds artifacts, each one source of the design such as a
   * schematic with its sheets, a board, or a BOM. It ties to no project, and a project lists it by
   * name. DesignViewService (design_view.proto) answers questions about a snapshot of a design instead.
   * A server holds designs in two kinds of place. The store named by `agni serve --store` keeps the
   * designs callers create as records, and `--uploads` keeps their files. Those designs can be changed
   * and deleted. Mounted folders declare designs too, and those are read-only. A design id is unique
   * across both. Nothing on this service names a folder, so each store decides how it keeps a design.
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
     * CreateDesign makes a new design with no artifacts. Its title, series, flatten_hierarchy and
     * intent come from `design`. An id any design already holds is ALREADY_EXISTS. A server started without
     * --store answers FAILED_PRECONDITION, and one without --uploads refuses an artifact's files the
     * same way.
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
     * UpdateDesign changes a stored design's title, series, flatten_hierarchy or intent, the fields
     * update_mask names. A design declared in a mounted folder is read-only, and a write to it is
     * FAILED_PRECONDITION.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Design> updateDesign(
        io.github.panyam.agni.v1.webapi.UpdateDesignRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getUpdateDesignMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * DeleteDesign removes a stored design and its artifacts. A project listing it keeps the name.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.google.protobuf.Empty> deleteDesign(
        io.github.panyam.agni.v1.webapi.DeleteDesignRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeleteDesignMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetArtifact returns one artifact of a stored design.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Artifact> getArtifact(
        io.github.panyam.agni.v1.webapi.GetArtifactRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetArtifactMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ListArtifacts returns a design's artifacts, ordered by name. A mounted design has none, since its
     * folder names its files.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListArtifactsResponse> listArtifacts(
        io.github.panyam.agni.v1.webapi.ListArtifactsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListArtifactsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * UpdateArtifact replaces an artifact's whole file set in one write, so a reader sees the old files
     * or the new ones. With allow_missing it creates the artifact. Contents the server lacks are
     * FAILED_PRECONDITION with a BlobsMissing detail naming their hashes, and the caller sends them with
     * SnapshotService.UploadBlobs and asks again.
     * The server reads the files to find the artifact's type and entry. Files that make two designs
     * are INVALID_ARGUMENT. A write leaving the design with two netlists, two schematics and no netlist,
     * or two boards is FAILED_PRECONDITION, since the server could not tell which one analysis reads.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Artifact> updateArtifact(
        io.github.panyam.agni.v1.webapi.UpdateArtifactRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getUpdateArtifactMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * BatchUpdateArtifacts applies several UpdateArtifact requests to one design in one write, so the
     * design moves from its old artifacts to its new ones with no state in between. Any request failing
     * fails the batch, and nothing is written.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsResponse> batchUpdateArtifacts(
        io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getBatchUpdateArtifactsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ImportDesignFiles puts a folder's files into a design as its artifacts, grouped the way
     * ProposeDesigns groups a dropped folder (agni issue 1719). The file analysis reads, with every file
     * it reads, becomes one artifact, and each view of it, such as the board, becomes another. Each is
     * named for its type: netlist, schematic, board. All of them are written in one write, as
     * BatchUpdateArtifacts writes, creating or replacing each. A `design.yaml` or `project.yaml` among
     * the files is left out, since the server writes a design's descriptor itself. Files that make no
     * design, or more than one, are INVALID_ARGUMENT naming what they make. Contents the server lacks
     * are FAILED_PRECONDITION with a BlobsMissing detail, as UpdateArtifact reports them.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ImportDesignFilesResponse> importDesignFiles(
        io.github.panyam.agni.v1.webapi.ImportDesignFilesRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getImportDesignFilesMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * AddDesigns makes designs from boards in a mounted folder (agni issue 1736). The server proposes
     * the folder's designs as WorkspaceService.ProposeDesigns does, and a person picks which to add.
     * Each added design is a stored one: the server copies its files by hash, groups them into
     * artifacts as ImportDesignFiles does, and records each artifact's source in the mount. Its title,
     * series and intent come from the proposal and the request, so the server writes nothing into the
     * mount. A proposal the request does not name is not added. With validate_only the server answers
     * the designs it would make and writes nothing. A folder outside every mount is NOT_FOUND, an id a
     * design already holds is ALREADY_EXISTS, and a server with no --store or --uploads is
     * FAILED_PRECONDITION.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.AddDesignsResponse> addDesigns(
        io.github.panyam.agni.v1.webapi.AddDesignsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getAddDesignsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * DeleteArtifact removes one artifact. Removing the design's only netlist or schematic leaves the
     * design unable to be read until another is put.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.google.protobuf.Empty> deleteArtifact(
        io.github.panyam.agni.v1.webapi.DeleteArtifactRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeleteArtifactMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * CreateReport checks a stored design against packs and keeps the run as
     * "designs/{design}/reports/{report}" (agni issue 1704). The packs apply in order over the design's
     * own intent: the default packs of the project the request names, then the packs the report names.
     * The run is a full check of the catalog the packs compose, as `agni check` with no --rule runs. A
     * report on a mounted design is FAILED_PRECONDITION, since only a stored design has the content
     * hash a report records. A server without --store answers FAILED_PRECONDITION.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Report> createReport(
        io.github.panyam.agni.v1.webapi.CreateReportRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCreateReportMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * GetReport returns one report, and says whether the design or a pack changed since it ran.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Report> getReport(
        io.github.panyam.agni.v1.webapi.GetReportRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetReportMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ListReports returns a design's reports, newest first, each saying what changed since it ran.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListReportsResponse> listReports(
        io.github.panyam.agni.v1.webapi.ListReportsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListReportsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * DeleteReport removes one report. DeleteDesign removes a design's reports with it.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.google.protobuf.Empty> deleteReport(
        io.github.panyam.agni.v1.webapi.DeleteReportRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeleteReportMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_CREATE_DESIGN = 0;
  private static final int METHODID_GET_DESIGN = 1;
  private static final int METHODID_LIST_DESIGNS = 2;
  private static final int METHODID_UPDATE_DESIGN = 3;
  private static final int METHODID_DELETE_DESIGN = 4;
  private static final int METHODID_GET_ARTIFACT = 5;
  private static final int METHODID_LIST_ARTIFACTS = 6;
  private static final int METHODID_UPDATE_ARTIFACT = 7;
  private static final int METHODID_BATCH_UPDATE_ARTIFACTS = 8;
  private static final int METHODID_IMPORT_DESIGN_FILES = 9;
  private static final int METHODID_ADD_DESIGNS = 10;
  private static final int METHODID_DELETE_ARTIFACT = 11;
  private static final int METHODID_CREATE_REPORT = 12;
  private static final int METHODID_GET_REPORT = 13;
  private static final int METHODID_LIST_REPORTS = 14;
  private static final int METHODID_DELETE_REPORT = 15;

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
        case METHODID_GET_ARTIFACT:
          serviceImpl.getArtifact((io.github.panyam.agni.v1.webapi.GetArtifactRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Artifact>) responseObserver);
          break;
        case METHODID_LIST_ARTIFACTS:
          serviceImpl.listArtifacts((io.github.panyam.agni.v1.webapi.ListArtifactsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListArtifactsResponse>) responseObserver);
          break;
        case METHODID_UPDATE_ARTIFACT:
          serviceImpl.updateArtifact((io.github.panyam.agni.v1.webapi.UpdateArtifactRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Artifact>) responseObserver);
          break;
        case METHODID_BATCH_UPDATE_ARTIFACTS:
          serviceImpl.batchUpdateArtifacts((io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsResponse>) responseObserver);
          break;
        case METHODID_IMPORT_DESIGN_FILES:
          serviceImpl.importDesignFiles((io.github.panyam.agni.v1.webapi.ImportDesignFilesRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ImportDesignFilesResponse>) responseObserver);
          break;
        case METHODID_ADD_DESIGNS:
          serviceImpl.addDesigns((io.github.panyam.agni.v1.webapi.AddDesignsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.AddDesignsResponse>) responseObserver);
          break;
        case METHODID_DELETE_ARTIFACT:
          serviceImpl.deleteArtifact((io.github.panyam.agni.v1.webapi.DeleteArtifactRequest) request,
              (io.grpc.stub.StreamObserver<com.google.protobuf.Empty>) responseObserver);
          break;
        case METHODID_CREATE_REPORT:
          serviceImpl.createReport((io.github.panyam.agni.v1.webapi.CreateReportRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Report>) responseObserver);
          break;
        case METHODID_GET_REPORT:
          serviceImpl.getReport((io.github.panyam.agni.v1.webapi.GetReportRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Report>) responseObserver);
          break;
        case METHODID_LIST_REPORTS:
          serviceImpl.listReports((io.github.panyam.agni.v1.webapi.ListReportsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListReportsResponse>) responseObserver);
          break;
        case METHODID_DELETE_REPORT:
          serviceImpl.deleteReport((io.github.panyam.agni.v1.webapi.DeleteReportRequest) request,
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
        .addMethod(
          getGetArtifactMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetArtifactRequest,
              io.github.panyam.agni.v1.webapi.Artifact>(
                service, METHODID_GET_ARTIFACT)))
        .addMethod(
          getListArtifactsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListArtifactsRequest,
              io.github.panyam.agni.v1.webapi.ListArtifactsResponse>(
                service, METHODID_LIST_ARTIFACTS)))
        .addMethod(
          getUpdateArtifactMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.UpdateArtifactRequest,
              io.github.panyam.agni.v1.webapi.Artifact>(
                service, METHODID_UPDATE_ARTIFACT)))
        .addMethod(
          getBatchUpdateArtifactsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsRequest,
              io.github.panyam.agni.v1.webapi.BatchUpdateArtifactsResponse>(
                service, METHODID_BATCH_UPDATE_ARTIFACTS)))
        .addMethod(
          getImportDesignFilesMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ImportDesignFilesRequest,
              io.github.panyam.agni.v1.webapi.ImportDesignFilesResponse>(
                service, METHODID_IMPORT_DESIGN_FILES)))
        .addMethod(
          getAddDesignsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.AddDesignsRequest,
              io.github.panyam.agni.v1.webapi.AddDesignsResponse>(
                service, METHODID_ADD_DESIGNS)))
        .addMethod(
          getDeleteArtifactMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.DeleteArtifactRequest,
              com.google.protobuf.Empty>(
                service, METHODID_DELETE_ARTIFACT)))
        .addMethod(
          getCreateReportMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.CreateReportRequest,
              io.github.panyam.agni.v1.webapi.Report>(
                service, METHODID_CREATE_REPORT)))
        .addMethod(
          getGetReportMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetReportRequest,
              io.github.panyam.agni.v1.webapi.Report>(
                service, METHODID_GET_REPORT)))
        .addMethod(
          getListReportsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListReportsRequest,
              io.github.panyam.agni.v1.webapi.ListReportsResponse>(
                service, METHODID_LIST_REPORTS)))
        .addMethod(
          getDeleteReportMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.DeleteReportRequest,
              com.google.protobuf.Empty>(
                service, METHODID_DELETE_REPORT)))
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
              .addMethod(getGetArtifactMethod())
              .addMethod(getListArtifactsMethod())
              .addMethod(getUpdateArtifactMethod())
              .addMethod(getBatchUpdateArtifactsMethod())
              .addMethod(getImportDesignFilesMethod())
              .addMethod(getAddDesignsMethod())
              .addMethod(getDeleteArtifactMethod())
              .addMethod(getCreateReportMethod())
              .addMethod(getGetReportMethod())
              .addMethod(getListReportsMethod())
              .addMethod(getDeleteReportMethod())
              .build();
        }
      }
    }
    return result;
  }
}
