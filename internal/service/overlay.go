package service

import (
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// Overlay is one request's composed catalog configuration: the rule sources to splice onto the
// service's catalog, and the naming lexicon its design reads must be stamped with. It is the single
// place overlay config becomes engine inputs, so every surface that accepts it composes identically
// (WS3-102).
//
// The split matters. A convention carries rules AND a lexicon, and they land in different places at
// different times: rules extend the catalog the rules run from, while the lexicon has to reach the
// READ, because net roles are resolved once at ingestion. A composer that did only the catalog half
// would compile the naming rules and still leave every OTHER rule blind to the project's rail names,
// which is the more damaging half of the bug.
type Overlay struct {
	Sources []check.RuleSource
	Lexicon *classify.Lexicon
}

// ComposeOverlay resolves an OverlayConfig into engine inputs. A nil or empty config yields a zero
// Overlay, which changes nothing, so a request that names no overlay behaves exactly as before.
//
// It performs no I/O: the config arrives as a value, so composing is pure and a caller can compose the
// same inputs in a test, a CLI, or a browser without a filesystem. An invalid convention (a pattern
// that will not compile, an unknown component class) is an ERROR, never a skip — an operator who asked
// for their conventions and silently got the built-ins would read the resulting clean report as a
// clean design.
func ComposeOverlay(cfg *webapi.OverlayConfig) (Overlay, error) {
	var o Overlay
	conv := cfg.GetConventions()
	if conv == nil {
		return o, nil
	}
	c := ConventionFromProto(conv)
	lex, err := naming.BuildLexicon(c)
	if err != nil {
		return Overlay{}, err
	}
	o.Lexicon = lex
	// Convention RULES are optional: a config may carry only a lexicon, which is exactly the shape a
	// project uses to teach the engine its rail names without adding any naming rule of its own.
	if len(c.Rules) > 0 {
		src, err := naming.Source(c)
		if err != nil {
			return Overlay{}, err
		}
		o.Sources = append(o.Sources, src)
	}
	return o, nil
}

// Catalog splices this overlay's rule sources ONTO base, returning base unchanged when the overlay
// carries none.
//
// It extends base rather than rebuilding a catalog, and that is the whole point (WS3-107). The
// previous implementation called check.CatalogWith(o.Sources...), which keeps the built-ins and every
// RegisterSource'd suite — true, and exactly what made the bug invisible — but drops base. For a
// review, base is the catalog composed from --profile-path and --intent-path, so a convention that
// carried a single naming RULE silently disabled every interface profile and the whole design-intent
// tier for that run. Measured on one design, 19 items went pass -> needs-design-intent and 16 went
// pass -> not-automated, with nothing anywhere to say why.
//
// A composition error is an error, not a panic: the sources come from a REQUEST here, and a caller
// sending a convention whose name collides with an existing source should get a message, not a
// crashed process.
func (o Overlay) Catalog(base *check.Catalog) (*check.Catalog, error) {
	if len(o.Sources) == 0 {
		return base, nil
	}
	return base.With(o.Sources...)
}

// ReadOptions is what the overlay contributes to each design READ: the naming lexicon, or nothing.
func (o Overlay) ReadOptions() []ReadOption {
	if o.Lexicon == nil {
		return nil
	}
	return []ReadOption{WithLexicon(o.Lexicon)}
}

// ConventionFromProto converts the wire form to the engine's config value. It lives here, not in
// core/check/naming, because the core must not depend on the wire types; this package already sits
// between the two.
func ConventionFromProto(p *webapi.NamingConvention) naming.Config {
	c := naming.Config{Name: p.GetName()}
	if lx := p.GetLexicon(); lx != nil {
		c.Lexicon = &naming.Lexicon{
			Rail:      vocabFromProto(lx.GetRail()),
			Ground:    vocabFromProto(lx.GetGround()),
			Feedback:  vocabFromProto(lx.GetFeedback()),
			SupplyPin: vocabFromProto(lx.GetSupplyPin()),
		}
		if cls := lx.GetClass(); len(cls) > 0 {
			c.Lexicon.Class = map[string]naming.VocabConfig{}
			for name, v := range cls {
				c.Lexicon.Class[name] = vocabFromProto(v)
			}
		}
	}
	for _, r := range p.GetRules() {
		c.Rules = append(c.Rules, naming.RuleConfig{
			Name: r.GetName(), Severity: r.GetSeverity(), Why: r.GetWhy(),
			Allow: r.GetAllow(), Exempt: r.GetExempt(), MatchFull: r.GetMatchFull(),
		})
	}
	return c
}

// ConventionProto converts an engine config value to the wire form, for a caller that loaded one from
// its own source (the CLI reads a YAML file the user named) and now has to send it.
func ConventionProto(c naming.Config) *webapi.NamingConvention {
	p := &webapi.NamingConvention{Name: c.Name}
	if c.Lexicon != nil {
		p.Lexicon = &webapi.NamingLexicon{
			Rail:      vocabProto(c.Lexicon.Rail),
			Ground:    vocabProto(c.Lexicon.Ground),
			Feedback:  vocabProto(c.Lexicon.Feedback),
			SupplyPin: vocabProto(c.Lexicon.SupplyPin),
		}
		if len(c.Lexicon.Class) > 0 {
			p.Lexicon.Class = map[string]*webapi.VocabPatterns{}
			for name, v := range c.Lexicon.Class {
				p.Lexicon.Class[name] = vocabProto(v)
			}
		}
	}
	for _, r := range c.Rules {
		p.Rules = append(p.Rules, &webapi.NamingRule{
			Name: r.Name, Severity: r.Severity, Why: r.Why,
			Allow: r.Allow, Exempt: r.Exempt, MatchFull: r.MatchFull,
		})
	}
	return p
}

func vocabFromProto(v *webapi.VocabPatterns) naming.VocabConfig {
	return naming.VocabConfig{Patterns: v.GetPatterns(), Replace: v.GetReplace()}
}

func vocabProto(v naming.VocabConfig) *webapi.VocabPatterns {
	if len(v.Patterns) == 0 && !v.Replace {
		return nil
	}
	return &webapi.VocabPatterns{Patterns: v.Patterns, Replace: v.Replace}
}
