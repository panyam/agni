package authgate

import (
	"slices"
	"strings"
)

// A token minted at sign-in carries claims the sign-in saw, kept in the token's scopes, so an
// operator's grant can follow a group rather than a list of people (agni issue 1896). Each claim
// starts with a prefix no subject can equal. A subject is "oidc:<sub>" or "github:<login>", and a
// GitHub login holds no ':'. An OIDC sub could, but the identity provider issues both the sub and
// the groups, so it could admit anyone to a group anyway.
const (
	// ClaimOIDCGroup prefixes a value of the OIDC groups claim, as --oidc-allow group: reads it.
	ClaimOIDCGroup = "oidc:group:"
	// ClaimGitHubOrg prefixes a GitHub org the person was a member of at sign-in, as
	// --github-allow org: checks it.
	ClaimGitHubOrg = "github:org:"
)

// Claims keeps the scopes of a request's token that are claims a sign-in recorded, and drops
// anything else, such as an OAuth scope on a token some other path minted.
func Claims(scopes []string) []string {
	var out []string
	for _, s := range scopes {
		if isClaim(s) {
			out = append(out, s)
		}
	}
	return out
}

func isClaim(s string) bool {
	for _, p := range []string{ClaimOIDCGroup, ClaimGitHubOrg} {
		if len(s) > len(p) && strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// GrantOrgs returns the orgs a GitHub sign-in checks membership of for grants, one per
// "github:org:<org>" pattern in patterns. A pattern ending in '*' names no one org, so it is left
// out, and it admits nobody.
func GrantOrgs(patterns []string) []string {
	var out []string
	for _, p := range patterns {
		org, ok := strings.CutPrefix(p, ClaimGitHubOrg)
		if ok && org != "" && !strings.HasSuffix(org, "*") && !slices.Contains(out, org) {
			out = append(out, org)
		}
	}
	return out
}

// groupClaims returns each value of an OIDC groups claim as a claim, whether the provider sent one
// string or a list.
func groupClaims(groups any) []string {
	var vs []string
	switch g := groups.(type) {
	case string:
		vs = []string{g}
	case []string:
		vs = g
	case []any:
		for _, v := range g {
			if s, ok := v.(string); ok {
				vs = append(vs, s)
			}
		}
	}
	var out []string
	for _, v := range vs {
		if v != "" && !slices.Contains(out, ClaimOIDCGroup+v) {
			out = append(out, ClaimOIDCGroup+v)
		}
	}
	return out
}
