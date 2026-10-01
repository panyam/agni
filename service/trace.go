package service

import (
	"context"
	"github.com/panyam/agni/artifact"

	"github.com/panyam/agni/core/check"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// TraceProto and TraceFromProto convert a trace between its Go form and its wire form.
//
// The twin is hand-written because check.Trace is a domain value the engine returns with its
// evidence attached, and its outcome is a Go string type the wire spells as an enum. Per C26 the pair
// has a round-trip guard (TestTraceProtoRoundTrip) and a field census, so a field added to
// check.Trace has to be placed on the wire or excluded explicitly.
func TraceProto(t check.Trace) *webapi.Trace {
	p := &webapi.Trace{
		From:    traceEndProto(t.From),
		To:      traceEndProto(t.To),
		Outcome: traceOutcomeProto(t.Outcome),
		Reason:  t.Reason,
		Radius:  int32(t.Radius),
	}
	for _, c := range t.Crossings {
		p.Crossings = append(p.Crossings, &webapi.TraceCross{
			RefDes: c.RefDes, Class: c.Class, EnterPin: c.EnterPin, ExitPin: c.ExitPin,
			FromNet: c.FromNet, ToNet: c.ToNet,
		})
	}
	for _, n := range t.Nets {
		tn := &webapi.TraceNet{Name: n.Name, StubsElided: int32(n.StubsElided), BusLike: n.BusLike}
		for _, s := range n.Stubs {
			tn.Stubs = append(tn.Stubs, &webapi.TraceStub{RefDes: s.RefDes, Pin: s.Pin, Class: s.Class})
		}
		p.Nets = append(p.Nets, tn)
	}
	return p
}

// TraceFromProto is TraceProto's inverse.
func TraceFromProto(p *webapi.Trace) check.Trace {
	t := check.Trace{
		From:    traceEndFromProto(p.GetFrom()),
		To:      traceEndFromProto(p.GetTo()),
		Outcome: traceOutcomeFromProto(p.GetOutcome()),
		Reason:  p.GetReason(),
		Radius:  int(p.GetRadius()),
	}
	for _, c := range p.GetCrossings() {
		t.Crossings = append(t.Crossings, check.TraceCross{
			RefDes: c.GetRefDes(), Class: c.GetClass(), EnterPin: c.GetEnterPin(),
			ExitPin: c.GetExitPin(), FromNet: c.GetFromNet(), ToNet: c.GetToNet(),
		})
	}
	for _, n := range p.GetNets() {
		tn := check.TraceNet{Name: n.GetName(), StubsElided: int(n.GetStubsElided()), BusLike: n.GetBusLike()}
		for _, s := range n.GetStubs() {
			tn.Stubs = append(tn.Stubs, check.TraceStub{RefDes: s.GetRefDes(), Pin: s.GetPin(), Class: s.GetClass()})
		}
		t.Nets = append(t.Nets, tn)
	}
	return t
}

func traceEndProto(e check.TraceEnd) *webapi.TraceEnd {
	return &webapi.TraceEnd{
		Endpoint: &webapi.TraceEndpoint{RefDes: e.RefDes, Pin: e.Pin},
		PinName:  e.PinName,
		Net:      e.Net,
	}
}

func traceEndFromProto(p *webapi.TraceEnd) check.TraceEnd {
	return check.TraceEnd{
		Endpoint: check.Endpoint{RefDes: p.GetEndpoint().GetRefDes(), Pin: p.GetEndpoint().GetPin()},
		PinName:  p.GetPinName(),
		Net:      p.GetNet(),
	}
}

// traceOutcomeProto maps the Go outcome onto the wire enum. An unknown value maps to UNSPECIFIED
// rather than ROUTED, so a mapping bug reads as "we cannot say" rather than as a connection the engine
// never found. VerdictProto's default arm leans the same way.
func traceOutcomeProto(o check.TraceOutcome) webapi.TraceOutcome {
	switch o {
	case check.TraceRouted:
		return webapi.TraceOutcome_TRACE_OUTCOME_ROUTED
	case check.TraceNoRoute:
		return webapi.TraceOutcome_TRACE_OUTCOME_NO_ROUTE
	case check.TraceUnresolved:
		return webapi.TraceOutcome_TRACE_OUTCOME_UNRESOLVED
	}
	return webapi.TraceOutcome_TRACE_OUTCOME_UNSPECIFIED
}

func traceOutcomeFromProto(o webapi.TraceOutcome) check.TraceOutcome {
	switch o {
	case webapi.TraceOutcome_TRACE_OUTCOME_ROUTED:
		return check.TraceRouted
	case webapi.TraceOutcome_TRACE_OUTCOME_NO_ROUTE:
		return check.TraceNoRoute
	case webapi.TraceOutcome_TRACE_OUTCOME_UNRESOLVED:
		return check.TraceUnresolved
	}
	return ""
}

// TraceDesign walks from one pin to another over the design's netlist and returns the route.
//
// It calls the same check.TracePins the CLI calls, so a route in a terminal and a route on the canvas
// cannot disagree. A design with no netlist is an ERROR rather than an empty trace, because an empty
// answer is indistinguishable from "these two pins are not connected".
func (s *DesignService) TraceDesign(ctx context.Context, req *webapi.TraceDesignRequest) (*webapi.TraceDesignResponse, error) {
	u, err := ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	opts, err := s.readOptions(ctx, u)
	if err != nil {
		return nil, err
	}
	// Tiers come from the design's declaration, like every other read here. The walk is over the
	// NETLIST and the sheets come from the geometry companion, a different artifact for a netlist
	// entry (C32).
	nu, _, gu, err := s.projects.TierURIs(ctx, u, artifact.URI{}, req.GetAsNamed())
	if err != nil {
		return nil, err
	}
	d, err := s.loader.Design(ctx, nu, opts...)
	if err != nil {
		return nil, err
	}
	from := check.Endpoint{RefDes: req.GetFrom().GetRefDes(), Pin: req.GetFrom().GetPin()}
	to := check.Endpoint{RefDes: req.GetTo().GetRefDes(), Pin: req.GetTo().GetPin()}
	m := check.NewModel(d)
	t := check.TracePins(m, from, to, int(req.GetHops()))
	p := TraceProto(t)
	// Where each net of the answer is drawn, so the panel can badge it and both renderers can open
	// the sheet the route is actually on rather than the design's first (agni issue 657).
	AnnotateTraceSheets(p, BuildGeometry(ctx, s.loader, gu, opts...), m)
	return &webapi.TraceDesignResponse{Trace: p}, nil
}
