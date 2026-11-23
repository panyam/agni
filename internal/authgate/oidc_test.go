package authgate

import (
	"context"
	"strings"
	"testing"
)

// A person whose provider left their groups out of the ID token, as Entra does past 200 groups, is
// refused with a message saying a group rule can't be checked, not that they aren't a member (agni
// issue 1811).
func TestOIDCSaysWhenGroupsAreLeftOut(t *testing.T) {
	allow, err := ParseOIDCAllow([]string{"group:g-eng"})
	if err != nil {
		t.Fatal(err)
	}
	s := &oidcSignIns{allow: allow}
	claims := map[string]any{"sub": "u-1", "email": "ada@example.com", "_claim_names": map[string]any{"groups": "src1"}}
	if _, _, err := s.admit(context.Background(), claims); err == nil || !strings.Contains(err.Error(), "too many groups") {
		t.Errorf("admit with the groups left out = %v; want a refusal saying the groups couldn't be checked", err)
	}
	delete(claims, "_claim_names")
	if _, _, err := s.admit(context.Background(), claims); err == nil || !strings.Contains(err.Error(), "isn't someone this server lets sign in") {
		t.Errorf("admit outside the group = %v; want the plain refusal", err)
	}
}
