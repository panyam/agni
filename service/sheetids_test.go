package service

import (
	"os"
	"testing"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/readers/edif"
)

// agni issue 1140: with every block's page named &1, a finding on BlockB's C1 was badged with an id
// four sheets shared, so opening it could land on any of them.
func TestAFindingInARepeatedPageBlockNamesItsOwnSheet(t *testing.T) {
	f, err := os.Open("../readers/edif/testdata/aggregator_dup_pages.eds")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := edif.ReadSchematic(f, "aggregator_dup_pages.eds")
	if err != nil {
		t.Fatal(err)
	}
	r1 := &checkspb.Finding{Subject: &checkspb.Subject{Kind: check.KindComponent, Ref: "R1"}}
	c1 := &checkspb.Finding{Subject: &checkspb.Subject{Kind: check.KindComponent, Ref: "C1"}}
	AnnotateSheets([]*checkspb.Finding{r1, c1}, g, nil)
	if got := r1.GetSheets(); len(got) != 1 || got[0] != "&block_a/&v_a/&1" {
		t.Errorf("R1's sheets %v, want [&block_a/&v_a/&1]", got)
	}
	if got := c1.GetSheets(); len(got) != 1 || got[0] != "&block_b/&v_b/&1" {
		t.Errorf("C1's sheets %v, want [&block_b/&v_b/&1]", got)
	}
}
