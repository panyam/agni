package io.github.panyam.agni.v1.webapi;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * This file defines ProjectService. It answers "which design is this, and whose design is it"
 * (agni issue 170).
 * Everything else in this API addresses a FILE. Almost everything the review layer needs is scoped to
 * something larger, and until now the engine had no name for it. Imagine a folder holding a netlist,
 * a schematic export, and a board. The folder says nothing about which of the three IS the design.
 * Some config decides what a design is checked against (naming vocabulary, interface profiles, seeded
 * parameters, a checklist). That config applies to some designs and not others. With no model of the
 * larger thing, the config could only be process-wide server flags. A deployment serving a mixed set
 * then applies them to every design it reads.
 * There are two resources, and the split follows the on-disk layout of a review project.
 *   Project  owns what is shared across designs (a team's config).
 *   Design   owns what is specific to one: which file is the analysis source, and which files are
 *            companion VIEWS of that same design rather than independent sources of it.
 * "Project" is an overloaded word here, and the overloading is worth naming. A KiCad `.kicad_pro` is
 * DESIGN-scoped. It is one board's own file, and it resolves to a Design below, never to a Project.
 * An overlay repository is a Go module that extends the engine, and has nothing to do with this. The
 * engine's Project is the scope of a team's REVIEW CONFIG. Neither of the other two names that.
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class ProjectServiceGrpc {

  private ProjectServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "agni.v1.webapi.ProjectService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetProjectRequest,
      io.github.panyam.agni.v1.webapi.Project> getGetProjectMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetProject",
      requestType = io.github.panyam.agni.v1.webapi.GetProjectRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Project.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetProjectRequest,
      io.github.panyam.agni.v1.webapi.Project> getGetProjectMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.GetProjectRequest, io.github.panyam.agni.v1.webapi.Project> getGetProjectMethod;
    if ((getGetProjectMethod = ProjectServiceGrpc.getGetProjectMethod) == null) {
      synchronized (ProjectServiceGrpc.class) {
        if ((getGetProjectMethod = ProjectServiceGrpc.getGetProjectMethod) == null) {
          ProjectServiceGrpc.getGetProjectMethod = getGetProjectMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.GetProjectRequest, io.github.panyam.agni.v1.webapi.Project>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetProject"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.GetProjectRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Project.getDefaultInstance()))
              .setSchemaDescriptor(new ProjectServiceMethodDescriptorSupplier("GetProject"))
              .build();
        }
      }
    }
    return getGetProjectMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListProjectsRequest,
      io.github.panyam.agni.v1.webapi.ListProjectsResponse> getListProjectsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListProjects",
      requestType = io.github.panyam.agni.v1.webapi.ListProjectsRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ListProjectsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListProjectsRequest,
      io.github.panyam.agni.v1.webapi.ListProjectsResponse> getListProjectsMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ListProjectsRequest, io.github.panyam.agni.v1.webapi.ListProjectsResponse> getListProjectsMethod;
    if ((getListProjectsMethod = ProjectServiceGrpc.getListProjectsMethod) == null) {
      synchronized (ProjectServiceGrpc.class) {
        if ((getListProjectsMethod = ProjectServiceGrpc.getListProjectsMethod) == null) {
          ProjectServiceGrpc.getListProjectsMethod = getListProjectsMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ListProjectsRequest, io.github.panyam.agni.v1.webapi.ListProjectsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListProjects"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListProjectsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ListProjectsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ProjectServiceMethodDescriptorSupplier("ListProjects"))
              .build();
        }
      }
    }
    return getListProjectsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateProjectRequest,
      io.github.panyam.agni.v1.webapi.Project> getCreateProjectMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CreateProject",
      requestType = io.github.panyam.agni.v1.webapi.CreateProjectRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Project.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateProjectRequest,
      io.github.panyam.agni.v1.webapi.Project> getCreateProjectMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.CreateProjectRequest, io.github.panyam.agni.v1.webapi.Project> getCreateProjectMethod;
    if ((getCreateProjectMethod = ProjectServiceGrpc.getCreateProjectMethod) == null) {
      synchronized (ProjectServiceGrpc.class) {
        if ((getCreateProjectMethod = ProjectServiceGrpc.getCreateProjectMethod) == null) {
          ProjectServiceGrpc.getCreateProjectMethod = getCreateProjectMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.CreateProjectRequest, io.github.panyam.agni.v1.webapi.Project>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CreateProject"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.CreateProjectRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Project.getDefaultInstance()))
              .setSchemaDescriptor(new ProjectServiceMethodDescriptorSupplier("CreateProject"))
              .build();
        }
      }
    }
    return getCreateProjectMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdateProjectRequest,
      io.github.panyam.agni.v1.webapi.Project> getUpdateProjectMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "UpdateProject",
      requestType = io.github.panyam.agni.v1.webapi.UpdateProjectRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.Project.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdateProjectRequest,
      io.github.panyam.agni.v1.webapi.Project> getUpdateProjectMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.UpdateProjectRequest, io.github.panyam.agni.v1.webapi.Project> getUpdateProjectMethod;
    if ((getUpdateProjectMethod = ProjectServiceGrpc.getUpdateProjectMethod) == null) {
      synchronized (ProjectServiceGrpc.class) {
        if ((getUpdateProjectMethod = ProjectServiceGrpc.getUpdateProjectMethod) == null) {
          ProjectServiceGrpc.getUpdateProjectMethod = getUpdateProjectMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.UpdateProjectRequest, io.github.panyam.agni.v1.webapi.Project>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "UpdateProject"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.UpdateProjectRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.Project.getDefaultInstance()))
              .setSchemaDescriptor(new ProjectServiceMethodDescriptorSupplier("UpdateProject"))
              .build();
        }
      }
    }
    return getUpdateProjectMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteProjectRequest,
      com.google.protobuf.Empty> getDeleteProjectMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "DeleteProject",
      requestType = io.github.panyam.agni.v1.webapi.DeleteProjectRequest.class,
      responseType = com.google.protobuf.Empty.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteProjectRequest,
      com.google.protobuf.Empty> getDeleteProjectMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.DeleteProjectRequest, com.google.protobuf.Empty> getDeleteProjectMethod;
    if ((getDeleteProjectMethod = ProjectServiceGrpc.getDeleteProjectMethod) == null) {
      synchronized (ProjectServiceGrpc.class) {
        if ((getDeleteProjectMethod = ProjectServiceGrpc.getDeleteProjectMethod) == null) {
          ProjectServiceGrpc.getDeleteProjectMethod = getDeleteProjectMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.DeleteProjectRequest, com.google.protobuf.Empty>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "DeleteProject"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.DeleteProjectRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.google.protobuf.Empty.getDefaultInstance()))
              .setSchemaDescriptor(new ProjectServiceMethodDescriptorSupplier("DeleteProject"))
              .build();
        }
      }
    }
    return getDeleteProjectMethod;
  }

  private static volatile io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ResolveDesignRequest,
      io.github.panyam.agni.v1.webapi.ResolveDesignResponse> getResolveDesignMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ResolveDesign",
      requestType = io.github.panyam.agni.v1.webapi.ResolveDesignRequest.class,
      responseType = io.github.panyam.agni.v1.webapi.ResolveDesignResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ResolveDesignRequest,
      io.github.panyam.agni.v1.webapi.ResolveDesignResponse> getResolveDesignMethod() {
    io.grpc.MethodDescriptor<io.github.panyam.agni.v1.webapi.ResolveDesignRequest, io.github.panyam.agni.v1.webapi.ResolveDesignResponse> getResolveDesignMethod;
    if ((getResolveDesignMethod = ProjectServiceGrpc.getResolveDesignMethod) == null) {
      synchronized (ProjectServiceGrpc.class) {
        if ((getResolveDesignMethod = ProjectServiceGrpc.getResolveDesignMethod) == null) {
          ProjectServiceGrpc.getResolveDesignMethod = getResolveDesignMethod =
              io.grpc.MethodDescriptor.<io.github.panyam.agni.v1.webapi.ResolveDesignRequest, io.github.panyam.agni.v1.webapi.ResolveDesignResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ResolveDesign"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ResolveDesignRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  io.github.panyam.agni.v1.webapi.ResolveDesignResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ProjectServiceMethodDescriptorSupplier("ResolveDesign"))
              .build();
        }
      }
    }
    return getResolveDesignMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static ProjectServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ProjectServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ProjectServiceStub>() {
        @java.lang.Override
        public ProjectServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ProjectServiceStub(channel, callOptions);
        }
      };
    return ProjectServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static ProjectServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ProjectServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ProjectServiceBlockingV2Stub>() {
        @java.lang.Override
        public ProjectServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ProjectServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return ProjectServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static ProjectServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ProjectServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ProjectServiceBlockingStub>() {
        @java.lang.Override
        public ProjectServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ProjectServiceBlockingStub(channel, callOptions);
        }
      };
    return ProjectServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static ProjectServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ProjectServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ProjectServiceFutureStub>() {
        @java.lang.Override
        public ProjectServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ProjectServiceFutureStub(channel, callOptions);
        }
      };
    return ProjectServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * This file defines ProjectService. It answers "which design is this, and whose design is it"
   * (agni issue 170).
   * Everything else in this API addresses a FILE. Almost everything the review layer needs is scoped to
   * something larger, and until now the engine had no name for it. Imagine a folder holding a netlist,
   * a schematic export, and a board. The folder says nothing about which of the three IS the design.
   * Some config decides what a design is checked against (naming vocabulary, interface profiles, seeded
   * parameters, a checklist). That config applies to some designs and not others. With no model of the
   * larger thing, the config could only be process-wide server flags. A deployment serving a mixed set
   * then applies them to every design it reads.
   * There are two resources, and the split follows the on-disk layout of a review project.
   *   Project  owns what is shared across designs (a team's config).
   *   Design   owns what is specific to one: which file is the analysis source, and which files are
   *            companion VIEWS of that same design rather than independent sources of it.
   * "Project" is an overloaded word here, and the overloading is worth naming. A KiCad `.kicad_pro` is
   * DESIGN-scoped. It is one board's own file, and it resolves to a Design below, never to a Project.
   * An overlay repository is a Go module that extends the engine, and has nothing to do with this. The
   * engine's Project is the scope of a team's REVIEW CONFIG. Neither of the other two names that.
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * GetProject returns one project by resource name.
     * </pre>
     */
    default void getProject(io.github.panyam.agni.v1.webapi.GetProjectRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Project> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetProjectMethod(), responseObserver);
    }

    /**
     * <pre>
     * ListProjects returns the projects visible across the server's mounts and its store.
     * </pre>
     */
    default void listProjects(io.github.panyam.agni.v1.webapi.ListProjectsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListProjectsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListProjectsMethod(), responseObserver);
    }

    /**
     * <pre>
     * CreateProject stores a new project, a list of designs (agni issue 1694). The caller picks its id
     * (AIP-122). An id a mounted project already declares is ALREADY_EXISTS, and a server started
     * without --store answers FAILED_PRECONDITION.
     * </pre>
     */
    default void createProject(io.github.panyam.agni.v1.webapi.CreateProjectRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Project> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCreateProjectMethod(), responseObserver);
    }

    /**
     * <pre>
     * UpdateProject changes a stored project's title or its designs, the fields update_mask names. A
     * project declared in a mounted folder is read-only, and a write to it is FAILED_PRECONDITION.
     * </pre>
     */
    default void updateProject(io.github.panyam.agni.v1.webapi.UpdateProjectRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Project> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getUpdateProjectMethod(), responseObserver);
    }

    /**
     * <pre>
     * DeleteProject removes a stored project. Its designs stay, since a project only lists them.
     * </pre>
     */
    default void deleteProject(io.github.panyam.agni.v1.webapi.DeleteProjectRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeleteProjectMethod(), responseObserver);
    }

    /**
     * <pre>
     * ResolveDesign answers a viewer's question when it opens a file. Is this file part of a declared
     * design, and if so which one? This rpc keeps resolution an INTERFACE rather than a path
     * convention. A file-backed store walks the tree for descriptors, and another store may consult an
     * index. A caller cannot tell which one it is talking to.
     * Resolving to nothing is a normal answer, not an error. A mounted folder with no descriptors is
     * the ordinary case. A file that resolves to no design gets the plain viewer and the built-in
     * catalog, so it cannot be checked against some other project's rules. So the project surfaces
     * cannot be misapplied, by structure rather than by convention.
     * </pre>
     */
    default void resolveDesign(io.github.panyam.agni.v1.webapi.ResolveDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ResolveDesignResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getResolveDesignMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service ProjectService.
   * <pre>
   * This file defines ProjectService. It answers "which design is this, and whose design is it"
   * (agni issue 170).
   * Everything else in this API addresses a FILE. Almost everything the review layer needs is scoped to
   * something larger, and until now the engine had no name for it. Imagine a folder holding a netlist,
   * a schematic export, and a board. The folder says nothing about which of the three IS the design.
   * Some config decides what a design is checked against (naming vocabulary, interface profiles, seeded
   * parameters, a checklist). That config applies to some designs and not others. With no model of the
   * larger thing, the config could only be process-wide server flags. A deployment serving a mixed set
   * then applies them to every design it reads.
   * There are two resources, and the split follows the on-disk layout of a review project.
   *   Project  owns what is shared across designs (a team's config).
   *   Design   owns what is specific to one: which file is the analysis source, and which files are
   *            companion VIEWS of that same design rather than independent sources of it.
   * "Project" is an overloaded word here, and the overloading is worth naming. A KiCad `.kicad_pro` is
   * DESIGN-scoped. It is one board's own file, and it resolves to a Design below, never to a Project.
   * An overlay repository is a Go module that extends the engine, and has nothing to do with this. The
   * engine's Project is the scope of a team's REVIEW CONFIG. Neither of the other two names that.
   * </pre>
   */
  public static abstract class ProjectServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return ProjectServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service ProjectService.
   * <pre>
   * This file defines ProjectService. It answers "which design is this, and whose design is it"
   * (agni issue 170).
   * Everything else in this API addresses a FILE. Almost everything the review layer needs is scoped to
   * something larger, and until now the engine had no name for it. Imagine a folder holding a netlist,
   * a schematic export, and a board. The folder says nothing about which of the three IS the design.
   * Some config decides what a design is checked against (naming vocabulary, interface profiles, seeded
   * parameters, a checklist). That config applies to some designs and not others. With no model of the
   * larger thing, the config could only be process-wide server flags. A deployment serving a mixed set
   * then applies them to every design it reads.
   * There are two resources, and the split follows the on-disk layout of a review project.
   *   Project  owns what is shared across designs (a team's config).
   *   Design   owns what is specific to one: which file is the analysis source, and which files are
   *            companion VIEWS of that same design rather than independent sources of it.
   * "Project" is an overloaded word here, and the overloading is worth naming. A KiCad `.kicad_pro` is
   * DESIGN-scoped. It is one board's own file, and it resolves to a Design below, never to a Project.
   * An overlay repository is a Go module that extends the engine, and has nothing to do with this. The
   * engine's Project is the scope of a team's REVIEW CONFIG. Neither of the other two names that.
   * </pre>
   */
  public static final class ProjectServiceStub
      extends io.grpc.stub.AbstractAsyncStub<ProjectServiceStub> {
    private ProjectServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ProjectServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ProjectServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetProject returns one project by resource name.
     * </pre>
     */
    public void getProject(io.github.panyam.agni.v1.webapi.GetProjectRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Project> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetProjectMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ListProjects returns the projects visible across the server's mounts and its store.
     * </pre>
     */
    public void listProjects(io.github.panyam.agni.v1.webapi.ListProjectsRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListProjectsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListProjectsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * CreateProject stores a new project, a list of designs (agni issue 1694). The caller picks its id
     * (AIP-122). An id a mounted project already declares is ALREADY_EXISTS, and a server started
     * without --store answers FAILED_PRECONDITION.
     * </pre>
     */
    public void createProject(io.github.panyam.agni.v1.webapi.CreateProjectRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Project> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCreateProjectMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * UpdateProject changes a stored project's title or its designs, the fields update_mask names. A
     * project declared in a mounted folder is read-only, and a write to it is FAILED_PRECONDITION.
     * </pre>
     */
    public void updateProject(io.github.panyam.agni.v1.webapi.UpdateProjectRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Project> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getUpdateProjectMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * DeleteProject removes a stored project. Its designs stay, since a project only lists them.
     * </pre>
     */
    public void deleteProject(io.github.panyam.agni.v1.webapi.DeleteProjectRequest request,
        io.grpc.stub.StreamObserver<com.google.protobuf.Empty> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeleteProjectMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ResolveDesign answers a viewer's question when it opens a file. Is this file part of a declared
     * design, and if so which one? This rpc keeps resolution an INTERFACE rather than a path
     * convention. A file-backed store walks the tree for descriptors, and another store may consult an
     * index. A caller cannot tell which one it is talking to.
     * Resolving to nothing is a normal answer, not an error. A mounted folder with no descriptors is
     * the ordinary case. A file that resolves to no design gets the plain viewer and the built-in
     * catalog, so it cannot be checked against some other project's rules. So the project surfaces
     * cannot be misapplied, by structure rather than by convention.
     * </pre>
     */
    public void resolveDesign(io.github.panyam.agni.v1.webapi.ResolveDesignRequest request,
        io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ResolveDesignResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getResolveDesignMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service ProjectService.
   * <pre>
   * This file defines ProjectService. It answers "which design is this, and whose design is it"
   * (agni issue 170).
   * Everything else in this API addresses a FILE. Almost everything the review layer needs is scoped to
   * something larger, and until now the engine had no name for it. Imagine a folder holding a netlist,
   * a schematic export, and a board. The folder says nothing about which of the three IS the design.
   * Some config decides what a design is checked against (naming vocabulary, interface profiles, seeded
   * parameters, a checklist). That config applies to some designs and not others. With no model of the
   * larger thing, the config could only be process-wide server flags. A deployment serving a mixed set
   * then applies them to every design it reads.
   * There are two resources, and the split follows the on-disk layout of a review project.
   *   Project  owns what is shared across designs (a team's config).
   *   Design   owns what is specific to one: which file is the analysis source, and which files are
   *            companion VIEWS of that same design rather than independent sources of it.
   * "Project" is an overloaded word here, and the overloading is worth naming. A KiCad `.kicad_pro` is
   * DESIGN-scoped. It is one board's own file, and it resolves to a Design below, never to a Project.
   * An overlay repository is a Go module that extends the engine, and has nothing to do with this. The
   * engine's Project is the scope of a team's REVIEW CONFIG. Neither of the other two names that.
   * </pre>
   */
  public static final class ProjectServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<ProjectServiceBlockingV2Stub> {
    private ProjectServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ProjectServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ProjectServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * GetProject returns one project by resource name.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Project getProject(io.github.panyam.agni.v1.webapi.GetProjectRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetProjectMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListProjects returns the projects visible across the server's mounts and its store.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListProjectsResponse listProjects(io.github.panyam.agni.v1.webapi.ListProjectsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListProjectsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * CreateProject stores a new project, a list of designs (agni issue 1694). The caller picks its id
     * (AIP-122). An id a mounted project already declares is ALREADY_EXISTS, and a server started
     * without --store answers FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Project createProject(io.github.panyam.agni.v1.webapi.CreateProjectRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getCreateProjectMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * UpdateProject changes a stored project's title or its designs, the fields update_mask names. A
     * project declared in a mounted folder is read-only, and a write to it is FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Project updateProject(io.github.panyam.agni.v1.webapi.UpdateProjectRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getUpdateProjectMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteProject removes a stored project. Its designs stay, since a project only lists them.
     * </pre>
     */
    public com.google.protobuf.Empty deleteProject(io.github.panyam.agni.v1.webapi.DeleteProjectRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getDeleteProjectMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ResolveDesign answers a viewer's question when it opens a file. Is this file part of a declared
     * design, and if so which one? This rpc keeps resolution an INTERFACE rather than a path
     * convention. A file-backed store walks the tree for descriptors, and another store may consult an
     * index. A caller cannot tell which one it is talking to.
     * Resolving to nothing is a normal answer, not an error. A mounted folder with no descriptors is
     * the ordinary case. A file that resolves to no design gets the plain viewer and the built-in
     * catalog, so it cannot be checked against some other project's rules. So the project surfaces
     * cannot be misapplied, by structure rather than by convention.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ResolveDesignResponse resolveDesign(io.github.panyam.agni.v1.webapi.ResolveDesignRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getResolveDesignMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service ProjectService.
   * <pre>
   * This file defines ProjectService. It answers "which design is this, and whose design is it"
   * (agni issue 170).
   * Everything else in this API addresses a FILE. Almost everything the review layer needs is scoped to
   * something larger, and until now the engine had no name for it. Imagine a folder holding a netlist,
   * a schematic export, and a board. The folder says nothing about which of the three IS the design.
   * Some config decides what a design is checked against (naming vocabulary, interface profiles, seeded
   * parameters, a checklist). That config applies to some designs and not others. With no model of the
   * larger thing, the config could only be process-wide server flags. A deployment serving a mixed set
   * then applies them to every design it reads.
   * There are two resources, and the split follows the on-disk layout of a review project.
   *   Project  owns what is shared across designs (a team's config).
   *   Design   owns what is specific to one: which file is the analysis source, and which files are
   *            companion VIEWS of that same design rather than independent sources of it.
   * "Project" is an overloaded word here, and the overloading is worth naming. A KiCad `.kicad_pro` is
   * DESIGN-scoped. It is one board's own file, and it resolves to a Design below, never to a Project.
   * An overlay repository is a Go module that extends the engine, and has nothing to do with this. The
   * engine's Project is the scope of a team's REVIEW CONFIG. Neither of the other two names that.
   * </pre>
   */
  public static final class ProjectServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<ProjectServiceBlockingStub> {
    private ProjectServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ProjectServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ProjectServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetProject returns one project by resource name.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Project getProject(io.github.panyam.agni.v1.webapi.GetProjectRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetProjectMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ListProjects returns the projects visible across the server's mounts and its store.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ListProjectsResponse listProjects(io.github.panyam.agni.v1.webapi.ListProjectsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListProjectsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * CreateProject stores a new project, a list of designs (agni issue 1694). The caller picks its id
     * (AIP-122). An id a mounted project already declares is ALREADY_EXISTS, and a server started
     * without --store answers FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Project createProject(io.github.panyam.agni.v1.webapi.CreateProjectRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCreateProjectMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * UpdateProject changes a stored project's title or its designs, the fields update_mask names. A
     * project declared in a mounted folder is read-only, and a write to it is FAILED_PRECONDITION.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.Project updateProject(io.github.panyam.agni.v1.webapi.UpdateProjectRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getUpdateProjectMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * DeleteProject removes a stored project. Its designs stay, since a project only lists them.
     * </pre>
     */
    public com.google.protobuf.Empty deleteProject(io.github.panyam.agni.v1.webapi.DeleteProjectRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeleteProjectMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ResolveDesign answers a viewer's question when it opens a file. Is this file part of a declared
     * design, and if so which one? This rpc keeps resolution an INTERFACE rather than a path
     * convention. A file-backed store walks the tree for descriptors, and another store may consult an
     * index. A caller cannot tell which one it is talking to.
     * Resolving to nothing is a normal answer, not an error. A mounted folder with no descriptors is
     * the ordinary case. A file that resolves to no design gets the plain viewer and the built-in
     * catalog, so it cannot be checked against some other project's rules. So the project surfaces
     * cannot be misapplied, by structure rather than by convention.
     * </pre>
     */
    public io.github.panyam.agni.v1.webapi.ResolveDesignResponse resolveDesign(io.github.panyam.agni.v1.webapi.ResolveDesignRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getResolveDesignMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service ProjectService.
   * <pre>
   * This file defines ProjectService. It answers "which design is this, and whose design is it"
   * (agni issue 170).
   * Everything else in this API addresses a FILE. Almost everything the review layer needs is scoped to
   * something larger, and until now the engine had no name for it. Imagine a folder holding a netlist,
   * a schematic export, and a board. The folder says nothing about which of the three IS the design.
   * Some config decides what a design is checked against (naming vocabulary, interface profiles, seeded
   * parameters, a checklist). That config applies to some designs and not others. With no model of the
   * larger thing, the config could only be process-wide server flags. A deployment serving a mixed set
   * then applies them to every design it reads.
   * There are two resources, and the split follows the on-disk layout of a review project.
   *   Project  owns what is shared across designs (a team's config).
   *   Design   owns what is specific to one: which file is the analysis source, and which files are
   *            companion VIEWS of that same design rather than independent sources of it.
   * "Project" is an overloaded word here, and the overloading is worth naming. A KiCad `.kicad_pro` is
   * DESIGN-scoped. It is one board's own file, and it resolves to a Design below, never to a Project.
   * An overlay repository is a Go module that extends the engine, and has nothing to do with this. The
   * engine's Project is the scope of a team's REVIEW CONFIG. Neither of the other two names that.
   * </pre>
   */
  public static final class ProjectServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<ProjectServiceFutureStub> {
    private ProjectServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ProjectServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ProjectServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * GetProject returns one project by resource name.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Project> getProject(
        io.github.panyam.agni.v1.webapi.GetProjectRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetProjectMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ListProjects returns the projects visible across the server's mounts and its store.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ListProjectsResponse> listProjects(
        io.github.panyam.agni.v1.webapi.ListProjectsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListProjectsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * CreateProject stores a new project, a list of designs (agni issue 1694). The caller picks its id
     * (AIP-122). An id a mounted project already declares is ALREADY_EXISTS, and a server started
     * without --store answers FAILED_PRECONDITION.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Project> createProject(
        io.github.panyam.agni.v1.webapi.CreateProjectRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCreateProjectMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * UpdateProject changes a stored project's title or its designs, the fields update_mask names. A
     * project declared in a mounted folder is read-only, and a write to it is FAILED_PRECONDITION.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.Project> updateProject(
        io.github.panyam.agni.v1.webapi.UpdateProjectRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getUpdateProjectMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * DeleteProject removes a stored project. Its designs stay, since a project only lists them.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.google.protobuf.Empty> deleteProject(
        io.github.panyam.agni.v1.webapi.DeleteProjectRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeleteProjectMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ResolveDesign answers a viewer's question when it opens a file. Is this file part of a declared
     * design, and if so which one? This rpc keeps resolution an INTERFACE rather than a path
     * convention. A file-backed store walks the tree for descriptors, and another store may consult an
     * index. A caller cannot tell which one it is talking to.
     * Resolving to nothing is a normal answer, not an error. A mounted folder with no descriptors is
     * the ordinary case. A file that resolves to no design gets the plain viewer and the built-in
     * catalog, so it cannot be checked against some other project's rules. So the project surfaces
     * cannot be misapplied, by structure rather than by convention.
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<io.github.panyam.agni.v1.webapi.ResolveDesignResponse> resolveDesign(
        io.github.panyam.agni.v1.webapi.ResolveDesignRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getResolveDesignMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_GET_PROJECT = 0;
  private static final int METHODID_LIST_PROJECTS = 1;
  private static final int METHODID_CREATE_PROJECT = 2;
  private static final int METHODID_UPDATE_PROJECT = 3;
  private static final int METHODID_DELETE_PROJECT = 4;
  private static final int METHODID_RESOLVE_DESIGN = 5;

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
        case METHODID_GET_PROJECT:
          serviceImpl.getProject((io.github.panyam.agni.v1.webapi.GetProjectRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Project>) responseObserver);
          break;
        case METHODID_LIST_PROJECTS:
          serviceImpl.listProjects((io.github.panyam.agni.v1.webapi.ListProjectsRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ListProjectsResponse>) responseObserver);
          break;
        case METHODID_CREATE_PROJECT:
          serviceImpl.createProject((io.github.panyam.agni.v1.webapi.CreateProjectRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Project>) responseObserver);
          break;
        case METHODID_UPDATE_PROJECT:
          serviceImpl.updateProject((io.github.panyam.agni.v1.webapi.UpdateProjectRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.Project>) responseObserver);
          break;
        case METHODID_DELETE_PROJECT:
          serviceImpl.deleteProject((io.github.panyam.agni.v1.webapi.DeleteProjectRequest) request,
              (io.grpc.stub.StreamObserver<com.google.protobuf.Empty>) responseObserver);
          break;
        case METHODID_RESOLVE_DESIGN:
          serviceImpl.resolveDesign((io.github.panyam.agni.v1.webapi.ResolveDesignRequest) request,
              (io.grpc.stub.StreamObserver<io.github.panyam.agni.v1.webapi.ResolveDesignResponse>) responseObserver);
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
          getGetProjectMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.GetProjectRequest,
              io.github.panyam.agni.v1.webapi.Project>(
                service, METHODID_GET_PROJECT)))
        .addMethod(
          getListProjectsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ListProjectsRequest,
              io.github.panyam.agni.v1.webapi.ListProjectsResponse>(
                service, METHODID_LIST_PROJECTS)))
        .addMethod(
          getCreateProjectMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.CreateProjectRequest,
              io.github.panyam.agni.v1.webapi.Project>(
                service, METHODID_CREATE_PROJECT)))
        .addMethod(
          getUpdateProjectMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.UpdateProjectRequest,
              io.github.panyam.agni.v1.webapi.Project>(
                service, METHODID_UPDATE_PROJECT)))
        .addMethod(
          getDeleteProjectMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.DeleteProjectRequest,
              com.google.protobuf.Empty>(
                service, METHODID_DELETE_PROJECT)))
        .addMethod(
          getResolveDesignMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              io.github.panyam.agni.v1.webapi.ResolveDesignRequest,
              io.github.panyam.agni.v1.webapi.ResolveDesignResponse>(
                service, METHODID_RESOLVE_DESIGN)))
        .build();
  }

  private static abstract class ProjectServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    ProjectServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return io.github.panyam.agni.v1.webapi.ProjectProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("ProjectService");
    }
  }

  private static final class ProjectServiceFileDescriptorSupplier
      extends ProjectServiceBaseDescriptorSupplier {
    ProjectServiceFileDescriptorSupplier() {}
  }

  private static final class ProjectServiceMethodDescriptorSupplier
      extends ProjectServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    ProjectServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (ProjectServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new ProjectServiceFileDescriptorSupplier())
              .addMethod(getGetProjectMethod())
              .addMethod(getListProjectsMethod())
              .addMethod(getCreateProjectMethod())
              .addMethod(getUpdateProjectMethod())
              .addMethod(getDeleteProjectMethod())
              .addMethod(getResolveDesignMethod())
              .build();
        }
      }
    }
    return result;
  }
}
