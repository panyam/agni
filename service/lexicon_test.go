package service

import (
	"context"
	"testing"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// A convention is attributed to the tier that supplied it: the project, the design's own descriptor
// (which replaces the project's), or the request (which replaces both).
func TestLexiconEntriesNameTheConventionsTier(t *testing.T) {
	conv := func(name, pat string) *configpb.NamingConvention {
		return &configpb.NamingConvention{Name: name, Lexicon: &configpb.NamingLexicon{
			Net: &configpb.NetNameVocab{Rail: &configpb.VocabPatterns{Patterns: []string{pat}}}}}
	}
	project := &webapi.Project{Name: "p", Config: &webapi.AnalysisConfig{Conventions: conv("house", "^P_RAIL$")}}
	for _, c := range []struct {
		name    string
		design  *webapi.Design
		req     *webapi.OverlayConfig
		pattern string
		src     webapi.LexiconSource
		conv    string
	}{
		{"project", &webapi.Design{Name: "d"}, nil, "^P_RAIL$", webapi.LexiconSource_LEXICON_SOURCE_PROJECT, "house"},
		{"design", &webapi.Design{Name: "d", Config: &webapi.AnalysisConfig{Conventions: conv("board", "^D_RAIL$")}}, nil,
			"^D_RAIL$", webapi.LexiconSource_LEXICON_SOURCE_DESIGN, "board"},
		{"request", &webapi.Design{Name: "d"}, &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Conventions: conv("try", "^R_RAIL$")}},
			"^R_RAIL$", webapi.LexiconSource_LEXICON_SOURCE_REQUEST, "try"},
	} {
		ov, err := OverlayFor(context.Background(), nil, nil, project, c.design, c.req, "")
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		cv, src := conventionOf(Snap{Overlay: ov})
		var found bool
		for _, e := range lexiconEntries(cv, src) {
			if e.GetVocabulary() == "net.rail" && e.GetValue() == c.pattern {
				found = true
				if e.GetSource() != c.src || e.GetConvention() != c.conv {
					t.Errorf("%s: %s came from %s (%q), want %s (%q)", c.name, c.pattern, e.GetSource(), e.GetConvention(), c.src, c.conv)
				}
			} else if e.GetVocabulary() == "net.rail" && e.GetSource() != webapi.LexiconSource_LEXICON_SOURCE_BUILTIN {
				t.Errorf("%s: %s is listed from %s, but only %s is the convention's", c.name, e.GetValue(), e.GetSource(), c.pattern)
			}
		}
		if !found {
			t.Errorf("%s: the effective lexicon lacks %s", c.name, c.pattern)
		}
	}
}
