package param

import (
	"context"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// A citation names its corpus with one of these, so a verdict says whose transcription it relies on.
const (
	// CorpusProject is the design's own project params/, which the project owns.
	CorpusProject = "project"
	// CorpusShared is any corpus the design's project does not own, such as a --params directory or a
	// --params-url service.
	CorpusShared = "shared"
)

// Layer is one corpus in a Layered provider, with the name a citation reports it by.
type Layer struct {
	Name     string
	Provider ParamProvider
}

// CorpusNamer is a provider that can say which of its corpora a spec it returned came from.
type CorpusNamer interface {
	CorpusOf(spec *parampb.PartSpec) string
}

// Layered is a provider that asks each layer in order and answers with the first that seeds the MPN
// (agni issue 749). A project's own params/ is layered over a shared corpus this way. The project
// decides every part it seeds and the shared corpus answers for the rest. Without the layering, a
// project that seeds two parts would lose the datasheet tier for every other part.
//
// A layer with a nil provider is skipped. Layered prefetches through every layer that needs it, so a
// remote shared corpus under a local project corpus still fails loudly when it cannot be reached.
type Layered []Layer

// Lookup returns the first layer's spec for the MPN, or nil when no layer seeds it.
func (l Layered) Lookup(mpn string) *parampb.PartSpec {
	for _, layer := range l {
		if layer.Provider == nil {
			continue
		}
		if s := layer.Provider.Lookup(mpn); s != nil {
			return s
		}
	}
	return nil
}

// Prefetch implements Prefetcher by prefetching every layer that is one.
func (l Layered) Prefetch(ctx context.Context, mpns []string) error {
	for _, layer := range l {
		if p, ok := layer.Provider.(Prefetcher); ok {
			if err := p.Prefetch(ctx, mpns); err != nil {
				return err
			}
		}
	}
	return nil
}

// CorpusOf names the layer a spec came from. That is the first layer whose own Lookup of the spec's
// MPN returns that same spec. It compares by identity rather than by MPN, because a lower layer can
// seed the same MPN, and a citation relies only on the spec actually returned. It returns "" when no
// layer returned the spec.
func (l Layered) CorpusOf(spec *parampb.PartSpec) string {
	if spec == nil {
		return ""
	}
	for _, layer := range l {
		if layer.Provider != nil && layer.Provider.Lookup(spec.GetMpn()) == spec {
			return layer.Name
		}
	}
	return ""
}

var (
	_ Prefetcher  = Layered(nil)
	_ CorpusNamer = Layered(nil)
)

// Fetches reports whether reading p goes beyond the process. It is true when p, or any layer of a
// Layered, fetches over the network, meaning it is a Prefetcher other than Layered itself. A local
// set, a nil provider, and a Layered of only those do not fetch. Layered implements Prefetch to
// forward to its layers, so asking whether a provider is a Prefetcher says nothing about whether it
// fetches. A cache deciding whether an answer can outlive the request asks Fetches instead (agni
// issue 895).
func Fetches(p ParamProvider) bool {
	switch v := p.(type) {
	case nil:
		return false
	case Layered:
		for _, layer := range v {
			if Fetches(layer.Provider) {
				return true
			}
		}
		return false
	case Prefetcher:
		return true
	}
	return false
}

