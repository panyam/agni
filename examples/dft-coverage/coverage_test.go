package main

import (
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/examples/common"
)

// TestTheFixtureAnswersAreTheDesignedOnes pins the coverage cases the fixture was built around (see
// README.md, "The design"), so a change to a bucket's query or to the library members it calls shows
// up as a changed answer rather than a quietly different walkthrough.
func TestTheFixtureAnswersAreTheDesignedOnes(t *testing.T) {
	d, err := common.Load("../common/designs/probe-coverage.edn")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	for name, c := range map[string]struct{ q, want string }{
		"both ends probed":   {bothQuery, "R1"},
		"one end probed":     {oneQuery, "C1/GND,C2/GND,R2/SENSE"},
		"neither end probed": {neitherQuery, "R3,R4"},
		"neither, by MPN":    {byMPNQuery, "DEMO-RES-4K7/2/R3 R4"},
		"unprobed nets":      {unprobedQuery, "GND,SENSE,STRAP"},
	} {
		got, err := rows(d, c.q)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		var cells []string
		for _, r := range got {
			cells = append(cells, strings.Join(r, "/"))
		}
		sort.Strings(cells)
		if s := strings.Join(cells, ","); s != c.want {
			t.Errorf("%s = %q, want %q", name, s, c.want)
		}
	}
}
