package authgate

import (
	"slices"
	"testing"
)

// A request's claims are the sign-in's alone. An OAuth scope, a bare prefix, or anything else a
// token's scopes might hold is dropped, so it can never match a grant (agni issue 1896).
func TestClaimsKeepsOnlySignInClaims(t *testing.T) {
	got := Claims([]string{"oidc:group:hw", "read", "github:org:acme", "oidc:group:", "github:acme", "oidc:ada"})
	if want := []string{"oidc:group:hw", "github:org:acme"}; !slices.Equal(got, want) {
		t.Errorf("Claims = %v; want %v", got, want)
	}
}

func TestGrantOrgsNamesEachExactOrg(t *testing.T) {
	got := GrantOrgs([]string{"local:*", "github:org:hw", "github:org:*", "github:org:hw", "github:org:sales", "oidc:group:hw", "github:org:"})
	if want := []string{"hw", "sales"}; !slices.Equal(got, want) {
		t.Errorf("GrantOrgs = %v; want %v", got, want)
	}
}

func TestGroupClaimsReadsEveryShape(t *testing.T) {
	for _, c := range []struct {
		in   any
		want []string
	}{
		{"hw", []string{"oidc:group:hw"}},
		{[]any{"hw", 7, "", "sales", "hw"}, []string{"oidc:group:hw", "oidc:group:sales"}},
		{[]string{"hw"}, []string{"oidc:group:hw"}},
		{nil, nil},
	} {
		if got := groupClaims(c.in); !slices.Equal(got, c.want) {
			t.Errorf("groupClaims(%v) = %v; want %v", c.in, got, c.want)
		}
	}
}
