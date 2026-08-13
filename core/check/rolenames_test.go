package check

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestDefaultRoleVocab pins the built-in rail/ground/feedback conventions (the historical Go literals,
// now regex) so the "defaults unchanged" contract is a test, not a claim.
func TestDefaultRoleVocab(t *testing.T) {
	v := DefaultRoleVocab()
	for _, n := range []string{"VCC", "VDD_CORE", "+3V3", "12V", "3V3", "PWR_IN", "/psu/12V"} {
		if !v.IsRail(n) {
			t.Errorf("IsRail(%q) = false, want true", n)
		}
	}
	for _, n := range []string{"SDA", "CLK", "USB_DP", "FB_LINE"} {
		if v.IsRail(n) {
			t.Errorf("IsRail(%q) = true, want false", n)
		}
	}
	for _, n := range []string{"GND", "AGND", "DGND", "VSS", "EARTH"} {
		if !v.IsGround(n) {
			t.Errorf("IsGround(%q) = false, want true", n)
		}
	}
	for _, n := range []string{"VCC0.8_ETH_FB", "VOUT_SENSE", "V_VFB", "FB"} {
		if !v.IsFeedback(n) {
			t.Errorf("IsFeedback(%q) = false, want true", n)
		}
	}
	for _, n := range []string{"FBUS", "VCC", "SDA"} { // FBUS must not read as feedback
		if v.IsFeedback(n) {
			t.Errorf("IsFeedback(%q) = true, want false", n)
		}
	}
}

// TestBuildRoleVocabExtendReplace: a project's patterns extend the built-ins by default and replace
// them when Replace is set; a bad regex is a returned error.
func TestBuildRoleVocabExtendReplace(t *testing.T) {
	ext, err := BuildRoleVocab(RoleVocabConfig{Rail: VocabPatterns{Patterns: []string{`^HV_`}}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !ext.IsRail("HV_BATT") {
		t.Error("extended rail pattern should match HV_BATT")
	}
	if !ext.IsRail("VCC") {
		t.Error("extend keeps the built-in rail patterns")
	}

	repl, err := BuildRoleVocab(RoleVocabConfig{Rail: VocabPatterns{Patterns: []string{`^HV_`}, Replace: true}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !repl.IsRail("HV_BATT") {
		t.Error("replaced rail pattern should match HV_BATT")
	}
	if repl.IsRail("VCC") {
		t.Error("replace drops the built-in rail patterns")
	}

	if _, err := BuildRoleVocab(RoleVocabConfig{Rail: VocabPatterns{Patterns: []string{`(bad`}}}); err == nil {
		t.Error("a malformed regex must be a returned error")
	}
}

// TestSetActiveRoleVocab: swapping the process vocab changes what the is*Name helpers report; nil
// restores the defaults.
func TestSetActiveRoleVocab(t *testing.T) {
	defer SetActiveRoleVocab(nil)
	v, err := BuildRoleVocab(RoleVocabConfig{
		Rail:     VocabPatterns{Patterns: []string{`^HV_`}},
		Feedback: VocabPatterns{Patterns: []string{`_ETH_FB$`}},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	SetActiveRoleVocab(v)
	if !IsPowerRailName("HV_BATT") {
		t.Error("IsPowerRailName should honor the active vocab's extra rail pattern")
	}
	if !IsFeedbackName("VCC0.8_ETH_FB") {
		t.Error("IsFeedbackName should honor the active vocab's extra feedback pattern")
	}
	SetActiveRoleVocab(nil)
	if IsPowerRailName("HV_BATT") {
		t.Error("nil should restore defaults (HV_ is not a built-in rail)")
	}
}

// NetRoleSource answers the question NetHasRole deliberately does not: how do we know. It is the
// hook a consumer needs to weigh a role rather than just read it.
func TestNetRoleSourceReportsTheEvidence(t *testing.T) {
	declared := &ir.Net{Name: "N$17", Roles: []*ir.NetRole{
		{Role: NetRoleGround, Source: ir.RoleSource_ROLE_SOURCE_DECLARED},
	}}
	src, ok := NetRoleSource(declared, NetRoleGround, func(string) bool { return false })
	if !ok || src != ir.RoleSource_ROLE_SOURCE_DECLARED {
		t.Errorf("declared ground: got (%v, %v), want (DECLARED, true)", src, ok)
	}

	// A role the net does not carry reports absent, not a weak source.
	if src, ok := NetRoleSource(declared, NetRoleRail, func(string) bool { return false }); ok {
		t.Errorf("a role the net lacks must report absent, got (%v, %v)", src, ok)
	}
}

// The name-fallback path (an IR built without the ingestion pass) reports CONVENTION, because that
// is precisely what the fallback is: a naming convention read at the point of use instead of at
// ingestion. Reporting UNSPECIFIED there would hide that the answer rests on a name.
func TestNetRoleSourceOnTheNameFallbackIsConvention(t *testing.T) {
	unstamped := &ir.Net{Name: "GND"}
	src, ok := NetRoleSource(unstamped, NetRoleGround, func(n string) bool { return n == "GND" })
	if !ok || src != ir.RoleSource_ROLE_SOURCE_CONVENTION {
		t.Errorf("name fallback: got (%v, %v), want (CONVENTION, true)", src, ok)
	}
}
