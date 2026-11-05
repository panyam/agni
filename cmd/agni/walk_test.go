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
