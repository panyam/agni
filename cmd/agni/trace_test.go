package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
)

const traceFixtureSch = "testdata/conformance/showcase.passes.kicad_sch"

func runTrace(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := traceCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// The demo case, end to end: a USB connector to the regulator it feeds, through the fuse between
// them. The fuse is invisible to any per-net question, because a series element splits the net.
func TestTraceCLIShowsTheRouteAndItsProbePoints(t *testing.T) {
	out, err := runTrace(t, traceFixtureSch, "--from", "J1.1", "--to", "U1.1")
	if err != nil {
		t.Fatalf("trace: %v\n%s", err, out)
	}
	for _, want := range []string{
		"J1.1 (VBUS) --> F1 --> U1.1 (VIN)", // the one line a reviewer copies
		"cross F1 (fuse) pin 1 --> pin 2",
		"net VBUS",
		"probe: TP3.1",
		"net VBUS_PROT",
		"probe: TP4.1",
		"also: C5.1 (capacitor)",
		"routed: 1 crossing, 2 nets, searched to a radius of 6",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("trace output missing %q:\n%s", want, out)
		}
	}
	// A virtual power symbol is connectivity evidence, not a part sitting on the net.
	if strings.Contains(out, "#PWR") || strings.Contains(out, "#FLG") {
		t.Errorf("virtual symbols listed as parts on the route:\n%s", out)
	}
}

// A route may END on a rail. The pull-up resistor from the I2C line lands on +3V3, which is where
// the walk that the protection rules use cannot go at all.
func TestTraceCLIEndsOnARail(t *testing.T) {
	out, err := runTrace(t, traceFixtureSch, "--from", "U2.3", "--to", "U1.2")
	if err != nil {
		t.Fatalf("trace: %v\n%s", err, out)
	}
	if !strings.Contains(out, "cross R1 (resistor) pin 2 --> pin 1") {
		t.Errorf("output does not cross the pull-up:\n%s", out)
	}
	if !strings.Contains(out, "net +3V3") {
		t.Errorf("output does not reach the rail:\n%s", out)
	}
}

// No route is an ANSWER, so it exits clean, and it says what the answer rests on: which nets the
// two pins are actually on, the radius searched, and what the walk will and will not cross.
func TestTraceCLINoRouteExplainsItself(t *testing.T) {
	out, err := runTrace(t, traceFixtureSch, "--from", "U2.3", "--to", "TP2.1")
	if err != nil {
		t.Fatalf("a no-route answer must not be an error: %v\n%s", err, out)
	}
	for _, want := range []string{
		"no route: U2.3 (SDA) --> TP2.1",
		"U2.3 (SDA) on net SDA",
		"TP2.1 on net GND",
		"no series path from SDA to GND within 6 crossings",
		"A capacitor is a DC block and is never crossed.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no-route output missing %q:\n%s", want, out)
		}
	}
}

// An endpoint that names nothing is a failed question, not a disconnection, so it exits NON-zero and
// says which name failed. A script must be able to tell this from "they are not connected".
func TestTraceCLIUnresolvedEndpointIsAnError(t *testing.T) {
	out, err := runTrace(t, traceFixtureSch, "--from", "U99.1", "--to", "U1.1")
	if err == nil {
		t.Fatalf("an unresolved endpoint must not exit clean:\n%s", out)
	}
	if !strings.Contains(err.Error(), "no component U99") {
		t.Errorf("error = %q, want it to name the endpoint that failed", err)
	}
	if strings.Contains(out, "no route") {
		t.Errorf("an unresolved endpoint rendered as a disconnection:\n%s", out)
	}
}

// The radius bounds the search, is reported, and can be widened. Two crossings is not enough to get
// from the connector to the regulator's output, three is.
func TestTraceCLIRadiusIsAFlag(t *testing.T) {
	out, err := runTrace(t, traceFixtureSch, "--from", "J1.1", "--to", "U1.1", "--hops", "1")
	if err != nil {
		t.Fatalf("trace: %v\n%s", err, out)
	}
	if !strings.Contains(out, "searched to a radius of 1") && !strings.Contains(out, "within 1 crossings") {
		t.Errorf("the radius the answer rests on is not stated:\n%s", out)
	}
}

func TestTraceCLIJSON(t *testing.T) {
	out, err := runTrace(t, traceFixtureSch, "--from", "J1.1", "--to", "U1.1", "--format", "json")
	if err != nil {
		t.Fatalf("trace --format json: %v\n%s", err, out)
	}
	var tr check.Trace
	if err := json.Unmarshal([]byte(out), &tr); err != nil {
		t.Fatalf("output is not json: %v\n%s", err, out)
	}
	if tr.Outcome != check.TraceRouted || len(tr.Crossings) != 1 {
		t.Fatalf("decoded = %+v, want one crossing on a routed answer", tr)
	}
	if tr.Crossings[0].RefDes != "F1" || tr.Crossings[0].EnterPin != "1" || tr.Crossings[0].ExitPin != "2" {
		t.Errorf("crossing = %+v, want F1 pin 1 to pin 2", tr.Crossings[0])
	}
	if tr.From.PinName != "VBUS" || tr.To.Net != "VBUS_PROT" {
		t.Errorf("endpoints = %+v / %+v", tr.From, tr.To)
	}
}

func TestTraceCLIRejectsAMalformedEndpoint(t *testing.T) {
	_, err := runTrace(t, traceFixtureSch, "--from", "J1", "--to", "U1.1")
	if err == nil || !strings.Contains(err.Error(), "<ref-des>.<pin>") {
		t.Errorf("err = %v, want it to say how to name a pin", err)
	}
}
