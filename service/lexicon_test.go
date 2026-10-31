package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check/naming"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/proto"
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

// A candidate is listed over the built-ins with its own source, and its rules come back with it, since
// a client editing a convention has to send them on to CompareLexicons or they stop running.
func TestGetLexiconListsACandidate(t *testing.T) {
	cand := &configpb.NamingConvention{Name: "edit", Rules: []*configpb.NamingRule{{Name: "r", Allow: []string{"^A"}}},
		Lexicon: &configpb.NamingLexicon{Net: &configpb.NetNameVocab{Rail: &configpb.VocabPatterns{Patterns: []string{"^USBH_3V3$"}}}}}
	checks := NewCheckService(fakeLoader{}, nil, nil, "", nil, nil)
	ctx := context.Background()
	snap, err := checks.Snapshots().Ingest(ctx, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://m/d/board.edn"}})
	if err != nil {
		t.Fatal(err)
	}
	name := snap.GetName()
	resp, err := NewLexiconService(checks).GetLexicon(ctx, &webapi.GetLexiconRequest{Snapshot: name, Candidate: cand})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range resp.GetEntries() {
		found = found || (e.GetValue() == "^USBH_3V3$" && e.GetSource() == webapi.LexiconSource_LEXICON_SOURCE_REQUEST && e.GetConvention() == "edit")
	}
	if !found {
		t.Errorf("the candidate's pattern is not listed under its own source: %v", resp.GetEntries())
	}
	if len(resp.GetConvention().GetRules()) != 1 {
		t.Errorf("convention = %v, want the candidate with its rule", resp.GetConvention())
	}
	cand.Lexicon.Net.Rail.Patterns = []string{"("}
	if _, err := NewLexiconService(checks).GetLexicon(ctx, &webapi.GetLexiconRequest{Snapshot: name, Candidate: cand}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a bad pattern: err = %v, want ErrInvalidArgument", err)
	}
}

// RenderConvention's file reads back to the message it was given, rules included.
func TestRenderConventionRoundTrips(t *testing.T) {
	off := false
	conv := &configpb.NamingConvention{Name: "house",
		Rules: []*configpb.NamingRule{{Name: "signal", Severity: "info", Why: "w", Allow: []string{"^[A-Z]"}, Exempt: []string{"^\\\\+"}}},
		Lexicon: &configpb.NamingLexicon{
			Net:   &configpb.NetNameVocab{Rail: &configpb.VocabPatterns{Patterns: []string{"^USBH_3V3$"}}, GateDrive: &configpb.VocabPatterns{Replace: true}, InferRailFromSupplyPins: &off},
			Class: map[string]*configpb.ClassVocab{"tvs": {Families: []string{"^acme"}}},
		}}
	resp, err := (&LexiconService{}).RenderConvention(context.Background(), &webapi.RenderConventionRequest{Conventions: conv})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetFilename() != "house.yaml" {
		t.Errorf("filename = %q", resp.GetFilename())
	}
	if !strings.HasPrefix(string(resp.GetContent()), "name: house\n") {
		t.Errorf("the file keeps JSON's spelling:\n%s", resp.GetContent())
	}
	back, err := naming.Parse(resp.GetContent())
	if err != nil {
		t.Fatalf("%v in\n%s", err, resp.GetContent())
	}
	if !proto.Equal(back, conv) {
		t.Errorf("read back %v, want %v", back, conv)
	}
}
