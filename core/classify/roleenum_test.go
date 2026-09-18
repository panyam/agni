package classify

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestRoleTokenIsDerivedNotTabulated: the tokens the query surface and conventions.yaml speak are
// produced from the generated enum name, so they cannot drift from the vocabulary and another
// language can perform the identical transform on its own generated enum. A table here would be the
// duplication agni 692 removed.
func TestRoleTokenIsDerivedNotTabulated(t *testing.T) {
	for _, c := range []struct {
		role ir.Role
		want string
	}{
		{ir.Role_ROLE_RAIL, "rail"},
		{ir.Role_ROLE_GROUND, "ground"},
		{ir.Role_ROLE_FEEDBACK, "feedback"},
		{ir.Role_ROLE_SWITCHING, "switching"},
		{ir.Role_ROLE_CONTROL, "control"},
		// The multi-word case, which is the one a naive transform gets wrong.
		{ir.Role_ROLE_GATE_DRIVE, "gate_drive"},
	} {
		if got := RoleToken(c.role); got != c.want {
			t.Errorf("RoleToken(%v) = %q, want %q", c.role, got, c.want)
		}
	}
	// The zero value must not render as a role, or it reaches a fact row looking like one.
	if got := RoleToken(ir.Role_ROLE_UNSPECIFIED); got != "" {
		t.Errorf("RoleToken(UNSPECIFIED) = %q, want empty", got)
	}
}

// TestRoleTokensAreStableAcrossTheVocabulary: every role must render to the token the docs, the
// config lexicon and every committed query already use. Renaming an enum value silently changes a
// user-facing string, and this is what objects.
func TestRoleTokensAreStableAcrossTheVocabulary(t *testing.T) {
	want := map[ir.Role]string{
		ir.Role_ROLE_RAIL: "rail", ir.Role_ROLE_GROUND: "ground",
		ir.Role_ROLE_FEEDBACK: "feedback", ir.Role_ROLE_SWITCHING: "switching",
		ir.Role_ROLE_CONTROL: "control", ir.Role_ROLE_GATE_DRIVE: "gate_drive",
	}
	all := AllNetRoles()
	if len(all) != len(want) {
		t.Fatalf("AllNetRoles has %d entries, the stability table has %d; a role was added to the proto without deciding its token", len(all), len(want))
	}
	for _, r := range all {
		if RoleToken(r) != want[r] {
			t.Errorf("token for %v changed to %q, want %q; this is a user-facing string in queries and conventions.yaml", r, RoleToken(r), want[r])
		}
	}
}

// TestParseRoleRefusesWhatTheVocabularyDoesNotHave: the loud half of a closed vocabulary. A reader
// translating its format's netclass, or config naming a role, gets a refusal rather than a value that
// silently matches nothing. That silence is the shape agni 677 is about.
func TestParseRoleRefusesWhatTheVocabularyDoesNotHave(t *testing.T) {
	for _, r := range AllNetRoles() {
		got, ok := ParseRole(RoleToken(r))
		if !ok || got != r {
			t.Errorf("ParseRole(RoleToken(%v)) = %v, %v; the round trip must hold for every role", r, got, ok)
		}
	}
	for _, bad := range []string{"", "clock", "RAIL", "gate-drive", "switching "} {
		if got, ok := ParseRole(bad); ok {
			t.Errorf("ParseRole(%q) = %v, true; an unknown token must refuse rather than resolve", bad, got)
		}
	}
}

// TestAllNetRolesReadsTheGeneratedEnum: the list is derived, so a role added to the proto is
// projected, parseable and configurable without anyone editing a second place. Before agni 692 this
// was a hand-kept slice, and two roles spent a release absent from the query surface.
func TestAllNetRolesReadsTheGeneratedEnum(t *testing.T) {
	all := AllNetRoles()
	if len(all) != len(ir.Role_name)-1 {
		t.Errorf("AllNetRoles has %d of the enum's %d non-zero values; it is not reading the generated list", len(all), len(ir.Role_name)-1)
	}
	for _, r := range all {
		if r == ir.Role_ROLE_UNSPECIFIED {
			t.Error("AllNetRoles must not include the zero value")
		}
	}
}
