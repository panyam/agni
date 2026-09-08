package main

import (
	"os"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/examples/common"
)

// TestMain clears AGNI_EXAMPLE_DESIGN, for the same reason examples/common does: every test here
// asserts what the walkthrough does on its BUNDLED fixture, and the variable exists to replace that
// fixture with someone's own board. Left set, these assert the narration against a design the prose
// was never written about.
func TestMain(m *testing.M) {
	os.Unsetenv(common.DesignPathEnv)
	os.Exit(m.Run())
}

// The walkthrough's prose states what the bundled fixture does, and prose cannot be checked by
// building. These hold the three claims it makes, so a change to the fixture or to the walk turns
// a false narration into a failure rather than into a demo that quietly teaches the wrong thing.

func TestDefaultEndpointsRouteThroughThePullUp(t *testing.T) {
	tr, err := traceOn(common.AskPath("design", defaultDesign), defaultFrom, defaultTo)
	if err != nil {
		t.Fatalf("traceOn: %v", err)
	}
	if tr.Outcome != check.TraceRouted {
		t.Fatalf("outcome = %q (%s), want routed", tr.Outcome, tr.Reason)
	}
	if len(tr.Crossings) != 1 || tr.Crossings[0].RefDes != "R1" {
		t.Fatalf("crossings = %+v, want one through R1", tr.Crossings)
	}
	if got := traceLines(*tr); !strings.Contains(got, "U1.3 -> R1 -> J1.1") {
		t.Errorf("narration does not show the route:\n%s", got)
	}
}

// Step 4 tells the reader that VCC on this fixture is a supply by name and not rail-scale by
// measure, so the walk stopped on arrival rather than by refusing to continue. That claim is about
// the fixture's fan-out, so it can stop being true without anyone editing the prose.
func TestTheFixturesSupplyIsNotRailScale(t *testing.T) {
	tr, err := traceOn(common.AskPath("design", defaultDesign), defaultFrom, defaultTo)
	if err != nil {
		t.Fatalf("traceOn: %v", err)
	}
	last := tr.Nets[len(tr.Nets)-1]
	if last.Name != "VCC" {
		t.Fatalf("last net = %q, want VCC", last.Name)
	}
	if last.BusLike {
		t.Error("VCC now measures as rail-scale, so step 4's narration describes the other branch")
	}
}

func TestTheNoRoutePairIsGenuinelyUnrouted(t *testing.T) {
	d, err := common.Load(defaultDesign)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	tr := check.TracePins(check.NewModel(d), unreachable[0], unreachable[1], check.DefaultTraceHops)
	if tr.Outcome != check.TraceNoRoute {
		t.Fatalf("outcome = %q, want no-route: step 5 shows this pair as the no-route case", tr.Outcome)
	}
	if tr.From.Net == "" || tr.To.Net == "" {
		t.Errorf("both endpoints must resolve, or this is the unresolved case instead: %+v", tr)
	}
}

func TestParseEndpointRejectsAPinWithoutADot(t *testing.T) {
	if _, err := parseEndpoint("U1"); err == nil {
		t.Error("want an error naming the <ref-des>.<pin> form")
	}
}
