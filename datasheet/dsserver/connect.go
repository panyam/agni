// Package dsserver adapts the datasheet service to Connect, the transport agnids serves it over. It
// is the datasheet module's counterpart of the engine's internal/server, which this module cannot
// import (agni issue 744).
package dsserver

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/panyam/agni/datasheet/dsservice"
	dsapi "github.com/panyam/agni/gen/go/agni/v1/dsapi"
	"github.com/panyam/agni/gen/go/agni/v1/dsapi/dsapiconnect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// toConnectErr maps the datasheet service's errors to Connect codes. It covers only what this
// service returns: the engine's shared sentinels it reuses, and its own two.
func toConnectErr(err error) error {
	switch {
	case errors.Is(err, service.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, dsservice.ErrConflict):
		return connect.NewError(connect.CodeAborted, err)
	case errors.Is(err, dsservice.ErrExtractNotEnabled):
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%w; start agnids serve with --pdf2doc", err))
	case errors.Is(err, service.ErrInternal):
		return connect.NewError(connect.CodeInternal, err)
	default: // ErrInvalidPath, ErrInvalidArgument, and anything unclassified
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
}

// unary runs one rpc: it hands the message to the service, maps a failure to a Connect code with this
// package's own mapping, and wraps the answer.
func unary[Req, Resp any](ctx context.Context, req *connect.Request[Req], call func(context.Context, *Req) (*Resp, error)) (*connect.Response[Resp], error) {
	resp, err := call(ctx, req.Msg)
	if err != nil {
		return nil, toConnectErr(err)
	}
	return connect.NewResponse(resp), nil
}

// Datasheet adapts dsservice.DatasheetService to the generated Connect handler interface (the
// extraction workbench's read side, WS13-006).
type Datasheet struct {
	dsapiconnect.UnimplementedDatasheetServiceHandler
	svc *dsservice.DatasheetService
}

// NewDatasheet wraps svc for Connect.
func NewDatasheet(svc *dsservice.DatasheetService) *Datasheet { return &Datasheet{svc: svc} }

func (a *Datasheet) GetDocument(ctx context.Context, req *connect.Request[dsapi.GetDocumentRequest]) (*connect.Response[dsapi.GetDocumentResponse], error) {
	return unary(ctx, req, a.svc.GetDocument)
}

func (a *Datasheet) GetPartSpec(ctx context.Context, req *connect.Request[dsapi.GetPartSpecRequest]) (*connect.Response[dsapi.GetPartSpecResponse], error) {
	return unary(ctx, req, a.svc.GetPartSpec)
}

func (a *Datasheet) SavePartSpec(ctx context.Context, req *connect.Request[dsapi.SavePartSpecRequest]) (*connect.Response[dsapi.SavePartSpecResponse], error) {
	return unary(ctx, req, a.svc.SavePartSpec)
}

func (a *Datasheet) ExtractDocIR(ctx context.Context, req *connect.Request[dsapi.ExtractDocIRRequest]) (*connect.Response[dsapi.ExtractDocIRResponse], error) {
	return unary(ctx, req, a.svc.ExtractDocIR)
}

func (a *Datasheet) GetAnnotations(ctx context.Context, req *connect.Request[dsapi.GetAnnotationsRequest]) (*connect.Response[dsapi.GetAnnotationsResponse], error) {
	return unary(ctx, req, a.svc.GetAnnotations)
}

func (a *Datasheet) SaveAnnotations(ctx context.Context, req *connect.Request[dsapi.SaveAnnotationsRequest]) (*connect.Response[dsapi.SaveAnnotationsResponse], error) {
	return unary(ctx, req, a.svc.SaveAnnotations)
}

func (a *Datasheet) ListMounts(ctx context.Context, req *connect.Request[webapi.ListMountsRequest]) (*connect.Response[webapi.ListMountsResponse], error) {
	return unary(ctx, req, a.svc.ListMounts)
}

func (a *Datasheet) ListDir(ctx context.Context, req *connect.Request[webapi.ListDirRequest]) (*connect.Response[webapi.ListDirResponse], error) {
	return unary(ctx, req, a.svc.ListDir)
}
