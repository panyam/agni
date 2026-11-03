package service

import (
	"testing"

	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// houseLexiconYAML declares a house spelling for one NET vocabulary and all four PIN vocabularies.
// The BUILT-IN vocabulary matches none of the patterns. Its defaults are ^G$/^GATE$ for gate,
// ^S$/^SOURCE$/^SRC$ for source, and ^D$/^DRAIN$/^DRN$ for drain. So if a vocabulary fails to
// travel, the name reads as "not recognized", and no default covering the same name can mask the
// failure.
const houseLexiconYAML = `
name: house
lexicon:
  net:
    rail:
      patterns: ["^HOUSERAIL$"]
  pin:
    supply:
      patterns: ["^HVIN$"]
    gate:
      patterns: ["^GT$"]
    source:
      patterns: ["^SRCE$"]
    drain:
      patterns: ["^DRNE$"]
`

// TestConventionLexiconSurvivesOverlay pins one path. An operator writes a vocabulary in
// conventions.yaml, and it must reach the lexicon that the design is READ with. Every path except
// serve's startup install depends on this. `agni check`, `agni review` and `agni query` all compose
// their convention through naming.Parse -> AnalysisConfig -> ComposeOverlay, and a project's own
// conventions.yaml takes the same route via internal/projects. All of them silently drop anything
// the wire form cannot carry. BuildRoleVocab then falls back to the built-in names, and the operator
// who asked for their vocabulary reads a report written against the defaults.
//
// The assertion is behavioural (does the vocabulary classify the name) rather than
// structural (does the proto have the field), so it keeps its meaning if the config schema is
// reshaped underneath it.
func TestConventionLexiconSurvivesOverlay(t *testing.T) {
	cfg, err := naming.Parse([]byte(houseLexiconYAML))
	if err != nil {
		t.Fatalf("parse convention: %v", err)
	}

	ov, err := ComposeOverlay(&webapi.OverlayConfig{
		Config: &webapi.AnalysisConfig{Conventions: cfg},
	}, "")
	if err != nil {
		t.Fatalf("compose overlay: %v", err)
	}
	if ov.Lexicon == nil {
		t.Fatal("overlay carries no lexicon; the convention did not compose at all")
	}
	v := ov.Lexicon.RoleVocab()

	for _, tc := range []struct {
		dim, name string
		got       bool
	}{
		{"rail", "HOUSERAIL", v.IsRail("HOUSERAIL")},
		{"supply_pin", "HVIN", v.IsSupplyPin("HVIN")},
		{"gate", "GT", v.IsGate("GT")},
		{"source", "SRCE", v.IsSource("SRCE")},
		{"drain", "DRNE", v.IsDrain("DRNE")},
	} {
		if !tc.got {
			t.Errorf("lexicon %s: %q not recognized after the overlay round-trip, so the vocabulary declared in conventions.yaml did not reach the read", tc.dim, tc.name)
		}
	}
}
