package main

import (
	"bytes"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
)

func runWalk(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := walkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// The showcase's USB connector. The series walk follows VBUS through the fuse, as trace does, and the
// small-parts walk stops at the rail.
func TestWalkCLIPrintsEachHopAndWhatItDidNotCross(t *testing.T) {
	out, err := runWalk(t, traceFixtureSch, "--from", "J1", "--kind", "series", "--hops", "3")
	if err != nil {
		t.Fatalf("walk: %v\n%s", err, out)
	}
	for _, want := range []string{
		"walk from J1, series, 3 hops",
		"hop 1\n",
		"  net /VBUS   (from J1 pin 1)",
		"hop 2\n",
		"through F1 (fuse) pin 1 --> pin 2, from /VBUS",
		"ends here: ground",
		"not crossed\n",
		"U2 (ic) on /USB_D+, hop 1: an ic is not a series part",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("series walk output missing %q:\n%s", want, out)
		}
	}
	out, err = runWalk(t, traceFixtureSch, "--from", "J1", "--kind", "small-parts", "--hops", "3")
	if err != nil {
		t.Fatalf("walk: %v\n%s", err, out)
	}
	if !strings.Contains(out, "  net /VBUS   (from J1 pin 1)\n      ends here: rail\n") || strings.Contains(out, "through F1") {
		t.Errorf("the small-parts walk went past the VBUS rail:\n%s", out)
	}
}

// --format json is the wire message (C31), and each net says where it is drawn.
func TestWalkCLIJSONIsTheWireMessageWithSheets(t *testing.T) {
	out, err := runWalk(t, traceFixtureSch, "--from", "J1.1", "--format", "json")
	if err != nil {
		t.Fatalf("walk: %v\n%s", err, out)
	}
	w := &webapi.Walk{}
	if err := protojson.Unmarshal([]byte(out), w); err != nil {
		t.Fatalf("walk json is not a Walk: %v\n%s", err, out)
	}
	if len(w.GetNets()) == 0 {
		t.Fatalf("the walk reached no net:\n%s", out)
	}
	for _, n := range w.GetNets() {
		if len(n.GetNet().GetSheetIds()) == 0 {
			t.Errorf("net %s carries no sheet ids", n.GetNet().GetName())
		}
	}
	if w.GetFrom().GetPinName() == "" || len(w.GetFrom().GetSheetIds()) == 0 {
		t.Errorf("the start pin lacks its name or its sheet: %+v", w.GetFrom())
	}
}

// A start the design lacks is a failed question and exits non-zero, as trace does for an endpoint.
func TestWalkCLIRefusesAStartTheDesignLacks(t *testing.T) {
	out, err := runWalk(t, traceFixtureSch, "--from", "U999")
	if err == nil || !strings.Contains(err.Error(), "no component U999") {
		t.Errorf("err = %v, want the missing part named\n%s", err, out)
	}
	if _, err := runWalk(t, traceFixtureSch, "--from", "J1", "--kind", "sideways"); err == nil || !strings.Contains(err.Error(), "unknown walk kind") {
		t.Errorf("an unknown kind: err = %v", err)
	}
}

// A pin goal asked from a pin start's own net routes as trace does between the same two pins, and
// the other two goals answer from the fixture's I2C pull-up.
func TestWalkCLIGoalRoutesAsTraceDoes(t *testing.T) {
	out, err := runWalk(t, traceFixtureSch, "--from", "J1.1", "--goal", "pin", "--to", "U1.1", "--format", "json")
	if err != nil {
		t.Fatalf("walk: %v\n%s", err, out)
	}
	w := &webapi.Walk{}
	if err := protojson.Unmarshal([]byte(out), w); err != nil {
		t.Fatal(err)
	}
	trOut, err := runTrace(t, traceFixtureSch, "--from", "J1.1", "--to", "U1.1", "--format", "json")
	if err != nil {
		t.Fatalf("trace: %v\n%s", err, trOut)
	}
	tr := &webapi.Trace{}
	if err := protojson.Unmarshal([]byte(trOut), tr); err != nil {
		t.Fatal(err)
	}
	g := w.GetGoal()
	if g.GetOutcome() != webapi.TraceOutcome_TRACE_OUTCOME_ROUTED || len(g.GetRoutes()) != 1 {
		t.Fatalf("goal %v, want one route", g)
	}
	r := g.GetRoutes()[0]
	for _, n := range r.GetNets() {
		if len(n.GetSheetIds()) == 0 {
			t.Errorf("route net %s carries no sheet ids", n.GetName())
		}
	}
	if len(g.GetTo().GetSheetIds()) == 0 {
		t.Errorf("the goal pin carries no sheet ids: %v", g.GetTo())
	}
	if len(r.GetCrossings()) != len(tr.GetCrossings()) || len(r.GetNets()) != len(tr.GetNets()) {
		t.Fatalf("goal crossed %d parts over %d nets, trace %d over %d", len(r.GetCrossings()), len(r.GetNets()), len(tr.GetCrossings()), len(tr.GetNets()))
	}
	for i, c := range r.GetCrossings() {
		if c.GetRefDes() != tr.GetCrossings()[i].GetRefDes() || r.GetNets()[i].GetName() != tr.GetNets()[i].GetName() {
			t.Errorf("step %d: goal %s on %s, trace %s on %s", i, c.GetRefDes(), r.GetNets()[i].GetName(), tr.GetCrossings()[i].GetRefDes(), tr.GetNets()[i].GetName())
		}
	}

	for goal, want := range map[string]string{
		"pull-up": "goal to the pull-up, from /SDA\n  route 1: /SDA -> [R1] -> +3V3\n",
		"rail":    "goal to the rail, from /SDA\n  route 1: /SDA -> [R1] -> +3V3\n",
	} {
		out, err := runWalk(t, traceFixtureSch, "--from", "U2.3", "--goal", goal)
		if err != nil || !strings.Contains(out, want) {
			t.Errorf("--goal %s: err %v, want %q in\n%s", goal, err, want, out)
		}
	}
}

// A goal naming nothing is a failed question and exits non-zero. A goal with no route is an answer.
func TestWalkCLIGoalKeepsItsOutcomesApart(t *testing.T) {
	out, err := runWalk(t, traceFixtureSch, "--from", "J1", "--goal", "pin", "--to", "U1.1", "--at", "/NOPE")
	if err == nil || !strings.Contains(err.Error(), "did not reach a net /NOPE") {
		t.Errorf("an unreached net: err = %v\n%s", err, out)
	}
	out, err = runWalk(t, traceFixtureSch, "--from", "U2.3", "--goal", "pin", "--to", "TP2.1")
	if err != nil || !strings.Contains(out, "  no route: no series path from /SDA to GND") {
		t.Errorf("no route: err = %v\n%s", err, out)
	}
	if _, err := runWalk(t, traceFixtureSch, "--from", "J1", "--goal", "pin"); err == nil || !strings.Contains(err.Error(), "--to") {
		t.Errorf("a pin goal without --to: err = %v", err)
	}
	if _, err := runWalk(t, traceFixtureSch, "--from", "J1", "--at", "/VBUS"); err == nil || !strings.Contains(err.Error(), "--goal") {
		t.Errorf("--at without --goal: err = %v", err)
	}
	if _, err := runWalk(t, traceFixtureSch, "--from", "J1", "--goal", "ground"); err == nil || !strings.Contains(err.Error(), "unknown walk goal") {
		t.Errorf("an unknown goal: err = %v", err)
	}
}
