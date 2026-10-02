package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// maxExtendsDepth bounds how far an `extends` chain is followed. Cycles are caught separately; the
// bound refuses a legitimately deep chain too, per
// docsite/content/architecture/projects-and-designs.md#sharing-config-between-projects.
const maxExtendsDepth = 4

// ResolveExtends returns the project's config with everything it inherits already layered underneath.
// It is exported for a caller that reads one tier of the composed config directly, as the CLI reads a
// project's checklists, so the answer cannot differ from what a run composes.
//
// The chain is walked leaf-to-root and composed root-FIRST, so a project overrides what it inherits,
// the same direction as request over project over deployment default. Why inheritance is declared
// rather than ambient is on docsite/content/architecture/projects-and-designs.md#sharing-config-between-projects.
//
// A nil store is fine for a project that extends nothing. A project that declared an extends the
// store cannot resolve gets an error rather than a subset of its declared config.
func ResolveExtends(ctx context.Context, store ProjectStore, p *webapi.Project) (*webapi.AnalysisConfig, error) {
	cfg := p.GetConfig()
	if cfg.GetExtends() == "" {
		return cfg, nil
	}
	// chain is leaf-first; seen guards the cycle, keyed on the resource name the store answers by.
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
	out := &webapi.AnalysisConfig{}
	for i := len(chain) - 1; i >= 0; i-- {
		out = mergeConfig(out, chain[i])
	}
	// The convention is a VALUE and layers by replacement, as a request-supplied one does, because two
	// naming vocabularies cannot both be in effect. The nearest declaration wins.
	for i := len(chain) - 1; i >= 0; i-- {
		if c := chain[i].GetConventions(); c != nil {
			out.Conventions = c
		}
	}
	// Clear extends so nothing re-reading the composed config walks the chain a second time.
	out.Extends = ""
	return out, nil
}
