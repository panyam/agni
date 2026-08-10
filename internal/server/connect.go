// Package server is the Connect translation layer over the transport-neutral service
// implementations (CONSTRAINTS C13): one adapter per service, each method a pure
// unwrap/call/wrap plus the sentinel-to-code mapping in toConnectErr. No business logic lives
// here — a later transport (grpc-gateway, a real gRPC server) is a sibling of this package,
// wrapping the same internal/service implementations.
package server

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

// toConnectErr maps the service tier's error sentinels to Connect codes — the single
// translation table for every adapter. An unclassified error is treated as an invalid
// argument, the service tier's documented default (load/parse failures).
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
	case errors.Is(err, service.ErrConflict):
		return connect.NewError(connect.CodeAborted, err)
	case errors.Is(err, service.ErrExtractNotEnabled):
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%w; start agni serve with --pdf2doc", err))
	case errors.Is(err, service.ErrReviewStoreNotConfigured):
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%w; start agni serve with --review-store <dir>", err))
	case errors.Is(err, service.ErrInternal):
		return connect.NewError(connect.CodeInternal, err)
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

// Datasheet adapts service.DatasheetService to the generated Connect handler interface (the
// extraction workbench's read side, WS13-006).
type Datasheet struct {
	webapiconnect.UnimplementedDatasheetServiceHandler
	svc *service.DatasheetService
}

// NewDatasheet wraps svc for Connect.
func NewDatasheet(svc *service.DatasheetService) *Datasheet { return &Datasheet{svc: svc} }

func (a *Datasheet) GetDocument(ctx context.Context, req *connect.Request[webapi.GetDocumentRequest]) (*connect.Response[webapi.GetDocumentResponse], error) {
	resp, err := a.svc.GetDocument(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Datasheet) GetPartSpec(ctx context.Context, req *connect.Request[webapi.GetPartSpecRequest]) (*connect.Response[webapi.GetPartSpecResponse], error) {
	resp, err := a.svc.GetPartSpec(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Datasheet) SavePartSpec(ctx context.Context, req *connect.Request[webapi.SavePartSpecRequest]) (*connect.Response[webapi.SavePartSpecResponse], error) {
	resp, err := a.svc.SavePartSpec(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Datasheet) ExtractDocIR(ctx context.Context, req *connect.Request[webapi.ExtractDocIRRequest]) (*connect.Response[webapi.ExtractDocIRResponse], error) {
	resp, err := a.svc.ExtractDocIR(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Datasheet) GetAnnotations(ctx context.Context, req *connect.Request[webapi.GetAnnotationsRequest]) (*connect.Response[webapi.GetAnnotationsResponse], error) {
	resp, err := a.svc.GetAnnotations(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Datasheet) SaveAnnotations(ctx context.Context, req *connect.Request[webapi.SaveAnnotationsRequest]) (*connect.Response[webapi.SaveAnnotationsResponse], error) {
	resp, err := a.svc.SaveAnnotations(ctx, req.Msg)
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

func (a *Query) ListRelations(ctx context.Context, req *connect.Request[webapi.ListRelationsRequest]) (*connect.Response[webapi.ListRelationsResponse], error) {
	resp, err := a.svc.ListRelations(ctx, req.Msg)
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
