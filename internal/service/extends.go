package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// maxExtendsDepth bounds how far an `extends` chain is followed.
//
// It is a bound rather than a trust in cycle detection alone. A cycle is caught below and named, but a
// legitimately deep chain is also a smell worth failing on: config a reader has to open six files to
// understand is config nobody will reason about correctly, and a project that needs that much sharing
// wants a flatter arrangement rather than a longer walk.
const maxExtendsDepth = 4

// resolveExtends returns the project's config with everything it inherits already layered underneath.
//
// The chain is walked leaf-to-root and then composed root-FIRST, so a project overrides what it
// inherits rather than the other way round. That is the direction every other layer here runs: a
// request overrides a project, a project overrides the deployment default, and a project overrides
// what it extends.
//
// Inheritance is DECLARED, which is what makes it safe. An ambient global applying unless overridden
// is the bug per-design config fixed — one team's profiles reaching every board on a server. An
// extends is written in the descriptor, scoped to the project that wrote it, and reaches no design
// that did not ask for it.
//
// A store that cannot answer is NOT fatal on its own: a project that extends nothing walks zero
// steps, so a nil store only matters to a project that declared one, and that project gets an error
// naming what it could not reach. Resolving to a subset of the declared config would run a design
// against config its operator believes is in effect.
func resolveExtends(ctx context.Context, store ProjectStore, p *webapi.Project) (*webapi.AnalysisConfig, error) {
	cfg := p.GetConfig()
	if cfg.GetExtends() == "" {
		return cfg, nil
	}
	// chain is leaf-first; seen guards the cycle. Both are keyed on the resource name, which is the
	// identity the store answers by.
	chain := []*webapi.AnalysisConfig{cfg}
	seen := map[string]bool{p.GetName(): true}
	order := []string{p.GetName()}
	next := cfg.GetExtends()
	for depth := 0; next != ""; depth++ {
		if depth >= maxExtendsDepth {
			return nil, fmt.Errorf("%w: %s extends more than %d levels deep (%s); flatten the chain",
				ErrInvalidArgument, p.GetName(), maxExtendsDepth, strings.Join(append(order, next), " -> "))
		}
		if seen[next] {
			return nil, fmt.Errorf("%w: extends cycle: %s", ErrInvalidArgument, strings.Join(append(order, next), " -> "))
		}
		if store == nil {
			return nil, fmt.Errorf("%w: %s extends %s, which this deployment cannot resolve (no project store wired)",
				ErrInvalidArgument, p.GetName(), next)
		}
		parent, err := store.Project(ctx, next)
		if err != nil {
			return nil, fmt.Errorf("%w: %s extends %s: %s", ErrInvalidArgument, p.GetName(), next, err)
		}
		if parent == nil {
			return nil, fmt.Errorf("%w: %s extends %s, which does not exist", ErrInvalidArgument, p.GetName(), next)
		}
		seen[next] = true
		order = append(order, next)
		chain = append(chain, parent.GetConfig())
		next = parent.GetConfig().GetExtends()
	}
	// Compose root-first so the leaf wins.
	out := &webapi.AnalysisConfig{}
	for i := len(chain) - 1; i >= 0; i-- {
		out = mergeConfig(out, chain[i])
	}
	// The convention is a VALUE and layers by replacement, on the same rule a request-supplied one
	// follows: two naming vocabularies cannot both be in effect, so the nearest declaration wins.
	for i := len(chain) - 1; i >= 0; i-- {
		if c := chain[i].GetConventions(); c != nil {
			out.Conventions = c
			out.ConventionsUri = chain[i].GetConventionsUri()
		}
	}
	// extends is not carried onto the result: it has been followed, and leaving it set would invite a
	// second walk by anything that re-read the composed config.
	out.Extends = ""
	return out, nil
}
