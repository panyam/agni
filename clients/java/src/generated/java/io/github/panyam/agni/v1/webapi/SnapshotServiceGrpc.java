package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * This file defines SnapshotService. A snapshot is a design loaded once and asked many questions
 * (agni issue 1061).
 * Every analysis rpc names a snapshot rather than files. Ingest creates a snapshot from a source. The
 * snapshot resolves the design's tiers and its config once. CheckDesign, RunQuery, GetSheet and the
 * rest then answer about that snapshot. So a question never resolves a companion, a project or a
 * convention again, and two questions about one snapshot cannot read the design two ways.
 * Each Ingest makes a new snapshot with an id of its own, even of the same files under the same
 * config. Two ingests are two designs, and a client can change each without the other. The server
 * still shares what it read for them underneath, keyed on the files' content. So a second ingest of
 * one design costs a lookup and holds no second copy until one of the two is updated.
 * A snapshot's files are still live after the ingest. The snapshot records every file its questions
 * read. Imagine one of those files changes. The next question is refused as stale
 * (FAILED_PRECONDITION) rather than answered from the files as they are now, so an answer is always
 * about the revision it names. UpdateSnapshot reads the snapshot again, whole or one tier at a time.
 * A snapshot is held in the host's memory. A server keeps it until it is deleted or evicted, and a
 * browser worker keeps it until the worker ends. The host also keeps each snapshot's definition, in
 * memory or in a store that outlives a restart (agni issue 1639). A question on a snapshot the host
 * evicted or lost to a restart then ingests it again under the same name and answers, and a client
 * need keep nothing. Only a snapshot of uploaded files (files set) is not recovered after a restart,
 * since its files went with the process, so it still answers NOT_FOUND. ServerInfo's features say
 * whether a host recovers (snapshot_recovery) and whether it does so across a restart
 * (snapshot_store).
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class SnapshotServiceGrpc {

  private SnapshotServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.SnapshotService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.IngestRequest,
      io.github.panyam.agni.v1.webapi.Snapshot> getIngestMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Ingest",
      requestType = io.github.panyam.agni.v1.webapi.IngestRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Snapshot.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.IngestRequest,
      io.github.panyam.agni.v1.webapi.Snapshot> getIngestMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.IngestRequest, io.github.panyam.agni.v1.webapi.Snapshot> getIngestMethod;
    if ((getIngestMethod = SnapshotServiceGrpc.getIngestMethod) == null) {
      synchronized (SnapshotServiceGrpc.class) {
        if ((getIngestMethod = SnapshotServiceGrpc.getIngestMethod) == null) {
          SnapshotServiceGrpc.getIngestMethod = getIngestMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.IngestRequest, io.github.panyam.agni.v1.webapi.Snapshot>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Ingest"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.IngestRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Snapshot.getDefaultInstance()))
              .setSchemaDescriptor(new SnapshotServiceMethodDescriptorSupplier("Ingest"))
              .build();
        }
      }
    }
    return getIngestMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetSnapshotRequest,
      io.github.panyam.agni.v1.webapi.Snapshot> getGetSnapshotMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetSnapshot",
      requestType = io.github.panyam.agni.v1.webapi.GetSnapshotRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Snapshot.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetSnapshotRequest,
      io.github.panyam.agni.v1.webapi.Snapshot> getGetSnapshotMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetSnapshotRequest, io.github.panyam.agni.v1.webapi.Snapshot> getGetSnapshotMethod;
    if ((getGetSnapshotMethod = SnapshotServiceGrpc.getGetSnapshotMethod) == null) {
      synchronized (SnapshotServiceGrpc.class) {
        if ((getGetSnapshotMethod = SnapshotServiceGrpc.getGetSnapshotMethod) == null) {
          SnapshotServiceGrpc.getGetSnapshotMethod = getGetSnapshotMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetSnapshotRequest, io.github.panyam.agni.v1.webapi.Snapshot>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetSnapshot"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetSnapshotRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Snapshot.getDefaultInstance()))
              .setSchemaDescriptor(new SnapshotServiceMethodDescriptorSupplier("GetSnapshot"))
              .build();
        }
      }
    }
    return getGetSnapshotMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListSnapshotsRequest,
      io.github.panyam.agni.v1.webapi.ListSnapshotsResponse> getListSnapshotsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListSnapshots",
      requestType = io.github.panyam.agni.v1.webapi.ListSnapshotsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListSnapshotsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListSnapshotsRequest,
      io.github.panyam.agni.v1.webapi.ListSnapshotsResponse> getListSnapshotsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListSnapshotsRequest, io.github.panyam.agni.v1.webapi.ListSnapshotsResponse> getListSnapshotsMethod;
    if ((getListSnapshotsMethod = SnapshotServiceGrpc.getListSnapshotsMethod) == null) {
      synchronized (SnapshotServiceGrpc.class) {
        if ((getListSnapshotsMethod = SnapshotServiceGrpc.getListSnapshotsMethod) == null) {
          SnapshotServiceGrpc.getListSnapshotsMethod = getListSnapshotsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListSnapshotsRequest, io.github.panyam.agni.v1.webapi.ListSnapshotsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListSnapshots"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListSnapshotsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListSnapshotsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new SnapshotServiceMethodDescriptorSupplier("ListSnapshots"))
              .build();
        }
      }
    }
    return getListSnapshotsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdateSnapshotRequest,
      io.github.panyam.agni.v1.webapi.Snapshot> getUpdateSnapshotMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "UpdateSnapshot",
      requestType = io.github.panyam.agni.v1.webapi.UpdateSnapshotRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Snapshot.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdateSnapshotRequest,
      io.github.panyam.agni.v1.webapi.Snapshot> getUpdateSnapshotMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdateSnapshotRequest, io.github.panyam.agni.v1.webapi.Snapshot> getUpdateSnapshotMethod;
    if ((getUpdateSnapshotMethod = SnapshotServiceGrpc.getUpdateSnapshotMethod) == null) {
      synchronized (SnapshotServiceGrpc.class) {
        if ((getUpdateSnapshotMethod = SnapshotServiceGrpc.getUpdateSnapshotMethod) == null) {
          SnapshotServiceGrpc.getUpdateSnapshotMethod = getUpdateSnapshotMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.UpdateSnapshotRequest, io.github.panyam.agni.v1.webapi.Snapshot>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "UpdateSnapshot"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.UpdateSnapshotRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Snapshot.getDefaultInstance()))
              .setSchemaDescriptor(new SnapshotServiceMethodDescriptorSupplier("UpdateSnapshot"))
              .build();
        }
      }
    }
    return getUpdateSnapshotMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest,
      com.google.protobuf.Empty> getDeleteSnapshotMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "DeleteSnapshot",
      requestType = io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest.class,
      responseType = com.google.protobuf.Empty.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest,
      com.google.protobuf.Empty> getDeleteSnapshotMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest, com.google.protobuf.Empty> getDeleteSnapshotMethod;
    if ((getDeleteSnapshotMethod = SnapshotServiceGrpc.getDeleteSnapshotMethod) == null) {
      synchronized (SnapshotServiceGrpc.class) {
        if ((getDeleteSnapshotMethod = SnapshotServiceGrpc.getDeleteSnapshotMethod) == null) {
          SnapshotServiceGrpc.getDeleteSnapshotMethod = getDeleteSnapshotMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest, com.google.protobuf.Empty>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "DeleteSnapshot"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.google.protobuf.Empty.getDefaultInstance()))
              .setSchemaDescriptor(new SnapshotServiceMethodDescriptorSupplier("DeleteSnapshot"))
              .build();
        }
      }
    }
    return getDeleteSnapshotMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UploadBlobsRequest,
      io.github.panyam.agni.v1.webapi.UploadBlobsResponse> getUploadBlobsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "UploadBlobs",
      requestType = io.github.panyam.agni.v1.webapi.UploadBlobsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.UploadBlobsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UploadBlobsRequest,
      io.github.panyam.agni.v1.webapi.UploadBlobsResponse> getUploadBlobsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UploadBlobsRequest, io.github.panyam.agni.v1.webapi.UploadBlobsResponse> getUploadBlobsMethod;
    if ((getUploadBlobsMethod = SnapshotServiceGrpc.getUploadBlobsMethod) == null) {
      synchronized (SnapshotServiceGrpc.class) {
        if ((getUploadBlobsMethod = SnapshotServiceGrpc.getUploadBlobsMethod) == null) {
          SnapshotServiceGrpc.getUploadBlobsMethod = getUploadBlobsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.UploadBlobsRequest, io.github.panyam.agni.v1.webapi.UploadBlobsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "UploadBlobs"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.UploadBlobsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.UploadBlobsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new SnapshotServiceMethodDescriptorSupplier("UploadBlobs"))
              .build();
        }
      }
    }
    return getUploadBlobsMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static SnapshotServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<SnapshotServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<SnapshotServiceStub>() {
        @java.lang.Override
        public SnapshotServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new SnapshotServiceStub(channel, callOptions);
        }
      };
    return SnapshotServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static SnapshotServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<SnapshotServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<SnapshotServiceBlockingV2Stub>() {
        @java.lang.Override
        public SnapshotServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new SnapshotServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return SnapshotServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static SnapshotServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<SnapshotServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<SnapshotServiceBlockingStub>() {
        @java.lang.Override
        public SnapshotServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new SnapshotServiceBlockingStub(channel, callOptions);
        }
      };
    return SnapshotServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static SnapshotServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<SnapshotServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<SnapshotServiceFutureStub>() {
        @java.lang.Override
        public SnapshotServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new SnapshotServiceFutureStub(channel, callOptions);
        }
      };
    return SnapshotServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * This file defines SnapshotService. A snapshot is a design loaded once and asked many questions
   * (agni issue 1061).
   * Every analysis rpc names a snapshot rather than files. Ingest creates a snapshot from a source. The
   * snapshot resolves the design's tiers and its config once. CheckDesign, RunQuery, GetSheet and the
   * rest then answer about that snapshot. So a question never resolves a companion, a project or a
   * convention again, and two questions about one snapshot cannot read the design two ways.
   * Each Ingest makes a new snapshot with an id of its own, even of the same files under the same
   * config. Two ingests are two designs, and a client can change each without the other. The server
   * still shares what it read for them underneath, keyed on the files' content. So a second ingest of
   * one design costs a lookup and holds no second copy until one of the two is updated.
   * A snapshot's files are still live after the ingest. The snapshot records every file its questions
   * read. Imagine one of those files changes. The next question is refused as stale
   * (FAILED_PRECONDITION) rather than answered from the files as they are now, so an answer is always
   * about the revision it names. UpdateSnapshot reads the snapshot again, whole or one tier at a time.
   * A snapshot is held in the host's memory. A server keeps it until it is deleted or evicted, and a
   * browser worker keeps it until the worker ends. The host also keeps each snapshot's definition, in
   * memory or in a store that outlives a restart (agni issue 1639). A question on a snapshot the host
   * evicted or lost to a restart then ingests it again under the same name and answers, and a client
   * need keep nothing. Only a snapshot of uploaded files (files set) is not recovered after a restart,
   * since its files went with the process, so it still answers NOT_FOUND. ServerInfo's features say
   * whether a host recovers (snapshot_recovery) and whether it does so across a restart
   * (snapshot_store).
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * Ingest creates a snapshot from a source. It resolves the design's tiers and composes its config,
     * and it reads no design file. Opening a KiCad design draws from its schematic's geometry, and the
     * netlist is parsed only for the first check or query. So a design that cannot be read fails at its
     * first question. With read_board, Ingest builds the same model a check builds, board included, and
     * a later check finds it parsed. Opening a design must not wait on its board (agni issue 943).
     * </pre>
     */
    default void ingest(io.github.panyam.agni.v1.webapi.IngestRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Snapshot> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getIngestMethod(), responseObserver);
    }

    /**
     */
    default void getSnapshot(io.github.panyam.agni.v1.webapi.GetSnapshotRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Snapshot> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetSnapshotMethod(), responseObserver);
    }

    /**
     */
    default void listSnapshots(io.github.panyam.agni.v1.webapi.ListSnapshotsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListSnapshotsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListSnapshotsMethod(), responseObserver);
    }

    /**
     * <pre>
     * UpdateSnapshot replaces a snapshot's definition, whole or by update_mask, and reads it again.
     * Its id does not change, and its revision_id moves on. An update that cannot be read leaves the
     * snapshot as it was.
     * </pre>
     */
    default void updateSnapshot(io.github.panyam.agni.v1.webapi.UpdateSnapshotRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Snapshot> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getUpdateSnapshotMethod(), responseObserver);
    }

    /**
     */
    default void deleteSnapshot(io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeleteSnapshotMethod(), responseObserver);
    }

    /**
     * <pre>
     * UploadBlobs stores the file contents named in an Ingest's manifest (agni issue 1061), keyed by
     * their SHA-256, and answers each one's hash. The server keeps what it can within its limit. It may
     * drop a blob that no snapshot holds, so a client sends a blob only when an Ingest reports it
     * missing.
     * </pre>
     */
    default void uploadBlobs(io.github.panyam.agni.v1.webapi.UploadBlobsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.UploadBlobsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getUploadBlobsMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service SnapshotService.
   * <pre>
   * This file defines SnapshotService. A snapshot is a design loaded once and asked many questions
   * (agni issue 1061).
   * Every analysis rpc names a snapshot rather than files. Ingest creates a snapshot from a source. The
   * snapshot resolves the design's tiers and its config once. CheckDesign, RunQuery, GetSheet and the
   * rest then answer about that snapshot. So a question never resolves a companion, a project or a
   * convention again, and two questions about one snapshot cannot read the design two ways.
   * Each Ingest makes a new snapshot with an id of its own, even of the same files under the same
   * config. Two ingests are two designs, and a client can change each without the other. The server
   * still shares what it read for them underneath, keyed on the files' content. So a second ingest of
   * one design costs a lookup and holds no second copy until one of the two is updated.
   * A snapshot's files are still live after the ingest. The snapshot records every file its questions
   * read. Imagine one of those files changes. The next question is refused as stale
   * (FAILED_PRECONDITION) rather than answered from the files as they are now, so an answer is always
   * about the revision it names. UpdateSnapshot reads the snapshot again, whole or one tier at a time.
   * A snapshot is held in the host's memory. A server keeps it until it is deleted or evicted, and a
   * browser worker keeps it until the worker ends. The host also keeps each snapshot's definition, in
   * memory or in a store that outlives a restart (agni issue 1639). A question on a snapshot the host
   * evicted or lost to a restart then ingests it again under the same name and answers, and a client
   * need keep nothing. Only a snapshot of uploaded files (files set) is not recovered after a restart,
   * since its files went with the process, so it still answers NOT_FOUND. ServerInfo's features say
   * whether a host recovers (snapshot_recovery) and whether it does so across a restart
   * (snapshot_store).
   * </pre>
   */
  public static abstract class SnapshotServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return SnapshotServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service SnapshotService.
   * <pre>
   * This file defines SnapshotService. A snapshot is a design loaded once and asked many questions
   * (agni issue 1061).
   * Every analysis rpc names a snapshot rather than files. Ingest creates a snapshot from a source. The
   * snapshot resolves the design's tiers and its config once. CheckDesign, RunQuery, GetSheet and the
   * rest then answer about that snapshot. So a question never resolves a companion, a project or a
   * convention again, and two questions about one snapshot cannot read the design two ways.
   * Each Ingest makes a new snapshot with an id of its own, even of the same files under the same
   * config. Two ingests are two designs, and a client can change each without the other. The server
   * still shares what it read for them underneath, keyed on the files' content. So a second ingest of
   * one design costs a lookup and holds no second copy until one of the two is updated.
   * A snapshot's files are still live after the ingest. The snapshot records every file its questions
   * read. Imagine one of those files changes. The next question is refused as stale
   * (FAILED_PRECONDITION) rather than answered from the files as they are now, so an answer is always
   * about the revision it names. UpdateSnapshot reads the snapshot again, whole or one tier at a time.
   * A snapshot is held in the host's memory. A server keeps it until it is deleted or evicted, and a
   * browser worker keeps it until the worker ends. The host also keeps each snapshot's definition, in
   * memory or in a store that outlives a restart (agni issue 1639). A question on a snapshot the host
   * evicted or lost to a restart then ingests it again under the same name and answers, and a client
   * need keep nothing. Only a snapshot of uploaded files (files set) is not recovered after a restart,
   * since its files went with the process, so it still answers NOT_FOUND. ServerInfo's features say
   * whether a host recovers (snapshot_recovery) and whether it does so across a restart
   * (snapshot_store).
   * </pre>
   */
  public static final class SnapshotServiceStub
      extends io.grpc.stub.AbstractAsyncStub<SnapshotServiceStub> {
    private SnapshotServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected SnapshotServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new SnapshotServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * Ingest creates a snapshot from a source. It resolves the design's tiers and composes its config,
     * and it reads no design file. Opening a KiCad design draws from its schematic's geometry, and the
     * netlist is parsed only for the first check or query. So a design that cannot be read fails at its
     * first question. With read_board, Ingest builds the same model a check builds, board included, and
     * a later check finds it parsed. Opening a design must not wait on its board (agni issue 943).
     * </pre>
     */
    public void ingest(io.github.panyam.agni.v1.webapi.IngestRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Snapshot> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getIngestMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void getSnapshot(io.github.panyam.agni.v1.webapi.GetSnapshotRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Snapshot> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetSnapshotMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void listSnapshots(io.github.panyam.agni.v1.webapi.ListSnapshotsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListSnapshotsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListSnapshotsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * UpdateSnapshot replaces a snapshot's definition, whole or by update_mask, and reads it again.
     * Its id does not change, and its revision_id moves on. An update that cannot be read leaves the
     * snapshot as it was.
     * </pre>
     */
    public void updateSnapshot(io.github.panyam.agni.v1.webapi.UpdateSnapshotRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Snapshot> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getUpdateSnapshotMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void deleteSnapshot(io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeleteSnapshotMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * UploadBlobs stores the file contents named in an Ingest's manifest (agni issue 1061), keyed by
     * their SHA-256, and answers each one's hash. The server keeps what it can within its limit. It may
     * drop a blob that no snapshot holds, so a client sends a blob only when an Ingest reports it
     * missing.
     * </pre>
     */
    public void uploadBlobs(io.github.panyam.agni.v1.webapi.UploadBlobsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.UploadBlobsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getUploadBlobsMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service SnapshotService.
   * <pre>
   * This file defines SnapshotService. A snapshot is a design loaded once and asked many questions
   * (agni issue 1061).
   * Every analysis rpc names a snapshot rather than files. Ingest creates a snapshot from a source. The
   * snapshot resolves the design's tiers and its config once. CheckDesign, RunQuery, GetSheet and the
   * rest then answer about that snapshot. So a question never resolves a companion, a project or a
   * convention again, and two questions about one snapshot cannot read the design two ways.
   * Each Ingest makes a new snapshot with an id of its own, even of the same files under the same
   * config. Two ingests are two designs, and a client can change each without the other. The server
   * still shares what it read for them underneath, keyed on the files' content. So a second ingest of
   * one design costs a lookup and holds no second copy until one of the two is updated.
   * A snapshot's files are still live after the ingest. The snapshot records every file its questions
   * read. Imagine one of those files changes. The next question is refused as stale
   * (FAILED_PRECONDITION) rather than answered from the files as they are now, so an answer is always
   * about the revision it names. UpdateSnapshot reads the snapshot again, whole or one tier at a time.
   * A snapshot is held in the host's memory. A server keeps it until it is deleted or evicted, and a
   * browser worker keeps it until the worker ends. The host also keeps each snapshot's definition, in
   * memory or in a store that outlives a restart (agni issue 1639). A question on a snapshot the host
   * evicted or lost to a restart then ingests it again under the same name and answers, and a client
   * need keep nothing. Only a snapshot of uploaded files (files set) is not recovered after a restart,
   * since its files went with the process, so it still answers NOT_FOUND. ServerInfo's features say
   * whether a host recovers (snapshot_recovery) and whether it does so across a restart
   * (snapshot_store).
   * </pre>
   */
  public static final class SnapshotServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<SnapshotServiceBlockingV2Stub> {
    private SnapshotServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected SnapshotServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new SnapshotServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * Ingest creates a snapshot from a source. It resolves the design's tiers and composes its config,
     * and it reads no design file. Opening a KiCad design draws from its schematic's geometry, and the
     * netlist is parsed only for the first check or query. So a design that cannot be read fails at its
     * first question. With read_board, Ingest builds the same model a check builds, board included, and
     * a later check finds it parsed. Opening a design must not wait on its board (agni issue 943).
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Snapshot ingest(io.github.panyam.agni.v1.webapi.IngestRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getIngestMethod(), getCallOptions(), request);
    }

    /**
     */
    public io.github.panyam.agni.v1.webapi.Snapshot getSnapshot(io.github.panyam.agni.v1.webapi.GetSnapshotRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetSnapshotMethod(), getCallOptions(), request);
    }

    /**
     */
    public io.github.panyam.agni.v1.webapi.ListSnapshotsResponse listSnapshots(io.github.panyam.agni.v1.webapi.ListSnapshotsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListSnapshotsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * UpdateSnapshot replaces a snapshot's definition, whole or by update_mask, and reads it again.
     * Its id does not change, and its revision_id moves on. An update that cannot be read leaves the
     * snapshot as it was.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Snapshot updateSnapshot(io.github.panyam.agni.v1.webapi.UpdateSnapshotRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getUpdateSnapshotMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.google.protobuf.Empty deleteSnapshot(io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getDeleteSnapshotMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * UploadBlobs stores the file contents named in an Ingest's manifest (agni issue 1061), keyed by
     * their SHA-256, and answers each one's hash. The server keeps what it can within its limit. It may
     * drop a blob that no snapshot holds, so a client sends a blob only when an Ingest reports it
     * missing.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.UploadBlobsResponse uploadBlobs(io.github.panyam.agni.v1.webapi.UploadBlobsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getUploadBlobsMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service SnapshotService.
   * <pre>
   * This file defines SnapshotService. A snapshot is a design loaded once and asked many questions
   * (agni issue 1061).
   * Every analysis rpc names a snapshot rather than files. Ingest creates a snapshot from a source. The
   * snapshot resolves the design's tiers and its config once. CheckDesign, RunQuery, GetSheet and the
   * rest then answer about that snapshot. So a question never resolves a companion, a project or a
   * convention again, and two questions about one snapshot cannot read the design two ways.
   * Each Ingest makes a new snapshot with an id of its own, even of the same files under the same
   * config. Two ingests are two designs, and a client can change each without the other. The server
   * still shares what it read for them underneath, keyed on the files' content. So a second ingest of
   * one design costs a lookup and holds no second copy until one of the two is updated.
   * A snapshot's files are still live after the ingest. The snapshot records every file its questions
   * read. Imagine one of those files changes. The next question is refused as stale
   * (FAILED_PRECONDITION) rather than answered from the files as they are now, so an answer is always
   * about the revision it names. UpdateSnapshot reads the snapshot again, whole or one tier at a time.
   * A snapshot is held in the host's memory. A server keeps it until it is deleted or evicted, and a
   * browser worker keeps it until the worker ends. The host also keeps each snapshot's definition, in
   * memory or in a store that outlives a restart (agni issue 1639). A question on a snapshot the host
   * evicted or lost to a restart then ingests it again under the same name and answers, and a client
   * need keep nothing. Only a snapshot of uploaded files (files set) is not recovered after a restart,
   * since its files went with the process, so it still answers NOT_FOUND. ServerInfo's features say
   * whether a host recovers (snapshot_recovery) and whether it does so across a restart
   * (snapshot_store).
   * </pre>
   */
  public static final class SnapshotServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<SnapshotServiceBlockingStub> {
    private SnapshotServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected SnapshotServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new SnapshotServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * Ingest creates a snapshot from a source. It resolves the design's tiers and composes its config,
     * and it reads no design file. Opening a KiCad design draws from its schematic's geometry, and the
     * netlist is parsed only for the first check or query. So a design that cannot be read fails at its
     * first question. With read_board, Ingest builds the same model a check builds, board included, and
     * a later check finds it parsed. Opening a design must not wait on its board (agni issue 943).
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Snapshot ingest(io.github.panyam.agni.v1.webapi.IngestRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getIngestMethod(), getCallOptions(), request);
    }

    /**
     */
    public io.github.panyam.agni.v1.webapi.Snapshot getSnapshot(io.github.panyam.agni.v1.webapi.GetSnapshotRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetSnapshotMethod(), getCallOptions(), request);
    }

    /**
     */
    public io.github.panyam.agni.v1.webapi.ListSnapshotsResponse listSnapshots(io.github.panyam.agni.v1.webapi.ListSnapshotsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListSnapshotsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * UpdateSnapshot replaces a snapshot's definition, whole or by update_mask, and reads it again.
     * Its id does not change, and its revision_id moves on. An update that cannot be read leaves the
     * snapshot as it was.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Snapshot updateSnapshot(io.github.panyam.agni.v1.webapi.UpdateSnapshotRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getUpdateSnapshotMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.google.protobuf.Empty deleteSnapshot(io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeleteSnapshotMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * UploadBlobs stores the file contents named in an Ingest's manifest (agni issue 1061), keyed by
     * their SHA-256, and answers each one's hash. The server keeps what it can within its limit. It may
     * drop a blob that no snapshot holds, so a client sends a blob only when an Ingest reports it
     * missing.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.UploadBlobsResponse uploadBlobs(io.github.panyam.agni.v1.webapi.UploadBlobsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getUploadBlobsMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service SnapshotService.
   * <pre>
   * This file defines SnapshotService. A snapshot is a design loaded once and asked many questions
   * (agni issue 1061).
   * Every analysis rpc names a snapshot rather than files. Ingest creates a snapshot from a source. The
   * snapshot resolves the design's tiers and its config once. CheckDesign, RunQuery, GetSheet and the
   * rest then answer about that snapshot. So a question never resolves a companion, a project or a
   * convention again, and two questions about one snapshot cannot read the design two ways.
   * Each Ingest makes a new snapshot with an id of its own, even of the same files under the same
   * config. Two ingests are two designs, and a client can change each without the other. The server
   * still shares what it read for them underneath, keyed on the files' content. So a second ingest of
   * one design costs a lookup and holds no second copy until one of the two is updated.
   * A snapshot's files are still live after the ingest. The snapshot records every file its questions
   * read. Imagine one of those files changes. The next question is refused as stale
   * (FAILED_PRECONDITION) rather than answered from the files as they are now, so an answer is always
   * about the revision it names. UpdateSnapshot reads the snapshot again, whole or one tier at a time.
   * A snapshot is held in the host's memory. A server keeps it until it is deleted or evicted, and a
   * browser worker keeps it until the worker ends. The host also keeps each snapshot's definition, in
   * memory or in a store that outlives a restart (agni issue 1639). A question on a snapshot the host
   * evicted or lost to a restart then ingests it again under the same name and answers, and a client
   * need keep nothing. Only a snapshot of uploaded files (files set) is not recovered after a restart,
   * since its files went with the process, so it still answers NOT_FOUND. ServerInfo's features say
   * whether a host recovers (snapshot_recovery) and whether it does so across a restart
   * (snapshot_store).
   * </pre>
   */
  public static final class SnapshotServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<SnapshotServiceFutureStub> {
    private SnapshotServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected SnapshotServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new SnapshotServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * Ingest creates a snapshot from a source. It resolves the design's tiers and composes its config,
     * and it reads no design file. Opening a KiCad design draws from its schematic's geometry, and the
     * netlist is parsed only for the first check or query. So a design that cannot be read fails at its
     * first question. With read_board, Ingest builds the same model a check builds, board included, and
     * a later check finds it parsed. Opening a design must not wait on its board (agni issue 943).
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Snapshot> ingest(
        io.github.panyam.agni.v1.webapi.IngestRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getIngestMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Snapshot> getSnapshot(
        io.github.panyam.agni.v1.webapi.GetSnapshotRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetSnapshotMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListSnapshotsResponse> listSnapshots(
        io.github.panyam.agni.v1.webapi.ListSnapshotsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListSnapshotsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * UpdateSnapshot replaces a snapshot's definition, whole or by update_mask, and reads it again.
     * Its id does not change, and its revision_id moves on. An update that cannot be read leaves the
     * snapshot as it was.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Snapshot> updateSnapshot(
        io.github.panyam.agni.v1.webapi.UpdateSnapshotRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getUpdateSnapshotMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.google.protobuf.Empty> deleteSnapshot(
        io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeleteSnapshotMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * UploadBlobs stores the file contents named in an Ingest's manifest (agni issue 1061), keyed by
     * their SHA-256, and answers each one's hash. The server keeps what it can within its limit. It may
     * drop a blob that no snapshot holds, so a client sends a blob only when an Ingest reports it
     * missing.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.UploadBlobsResponse> uploadBlobs(
        io.github.panyam.agni.v1.webapi.UploadBlobsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getUploadBlobsMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_INGEST = 0;
  private static final int METHODID_GET_SNAPSHOT = 1;
  private static final int METHODID_LIST_SNAPSHOTS = 2;
  private static final int METHODID_UPDATE_SNAPSHOT = 3;
  private static final int METHODID_DELETE_SNAPSHOT = 4;
  private static final int METHODID_UPLOAD_BLOBS = 5;

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
        case METHODID_INGEST:
          serviceImpl.ingest((io.github.panyam.agni.v1.webapi.IngestRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Snapshot>) responseObserver);
          break;
        case METHODID_GET_SNAPSHOT:
          serviceImpl.getSnapshot((io.github.panyam.agni.v1.webapi.GetSnapshotRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Snapshot>) responseObserver);
          break;
        case METHODID_LIST_SNAPSHOTS:
          serviceImpl.listSnapshots((io.github.panyam.agni.v1.webapi.ListSnapshotsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListSnapshotsResponse>) responseObserver);
          break;
        case METHODID_UPDATE_SNAPSHOT:
          serviceImpl.updateSnapshot((io.github.panyam.agni.v1.webapi.UpdateSnapshotRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Snapshot>) responseObserver);
          break;
        case METHODID_DELETE_SNAPSHOT:
          serviceImpl.deleteSnapshot((io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest) request,
              (io.grpc.stub.StreamObserver<com.google.protobuf.Empty>) responseObserver);
          break;
        case METHODID_UPLOAD_BLOBS:
          serviceImpl.uploadBlobs((io.github.panyam.agni.v1.webapi.UploadBlobsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.UploadBlobsResponse>) responseObserver);
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
          getIngestMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.IngestRequest,
              io.github.panyam.agni.v1.webapi.Snapshot>(
                service, METHODID_INGEST)))
        .addMethod(
          getGetSnapshotMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetSnapshotRequest,
              io.github.panyam.agni.v1.webapi.Snapshot>(
                service, METHODID_GET_SNAPSHOT)))
        .addMethod(
          getListSnapshotsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListSnapshotsRequest,
              io.github.panyam.agni.v1.webapi.ListSnapshotsResponse>(
                service, METHODID_LIST_SNAPSHOTS)))
        .addMethod(
          getUpdateSnapshotMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.UpdateSnapshotRequest,
              io.github.panyam.agni.v1.webapi.Snapshot>(
                service, METHODID_UPDATE_SNAPSHOT)))
        .addMethod(
          getDeleteSnapshotMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.DeleteSnapshotRequest,
              com.google.protobuf.Empty>(
                service, METHODID_DELETE_SNAPSHOT)))
        .addMethod(
          getUploadBlobsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.UploadBlobsRequest,
              io.github.panyam.agni.v1.webapi.UploadBlobsResponse>(
                service, METHODID_UPLOAD_BLOBS)))
        .build();
  }

  private static abstract class SnapshotServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    SnapshotServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.SnapshotProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("SnapshotService");
    }
  }

  private static final class SnapshotServiceFileDescriptorSupplier
      extends SnapshotServiceBaseDescriptorSupplier {
    SnapshotServiceFileDescriptorSupplier() {}
  }

  private static final class SnapshotServiceMethodDescriptorSupplier
      extends SnapshotServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    SnapshotServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (SnapshotServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new SnapshotServiceFileDescriptorSupplier())
              .addMethod(getIngestMethod())
              .addMethod(getGetSnapshotMethod())
              .addMethod(getListSnapshotsMethod())
              .addMethod(getUpdateSnapshotMethod())
              .addMethod(getDeleteSnapshotMethod())
              .addMethod(getUploadBlobsMethod())
              .build();
        }
      }
    }
    return result;
  }
}
