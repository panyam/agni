package main

import (
	"strings"
	"testing"
)

// A thin curve on an otherwise wide net is the copper the board rules could not see until the KiCad
// reader carried arc tracks (agni issue 1206). The net's straight run clears the floor, so CURVED
// fails on the arc alone, and STRAIGHT is the control for a rule that fires on everything.
func TestTrackWidthSeesAnArcTrack(t *testing.T) {
	got := runCheck(t, "--rule", "track-width", "--format", "csv", "testdata/arc-track/thin-arc.kicad_pcb")
	if !strings.Contains(got, "CURVED") {
		t.Errorf("track-width did not report the thin arc on CURVED:\n%s", got)
	}
	if strings.Contains(got, "STRAIGHT") {
		t.Errorf("track-width reported STRAIGHT, whose copper clears the floor:\n%s", got)
	}
}
