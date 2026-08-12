package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/panyam/agni/internal/artifact"

	"github.com/panyam/agni/datasheet/param"
	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// ErrConflict is the optimistic-concurrency failure: a SavePartSpec whose base_version no longer
// matches the on-disk version (another writer got there first). A transport maps it to a code the
// client treats as "refetch and retry" (Connect Aborted), distinct from a bad request.
var ErrConflict = errors.New("version conflict")

// ErrExtractNotEnabled is returned when ExtractDocIR is called on a server started without a
// configured doc-IR producer (--pdf2doc). A transport maps it to FailedPrecondition (the operator
// must enable extraction), distinct from a bad request.
var ErrExtractNotEnabled = errors.New("doc-IR extraction not enabled")

// DocLoader materializes a datasheet's doc-IR from a (mount, path). path names the source
// document (the PDF the browser renders); the adapter resolves the datasheet's sibling doc-IR
// and parses it. The os-backed adapter (cmd/agni) owns all file I/O and the sibling-file
// convention (CONSTRAINTS C1/C13); the service package stays os-free. A datasheet with no
// derived doc-IR yet returns (nil, nil): "not yet extracted" is a normal state, not an error,
// and GetDocument reports it as extracted=false. An unknown mount or a containment violation is
// returned already classified (ErrNotFound / ErrInvalidPath); a present-but-unparseable doc-IR
// is any other error, classified as invalid.
type DocLoader interface {
	Document(ctx context.Context, uri artifact.URI) (*docpb.Document, error)
}

// PartSpecStore persists and loads a datasheet's shared PartSpec (the manual backend's output).
// The os-backed adapter (cmd/agni) writes it as a sibling file in the mount and owns all I/O
// (C1/C13). Get returns (nil, "", false, nil) when nothing is saved yet. Save is compare-and-swap:
// baseVersion must equal the current on-disk version (empty asserts absence), else it returns
// ErrConflict; the read/compare/write is atomic per path. version is an opaque content token.
type PartSpecStore interface {
	Get(ctx context.Context, uri artifact.URI) (spec *parampb.PartSpec, version string, found bool, err error)
	Save(ctx context.Context, uri artifact.URI, spec *parampb.PartSpec, baseVersion string) (newVersion string, err error)
}

// DocExtractor runs the configured doc-IR producer (pdf2doc/docling) over a datasheet, writing the
// sibling doc-IR and returning it. The os-backed adapter (cmd/agni) shells out to the configured
// command and owns all I/O (C1/C13). Available reports whether a producer is configured, so the
// service can tell the client whether to offer extraction; Extract returns the produced Document,
// or an error (a producer run/parse failure, or a bad mount/path).
type DocExtractor interface {
	Available() bool
	Extract(ctx context.Context, uri artifact.URI) (*docpb.Document, error)
}

// AnnotationStore persists and loads a datasheet's per-author region-annotation overlays
// (WS13-011). The os-backed adapter (cmd/agni) writes one file per author in the mount and owns
// all I/O (C1/C13). Unlike PartSpecStore there is NO compare-and-swap: each author owns their own
// file, so Save overwrites just that author's overlay and never contends, and Get UNIONS every
// author's overlay for the datasheet. Get returns an empty slice (not an error) when nobody has
// annotated yet. author is a client-supplied coordination namespace, not an authenticated identity.
type AnnotationStore interface {
	Get(ctx context.Context, uri artifact.URI) ([]*webapi.AnnotationSet, error)
	Save(ctx context.Context, uri artifact.URI, author string, set *webapi.AnnotationSet) error
}

// DatasheetService serves a datasheet's doc-IR and its saved PartSpec to the extraction workbench
// (the /datasheets page, WS13-006) over injected ports (CONSTRAINTS C13). It is the document
// analogue of DesignService, plus the manual backend's read/write side for the shared PartSpec.
// It performs no file I/O and knows no transport; directory listing is the shared WorkspaceService,
// and per-user workbench UI state stays in the client (localStorage), never here.
type DatasheetService struct {
	loader      DocLoader
	store       PartSpecStore
	extractor   DocExtractor
	annotations AnnotationStore
}

// NewDatasheetService returns a DatasheetService backed by the given doc-IR loader, PartSpec store,
// doc-IR extractor, and per-author annotation store.
func NewDatasheetService(loader DocLoader, store PartSpecStore, extractor DocExtractor, annotations AnnotationStore) *DatasheetService {
	return &DatasheetService{loader: loader, store: store, extractor: extractor, annotations: annotations}
}

// GetDocument returns the doc-IR for the datasheet at (mount, path). A datasheet with no derived
// doc-IR yet yields extracted=false and no document (the workbench then shows the PDF with an empty
// region overlay — silence never reads as coverage). A load or parse failure is classified for the
// transport (invalid argument), while an unknown mount or containment violation keeps its loader
// classification.
func (s *DatasheetService) GetDocument(ctx context.Context, req *webapi.GetDocumentRequest) (*webapi.GetDocumentResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	d, err := s.loader.Document(ctx, u)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	if d == nil {
		return &webapi.GetDocumentResponse{Extracted: false, ExtractAvailable: s.extractor.Available()}, nil
	}
	return &webapi.GetDocumentResponse{Extracted: true, Document: d, ExtractAvailable: s.extractor.Available()}, nil
}

// ExtractDocIR runs the configured doc-IR producer over the datasheet and returns the produced
// doc-IR (the "first pass" the workbench then shows for review). A server with no producer
// configured rejects it as ErrExtractNotEnabled (FailedPrecondition). A producer run or parse
// failure is a server-side error (Internal); a bad mount/path keeps its classification.
func (s *DatasheetService) ExtractDocIR(ctx context.Context, req *webapi.ExtractDocIRRequest) (*webapi.ExtractDocIRResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	if !s.extractor.Available() {
		return nil, ErrExtractNotEnabled
	}
	d, err := s.extractor.Extract(ctx, u)
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidPath) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: doc-IR extraction failed: %s", ErrInternal, err)
	}
	return &webapi.ExtractDocIRResponse{Document: d}, nil
}

// GetPartSpec loads the datasheet's saved PartSpec and its version token. Absence is found=false
// with an empty version (a normal first-open state), not an error.
func (s *DatasheetService) GetPartSpec(ctx context.Context, req *webapi.GetPartSpecRequest) (*webapi.GetPartSpecResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	spec, version, found, err := s.store.Get(ctx, u)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	return &webapi.GetPartSpecResponse{Found: found, Spec: spec, Version: version}, nil
}

// SavePartSpec persists the PartSpec with optimistic concurrency. A version mismatch surfaces as
// ErrConflict (mapped to Aborted so the client refetches); an absent spec is an invalid argument.
func (s *DatasheetService) SavePartSpec(ctx context.Context, req *webapi.SavePartSpecRequest) (*webapi.SavePartSpecResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	if req.GetSpec() == nil {
		return nil, fmt.Errorf("%w: SavePartSpec requires a spec", ErrInvalidArgument)
	}
	// STRUCTURAL coherence only, never param.Validate. A spec under transcription has no MPN yet and
	// grows a parameter at a time, all of which Validate rightly rejects for corpus purposes and
	// none of which should block saving progress. What ValidatePins covers cannot be a
	// not-yet-filled-in state: a duplicate pin id or a binding to a pin that does not exist is wrong
	// the moment it is written, and letting it reach disk breaks param.LoadSet for the whole corpus,
	// not just this file.
	if err := param.ValidatePins(req.GetSpec()); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}
	version, err := s.store.Save(ctx, u, req.GetSpec(), req.GetBaseVersion())
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return nil, err // keep it ErrConflict for the transport (Aborted), not invalid-argument
		}
		return nil, classifyLoadErr(err)
	}
	return &webapi.SavePartSpecResponse{Version: version}, nil
}

// GetAnnotations returns the region-annotation overlay for a datasheet as the union of every
// author's overlay. An empty union (nobody has annotated) is a normal state, not an error.
func (s *DatasheetService) GetAnnotations(ctx context.Context, req *webapi.GetAnnotationsRequest) (*webapi.GetAnnotationsResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	sets, err := s.annotations.Get(ctx, u)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	return &webapi.GetAnnotationsResponse{Sets: sets}, nil
}

// SaveAnnotations persists one author's overlay, replacing that author's prior overlay for the
// datasheet. There is no optimistic concurrency: each author owns their own file. An absent set or
// an empty author is an invalid argument (the author names the file and cannot be inferred).
func (s *DatasheetService) SaveAnnotations(ctx context.Context, req *webapi.SaveAnnotationsRequest) (*webapi.SaveAnnotationsResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	set := req.GetSet()
	if set == nil {
		return nil, fmt.Errorf("%w: SaveAnnotations requires a set", ErrInvalidArgument)
	}
	if set.GetAuthor() == "" {
		return nil, fmt.Errorf("%w: SaveAnnotations requires a non-empty author", ErrInvalidArgument)
	}
	if err := s.annotations.Save(ctx, u, set.GetAuthor(), set); err != nil {
		return nil, classifyLoadErr(err)
	}
	return &webapi.SaveAnnotationsResponse{}, nil
}
