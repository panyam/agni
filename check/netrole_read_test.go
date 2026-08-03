package check

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestNetRoleStampedIsAuthoritative (WS3-072): the core reads the STAMPED net.role fact, not the name,
// when a net carries one. A net named "MYSTERY" (which no naming pattern matches) but stamped with the
// ground role projects a net.ground fact — proof the read comes from the field, not a re-run of the name
// lexicon. This is the left-shift made observable: had the projector still called isGroundName, MYSTERY
// would yield nothing.
func TestNetRoleStampedIsAuthoritative(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{
		{Name: "MYSTERY", Roles: []string{NetRoleGround}, Prov: &ir.Provenance{SourceFile: "t"}},
	}}
	gf := factsByRelation(Facts(NewModel(d)))[RelNetGround]
	if len(gf) != 1 || gf[0].Subject != "MYSTERY" {
		t.Errorf("stamped ground role must drive net.ground regardless of name, got %+v", gf)
	}
}

// TestNetRoleFallsBackToName (WS3-072): a net with NO stamped roles (a hand-authored test IR that
// skipped the ingestion pass) re-derives its role from the name, so behavior is unchanged for every
// existing fixture. A plain GND net still grounds; a signal net still does not.
func TestNetRoleFallsBackToName(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{
		{Name: "GND", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "SDA", Prov: &ir.Provenance{SourceFile: "t"}},
	}}
	gf := factsByRelation(Facts(NewModel(d)))[RelNetGround]
	if len(gf) != 1 || gf[0].Subject != "GND" {
		t.Errorf("unstamped GND must fall back to name match, got %+v", gf)
	}
}
