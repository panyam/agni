package service

import (
	"context"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// ConventionsLoader is the file surface overlay composition needs: an operator naming-convention
// config, mount-scoped by the impl like every other loader read.
type ConventionsLoader interface {
	Conventions(ctx context.Context, mount, path string) (naming.Config, error)
}

// Overlay is one request's composed catalog configuration: the rule sources to splice onto the
// service's catalog, and the naming lexicon its design reads must be stamped with. It is the single
// place a config PATH becomes engine inputs, so every surface that accepts overlay config resolves it
// identically (WS3-102).
//
// The split matters. A convention config carries rules AND a lexicon, and they land in different
// places at different times: rules extend the catalog the rules run from, while the lexicon has to
// reach the READ, because net roles are resolved once at ingestion. A composer that only did the
// catalog half would compile the naming rules and still leave every OTHER rule blind to the project's
// rail names, which is the more damaging half of the bug.
type Overlay struct {
	Sources []check.RuleSource
	Lexicon *classify.Lexicon
}

// ComposeOverlay resolves an OverlayConfig into engine inputs. A nil or empty config yields a zero
// Overlay, which changes nothing — so a request that names no overlay behaves exactly as before.
//
// A named config that cannot be loaded or parsed is an ERROR, never a skip: an operator who asked for
// their conventions and silently got the built-ins would read the resulting clean report as a clean
// design.
func ComposeOverlay(ctx context.Context, loader ConventionsLoader, mount string, cfg *webapi.OverlayConfig) (Overlay, error) {
	var o Overlay
	path := cfg.GetConventionsPath()
	if path == "" {
		return o, nil
	}
	conv, err := loader.Conventions(ctx, mount, path)
	if err != nil {
		return Overlay{}, classifyLoadErr(err)
	}
	lex, err := naming.BuildLexicon(conv)
	if err != nil {
		return Overlay{}, err
	}
	o.Lexicon = lex
	// Convention RULES are optional: a config may carry only a lexicon, which is exactly the shape a
	// project uses to teach the engine its rail names without adding any naming rule of its own.
	if len(conv.Rules) > 0 {
		src, err := naming.Source(conv)
		if err != nil {
			return Overlay{}, err
		}
		o.Sources = append(o.Sources, src)
	}
	return o, nil
}

// Catalog splices this overlay's rule sources onto a base catalog, returning base unchanged when the
// overlay carries none. CatalogWith keeps the built-ins and any RegisterSource'd suites, so composing
// never silently drops the shipped rules.
func (o Overlay) Catalog(base *check.Catalog) *check.Catalog {
	if len(o.Sources) == 0 {
		return base
	}
	return check.CatalogWith(o.Sources...)
}

// ReadOptions is what the overlay contributes to each design READ: the naming lexicon, or nothing.
func (o Overlay) ReadOptions() []ReadOption {
	if o.Lexicon == nil {
		return nil
	}
	return []ReadOption{WithLexicon(o.Lexicon)}
}
