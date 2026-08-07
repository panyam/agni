package check

import (
	"testing"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func projectLexicon(t *testing.T, railPattern string) *classify.Lexicon {
	t.Helper()
	rv, err := classify.BuildRoleVocab(classify.RoleVocabConfig{
		Rail: classify.VocabPatterns{Patterns: []string{railPattern}},
	})
	if err != nil {
		t.Fatalf("BuildRoleVocab: %v", err)
	}
	return &classify.Lexicon{Role: rv}
}

// TestModelNameProjectionsUseItsLexicon covers the residual name matches — the ones with no net to
// read a stamped role from (the spec name FFIs over a literal, pin-role derivation). A model built
// WithLexicon answers with the project's conventions; one built without it keeps the built-ins, and
// neither disturbs the other.
func TestModelNameProjectionsUseItsLexicon(t *testing.T) {
	d := &ir.Design{}
	plain := NewModel(d)
	project := NewModel(d, WithLexicon(projectLexicon(t, "^PMIC_VDD_LPM_1V8$")))

	if plain.IsPowerRailName("PMIC_VDD_LPM_1V8") {
		t.Error("a function-first rail name must not match the start-anchored built-ins")
	}
	if !project.IsPowerRailName("PMIC_VDD_LPM_1V8") {
		t.Error("the project lexicon must reach the model's name projection")
	}
	if !plain.IsPowerRailName("VCC") || !project.IsPowerRailName("VCC") {
		t.Error("a project lexicon extends the built-ins, it does not replace them")
	}
	if !plain.IsGroundName("GND") || !project.IsGroundName("GND") {
		t.Error("an untouched vocabulary half keeps its built-ins")
	}
}

// TestIsGroundNetPrefersStampedRole pins the trust rule the net-taking predicates rely on: the role
// set stamped at ingestion wins, and this model's lexicon is consulted only for a net that carries
// none (an IR built without the loader). This is why converting a rule from a bare name match to
// IsGroundNet is behavior-preserving.
func TestIsGroundNetPrefersStampedRole(t *testing.T) {
	stamped := &ir.Net{Name: "AGND_ANALOG", Roles: []string{NetRoleGround}}
	unstamped := &ir.Net{Name: "GND"}
	notGround := &ir.Net{Name: "SIG", Roles: []string{NetRoleRail}}

	m := NewModel(&ir.Design{Nets: []*ir.Net{stamped, unstamped, notGround}})
	if !m.IsGroundNet(stamped) {
		t.Error("a stamped ground role is authoritative even when the name matches no vocabulary")
	}
	if !m.IsGroundNet(unstamped) {
		t.Error("an unstamped net must fall back to the model's naming lexicon")
	}
	if m.IsGroundNet(notGround) {
		t.Error("a net with a stamped role set that omits ground is not ground")
	}
	if !m.IsRailNet(notGround) || m.IsRailNet(stamped) {
		t.Error("IsRailNet reads the stamped set the same way")
	}
}

// TestIsRailNetIsNarrowerThanIsPowerRail states the distinction deliberately: IsPowerRail also
// answers true for a driven-or-global net and for grounds, because it serves the "distributed by
// power-symbol taps, nothing to stroke" locate question. A rule asking whether a net is a rail must
// not inherit that.
func TestIsRailNetIsNarrowerThanIsPowerRail(t *testing.T) {
	gnd := &ir.Net{Name: "GND", Roles: []string{NetRoleGround}}
	m := NewModel(&ir.Design{Nets: []*ir.Net{gnd}})
	if !m.IsPowerRail("GND") {
		t.Fatal("IsPowerRail answers true for a ground (the locate question)")
	}
	if m.IsRailNet(gnd) {
		t.Error("IsRailNet is the role question alone, so a ground is not a rail")
	}
}
