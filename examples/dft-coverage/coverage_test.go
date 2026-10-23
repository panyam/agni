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
		"neither, by MPN":    {byMPNQuery, `DEMO-RES-4K7/2/["R3","R4"]`},
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

// TestTheWhyStepExplainsARowThroughTheLibrary pins what the witness step prints for the fixture's first
// one-end row: the walk's own passive rule, then the library member with its body facts, ending in the
// `not` that held on the unprobed net (agni issue 794).
func TestTheWhyStepExplainsARowThroughTheLibrary(t *testing.T) {
	d, err := common.Load("../common/designs/probe-coverage.edn")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	cells, ws, err := witnessed(d, oneQuery)
	if err != nil || len(cells) != 2 {
		t.Fatalf("witnessed = %v, %v", cells, err)
	}
	if len(ws) != 2 || ws[0].Relation != "passive" || ws[1].Relation != "component.probed_one" {
		t.Fatalf("witnesses %+v, want passive then component.probed_one", ws)
	}
	member := ws[1]
	last := member.Children[len(member.Children)-1]
	if !last.Negated || last.Relation != "net.has_test_point" || last.Values[0] != cells[1] {
		t.Errorf("the member's last body fact is %+v, want not net.has_test_point(%s)", last, cells[1])
	}
	if !strings.Contains(member.Rule, ":-") {
		t.Errorf("the member's rule %q is not its written definition", member.Rule)
	}
}
