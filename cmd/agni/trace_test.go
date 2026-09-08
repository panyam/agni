package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"

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

// The json is protojson of the WIRE message, the same shape TraceDesign returns, so this decodes
// into the proto rather than into the Go struct. That is the point of the change: a script reading
// the CLI and a client reading the rpc now parse one thing.
func TestTraceCLIJSON(t *testing.T) {
	out, err := runTrace(t, traceFixtureSch, "--from", "J1.1", "--to", "U1.1", "--format", "json")
	if err != nil {
		t.Fatalf("trace --format json: %v\n%s", err, out)
	}
	var tr webapi.Trace
	if err := protojson.Unmarshal([]byte(out), &tr); err != nil {
		t.Fatalf("output is not protojson of webapi.Trace: %v\n%s", err, out)
	}
	if tr.GetOutcome() != webapi.TraceOutcome_TRACE_OUTCOME_ROUTED || len(tr.GetCrossings()) != 1 {
		t.Fatalf("decoded = %+v, want one crossing on a routed answer", &tr)
	}
	c := tr.GetCrossings()[0]
	if c.GetRefDes() != "F1" || c.GetEnterPin() != "1" || c.GetExitPin() != "2" {
		t.Errorf("crossing = %+v, want F1 pin 1 to pin 2", c)
	}
	if tr.GetFrom().GetPinName() != "VBUS" || tr.GetTo().GetNet() != "VBUS_PROT" {
		t.Errorf("endpoints = %+v / %+v", tr.GetFrom(), tr.GetTo())
	}
	if tr.GetFrom().GetEndpoint().GetRefDes() != "J1" {
		t.Errorf("the endpoint's ref-des did not survive: %+v", tr.GetFrom())
	}
	// EmitUnpopulated keeps a zero field present, so a client never has to tell "absent" from
	// "zero" by whether a key showed up.
	if !strings.Contains(out, "\"stubsElided\"") {
		t.Errorf("a zero-valued field was omitted, so the shape changes per run:\n%s", out)
	}
}

func TestTraceCLIRejectsAMalformedEndpoint(t *testing.T) {
	_, err := runTrace(t, traceFixtureSch, "--from", "J1", "--to", "U1.1")
	if err == nil || !strings.Contains(err.Error(), "<ref-des>.<pin>") {
		t.Errorf("err = %v, want it to say how to name a pin", err)
	}
}

func TestTraceCLIRendersTheRouteOntoTheSchematic(t *testing.T) {
	out := filepath.Join(t.TempDir(), "route.svg")
	stdout, err := runTrace(t, traceFixtureSch, "--from", "J1.1", "--to", "U1.1", "--render", out)
	if err != nil {
		t.Fatalf("trace --render: %v\n%s", err, stdout)
	}
	svg, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read %s: %v", out, err)
	}
	// The route's own colour has to be in the drawing, or the flag wrote a plain render.
	if !bytes.Contains(svg, []byte(traceRouteColor)) {
		t.Errorf("rendered svg carries no route highlight:\n%s", firstBytes(svg, 400))
	}
	if !bytes.Contains(svg, []byte(traceCrossColor)) {
		t.Errorf("rendered svg does not mark the part the route crosses")
	}
	// The board has a drawn schematic, so nothing should claim an auto-layout.
	if strings.Contains(stdout, "auto-layout") {
		t.Errorf("a design with its own sheets was drawn as an auto-layout:\n%s", stdout)
	}
}

// A netlist has no sheets, and refusing to draw it would make the flag useless on most designs. It
// falls back to an auto-layout and SAYS so, which is what stops the picture being mistaken for a
// schematic somebody drew.
func TestTraceCLIFallsBackToAnAutoLayoutAndSaysSo(t *testing.T) {
	out := filepath.Join(t.TempDir(), "route.svg")
	stdout, err := runTrace(t, "testdata/conformance/fires.edn", "--from", "U1.5", "--to", "R5.1", "--render", out)
	if err != nil {
		t.Fatalf("trace --render on a netlist: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "auto-layout") {
		t.Errorf("no note that the drawing is an auto-layout:\n%s", stdout)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("nothing was drawn: %v", err)
	}
}

// A no-route draws the two nets that fail to join, because that is the picture a reader goes looking
// for the moment they read the words.
func TestTraceSpecsDrawTheAnswerWhateverItWas(t *testing.T) {
	routed := check.Trace{
		Outcome:   check.TraceRouted,
		From:      check.TraceEnd{Endpoint: check.Endpoint{RefDes: "U1", Pin: "3"}, Net: "SDA"},
		To:        check.TraceEnd{Endpoint: check.Endpoint{RefDes: "J1", Pin: "1"}, Net: "VCC"},
		Nets:      []check.TraceNet{{Name: "SDA"}, {Name: "VCC"}},
		Crossings: []check.TraceCross{{RefDes: "R1"}},
	}
	specs := traceSpecs(routed)
	if n := countSpecNets(specs); n != 2 {
		t.Errorf("routed: %d net highlights, want 2", n)
	}
	if n := countSpecComponents(specs); n != 1 {
		t.Errorf("routed: %d component highlights, want the one crossing", n)
	}

	noRoute := check.Trace{
		Outcome: check.TraceNoRoute,
		From:    check.TraceEnd{Endpoint: check.Endpoint{RefDes: "U1", Pin: "3"}, Net: "SDA"},
		To:      check.TraceEnd{Endpoint: check.Endpoint{RefDes: "U1", Pin: "2"}, Net: "GND"},
	}
	if n := countSpecNets(traceSpecs(noRoute)); n != 2 {
		t.Errorf("no-route: %d net highlights, want both endpoints' nets", n)
	}

	// An endpoint that resolved to nothing contributes no net, and the one that did still draws.
	unresolved := check.Trace{
		Outcome: check.TraceUnresolved,
		From:    check.TraceEnd{Endpoint: check.Endpoint{RefDes: "U99", Pin: "1"}},
		To:      check.TraceEnd{Endpoint: check.Endpoint{RefDes: "U1", Pin: "3"}, Net: "SDA"},
	}
	if n := countSpecNets(traceSpecs(unresolved)); n != 1 {
		t.Errorf("unresolved: %d net highlights, want only the end that resolved", n)
	}
}

func countSpecNets(specs []*geom.HighlightSpec) int {
	n := 0
	for _, s := range specs {
		n += len(s.GetNets())
	}
	return n
}

func countSpecComponents(specs []*geom.HighlightSpec) int {
	n := 0
	for _, s := range specs {
		n += len(s.GetComponents())
	}
	return n
}

func firstBytes(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}
