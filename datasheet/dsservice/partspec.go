package dsservice

import (
	"context"
	"errors"

	"github.com/panyam/agni/core/param"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// ErrNoCorpus is returned by PartSpecService on a server started without a published corpus
// (--corpus). A transport maps it to FailedPrecondition, since the operator must name one.
var ErrNoCorpus = errors.New("no published corpus is served")

// ErrCorpusNotReady is a corpus that exists but cannot be served as it stands: it has no index, or the
// index no longer matches its files. The store wraps its own cause in it, so the message says what to
// run, and a transport maps it to FailedPrecondition.
var ErrCorpusNotReady = errors.New("the published corpus cannot be served")

// PartSpecService is the read side of the published corpus, the contract's PartSpecService (agni
// issue 749). It is how `agni serve --params-url` reaches the specs this service publishes. It serves
// published specs only, through store, and never a workbench draft.
type PartSpecService struct {
	store param.Fetcher
}

// NewPartSpecService returns the service over store, which may be nil for a server that publishes no
// corpus; every call then reports ErrNoCorpus.
func NewPartSpecService(store param.Fetcher) *PartSpecService {
	return &PartSpecService{store: store}
}

// BatchGetPartSpecs returns the current spec for each requested part number that has one.
func (s *PartSpecService) BatchGetPartSpecs(ctx context.Context, req *parampb.BatchGetPartSpecsRequest) (*parampb.BatchGetPartSpecsResponse, error) {
	if s.store == nil {
		return nil, ErrNoCorpus
	}
	specs, gen, err := s.store.BatchGet(ctx, req.GetMpns())
	if err != nil {
		return nil, err
	}
	return &parampb.BatchGetPartSpecsResponse{Specs: specs, Generation: gen}, nil
}

// GetGeneration returns the corpus generation.
func (s *PartSpecService) GetGeneration(ctx context.Context, _ *parampb.GetGenerationRequest) (*parampb.GetGenerationResponse, error) {
	if s.store == nil {
		return nil, ErrNoCorpus
	}
	gen, err := s.store.Generation(ctx)
	if err != nil {
		return nil, err
	}
	return &parampb.GetGenerationResponse{Generation: gen}, nil
}
