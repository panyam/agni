// Package dsservice is the datasheet producer's service: the extraction workbench's doc-IR, its
// shared PartSpec draft, the per-author annotation overlays, and the folder tree it browses. It is
// transport-neutral over injected ports, the shape the engine's service package has (C13), and it
// lives in the datasheet module so the engine never imports it (C34, agni issue 744).
package dsservice

import (
	"context"
	"errors"
	"fmt"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/param"
	docpb "github.com/panyam/agni/datasheet/gen/go/agni/v1/doc"
	dsapi "github.com/panyam/agni/datasheet/gen/go/agni/v1/dsapi"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// ErrConflict is the optimistic-concurrency failure, a SavePartSpec whose base_version no longer
// matches the on-disk version because another writer got there first. A transport maps it to a code
// the client treats as "refetch and retry" (Connect Aborted), distinct from a bad request.
var ErrConflict = errors.New("version conflict")

// ErrExtractNotEnabled is returned when ExtractDocIR is called on a server started without a
// configured doc-IR producer (--pdf2doc). A transport maps it to FailedPrecondition (the operator
// must enable extraction), distinct from a bad request.
var ErrExtractNotEnabled = errors.New("doc-IR extraction not enabled")

// DocLoader materializes a datasheet's doc-IR. The URI names the source document (the PDF the
// browser renders), and the adapter resolves and parses the datasheet's sibling doc-IR. The
// os-backed adapter (agnids) owns the sibling-file convention. A datasheet with no derived doc-IR
// yet returns (nil, nil), a normal state that GetDocument reports as extracted=false. An unknown
// mount or a containment violation is returned already classified (service.ErrNotFound / service.ErrInvalidPath),
// and a present-but-unparseable doc-IR is any other error, classified as invalid.
type DocLoader interface {
	Document(ctx context.Context, uri artifact.URI) (*docpb.Document, error)
}

// PartSpecStore persists and loads a datasheet's shared PartSpec (the manual backend's output).
// The os-backed adapter (agnids) writes it as a sibling file in the mount. Get returns
// (nil, "", false, nil) when nothing is saved yet. Save is compare-and-swap, so baseVersion must
// equal the current on-disk version (empty asserts absence) or it returns ErrConflict. The
// read/compare/write is atomic per path. version is an opaque content token.
type PartSpecStore interface {
	Get(ctx context.Context, uri artifact.URI) (spec *parampb.PartSpec, version string, found bool, err error)
	Save(ctx context.Context, uri artifact.URI, spec *parampb.PartSpec, baseVersion string) (newVersion string, err error)
}

// DocExtractor runs the configured doc-IR producer (pdf2doc/docling) over a datasheet, writing the
// sibling doc-IR and returning it. The os-backed adapter (agnids) shells out to the configured
// command. Available reports whether a producer is configured, so the service can tell the client
// whether to offer extraction. Extract returns the produced Document, or an error (a producer run or
// parse failure, or a bad URI).
type DocExtractor interface {
	Available() bool
	Extract(ctx context.Context, uri artifact.URI) (*docpb.Document, error)
}

// AnnotationStore persists and loads a datasheet's per-author region-annotation overlays
// (WS13-011). The os-backed adapter (agnids) writes one file per author in the mount. Unlike
// PartSpecStore there is NO compare-and-swap, because each author owns their own file, so Save
// overwrites just that author's overlay and Get UNIONS every author's overlay for the datasheet.
// Get returns an empty slice (not an error) when nobody has annotated yet. author is a
// client-supplied coordination namespace, not an authenticated identity.
type AnnotationStore interface {
	Get(ctx context.Context, uri artifact.URI) ([]*dsapi.AnnotationSet, error)
	Save(ctx context.Context, uri artifact.URI, author string, set *dsapi.AnnotationSet) error
}

// DatasheetService serves a datasheet's doc-IR and its saved PartSpec to the extraction workbench
// (the /datasheets page, WS13-006) over injected ports (CONSTRAINTS C13). It is the document
// analogue of DesignService, plus the manual backend's read/write side for the shared PartSpec.
// It performs no file I/O and knows no transport. Directory listing goes through the engine's
// WorkspaceService over the same Workspace port, so the workbench's folder tree answers exactly as
// the viewer's does, and per-user workbench UI state stays in the client (localStorage), never here.
type DatasheetService struct {
	loader      DocLoader
	store       PartSpecStore
	extractor   DocExtractor
	annotations AnnotationStore
	workspace   *service.WorkspaceService
}

// NewDatasheetService returns a DatasheetService backed by the given doc-IR loader, PartSpec store,
// doc-IR extractor, per-author annotation store, and folder listing.
func NewDatasheetService(loader DocLoader, store PartSpecStore, extractor DocExtractor, annotations AnnotationStore, workspace service.Workspace) *DatasheetService {
	return &DatasheetService{loader: loader, store: store, extractor: extractor, annotations: annotations, workspace: service.NewWorkspaceService(workspace)}
}

// ListMounts answers as the engine's WorkspaceService.ListMounts does, over this service's mounts.
func (s *DatasheetService) ListMounts(ctx context.Context, req *webapi.ListMountsRequest) (*webapi.ListMountsResponse, error) {
	return s.workspace.ListMounts(ctx, req)
}

// ListDir answers as the engine's WorkspaceService.ListDir does, over this service's mounts.
func (s *DatasheetService) ListDir(ctx context.Context, req *webapi.ListDirRequest) (*webapi.ListDirResponse, error) {
	return s.workspace.ListDir(ctx, req)
}

// GetDocument returns the doc-IR for the datasheet at the request's URI. A datasheet with no
// derived doc-IR yet yields extracted=false and no document, and the workbench then shows the PDF
// with an empty region overlay. A load or parse failure is classified as an invalid argument, while
// an unknown mount or containment violation keeps its loader classification.
func (s *DatasheetService) GetDocument(ctx context.Context, req *dsapi.GetDocumentRequest) (*dsapi.GetDocumentResponse, error) {
	u, err := service.ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	d, err := s.loader.Document(ctx, u)
	if err != nil {
		return nil, service.ClassifyLoadErr(err)
	}
	if d == nil {
		return &dsapi.GetDocumentResponse{Extracted: false, ExtractAvailable: s.extractor.Available()}, nil
	}
	return &dsapi.GetDocumentResponse{Extracted: true, Document: d, ExtractAvailable: s.extractor.Available()}, nil
}

// ExtractDocIR runs the configured doc-IR producer over the datasheet and returns the produced
// doc-IR (the "first pass" the workbench then shows for review). A server with no producer
// configured rejects it as ErrExtractNotEnabled (FailedPrecondition). A producer run or parse
// failure is a server-side error (Internal), and a bad URI keeps its classification.
func (s *DatasheetService) ExtractDocIR(ctx context.Context, req *dsapi.ExtractDocIRRequest) (*dsapi.ExtractDocIRResponse, error) {
	u, err := service.ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	if !s.extractor.Available() {
		return nil, ErrExtractNotEnabled
	}
	d, err := s.extractor.Extract(ctx, u)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) || errors.Is(err, service.ErrInvalidPath) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: doc-IR extraction failed: %s", service.ErrInternal, err)
	}
	return &dsapi.ExtractDocIRResponse{Document: d}, nil
}

// GetPartSpec loads the datasheet's saved PartSpec and its version token. Absence is found=false
// with an empty version (a normal first-open state), not an error.
func (s *DatasheetService) GetPartSpec(ctx context.Context, req *dsapi.GetPartSpecRequest) (*dsapi.GetPartSpecResponse, error) {
	u, err := service.ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	spec, version, found, err := s.store.Get(ctx, u)
	if err != nil {
		return nil, service.ClassifyLoadErr(err)
	}
	return &dsapi.GetPartSpecResponse{Found: found, Spec: spec, Version: version}, nil
}

// SavePartSpec persists the PartSpec with optimistic concurrency. A version mismatch surfaces as
// ErrConflict (mapped to Aborted so the client refetches); an absent spec is an invalid argument.
func (s *DatasheetService) SavePartSpec(ctx context.Context, req *dsapi.SavePartSpecRequest) (*dsapi.SavePartSpecResponse, error) {
	u, err := service.ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	if req.GetSpec() == nil {
		return nil, fmt.Errorf("%w: SavePartSpec requires a spec", service.ErrInvalidArgument)
	}
	// NO VALIDATION HERE, DELIBERATELY. Saving records what the author has, and whether it is any
	// good is reported as status after the write. Rejecting an invalid save would leave a document
	// its author cannot save and cannot fix through the UI.
	//
	// This sibling is <stem>.partspec.json and param.LoadSet reads *.textproto only, so an
	// incoherent draft cannot reach the corpus by sitting on disk. param.Validate belongs on the
	// separate step that promotes a spec into a seeded corpus.
	version, err := s.store.Save(ctx, u, req.GetSpec(), req.GetBaseVersion())
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return nil, err // stays ErrConflict (Aborted), not invalid-argument
		}
		return nil, service.ClassifyLoadErr(err)
	}
	// Judged AFTER the write and reported rather than enforced. The editor (web/src/transcribe.tsx)
	// renders these rather than keeping its own copy of the rules.
	return &dsapi.SavePartSpecResponse{Version: version, Problems: validationProblems(req.GetSpec())}, nil
}

// validationProblems renders param's classified findings onto the wire type. The mapping is total,
// so a kind this does not recognize travels as UNSPECIFIED and still shows its message rather than
// vanishing from the editor.
func validationProblems(spec *parampb.PartSpec) []*dsapi.ValidationProblem {
	found := param.Problems(spec)
	if len(found) == 0 {
		return nil
	}
	out := make([]*dsapi.ValidationProblem, 0, len(found))
	for _, p := range found {
		kind := dsapi.ValidationProblem_KIND_UNSPECIFIED
		switch p.Kind {
		case param.ProblemStructural:
			kind = dsapi.ValidationProblem_KIND_STRUCTURAL
		case param.ProblemCompleteness:
			kind = dsapi.ValidationProblem_KIND_COMPLETENESS
		}
		out = append(out, &dsapi.ValidationProblem{Kind: kind, Message: p.Message})
	}
	return out
}

// GetAnnotations returns the region-annotation overlay for a datasheet as the union of every
// author's overlay. An empty union (nobody has annotated) is a normal state, not an error.
func (s *DatasheetService) GetAnnotations(ctx context.Context, req *dsapi.GetAnnotationsRequest) (*dsapi.GetAnnotationsResponse, error) {
	u, err := service.ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	sets, err := s.annotations.Get(ctx, u)
	if err != nil {
		return nil, service.ClassifyLoadErr(err)
	}
	return &dsapi.GetAnnotationsResponse{Sets: sets}, nil
}

// SaveAnnotations persists one author's overlay, replacing that author's prior overlay for the
// datasheet. There is no optimistic concurrency: each author owns their own file. An absent set or
// an empty author is an invalid argument (the author names the file and cannot be inferred).
func (s *DatasheetService) SaveAnnotations(ctx context.Context, req *dsapi.SaveAnnotationsRequest) (*dsapi.SaveAnnotationsResponse, error) {
	u, err := service.ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	set := req.GetSet()
	if set == nil {
		return nil, fmt.Errorf("%w: SaveAnnotations requires a set", service.ErrInvalidArgument)
	}
	if set.GetAuthor() == "" {
		return nil, fmt.Errorf("%w: SaveAnnotations requires a non-empty author", service.ErrInvalidArgument)
	}
	if err := s.annotations.Save(ctx, u, set.GetAuthor(), set); err != nil {
		return nil, service.ClassifyLoadErr(err)
	}
	return &dsapi.SaveAnnotationsResponse{}, nil
}
