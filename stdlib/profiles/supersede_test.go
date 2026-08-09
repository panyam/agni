package profiles

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
)

// An overlay profile carrying a built-in's name supersedes that built-in's rules rather than running
// beside them (WS3-056). Augmenting is what produced the false failures this replaced: see the partial
// naming-overlap case in cmd/agni's TestCheckNamingMapSupersedesCoreProfile.
func TestOverlaySourceSupersedesBuiltinProfile(t *testing.T) {
	p, err := Load(strings.NewReader("override: SPI_NOR\nsuffixes: {IO0: _DQ0}\n"))
	if err != nil {
		t.Fatalf("Load naming map: %v", err)
	}
	src, ok := Source("profile-overlay", []Profile{p}).(check.SupersedingSource)
	if !ok {
		t.Fatal("an overlay profile shadowing a built-in must produce a SupersedingSource")
	}
	got := src.Supersedes()
	if len(got) != 1 {
		t.Fatalf("Supersedes() = %+v, want one selection", got)
	}
	if want := []string{"SPI_NOR"}; !equalStrings(got[0].Tags[TagProfile], want) {
		t.Errorf("supersedes profile tag %v, want %v", got[0].Tags[TagProfile], want)
	}
	// Scoped to the built-in source, so two overlay sources defining one interface cannot delete each
	// other in composition order.
	if want := []string{BuiltinSourceName}; !equalStrings(got[0].Tags[check.KeySource], want) {
		t.Errorf("supersedes source tag %v, want %v", got[0].Tags[check.KeySource], want)
	}
}

// A profile whose name matches no built-in is additive: it declares nothing, so the catalog composes
// it exactly as before. This is what keeps a customer's own proprietary interface from silently
// suppressing anything.
func TestOverlaySourceWithoutBuiltinNameDoesNotSupersede(t *testing.T) {
	p, err := Load(strings.NewReader(
		"name: HOUSEBUS\nsignals: [{name: A, suffix: _HA, anchor: true}]\nrequirements: [{type: signal-dangling}]\n"))
	if err != nil {
		t.Fatalf("Load profile: %v", err)
	}
	if sup, ok := Source("profile-overlay", []Profile{p}).(check.SupersedingSource); ok && len(sup.Supersedes()) > 0 {
		t.Errorf("a non-shadowing overlay declared supersessions: %+v", sup.Supersedes())
	}
}

// The composed catalog keeps exactly one rule per requirement of a superseded interface, and it is the
// overlay's. Asserted end-to-end through the catalog because the two halves (declaring, and applying
// the declaration) can each look right while disagreeing about the source name they key on.
func TestSupersededCatalogRunsOverlayRulesOnly(t *testing.T) {
	p, err := Load(strings.NewReader("override: SPI_NOR\nsuffixes: {IO0: _DQ0}\n"))
	if err != nil {
		t.Fatalf("Load naming map: %v", err)
	}
	c := check.CatalogWith(Source("profile-overlay", []Profile{p}))
	var core, overlay int
	for _, r := range c.Rules() {
		if r.Tags[TagProfile] != "SPI_NOR" {
			continue
		}
		switch r.Tags[check.KeySource] {
		case BuiltinSourceName:
			core++
		case "profile-overlay":
			overlay++
		}
	}
	if core != 0 {
		t.Errorf("%d built-in SPI_NOR rules survived supersession, want 0", core)
	}
	if overlay != len(SPINOR.Requirements) {
		t.Errorf("overlay contributed %d SPI_NOR rules, want %d", overlay, len(SPINOR.Requirements))
	}
	// Another interface's rules are untouched: supersession is scoped to the interface, not the source.
	var can int
	for _, r := range c.Rules() {
		if r.Tags[TagProfile] == "CAN" {
			can++
		}
	}
	if can != len(CAN.Requirements) {
		t.Errorf("CAN rules = %d, want %d (supersession must not reach another interface)", can, len(CAN.Requirements))
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
