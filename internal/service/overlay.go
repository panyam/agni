package service

import (
	"fmt"
	"strings"

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
	// baseConvention is the catalog source name of the SERVER's startup convention (`--conventions`),
	// empty when the caller composed no such default. Catalog drops it before splicing this request's
	// own, which is what makes a request-supplied convention override rather than stack (WS3-124).
	//
	// It is supplied by whoever built the base catalog, since only they know which of its sources came
	// from the startup flag. The zero value replaces nothing, which is the honest answer for a caller
	// (the CLI) whose catalog has no startup convention at all.
	baseConvention string
}

// ComposeOverlay resolves an OverlayConfig into engine inputs. A nil or empty config yields a zero
// Overlay, which changes nothing, so a request that names no overlay behaves exactly as before.
//
// baseConvention is the catalog source name of the SERVER's startup convention, which this request's
// own convention replaces (WS3-124); "" when the caller composed no such default, and then nothing is
// replaced. It is a REQUIRED parameter rather than an optional wither because there are three call
// sites across two services, and "remember to also call the other thing" is precisely the shape that
// let --conventions reach one rule-running surface and not the other (WS3-102, WS3-109). A required
// argument makes forgetting it a compile error instead of a silently additive catalog.
//
// It performs no I/O: the config arrives as a value, so composing is pure and a caller can compose the
// same inputs in a test, a CLI, or a browser without a filesystem. An invalid convention (a pattern
// that will not compile, an unknown component class) is an ERROR, never a skip — an operator who asked
// for their conventions and silently got the built-ins would read the resulting clean report as a
// clean design.
func ComposeOverlay(cfg *webapi.OverlayConfig, baseConvention string) (Overlay, error) {
	o := Overlay{baseConvention: baseConvention}
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
// A request's convention REPLACES the server's startup one rather than stacking on it (WS3-124),
// which is what the serve flag help and the lexicon half both already promised. The replacement is a
// tag filter: catalog composition stamps every rule with the name of the source that contributed it
// (check.KeySource), so dropping the startup convention is dropping the rules carrying its name. Only
// that one source goes; the built-ins and any --profile-path / --intent-path sources are not the
// request's to remove, and a request that removed them would report a design clean by asking nothing.
//
// Replacement is scoped to a NAMED base convention rather than "any convention-looking source" on
// purpose. A caller that never names one (the CLI, whose catalog has no startup convention) keeps the
// additive behaviour, so this cannot silently subtract rules from a caller that did not opt in.
//
// A composition error is an error, not a panic: the sources come from a REQUEST here, and a caller
// sending a convention whose name collides with an existing source should get a message, not a
// crashed process.
func (o Overlay) Catalog(base *check.Catalog) (*check.Catalog, error) {
	if len(o.Sources) == 0 {
		return base, nil
	}
	if o.baseConvention != "" {
		base = base.Without(check.Facets{Tags: map[string][]string{check.KeySource: {o.baseConvention}}})
	}
	out, err := base.With(o.Sources...)
	if err != nil {
		return nil, o.explainCollision(err)
	}
	return out, nil
}

// explainCollision adds the one piece of context a duplicate-source error cannot carry on its own:
// that the source it collided with may have come from a server flag rather than from this request.
// The caller sees only their own convention, so "duplicate rule source" reads as though they sent it
// twice, and the actual other party is invisible.
func (o Overlay) explainCollision(err error) error {
	if !strings.Contains(err.Error(), "duplicate rule source") {
		return err
	}
	return fmt.Errorf("%w (a source of that name is already composed into this server's catalog, "+
		"from --conventions or --profile-path; rename the convention, or name it exactly as the "+
		"server's --conventions to replace it)", err)
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
