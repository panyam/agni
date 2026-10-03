// Package server is the Connect translation layer over the transport-neutral service
// implementations (CONSTRAINTS C13), with one adapter per service and each method a pure
// unwrap/call/wrap plus the sentinel-to-code mapping in toConnectErr. No business logic lives
// here, so another transport (grpc-gateway, a real gRPC server) would be a sibling package wrapping
// the same service/ implementations.
package server

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

// toConnectErr maps the service tier's error sentinels to Connect codes, for every adapter. An
// unclassified error is treated as an invalid argument, the service tier's documented default
// (load/parse failures).
func toConnectErr(err error) error {
	switch {
	case errors.Is(err, service.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, service.ErrNativeNoTool):
		return connect.NewError(connect.CodeUnimplemented, err)
	case errors.Is(err, service.ErrNativeNotEnabled):
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%w; start agni serve with --enable-native", err))
	case errors.Is(err, service.ErrNativeNotFound):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, service.ErrReviewStoreNotConfigured):
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%w; start agni serve with --review-store <dir>", err))
	case errors.Is(err, service.ErrUnavailable):
		return connect.NewError(connect.CodeUnavailable, err)
	case errors.Is(err, service.ErrInternal):
		return connect.NewError(connect.CodeInternal, err)
	// A request whose caller went away, or whose deadline passed, stopped rather than failed, and a
	// client retrying on INVALID_ARGUMENT would never succeed (agni issues 792, 795).
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, err)
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, err)
	case errors.Is(err, service.ErrResourceExhausted):
		return connect.NewError(connect.CodeResourceExhausted, err)
	default: // ErrInvalidPath, ErrInvalidArgument, and anything unclassified
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
}

// Workspace adapts service.WorkspaceService to the generated Connect handler interface. The
// embedded Unimplemented handler keeps newly generated RPCs returning CodeUnimplemented until
// wired, so adding a proto method never breaks the build.
type Workspace struct {
	webapiconnect.UnimplementedWorkspaceServiceHandler
	svc *service.WorkspaceService
}

// NewWorkspace wraps svc for Connect.
func NewWorkspace(svc *service.WorkspaceService) *Workspace { return &Workspace{svc: svc} }

func (a *Workspace) ListMounts(ctx context.Context, req *connect.Request[webapi.ListMountsRequest]) (*connect.Response[webapi.ListMountsResponse], error) {
	resp, err := a.svc.ListMounts(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Workspace) ListDir(ctx context.Context, req *connect.Request[webapi.ListDirRequest]) (*connect.Response[webapi.ListDirResponse], error) {
	resp, err := a.svc.ListDir(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Workspace) ListDesignFiles(ctx context.Context, req *connect.Request[webapi.ListDesignFilesRequest]) (*connect.Response[webapi.ListDesignFilesResponse], error) {
	resp, err := a.svc.ListDesignFiles(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

// Design adapts service.DesignService to the generated Connect handler interface.
type Design struct {
	webapiconnect.UnimplementedDesignServiceHandler
	svc *service.DesignService
}

// NewDesign wraps svc for Connect.
func NewDesign(svc *service.DesignService) *Design { return &Design{svc: svc} }

func (a *Design) GetDesign(ctx context.Context, req *connect.Request[webapi.GetDesignRequest]) (*connect.Response[webapi.GetDesignResponse], error) {
	resp, err := a.svc.GetDesign(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Design) GetSheet(ctx context.Context, req *connect.Request[webapi.GetSheetRequest]) (*connect.Response[webapi.GetSheetResponse], error) {
	resp, err := a.svc.GetSheet(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Design) HighlightSheet(ctx context.Context, req *connect.Request[webapi.HighlightSheetRequest]) (*connect.Response[webapi.HighlightSheetResponse], error) {
	resp, err := a.svc.HighlightSheet(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Design) GetLayoutReport(ctx context.Context, req *connect.Request[webapi.GetLayoutReportRequest]) (*connect.Response[webapi.GetLayoutReportResponse], error) {
	resp, err := a.svc.GetLayoutReport(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Design) TraceDesign(ctx context.Context, req *connect.Request[webapi.TraceDesignRequest]) (*connect.Response[webapi.TraceDesignResponse], error) {
	resp, err := a.svc.TraceDesign(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

// Check adapts service.CheckService to the generated Connect handler interface.
type Check struct {
	webapiconnect.UnimplementedCheckServiceHandler
	svc *service.CheckService
}

// NewCheck wraps svc for Connect.
func NewCheck(svc *service.CheckService) *Check { return &Check{svc: svc} }

func (a *Check) ListRules(ctx context.Context, req *connect.Request[webapi.ListRulesRequest]) (*connect.Response[webapi.ListRulesResponse], error) {
	resp, err := a.svc.ListRules(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Check) CheckDesign(ctx context.Context, req *connect.Request[webapi.CheckDesignRequest]) (*connect.Response[webapi.CheckDesignResponse], error) {
	resp, err := a.svc.CheckDesign(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Check) GetExpectations(ctx context.Context, req *connect.Request[webapi.GetExpectationsRequest]) (*connect.Response[webapi.GetExpectationsResponse], error) {
	resp, err := a.svc.GetExpectations(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Check) GetCheckReport(ctx context.Context, req *connect.Request[webapi.GetCheckReportRequest]) (*connect.Response[webapi.GetCheckReportResponse], error) {
	resp, err := a.svc.GetCheckReport(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Check) GetInterfaceCoverage(ctx context.Context, req *connect.Request[webapi.GetInterfaceCoverageRequest]) (*connect.Response[webapi.GetInterfaceCoverageResponse], error) {
	resp, err := a.svc.GetInterfaceCoverage(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Check) GetComponentParams(ctx context.Context, req *connect.Request[webapi.GetComponentParamsRequest]) (*connect.Response[webapi.GetComponentParamsResponse], error) {
	resp, err := a.svc.GetComponentParams(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

// Diff adapts service.DiffService to the generated Connect handler interface.
type Diff struct {
	webapiconnect.UnimplementedDiffServiceHandler
	svc *service.DiffService
}

// NewDiff wraps svc for Connect.
func NewDiff(svc *service.DiffService) *Diff { return &Diff{svc: svc} }

func (a *Diff) DiffDesigns(ctx context.Context, req *connect.Request[webapi.DiffDesignsRequest]) (*connect.Response[webapi.DiffDesignsResponse], error) {
	resp, err := a.svc.DiffDesigns(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

// Table adapts service.TableService to the generated Connect handler interface.
type Table struct {
	webapiconnect.UnimplementedTableServiceHandler
	svc service.TableService
}

// NewTable wraps the projection service for Connect. It holds no state, so it takes none.
func NewTable() *Table { return &Table{} }

func (a *Table) Tabulate(ctx context.Context, req *connect.Request[webapi.TabulateRequest]) (*connect.Response[webapi.TabulateResponse], error) {
	resp, err := a.svc.Tabulate(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

// Query adapts service.QueryService to the generated Connect handler interface.
type Query struct {
	webapiconnect.UnimplementedQueryServiceHandler
	svc *service.QueryService
}

// NewQuery wraps svc for Connect.
func NewQuery(svc *service.QueryService) *Query { return &Query{svc: svc} }

func (a *Query) RunQuery(ctx context.Context, req *connect.Request[webapi.RunQueryRequest]) (*connect.Response[webapi.RunQueryResponse], error) {
	resp, err := a.svc.RunQuery(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Query) RunQueries(ctx context.Context, req *connect.Request[webapi.RunQueriesRequest]) (*connect.Response[webapi.RunQueriesResponse], error) {
	resp, err := a.svc.RunQueries(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Query) ListRelations(ctx context.Context, req *connect.Request[webapi.ListRelationsRequest]) (*connect.Response[webapi.ListRelationsResponse], error) {
	resp, err := a.svc.ListRelations(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Check) GetNamingConvention(ctx context.Context, req *connect.Request[webapi.GetNamingConventionRequest]) (*connect.Response[webapi.GetNamingConventionResponse], error) {
	resp, err := a.svc.GetNamingConvention(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

// Review adapts service.ReviewService to the generated Connect handler interface.
type Review struct {
	webapiconnect.UnimplementedReviewServiceHandler
	svc *service.ReviewService
}

// NewReview wraps svc for Connect.
func NewReview(svc *service.ReviewService) *Review { return &Review{svc: svc} }

func (a *Review) CreateReview(ctx context.Context, req *connect.Request[webapi.CreateReviewRequest]) (*connect.Response[webapi.Review], error) {
	resp, err := a.svc.CreateReview(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Review) GetReview(ctx context.Context, req *connect.Request[webapi.GetReviewRequest]) (*connect.Response[webapi.Review], error) {
	resp, err := a.svc.GetReview(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Review) ListReviews(ctx context.Context, req *connect.Request[webapi.ListReviewsRequest]) (*connect.Response[webapi.ListReviewsResponse], error) {
	resp, err := a.svc.ListReviews(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Review) DeleteReview(ctx context.Context, req *connect.Request[webapi.DeleteReviewRequest]) (*connect.Response[emptypb.Empty], error) {
	resp, err := a.svc.DeleteReview(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Review) GetReviewManifest(ctx context.Context, req *connect.Request[webapi.GetReviewManifestRequest]) (*connect.Response[webapi.GetReviewManifestResponse], error) {
	resp, err := a.svc.GetReviewManifest(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

// Project adapts service.ProjectService to the generated Connect handler interface.
type Project struct {
	webapiconnect.UnimplementedProjectServiceHandler
	svc *service.ProjectService
}

// NewProject wraps svc for Connect.
func NewProject(svc *service.ProjectService) *Project { return &Project{svc: svc} }

func (a *Project) GetProject(ctx context.Context, req *connect.Request[webapi.GetProjectRequest]) (*connect.Response[webapi.Project], error) {
	resp, err := a.svc.GetProject(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Project) ListProjects(ctx context.Context, req *connect.Request[webapi.ListProjectsRequest]) (*connect.Response[webapi.ListProjectsResponse], error) {
	resp, err := a.svc.ListProjects(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Project) GetDesign(ctx context.Context, req *connect.Request[webapi.GetProjectDesignRequest]) (*connect.Response[webapi.Design], error) {
	resp, err := a.svc.GetDesign(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Project) ListDesigns(ctx context.Context, req *connect.Request[webapi.ListProjectDesignsRequest]) (*connect.Response[webapi.ListProjectDesignsResponse], error) {
	resp, err := a.svc.ListDesigns(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Project) ResolveDesign(ctx context.Context, req *connect.Request[webapi.ResolveDesignRequest]) (*connect.Response[webapi.ResolveDesignResponse], error) {
	resp, err := a.svc.ResolveDesign(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}
